package httpapi_test

import (
	"testing"
)

func TestHealthAndReadiness(t *testing.T) {
	h := newHarness(t)
	if rec := h.do("GET", "/healthz", "", nil, nil); rec.Code != 200 {
		t.Fatalf("healthz = %d", rec.Code)
	}
	if rec := h.do("GET", "/readyz", "", nil, nil); rec.Code != 200 {
		t.Fatalf("readyz = %d", rec.Code)
	}
	h.pool.Close()
	if rec := h.do("GET", "/readyz", "", nil, nil); rec.Code != 503 {
		t.Fatalf("readyz with closed pool = %d, want 503", rec.Code)
	}
}
