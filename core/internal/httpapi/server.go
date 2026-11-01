// Package httpapi implements the public HTTP API (contract: contracts/openapi/v1.yaml).
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

// Server holds dependencies of the HTTP handlers.
type Server struct {
	Pool   *pgxpool.Pool
	Jobs   *river.Client[pgx.Tx]
	GitHub *github.Client // nil when the GitHub App is not configured
	Sealer *secure.Sealer
	Cfg    *config.Config
	Log    *slog.Logger
	Scorer *scorer.Runner
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
	mux.HandleFunc("GET /v1/organizations/{login}/scores", s.authed(s.getScores))
	mux.HandleFunc("POST /v1/organizations/{login}/scores/query", s.authed(s.queryScores))
	mux.HandleFunc("GET /v1/organizations/{login}/scores/presets", s.authed(s.listScorePresets))
	mux.HandleFunc("PUT /v1/organizations/{login}/scores/presets/{name}", s.authed(s.saveScorePreset))
	mux.HandleFunc("DELETE /v1/organizations/{login}/scores/presets/{name}", s.authed(s.deleteScorePreset))

	mux.HandleFunc("POST /webhooks/github", s.requireGitHub(limitConcurrent(maxWebhookReads, s.webhook)))

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

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz requires PostgreSQL before Kubernetes routes traffic to the pod.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "ok"})
}
