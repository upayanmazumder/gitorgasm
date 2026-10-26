// Package store holds every SQL statement the service runs. Handlers and workers never
// build SQL themselves.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/upayanmazumder/gitorgasm/services/core/internal/github"
)

// ErrNotFound is returned when a row does not exist or is not visible to the caller.
var ErrNotFound = errors.New("not found")

// DBTX is satisfied by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// UpsertUser stores the GitHub identity and returns the internal user id.
func UpsertUser(ctx context.Context, q DBTX, u *github.User) (int64, error) {
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
func CreateSession(ctx context.Context, q DBTX, s NewSession) error {
	_, err := q.Exec(ctx, `
		INSERT INTO sessions (id_hash, user_id, key_version, github_token_sealed, github_refresh_sealed,
		                      github_token_expires_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.IDHash, s.UserID, s.KeyVersion, s.TokenSealed, s.RefreshSealed, s.TokenExpiresAt, s.ExpiresAt)
	return err
}

// SessionUser is the authenticated principal resolved from a session cookie.
type SessionUser struct {
	UserID    int64   `json:"-"`
	GitHubID  int64   `json:"github_id"`
	Login     string  `json:"login"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

// LookupSession returns the user for a live session and refreshes last_seen_at at most
// once per minute to limit write load.
func LookupSession(ctx context.Context, q DBTX, idHash []byte) (*SessionUser, error) {
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
		return nil, ErrNotFound
	}
	return &u, err
}

// DeleteSession removes one session.
func DeleteSession(ctx context.Context, q DBTX, idHash []byte) error {
	_, err := q.Exec(ctx, `DELETE FROM sessions WHERE id_hash = $1`, idHash)
	return err
}

// RevokeGitHubUser removes every session and installation grant of a user, for example
// after the user revokes the App authorization. The user must sign in again, and the
// fresh GET /user/installations result rebuilds the grants.
func RevokeGitHubUser(ctx context.Context, q DBTX, githubID int64) (int64, error) {
	if _, err := q.Exec(ctx, `
		DELETE FROM user_installations WHERE user_id = (SELECT id FROM users WHERE github_id = $1)`, githubID); err != nil {
		return 0, err
	}
	tag, err := q.Exec(ctx, `
		DELETE FROM sessions WHERE user_id = (SELECT id FROM users WHERE github_id = $1)`, githubID)
	return tag.RowsAffected(), err
}

// UpsertInstallation mirrors an installation and its account.
func UpsertInstallation(ctx context.Context, q DBTX, inst *github.Installation) error {
	if inst.Account == nil {
		return errors.New("installation has no account")
	}
	perms, err := json.Marshal(inst.Permissions)
	if err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO accounts (github_id, login, type, avatar_url) VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_id) DO UPDATE
		   SET login = EXCLUDED.login, type = EXCLUDED.type, avatar_url = EXCLUDED.avatar_url, updated_at = now()`,
		inst.Account.ID, inst.Account.Name(), inst.Account.Kind(), inst.Account.AvatarURL); err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO installations (github_id, account_id, repository_selection, permissions, suspended_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (github_id) DO UPDATE
		   SET account_id = EXCLUDED.account_id, repository_selection = EXCLUDED.repository_selection,
		       permissions = EXCLUDED.permissions, suspended_at = EXCLUDED.suspended_at,
		       deleted_at = NULL, updated_at = now()`,
		inst.ID, inst.Account.ID, inst.RepositorySelection, perms, inst.SuspendedAt)
	return err
}

// ReplaceUserInstallations sets the exact set of installations a user can access, as
// reported by GitHub. Stale grants are removed in the same transaction.
func ReplaceUserInstallations(ctx context.Context, q DBTX, userID int64, installationIDs []int64) error {
	if _, err := q.Exec(ctx, `
		DELETE FROM user_installations WHERE user_id = $1 AND NOT (installation_id = ANY($2))`,
		userID, installationIDs); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `
		INSERT INTO user_installations (user_id, installation_id)
		SELECT $1, unnest($2::bigint[])
		ON CONFLICT (user_id, installation_id) DO UPDATE SET verified_at = now()`, userID, installationIDs)
	return err
}

