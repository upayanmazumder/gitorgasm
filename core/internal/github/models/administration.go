package models

// Membership is the shared state/role subset of organization and team memberships.
// Organization and User are present on organization membership responses only.
type Membership struct {
	State        string   `json:"state"`
	Role         string   `json:"role"`
	Organization *Account `json:"organization,omitempty"`
	User         *User    `json:"user,omitempty"`
}

// DirectoryUser excludes email addresses and unrelated profile metadata.
type DirectoryUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

// Team excludes descriptions and other private team details.
type Team struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}
