// Package httpapi implements the public HTTP API (contract: contracts/openapi/v1.yaml).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/riverqueue/river"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/upayanmazumder/gitorgasm/services/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/services/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/services/core/internal/secure"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gitorgasm_http_requests_total",
		Help: "HTTP requests by route pattern, method, and status code.",
	}, []string{"route", "method", "status"})
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gitorgasm_http_request_duration_seconds",
		Help:    "HTTP request latency by route pattern.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"route"})
)

// Server holds dependencies of the HTTP handlers.
type Server struct {
	Pool   *pgxpool.Pool
	Jobs   *river.Client[pgx.Tx]
	GitHub *github.Client // nil when the GitHub App is not configured
	Sealer *secure.Sealer
	Cfg    *config.Config
	Log    *slog.Logger
	Now    func() time.Time
}

// Handler returns the routed and instrumented handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)

	mux.HandleFunc("GET /v1/auth/login", s.requireGitHub(s.login))
	mux.HandleFunc("GET /v1/auth/callback", s.requireGitHub(s.callback))
	mux.HandleFunc("GET /v1/auth/install", s.requireGitHub(s.install))
	mux.HandleFunc("POST /v1/auth/logout", s.logout)
	mux.HandleFunc("GET /v1/me", s.authed(s.me))
	mux.HandleFunc("GET /v1/organizations", s.authed(s.listOrganizations))
	mux.HandleFunc("GET /v1/organizations/{login}/repositories", s.authed(s.listRepositories))
	mux.HandleFunc("POST /v1/organizations/{login}/sync", s.authed(s.requestSync))

	mux.HandleFunc("POST /webhooks/github", s.requireGitHub(s.webhook))

	var h http.Handler = mux
	h = s.checkOrigin(h)
	h = s.accessLog(h)
	h = securityHeaders(h)
	h = s.recoverer(h)
	h = requestID(h)
	return otelhttp.NewHandler(h, "http", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if r.Pattern != "" {
			return r.Pattern
		}
		return r.Method
	}))
}

// apiError is the single error body shape for every endpoint.
type apiError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	var e apiError
	e.Error.Code, e.Error.Message, e.Error.RequestID = code, msg, RequestIDFrom(r.Context())
	writeJSON(w, status, e)
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.Log.ErrorContext(r.Context(), "request failed", "error", err, "route", r.Pattern, "request_id", RequestIDFrom(r.Context()))
	writeError(w, r, http.StatusInternalServerError, "internal", "internal error")
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz reports ready only when PostgreSQL answers. Kubernetes removes the pod from
// Service endpoints otherwise.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "ok"})
}

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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		elapsed := time.Since(start)
		httpRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
		httpDuration.WithLabelValues(route).Observe(elapsed.Seconds())
		if route == "GET /healthz" || route == "GET /readyz" {
			return
		}
		s.Log.InfoContext(r.Context(), "request", "method", r.Method, "route", route, "status", rec.status,
			"duration_ms", elapsed.Milliseconds(), "request_id", RequestIDFrom(r.Context()))
	})
}

// checkOrigin rejects cross-site state-changing requests. Browsers always send Origin on
// POST. The webhook route is authenticated by its HMAC signature instead.
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
