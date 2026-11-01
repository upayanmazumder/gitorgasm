package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/audit"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/webhooks"
)

// webhookPayload holds the fields Gitorgasm reads from supported events.
type webhookPayload struct {
	Action       string               `json:"action"`
	Installation *models.Installation `json:"installation"`
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

// apply commits state and follow-up jobs together; subsequent syncs resolve duplicate or reordered webhooks.
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
				return tenancy.MarkInstallationDeleted(ctx, tx, p.Installation.ID)
			}
			if err := tenancy.UpsertInstallation(ctx, tx, p.Installation); err != nil {
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
				_, err := identity.RevokeGitHubUser(ctx, tx, p.Sender.ID)
				if err == nil {
					err = audit.Audit(ctx, tx, audit.AuditEvent{Action: "session.revoked_by_github", Target: fmt.Sprintf("github_user:%d", p.Sender.ID)})
				}
				return err
			}
			return nil
		default:
			return nil // ping and unsubscribed events are acknowledged without action.
		}
	})
}
