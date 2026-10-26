// Package jobs defines durable background jobs. River stores jobs in PostgreSQL, so a job is
// committed atomically with the data change that caused it (transactional outbox).
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// QueueGitHub serializes GitHub-bound work. Its worker count is the global GitHub
// concurrency limit for one worker replica.
const QueueGitHub = "github"

// SyncInstallationArgs requests a full repository sync of one installation.
type SyncInstallationArgs struct {
	InstallationID int64 `json:"installation_id"`
}

func (SyncInstallationArgs) Kind() string { return "sync_installation" }

// InsertOpts collapses sync requests for the same installation within one minute.
// River's default unique states include `running` and `completed`, so ByArgs alone would
// drop every new request until the completed job is cleaned (24 h). ByPeriod opens a new
// unique window each minute, so a webhook that arrives during a running sync still queues
// a follow-up pass. SyncWorker serializes passes per installation with an advisory lock.
func (SyncInstallationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueGitHub, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: time.Minute}}
}

// ProcessWebhookArgs points at a stored webhook delivery.
type ProcessWebhookArgs struct {
	DeliveryID string `json:"delivery_id"`
}

func (ProcessWebhookArgs) Kind() string { return "process_webhook" }

func (ProcessWebhookArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ReconcileArgs triggers a sync of every active installation. Webhooks can be lost, so
// periodic reconciliation is the correctness backstop.
type ReconcileArgs struct{}

func (ReconcileArgs) Kind() string { return "reconcile_installations" }

// NewInsertClient returns a River client that only inserts jobs (used by the API process).
func NewInsertClient(pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{})
}

// WorkerOptions configures the worker process.
type WorkerOptions struct {
	Concurrency    int
	ReconcileEvery time.Duration
	Logger         *slog.Logger
}

// NewWorkerClient returns a River client that executes jobs.
func NewWorkerClient(pool *pgxpool.Pool, gh *github.Client, opts WorkerOptions) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &SyncWorker{Pool: pool, GitHub: gh, Now: time.Now})
	river.AddWorker(workers, &WebhookWorker{Pool: pool, GitHub: gh})
	river.AddWorker(workers, &ReconcileWorker{Pool: pool})
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:  opts.Logger,
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: opts.Concurrency},
			QueueGitHub:        {MaxWorkers: opts.Concurrency},
		},
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(opts.ReconcileEvery),
				func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true}),
		},
	})
}

// SyncWorker mirrors an installation and its repositories from GitHub.
type SyncWorker struct {
	river.WorkerDefaults[SyncInstallationArgs]
	Pool   *pgxpool.Pool
	GitHub *github.Client
	Now    func() time.Time
}

func (w *SyncWorker) Timeout(*river.Job[SyncInstallationArgs]) time.Duration { return 15 * time.Minute }

// syncLockKey derives a 64-bit advisory lock key per installation. Hashing a namespaced
// string keeps the key apart from goose and River locks and avoids truncating the id.
const syncLockKey = `hashtextextended('gitorgasm:sync_installation:' || $1::bigint::text, 0)`

func (w *SyncWorker) Work(ctx context.Context, job *river.Job[SyncInstallationArgs]) error {
	id := job.Args.InstallationID
	// Two overlapping passes could tombstone or resurrect rows out of order. Hold a
	// session advisory lock for the whole pass and retry later when another pass runs.
	conn, err := w.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(`+syncLockKey+`)`, id).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return river.JobSnooze(30 * time.Second)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(`+syncLockKey+`)`, id)
	}()

	inst, err := w.GitHub.GetInstallation(ctx, id)
	if github.IsKind(err, github.KindNotFound) {
		// The App was uninstalled and the webhook was missed.
		w.GitHub.ForgetInstallation(id)
		return store.MarkInstallationDeleted(ctx, w.Pool, id)
	}
	if err != nil {
		return retryDecision(err)
	}
	if err := store.UpsertInstallation(ctx, w.Pool, inst); err != nil {
		return err
	}
	if inst.SuspendedAt != nil {
		return nil // GitHub refuses installation tokens for suspended installations.
	}

	started := w.Now().UTC()
	if err := store.SyncStarted(ctx, w.Pool, id, started); err != nil {
		return err
	}
	seen := 0
	err = w.GitHub.ListInstallationRepositories(ctx, id, func(page []github.Repository) error {
		seen += len(page)
		return store.UpsertRepositories(ctx, w.Pool, id, page, started)
	})
	if err != nil {
		if ferr := store.SyncFinished(ctx, w.Pool, id, seen, err.Error()); ferr != nil {
			return errors.Join(err, ferr)
		}
		return retryDecision(err)
	}
	// Every page succeeded, so repositories not seen in this pass are gone or no longer granted.
	if _, err := store.TombstoneUnseenRepositories(ctx, w.Pool, id, started); err != nil {
		return err
	}
	return store.SyncFinished(ctx, w.Pool, id, seen, "")
}

