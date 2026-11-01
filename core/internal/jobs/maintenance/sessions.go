package maintenance

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/maintenance"
)

// PurgeSessionsWorker deletes expired sessions.
type PurgeSessionsWorker struct {
	river.WorkerDefaults[queue.PurgeSessions]
	Pool *pgxpool.Pool
	Now  func() time.Time
	Log  *slog.Logger
}

func (w *PurgeSessionsWorker) Work(ctx context.Context, _ *river.Job[queue.PurgeSessions]) error {
	n, err := maintenance.PurgeExpiredSessions(ctx, w.Pool, w.Now())
	w.Log.InfoContext(ctx, "purged expired sessions", "rows", n)
	return err
}
