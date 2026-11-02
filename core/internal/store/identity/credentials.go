package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// SessionCredential is internal credential material from one exact actor session.
// It must never be encoded into an HTTP response or queued job.
type SessionCredential struct {
	UserID         int64      `json:"-"`
	GitHubID       int64      `json:"-"`
	Login          string     `json:"-"`
	KeyVersion     int16      `json:"-"`
	TokenSealed    []byte     `json:"-"`
	TokenExpiresAt *time.Time `json:"-"`
}

// LookupSessionCredential does not fall back to another session for this user.
func LookupSessionCredential(ctx context.Context, q store.DBTX, userID int64, sessionHash []byte, now time.Time) (*SessionCredential, error) {
	var out SessionCredential
	err := q.QueryRow(ctx, `
		SELECT u.id,u.github_id,u.login,s.key_version,s.github_token_sealed,s.github_token_expires_at
		  FROM sessions s JOIN users u ON u.id=s.user_id
		 WHERE s.id_hash=$1 AND s.user_id=$2 AND s.expires_at>$3`, sessionHash, userID, now).
		Scan(&out.UserID, &out.GitHubID, &out.Login, &out.KeyVersion, &out.TokenSealed, &out.TokenExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
