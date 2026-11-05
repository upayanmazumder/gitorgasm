package scorer

import (
	"context"
	"sync"
	"time"
)

type tenantKind uint8

const (
	kindUser tenantKind = iota + 1
	kindInstallation
)

type tenantKey struct {
	kind tenantKind
	id   int64
}

// tenantSlots is a counting semaphore for one tenant; refs counts holders and waiters so idle entries are removed.
type tenantSlots struct {
	slots chan struct{}
	refs  int
}

// tenantLimiter bounds how many scorer slots one user or one installation can hold at once.
type tenantLimiter struct {
	limit int
	mu    sync.Mutex
	held  map[tenantKey]*tenantSlots
}

func newTenantLimiter(limit int) *tenantLimiter {
	return &tenantLimiter{limit: limit, held: make(map[tenantKey]*tenantSlots)}
}

// acquire waits for one of key's slots until ctx ends or deadline fires (ErrBusy).
func (l *tenantLimiter) acquire(ctx context.Context, deadline <-chan time.Time, key tenantKey) (func(), error) {
	l.mu.Lock()
	entry := l.held[key]
	if entry == nil {
		entry = &tenantSlots{slots: make(chan struct{}, l.limit)}
		l.held[key] = entry
	}
	entry.refs++
	l.mu.Unlock()
	unref := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if entry.refs--; entry.refs == 0 {
			delete(l.held, key)
		}
	}
	select {
	case entry.slots <- struct{}{}:
		return func() { <-entry.slots; unref() }, nil
	case <-ctx.Done():
		unref()
		return nil, ctx.Err()
	case <-deadline:
		unref()
		return nil, ErrBusy
	}
}
