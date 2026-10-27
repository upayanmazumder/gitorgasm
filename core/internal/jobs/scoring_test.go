package jobs_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

func TestDailyIngestionScoresOnlyCompleteWindows(t *testing.T) {
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
	if err := ingest.Work(ctx, job); err != nil {
		t.Fatalf("initial ingestion: %v", err)
	}
	state, err := store.GetScoringState(ctx, pool, instID, 0)
	if err != nil || state.Generation != 1 || state.ScoredGeneration != 0 {
		t.Fatalf("state before scoring = %+v, %v", state, err)
	}
	score := &jobs.ScoreWorker{Pool: pool}
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

	ingest.Now = func() time.Time { return at.Add(25 * time.Hour) }
	f.failEvents.Store(true)
	if err := ingest.Work(ctx, job); err == nil {
		t.Fatal("failed page did not abort ingestion")
	}
	state, err = store.GetScoringState(ctx, pool, instID, 0)
	if err != nil || state.Generation != 1 || !state.IngestionFailed {
		t.Fatalf("state after failed ingestion = %+v, %v", state, err)
	}
	unchanged, err := store.LoadSnapshot(ctx, pool, instID, 1, 0, "default")
	if err != nil || string(unchanged) != string(original) {
		t.Fatalf("failed ingestion changed scores: %s, %v", unchanged, err)
	}

	f.failEvents.Store(false)
	if err := ingest.Work(ctx, job); err != nil {
		t.Fatalf("retry ingestion: %v", err)
	}
	state, err = store.GetScoringState(ctx, pool, instID, 0)
	if err != nil || state.Generation != 2 {
		t.Fatalf("state after retry = %+v, %v", state, err)
	}
	if err := score.Work(ctx, &river.Job[jobs.ScoreOrganizationArgs]{Args: jobs.ScoreOrganizationArgs{InstallationID: instID, Generation: 2}}); err != nil {
		t.Fatalf("retry score: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contribution_events WHERE installation_id = $1`, instID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("idempotent event count = %d, %v", count, err)
	}
	state, err = store.GetScoringState(ctx, pool, instID, 0)
	if err != nil || state.ScoredGeneration != 2 || state.IngestionFailed {
		t.Fatalf("state after complete retry = %+v, %v", state, err)
	}
}
