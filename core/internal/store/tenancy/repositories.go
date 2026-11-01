package tenancy

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

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
func ListRepositories(ctx context.Context, q store.DBTX, installationID int64, after *RepoCursor, limit int) ([]Repository, error) {
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
func UpsertRepositories(ctx context.Context, q store.DBTX, installationID int64, repos []models.Repository, syncedAt time.Time) error {
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

// TombstoneUnseenRepositories must run only after every page succeeds to avoid deleting live repositories.
func TombstoneUnseenRepositories(ctx context.Context, q store.DBTX, installationID int64, syncStartedAt time.Time) (int64, error) {
	tag, err := q.Exec(ctx, `
		UPDATE repositories SET deleted_at = now()
		 WHERE installation_id = $1 AND deleted_at IS NULL AND synced_at < $2`, installationID, syncStartedAt)
	return tag.RowsAffected(), err
}

// SyncStarted records the start of a sync attempt.
func SyncStarted(ctx context.Context, q store.DBTX, installationID int64, at time.Time) error {
	_, err := q.Exec(ctx, `
		INSERT INTO sync_state (installation_id, last_started_at) VALUES ($1, $2)
		ON CONFLICT (installation_id) DO UPDATE SET last_started_at = EXCLUDED.last_started_at`, installationID, at)
	return err
}

// SyncFinished records the outcome of a sync attempt. errText is empty on success.
func SyncFinished(ctx context.Context, q store.DBTX, installationID int64, seen int, errText string) error {
	_, err := q.Exec(ctx, `
		UPDATE sync_state
		   SET last_succeeded_at = CASE WHEN $3 = '' THEN now() ELSE last_succeeded_at END,
		       last_error = left(NULLIF($3, ''), $4),
		       repositories_seen = CASE WHEN $3 = '' THEN $2 ELSE repositories_seen END
		 WHERE installation_id = $1`, installationID, seen, errText, store.MaxErrorText)
	return err
}
