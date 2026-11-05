package webhook

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/admission"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/webhooks"
)

// maxWebhookBytes matches GitHub's documented 25 MB payload cap.
const maxWebhookBytes = 25 << 20

// maxWebhookReads caps memory pinned by unsigned bodies at maxWebhookReads × maxWebhookBytes.
const maxWebhookReads = 2

// webhookBodyTimeout bounds how long an unsigned body may take to arrive while it holds a read slot.
// It is well below GitHub's 10 s delivery timeout and far above the time a 25 MB body needs over a
// datacenter link, so only stalled or trickling senders are cut off.
const webhookBodyTimeout = 5 * time.Second

// Header and payload limits match webhook_deliveries CHECK constraints.
const (
	maxDeliveryIDLen = 100
	maxActionLen     = 100
)

// validEvent matches GitHub event names such as installation_repositories.
var validEvent = regexp.MustCompile(`^[a-z_]{1,64}$`)

// deliver acknowledges verified deliveries before worker processing to meet GitHub's timeout.
func (s *Handler) deliver(w http.ResponseWriter, r *http.Request) {
	body, err := s.readBody(w, r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Error(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "payload exceeds 25 MB")
			return
		}
		response.Error(w, r, http.StatusBadRequest, "invalid_payload", "payload could not be read")
		return
	}
	if !github.VerifySignature(s.Cfg.GitHub.WebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		response.Error(w, r, http.StatusUnauthorized, "invalid_signature", "signature verification failed")
		return
	}
	delivery, event := r.Header.Get("X-GitHub-Delivery"), r.Header.Get("X-GitHub-Event")
	if delivery == "" || len(delivery) > maxDeliveryIDLen || !validEvent.MatchString(event) {
		response.Error(w, r, http.StatusBadRequest, "invalid_delivery", "missing or malformed delivery headers")
		return
	}
	var meta struct {
		Action       *string `json:"action"`
		Installation *struct {
			ID int64 `json:"id"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		response.Error(w, r, http.StatusBadRequest, "invalid_payload", "payload is not JSON")
		return
	}
	if meta.Action != nil && len(*meta.Action) > maxActionLen {
		response.Error(w, r, http.StatusBadRequest, "invalid_payload", "payload action is too long")
		return
	}
	d := webhooks.WebhookDelivery{DeliveryID: delivery, Event: event, Action: meta.Action, Payload: body}
	if meta.Installation != nil && meta.Installation.ID > 0 {
		d.InstallationID = &meta.Installation.ID
	}
	duplicate := false
	err = pgx.BeginFunc(r.Context(), s.Pool, func(tx pgx.Tx) error {
		inserted, err := webhooks.InsertWebhookDelivery(r.Context(), tx, d)
		if err != nil || !inserted {
			duplicate = !inserted
			return err
		}
		_, err = s.Jobs.InsertTx(r.Context(), tx, queue.ProcessWebhook{DeliveryID: delivery}, nil)
		return err
	})
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"accepted": true, "duplicate": duplicate})
}

// readBody reads the capped body under a read deadline.
//
// After a complete read the deadline is cleared: an expired one would make the server's background
// connection read cancel the request context. After a failed read it is forced to expire: the server
// otherwise tries to drain the unread body before writing the error response, which would let a stalled
// sender keep the slot until the server's ReadTimeout.
func (s *Handler) readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	timeout := s.bodyTimeout
	if timeout <= 0 {
		timeout = webhookBodyTimeout
	}
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Now().Add(timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return nil, err
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		_ = rc.SetReadDeadline(time.Now())
		return nil, err
	}
	if err := rc.SetReadDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return nil, err
	}
	return body, nil
}

func (s *Handler) Handler() http.HandlerFunc {
	return admission.LimitConcurrent(maxWebhookReads, s.deliver)
}
