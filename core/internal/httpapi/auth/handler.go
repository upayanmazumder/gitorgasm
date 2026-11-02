package auth

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

type Handler struct {
	Pool   *pgxpool.Pool
	Jobs   *river.Client[pgx.Tx]
	GitHub *github.Client
	Sealer *secure.Sealer
	Cfg    *config.Config
	Log    *slog.Logger
	Now    func() time.Time
}
