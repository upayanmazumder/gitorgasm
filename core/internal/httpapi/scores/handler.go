package scores

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
)

type Handler struct {
	Pool   *pgxpool.Pool
	Scorer *scorer.Runner
	Log    *slog.Logger
	Now    func() time.Time
}
