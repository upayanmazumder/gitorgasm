package httpapi

import (
	"net/http"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/auth"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/organization"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/scores"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/webhook"
)

func (s *Server) routes() *http.ServeMux {
	sessions := &auth.Handler{
		Pool: s.Pool, Jobs: s.Jobs, GitHub: s.GitHub, Sealer: s.Sealer,
		Cfg: s.Cfg, Log: s.Log, Now: s.Now,
	}
	organizations := &organization.Handler{Pool: s.Pool, Jobs: s.Jobs, Log: s.Log}
	scoring := &scores.Handler{Pool: s.Pool, Scorer: s.Scorer, Log: s.Log, Now: s.Now}
	webhooks := &webhook.Handler{Pool: s.Pool, Jobs: s.Jobs, Cfg: s.Cfg, Log: s.Log}
	administration := &administration.Handler{
		Pool: s.Pool, Jobs: s.Jobs, GitHub: s.GitHub, Sealer: s.Sealer, Log: s.Log, Now: s.Now,
	}

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
	mux.HandleFunc("GET /v1/organizations/{login}/administration", sessions.RequireSession(administration.Capabilities))
	mux.HandleFunc("GET /v1/organizations/{login}/administration/users", sessions.RequireSession(administration.SearchUsers))
	mux.HandleFunc("GET /v1/organizations/{login}/administration/teams", sessions.RequireSession(administration.Teams))
	mux.HandleFunc("GET /v1/organizations/{login}/administration/members", sessions.RequireSession(administration.Members))
	mux.HandleFunc("POST /v1/organizations/{login}/administration/operations", sessions.RequireSession(administration.Submit))
	mux.HandleFunc("GET /v1/organizations/{login}/administration/operations", sessions.RequireSession(administration.ListOperations))
	mux.HandleFunc("GET /v1/organizations/{login}/administration/operations/{id}", sessions.RequireSession(administration.GetOperation))

	mux.HandleFunc("POST /webhooks/github", s.requireGitHub(webhooks.Handler()))
	return mux
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
