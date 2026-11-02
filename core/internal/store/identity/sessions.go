package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// UpsertUser stores the GitHub identity and returns the internal user id.
func UpsertUser(ctx context.Context, q store.DBTX, u *models.User) (int64, error) {
	var id int64
	err := q.QueryRow(ctx, `
		INSERT INTO users (github_id, login, name, avatar_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_id) DO UPDATE
		   SET login = EXCLUDED.login, name = EXCLUDED.name, avatar_url = EXCLUDED.avatar_url, updated_at = now()
		RETURNING id`, u.ID, u.Login, u.Name, u.AvatarURL).Scan(&id)
	return id, err
}

// NewSession is the data needed to create a session row.
type NewSession struct {
	IDHash         []byte
	UserID         int64
	KeyVersion     int16
	TokenSealed    []byte
	RefreshSealed  []byte
	TokenExpiresAt *time.Time
	ExpiresAt      time.Time
}

// CreateSession inserts a session.
func CreateSession(ctx context.Context, q store.DBTX, s NewSession) error {
	_, err := q.Exec(ctx, `
		INSERT INTO sessions (id_hash, user_id, key_version, github_token_sealed, github_refresh_sealed,
		                      github_token_expires_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.IDHash, s.UserID, s.KeyVersion, s.TokenSealed, s.RefreshSealed, s.TokenExpiresAt, s.ExpiresAt)
	return err
}

// SessionUser is the authenticated principal resolved from a session cookie.
type SessionUser struct {
	UserID      int64   `json:"-"`
	SessionHash []byte  `json:"-"`
	GitHubID    int64   `json:"github_id"`
	Login       string  `json:"login"`
	Name        *string `json:"name"`
	AvatarURL   *string `json:"avatar_url"`
}

// LookupSession updates last_seen_at at most once per minute to limit writes.
func LookupSession(ctx context.Context, q store.DBTX, idHash []byte) (*SessionUser, error) {
	var u SessionUser
	err := q.QueryRow(ctx, `
		WITH s AS (
		  UPDATE sessions SET last_seen_at = now()
		   WHERE id_hash = $1 AND expires_at > now() AND last_seen_at < now() - interval '1 minute'
		  RETURNING user_id
		)
		SELECT u.id, u.github_id, u.login, u.name, u.avatar_url
		  FROM sessions ss JOIN users u ON u.id = ss.user_id
		 WHERE ss.id_hash = $1 AND ss.expires_at > now()`, idHash).
		Scan(&u.UserID, &u.GitHubID, &u.Login, &u.Name, &u.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	return &u, err
}

// DeleteSession removes one session.
func DeleteSession(ctx context.Context, q store.DBTX, idHash []byte) error {
	_, err := q.Exec(ctx, `DELETE FROM sessions WHERE id_hash = $1`, idHash)
	return err
}

// RevokeGitHubUser removes sessions and grants; a fresh login rebuilds authorized installations.
func RevokeGitHubUser(ctx context.Context, q store.DBTX, githubID int64) (int64, error) {
	if _, err := q.Exec(ctx, `
		DELETE FROM user_installations WHERE user_id = (SELECT id FROM users WHERE github_id = $1)`, githubID); err != nil {
		return 0, err
	}
	tag, err := q.Exec(ctx, `
		DELETE FROM sessions WHERE user_id = (SELECT id FROM users WHERE github_id = $1)`, githubID)
	return tag.RowsAffected(), err
}
