package jobs_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"

	"github.com/upayanmazumder/gitorgasm/services/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/services/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/services/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/services/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/services/core/internal/testdb"
)

const instID = 10

// fakeGitHub serves the endpoints used by a sync. It checks App JWTs and installation tokens.
type fakeGitHub struct {
	key           *rsa.PrivateKey
	rateLimitPage int // when > 0, this repository page answers with a primary rate limit
	gone          bool
	tokenMints    atomic.Int32
}

func (f *fakeGitHub) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	appAuth := func(r *http.Request) bool {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		tok, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(*jwt.Token) (any, error) { return &f.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"RS256"}))
		if err != nil {
			return false
		}
		iss, _ := tok.Claims.GetIssuer()
		return iss == "42"
	}
	mux.HandleFunc("GET /app/installations/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !appAuth(r) {
			w.WriteHeader(401)
			return
		}
		if f.gone {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":%s,"repository_selection":"all","permissions":{"metadata":"read"},
			"account":{"id":100,"login":"acme","type":"Organization","avatar_url":"https://a.example/acme"}}`, r.PathValue("id"))
	})
	mux.HandleFunc("POST /app/installations/{id}/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		if !appAuth(r) {
			w.WriteHeader(401)
			return
		}
		f.tokenMints.Add(1)
		_, _ = fmt.Fprintf(w, `{"token":"ghs_test","expires_at":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	})
	mux.HandleFunc("GET /installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghs_test" || r.Header.Get("X-GitHub-Api-Version") != github.APIVersion {
			w.WriteHeader(401)
			return
		}
		pageNo, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if pageNo == 0 {
			pageNo = 1
		}
		if pageNo == f.rateLimitPage {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(2*time.Minute).Unix(), 10))
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		pages := map[int][]string{1: {"api", "web"}, 2: {"infra"}}
		var repos []map[string]any
		for i, n := range pages[pageNo] {
			repos = append(repos, map[string]any{
				"id": pageNo*100 + i, "name": n, "full_name": "acme/" + n, "owner": map[string]any{"id": 100, "login": "acme"},
				"private": true, "fork": false, "archived": false, "visibility": "private", "default_branch": "main",
				"stargazers_count": 3, "pushed_at": "2026-10-01T00:00:00Z", "updated_at": "2026-10-02T00:00:00Z",
			})
		}
		if pageNo == 1 {
			next := "http://" + r.Host + "/installation/repositories?per_page=100&page=2"
			w.Header().Set("Link", `<`+next+`>; rel="next", <`+next+`>; rel="last"`)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total_count": 3, "repository_selection": "all", "repositories": repos})
	})
	return mux
}

func setup(t *testing.T, f *fakeGitHub) (*pgxpool.Pool, *github.Client) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f.key = key
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	gh := github.New(&config.GitHub{AppID: 42, PrivateKey: key, APIBaseURL: srv.URL, WebBaseURL: srv.URL, RequestTimeout: 5 * time.Second})
	return testdb.New(t), gh
}

