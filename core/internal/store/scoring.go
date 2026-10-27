package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
)

var (
	ErrStaleGeneration = errors.New("scoring generation changed")
	ErrPresetLimit     = errors.New("too many scoring presets")
)

const (
	// Incomplete runs older than this restart instead of resuming with a stale window end.
	runResumeLimit = 7 * 24 * time.Hour
	runRetention   = 30 * 24 * time.Hour
	// Commits can arrive with old committer dates, so their window reaches further back.
	commitLookback   = 30 * 24 * time.Hour
	activityLookback = time.Hour
	// MaxQuerySnapshots bounds cached custom queries per installation and generation.
	MaxQuerySnapshots = 200
	// MaxScoringPresets bounds saved presets per installation.
	MaxScoringPresets = 50
	// publishStatementTimeout covers the first publish of a large organization's history.
	publishStatementTimeout = "10min"
)

// ContributionRun is the open ingestion pass of an installation.
type ContributionRun struct {
	ID        int64
	WindowEnd time.Time
}

// OpenContributionRun prunes old runs and returns the incomplete run to resume, if any.
func OpenContributionRun(ctx context.Context, q DBTX, installationID int64, now time.Time) (*ContributionRun, error) {
	if _, err := q.Exec(ctx, `DELETE FROM contribution_runs WHERE installation_id = $1
		AND ((completed_at IS NULL AND started_at < $2) OR started_at < $3)`,
		installationID, now.Add(-runResumeLimit), now.Add(-runRetention)); err != nil {
		return nil, err
	}
	var run ContributionRun
	err := q.QueryRow(ctx, `SELECT id, window_end FROM contribution_runs
		WHERE installation_id = $1 AND completed_at IS NULL ORDER BY id DESC LIMIT 1`, installationID).
		Scan(&run.ID, &run.WindowEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &run, err
}

// BeginContributionRun starts a pass that collects events before windowEnd.
func BeginContributionRun(ctx context.Context, q DBTX, installationID int64, windowEnd time.Time) (*ContributionRun, error) {
	run := ContributionRun{WindowEnd: windowEnd}
	err := q.QueryRow(ctx, `INSERT INTO contribution_runs (installation_id, window_end) VALUES ($1, $2) RETURNING id`,
		installationID, windowEnd).Scan(&run.ID)
	return &run, err
}

// RunRepository is the progress of one repository inside a run.
type RunRepository struct {
	Window github.ContributionWindow
	Cursor github.ContributionCursor
}

// Finished reports whether the repository needs no more GitHub requests in this run.
func (r RunRepository) Finished() bool {
	return r.Cursor.Phase == github.PhaseDone || r.Cursor.Phase == "skipped"
}

// ClaimRunRepository returns the stored progress, or starts the repository from its watermark.
// A repository without a watermark is read from the beginning of GitHub history.
func ClaimRunRepository(ctx context.Context, q DBTX, run *ContributionRun, repositoryID int64) (RunRepository, error) {
	history := time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)
	latest := run.WindowEnd.Add(-time.Second)
	r := RunRepository{Window: github.ContributionWindow{Until: run.WindowEnd}}
	err := q.QueryRow(ctx, `
		WITH claimed AS (
		  INSERT INTO contribution_run_repositories (run_id, repository_id, commits_since, activity_since, phase)
		  SELECT $1, $2,
		         LEAST(COALESCE(s.ingested_until - make_interval(secs => $4), $3), $6),
		         LEAST(COALESCE(s.ingested_until - make_interval(secs => $5), $3), $6), 'commits'
		    FROM (SELECT 1) one LEFT JOIN repository_ingest_state s ON s.repository_id = $2
		  ON CONFLICT (run_id, repository_id) DO NOTHING
		  RETURNING commits_since, activity_since, phase, next_url
		)
		SELECT commits_since, activity_since, phase, next_url FROM claimed
		UNION ALL
		SELECT commits_since, activity_since, phase, next_url FROM contribution_run_repositories
		 WHERE run_id = $1 AND repository_id = $2
		LIMIT 1`, run.ID, repositoryID, history, commitLookback.Seconds(), activityLookback.Seconds(), latest).
		Scan(&r.Window.CommitsSince, &r.Window.ActivitySince, &r.Cursor.Phase, &r.Cursor.Next)
	return r, err
}

