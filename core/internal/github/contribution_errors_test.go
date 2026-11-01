package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

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
	err := c.ListContributions(context.Background(), 42, 99, "org", "repo", testWindow(t), models.ContributionCursor{}, func([]models.Contribution, models.ContributionCursor) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("callback failure swallowed: %v", err)
	}
}
