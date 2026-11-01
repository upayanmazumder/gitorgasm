package models

import "time"

// Account represents an installation account, including enterprises with a slug instead of a login.
type Account struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Slug      string `json:"slug"`
	Type      string `json:"type"`
	AvatarURL string `json:"avatar_url"`
}

// Name returns the account login, or the enterprise slug.
func (a Account) Name() string {
	if a.Login != "" {
		return a.Login
	}
	return a.Slug
}

// Kind returns Organization, User, or Enterprise.
func (a Account) Kind() string {
	if a.Type != "" {
		return a.Type
	}
	return "Enterprise"
}

// Installation is the subset of components/schemas/installation that Gitorgasm stores.
type Installation struct {
	ID                  int64             `json:"id"`
	Account             *Account          `json:"account"`
	RepositorySelection string            `json:"repository_selection"`
	Permissions         map[string]string `json:"permissions"`
	SuspendedAt         *time.Time        `json:"suspended_at"`
}
