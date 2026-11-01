package jobs_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/sync"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

func TestSyncMirrorsRepositoriesAndTombstonesRemoved(t *testing.T) {
	f := &fakeGitHub{}
	pool, gh := setup(t, f)
	seedStale(t, pool)
	w := &sync.Worker{Pool: pool, GitHub: gh, Now: time.Now}

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
	err := work(t, &sync.Worker{Pool: pool, GitHub: gh, Now: time.Now})

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
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended('gitorgasm:sync_installation:' || $1::bigint::text, 0))`, instID); err != nil {
		t.Fatal(err)
	}
	err = work(t, &sync.Worker{Pool: pool, GitHub: gh, Now: time.Now})
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
	if err := work(t, &sync.Worker{Pool: pool, GitHub: gh, Now: time.Now}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	ids, err := tenancy.ActiveInstallationIDs(context.Background(), pool)
	if err != nil || len(ids) != 0 {
		t.Fatalf("active installations = %v (%v), want none", ids, err)
	}
}
