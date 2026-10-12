package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeActivityStore struct {
	mu    sync.Mutex
	calls []uuid.UUID
	err   error
}

func (f *fakeActivityStore) TouchOwnerLastActive(_ context.Context, id uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)
	return true, f.err
}

func (f *fakeActivityStore) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// synchronous runs the write inline so a test can read the store right after.
func synchronous(fn func()) { fn() }

func newTestTracker(store ActivityStore, clock *time.Time) *ActivityTracker {
	t := NewActivityTracker(store, synchronous, discardLogger())
	t.now = func() time.Time { return *clock }
	return t
}

func TestActivityWritesAtMostOncePerHourPerOwner(t *testing.T) {
	store := &fakeActivityStore{}
	clock := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	tr := newTestTracker(store, &clock)
	owner := uuid.New()

	for range 3 {
		tr.TouchOwner(owner)
	}
	if got := store.count(); got != 1 {
		t.Fatalf("three requests in one instant wrote %d times, want 1", got)
	}

	clock = clock.Add(59 * time.Minute)
	tr.TouchOwner(owner)
	if got := store.count(); got != 1 {
		t.Fatalf("a request 59 minutes later wrote again (%d writes), the throttle is an hour", got)
	}

	clock = clock.Add(2 * time.Minute)
	tr.TouchOwner(owner)
	if got := store.count(); got != 2 {
		t.Fatalf("a request after the hour wrote %d times total, want 2", got)
	}
}

func TestActivityThrottlesEachOwnerIndependently(t *testing.T) {
	store := &fakeActivityStore{}
	clock := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	tr := newTestTracker(store, &clock)

	tr.TouchOwner(uuid.New())
	tr.TouchOwner(uuid.New())
	if got := store.count(); got != 2 {
		t.Fatalf("two different owners wrote %d times, want 2", got)
	}
}

func TestActivityFailureIsLoggedAndNotRetriedEveryRequest(t *testing.T) {
	store := &fakeActivityStore{err: errors.New("db down")}
	clock := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	tr := newTestTracker(store, &clock)
	owner := uuid.New()

	for range 5 {
		tr.TouchOwner(owner) // must not panic, must not block the request
	}
	if got := store.count(); got != 1 {
		t.Fatalf("a failing database was retried %d times in one hour, want 1", got)
	}
}

func TestActivityPrunesStaleEntries(t *testing.T) {
	store := &fakeActivityStore{}
	clock := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	tr := newTestTracker(store, &clock)

	for range pruneAt {
		tr.TouchOwner(uuid.New())
	}
	clock = clock.Add(2 * activityInterval)
	tr.TouchOwner(uuid.New())

	tr.mu.Lock()
	size := len(tr.seen)
	tr.mu.Unlock()
	if size > 1 {
		t.Fatalf("the in-process throttle kept %d stale entries, want them pruned", size)
	}
}
