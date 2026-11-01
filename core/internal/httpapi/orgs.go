package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/audit"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

type listResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func (s *Server) listOrganizations(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	orgs, err := tenancy.ListOrganizations(r.Context(), s.Pool, u.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[tenancy.Organization]{Data: orgs})
}

// resolveOrg returns 404 for unknown and forbidden organizations to prevent tenant enumeration.
func (s *Server) resolveOrg(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) (int64, bool) {
	id, err := tenancy.AccessibleInstallation(r.Context(), s.Pool, u.UserID, r.PathValue("login"))
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

func (s *Server) listRepositories(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
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
	var after *tenancy.RepoCursor
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
	repos, err := tenancy.ListRepositories(r.Context(), s.Pool, instID, after, limit+1)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	resp := listResponse[tenancy.Repository]{Data: repos}
	if len(repos) > limit {
		resp.Data = repos[:limit]
		last := resp.Data[limit-1]
		next := encodeCursor(tenancy.RepoCursor{Name: strings.ToLower(last.Name), ID: last.GitHubID})
		resp.NextCursor = &next
	}
	writeJSON(w, http.StatusOK, resp)
}

func encodeCursor(c tenancy.RepoCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(v string) (*tenancy.RepoCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	var c tenancy.RepoCursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID <= 0 {
		return nil, errors.New("invalid cursor")
	}
	return &c, nil
}

type syncAccepted struct {
	JobID     int64 `json:"job_id"`
	Duplicate bool  `json:"duplicate"`
}

// requestSync deduplicates pending syncs for the same installation.
func (s *Server) requestSync(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
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
		return audit.Audit(r.Context(), tx, audit.AuditEvent{
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
