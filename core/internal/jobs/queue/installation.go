package queue

import (
	"time"

	"github.com/riverqueue/river"
)

// SyncInstallation requests a full repository sync of one installation.
type SyncInstallation struct {
	InstallationID int64 `json:"installation_id"`
}

func (SyncInstallation) Kind() string { return "sync_installation" }

// InsertOpts opens a new deduplication window each minute so running syncs cannot drop follow-up requests.
func (SyncInstallation) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: GitHub, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: time.Minute}}
}

type IngestContributions struct {
	InstallationID int64 `json:"installation_id"`
}

func (IngestContributions) Kind() string { return "ingest_contributions" }
func (IngestContributions) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: GitHub, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: time.Minute}}
}
