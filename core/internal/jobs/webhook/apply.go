package webhook

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/audit"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// webhookPayload holds the fields Gitorgasm reads from supported events.
type webhookPayload struct {
	Action       string               `json:"action"`
	Installation *models.Installation `json:"installation"`
	Sender       *struct {
		ID int64 `json:"id"`
	} `json:"sender"`
}

// apply commits state and follow-up jobs together; subsequent syncs resolve duplicate or reordered webhooks.
func (w *Worker) apply(ctx context.Context, event string, p *webhookPayload) error {
	return pgx.BeginFunc(ctx, w.Pool, func(tx pgx.Tx) error {
		client := river.ClientFromContext[pgx.Tx](ctx)
		enqueueSync := func(id int64) error {
			_, err := client.InsertTx(ctx, tx, queue.SyncInstallation{InstallationID: id}, nil)
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
