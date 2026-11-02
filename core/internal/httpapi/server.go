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
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/auth"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/organization"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/scores"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/webhook"
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
	sessions := &auth.Handler{
		Pool: s.Pool, Jobs: s.Jobs, GitHub: s.GitHub, Sealer: s.Sealer,
		Cfg: s.Cfg, Log: s.Log, Now: s.Now,
	}
	organizations := &organization.Handler{Pool: s.Pool, Jobs: s.Jobs, Log: s.Log}
	scoring := &scores.Handler{Pool: s.Pool, Scorer: s.Scorer, Log: s.Log, Now: s.Now}
	webhooks := &webhook.Handler{Pool: s.Pool, Jobs: s.Jobs, Cfg: s.Cfg, Log: s.Log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)

	mux.HandleFunc("GET /v1/auth/login", s.requireGitHub(sessions.Login))
	mux.HandleFunc("GET /v1/auth/callback", s.requireGitHub(sessions.Callback))
	mux.HandleFunc("GET /v1/auth/install", s.requireGitHub(sessions.Install))
	mux.HandleFunc("POST /v1/auth/logout", sessions.Logout)
	mux.HandleFunc("GET /v1/me", sessions.RequireSession(sessions.Me))
	mux.HandleFunc("GET /v1/organizations", sessions.RequireSession(organizations.List))
	mux.HandleFunc("GET /v1/organizations/{login}/repositories", sessions.RequireSession(organizations.Repositories))
	mux.HandleFunc("POST /v1/organizations/{login}/sync", sessions.RequireSession(organizations.Sync))
	mux.HandleFunc("GET /v1/organizations/{login}/scores", sessions.RequireSession(scoring.Get))
	mux.HandleFunc("POST /v1/organizations/{login}/scores/query", sessions.RequireSession(scoring.Query))
	mux.HandleFunc("GET /v1/organizations/{login}/scores/presets", sessions.RequireSession(scoring.ListPresets))
	mux.HandleFunc("PUT /v1/organizations/{login}/scores/presets/{name}", sessions.RequireSession(scoring.SavePreset))
	mux.HandleFunc("DELETE /v1/organizations/{login}/scores/presets/{name}", sessions.RequireSession(scoring.DeletePreset))

	mux.HandleFunc("POST /webhooks/github", s.requireGitHub(webhooks.Handler()))

	var h http.Handler = mux
	h = middleware.CheckOrigin(s.Cfg.AppOrigin.Scheme+"://"+s.Cfg.AppOrigin.Host, h)
	h = middleware.AccessLog(s.Log, h)
	h = middleware.SecurityHeaders(h)
	h = middleware.Recoverer(s.Log, h)
	h = middleware.RequestID(h)
	return otelhttp.NewHandler(h, "http", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if r.Pattern != "" {
			return r.Pattern
		}
		return r.Method
	}))
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz requires PostgreSQL before Kubernetes routes traffic to the pod.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Pool.Ping(ctx); err != nil {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "unreachable"})
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "ok"})
}

func (s *Server) requireGitHub(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.GitHub == nil {
			response.Error(w, r, http.StatusServiceUnavailable, "github_not_configured", "the GitHub App is not configured on this deployment")
			return
		}
		next(w, r)
	}
}
