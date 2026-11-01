package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/store/maintenance"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// Processed webhook GUIDs remain for 14 days to deduplicate redeliveries.
const (
	purgeInterval               = 24 * time.Hour
	webhookProcessedRetention   = 14 * 24 * time.Hour
	webhookUnprocessedRetention = 30 * 24 * time.Hour
)

// ReconcileWorker enqueues a sync for every active installation.
type ReconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	Pool *pgxpool.Pool
}

func (w *ReconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	ids, err := tenancy.ActiveInstallationIDs(ctx, w.Pool)
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

// PurgeSessionsWorker deletes expired sessions.
type PurgeSessionsWorker struct {
	river.WorkerDefaults[PurgeSessionsArgs]
	Pool *pgxpool.Pool
	Now  func() time.Time
	Log  *slog.Logger
}

func (w *PurgeSessionsWorker) Work(ctx context.Context, _ *river.Job[PurgeSessionsArgs]) error {
	n, err := maintenance.PurgeExpiredSessions(ctx, w.Pool, w.Now())
	w.Log.InfoContext(ctx, "purged expired sessions", "rows", n)
	return err
}

// PurgeWebhookDeliveriesWorker deletes webhook payloads after their retention window.
type PurgeWebhookDeliveriesWorker struct {
	river.WorkerDefaults[PurgeWebhookDeliveriesArgs]
	Pool *pgxpool.Pool
	Now  func() time.Time
	Log  *slog.Logger
}

func (w *PurgeWebhookDeliveriesWorker) Work(ctx context.Context, _ *river.Job[PurgeWebhookDeliveriesArgs]) error {
	now := w.Now()
	n, err := maintenance.PurgeWebhookDeliveries(ctx, w.Pool, now.Add(-webhookProcessedRetention), now.Add(-webhookUnprocessedRetention))
	w.Log.InfoContext(ctx, "purged webhook deliveries", "rows", n)
	return err
}
