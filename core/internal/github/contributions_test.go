package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

const contributionSince = "2026-10-26T00:00:00Z"
const contributionUntil = "2026-10-27T00:00:00Z"

func contributionClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(&config.GitHub{APIBaseURL: srv.URL, RequestTimeout: time.Second})
	c.tokens[42] = installationToken{value: "installation-token", expiresAt: time.Now().Add(time.Hour)}
	return c
}

func testWindow(t *testing.T) models.ContributionWindow {
	t.Helper()
	since := mustTime(t, contributionSince)
	return models.ContributionWindow{CommitsSince: since, ActivitySince: since, Until: mustTime(t, contributionUntil)}
}

// contributionWindow collects unique events, as idempotent storage does, and the cursors seen.
func contributionWindow(t *testing.T, c *Client) ([]models.Contribution, error) {
	t.Helper()
	got, _, err := collect(t, c, models.ContributionCursor{})
	return got, err
}

func collect(t *testing.T, c *Client, from models.ContributionCursor) ([]models.Contribution, []models.ContributionCursor, error) {
	t.Helper()
	var got []models.Contribution
	var cursors []models.ContributionCursor
	seen := map[string]bool{}
	err := c.ListContributions(context.Background(), 42, 99, "org", "repo", testWindow(t), from, func(page []models.Contribution, next models.ContributionCursor) error {
		for _, event := range page {
			if !seen[event.EventID] {
				seen[event.EventID] = true
				got = append(got, event)
			}
		}
		cursors = append(cursors, next)
		return nil
	})
	return got, cursors, err
}

