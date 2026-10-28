package jobs_test

import (
	"context"
	"errors"
	"expvar"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// fakeEnqueueStore stands in for the jobs table. It answers the way the insert
// does: an error, a dedup hit (recorded false, no error), or a new row.
type fakeEnqueueStore struct {
	err      error
	recorded bool

	gotRunAt  time.Time
	gotDedup  string
	gotMaxTry int
}

func (s *fakeEnqueueStore) Enqueue(_ context.Context, _ string, _ any, runAt time.Time, maxAttempts int, dedupKey string) (uuid.UUID, bool, error) {
	s.gotRunAt, s.gotMaxTry, s.gotDedup = runAt, maxAttempts, dedupKey
	if s.err != nil {
		return uuid.Nil, false, s.err
	}
	if !s.recorded {
		return uuid.Nil, false, nil
	}
	return uuid.New(), true, nil
}

// counted reads one enqueuer counter, reading an unset one as zero.
func counted(m *expvar.Map, name string) string {
	if v := m.Get(name); v != nil {
		return v.String()
	}
	return "0"
}

func newEnqueuer(store jobs.EnqueueStore) (*jobs.Enqueuer, *expvar.Map) {
	metrics := new(expvar.Map).Init()
	return &jobs.Enqueuer{
		Store:       store,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics:     metrics,
		MaxAttempts: 3,
	}, metrics
}

// An insert that fails is a failure the caller is told about. The sweep that
// queues a reminder relies on this: it leaves the booking unflagged, so the next
// tick tries again.
func TestTryEnqueueAtReturnsAnInsertFailure(t *testing.T) {
	insertErr := errors.New("database is down")
	e, metrics := newEnqueuer(&fakeEnqueueStore{err: insertErr})

	err := e.TryEnqueueAt("email", map[string]string{"to": "ana@example.com"}, time.Time{}, "key-1")

	if !errors.Is(err, insertErr) {
		t.Fatalf("an insert failure must come back to the caller; got %v", err)
	}
	if got := counted(metrics, "enqueue_failed"); got != "1" {
		t.Errorf("the failed insert must be counted; enqueue_failed = %s", got)
	}
}

// A dedup hit is not a failure. The work is already queued, which is what the
// key is for, so the caller must see success and move on.
func TestTryEnqueueAtTreatsADedupHitAsSuccess(t *testing.T) {
	e, metrics := newEnqueuer(&fakeEnqueueStore{recorded: false})

	if err := e.TryEnqueueAt("email", map[string]string{}, time.Time{}, "key-1"); err != nil {
		t.Fatalf("a dedup hit means the work is already queued, not failed; got %v", err)
	}
	if got := counted(metrics, "deduplicated"); got != "1" {
		t.Errorf("the dedup hit must be counted as deduplicated; got %s", got)
	}
	if got := counted(metrics, "enqueue_failed"); got != "0" {
		t.Errorf("a dedup hit must not count as a failed enqueue; enqueue_failed = %s", got)
	}
}

// A new job is recorded with the run time, dedup key and attempt budget the
// caller asked for.
func TestTryEnqueueAtRecordsANewJob(t *testing.T) {
	store := &fakeEnqueueStore{recorded: true}
	e, metrics := newEnqueuer(store)
	runAt := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

	if err := e.TryEnqueueAt("email", map[string]string{}, runAt, "key-2"); err != nil {
		t.Fatalf("a recorded job is success; got %v", err)
	}
	if !store.gotRunAt.Equal(runAt) || store.gotDedup != "key-2" || store.gotMaxTry != 3 {
		t.Errorf("the insert must carry runAt, dedup key and MaxAttempts; got %v, %q, %d",
			store.gotRunAt, store.gotDedup, store.gotMaxTry)
	}
	if got := counted(metrics, "enqueued"); got != "1" {
		t.Errorf("a new job must be counted as enqueued; got %s", got)
	}
}

// TryEnqueue is the due-now form of TryEnqueueAt: the job carries no run time,
// so the store makes it due immediately.
func TestTryEnqueueRecordsTheJobAsDueNow(t *testing.T) {
	store := &fakeEnqueueStore{recorded: true}
	e, _ := newEnqueuer(store)

	if err := e.TryEnqueue("email", map[string]string{}, "key-4"); err != nil {
		t.Fatalf("a recorded job is success; got %v", err)
	}
	if !store.gotRunAt.IsZero() || store.gotDedup != "key-4" {
		t.Errorf("TryEnqueue must record the job due now under its dedup key; got runAt %v, key %q",
			store.gotRunAt, store.gotDedup)
	}
}

// EnqueueAt keeps its contract: callers that do nothing about a failure still
// get no return value, and the failure is counted rather than lost.
func TestEnqueueAtStillSwallowsAnInsertFailure(t *testing.T) {
	e, metrics := newEnqueuer(&fakeEnqueueStore{err: errors.New("database is down")})

	e.EnqueueAt("email", map[string]string{}, time.Time{}, "key-3")

	if got := counted(metrics, "enqueue_failed"); got != "1" {
		t.Errorf("EnqueueAt must still count the failed insert; enqueue_failed = %s", got)
	}
}
