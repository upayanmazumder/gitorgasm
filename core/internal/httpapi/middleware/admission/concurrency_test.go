package admission_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/admission"
)

func TestLimitConcurrentRejectsAtOnceWhenFull(t *testing.T) {
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	h := admission.LimitConcurrent(1, func(w http.ResponseWriter, _ *http.Request) {
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

	second := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		h.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/", nil))
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("request queued behind a full limiter instead of failing at once")
	}
	if second.Code != http.StatusServiceUnavailable || calls.Load() != 1 {
		t.Fatalf("full limiter: status=%d calls=%d", second.Code, calls.Load())
	}

	close(release)
	<-done
	third := httptest.NewRecorder()
	h.ServeHTTP(third, httptest.NewRequest(http.MethodPost, "/", nil))
	if calls.Load() != 2 || first.Code != http.StatusNoContent || third.Code != http.StatusNoContent {
		t.Fatalf("slot not released: calls=%d statuses=%d,%d", calls.Load(), first.Code, third.Code)
	}
}