// MarkInstallationDeleted soft-deletes an installation and revokes all user grants to it.
func MarkInstallationDeleted(ctx context.Context, q DBTX, installationID int64) error {
	if _, err := q.Exec(ctx, `UPDATE installations SET deleted_at = now(), updated_at = now() WHERE github_id = $1`, installationID); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `DELETE FROM user_installations WHERE installation_id = $1`, installationID)
	return err
}

// Organization is one entry of GET /v1/organizations.
type Organization struct {
	Login               string     `json:"login"`
	GitHubID            int64      `json:"github_id"`
	Type                string     `json:"type"`
	AvatarURL           *string    `json:"avatar_url"`
	InstallationID      int64      `json:"installation_id"`
	RepositorySelection string     `json:"repository_selection"`
	Suspended           bool       `json:"suspended"`
	RepositoryCount     int64      `json:"repository_count"`
	LastSyncedAt        *time.Time `json:"last_synced_at"`
}

// ListOrganizations returns the accounts the user can access through live installations.
// The user_installations join is the tenant boundary.
func ListOrganizations(ctx context.Context, q DBTX, userID int64) ([]Organization, error) {
	rows, err := q.Query(ctx, `
		SELECT a.login, a.github_id, a.type, a.avatar_url, i.github_id, i.repository_selection,
		       i.suspended_at IS NOT NULL,
		       (SELECT count(*) FROM repositories r WHERE r.installation_id = i.github_id AND r.deleted_at IS NULL),
		       s.last_succeeded_at
		  FROM user_installations ui
		  JOIN installations i ON i.github_id = ui.installation_id AND i.deleted_at IS NULL
		  JOIN accounts a ON a.github_id = i.account_id
		  LEFT JOIN sync_state s ON s.installation_id = i.github_id
		 WHERE ui.user_id = $1
		 ORDER BY lower(a.login)`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Organization, error) {
		var o Organization
		err := r.Scan(&o.Login, &o.GitHubID, &o.Type, &o.AvatarURL, &o.InstallationID, &o.RepositorySelection,
			&o.Suspended, &o.RepositoryCount, &o.LastSyncedAt)
		return o, err
	})
}

// AccessibleInstallation returns the installation id for an account login if the user may
// access it. A missing grant returns ErrNotFound so callers cannot probe other tenants.
func AccessibleInstallation(ctx context.Context, q DBTX, userID int64, login string) (int64, error) {
	var id int64
	err := q.QueryRow(ctx, `
		SELECT i.github_id
		  FROM accounts a
		  JOIN installations i ON i.account_id = a.github_id AND i.deleted_at IS NULL
		  JOIN user_installations ui ON ui.installation_id = i.github_id AND ui.user_id = $1
		 WHERE lower(a.login) = lower($2)`, userID, login).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// Repository is one entry of the repository list.
type Repository struct {
	GitHubID      int64      `json:"github_id"`
	Name          string     `json:"name"`
	FullName      string     `json:"full_name"`
	Description   *string    `json:"description"`
	Private       bool       `json:"private"`
	Fork          bool       `json:"fork"`
	Archived      bool       `json:"archived"`
	Visibility    *string    `json:"visibility"`
	DefaultBranch *string    `json:"default_branch"`
	Language      *string    `json:"language"`
	Stargazers    int        `json:"stargazers"`
	PushedAt      *time.Time `json:"pushed_at"`
	SyncedAt      time.Time  `json:"synced_at"`
}

// RepoCursor is the keyset position (lower(name), github_id) of the last returned row.
type RepoCursor struct {
	Name string `json:"n"`
	ID   int64  `json:"i"`
}

// ListRepositories returns up to limit repositories after the cursor, ordered by name.
func ListRepositories(ctx context.Context, q DBTX, installationID int64, after *RepoCursor, limit int) ([]Repository, error) {
	afterName, afterID := "", int64(0)
	if after != nil {
		afterName, afterID = after.Name, after.ID
	}
	rows, err := q.Query(ctx, `
		SELECT github_id, name, full_name, description, private, fork, archived, visibility,
		       default_branch, language, stargazers, pushed_at, synced_at
		  FROM repositories
		 WHERE installation_id = $1 AND deleted_at IS NULL
		   AND ($2 = '' AND $3 = 0 OR (lower(name), github_id) > ($2, $3))
		 ORDER BY lower(name), github_id
		 LIMIT $4`, installationID, afterName, afterID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Repository, error) {
		var x Repository
		err := r.Scan(&x.GitHubID, &x.Name, &x.FullName, &x.Description, &x.Private, &x.Fork, &x.Archived,
			&x.Visibility, &x.DefaultBranch, &x.Language, &x.Stargazers, &x.PushedAt, &x.SyncedAt)
		return x, err
	})
}

