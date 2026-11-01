package maintenance

import (
	"context"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// purgeBatch caps the rows one DELETE removes, so a large backlog never holds long locks.
const purgeBatch = 10_000

// purge commits each batch separately so cancellation retains completed deletions.
func purge(ctx context.Context, q store.DBTX, sql string, args ...any) (int64, error) {
	var total int64
	for {
		tag, err := q.Exec(ctx, sql, args...)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < purgeBatch {
			return total, nil
		}
	}
}

// PurgeExpiredSessions deletes sessions that expired before now and returns the row count.
func PurgeExpiredSessions(ctx context.Context, q store.DBTX, now time.Time) (int64, error) {
	return purge(ctx, q, `
		DELETE FROM sessions WHERE id_hash IN (
		  SELECT id_hash FROM sessions WHERE expires_at < $1 LIMIT $2)`, now, purgeBatch)
}

// PurgeWebhookDeliveries returns the count of old processed and unprocessed deliveries removed.
func PurgeWebhookDeliveries(ctx context.Context, q store.DBTX, processedBefore, unprocessedBefore time.Time) (int64, error) {
	return purge(ctx, q, `
		DELETE FROM webhook_deliveries WHERE delivery_id IN (
		  SELECT delivery_id FROM webhook_deliveries
		   WHERE received_at < $1 AND (processed_at IS NOT NULL OR received_at < $2)
		   LIMIT $3)`, processedBefore, unprocessedBefore, purgeBatch)
}
