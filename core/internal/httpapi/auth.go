package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/audit"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// login starts sign-in or a post-install grant refresh with browser-bound state and PKCE.
// GitHub's Setup URL points here; installation_id and all other incoming query values
// are deliberately ignored. Only GET /user/installations authorizes installations.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	state, verifier := secure.RandomToken(), secure.RandomToken()
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	s.setCookie(w, oauthCookie, state+"."+verifier, 10*time.Minute)
	http.Redirect(w, r, s.GitHub.AuthorizeURL(state, challenge), http.StatusFound)
}

func (s *Server) install(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.GitHub.InstallURL(), http.StatusFound)
}

// callback stores identity, installation grants, and a session before queuing syncs.
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := r.Cookie(s.cookieName(oauthCookie))
	s.clearCookie(w, oauthCookie)
	if err != nil {
		response.Error(w, r, http.StatusBadRequest, "oauth_state_missing", "login session expired; start again")
		return
	}
	state, verifier, ok := strings.Cut(c.Value, ".")
	got := r.URL.Query().Get("state")
	if !ok || state == "" || verifier == "" || got == "" || subtle.ConstantTimeCompare([]byte(state), []byte(got)) != 1 {
		response.Error(w, r, http.StatusBadRequest, "oauth_state_mismatch", "login state mismatch")
		return
	}
	code := r.URL.Query().Get("code")
	if r.URL.Query().Get("error") != "" || code == "" {
		response.Error(w, r, http.StatusBadRequest, "oauth_denied", "GitHub did not return an authorization code")
		return
	}

	tok, err := s.GitHub.ExchangeCode(ctx, code, verifier)
	if err != nil {
		s.githubError(w, r, err)
		return
	}
	user, err := s.GitHub.GetAuthenticatedUser(ctx, tok.AccessToken)
	if err != nil {
		s.githubError(w, r, err)
		return
	}
	insts, err := s.GitHub.ListUserInstallations(ctx, tok.AccessToken)
	if err != nil {
		s.githubError(w, r, err)
		return
	}

	sessionID := secure.RandomToken()
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		userID, err := identity.UpsertUser(ctx, tx, user)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(insts))
		for i := range insts {
			if err := tenancy.UpsertInstallation(ctx, tx, &insts[i]); err != nil {
				return err
			}
			ids = append(ids, insts[i].ID)
		}
		if err := tenancy.ReplaceUserInstallations(ctx, tx, userID, ids); err != nil {
			return err
		}
		if err := s.createSession(ctx, tx, sessionID, userID, tok); err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.Jobs.InsertTx(ctx, tx, queue.SyncInstallation{InstallationID: id}, nil); err != nil {
				return err
			}
		}
		return audit.Audit(ctx, tx, audit.AuditEvent{
			ActorUserID: &userID, Action: "session.created", RequestID: requestid.From(ctx),
			Metadata: map[string]any{"installations": len(ids)},
		})
	})
	if err != nil {
		response.InternalError(s.Log, w, r, err)
		return
	}
	s.setCookie(w, sessionCookie, sessionID, s.Cfg.SessionTTL)
	http.Redirect(w, r, s.Cfg.AppOrigin.JoinPath("organizations").String(), http.StatusFound)
}
