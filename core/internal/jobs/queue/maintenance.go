package queue

// Reconcile periodically repairs installations missed by webhooks.
type Reconcile struct{}

func (Reconcile) Kind() string { return "reconcile_installations" }

// PurgeSessions deletes expired sessions and the sealed GitHub tokens they hold.
type PurgeSessions struct{}

func (PurgeSessions) Kind() string { return "purge_sessions" }

// PurgeWebhookDeliveries deletes stored webhook payloads after their retention window.
type PurgeWebhookDeliveries struct{}

func (PurgeWebhookDeliveries) Kind() string { return "purge_webhook_deliveries" }
