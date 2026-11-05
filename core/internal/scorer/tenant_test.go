package scorer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeBinary blocks every invocation whose stdin contains "hold" until the returned release func runs.
func fakeBinary(t *testing.T) (bin string, release func()) {
	t.Helper()
	dir := t.TempDir()
	gate := filepath.Join(dir, "release")
	bin = filepath.Join(dir, "score")
	script := "#!/bin/sh\ninput=$(cat)\ncase \"$input\" in\n*hold*) while [ ! -e '" + gate + "' ]; do sleep 0.01; done;;\nesac\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // test helper script
		t.Fatal(err)
	}
	var once bool
	release = func() {
		if !once {
			once = true
			if err := os.WriteFile(gate, nil, 0o600); err != nil {
				t.Error(err)
			}
		}
	}
	t.Cleanup(release)
	return bin, release
}

var (
	hold = json.RawMessage(`{"hold":true}`)
	fast = json.RawMessage(`{}`)
)

// startHolder runs a blocking Validate and returns once it occupies its slots.
func startHolder(t *testing.T, r *Runner, installationID, userID int64) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- r.Validate(context.Background(), installationID, userID, hold) }()
	for deadline := time.Now().Add(2 * time.Second); !r.holds(installationID, userID); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("holder never took its slots")
		}
	}
	return done
}

// holds reports whether a tenant limiter entry exists with a taken slot for the given tenant.
func (r *Runner) holds(installationID, userID int64) bool {
	for _, c := range []struct {
		l   *tenantLimiter
		key tenantKey
	}{{r.users, tenantKey{kindUser, userID}}, {r.installations, tenantKey{kindInstallation, installationID}}} {
		c.l.mu.Lock()
		e := c.l.held[c.key]
		taken := e != nil && len(e.slots) > 0
		c.l.mu.Unlock()
		if !taken {
			return false
		}
	}
	return len(r.slots) > 0
}

func TestOneTenantCannotTakeBothSlots(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		holdInst, holdUser     int64
		greedyInst, greedyUser int64
		otherInst, otherUser   int64
	}{
		{name: "same user", holdInst: 10, holdUser: 1, greedyInst: 11, greedyUser: 1, otherInst: 12, otherUser: 2},
		{name: "same installation", holdInst: 10, holdUser: 1, greedyInst: 10, greedyUser: 2, otherInst: 11, otherUser: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin, release := fakeBinary(t)
			r := New(bin, 2, 150*time.Millisecond)
			holder := startHolder(t, r, tc.holdInst, tc.holdUser)

			begin := time.Now()
			err := r.Validate(context.Background(), tc.greedyInst, tc.greedyUser, fast)
			if !errors.Is(err, ErrBusy) {
				t.Fatalf("second request from the same tenant = %v, want ErrBusy", err)
			}
			if len(r.slots) != 1 {
				t.Fatalf("process slots in use = %d, want 1 (one slot must stay free)", len(r.slots))
			}
			if time.Since(begin) > time.Second {
				t.Fatalf("busy took %s", time.Since(begin))
			}
			// Another tenant gets the free slot while the first tenant's call is still running.
			if err := r.Validate(context.Background(), tc.otherInst, tc.otherUser, fast); err != nil {
				t.Fatalf("other tenant = %v, want success", err)
			}
			release()
			if err := <-holder; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTenantWaiterRunsWhenHolderFinishesInTime(t *testing.T) {
	bin, release := fakeBinary(t)
	r := New(bin, 2, 5*time.Second)
	holder := startHolder(t, r, 10, 1)
	result := make(chan error, 1)
	go func() { result <- r.Validate(context.Background(), 10, 1, fast) }()
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-result:
		t.Fatalf("waiter ran while its tenant held a slot: %v", err)
	default:
	}
	release()
	if err := <-holder; err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("waiter = %v", err)
	}
}

func TestTenantEntriesAreRemovedWhenIdle(t *testing.T) {
	bin, release := fakeBinary(t)
	r := New(bin, 2, 5*time.Second)
	holder := startHolder(t, r, 10, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := r.Validate(ctx, 10, 1, fast); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter = %v, want deadline exceeded", err)
	}
	release()
	if err := <-holder; err != nil {
		t.Fatal(err)
	}
	for _, l := range []*tenantLimiter{r.users, r.installations} {
		l.mu.Lock()
		n := len(l.held)
		l.mu.Unlock()
		if n != 0 || len(r.slots) != 0 {
			t.Fatalf("leaked entries=%d process slots=%d", n, len(r.slots))
		}
	}
}
