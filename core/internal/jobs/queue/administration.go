package queue

import "github.com/riverqueue/river"

// Administration is deliberately separate from read-only ingestion work.
const Administration = "administration"

// AdministerOrganization references durable state; credentials never enter River arguments.
type AdministerOrganization struct {
	OperationID string `json:"operation_id"`
}

func (AdministerOrganization) Kind() string { return "administer_organization" }

// A discarded or ambiguous write must be reviewed, never automatically replayed.
func (AdministerOrganization) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Administration, MaxAttempts: 1}
}
