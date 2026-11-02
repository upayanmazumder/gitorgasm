package organization

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/internal/access"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

func (s *Handler) Repositories(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	q := r.URL.Query()
	limit := defaultPageSize
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPageSize {
			response.Error(w, r, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = n
	}
	var after *tenancy.RepoCursor
	if v := q.Get("cursor"); v != "" {
		c, err := decodeCursor(v)
		if err != nil {
			response.Error(w, r, http.StatusBadRequest, "invalid_cursor", "cursor is malformed")
			return
		}
		after = c
	}
	instID, ok := access.ResolveOrganization(s.Pool, s.Log, w, r, u)
	if !ok {
		return
	}
	repos, err := tenancy.ListRepositories(r.Context(), s.Pool, instID, after, limit+1)
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return
	}
	resp := listResponse[tenancy.Repository]{Data: repos}
	if len(repos) > limit {
		resp.Data = repos[:limit]
		last := resp.Data[limit-1]
		next := encodeCursor(tenancy.RepoCursor{Name: strings.ToLower(last.Name), ID: last.GitHubID})
		resp.NextCursor = &next
	}
	response.JSON(w, http.StatusOK, resp)
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
