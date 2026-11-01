package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
)

// createSession binds sealed tokens to their user id so copied ciphertext cannot decrypt.
func (s *Server) createSession(ctx context.Context, tx pgx.Tx, sessionID string, userID int64, tok *models.UserToken) error {
	ad := []byte(strconv.FormatInt(userID, 10))
	ns := identity.NewSession{
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
	return identity.CreateSession(ctx, tx, ns)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cookieName(sessionCookie)); err == nil {
		if err := identity.DeleteSession(r.Context(), s.Pool, secure.Hash(c.Value)); err != nil {
			s.internalError(w, r, err)
			return
		}
	}
	s.clearCookie(w, sessionCookie)
	s.clearCookie(w, oauthCookie)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authed(next func(http.ResponseWriter, *http.Request, *identity.SessionUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.cookieName(sessionCookie))
		if err != nil || c.Value == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "sign in required")
			return
		}
		u, err := identity.LookupSession(r.Context(), s.Pool, secure.Hash(c.Value))
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

func (s *Server) me(w http.ResponseWriter, _ *http.Request, u *identity.SessionUser) {
	writeJSON(w, http.StatusOK, u)
}
