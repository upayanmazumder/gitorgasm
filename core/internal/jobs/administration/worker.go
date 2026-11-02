// Package administration executes reviewed organization changes without replaying external writes.
package administration

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/internal/execution"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	adminstore "github.com/upayanmazumder/gitorgasm/core/internal/store/administration"
)

// Worker revalidates the initiating session and live ownership for every target.
type Worker struct {
	river.WorkerDefaults[queue.AdministerOrganization]
	Pool          *pgxpool.Pool
	GitHub        *github.Client
	Sealer        *secure.Sealer
	Now           func() time.Time
	Log           *slog.Logger
	WriteInterval time.Duration
}

func (*Worker) Timeout(*river.Job[queue.AdministerOrganization]) time.Duration {
	return 15 * time.Minute
}

// A single key, not an installation key: writes are serialized across queues and replicas.
const administrationLockKey = `hashtextextended('gitorgasm:administration:' || $1::bigint::text, 0)`

var errExecution = errors.New("organization administration execution failed")

func (w *Worker) Work(ctx context.Context, job *river.Job[queue.AdministerOrganization]) error {
	err := execution.WithSessionLock(ctx, w.Pool, administrationLockKey, 0, func(conn *pgxpool.Conn) error {
		return w.execute(ctx, conn, job.Args.OperationID)
	})
	if err == nil {
		return nil
	}
	var snooze *river.JobSnoozeError
	if errors.As(err, &snooze) {
		return snooze // No durable claim and no external request has occurred.
	}
	// Neither PostgreSQL nor upstream errors (which may contain credentials) enter River logs.
	return errExecution
}

func (w *Worker) execute(ctx context.Context, conn *pgxpool.Conn, id string) (resultErr error) {
	op, err := adminstore.LoadExecution(ctx, conn, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil // A deleted/missing receipt cannot authorize any work.
	}
	if err != nil {
		return errExecution
	}
	if op.Status == "completed" || op.Status == "interrupted" {
		return nil
	}
	remainingCode := "interrupted"
	finalized := false
	// This also covers cancellation while a write is in flight and failed durable acknowledgments.
	// It intentionally does not reuse the cancelled job context or a possibly broken lock connection.
	defer func() {
		if finalized {
			return
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := adminstore.FinalizeOperation(cleanup, w.Pool, id, "interrupted", remainingCode); err != nil {
			resultErr = errExecution
		}
	}()
	claimed, err := adminstore.ClaimOperation(ctx, conn, id)
	if err != nil {
		return errExecution
	}
	if !claimed {
		// The exclusive lock excludes a live executor. A previous claim means interruption,
		// not permission to replay an item whose write may already have reached GitHub.
		return nil
	}
	authorizer := administration.Authorizer{Pool: w.Pool, GitHub: w.GitHub, Sealer: w.Sealer, Now: w.Now}
	interval := w.WriteInterval
	if interval <= 0 {
		interval = time.Second
	}
	for position, target := range op.Results {
		// Waiting before authorization avoids a stale owner check during the pacing delay.
		// Also wait for the first target, so consecutive batches remain paced across replicas.
		if err := wait(ctx, interval); err != nil {
			return nil
		}
		access, err := authorizer.Check(ctx, op.ActorUserID, op.SessionHash, op.Organization)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			remainingCode = accessFailureCode(err)
			return nil
		}
		if !access.CanManage {
			if access.Reason != nil {
				remainingCode = allowedAccessCode(*access.Reason)
			}
			return nil
		}
		// A reinstall/rename must not silently redirect the reviewed operation to another account.
		if access.InstallationID != op.InstallationID || access.AccountID != op.AccountID {
			remainingCode = "organization_not_found"
			return nil
		}
		outcome, err := w.target(ctx, conn, &op, access, position, target.Login)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return errExecution
		}
		if outcome.status != "" {
			if err := adminstore.FinishTarget(ctx, conn, id, position, outcome.status, outcome.code); err != nil {
				return errExecution
			}
		}
		if outcome.stop {
			remainingCode = outcome.code
			if outcome.remainingCode != "" {
				remainingCode = outcome.remainingCode
			}
			return nil
		}
	}
	if err := adminstore.FinalizeOperation(ctx, conn, id, "completed", ""); err != nil {
		return errExecution
	}
	finalized = true
	return nil
}

func wait(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func accessFailureCode(err error) string {
	var accessErr *administration.AccessError
	if errors.As(err, &accessErr) {
		return allowedAccessCode(accessErr.Code)
	}
	return "github_unavailable"
}

func allowedAccessCode(code string) string {
	switch code {
	case "session_expired", "organization_not_found", "github_reauthentication_required", "not_owner",
		"members_permission_required", "unsupported_account", "installation_suspended", "github_rate_limited", "github_unavailable", "github_forbidden":
		return code
	default:
		return "github_unavailable"
	}
}
