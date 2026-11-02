package organization

import (
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Handler struct {
	Pool *pgxpool.Pool
	Jobs *river.Client[pgx.Tx]
	Log  *slog.Logger
}
