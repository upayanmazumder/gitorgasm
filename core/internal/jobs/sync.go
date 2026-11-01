package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// SyncWorker mirrors an installation and its repositories from GitHub.
type SyncWorker struct {
	river.WorkerDefaults[SyncInstallationArgs]
	Pool   *pgxpool.Pool
	GitHub *github.Client
	Now    func() time.Time
}

func (w *SyncWorker) Timeout(*river.Job[SyncInstallationArgs]) time.Duration { return 15 * time.Minute }

// syncLockKey namespaces per-installation advisory locks away from goose and River.
const syncLockKey = `hashtextextended('gitorgasm:sync_installation:' || $1::bigint::text, 0)`

func (w *SyncWorker) Work(ctx context.Context, job *river.Job[SyncInstallationArgs]) error {
	// One connection holds the session lock for the full pass to prevent out-of-order tombstones.
	return withSessionLock(ctx, w.Pool, syncLockKey, job.Args.InstallationID, func(conn *pgxpool.Conn) error {
		return w.sync(ctx, conn, job.Args.InstallationID)
	})
}

func (w *SyncWorker) sync(ctx context.Context, conn *pgxpool.Conn, id int64) error {
	inst, err := w.GitHub.GetInstallation(ctx, id)
	if github.IsKind(err, github.KindNotFound) {
		w.GitHub.ForgetInstallation(id)
		return tenancy.MarkInstallationDeleted(ctx, conn, id)
	}
	if err != nil {
		return retryDecision(err)
	}
	if err := tenancy.UpsertInstallation(ctx, conn, inst); err != nil {
		return err
	}
	if inst.SuspendedAt != nil {
		return nil // GitHub refuses installation tokens for suspended installations.
	}

	started := w.Now().UTC()
	if err := tenancy.SyncStarted(ctx, conn, id, started); err != nil {
		return err
	}
	seen := 0
	err = w.GitHub.ListInstallationRepositories(ctx, id, func(page []models.Repository) error {
		seen += len(page)
		return tenancy.UpsertRepositories(ctx, conn, id, page, started)
	})
	if err != nil {
		if ferr := tenancy.SyncFinished(ctx, conn, id, seen, err.Error()); ferr != nil {
			return errors.Join(err, ferr)
		}
		return retryDecision(err)
	}
	client, err := insertClient(ctx, w.Pool)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tenancy.TombstoneUnseenRepositories(ctx, tx, id, started); err != nil {
			return err
		}
		if err := tenancy.SyncFinished(ctx, tx, id, seen, ""); err != nil {
			return err
		}
		_, err := client.InsertTx(ctx, tx, IngestContributionsArgs{InstallationID: id}, nil)
		return err
	})
}
