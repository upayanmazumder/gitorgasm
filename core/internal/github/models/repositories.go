package models

import "time"

// Owner is the repository owner (simple-user).
type Owner struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// Repository is the subset of components/schemas/repository that Gitorgasm stores.
type Repository struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	FullName        string     `json:"full_name"`
	Owner           Owner      `json:"owner"`
	Description     *string    `json:"description"`
	Private         bool       `json:"private"`
	Fork            bool       `json:"fork"`
	Archived        bool       `json:"archived"`
	Visibility      string     `json:"visibility"`
	DefaultBranch   string     `json:"default_branch"`
	Language        *string    `json:"language"`
	StargazersCount int        `json:"stargazers_count"`
	PushedAt        *time.Time `json:"pushed_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
}
