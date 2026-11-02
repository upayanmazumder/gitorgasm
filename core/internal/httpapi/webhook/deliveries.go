package webhook

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

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

// Header and payload limits match webhook_deliveries CHECK constraints.
const (
	maxDeliveryIDLen = 100
	maxActionLen     = 100
)

// validEvent matches GitHub event names such as installation_repositories.
var validEvent = regexp.MustCompile(`^[a-z_]{1,64}$`)

// deliver acknowledges verified deliveries before worker processing to meet GitHub's timeout.
func (s *Handler) deliver(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
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

func (s *Handler) Handler() http.HandlerFunc {
	return admission.LimitConcurrent(maxWebhookReads, s.deliver)
}
