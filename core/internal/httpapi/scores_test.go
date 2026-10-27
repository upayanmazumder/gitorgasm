package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

func TestScoresRespectTenantPeriodsPresetsAndGeneration(t *testing.T) {
	h := newHarness(t)
	alice := h.seedTenant(1, "acme", 100, 10, "api")
	bob := h.seedTenant(2, "globex", 200, 20, "secret")
	ctx := context.Background()
	start := time.Now().UTC().Add(-4 * time.Hour).Truncate(time.Second)
	publish := func(instID, repoID, userID int64, login string, moments ...time.Time) int64 {
		t.Helper()
		run, err := store.BeginContributionRun(ctx, h.pool, instID, start.Add(-time.Hour), start.Add(4*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		conn, err := h.pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		for i, at := range moments {
			events := []github.Contribution{{RepositoryID: repoID, EventID: fmt.Sprintf("%d-%d", run, i), UserID: userID, Login: login, Kind: "commit", OccurredAt: at}}
			if err := store.StageContributions(ctx, conn, run, events); err != nil {
				t.Fatal(err)
			}
		}
		var generation int64
		err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
			var publishErr error
			generation, publishErr = store.PublishContributionRun(ctx, tx, run, instID)
			return publishErr
		})
		if err != nil {
			t.Fatal(err)
		}
		job := &river.Job[jobs.ScoreOrganizationArgs]{Args: jobs.ScoreOrganizationArgs{InstallationID: instID, Generation: generation}}
		if err := (&jobs.ScoreWorker{Pool: h.pool}).Work(ctx, job); err != nil {
			t.Fatal(err)
		}
		return generation
	}
	publish(10, 10000, 7, "alice", start, start.Add(time.Hour))
	publish(20, 20000, 8, "bob", start)

	if rec := h.do("GET", "/v1/organizations/globex/scores", alice, nil, nil); rec.Code != 404 {
		t.Fatalf("cross-org score = %d", rec.Code)
	}
	if rec := h.do("GET", "/v1/organizations/acme/scores", alice, nil, nil); rec.Code != 200 {
		t.Fatalf("default scores = %d %s", rec.Code, rec.Body)
	}
	query := fmt.Sprintf(`{"period":{"kind":"custom","start":%q,"end":%q},"filter":{"search":"alice"}}`, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339))
	rec := h.do("POST", "/v1/organizations/acme/scores/query", alice, []byte(query), map[string]string{"Origin": appOrigin})
	if rec.Code != 200 {
		t.Fatalf("custom scores = %d %s", rec.Code, rec.Body)
	}
	var result struct {
		Data struct {
			Contributors []struct {
				UserID int64 `json:"userId"`
				Counts struct {
					Commits uint32 `json:"commits"`
				} `json:"counts"`
			} `json:"contributors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || len(result.Data.Contributors) != 1 || result.Data.Contributors[0].UserID != 7 || result.Data.Contributors[0].Counts.Commits != 1 {
		t.Fatalf("boundary or tenant result = %s, %v", rec.Body, err)
	}
	if rec := h.do("PUT", "/v1/organizations/acme/scores/presets/first-hour", alice, []byte(query), map[string]string{"Origin": appOrigin}); rec.Code != 200 {
		t.Fatalf("save preset = %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("GET", "/v1/organizations/acme/scores/presets", bob, nil, nil); rec.Code != 404 {
		t.Fatalf("cross-org preset = %d", rec.Code)
	}
	if rec := h.do("GET", "/v1/organizations/acme/scores/presets", alice, nil, nil); rec.Code != 200 || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("list preset = %d %s", rec.Code, rec.Body)
	}
	publish(10, 10000, 7, "alice", start, start.Add(30*time.Minute))
	rec = h.do("POST", "/v1/organizations/acme/scores/query", alice, []byte(query), map[string]string{"Origin": appOrigin})
	if rec.Code != 200 {
		t.Fatalf("fresh query = %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Data.Contributors[0].Counts.Commits != 2 {
		t.Fatalf("new generation result = %s, %v", rec.Body, err)
	}
}
