package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
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

// limitConcurrent bounds concurrent calls and stops waiting when the client disconnects.
func limitConcurrent(n int, next http.HandlerFunc) http.HandlerFunc {
	slots := make(chan struct{}, n)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next(w, r)
		case <-r.Context().Done():
		}
	}
}

// webhook acknowledges verified deliveries before worker processing to meet GitHub's 10-second timeout.
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "payload exceeds 25 MB")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_payload", "payload could not be read")
		return
	}
	if !github.VerifySignature(s.Cfg.GitHub.WebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		writeError(w, r, http.StatusUnauthorized, "invalid_signature", "signature verification failed")
		return
	}
	delivery, event := r.Header.Get("X-GitHub-Delivery"), r.Header.Get("X-GitHub-Event")
	if delivery == "" || len(delivery) > maxDeliveryIDLen || !validEvent.MatchString(event) {
		writeError(w, r, http.StatusBadRequest, "invalid_delivery", "missing or malformed delivery headers")
		return
	}
	var meta struct {
		Action       *string `json:"action"`
		Installation *struct {
			ID int64 `json:"id"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", "payload is not JSON")
		return
	}
	if meta.Action != nil && len(*meta.Action) > maxActionLen {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", "payload action is too long")
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
		_, err = s.Jobs.InsertTx(r.Context(), tx, jobs.ProcessWebhookArgs{DeliveryID: delivery}, nil)
		return err
	})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "duplicate": duplicate})
}
