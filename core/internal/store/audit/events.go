package audit

import (
	"context"
	"encoding/json"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// AuditEvent is one security-relevant action.
type AuditEvent struct {
	ActorUserID *int64
	Action      string
	Target      string
	RequestID   string
	Metadata    map[string]any
}

// Audit appends an audit event.
func Audit(ctx context.Context, q store.DBTX, e AuditEvent) error {
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return err
	}
	if e.Metadata == nil {
		meta = []byte("{}")
	}
	_, err = q.Exec(ctx, `
		INSERT INTO audit_events (actor_user_id, action, target, request_id, metadata)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)`, e.ActorUserID, e.Action, e.Target, e.RequestID, meta)
	return err
}
