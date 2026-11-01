package tenancy

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// UpsertInstallation mirrors an installation and its account.
func UpsertInstallation(ctx context.Context, q store.DBTX, inst *models.Installation) error {
	if inst.Account == nil {
		return errors.New("installation has no account")
	}
	perms := []byte("{}") // a nil map would marshal to JSON null, which the CHECK rejects
	if inst.Permissions != nil {
		var err error
		if perms, err = json.Marshal(inst.Permissions); err != nil {
			return err
		}
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO accounts (github_id, login, type, avatar_url) VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_id) DO UPDATE
		   SET login = EXCLUDED.login, type = EXCLUDED.type, avatar_url = EXCLUDED.avatar_url, updated_at = now()`,
		inst.Account.ID, inst.Account.Name(), inst.Account.Kind(), inst.Account.AvatarURL); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `
		INSERT INTO installations (github_id, account_id, repository_selection, permissions, suspended_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (github_id) DO UPDATE
		   SET account_id = EXCLUDED.account_id, repository_selection = EXCLUDED.repository_selection,
		       permissions = EXCLUDED.permissions, suspended_at = EXCLUDED.suspended_at,
		       deleted_at = NULL, updated_at = now()`,
		inst.ID, inst.Account.ID, inst.RepositorySelection, perms, inst.SuspendedAt)
	return err
}

// ReplaceUserInstallations removes stale grants while setting GitHub's current access list.
func ReplaceUserInstallations(ctx context.Context, q store.DBTX, userID int64, installationIDs []int64) error {
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
func MarkInstallationDeleted(ctx context.Context, q store.DBTX, installationID int64) error {
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

// ListOrganizations enforces the tenant boundary through user_installations.
func ListOrganizations(ctx context.Context, q store.DBTX, userID int64) ([]Organization, error) {
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

// AccessibleInstallation hides missing grants and chooses the newest account when a renamed login is reused.
func AccessibleInstallation(ctx context.Context, q store.DBTX, userID int64, login string) (int64, error) {
	var id int64
	err := q.QueryRow(ctx, `
		SELECT i.github_id
		  FROM accounts a
		  JOIN installations i ON i.account_id = a.github_id AND i.deleted_at IS NULL
		  JOIN user_installations ui ON ui.installation_id = i.github_id AND ui.user_id = $1
		 WHERE lower(a.login) = lower($2)
		 ORDER BY a.updated_at DESC
		 LIMIT 1`, userID, login).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, store.ErrNotFound
	}
	return id, err
}

// ActiveInstallationIDs lists installations that are neither deleted nor suspended.
func ActiveInstallationIDs(ctx context.Context, q store.DBTX) ([]int64, error) {
	rows, err := q.Query(ctx, `SELECT github_id FROM installations WHERE deleted_at IS NULL AND suspended_at IS NULL ORDER BY github_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

func InstallationLogin(ctx context.Context, q store.DBTX, installationID, userID int64) (string, error) {
	var login string
	err := q.QueryRow(ctx, `SELECT a.login FROM installations i
		JOIN accounts a ON a.github_id = i.account_id
		LEFT JOIN user_installations ui ON ui.installation_id = i.github_id AND ui.user_id = $2
		WHERE i.github_id = $1 AND i.deleted_at IS NULL AND i.suspended_at IS NULL
		AND ($2 = 0 OR ui.user_id IS NOT NULL)`, installationID, userID).Scan(&login)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", store.ErrNotFound
	}
	return login, err
}
