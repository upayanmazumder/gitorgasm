package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
)

var ErrStaleGeneration = errors.New("scoring generation changed")

func BeginContributionRun(ctx context.Context, q DBTX, installationID int64, from, until time.Time) (int64, error) {
	var id int64
	err := q.QueryRow(ctx, `INSERT INTO contribution_runs (installation_id, window_start, window_end)
		VALUES ($1, $2, $3) RETURNING id`, installationID, from, until).Scan(&id)
	return id, err
}

func StageContributions(ctx context.Context, conn *pgxpool.Conn, runID int64, events []github.Contribution) error {
	if len(events) == 0 {
		return nil
	}
	_, err := conn.CopyFrom(ctx, pgx.Identifier{"staged_contributions"},
		[]string{"run_id", "repository_id", "event_id", "user_id", "user_login", "kind", "occurred_at", "is_bot"},
		pgx.CopyFromSlice(len(events), func(i int) ([]any, error) {
			e := &events[i]
			return []any{runID, e.RepositoryID, e.EventID, e.UserID, e.Login, e.Kind, e.OccurredAt, e.IsBot}, nil
		}))
	return err
}

// PublishContributionRun replaces the covered period and enqueues scoring in one transaction.
func PublishContributionRun(ctx context.Context, tx pgx.Tx, runID, installationID int64) (int64, error) {
	var generation int64
	var from, until time.Time
	err := tx.QueryRow(ctx, `SELECT window_start, window_end FROM contribution_runs
		WHERE id = $1 AND installation_id = $2 AND completed_at IS NULL FOR UPDATE`, runID, installationID).Scan(&from, &until)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM contribution_events
		WHERE installation_id = $1 AND occurred_at >= $2 AND occurred_at < $3`, installationID, from, until); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO contribution_events
		(installation_id, repository_id, event_id, user_id, user_login, kind, occurred_at, is_bot)
		SELECT $1, repository_id, event_id, user_id, user_login, kind, occurred_at, is_bot
		FROM staged_contributions WHERE run_id = $2 AND occurred_at >= $3 AND occurred_at < $4`,
		installationID, runID, from, until); err != nil {
		return 0, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO scoring_state
		(installation_id, last_run_id, generation, last_ingested_at)
		VALUES ($1, $2, 1, now())
		ON CONFLICT (installation_id) DO UPDATE SET
		last_run_id = EXCLUDED.last_run_id, generation = scoring_state.generation + 1,
		last_ingested_at = EXCLUDED.last_ingested_at
		RETURNING generation`, installationID, runID).Scan(&generation)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE contribution_runs SET completed_at = now() WHERE id = $1`, runID); err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `DELETE FROM staged_contributions WHERE run_id = $1`, runID)
	return generation, err
}

func PruneContributionRuns(ctx context.Context, q DBTX, installationID int64, before time.Time) error {
	_, err := q.Exec(ctx, `DELETE FROM contribution_runs WHERE installation_id = $1 AND started_at < $2`, installationID, before)
	return err
}

