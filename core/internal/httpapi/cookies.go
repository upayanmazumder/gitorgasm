package httpapi

import (
	"net/http"
	"time"
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
