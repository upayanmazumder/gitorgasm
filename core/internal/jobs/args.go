package jobs

import (
	"time"

	"github.com/riverqueue/river"
)

// QueueGitHub limits GitHub concurrency per worker replica.
const QueueGitHub = "github"

const QueueScoring = "scoring"

// SyncInstallationArgs requests a full repository sync of one installation.
type SyncInstallationArgs struct {
	InstallationID int64 `json:"installation_id"`
}

func (SyncInstallationArgs) Kind() string { return "sync_installation" }

// InsertOpts opens a new deduplication window each minute so running syncs cannot drop follow-up requests.
func (SyncInstallationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueGitHub, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: time.Minute}}
}

// ProcessWebhookArgs points at a stored webhook delivery.
type ProcessWebhookArgs struct {
	DeliveryID string `json:"delivery_id"`
}

func (ProcessWebhookArgs) Kind() string { return "process_webhook" }

func (ProcessWebhookArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ReconcileArgs periodically repairs installations missed by webhooks.
type ReconcileArgs struct{}

func (ReconcileArgs) Kind() string { return "reconcile_installations" }

// PurgeSessionsArgs deletes expired sessions and the sealed GitHub tokens they hold.
type PurgeSessionsArgs struct{}

func (PurgeSessionsArgs) Kind() string { return "purge_sessions" }

// PurgeWebhookDeliveriesArgs deletes stored webhook payloads after their retention window.
type PurgeWebhookDeliveriesArgs struct{}

func (PurgeWebhookDeliveriesArgs) Kind() string { return "purge_webhook_deliveries" }

type IngestContributionsArgs struct {
	InstallationID int64 `json:"installation_id"`
}

func (IngestContributionsArgs) Kind() string { return "ingest_contributions" }
func (IngestContributionsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueGitHub, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: time.Minute}}
}

type ScoreOrganizationArgs struct {
	InstallationID int64 `json:"installation_id"`
	Generation     int64 `json:"generation"`
}

func (ScoreOrganizationArgs) Kind() string { return "score_organization" }
func (ScoreOrganizationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueScoring, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}
