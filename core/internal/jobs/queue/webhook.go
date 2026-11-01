package queue

import "github.com/riverqueue/river"

// ProcessWebhook points at a stored webhook delivery.
type ProcessWebhook struct {
	DeliveryID string `json:"delivery_id"`
}

func (ProcessWebhook) Kind() string { return "process_webhook" }

func (ProcessWebhook) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}
