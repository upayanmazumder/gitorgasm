package models

import "time"

// Contribution records an attributed event without storing GitHub content.
type Contribution struct {
	EventID      string
	RepositoryID int64
	UserID       int64
	Login        string
	Kind         string
	OccurredAt   time.Time
	IsBot        bool
}

// ContributionWindow gives commits a longer lookback because pushes can carry old committer dates.
type ContributionWindow struct {
	CommitsSince  time.Time
	ActivitySince time.Time
	Until         time.Time
}

// Contribution phases in the order a repository pass runs them.
const (
	PhaseCommits = "commits"
	PhasePulls   = "pulls"
	PhaseIssues  = "issues"
	PhaseDone    = "done"
)

// ContributionCursor resumes a phase at Next; an empty Next starts its first page.
type ContributionCursor struct {
	Phase string
	Next  string
}
