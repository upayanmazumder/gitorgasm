package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"runtime/debug"

	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

func (s *Server) requireGitHub(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.GitHub == nil {
			writeError(w, r, http.StatusServiceUnavailable, "github_not_configured", "the GitHub App is not configured on this deployment")
			return
		}
		next(w, r)
	}
}

type ctxKey int

const requestIDKey ctxKey = iota

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// requestID accepts a well-formed inbound X-Request-Id or generates one, and echoes it.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !validRequestID.MatchString(id) {
			id = secure.RandomToken()[:22]
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// RequestIDFrom returns the request id stored in the context.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				s.Log.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				writeError(w, r, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// checkOrigin guards state-changing browser requests; webhooks use HMAC instead.
func (s *Server) checkOrigin(next http.Handler) http.Handler {
	allowed := s.Cfg.AppOrigin.Scheme + "://" + s.Cfg.AppOrigin.Host
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if unsafe && r.URL.Path != "/webhooks/github" && r.Header.Get("Origin") != allowed {
			writeError(w, r, http.StatusForbidden, "origin_rejected", "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}
