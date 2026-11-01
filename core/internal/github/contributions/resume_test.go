package contributions

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

func TestCollectResumesStopsEarlyAndSkipsGoneResources(t *testing.T) {
	var visits []string
	c, baseURL := contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	resume := models.ContributionCursor{Phase: models.PhasePulls, Next: baseURL + "/repos/org/repo/pulls?page=2"}
	got, cursors, err := collect(t, c, resume)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].EventID != "99:pr_open:8" || got[1].EventID != "99:issue_open:18" {
		t.Fatalf("events = %+v", got)
	}
	want := []models.ContributionCursor{{Phase: models.PhaseIssues}, {Phase: models.PhaseDone}}
	if !reflect.DeepEqual(cursors, want) {
		t.Fatalf("cursors = %+v, want %+v", cursors, want)
	}
	if strings.Join(visits, ",") != "/repos/org/repo/pulls?2,/repos/org/repo/pulls/3/reviews?,/repos/org/repo/issues?,/repos/org/repo/issues/5/events?" {
		t.Fatalf("visits = %v", visits)
	}
}

func TestCollectTreatsEmptyRepositoryAsNoCommits(t *testing.T) {
	c, _ := contributionClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/commits") {
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, `{"message":"Git Repository is empty."}`)
			return
		}
		_, _ = fmt.Fprint(w, `[]`)
	})
	_, cursors, err := collect(t, c, models.ContributionCursor{})
	if err != nil || len(cursors) == 0 || cursors[len(cursors)-1].Phase != models.PhaseDone {
		t.Fatalf("empty repository: cursors=%+v err=%v", cursors, err)
	}
}
