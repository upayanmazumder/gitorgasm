package webhooks

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// WebhookDelivery is a stored webhook.
type WebhookDelivery struct {
	DeliveryID     string
	Event          string
	Action         *string
	InstallationID *int64
	Payload        []byte
	ProcessedAt    *time.Time
}

// InsertWebhookDelivery returns false for an already-stored delivery GUID.
func InsertWebhookDelivery(ctx context.Context, q store.DBTX, d WebhookDelivery) (bool, error) {
	tag, err := q.Exec(ctx, `
		INSERT INTO webhook_deliveries (delivery_id, event, action, installation_id, payload)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT (delivery_id) DO NOTHING`,
		d.DeliveryID, d.Event, d.Action, d.InstallationID, d.Payload)
	return tag.RowsAffected() == 1, err
}

// GetWebhookDelivery loads a delivery.
func GetWebhookDelivery(ctx context.Context, q store.DBTX, id string) (*WebhookDelivery, error) {
	var d WebhookDelivery
	err := q.QueryRow(ctx, `
		SELECT delivery_id, event, action, installation_id, payload, processed_at
		  FROM webhook_deliveries WHERE delivery_id = $1`, id).
		Scan(&d.DeliveryID, &d.Event, &d.Action, &d.InstallationID, &d.Payload, &d.ProcessedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	return &d, err
}

// FinishWebhookDelivery marks a delivery processed, or records the failure.
func FinishWebhookDelivery(ctx context.Context, q store.DBTX, id string, errText string) error {
	_, err := q.Exec(ctx, `
		UPDATE webhook_deliveries
		   SET processed_at = CASE WHEN $2 = '' THEN now() ELSE processed_at END, last_error = left(NULLIF($2, ''), $3)
		 WHERE delivery_id = $1`, id, errText, store.MaxErrorText)
	return err
}
