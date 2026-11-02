// Package jobs uses River's transactional outbox for durable PostgreSQL-backed work.
package jobs

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	adminjobs "github.com/upayanmazumder/gitorgasm/core/internal/jobs/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/ingest"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/maintenance"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/scoring"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/sync"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/webhook"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

// WorkerOptions configures the worker process.
type WorkerOptions struct {
	Concurrency    int
	ReconcileEvery time.Duration
	Logger         *slog.Logger
	Scorer         *scorer.Runner
	Sealer         *secure.Sealer
}

// NewWorkerClient returns a River client that executes jobs.
func NewWorkerClient(pool *pgxpool.Pool, gh *github.Client, opts WorkerOptions) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &sync.Worker{Pool: pool, GitHub: gh, Now: time.Now})
	river.AddWorker(workers, &ingest.Worker{Pool: pool, GitHub: gh, Now: time.Now})
	river.AddWorker(workers, &scoring.Worker{Pool: pool, Scorer: opts.Scorer})
	river.AddWorker(workers, &adminjobs.Worker{Pool: pool, GitHub: gh, Sealer: opts.Sealer, Now: time.Now, Log: opts.Logger})
	river.AddWorker(workers, &webhook.Worker{Pool: pool, GitHub: gh})
	river.AddWorker(workers, &maintenance.ReconcileWorker{Pool: pool})
	river.AddWorker(workers, &maintenance.PurgeSessionsWorker{Pool: pool, Now: time.Now, Log: opts.Logger})
	river.AddWorker(workers, &maintenance.PurgeWebhookDeliveriesWorker{Pool: pool, Now: time.Now, Log: opts.Logger})
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:  opts.Logger,
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault:   {MaxWorkers: opts.Concurrency},
			queue.GitHub:         {MaxWorkers: opts.Concurrency},
			queue.Scoring:        {MaxWorkers: 1},
			queue.Administration: {MaxWorkers: 1},
		},
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(opts.ReconcileEvery),
				func() (river.JobArgs, *river.InsertOpts) { return queue.Reconcile{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(24*time.Hour),
				func() (river.JobArgs, *river.InsertOpts) { return queue.PurgeSessions{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(24*time.Hour),
				func() (river.JobArgs, *river.InsertOpts) { return queue.PurgeWebhookDeliveries{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true}),
		},
	})
}
