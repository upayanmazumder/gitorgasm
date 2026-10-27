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

// River rescues jobs that run longer than one hour, so one attempt stops well before that and
// snoozes. The next attempt resumes the same run from the stored cursors.
const (
	ingestTimeout = 50 * time.Minute
	ingestBudget  = 40 * time.Minute
)

func (w *IngestWorker) Timeout(*river.Job[IngestContributionsArgs]) time.Duration {
	return ingestTimeout
}

const ingestLockKey = `hashtextextended('gitorgasm:ingest_contributions:' || $1::bigint::text, 0)`

func (w *IngestWorker) Work(ctx context.Context, job *river.Job[IngestContributionsArgs]) error {
	id := job.Args.InstallationID
	return withSessionLock(ctx, w.Pool, ingestLockKey, id, func(conn *pgxpool.Conn) error {
		err := w.ingest(ctx, conn, id)
		var snooze *river.JobSnoozeError
		if err == nil || errors.As(err, &snooze) || ctx.Err() != nil {
			return err
		}
		if ferr := store.RecordIngestionFailure(context.WithoutCancel(ctx), conn, id, err.Error()); ferr != nil {
			return errors.Join(err, ferr)
		}
		return err
	})
}

func (w *IngestWorker) ingest(ctx context.Context, conn *pgxpool.Conn, id int64) error {
	started := w.Now().UTC()
	run, err := store.OpenContributionRun(ctx, conn, id, started)
	if err != nil {
		return err
	}
	if run == nil {
		last, err := store.LastIngestion(ctx, conn, id)
		if err != nil {
			return err
		}
		dayStart := started.Truncate(24 * time.Hour)
		if last != nil && !last.Before(dayStart) {
			return nil
		}
		if run, err = store.BeginContributionRun(ctx, conn, id, started); err != nil {
			return err
		}
	}

	var cursor *store.RepoCursor
	for {
		repos, err := store.ListRepositories(ctx, conn, id, cursor, 100)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if w.Now().Sub(started) > ingestBudget {
				return river.JobSnooze(time.Second)
			}
			if err := w.ingestRepository(ctx, conn, id, run, repo, started.Add(ingestBudget)); err != nil {
				return err
			}
		}
		if len(repos) < 100 {
			break
		}
		end := repos[len(repos)-1]
		cursor = &store.RepoCursor{Name: strings.ToLower(end.Name), ID: end.GitHubID}
	}
	client, err := insertClient(ctx, w.Pool)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		generation, err := store.PublishContributionRun(ctx, tx, run.ID, id)
		if err != nil {
			return err
		}
		_, err = client.InsertTx(ctx, tx, ScoreOrganizationArgs{InstallationID: id, Generation: generation}, nil)
		return err
	})
}

// ingestRepository stages one repository page by page. A repository GitHub no longer serves
// is skipped and keeps its stored events; other failures stop the run so it can resume later.
func (w *IngestWorker) ingestRepository(ctx context.Context, conn *pgxpool.Conn, id int64, run *store.ContributionRun, repo store.Repository, deadline time.Time) error {
	progress, err := store.ClaimRunRepository(ctx, conn, run, repo.GitHubID)
	if err != nil || progress.Finished() {
		return err
	}
	owner, name, ok := strings.Cut(repo.FullName, "/")
	if !ok || owner == "" || name != repo.Name {
		return fmt.Errorf("invalid repository name for %d", repo.GitHubID)
	}
	fetchCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	err = w.GitHub.ListContributions(fetchCtx, id, repo.GitHubID, owner, name, progress.Window, progress.Cursor,
		func(page []github.Contribution, next github.ContributionCursor) error {
			return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
				return store.StageContributionPage(ctx, tx, run.ID, repo.GitHubID, page, next)
			})
		})
	switch {
	case err == nil:
		return nil
	case fetchCtx.Err() != nil && ctx.Err() == nil:
		return river.JobSnooze(time.Second) // Attempt budget spent; staged pages are kept.
	case github.IsKind(err, github.KindNotFound):
		return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			return store.SkipRunRepository(ctx, tx, run.ID, repo.GitHubID)
		})
	default:
		return retryDecision(err)
	}
}

type ScoreWorker struct {
	river.WorkerDefaults[ScoreOrganizationArgs]
	Pool   *pgxpool.Pool
	Scorer *scorer.Runner
}

const scoreJobStatementTimeout = 5 * time.Minute

func (w *ScoreWorker) Timeout(*river.Job[ScoreOrganizationArgs]) time.Duration {
	return 10 * time.Minute
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
	if errors.Is(err, store.ErrNotFound) {
		return river.JobCancel(err) // Deleted or suspended installation.
	}
	if err != nil {
		return err
	}
	var result json.RawMessage
	err = scorer.ReadOnly(ctx, w.Pool, scoreJobStatementTimeout, func(tx pgx.Tx) error {
		var err error
		result, err = w.Scorer.Calculate(ctx, tx, id, 0, login, json.RawMessage(`{}`))
		return err
	})
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
