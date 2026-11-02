// Package httpapi implements the public HTTP API (contract: contracts/openapi/v1.yaml).
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/observability"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/security"
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
	var h http.Handler = s.routes()
	h = security.CheckOrigin(s.Cfg.AppOrigin.Scheme+"://"+s.Cfg.AppOrigin.Host, h)
	h = observability.AccessLog(s.Log, h)
	h = security.SecurityHeaders(h)
	h = observability.Recoverer(s.Log, h)
	h = observability.RequestID(h)
	return otelhttp.NewHandler(h, "http", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if r.Pattern != "" {
			return r.Pattern
		}
		return r.Method
	}))
}
