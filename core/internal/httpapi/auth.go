package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// cookieName uses __Host- for Secure, host-only, Path=/ cookies.
func (s *Server) cookieName(base string) string {
	if s.Cfg.CookieSecure {
		return "__Host-" + base
	}
	return base
}

func (s *Server) setCookie(w http.ResponseWriter, base, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // HttpOnly and SameSite are set; Secure is configured
		Name:     s.cookieName(base),
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   s.Cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearCookie(w http.ResponseWriter, base string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // same attributes as setCookie
		Name: s.cookieName(base), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.Cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

const (
	sessionCookie = "gitorgasm_session"
	oauthCookie   = "gitorgasm_oauth"
)

// login binds OAuth state and PKCE verifier to a short-lived HttpOnly browser cookie.
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
		writeError(w, r, http.StatusBadRequest, "oauth_state_missing", "login session expired; start again")
		return
	}
	state, verifier, ok := strings.Cut(c.Value, ".")
	got := r.URL.Query().Get("state")
	if !ok || got == "" || subtle.ConstantTimeCompare([]byte(state), []byte(got)) != 1 {
		writeError(w, r, http.StatusBadRequest, "oauth_state_mismatch", "login state mismatch")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, r, http.StatusBadRequest, "oauth_denied", "GitHub did not return an authorization code")
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
		userID, err := store.UpsertUser(ctx, tx, user)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(insts))
		syncs := make([]river.InsertManyParams, 0, len(insts))
		for i := range insts {
			if err := store.UpsertInstallation(ctx, tx, &insts[i]); err != nil {
				return err
			}
			ids = append(ids, insts[i].ID)
			syncs = append(syncs, river.InsertManyParams{Args: jobs.SyncInstallationArgs{InstallationID: insts[i].ID}})
		}
		if err := store.ReplaceUserInstallations(ctx, tx, userID, ids); err != nil {
			return err
		}
		if err := s.createSession(ctx, tx, sessionID, userID, tok); err != nil {
			return err
		}
		if len(syncs) > 0 {
			if _, err := s.Jobs.InsertManyTx(ctx, tx, syncs); err != nil {
				return err
			}
		}
		return store.Audit(ctx, tx, store.AuditEvent{
			ActorUserID: &userID, Action: "session.created", RequestID: RequestIDFrom(ctx),
			Metadata: map[string]any{"installations": len(ids)},
		})
	})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.setCookie(w, sessionCookie, sessionID, s.Cfg.SessionTTL)
	http.Redirect(w, r, s.Cfg.AppOrigin.String(), http.StatusFound)
}

// createSession binds sealed tokens to their user id so copied ciphertext cannot decrypt.
func (s *Server) createSession(ctx context.Context, tx pgx.Tx, sessionID string, userID int64, tok *github.UserToken) error {
	ad := []byte(strconv.FormatInt(userID, 10))
	ns := store.NewSession{
		IDHash:      secure.Hash(sessionID),
		UserID:      userID,
		KeyVersion:  secure.KeyVersion,
		TokenSealed: s.Sealer.Seal([]byte(tok.AccessToken), ad),
		ExpiresAt:   s.Now().Add(s.Cfg.SessionTTL),
	}
	if tok.RefreshToken != "" {
		ns.RefreshSealed = s.Sealer.Seal([]byte(tok.RefreshToken), ad)
	}
	if !tok.ExpiresAt.IsZero() {
		ns.TokenExpiresAt = &tok.ExpiresAt
	}
	return store.CreateSession(ctx, tx, ns)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cookieName(sessionCookie)); err == nil {
		if err := store.DeleteSession(r.Context(), s.Pool, secure.Hash(c.Value)); err != nil {
			s.internalError(w, r, err)
			return
		}
	}
	s.clearCookie(w, sessionCookie)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authed(next func(http.ResponseWriter, *http.Request, *store.SessionUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.cookieName(sessionCookie))
		if err != nil || c.Value == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "sign in required")
			return
		}
		u, err := store.LookupSession(r.Context(), s.Pool, secure.Hash(c.Value))
		if errors.Is(err, store.ErrNotFound) {
			s.clearCookie(w, sessionCookie)
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "session expired")
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		next(w, r, u)
	}
}

func (s *Server) me(w http.ResponseWriter, _ *http.Request, u *store.SessionUser) {
	writeJSON(w, http.StatusOK, u)
}

// githubError maps upstream failures to stable API errors without leaking upstream bodies.
func (s *Server) githubError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		s.Log.WarnContext(r.Context(), "github call failed", "operation", apiErr.Operation, "status", apiErr.Status, "kind", int(apiErr.Kind))
		switch apiErr.Kind {
		case github.KindRateLimited:
			w.Header().Set("Retry-After", strconv.Itoa(int(apiErr.RetryAfter.Seconds())))
			writeError(w, r, http.StatusServiceUnavailable, "github_rate_limited", "GitHub rate limit reached; retry later")
			return
		case github.KindInvalid, github.KindUnauthorized:
			writeError(w, r, http.StatusBadRequest, "github_auth_failed", "GitHub rejected the authorization; start again")
			return
		}
	}
	s.Log.ErrorContext(r.Context(), "github call failed", "error", err)
	writeError(w, r, http.StatusBadGateway, "github_unavailable", "GitHub request failed")
}
