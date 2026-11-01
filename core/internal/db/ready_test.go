package db

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"testing"
	"time"
)

func testURL(t *testing.T) *url.URL {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; run make test")
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// forwardAfter accepts connections on addr after delay and relays them to target.
func forwardAfter(t *testing.T, addr, target string, delay time.Duration) {
	t.Helper()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-time.After(delay):
		case <-done:
			return
		}
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		go func() { <-done; _ = l.Close() }()
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				up, err := net.Dial("tcp", target)
				if err != nil {
					return
				}
				defer func() { _ = up.Close() }()
				go func() { _, _ = io.Copy(up, c) }()
				_, _ = io.Copy(c, up)
			}()
		}
	}()
}

func TestOpenWaitsForDatabaseThatStartsLate(t *testing.T) {
	u := testURL(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	const delay = 1500 * time.Millisecond
	forwardAfter(t, addr, u.Host, delay)
	late := *u
	late.Host = addr

	started := time.Now()
	pool, err := Open(context.Background(), late.String(), Limits{MaxConns: 1})
	if err != nil {
		t.Fatalf("Open should wait for the database: %v", err)
	}
	defer pool.Close()
	if elapsed := time.Since(started); elapsed < delay {
		t.Fatalf("Open returned after %v, before the database was reachable", elapsed)
	}
}

func TestOpenFailsFastOnWrongPassword(t *testing.T) {
	u := testURL(t)
	bad := *u
	bad.User = url.UserPassword(u.User.Username(), "not-the-password")

	started := time.Now()
	pool, err := Open(context.Background(), bad.String(), Limits{MaxConns: 1})
	if err == nil {
		pool.Close()
		t.Fatal("wrong password must fail")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("auth failure took %v; it must not be retried", elapsed)
	}
}

func TestRetryUntilReadyStopsOnNonRetryableError(t *testing.T) {
	calls := 0
	fatal := errors.New("bad credentials")
	err := retryUntilReady(context.Background(),
		func(context.Context) error { calls++; return fatal },
		func(error) bool { return false }, time.Millisecond)
	if !errors.Is(err, fatal) || calls != 1 {
		t.Fatalf("err = %v after %d calls, want one call and the original error", err, calls)
	}
}

func TestRetryUntilReadyReportsLastErrorAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	want := errors.New("connection refused")
	err := retryUntilReady(ctx,
		func(context.Context) error { return want },
		func(error) bool { return true }, time.Millisecond)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want the last ping error", err)
	}
}
