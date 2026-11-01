package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/contributions"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

type IngestWorker struct {
	river.WorkerDefaults[IngestContributionsArgs]
	Pool   *pgxpool.Pool
	GitHub *github.Client
	Now    func() time.Time
}

// Ingestion attempts snooze before River's rescue deadline and resume from stored cursors.
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
		if ferr := contributions.RecordIngestionFailure(context.WithoutCancel(ctx), conn, id, err.Error()); ferr != nil {
			return errors.Join(err, ferr)
		}
		return err
	})
}

func (w *IngestWorker) ingest(ctx context.Context, conn *pgxpool.Conn, id int64) error {
	started := w.Now().UTC()
	run, err := contributions.OpenContributionRun(ctx, conn, id, started)
	if err != nil {
		return err
	}
	if run == nil {
		last, err := contributions.LastIngestion(ctx, conn, id)
		if err != nil {
			return err
		}
		dayStart := started.Truncate(24 * time.Hour)
		if last != nil && !last.Before(dayStart) {
			return nil
		}
		if run, err = contributions.BeginContributionRun(ctx, conn, id, started); err != nil {
			return err
		}
	}

	var cursor *tenancy.RepoCursor
	for {
		repos, err := tenancy.ListRepositories(ctx, conn, id, cursor, 100)
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
		cursor = &tenancy.RepoCursor{Name: strings.ToLower(end.Name), ID: end.GitHubID}
	}
	client, err := insertClient(ctx, w.Pool)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		generation, err := contributions.PublishContributionRun(ctx, tx, run.ID, id)
		if err != nil {
			return err
		}
		_, err = client.InsertTx(ctx, tx, ScoreOrganizationArgs{InstallationID: id, Generation: generation}, nil)
		return err
	})
}

// ingestRepository stages pages and skips vanished repositories without deleting stored events.
func (w *IngestWorker) ingestRepository(ctx context.Context, conn *pgxpool.Conn, id int64, run *contributions.ContributionRun, repo tenancy.Repository, deadline time.Time) error {
	progress, err := contributions.ClaimRunRepository(ctx, conn, run, repo.GitHubID)
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
		func(page []models.Contribution, next models.ContributionCursor) error {
			return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
				return contributions.StageContributionPage(ctx, tx, run.ID, repo.GitHubID, page, next)
			})
		})
	switch {
	case err == nil:
		return nil
	case fetchCtx.Err() != nil && ctx.Err() == nil:
		return river.JobSnooze(time.Second) // Attempt budget spent; staged pages are kept.
	case github.IsKind(err, github.KindNotFound):
		return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			return contributions.SkipRunRepository(ctx, tx, run.ID, repo.GitHubID)
		})
	default:
		return retryDecision(err)
	}
}
