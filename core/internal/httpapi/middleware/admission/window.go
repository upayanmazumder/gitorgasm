package admission

import (
	"sync"
	"time"
)

// WindowLimiter allows at most limit events per key within a sliding window. State is
// in-process, so with N API replicas the effective ceiling is N times the limit.
type WindowLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time
	mu     sync.Mutex
	events map[string][]time.Time
}

func NewWindowLimiter(limit int, window time.Duration, now func() time.Time) *WindowLimiter {
	return &WindowLimiter{limit: limit, window: window, now: now, events: map[string][]time.Time{}}
}

// Allow records one event for key. When refused, retryAfter is the wait until a slot frees.
func (l *WindowLimiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, times := range l.events { // Bounds memory: drop keys whose events all expired.
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= l.window {
			delete(l.events, k)
		}
	}
	times := l.events[key]
	for len(times) > 0 && now.Sub(times[0]) >= l.window {
		times = times[1:]
	}
	if len(times) >= l.limit {
		l.events[key] = times
		return false, l.window - now.Sub(times[0])
	}
	l.events[key] = append(times, now)
	return true, 0
}

// Refund removes the most recent event for key, e.g. after a failed upstream call that
// did not spend shared quota.
func (l *WindowLimiter) Refund(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if times := l.events[key]; len(times) > 0 {
		l.events[key] = times[:len(times)-1]
	}
}

// TTLCache holds values for a fixed duration, with a hard entry bound.
type TTLCache[V any] struct {
	ttl     time.Duration
	max     int
	now     func() time.Time
	mu      sync.Mutex
	entries map[string]ttlEntry[V]
}

type ttlEntry[V any] struct {
	value   V
	expires time.Time
}

func NewTTLCache[V any](ttl time.Duration, max int, now func() time.Time) *TTLCache[V] {
	return &TTLCache[V]{ttl: ttl, max: max, now: now, entries: map[string]ttlEntry[V]{}}
}

func (c *TTLCache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !c.now().Before(e.expires) {
		delete(c.entries, key)
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *TTLCache[V]) Set(key string, value V) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
		for k := range c.entries { // Still full: evict arbitrary entries; the cache is only an optimization.
			if len(c.entries) < c.max {
				break
			}
			delete(c.entries, k)
		}
	}
	c.entries[key] = ttlEntry[V]{value, now.Add(c.ttl)}
}
