package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

func eventCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM contribution_events WHERE installation_id = $1`, instID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func scoringState(t *testing.T, pool *pgxpool.Pool) store.ScoringState {
	t.Helper()
	state, err := store.GetScoringState(context.Background(), pool, instID, 0)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestIngestionSkipsGoneRepositoriesResumesAndPublishesOnlyCompleteRuns(t *testing.T) {
	f := &fakeGitHub{contributions: true}
	pool, gh := setup(t, f)
	ctx := context.Background()
	seedStale(t, pool)
	if err := work(t, &jobs.SyncWorker{Pool: pool, GitHub: gh, Now: time.Now}); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	ingest := &jobs.IngestWorker{Pool: pool, GitHub: gh, Now: func() time.Time { return at }}
	job := &river.Job[jobs.IngestContributionsArgs]{Args: jobs.IngestContributionsArgs{InstallationID: instID}}
	score := &jobs.ScoreWorker{Pool: pool, Scorer: scorer.New(os.Getenv("SCORING_BIN"), 1, time.Minute)}

	// A repository GitHub no longer serves is skipped instead of blocking the organization.
	f.setRepoStatus("web", http.StatusNotFound)
	if err := ingest.Work(ctx, job); err != nil {
		t.Fatalf("initial ingestion: %v", err)
	}
	if state := scoringState(t, pool); state.Generation != 1 || state.IngestionFailed || eventCount(t, pool) != 2 {
		t.Fatalf("after skip: state=%+v events=%d", state, eventCount(t, pool))
	}
	if err := score.Work(ctx, &river.Job[jobs.ScoreOrganizationArgs]{Args: jobs.ScoreOrganizationArgs{InstallationID: instID, Generation: 1}}); err != nil {
		t.Fatalf("score: %v", err)
	}
	original, err := store.LoadSnapshot(ctx, pool, instID, 1, 0, "default")
	if err != nil || original == nil {
		t.Fatalf("stored scores = %s, %v", original, err)
	}
	var data struct {
		Contributors []json.RawMessage `json:"contributors"`
	}
	if err := json.Unmarshal(original, &data); err != nil || len(data.Contributors) != 1 {
		t.Fatalf("contributors = %s, %v", original, err)
	}

	// The same day does not ingest again.
	if err := ingest.Work(ctx, job); err != nil || scoringState(t, pool).Generation != 1 {
		t.Fatalf("same-day rerun: %v", err)
	}

	// Next day: "web" is back, "infra" hits a rate limit. The run snoozes, keeps finished
	// repositories, and does not mark the scores as failed.
	ingest.Now = func() time.Time { return at.Add(25 * time.Hour) }
	f.setRepoStatus("web", 0)
	f.setRepoStatus("infra", http.StatusForbidden)
	apiCalls := f.calls("api")
	err = ingest.Work(ctx, job)
	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) {
		t.Fatalf("rate limit: err = %v, want snooze", err)
	}
	if state := scoringState(t, pool); state.Generation != 1 || state.IngestionFailed {
		t.Fatalf("after rate limit: %+v", state)
	}

	// A server error is recorded; published scores stay unchanged.
	f.setRepoStatus("infra", http.StatusBadGateway)
	if err := ingest.Work(ctx, job); err == nil {
		t.Fatal("server error did not stop ingestion")
	}
	if state := scoringState(t, pool); state.Generation != 1 || !state.IngestionFailed {
		t.Fatalf("after failure: %+v", state)
	}
	if unchanged, _ := store.LoadSnapshot(ctx, pool, instID, 1, 0, "default"); string(unchanged) != string(original) {
		t.Fatal("failed ingestion changed scores")
	}

	// The retry resumes: "api" finished in the first attempt and is not fetched again.
	f.setRepoStatus("infra", 0)
	if err := ingest.Work(ctx, job); err != nil {
		t.Fatalf("resumed ingestion: %v", err)
	}
	if got := f.calls("api") - apiCalls; got != 1 {
		t.Fatalf("api fetched %d times across resumed attempts, want 1", got)
	}
	state := scoringState(t, pool)
	if state.Generation != 2 || state.IngestionFailed {
		t.Fatalf("after resume: %+v", state)
	}
	// "web" was skipped yesterday, so it has no watermark and is claimed fresh in this run.
	if n := eventCount(t, pool); n != 3 {
		t.Fatalf("events = %d, want 3 (idempotent per repository)", n)
	}
	if err := score.Work(ctx, &river.Job[jobs.ScoreOrganizationArgs]{Args: jobs.ScoreOrganizationArgs{InstallationID: instID, Generation: 2}}); err != nil {
		t.Fatalf("second score: %v", err)
	}
	if state := scoringState(t, pool); state.ScoredGeneration != 2 {
		t.Fatalf("scored generation = %d", state.ScoredGeneration)
	}
}