// UpsertRepositories writes one page of repositories in a single statement.
func UpsertRepositories(ctx context.Context, q DBTX, installationID int64, repos []github.Repository, syncedAt time.Time) error {
	if len(repos) == 0 {
		return nil
	}
	n := len(repos)
	ids, owners, stars := make([]int64, n), make([]int64, n), make([]int32, n)
	names, fulls, vis, branches := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	descs, langs := make([]*string, n), make([]*string, n)
	priv, fork, arch := make([]bool, n), make([]bool, n), make([]bool, n)
	pushed, updated := make([]*time.Time, n), make([]*time.Time, n)
	for i, r := range repos {
		ids[i], owners[i], stars[i] = r.ID, r.Owner.ID, int32(min(r.StargazersCount, math.MaxInt32)) //nolint:gosec // clamped
		names[i], fulls[i], vis[i], branches[i] = r.Name, r.FullName, r.Visibility, r.DefaultBranch
		descs[i], langs[i] = r.Description, r.Language
		priv[i], fork[i], arch[i] = r.Private, r.Fork, r.Archived
		pushed[i], updated[i] = r.PushedAt, r.UpdatedAt
	}
	_, err := q.Exec(ctx, `
		INSERT INTO repositories (github_id, installation_id, owner_id, name, full_name, description, private, fork,
		                          archived, visibility, default_branch, language, stargazers, pushed_at,
		                          github_updated_at, synced_at, deleted_at)
		SELECT t.id, $1, t.owner, t.name, t.fullname, t.descr, t.priv, t.fork, t.arch, NULLIF(t.vis, ''),
		       NULLIF(t.branch, ''), t.lang, t.stars, t.pushed, t.updated, $2, NULL
		  FROM unnest($3::bigint[], $4::bigint[], $5::text[], $6::text[], $7::text[], $8::bool[], $9::bool[],
		              $10::bool[], $11::text[], $12::text[], $13::text[], $14::int[], $15::timestamptz[],
		              $16::timestamptz[])
		       AS t(id, owner, name, fullname, descr, priv, fork, arch, vis, branch, lang, stars, pushed, updated)
		ON CONFLICT (github_id) DO UPDATE
		   SET installation_id = EXCLUDED.installation_id, owner_id = EXCLUDED.owner_id, name = EXCLUDED.name,
		       full_name = EXCLUDED.full_name, description = EXCLUDED.description, private = EXCLUDED.private,
		       fork = EXCLUDED.fork, archived = EXCLUDED.archived, visibility = EXCLUDED.visibility,
		       default_branch = EXCLUDED.default_branch, language = EXCLUDED.language,
		       stargazers = EXCLUDED.stargazers, pushed_at = EXCLUDED.pushed_at,
		       github_updated_at = EXCLUDED.github_updated_at, synced_at = EXCLUDED.synced_at, deleted_at = NULL`,
		installationID, syncedAt, ids, owners, names, fulls, descs, priv, fork, arch, vis, branches, langs, stars,
		pushed, updated)
	return err
}