func LastIngestion(ctx context.Context, q DBTX, installationID int64) (*time.Time, error) {
	var at *time.Time
	err := q.QueryRow(ctx, `SELECT last_ingested_at FROM scoring_state WHERE installation_id = $1`, installationID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return at, err
}

type ScoringState struct {
	Generation       int64      `json:"generation"`
	ScoredGeneration int64      `json:"scored_generation"`
	LastIngestedAt   *time.Time `json:"last_ingested_at"`
	LastScoredAt     *time.Time `json:"last_scored_at"`
	IngestionFailed  bool       `json:"ingestion_failed"`
}

func GetScoringState(ctx context.Context, q DBTX, installationID, userID int64) (ScoringState, error) {
	var s ScoringState
	err := q.QueryRow(ctx, `SELECT COALESCE(state.generation, 0), COALESCE(state.scored_generation, 0),
		state.last_ingested_at, state.last_scored_at,
		EXISTS (SELECT 1 FROM contribution_runs r WHERE r.installation_id = $1
		AND r.completed_at IS NULL AND r.started_at > COALESCE(state.last_ingested_at, '-infinity'::timestamptz))
		FROM installations i
		LEFT JOIN scoring_state state ON state.installation_id = i.github_id
		LEFT JOIN user_installations ui ON ui.installation_id = i.github_id AND ui.user_id = $2
		WHERE i.github_id = $1 AND i.deleted_at IS NULL AND ($2 = 0 OR ui.user_id IS NOT NULL)`,
		installationID, userID).
		Scan(&s.Generation, &s.ScoredGeneration, &s.LastIngestedAt, &s.LastScoredAt, &s.IngestionFailed)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

func ContributionRows(ctx context.Context, q DBTX, installationID, userID int64) (pgx.Rows, error) {
	return q.Query(ctx, `SELECT e.user_id, e.user_login, e.kind, r.full_name, e.occurred_at, e.is_bot
		FROM contribution_events e JOIN repositories r ON r.github_id = e.repository_id
		LEFT JOIN user_installations ui ON ui.installation_id = e.installation_id AND ui.user_id = $2
		WHERE e.installation_id = $1 AND r.installation_id = $1 AND r.deleted_at IS NULL
		AND ($2 = 0 OR ui.user_id IS NOT NULL)`, installationID, userID)
}

func StoreSnapshot(ctx context.Context, tx pgx.Tx, installationID, generation, userID int64, queryKey string, result json.RawMessage, markScored bool) error {
	var current int64
	err := tx.QueryRow(ctx, `SELECT state.generation FROM scoring_state state
		LEFT JOIN user_installations ui ON ui.installation_id = state.installation_id AND ui.user_id = $2
		WHERE state.installation_id = $1 AND ($2 = 0 OR ui.user_id IS NOT NULL) FOR UPDATE OF state`,
		installationID, userID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if current != generation {
		return ErrStaleGeneration
	}
	if _, err := tx.Exec(ctx, `INSERT INTO score_snapshots (installation_id, generation, query_key, result)
		VALUES ($1, $2, $3, $4) ON CONFLICT (installation_id, generation, query_key)
		DO UPDATE SET result = EXCLUDED.result, created_at = now()`, installationID, generation, queryKey, result); err != nil {
		return err
	}
	if markScored {
		if _, err := tx.Exec(ctx, `UPDATE scoring_state SET scored_generation = $2, last_scored_at = now()
			WHERE installation_id = $1`, installationID, generation); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM score_snapshots WHERE installation_id = $1 AND generation < $2`, installationID, generation)
		return err
	}
	return nil
}

func LoadSnapshot(ctx context.Context, q DBTX, installationID, generation, userID int64, queryKey string) (json.RawMessage, error) {
	var data json.RawMessage
	err := q.QueryRow(ctx, `SELECT ss.result FROM score_snapshots ss
		LEFT JOIN user_installations ui ON ui.installation_id = ss.installation_id AND ui.user_id = $4
		WHERE ss.installation_id = $1 AND ss.generation = $2 AND ss.query_key = $3
		AND ($4 = 0 OR ui.user_id IS NOT NULL)`, installationID, generation, queryKey, userID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return data, err
}

type ScoringPreset struct {
	Name  string          `json:"name"`
	Query json.RawMessage `json:"query"`
}

func ListScoringPresets(ctx context.Context, q DBTX, installationID, userID int64) ([]ScoringPreset, error) {
	rows, err := q.Query(ctx, `SELECT p.name, p.query FROM scoring_presets p
		JOIN user_installations ui ON ui.installation_id = p.installation_id AND ui.user_id = $2
		WHERE p.installation_id = $1 ORDER BY lower(p.name)`, installationID, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ScoringPreset, error) {
		var preset ScoringPreset
		err := row.Scan(&preset.Name, &preset.Query)
		return preset, err
	})
}

func UpsertScoringPreset(ctx context.Context, q DBTX, installationID, userID int64, name string, query json.RawMessage) error {
	tag, err := q.Exec(ctx, `INSERT INTO scoring_presets (installation_id, name, query)
		SELECT ui.installation_id, $3, $4 FROM user_installations ui
		WHERE ui.installation_id = $1 AND ui.user_id = $2
		ON CONFLICT (installation_id, name) DO UPDATE SET query = EXCLUDED.query, updated_at = now()`,
		installationID, userID, name, query)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func DeleteScoringPreset(ctx context.Context, q DBTX, installationID, userID int64, name string) (bool, error) {
	tag, err := q.Exec(ctx, `DELETE FROM scoring_presets p USING user_installations ui
		WHERE p.installation_id = $1 AND p.name = $3 AND ui.installation_id = p.installation_id
		AND ui.user_id = $2`, installationID, userID, name)
	return tag.RowsAffected() > 0, err
}

func InstallationLogin(ctx context.Context, q DBTX, installationID, userID int64) (string, error) {
	var login string
	err := q.QueryRow(ctx, `SELECT a.login FROM installations i
		JOIN accounts a ON a.github_id = i.account_id
		LEFT JOIN user_installations ui ON ui.installation_id = i.github_id AND ui.user_id = $2
		WHERE i.github_id = $1 AND i.deleted_at IS NULL AND i.suspended_at IS NULL
		AND ($2 = 0 OR ui.user_id IS NOT NULL)`, installationID, userID).Scan(&login)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return login, err
}
