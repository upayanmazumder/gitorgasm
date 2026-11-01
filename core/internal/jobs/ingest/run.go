package ingest

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/internal/execution"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/contributions"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

func (w *Worker) ingest(ctx context.Context, conn *pgxpool.Conn, id int64) error {
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
	client, err := execution.InsertClient(ctx, w.Pool)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		generation, err := contributions.PublishContributionRun(ctx, tx, run.ID, id)
		if err != nil {
			return err
		}
		_, err = client.InsertTx(ctx, tx, queue.ScoreOrganization{InstallationID: id, Generation: generation}, nil)
		return err
	})
}
