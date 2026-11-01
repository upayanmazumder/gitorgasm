package models

import (
	"encoding/json"
	"time"
)

type ContributionUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type ContributionCommit struct {
	SHA    string            `json:"sha"`
	Author *ContributionUser `json:"author"`
	Commit struct {
		Committer *struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type ContributionPull struct {
	ID        int64             `json:"id"`
	Number    int64             `json:"number"`
	User      *ContributionUser `json:"user"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	MergedAt  *time.Time        `json:"merged_at"`
}

type ContributionReview struct {
	ID          int64             `json:"id"`
	User        *ContributionUser `json:"user"`
	SubmittedAt *time.Time        `json:"submitted_at"`
}

type ContributionIssue struct {
	ID          int64             `json:"id"`
	Number      int64             `json:"number"`
	User        *ContributionUser `json:"user"`
	PullRequest *json.RawMessage  `json:"pull_request"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ContributionIssueEvent struct {
	ID        int64             `json:"id"`
	Event     string            `json:"event"`
	Actor     *ContributionUser `json:"actor"`
	CreatedAt time.Time         `json:"created_at"`
}
