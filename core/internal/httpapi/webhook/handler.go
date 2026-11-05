package webhook

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

type Handler struct {
	Pool *pgxpool.Pool
	Jobs *river.Client[pgx.Tx]
	Cfg  *config.Config
	Log  *slog.Logger

	bodyTimeout time.Duration // zero means webhookBodyTimeout; tests shorten it
}
