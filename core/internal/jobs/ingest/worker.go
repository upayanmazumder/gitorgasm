package ingest

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/internal/execution"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/contributions"
)

type Worker struct {
	river.WorkerDefaults[queue.IngestContributions]
	Pool   *pgxpool.Pool
	GitHub *github.Client
	Now    func() time.Time
}

// Ingestion attempts snooze before River's rescue deadline and resume from stored cursors.
const (
	ingestTimeout = 50 * time.Minute
	ingestBudget  = 40 * time.Minute
)

func (w *Worker) Timeout(*river.Job[queue.IngestContributions]) time.Duration {
	return ingestTimeout
}

const ingestLockKey = `hashtextextended('gitorgasm:ingest_contributions:' || $1::bigint::text, 0)`

func (w *Worker) Work(ctx context.Context, job *river.Job[queue.IngestContributions]) error {
	id := job.Args.InstallationID
	return execution.WithSessionLock(ctx, w.Pool, ingestLockKey, id, func(conn *pgxpool.Conn) error {
		err := w.ingest(ctx, conn, id)
		var snooze *river.JobSnoozeError
		if err == nil || errors.As(err, &snooze) || ctx.Err() != nil {
			return err
		}
		if ferr := contributions.RecordIngestionFailure(context.WithoutCancel(ctx), conn, id, err.Error()); ferr != nil {
			return errors.Join(err, ferr)
		}
		return err
	})
}
