package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	githubcontributions "github.com/upayanmazumder/gitorgasm/core/internal/github/contributions"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/internal/execution"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/contributions"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

// ingestRepository stages pages and skips vanished repositories without deleting stored events.
func (w *Worker) ingestRepository(ctx context.Context, conn *pgxpool.Conn, id int64, run *contributions.ContributionRun, repo tenancy.Repository, deadline time.Time) error {
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
	err = githubcontributions.Collect(fetchCtx, w.GitHub, id, repo.GitHubID, owner, name, progress.Window, progress.Cursor,
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
		return execution.RetryDecision(err)
	}
}
