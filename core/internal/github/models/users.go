package models

import "time"

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
