package scoring

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/scores"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

type Worker struct {
	river.WorkerDefaults[queue.ScoreOrganization]
	Pool   *pgxpool.Pool
	Scorer *scorer.Runner
}

const scoreJobStatementTimeout = 5 * time.Minute

func (w *Worker) Timeout(*river.Job[queue.ScoreOrganization]) time.Duration {
	return 10 * time.Minute
}

func (w *Worker) Work(ctx context.Context, job *river.Job[queue.ScoreOrganization]) error {
	id, generation := job.Args.InstallationID, job.Args.Generation
	state, err := scores.GetScoringState(ctx, w.Pool, id, 0)
	if err != nil {
		return err
	}
	if state.Generation != generation || state.ScoredGeneration >= generation {
		return nil
	}
	login, err := tenancy.InstallationLogin(ctx, w.Pool, id, 0)
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
		return scores.StoreSnapshot(ctx, tx, id, generation, 0, "default", result, true)
	})
	if errors.Is(err, scores.ErrStaleGeneration) {
		return nil
	}
	return err
}