// StageContributionPage stores one page and its resume cursor atomically. Replayed pages are ignored.
func StageContributionPage(ctx context.Context, tx pgx.Tx, runID, repositoryID int64, events []github.Contribution, next github.ContributionCursor) error {
	if len(events) > 0 {
		n := len(events)
		ids, users, logins, kinds := make([]string, n), make([]int64, n), make([]string, n), make([]string, n)
		at, bots := make([]time.Time, n), make([]bool, n)
		for i := range events {
			e := &events[i]
			ids[i], users[i], logins[i], kinds[i], at[i], bots[i] = e.EventID, e.UserID, e.Login, e.Kind, e.OccurredAt, e.IsBot
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staged_contributions
			(run_id, repository_id, event_id, user_id, user_login, kind, occurred_at, is_bot)
			SELECT $1, $2, t.id, t.uid, t.login, t.kind, t.at, t.bot
			  FROM unnest($3::text[], $4::bigint[], $5::text[], $6::text[], $7::timestamptz[], $8::bool[])
			       AS t(id, uid, login, kind, at, bot)
			ON CONFLICT (run_id, repository_id, event_id) DO NOTHING`,
			runID, repositoryID, ids, users, logins, kinds, at, bots); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE contribution_run_repositories SET phase = $3, next_url = $4
		WHERE run_id = $1 AND repository_id = $2`, runID, repositoryID, next.Phase, next.Next)
	return err
}

// SkipRunRepository drops a repository that GitHub no longer serves. Its stored events stay.
func SkipRunRepository(ctx context.Context, tx pgx.Tx, runID, repositoryID int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM staged_contributions WHERE run_id = $1 AND repository_id = $2`, runID, repositoryID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE contribution_run_repositories SET phase = 'skipped', next_url = ''
		WHERE run_id = $1 AND repository_id = $2`, runID, repositoryID)
	return err
}

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
func RecordIngestionFailure(ctx context.Context, q DBTX, installationID int64, errText string) error {
	_, err := q.Exec(ctx, `INSERT INTO scoring_state (installation_id, last_failed_at, last_error)
		VALUES ($1, now(), left($2, $3))
		ON CONFLICT (installation_id) DO UPDATE SET last_failed_at = EXCLUDED.last_failed_at, last_error = EXCLUDED.last_error`,
		installationID, errText, maxErrorText)
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
		state.last_ingested_at, state.last_scored_at, state.last_failed_at IS NOT NULL
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
	_, err = tx.Exec(ctx, `DELETE FROM score_snapshots WHERE installation_id = $1 AND generation = $2
		AND query_key <> 'default' AND query_key NOT IN (
		  SELECT query_key FROM score_snapshots WHERE installation_id = $1 AND generation = $2 AND query_key <> 'default'
		   ORDER BY created_at DESC LIMIT $3)`, installationID, generation, MaxQuerySnapshots)
	return err
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

// UpsertScoringPreset saves a preset if the caller has access and the installation is under the preset limit.
func UpsertScoringPreset(ctx context.Context, q DBTX, installationID, userID int64, name string, query json.RawMessage) error {
	tag, err := q.Exec(ctx, `INSERT INTO scoring_presets (installation_id, name, query)
		SELECT ui.installation_id, $3, $4 FROM user_installations ui
		WHERE ui.installation_id = $1 AND ui.user_id = $2
		  AND ((SELECT count(*) FROM scoring_presets WHERE installation_id = $1) < $5
		       OR EXISTS (SELECT 1 FROM scoring_presets WHERE installation_id = $1 AND name = $3))
		ON CONFLICT (installation_id, name) DO UPDATE SET query = EXCLUDED.query, updated_at = now()`,
		installationID, userID, name, query, MaxScoringPresets)
	if err != nil || tag.RowsAffected() > 0 {
		return err
	}
	var granted bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_installations WHERE installation_id = $1 AND user_id = $2)`,
		installationID, userID).Scan(&granted); err != nil {
		return err
	}
	if granted {
		return ErrPresetLimit
	}
	return ErrNotFound
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
