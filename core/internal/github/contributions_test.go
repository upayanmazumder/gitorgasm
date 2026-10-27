package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
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

func testWindow(t *testing.T) ContributionWindow {
	t.Helper()
	since := mustTime(t, contributionSince)
	return ContributionWindow{CommitsSince: since, ActivitySince: since, Until: mustTime(t, contributionUntil)}
}

// contributionWindow collects unique events, as idempotent storage does, and the cursors seen.
func contributionWindow(t *testing.T, c *Client) ([]Contribution, error) {
	t.Helper()
	got, _, err := collect(t, c, ContributionCursor{})
	return got, err
}

func collect(t *testing.T, c *Client, from ContributionCursor) ([]Contribution, []ContributionCursor, error) {
	t.Helper()
	var got []Contribution
	var cursors []ContributionCursor
	seen := map[string]bool{}
	err := c.ListContributions(context.Background(), 42, 99, "org", "repo", testWindow(t), from, func(page []Contribution, next ContributionCursor) error {
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
	want := map[string]Contribution{
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

func TestListContributionsFailsOnAnyPage(t *testing.T) {
	for _, failure := range []string{"commits", "pulls", "reviews", "issues", "events"} {
		t.Run(failure, func(t *testing.T) {
			c := contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				if strings.HasSuffix(path, "/"+failure) && r.URL.Query().Get("page") == "2" {
					w.WriteHeader(http.StatusForbidden)
					_, _ = fmt.Fprint(w, `{"message":"Resource not accessible by integration"}`)
					return
				}
				if r.URL.Query().Get("page") == "" {
					w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=2>; rel="next"`, r.Host, path))
				}
				switch {
				case strings.HasSuffix(path, "/commits"):
					_, _ = fmt.Fprint(w, `[]`)
				case strings.HasSuffix(path, "/pulls"):
					_, _ = fmt.Fprint(w, `[{"id":1,"number":1,"user":{"id":1,"login":"alice"},"created_at":"2026-10-26T01:00:00Z","updated_at":"2026-10-26T02:00:00Z"}]`)
				case strings.HasSuffix(path, "/reviews"):
					_, _ = fmt.Fprint(w, `[]`)
				case strings.HasSuffix(path, "/issues"):
					_, _ = fmt.Fprint(w, `[{"id":2,"number":2,"user":{"id":1,"login":"alice"},"created_at":"2026-10-26T01:00:00Z","updated_at":"2026-10-26T02:00:00Z"}]`)
				case strings.HasSuffix(path, "/events"):
					_, _ = fmt.Fprint(w, `[]`)
				default:
					t.Errorf("unexpected %s", path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			_, err := contributionWindow(t, c)
			if err == nil {
				t.Fatal("partial window returned nil error")
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Kind != KindForbidden {
				t.Fatalf("failure was not the API error: %v", err)
			}
		})
	}
}

func TestListContributionsRejectsMalformedPageAndCallbackFailure(t *testing.T) {
	c := contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `null`)
	})
	if _, err := contributionWindow(t, c); err == nil {
		t.Fatal("null list page accepted as a complete page")
	}
	c = contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/commits") {
			_, _ = fmt.Fprint(w, `[{"sha":"abc","author":{"id":1,"login":"alice"},"commit":{"committer":{"date":"2026-10-26T01:00:00Z"}}}]`)
		} else {
			_, _ = fmt.Fprint(w, `[]`)
		}
	})
	failure := errors.New("storage failed")
	err := c.ListContributions(context.Background(), 42, 99, "org", "repo", testWindow(t), ContributionCursor{}, func([]Contribution, ContributionCursor) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("callback failure swallowed: %v", err)
	}
}

func TestListContributionsResumesStopsEarlyAndSkipsGoneResources(t *testing.T) {
	var visits []string
	c := contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
		visits = append(visits, r.URL.Path+"?"+r.URL.Query().Get("page"))
		switch r.URL.Path + "?" + r.URL.Query().Get("page") {
		case "/repos/org/repo/pulls?2":
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/repos/org/repo/pulls?page=3>; rel="next"`, r.Host))
			_, _ = fmt.Fprint(w, `[{"id":8,"number":3,"user":{"id":1,"login":"alice"},"created_at":"2026-10-26T11:00:00Z","updated_at":"2026-10-26T17:00:00Z"},{"id":7,"number":2,"user":{"id":1,"login":"alice"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-10-25T00:00:00Z"}]`)
		case "/repos/org/repo/pulls/3/reviews?":
			_, _ = fmt.Fprint(w, `[]`)
		case "/repos/org/repo/issues?":
			_, _ = fmt.Fprint(w, `[{"id":18,"number":5,"user":{"id":3,"login":"carol"},"created_at":"2026-10-26T00:00:00Z","updated_at":"2026-10-26T20:00:00Z"}]`)
		case "/repos/org/repo/issues/5/events?":
			w.WriteHeader(http.StatusGone)
			_, _ = fmt.Fprint(w, `{"message":"Gone"}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	resume := ContributionCursor{Phase: PhasePulls, Next: c.cfg.APIBaseURL + "/repos/org/repo/pulls?page=2"}
	got, cursors, err := collect(t, c, resume)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].EventID != "99:pr_open:8" || got[1].EventID != "99:issue_open:18" {
		t.Fatalf("events = %+v", got)
	}
	want := []ContributionCursor{{Phase: PhaseIssues}, {Phase: PhaseDone}}
	if !reflect.DeepEqual(cursors, want) {
		t.Fatalf("cursors = %+v, want %+v", cursors, want)
	}
	if strings.Join(visits, ",") != "/repos/org/repo/pulls?2,/repos/org/repo/pulls/3/reviews?,/repos/org/repo/issues?,/repos/org/repo/issues/5/events?" {
		t.Fatalf("visits = %v", visits)
	}
}

func TestListContributionsTreatsEmptyRepositoryAsNoCommits(t *testing.T) {
	c := contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/commits") {
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, `{"message":"Git Repository is empty."}`)
			return
		}
		_, _ = fmt.Fprint(w, `[]`)
	})
	_, cursors, err := collect(t, c, ContributionCursor{})
	if err != nil || len(cursors) == 0 || cursors[len(cursors)-1].Phase != PhaseDone {
		t.Fatalf("empty repository: cursors=%+v err=%v", cursors, err)
	}
}
