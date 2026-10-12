package lifecycle

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// activityInterval is how often one owner's activity is written at most.
	activityInterval = time.Hour

	// pruneAt is the size the in-process throttle is allowed to reach before stale
	// entries are dropped. Owners are few; this only bounds a long-lived process.
	pruneAt = 4096
)

// ActivityStore writes an owner's last-active stamp. It reports whether a row
// changed; a row is skipped when the database already holds a stamp under an hour
// old, which is how the throttle holds across instances.
type ActivityStore interface {
	TouchOwnerLastActive(ctx context.Context, id uuid.UUID) (bool, error)
}

// ActivityTracker records when an owner last made a request. It sits on the
// request path, so it does no I/O there: the write runs on the background runner,
// and a failure is logged and dropped. The inactive-owner nudge reads the stamp
// only to the nearest hour, so losing one write costs nothing.
type ActivityTracker struct {
	store      ActivityStore
	background func(func())
	logger     *slog.Logger
	now        func() time.Time

	mu   sync.Mutex
	seen map[uuid.UUID]time.Time
}

// NewActivityTracker returns a tracker. background is the application's
// goroutine runner, so shutdown waits for in-flight writes.
func NewActivityTracker(store ActivityStore, background func(func()), logger *slog.Logger) *ActivityTracker {
	return &ActivityTracker{
		store:      store,
		background: background,
		logger:     logger,
		now:        time.Now,
		seen:       make(map[uuid.UUID]time.Time),
	}
}

// TouchOwner notes that an owner made an authenticated request. It writes to the
// database at most once an hour per owner in this process, and the database's own
// condition makes the same promise across instances.
func (t *ActivityTracker) TouchOwner(id uuid.UUID) {
	t.mu.Lock()
	now := t.now()
	if last, ok := t.seen[id]; ok && now.Sub(last) < activityInterval {
		t.mu.Unlock()
		return
	}
	if len(t.seen) >= pruneAt {
		for k, last := range t.seen {
			if now.Sub(last) >= activityInterval {
				delete(t.seen, k)
			}
		}
	}
	// Recorded before the write, so a failing database is not retried by every
	// request in the hour that follows.
	t.seen[id] = now
	t.mu.Unlock()

	t.background(func() {
		if _, err := t.store.TouchOwnerLastActive(context.Background(), id); err != nil {
			t.logger.Warn("lifecycle: owner activity not recorded", "error", err)
		}
	})
}
