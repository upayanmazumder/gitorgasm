package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware"
)

func TestLimitConcurrentStopsCanceledWaiter(t *testing.T) {
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	h := middleware.LimitConcurrent(1, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		w.WriteHeader(http.StatusNoContent)
	})
	first := httptest.NewRecorder()
	go func() {
		defer close(done)
		h.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/", nil))
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx))
	close(release)
	<-done
	if calls.Load() != 1 || second.Body.Len() != 0 {
		t.Fatalf("canceled waiter dispatched: calls=%d body=%q", calls.Load(), second.Body.String())
	}
	third := httptest.NewRecorder()
	h.ServeHTTP(third, httptest.NewRequest(http.MethodPost, "/", nil))
	if calls.Load() != 2 || first.Code != http.StatusNoContent || third.Code != http.StatusNoContent {
		t.Fatalf("slot not released: calls=%d statuses=%d,%d", calls.Load(), first.Code, third.Code)
	}
}
