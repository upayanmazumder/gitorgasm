package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
)

// countingScorer wraps the real binary and appends a line to the returned log per --validate-query run.
func countingScorer(t *testing.T) (*scorer.Runner, func() int) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "validations")
	wrapper := filepath.Join(dir, "score")
	script := "#!/bin/sh\nif [ \"$1\" = \"--validate-query\" ]; then echo v >> '" + log + "'; fi\nexec '" + os.Getenv("SCORING_BIN") + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil { //nolint:gosec // test wrapper script
		t.Fatal(err)
	}
	count := func() int {
		data, err := os.ReadFile(log) //nolint:gosec // path built in this test
		if os.IsNotExist(err) {
			return 0
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(data), "v\n")
	}
	return scorer.New(wrapper, 2, 5*time.Second), count
}

func TestSavingUnchangedPresetSkipsValidation(t *testing.T) {
	runner, validations := countingScorer(t)
	h := newHarnessWithScorer(t, &config.GitHub{
		WebhookSecret: []byte(webhookSecret), APIBaseURL: "http://127.0.0.1:1", WebBaseURL: "http://127.0.0.1:1",
	}, runner)
	alice := h.seedTenant(1, "acme", 100, 10, "api")
	save := func(body string) int {
		t.Helper()
		rec := h.do("PUT", "/v1/organizations/acme/scores/presets/mine", alice, []byte(body), map[string]string{"Origin": appOrigin})
		if rec.Code != 200 {
			t.Fatalf("save preset = %d %s", rec.Code, rec.Body)
		}
		return validations()
	}
	if n := save(`{"period":{"kind":"custom","start":"2026-01-01T00:00:00Z","end":"2026-01-02T00:00:00Z"},"filter":{"search":"alice"}}`); n != 1 {
		t.Fatalf("first save validated %d times, want 1", n)
	}
	// Same body with another key order and whitespace.
	if n := save(`{ "filter": { "search": "alice" }, "period": { "end": "2026-01-02T00:00:00Z", "start": "2026-01-01T00:00:00Z", "kind": "custom" } }`); n != 1 {
		t.Fatalf("unchanged save validated again: %d runs", n)
	}
	if n := save(`{"filter":{"search":"bob"}}`); n != 2 {
		t.Fatalf("changed save validated %d times in total, want 2", n)
	}
	if rec := h.do("PUT", "/v1/organizations/acme/scores/presets/mine", alice, []byte(`{"filter":{"preset":"nope"}}`), map[string]string{"Origin": appOrigin}); rec.Code != 400 {
		t.Fatalf("invalid changed body = %d %s", rec.Code, rec.Body)
	}
}