// retryDecision maps GitHub errors to River outcomes: wait out rate limits, stop on
// permission errors, and let River back off on everything else.
func retryDecision(err error) error {
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.Kind {
	case github.KindRateLimited:
		return river.JobSnooze(apiErr.RetryAfter)
	case github.KindUnauthorized, github.KindForbidden, github.KindInvalid:
		return river.JobCancel(err)
	default:
		return err
	}
}

// ReconcileWorker enqueues a sync for every active installation.
type ReconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	Pool *pgxpool.Pool
}

func (w *ReconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	ids, err := store.ActiveInstallationIDs(ctx, w.Pool)
	if err != nil {
		return err
	}
	client := river.ClientFromContext[pgx.Tx](ctx)
	params := make([]river.InsertManyParams, 0, len(ids))
	for _, id := range ids {
		params = append(params, river.InsertManyParams{Args: SyncInstallationArgs{InstallationID: id}})
	}
	if len(params) == 0 {
		return nil
	}
	_, err = client.InsertMany(ctx, params)
	return err
}

// webhookPayload holds the fields Gitorgasm reads from supported events.
type webhookPayload struct {
	Action       string               `json:"action"`
	Installation *github.Installation `json:"installation"`
	Sender       *struct {
		ID int64 `json:"id"`
	} `json:"sender"`
}

// WebhookWorker applies a stored webhook delivery.
type WebhookWorker struct {
	river.WorkerDefaults[ProcessWebhookArgs]
	Pool   *pgxpool.Pool
	GitHub *github.Client
}

func (w *WebhookWorker) Work(ctx context.Context, job *river.Job[ProcessWebhookArgs]) error {
	d, err := store.GetWebhookDelivery(ctx, w.Pool, job.Args.DeliveryID)
	if errors.Is(err, store.ErrNotFound) {
		return river.JobCancel(err)
	}
	if err != nil {
		return err
	}
	if d.ProcessedAt != nil {
		return nil
	}
	var p webhookPayload
	if err := json.Unmarshal(d.Payload, &p); err != nil {
		_ = store.FinishWebhookDelivery(ctx, w.Pool, d.DeliveryID, "invalid payload: "+err.Error())
		return river.JobCancel(err)
	}
	if err := w.apply(ctx, d.Event, &p); err != nil {
		_ = store.FinishWebhookDelivery(ctx, w.Pool, d.DeliveryID, err.Error())
		return err
	}
	return store.FinishWebhookDelivery(ctx, w.Pool, d.DeliveryID, "")
}

// apply handles one event inside a transaction so state changes and follow-up jobs commit
// together. Webhooks are hints: the sync job re-reads GitHub, so duplicates and reordering
// converge to the same state.
func (w *WebhookWorker) apply(ctx context.Context, event string, p *webhookPayload) error {
	return pgx.BeginFunc(ctx, w.Pool, func(tx pgx.Tx) error {
		client := river.ClientFromContext[pgx.Tx](ctx)
		enqueueSync := func(id int64) error {
			_, err := client.InsertTx(ctx, tx, SyncInstallationArgs{InstallationID: id}, nil)
			return err
		}
		switch event {
		case "installation":
			if p.Installation == nil {
				return errors.New("installation event without installation")
			}
			if p.Action == "deleted" {
				w.GitHub.ForgetInstallation(p.Installation.ID)
				return store.MarkInstallationDeleted(ctx, tx, p.Installation.ID)
			}
			if err := store.UpsertInstallation(ctx, tx, p.Installation); err != nil {
				return err
			}
			return enqueueSync(p.Installation.ID)
		case "installation_repositories", "repository":
			if p.Installation == nil {
				return nil
			}
			return enqueueSync(p.Installation.ID)
		case "github_app_authorization":
			if p.Action == "revoked" && p.Sender != nil {
				_, err := store.RevokeGitHubUser(ctx, tx, p.Sender.ID)
				if err == nil {
					err = store.Audit(ctx, tx, store.AuditEvent{Action: "session.revoked_by_github", Target: fmt.Sprintf("github_user:%d", p.Sender.ID)})
				}
				return err
			}
			return nil
		default:
			return nil // ping and unsubscribed events are acknowledged without action.
		}
	})
}
