package contributions

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

// publishStatementTimeout covers the first publish of a large organization's history.
const publishStatementTimeout = "10min"

// PublishContributionRun makes the run visible and returns the new generation. Commits are
// replaced inside each repository's commit window, so rewritten history disappears. Other
// events are upserted by ID.
func PublishContributionRun(ctx context.Context, tx pgx.Tx, runID, installationID int64) (int64, error) {
	if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '`+publishStatementTimeout+`'`); err != nil {
		return 0, err
	}
	var until time.Time
	err := tx.QueryRow(ctx, `SELECT window_end FROM contribution_runs
		WHERE id = $1 AND installation_id = $2 AND completed_at IS NULL FOR UPDATE`, runID, installationID).Scan(&until)
	if err != nil {
		return 0, err
	}
	steps := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM contribution_events e USING contribution_run_repositories rr
		  WHERE rr.run_id = $1 AND rr.phase = 'done' AND e.installation_id = $2 AND e.repository_id = rr.repository_id
		    AND e.kind = 'commit' AND e.occurred_at >= rr.commits_since AND e.occurred_at < $3`,
			[]any{runID, installationID, until}},
		{`INSERT INTO contribution_events (installation_id, repository_id, event_id, user_id, user_login, kind, occurred_at, is_bot)
		  SELECT $2, s.repository_id, s.event_id, s.user_id, s.user_login, s.kind, s.occurred_at, s.is_bot
		    FROM staged_contributions s
		    JOIN contribution_run_repositories rr ON rr.run_id = s.run_id AND rr.repository_id = s.repository_id AND rr.phase = 'done'
		   WHERE s.run_id = $1
		  ON CONFLICT (installation_id, repository_id, event_id) DO UPDATE
		     SET user_id = EXCLUDED.user_id, user_login = EXCLUDED.user_login, kind = EXCLUDED.kind,
		         occurred_at = EXCLUDED.occurred_at, is_bot = EXCLUDED.is_bot`,
			[]any{runID, installationID}},
		{`INSERT INTO repository_ingest_state (repository_id, ingested_until)
		  SELECT repository_id, $2 FROM contribution_run_repositories WHERE run_id = $1 AND phase = 'done'
		  ON CONFLICT (repository_id) DO UPDATE SET ingested_until = GREATEST(repository_ingest_state.ingested_until, EXCLUDED.ingested_until)`,
			[]any{runID, until}},
	}
	for _, step := range steps {
		if _, err := tx.Exec(ctx, step.sql, step.args...); err != nil {
			return 0, err
		}
	}
	var generation int64
	err = tx.QueryRow(ctx, `INSERT INTO scoring_state (installation_id, generation, last_ingested_at)
		VALUES ($1, 1, $2)
		ON CONFLICT (installation_id) DO UPDATE SET generation = scoring_state.generation + 1,
		    last_ingested_at = EXCLUDED.last_ingested_at, last_failed_at = NULL, last_error = NULL
		RETURNING generation`, installationID, until).Scan(&generation)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE contribution_runs SET completed_at = now() WHERE id = $1`, runID); err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `DELETE FROM staged_contributions WHERE run_id = $1`, runID)
	return generation, err
}

// RecordIngestionFailure marks the scores as not current until the next successful publish.
func RecordIngestionFailure(ctx context.Context, q store.DBTX, installationID int64, errText string) error {
	_, err := q.Exec(ctx, `INSERT INTO scoring_state (installation_id, last_failed_at, last_error)
		VALUES ($1, now(), left($2, $3))
		ON CONFLICT (installation_id) DO UPDATE SET last_failed_at = EXCLUDED.last_failed_at, last_error = EXCLUDED.last_error`,
		installationID, errText, store.MaxErrorText)
	return err
}

func LastIngestion(ctx context.Context, q store.DBTX, installationID int64) (*time.Time, error) {
	var at *time.Time
	err := q.QueryRow(ctx, `SELECT last_ingested_at FROM scoring_state WHERE installation_id = $1`, installationID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return at, err
}

func ContributionRows(ctx context.Context, q store.DBTX, installationID, userID int64) (pgx.Rows, error) {
	return q.Query(ctx, `SELECT e.user_id, e.user_login, e.kind, r.full_name, e.occurred_at, e.is_bot
		FROM contribution_events e JOIN repositories r ON r.github_id = e.repository_id
		LEFT JOIN user_installations ui ON ui.installation_id = e.installation_id AND ui.user_id = $2
		WHERE e.installation_id = $1 AND r.installation_id = $1 AND r.deleted_at IS NULL
		AND ($2 = 0 OR ui.user_id IS NOT NULL)`, installationID, userID)
}
