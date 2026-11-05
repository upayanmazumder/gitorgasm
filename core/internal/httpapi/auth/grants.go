package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// GrantMaxAge bounds how long a session keeps installation grants without GitHub confirming them.
// A user removed from an organization loses access on the first request after this age.
const GrantMaxAge = 15 * time.Minute

// verifiedInstallations keeps only installations whose data the user may read as a whole.
// GET /user/installations also lists installations where the user is merely a collaborator on
// one repository, so each entry needs proof of membership or ownership.
func (s *Handler) verifiedInstallations(ctx context.Context, token string, githubUserID int64) ([]models.Installation, error) {
	listed, err := s.GitHub.ListUserInstallations(ctx, token)
	if err != nil {
		return nil, err
	}
	kept := make([]models.Installation, 0, len(listed))
	for i := range listed {
		ok, err := s.mayReadInstallation(ctx, token, githubUserID, &listed[i])
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, listed[i])
		}
	}
	return kept, nil
}

func (s *Handler) mayReadInstallation(ctx context.Context, token string, githubUserID int64, inst *models.Installation) (bool, error) {
	if inst.Account == nil {
		return false, nil
	}
	switch inst.Account.Kind() {
	case "User":
		return inst.Account.ID == githubUserID, nil
	case "Organization":
		// orgs/get-membership-for-authenticated-user: 404 means not affiliated; 403 means the
		// organization blocks the app. Neither grants access.
		m, err := s.GitHub.OrganizationMembership(ctx, token, inst.Account.Name())
		if github.IsKind(err, github.KindNotFound) || github.IsKind(err, github.KindForbidden) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return m.State == "active" && (m.Role == "admin" || m.Role == "member") &&
			m.Organization != nil && m.Organization.ID == inst.Account.ID &&
			(m.User == nil || m.User.ID == githubUserID), nil
	default:
		return false, nil
	}
}

// reverifyGrants replaces the user's grants from GitHub using this session's own token.
// It writes the response itself and returns false when the request must not continue.
func (s *Handler) reverifyGrants(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) bool {
	ctx := r.Context()
	reauthenticate := func() bool {
		response.Error(w, r, http.StatusUnauthorized, "github_reauthentication_required", "Sign in again to reconnect your GitHub account.")
		return false
	}
	cred, err := identity.LookupSessionCredential(ctx, s.Pool, u.UserID, u.SessionHash, s.Now())
	if errors.Is(err, store.ErrNotFound) {
		response.Error(w, r, http.StatusUnauthorized, "unauthenticated", "session expired")
		return false
	}
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return false
	}
	if s.GitHub == nil || s.Sealer == nil || cred.KeyVersion != secure.KeyVersion || len(cred.TokenSealed) == 0 ||
		(cred.TokenExpiresAt != nil && !cred.TokenExpiresAt.After(s.Now())) {
		return reauthenticate()
	}
	plain, err := s.Sealer.Open(cred.TokenSealed, []byte(strconv.FormatInt(u.UserID, 10)))
	if err != nil || len(plain) == 0 {
		return reauthenticate()
	}
	token := string(plain)
	clear(plain)

	insts, err := s.verifiedInstallations(ctx, token, u.GitHubID)
	if github.IsKind(err, github.KindUnauthorized) {
		// GitHub rejected the stored token, so this session can never be verified again.
		if err := identity.DeleteSession(ctx, s.Pool, u.SessionHash); err != nil {
			response.InternalError(s.Log, w, r, err)
			return false
		}
		return reauthenticate()
	}
	if err != nil {
		// Fail closed: a user must not keep access by making GitHub verification fail.
		s.refreshFailure(w, r, err)
		return false
	}
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := storeGrants(ctx, tx, u.UserID, insts); err != nil {
			return err
		}
		return identity.MarkGrantsVerified(ctx, tx, u.SessionHash)
	})
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return false
	}
	return true
}

// storeGrants mirrors the verified installations and makes them the user's complete grant set.
func storeGrants(ctx context.Context, tx pgx.Tx, userID int64, insts []models.Installation) ([]int64, error) {
	ids := make([]int64, 0, len(insts))
	for i := range insts {
		if err := tenancy.UpsertInstallation(ctx, tx, &insts[i]); err != nil {
			return nil, err
		}
		ids = append(ids, insts[i].ID)
	}
	return ids, tenancy.ReplaceUserInstallations(ctx, tx, userID, ids)
}

// refreshFailure maps a failed re-verification to a retryable error without upstream detail.
func (s *Handler) refreshFailure(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		s.Log.WarnContext(r.Context(), "grant re-verification failed", "operation", apiErr.Operation, "status", apiErr.Status, "kind", int(apiErr.Kind))
		if apiErr.Kind == github.KindRateLimited {
			w.Header().Set("Retry-After", strconv.Itoa(int(apiErr.RetryAfter.Seconds())))
			response.Error(w, r, http.StatusServiceUnavailable, "github_rate_limited", "GitHub rate limit reached; retry later")
			return
		}
	} else {
		s.Log.ErrorContext(r.Context(), "grant re-verification failed", "error", err)
	}
	response.Error(w, r, http.StatusServiceUnavailable, "github_unavailable", "GitHub could not confirm your access; retry later")
}
