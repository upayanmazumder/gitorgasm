package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestScorePresetsRespectTenantsAndNamedPeriods(t *testing.T) {
	h, alice, bob, start, _ := newScoreFixture(t)
	query := fmt.Sprintf(`{"period":{"kind":"custom","start":%q,"end":%q},"filter":{"search":"alice"}}`, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339))
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
	if rec := h.do("PUT", "/v1/organizations/acme/scores/presets/first-hour", alice, []byte(query), map[string]string{"Origin": appOrigin}); rec.Code != 200 {
		t.Fatalf("save preset = %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("GET", "/v1/organizations/acme/scores/presets", bob, nil, nil); rec.Code != 404 {
		t.Fatalf("cross-org preset = %d", rec.Code)
	}
	if rec := h.do("GET", "/v1/organizations/acme/scores/presets", alice, nil, nil); rec.Code != 200 || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("list preset = %d %s", rec.Code, rec.Body)
	}

	// Named periods resolve against the latest ingest; saved presets do not pin an anchor.
	named := `{"period":{"kind":"preset","preset":"last7_days","as_of":"2020-01-01T00:00:00Z"}}`
	rec := h.do("PUT", "/v1/organizations/acme/scores/presets/%20week%20", alice, []byte(named), map[string]string{"Origin": appOrigin})
	if rec.Code != 200 || bytes.Contains(rec.Body.Bytes(), []byte("as_of")) || !bytes.Contains(rec.Body.Bytes(), []byte(`"name":"week"`)) {
		t.Fatalf("named preset = %d %s", rec.Code, rec.Body)
	}
	rec = h.do("POST", "/v1/organizations/acme/scores/query", alice, []byte(`{"period":{"kind":"preset","preset":"last7_days"}}`), map[string]string{"Origin": appOrigin})
	if err := json.Unmarshal(rec.Body.Bytes(), &result); rec.Code != 200 || err != nil || result.Data.Contributors[0].Counts.Commits != 2 {
		t.Fatalf("named period = %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("DELETE", "/v1/organizations/acme/scores/presets/week%20", alice, nil, map[string]string{"Origin": appOrigin}); rec.Code != 204 {
		t.Fatalf("delete trimmed preset = %d %s", rec.Code, rec.Body)
	}
}
