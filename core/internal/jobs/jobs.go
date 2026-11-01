// Package jobs uses River's transactional outbox for durable PostgreSQL-backed work.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
)

// NewInsertClient returns a River client that only inserts jobs (used by the API process).
func NewInsertClient(pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{})
}

// WorkerOptions configures the worker process.
type WorkerOptions struct {
	Concurrency    int
	ReconcileEvery time.Duration
	Logger         *slog.Logger
	Scorer         *scorer.Runner
}

// NewWorkerClient returns a River client that executes jobs.
func NewWorkerClient(pool *pgxpool.Pool, gh *github.Client, opts WorkerOptions) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &SyncWorker{Pool: pool, GitHub: gh, Now: time.Now})
	river.AddWorker(workers, &IngestWorker{Pool: pool, GitHub: gh, Now: time.Now})
	river.AddWorker(workers, &ScoreWorker{Pool: pool, Scorer: opts.Scorer})
	river.AddWorker(workers, &WebhookWorker{Pool: pool, GitHub: gh})
	river.AddWorker(workers, &ReconcileWorker{Pool: pool})
	river.AddWorker(workers, &PurgeSessionsWorker{Pool: pool, Now: time.Now, Log: opts.Logger})
	river.AddWorker(workers, &PurgeWebhookDeliveriesWorker{Pool: pool, Now: time.Now, Log: opts.Logger})
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:  opts.Logger,
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: opts.Concurrency},
			QueueGitHub:        {MaxWorkers: opts.Concurrency},
			QueueScoring:       {MaxWorkers: 1},
		},
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(opts.ReconcileEvery),
				func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(purgeInterval),
				func() (river.JobArgs, *river.InsertOpts) { return PurgeSessionsArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(purgeInterval),
				func() (river.JobArgs, *river.InsertOpts) { return PurgeWebhookDeliveriesArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true}),
		},
	})
}

// insertClient reuses the worker's River client and builds an insert-only client outside River.
func insertClient(ctx context.Context, pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	if client, err := river.ClientFromContextSafely[pgx.Tx](ctx); err == nil {
		return client, nil
	}
	return NewInsertClient(pool)
}
