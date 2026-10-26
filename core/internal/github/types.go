package github

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

// User is the subset of GET /user that Gitorgasm stores.
type User struct {
	ID        int64   `json:"id"`
	Login     string  `json:"login"`
	Name      *string `json:"name"`
	AvatarURL string  `json:"avatar_url"`
}

// UserToken omits RefreshToken and ExpiresIn when expiring user tokens are disabled.
type UserToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"-"`
}
