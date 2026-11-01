package queue

import "github.com/riverqueue/river"

type ScoreOrganization struct {
	InstallationID int64 `json:"installation_id"`
	Generation     int64 `json:"generation"`
}

func (ScoreOrganization) Kind() string { return "score_organization" }
func (ScoreOrganization) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Scoring, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}