func activeRepos(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT name FROM repositories WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// seedStale stores a repository that GitHub no longer reports.
func seedStale(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	inst := &github.Installation{ID: instID, RepositorySelection: "all", Account: &github.Account{ID: 100, Login: "acme", Type: "Organization"}}
	if err := store.UpsertInstallation(ctx, pool, inst); err != nil {
		t.Fatal(err)
	}
	stale := []github.Repository{{ID: 999, Name: "removed", FullName: "acme/removed", Owner: github.Owner{ID: 100}}}
	if err := store.UpsertRepositories(ctx, pool, instID, stale, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func work(t *testing.T, w *jobs.SyncWorker) error {
	t.Helper()
	return w.Work(context.Background(), &river.Job[jobs.SyncInstallationArgs]{Args: jobs.SyncInstallationArgs{InstallationID: instID}})
}

func TestSyncMirrorsRepositoriesAndTombstonesRemoved(t *testing.T) {
	f := &fakeGitHub{}
	pool, gh := setup(t, f)
	seedStale(t, pool)
	w := &jobs.SyncWorker{Pool: pool, GitHub: gh, Now: time.Now}

	if err := work(t, w); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got := activeRepos(t, pool); strings.Join(got, ",") != "api,infra,web" {
		t.Fatalf("active repos = %v", got)
	}
	var seen int
	var lastErr *string
	if err := pool.QueryRow(context.Background(), `SELECT repositories_seen, last_error FROM sync_state WHERE installation_id = $1`, instID).Scan(&seen, &lastErr); err != nil {
		t.Fatal(err)
	}
	if seen != 3 || lastErr != nil {
		t.Fatalf("sync_state seen=%d err=%v", seen, lastErr)
	}

	// A second run is idempotent and reuses the cached installation token.
	if err := work(t, w); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if got := activeRepos(t, pool); len(got) != 3 {
		t.Fatalf("after second sync = %v", got)
	}
	if n := f.tokenMints.Load(); n != 1 {
		t.Fatalf("token mints = %d, want 1", n)
	}
}

func TestSyncSnoozesOnRateLimitWithoutTombstoning(t *testing.T) {
	f := &fakeGitHub{rateLimitPage: 2}
	pool, gh := setup(t, f)
	seedStale(t, pool)
	err := work(t, &jobs.SyncWorker{Pool: pool, GitHub: gh, Now: time.Now})

	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) || snooze.Duration < time.Minute || snooze.Duration > 3*time.Minute {
		t.Fatalf("err = %v, want snooze until the rate-limit reset", err)
	}
	// A partial pass must not delete repositories it did not reach.
	if got := activeRepos(t, pool); strings.Join(got, ",") != "api,removed,web" {
		t.Fatalf("active repos = %v", got)
	}
	var lastErr *string
	_ = pool.QueryRow(context.Background(), `SELECT last_error FROM sync_state WHERE installation_id = $1`, instID).Scan(&lastErr)
	if lastErr == nil || !strings.Contains(*lastErr, "rate limit") {
		t.Fatalf("last_error = %v", lastErr)
	}
}

func TestConcurrentSyncOfSameInstallationIsDeferred(t *testing.T) {
	f := &fakeGitHub{}
	pool, gh := setup(t, f)
	ctx := context.Background()
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	// Simulate a pass already running on another worker.
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended('gitorgasm:sync_installation:' || $1::bigint::text, 0))`, instID); err != nil {
		t.Fatal(err)
	}
	err = work(t, &jobs.SyncWorker{Pool: pool, GitHub: gh, Now: time.Now})
	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) {
		t.Fatalf("err = %v, want snooze while another pass holds the lock", err)
	}
	if f.tokenMints.Load() != 0 {
		t.Fatal("the deferred pass called GitHub")
	}
}

func TestSyncMarksUninstalledInstallationDeleted(t *testing.T) {
	f := &fakeGitHub{gone: true}
	pool, gh := setup(t, f)
	seedStale(t, pool)
	if err := work(t, &jobs.SyncWorker{Pool: pool, GitHub: gh, Now: time.Now}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	ids, err := store.ActiveInstallationIDs(context.Background(), pool)
	if err != nil || len(ids) != 0 {
		t.Fatalf("active installations = %v (%v), want none", ids, err)
	}
}

func TestWebhookWorkerAppliesEvents(t *testing.T) {
	f := &fakeGitHub{}
	pool, gh := setup(t, f)
	ctx := context.Background()
	seedStale(t, pool)
	userID, err := store.UpsertUser(ctx, pool, &github.User{ID: 7, Login: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceUserInstallations(ctx, pool, userID, []int64{instID}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(ctx, pool, store.NewSession{IDHash: []byte("h"), UserID: userID, KeyVersion: 1, TokenSealed: []byte{1}, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	tw := rivertest.NewWorker(t, riverpgxv5.New(pool), &river.Config{}, &jobs.WebhookWorker{Pool: pool, GitHub: gh})
	deliver := func(id, event, payload string) {
		t.Helper()
		if _, err := store.InsertWebhookDelivery(ctx, pool, store.WebhookDelivery{DeliveryID: id, Event: event, Payload: []byte(payload)}); err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		res, err := tw.Work(ctx, t, tx, jobs.ProcessWebhookArgs{DeliveryID: id}, nil)
		if err != nil || res.EventKind != river.EventKindJobCompleted {
			t.Fatalf("work %s: %v %+v", id, err, res)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}

	deliver("d-revoke", "github_app_authorization", `{"action":"revoked","sender":{"id":7}}`)
	var sessions, grants int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM user_installations`).Scan(&grants)
	if sessions != 0 || grants != 0 {
		t.Fatalf("after revoke sessions=%d grants=%d, want 0 and 0", sessions, grants)
	}

	// Restore the grant (the user signed in again), then uninstall the App.
	if err := store.ReplaceUserInstallations(ctx, pool, userID, []int64{instID}); err != nil {
		t.Fatal(err)
	}
	deliver("d-uninstall", "installation", `{"action":"deleted","installation":{"id":10,"repository_selection":"all","account":{"id":100,"login":"acme","type":"Organization"}}}`)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM user_installations`).Scan(&grants)
	if grants != 0 {
		t.Fatalf("grants after uninstall = %d, want 0", grants)
	}
	var processed int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE processed_at IS NOT NULL`).Scan(&processed)
	if processed != 2 {
		t.Fatalf("processed deliveries = %d, want 2", processed)
	}
}
