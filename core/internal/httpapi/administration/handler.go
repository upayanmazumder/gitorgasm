// Package administration exposes owner-only organization administration receipts and directories.
package administration

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	domain "github.com/upayanmazumder/gitorgasm/core/internal/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
)

type Handler struct {
	Pool   *pgxpool.Pool
	Jobs   *river.Client[pgx.Tx]
	GitHub *github.Client
	Sealer *secure.Sealer
	Log    *slog.Logger
	Now    func() time.Time
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, u *identity.SessionUser, requireOwner bool) *domain.Access {
	authorizer := domain.Authorizer{Pool: h.Pool, GitHub: h.GitHub, Sealer: h.Sealer, Now: h.Now}
	access, err := authorizer.Check(r.Context(), u.UserID, u.SessionHash, r.PathValue("login"))
	if err != nil {
		var denied *domain.AccessError
		if errors.As(err, &denied) {
			response.Error(w, r, denied.Status, denied.Code, denied.Message)
		} else {
			response.InternalError(h.Log, w, r, errors.New("administration authorization failed"))
		}
		return nil
	}
	if requireOwner && !access.CanManage {
		code := "not_owner"
		if access.Reason != nil {
			code = *access.Reason
		}
		response.Error(w, r, http.StatusForbidden, code, "organization owner access and Members write permission are required")
		return nil
	}
	return access
}

func (h *Handler) Capabilities(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
	if !validateQuery(w, r) {
		return
	}
	if access := h.authorize(w, r, u, false); access != nil {
		response.JSON(w, http.StatusOK, struct {
			Data *domain.Access `json:"data"`
		}{access})
	}
}

// githubError exposes only fixed product copy, never an upstream message or token.
func (h *Handler) githubError(w http.ResponseWriter, r *http.Request, err error) {
	var upstream *github.APIError
	if errors.As(err, &upstream) {
		h.Log.WarnContext(r.Context(), "administration GitHub read failed", "status", upstream.Status, "kind", int(upstream.Kind))
		switch upstream.Kind {
		case github.KindRateLimited:
			seconds := max(1, int(upstream.RetryAfter.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			response.Error(w, r, http.StatusServiceUnavailable, "github_rate_limited", "GitHub rate limit reached; retry later")
			return
		case github.KindUnauthorized:
			response.Error(w, r, http.StatusUnauthorized, "github_reauthentication_required", "sign in again to authorize GitHub administration")
			return
		case github.KindForbidden:
			response.Error(w, r, http.StatusForbidden, "github_forbidden", "GitHub denied access to this organization resource")
			return
		case github.KindNotFound:
			response.Error(w, r, http.StatusNotFound, "github_not_found", "the organization resource is no longer available")
			return
		case github.KindInvalid:
			response.Error(w, r, http.StatusBadRequest, "github_invalid", "GitHub could not accept this directory request")
			return
		}
	}
	response.Error(w, r, http.StatusBadGateway, "github_unavailable", "GitHub is unavailable; try again later")
}
