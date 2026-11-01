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

// Processed webhook GUIDs remain for 14 days to deduplicate redeliveries.
const (
	webhookProcessedRetention   = 14 * 24 * time.Hour
	webhookUnprocessedRetention = 30 * 24 * time.Hour
)

// PurgeWebhookDeliveriesWorker deletes webhook payloads after their retention window.
type PurgeWebhookDeliveriesWorker struct {
	river.WorkerDefaults[queue.PurgeWebhookDeliveries]
	Pool *pgxpool.Pool
	Now  func() time.Time
	Log  *slog.Logger
}

func (w *PurgeWebhookDeliveriesWorker) Work(ctx context.Context, _ *river.Job[queue.PurgeWebhookDeliveries]) error {
	now := w.Now()
	n, err := maintenance.PurgeWebhookDeliveries(ctx, w.Pool, now.Add(-webhookProcessedRetention), now.Add(-webhookUnprocessedRetention))
	w.Log.InfoContext(ctx, "purged webhook deliveries", "rows", n)
	return err
}
