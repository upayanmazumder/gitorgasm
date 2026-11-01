package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryUntilReadyWaitsForDatabase(t *testing.T) {
	calls := 0
	ping := func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	}
	if err := retryUntilReady(context.Background(), ping, time.Millisecond); err != nil {
		t.Fatalf("ready after two failures: %v", err)
	}
	if calls != 3 {
		t.Fatalf("pings = %d, want 3", calls)
	}
}

func TestRetryUntilReadyReportsLastErrorAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	want := errors.New("connection refused")
	err := retryUntilReady(ctx, func(context.Context) error { return want }, time.Millisecond)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want the last ping error", err)
	}
}
