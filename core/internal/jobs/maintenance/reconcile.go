package maintenance

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// ReconcileWorker enqueues a sync for every active installation.
type ReconcileWorker struct {
	river.WorkerDefaults[queue.Reconcile]
	Pool *pgxpool.Pool
}

func (w *ReconcileWorker) Work(ctx context.Context, _ *river.Job[queue.Reconcile]) error {
	ids, err := tenancy.ActiveInstallationIDs(ctx, w.Pool)
	if err != nil {
		return err
	}
	client := river.ClientFromContext[pgx.Tx](ctx)
	params := make([]river.InsertManyParams, 0, len(ids))
	for _, id := range ids {
		params = append(params, river.InsertManyParams{Args: queue.SyncInstallation{InstallationID: id}})
	}
	if len(params) == 0 {
		return nil
	}
	_, err = client.InsertMany(ctx, params)
	return err
}