func TestListContributionsPagesAndDateEdges(t *testing.T) {
	visits := make(map[string]int)
	var serverURL string
	c := contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer installation-token" || r.Header.Get("X-GitHub-Api-Version") != APIVersion {
			t.Errorf("missing installation authentication or API version: %v", r.Header)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		key := r.URL.Path + "?" + r.URL.Query().Get("page")
		visits[key]++
		if r.URL.Query().Get("page") == "2" {
			w.Header().Set("Link", "")
		} else if strings.HasSuffix(r.URL.Path, "/commits") || strings.HasSuffix(r.URL.Path, "/pulls") || strings.HasSuffix(r.URL.Path, "/issues") || strings.HasSuffix(r.URL.Path, "/reviews") || strings.HasSuffix(r.URL.Path, "/events") {
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=2>; rel="next"`, serverURL, r.URL.Path))
		}
		switch key {
		case "/repos/org/repo/commits?":
			_, _ = fmt.Fprint(w, `[{"sha":"same","author":{"id":1,"login":"alice","type":"User"},"commit":{"committer":{"date":"2026-10-26T00:00:00Z"}}},{"sha":"orphan","author":null,"commit":{"committer":{"date":"2026-10-26T12:00:00Z"}}}]`)
		case "/repos/org/repo/commits?2":
			_, _ = fmt.Fprint(w, `[{"sha":"late","author":{"id":2,"login":"bob"},"commit":{"committer":{"date":"2026-10-27T00:00:00Z"}}},{"sha":"same","author":{"id":1,"login":"alice"},"commit":{"committer":{"date":"2026-10-26T00:00:00Z"}}}]`)
		case "/repos/org/repo/pulls?":
			_, _ = fmt.Fprint(w, `[{"id":8,"number":3,"user":{"id":1,"login":"alice"},"created_at":"2026-10-26T11:00:00Z","updated_at":"2026-10-26T17:00:00Z","merged_at":"2026-10-26T17:00:00Z"}]`)
		case "/repos/org/repo/pulls?2":
			_, _ = fmt.Fprint(w, `[{"id":9,"number":4,"user":null,"created_at":"2026-10-26T12:00:00Z","updated_at":"2026-10-26T18:00:00Z"}]`)
		case "/repos/org/repo/pulls/3/reviews?":
			_, _ = fmt.Fprint(w, `[{"id":12,"user":{"id":2,"login":"bot[bot]","type":"Bot"},"state":"APPROVED","submitted_at":"2026-10-26T13:00:00Z"},{"id":13,"user":null,"submitted_at":"2026-10-26T14:00:00Z"}]`)
		case "/repos/org/repo/pulls/3/reviews?2", "/repos/org/repo/pulls/4/reviews?", "/repos/org/repo/pulls/4/reviews?2":
			_, _ = fmt.Fprint(w, `[]`)
		case "/repos/org/repo/issues?":
			_, _ = fmt.Fprint(w, `[{"id":8,"number":3,"pull_request":{},"user":{"id":1,"login":"alice"},"created_at":"2026-10-26T01:00:00Z","updated_at":"2026-10-26T17:00:00Z"},{"id":18,"number":5,"user":{"id":3,"login":"carol"},"created_at":"2026-10-26T00:00:00Z","updated_at":"2026-10-26T20:00:00Z"}]`)
		case "/repos/org/repo/issues?2":
			_, _ = fmt.Fprint(w, `[{"id":19,"number":6,"user":null,"created_at":"2026-10-26T01:00:00Z","updated_at":"2026-10-26T18:00:00Z"}]`)
		case "/repos/org/repo/issues/5/events?":
			_, _ = fmt.Fprint(w, `[{"id":31,"event":"closed","actor":{"id":4,"login":"closer"},"created_at":"2026-10-26T21:00:00Z"},{"id":32,"event":"closed","actor":null,"created_at":"2026-10-26T22:00:00Z"}]`)
		case "/repos/org/repo/issues/5/events?2":
			_, _ = fmt.Fprint(w, `[{"id":33,"event":"reopened","actor":{"id":3,"login":"carol"},"created_at":"2026-10-26T22:10:00Z"},{"id":34,"event":"closed","actor":{"id":4,"login":"closer"},"created_at":"2026-10-26T23:00:00Z"}]`)
		case "/repos/org/repo/issues/6/events?", "/repos/org/repo/issues/6/events?2":
			_, _ = fmt.Fprint(w, `[]`)
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	serverURL = c.cfg.APIBaseURL
	got, err := contributionWindow(t, c)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]models.Contribution{
		"99:commit:same":    {EventID: "99:commit:same", RepositoryID: 99, UserID: 1, Login: "alice", Kind: "commit"},
		"99:pr_open:8":      {EventID: "99:pr_open:8", RepositoryID: 99, UserID: 1, Login: "alice", Kind: "pr_open"},
		"99:pr_merge:8":     {EventID: "99:pr_merge:8", RepositoryID: 99, UserID: 1, Login: "alice", Kind: "pr_merge"},
		"99:review:12":      {EventID: "99:review:12", RepositoryID: 99, UserID: 2, Login: "bot[bot]", Kind: "review", IsBot: true},
		"99:issue_open:18":  {EventID: "99:issue_open:18", RepositoryID: 99, UserID: 3, Login: "carol", Kind: "issue_open"},
		"99:issue_close:31": {EventID: "99:issue_close:31", RepositoryID: 99, UserID: 4, Login: "closer", Kind: "issue_close"},
		"99:issue_close:34": {EventID: "99:issue_close:34", RepositoryID: 99, UserID: 4, Login: "closer", Kind: "issue_close"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d contributions: %+v; want %d", len(got), got, len(want))
	}
	for _, event := range got {
		expected, ok := want[event.EventID]
		if !ok {
			t.Errorf("duplicate or unexpected event %+v", event)
			continue
		}
		if event.OccurredAt.IsZero() || !event.OccurredAt.Before(mustTime(t, contributionUntil)) || event.OccurredAt.Before(mustTime(t, contributionSince)) {
			t.Errorf("event outside window: %+v", event)
		}
		event.OccurredAt = time.Time{}
		if !reflect.DeepEqual(event, expected) {
			t.Errorf("got %+v, want %+v", event, expected)
		}
		delete(want, event.EventID)
	}
	if len(want) != 0 {
		t.Errorf("missing events: %+v", want)
	}
	for _, path := range []string{"/repos/org/repo/commits?2", "/repos/org/repo/pulls?2", "/repos/org/repo/pulls/3/reviews?2", "/repos/org/repo/issues?2", "/repos/org/repo/issues/5/events?2"} {
		if visits[path] != 1 {
			t.Errorf("pagination skipped %s: %d visits", path, visits[path])
		}
	}
}

func mustTime(t *testing.T, text string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, text)
	if err != nil {
		t.Fatal(err)
	}
	return at
}
