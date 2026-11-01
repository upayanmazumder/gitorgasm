package httpapi_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/contributions"
)

func newScoreFixture(t *testing.T) (*harness, string, string, time.Time, func(int64, int64, int64, string, ...time.Time) int64) {
	t.Helper()
	h := newHarness(t)
	alice := h.seedTenant(1, "acme", 100, 10, "api")
	bob := h.seedTenant(2, "globex", 200, 20, "secret")
	ctx := context.Background()
	start := time.Now().UTC().Add(-4 * time.Hour).Truncate(time.Second)
	publish := func(instID, repoID, userID int64, login string, moments ...time.Time) int64 {
		t.Helper()
		run, err := contributions.BeginContributionRun(ctx, h.pool, instID, start.Add(4*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := contributions.ClaimRunRepository(ctx, h.pool, run, repoID); err != nil {
			t.Fatal(err)
		}
		events := make([]models.Contribution, len(moments))
		for i, at := range moments {
			events[i] = models.Contribution{RepositoryID: repoID, EventID: fmt.Sprintf("%d-%d", run.ID, i), UserID: userID, Login: login, Kind: "commit", OccurredAt: at}
		}
		var generation int64
		err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
			if err := contributions.StageContributionPage(ctx, tx, run.ID, repoID, events, models.ContributionCursor{Phase: models.PhaseDone}); err != nil {
				return err
			}
			var publishErr error
			generation, publishErr = contributions.PublishContributionRun(ctx, tx, run.ID, instID)
			return publishErr
		})
		if err != nil {
			t.Fatal(err)
		}
		job := &river.Job[jobs.ScoreOrganizationArgs]{Args: jobs.ScoreOrganizationArgs{InstallationID: instID, Generation: generation}}
		if err := (&jobs.ScoreWorker{Pool: h.pool, Scorer: testScorer()}).Work(ctx, job); err != nil {
			t.Fatal(err)
		}
		return generation
	}
	publish(10, 10000, 7, "alice", start, start.Add(time.Hour))
	publish(20, 20000, 8, "bob", start)
	return h, alice, bob, start, publish
}
