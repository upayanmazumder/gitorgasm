package webhook

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/webhooks"
)

// Worker applies a stored webhook delivery.
type Worker struct {
	river.WorkerDefaults[queue.ProcessWebhook]
	Pool   *pgxpool.Pool
	GitHub *github.Client
}

func (w *Worker) Work(ctx context.Context, job *river.Job[queue.ProcessWebhook]) error {
	d, err := webhooks.GetWebhookDelivery(ctx, w.Pool, job.Args.DeliveryID)
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
		_ = webhooks.FinishWebhookDelivery(ctx, w.Pool, d.DeliveryID, "invalid payload: "+err.Error())
		return river.JobCancel(err)
	}
	if err := w.apply(ctx, d.Event, &p); err != nil {
		_ = webhooks.FinishWebhookDelivery(ctx, w.Pool, d.DeliveryID, err.Error())
		return err
	}
	return webhooks.FinishWebhookDelivery(ctx, w.Pool, d.DeliveryID, "")
}
