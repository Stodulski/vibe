package stores

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
)

// The retrier is worth nothing if the stores do not go through it, and this
// file is where that is provable: every store is built here, so this is the
// one place that can drive all three shapes of database access — a generated
// query, a hand-written read and a hand-written write — over a runner that
// counts what reached it.
//
// The doubles are deliberately the smallest thing that satisfies
// data.StatementRunner. internal/data's own retry_test.go owns the retry
// behaviour itself; what is under test here is the wiring.

// countingRunner fails every statement with a retryable connection error and
// records each attempt.
type countingRunner struct {
	calls int
}

func (r *countingRunner) record() error {
	r.calls++
	return &pgconn.PgError{Code: "08006", Message: "connection failure"}
}

func (r *countingRunner) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, r.record()
}

func (r *countingRunner) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, r.record()
}

func (r *countingRunner) QueryRow(context.Context, string, ...any) pgx.Row {
	return countingRow{r}
}

// countingRow defers to Scan the way pgx does, so the retrier's deferred-retry
// path is the one exercised rather than a simplification of it.
type countingRow struct{ r *countingRunner }

func (row countingRow) Scan(...any) error { return row.r.record() }

// noWait records nothing and waits for nothing, so the backoff costs a test
// no wall-clock time.
func noWait(context.Context, time.Duration) error { return nil }

func storesOverRunner(r *countingRunner) Stores {
	return newStores(data.NewDBOver(r, noWait), Config{})
}

// A generated query is the one that dies silently if db.New is ever handed the
// raw pool instead of the retrying handle: it would compile and pass every
// other test.
func TestAGeneratedQueryRunsThroughTheRetrier(t *testing.T) {
	r := &countingRunner{}
	s := storesOverRunner(r)

	if _, err := s.Users.GetByID(t.Context(), uuid.New()); err == nil {
		t.Fatal("want the connection failure to surface")
	}
	if r.calls != data.MaxAttempts {
		t.Fatalf("Users.GetByID reached the runner %d times, want %d; "+
			"one attempt means the generated queries are not behind the retrier",
			r.calls, data.MaxAttempts)
	}
}

// A store that writes its own SQL rather than going through sqlc shares the
// same handle, so it gets the same treatment.
func TestAHandWrittenReadRunsThroughTheRetrier(t *testing.T) {
	r := &countingRunner{}
	s := storesOverRunner(r)

	if _, err := s.FailedRefunds.GetPendingDue(t.Context()); err == nil {
		t.Fatal("want the connection failure to surface")
	}
	if r.calls != data.MaxAttempts {
		t.Fatalf("FailedRefunds.GetPendingDue reached the runner %d times, want %d",
			r.calls, data.MaxAttempts)
	}
}

// And the write rule reaches a hand-written statement too. TryAdvisory's
// statement is an INSERT with a RETURNING clause, so it looks like a read from
// the method it is run through and is a write in every way that matters: a
// second attempt would take a lease the first attempt may already have taken.
func TestAHandWrittenWriteIsNotRepeated(t *testing.T) {
	r := &countingRunner{}
	s := storesOverRunner(r)

	if _, _, err := s.Locks.TryAdvisory(t.Context(), "some-job"); err == nil {
		t.Fatal("want the connection failure to surface")
	}
	if r.calls != 1 {
		t.Fatalf("Locks.TryAdvisory was sent %d times; the lease may already be taken", r.calls)
	}
}
