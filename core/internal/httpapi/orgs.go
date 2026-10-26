package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

type listResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func (s *Server) listOrganizations(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	orgs, err := store.ListOrganizations(r.Context(), s.Pool, u.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[store.Organization]{Data: orgs})
}

// resolveOrg returns the installation for {login} if the user may access it. Unknown and
// forbidden organizations both return 404 so that tenants cannot be enumerated.
func (s *Server) resolveOrg(w http.ResponseWriter, r *http.Request, u *store.SessionUser) (int64, bool) {
	id, err := store.AccessibleInstallation(r.Context(), s.Pool, u.UserID, r.PathValue("login"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "organization_not_found", "organization not found")
		return 0, false
	}
	if err != nil {
		s.internalError(w, r, err)
		return 0, false
	}
	return id, true
}

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

func (s *Server) listRepositories(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	q := r.URL.Query()
	limit := defaultPageSize
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPageSize {
			writeError(w, r, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = n
	}
	var after *store.RepoCursor
	if v := q.Get("cursor"); v != "" {
		c, err := decodeCursor(v)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_cursor", "cursor is malformed")
			return
		}
		after = c
	}
	instID, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	// Fetch one extra row to learn whether another page exists.
	repos, err := store.ListRepositories(r.Context(), s.Pool, instID, after, limit+1)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	resp := listResponse[store.Repository]{Data: repos}
	if len(repos) > limit {
		resp.Data = repos[:limit]
		last := resp.Data[limit-1]
		next := encodeCursor(store.RepoCursor{Name: strings.ToLower(last.Name), ID: last.GitHubID})
		resp.NextCursor = &next
	}
	writeJSON(w, http.StatusOK, resp)
}

func encodeCursor(c store.RepoCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(v string) (*store.RepoCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	var c store.RepoCursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID <= 0 {
		return nil, errors.New("invalid cursor")
	}
	return &c, nil
}

type syncAccepted struct {
	JobID     int64 `json:"job_id"`
	Duplicate bool  `json:"duplicate"`
}

// requestSync queues a repository sync. Pending syncs for the same installation are
// deduplicated, so repeated clicks cost one GitHub pass.
func (s *Server) requestSync(w http.ResponseWriter, r *http.Request, u *store.SessionUser) {
	instID, ok := s.resolveOrg(w, r, u)
	if !ok {
		return
	}
	var res syncAccepted
	err := pgx.BeginFunc(r.Context(), s.Pool, func(tx pgx.Tx) error {
		ins, err := s.Jobs.InsertTx(r.Context(), tx, jobs.SyncInstallationArgs{InstallationID: instID}, nil)
		if err != nil {
			return err
		}
		res = syncAccepted{JobID: ins.Job.ID, Duplicate: ins.UniqueSkippedAsDuplicate}
		return store.Audit(r.Context(), tx, store.AuditEvent{
			ActorUserID: &u.UserID, Action: "installation.sync_requested", RequestID: RequestIDFrom(r.Context()),
			Target: "installation:" + strconv.FormatInt(instID, 10),
		})
	})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}

// maxWebhookBytes matches GitHub's documented 25 MB payload cap.
const maxWebhookBytes = 25 << 20

// webhook verifies, stores, and queues a delivery, then acknowledges at once. Processing
// happens in the worker so GitHub's 10 s delivery timeout is never at risk.
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "payload exceeds 25 MB")
		return
	}
	if !github.VerifySignature(s.Cfg.GitHub.WebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		writeError(w, r, http.StatusUnauthorized, "invalid_signature", "signature verification failed")
		return
	}
	delivery, event := r.Header.Get("X-GitHub-Delivery"), r.Header.Get("X-GitHub-Event")
	if delivery == "" || event == "" || len(delivery) > 100 {
		writeError(w, r, http.StatusBadRequest, "invalid_delivery", "missing delivery headers")
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
	d := store.WebhookDelivery{DeliveryID: delivery, Event: event, Action: meta.Action, Payload: body}
	if meta.Installation != nil {
		d.InstallationID = &meta.Installation.ID
	}
	duplicate := false
	err = pgx.BeginFunc(r.Context(), s.Pool, func(tx pgx.Tx) error {
		inserted, err := store.InsertWebhookDelivery(r.Context(), tx, d)
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
