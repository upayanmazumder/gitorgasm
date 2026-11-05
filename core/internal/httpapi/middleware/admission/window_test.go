package admission_test

import (
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/admission"
)

func TestWindowLimiterIsPerKeyAndSlides(t *testing.T) {
	now := time.Unix(1000, 0)
	l := admission.NewWindowLimiter(2, time.Minute, func() time.Time { return now })
	for range 2 {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatal("refused below limit")
		}
	}
	ok, wait := l.Allow("a")
	if ok || wait != time.Minute {
		t.Fatalf("over limit = %v, wait %v", ok, wait)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("another key was limited")
	}
	now = now.Add(time.Minute)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("window did not slide")
	}
	l.Refund("a")
	l.Refund("a")
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("refund did not free a slot")
	}
}

func TestTTLCacheExpiresAndBounds(t *testing.T) {
	now := time.Unix(1000, 0)
	c := admission.NewTTLCache[int](time.Minute, 2, func() time.Time { return now })
	c.Set("a", 1)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatal("miss before expiry")
	}
	now = now.Add(time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("hit after expiry")
	}
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Fatal("newest entry evicted")
	}
}
