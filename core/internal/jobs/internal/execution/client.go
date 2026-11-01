package execution

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
)

// InsertClient reuses the worker's River client and builds an insert-only client outside River.
func InsertClient(ctx context.Context, pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	if client, err := river.ClientFromContextSafely[pgx.Tx](ctx); err == nil {
		return client, nil
	}
	return queue.NewInsertClient(pool)
}
