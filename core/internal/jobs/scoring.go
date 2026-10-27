package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

type IngestContributionsArgs struct {
	InstallationID int64 `json:"installation_id"`
}

func (IngestContributionsArgs) Kind() string { return "ingest_contributions" }
func (IngestContributionsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueGitHub, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: time.Minute}}
}

type ScoreOrganizationArgs struct {
	InstallationID int64 `json:"installation_id"`
	Generation     int64 `json:"generation"`
}

func (ScoreOrganizationArgs) Kind() string { return "score_organization" }
func (ScoreOrganizationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueScoring, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type IngestWorker struct {
	river.WorkerDefaults[IngestContributionsArgs]
	Pool   *pgxpool.Pool
	GitHub *github.Client
	Now    func() time.Time
}

func (w *IngestWorker) Timeout(*river.Job[IngestContributionsArgs]) time.Duration {
	return 3 * time.Hour
}

const ingestLockKey = `hashtextextended('gitorgasm:ingest_contributions:' || $1::bigint::text, 0)`

func (w *IngestWorker) Work(ctx context.Context, job *river.Job[IngestContributionsArgs]) error {
	id := job.Args.InstallationID
	conn, err := w.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(`+ingestLockKey+`)`, id).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return river.JobSnooze(30 * time.Second)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(`+ingestLockKey+`)`, id)
	}()

	last, err := store.LastIngestion(ctx, conn, id)
	if err != nil {
		return err
	}
	until := w.Now().UTC()
	dayStart := time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, time.UTC)
	if last != nil && !last.Before(dayStart) {
		return nil
	}
	from := time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)
	if last != nil {
		from = last.Add(-30 * 24 * time.Hour)
	}
	if err := store.PruneContributionRuns(ctx, conn, id, until.Add(-30*24*time.Hour)); err != nil {
		return err
	}
	runID, err := store.BeginContributionRun(ctx, conn, id, from, until)
	if err != nil {
		return err
	}

	var cursor *store.RepoCursor
	for {
		repos, err := store.ListRepositories(ctx, conn, id, cursor, 100)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			owner, name, ok := strings.Cut(repo.FullName, "/")
			if !ok || owner == "" || name != repo.Name {
				return fmt.Errorf("invalid repository name for %d", repo.GitHubID)
			}
			err := w.GitHub.ListContributions(ctx, id, repo.GitHubID, owner, repo.Name, from, until, func(page []github.Contribution) error {
				return store.StageContributions(ctx, conn, runID, page)
			})
			if err != nil {
				return retryDecision(err)
			}
		}
		if len(repos) < 100 {
			break
		}
		end := repos[len(repos)-1]
		cursor = &store.RepoCursor{Name: strings.ToLower(end.Name), ID: end.GitHubID}
	}
	client, err := NewInsertClient(w.Pool)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		generation, err := store.PublishContributionRun(ctx, tx, runID, id)
		if err != nil {
			return err
		}
		_, err = client.InsertTx(ctx, tx, ScoreOrganizationArgs{InstallationID: id, Generation: generation}, nil)
		return err
	})
}

type ScoreWorker struct {
	river.WorkerDefaults[ScoreOrganizationArgs]
	Pool *pgxpool.Pool
}

func (w *ScoreWorker) Timeout(*river.Job[ScoreOrganizationArgs]) time.Duration {
	return 5 * time.Minute
}

func (w *ScoreWorker) Work(ctx context.Context, job *river.Job[ScoreOrganizationArgs]) error {
	id, generation := job.Args.InstallationID, job.Args.Generation
	state, err := store.GetScoringState(ctx, w.Pool, id, 0)
	if err != nil {
		return err
	}
	if state.Generation != generation || state.ScoredGeneration >= generation {
		return nil
	}
	login, err := store.InstallationLogin(ctx, w.Pool, id, 0)
	if err != nil {
		return err
	}
	result, err := scorer.Calculate(ctx, w.Pool, id, 0, login, json.RawMessage(`{}`))
	if err != nil {
		return err
	}
	err = pgx.BeginFunc(ctx, w.Pool, func(tx pgx.Tx) error {
		return store.StoreSnapshot(ctx, tx, id, generation, 0, "default", result, true)
	})
	if errors.Is(err, store.ErrStaleGeneration) {
		return nil
	}
	return err
}
