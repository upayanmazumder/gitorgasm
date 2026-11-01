package contributions

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

const (
	// Incomplete runs older than this restart instead of resuming with a stale window end.
	runResumeLimit = 7 * 24 * time.Hour
	runRetention   = 30 * 24 * time.Hour
	// Commits can arrive with old committer dates, so their window reaches further back.
	commitLookback   = 30 * 24 * time.Hour
	activityLookback = time.Hour
)

// ContributionRun is the open ingestion pass of an installation.
type ContributionRun struct {
	ID        int64
	WindowEnd time.Time
}

// OpenContributionRun prunes old runs and returns the incomplete run to resume, if any.
func OpenContributionRun(ctx context.Context, q store.DBTX, installationID int64, now time.Time) (*ContributionRun, error) {
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
func BeginContributionRun(ctx context.Context, q store.DBTX, installationID int64, windowEnd time.Time) (*ContributionRun, error) {
	run := ContributionRun{WindowEnd: windowEnd}
	err := q.QueryRow(ctx, `INSERT INTO contribution_runs (installation_id, window_end) VALUES ($1, $2) RETURNING id`,
		installationID, windowEnd).Scan(&run.ID)
	return &run, err
}

// RunRepository is the progress of one repository inside a run.
type RunRepository struct {
	Window models.ContributionWindow
	Cursor models.ContributionCursor
}

// Finished reports whether the repository needs no more GitHub requests in this run.
func (r RunRepository) Finished() bool {
	return r.Cursor.Phase == models.PhaseDone || r.Cursor.Phase == "skipped"
}

// ClaimRunRepository returns the stored progress, or starts the repository from its watermark.
// A repository without a watermark is read from the beginning of GitHub history.
func ClaimRunRepository(ctx context.Context, q store.DBTX, run *ContributionRun, repositoryID int64) (RunRepository, error) {
	history := time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)
	latest := run.WindowEnd.Add(-time.Second)
	r := RunRepository{Window: models.ContributionWindow{Until: run.WindowEnd}}
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
func StageContributionPage(ctx context.Context, tx pgx.Tx, runID, repositoryID int64, events []models.Contribution, next models.ContributionCursor) error {
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