// TombstoneUnseenRepositories marks repositories that a completed full sync did not see as
// deleted. Run it only after every page succeeded, otherwise live repositories disappear.
func TombstoneUnseenRepositories(ctx context.Context, q DBTX, installationID int64, syncStartedAt time.Time) (int64, error) {
	tag, err := q.Exec(ctx, `
		UPDATE repositories SET deleted_at = now()
		 WHERE installation_id = $1 AND deleted_at IS NULL AND synced_at < $2`, installationID, syncStartedAt)
	return tag.RowsAffected(), err
}

// SyncStarted records the start of a sync attempt.
func SyncStarted(ctx context.Context, q DBTX, installationID int64, at time.Time) error {
	_, err := q.Exec(ctx, `
		INSERT INTO sync_state (installation_id, last_started_at) VALUES ($1, $2)
		ON CONFLICT (installation_id) DO UPDATE SET last_started_at = EXCLUDED.last_started_at`, installationID, at)
	return err
}

// SyncFinished records the outcome of a sync attempt. errText is empty on success.
func SyncFinished(ctx context.Context, q DBTX, installationID int64, seen int, errText string) error {
	_, err := q.Exec(ctx, `
		UPDATE sync_state
		   SET last_succeeded_at = CASE WHEN $3 = '' THEN now() ELSE last_succeeded_at END,
		       last_error = NULLIF($3, ''),
		       repositories_seen = CASE WHEN $3 = '' THEN $2 ELSE repositories_seen END
		 WHERE installation_id = $1`, installationID, seen, errText)
	return err
}

// ActiveInstallationIDs lists installations that are neither deleted nor suspended.
func ActiveInstallationIDs(ctx context.Context, q DBTX) ([]int64, error) {
	rows, err := q.Query(ctx, `SELECT github_id FROM installations WHERE deleted_at IS NULL AND suspended_at IS NULL ORDER BY github_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// WebhookDelivery is a stored webhook.
type WebhookDelivery struct {
	DeliveryID     string
	Event          string
	Action         *string
	InstallationID *int64
	Payload        []byte
	ProcessedAt    *time.Time
}

// InsertWebhookDelivery stores a delivery. It returns false when GitHub redelivered a GUID
// that is already stored.
func InsertWebhookDelivery(ctx context.Context, q DBTX, d WebhookDelivery) (bool, error) {
	tag, err := q.Exec(ctx, `
		INSERT INTO webhook_deliveries (delivery_id, event, action, installation_id, payload)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT (delivery_id) DO NOTHING`,
		d.DeliveryID, d.Event, d.Action, d.InstallationID, d.Payload)
	return tag.RowsAffected() == 1, err
}

// GetWebhookDelivery loads a delivery.
func GetWebhookDelivery(ctx context.Context, q DBTX, id string) (*WebhookDelivery, error) {
	var d WebhookDelivery
	err := q.QueryRow(ctx, `
		SELECT delivery_id, event, action, installation_id, payload, processed_at
		  FROM webhook_deliveries WHERE delivery_id = $1`, id).
		Scan(&d.DeliveryID, &d.Event, &d.Action, &d.InstallationID, &d.Payload, &d.ProcessedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

// FinishWebhookDelivery marks a delivery processed, or records the failure.
func FinishWebhookDelivery(ctx context.Context, q DBTX, id string, errText string) error {
	_, err := q.Exec(ctx, `
		UPDATE webhook_deliveries
		   SET processed_at = CASE WHEN $2 = '' THEN now() ELSE processed_at END, last_error = NULLIF($2, '')
		 WHERE delivery_id = $1`, id, errText)
	return err
}

// AuditEvent is one security-relevant action.
type AuditEvent struct {
	ActorUserID *int64
	Action      string
	Target      string
	RequestID   string
	Metadata    map[string]any
}

// Audit appends an audit event.
func Audit(ctx context.Context, q DBTX, e AuditEvent) error {
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return err
	}
	if e.Metadata == nil {
		meta = []byte("{}")
	}
	_, err = q.Exec(ctx, `
		INSERT INTO audit_events (actor_user_id, action, target, request_id, metadata)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)`, e.ActorUserID, e.Action, e.Target, e.RequestID, meta)
	return err
}
