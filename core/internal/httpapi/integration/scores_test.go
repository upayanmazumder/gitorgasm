package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestScoreQueriesRespectTenantPeriodsAndGeneration(t *testing.T) {
	h, alice, _, start, publish := newScoreFixture(t)
	ctx := context.Background()
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
	publish(10, 10000, 7, "alice", start, start.Add(30*time.Minute))
	rec = h.do("POST", "/v1/organizations/acme/scores/query", alice, []byte(query), map[string]string{"Origin": appOrigin})
	if rec.Code != 200 {
		t.Fatalf("fresh query = %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Data.Contributors[0].Counts.Commits != 2 {
		t.Fatalf("new generation result = %s, %v", rec.Body, err)
	}

	// Key order does not create a second cache entry.
	reordered := fmt.Sprintf(`{"filter":{"search":"alice"},"period":{"end":%q,"start":%q,"kind":"custom"}}`, start.Add(time.Hour).Format(time.RFC3339), start.Format(time.RFC3339))
	if rec := h.do("POST", "/v1/organizations/acme/scores/query", alice, []byte(reordered), map[string]string{"Origin": appOrigin}); rec.Code != 200 {
		t.Fatalf("reordered query = %d %s", rec.Code, rec.Body)
	}
	var snapshots int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM score_snapshots WHERE installation_id = 10 AND query_key <> 'default'`).Scan(&snapshots); err != nil || snapshots != 1 {
		t.Fatalf("custom snapshots = %d, %v; want 1", snapshots, err)
	}
}
