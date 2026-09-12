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

// ---------------------------------------------------------------------------
// Every statement carries a deadline
// ---------------------------------------------------------------------------

// deadlineRunner answers every statement with the same retryable failure the
// counting one does, and records whether the context that reached it had a
// deadline.
//
// The question matters because nothing else bounds these queries end to end. No
// http.TimeoutHandler puts a deadline on a request, so a store method that
// passes the caller's context straight to the query runs with whatever budget
// the caller happened to have — usually none — and the only remaining limit is
// the server's statement_timeout, which is fifteen seconds and is the backstop
// rather than the budget.
type deadlineRunner struct {
	statements   int
	withDeadline int
}

func (r *deadlineRunner) record(ctx context.Context) error {
	r.statements++
	if _, ok := ctx.Deadline(); ok {
		r.withDeadline++
	}
	return &pgconn.PgError{Code: "08006", Message: "connection failure"}
}

func (r *deadlineRunner) Exec(ctx context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, r.record(ctx)
}

func (r *deadlineRunner) Query(ctx context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, r.record(ctx)
}

func (r *deadlineRunner) QueryRow(ctx context.Context, _ string, _ ...any) pgx.Row {
	return deadlineRow{r: r, ctx: ctx}
}

type deadlineRow struct {
	r   *deadlineRunner
	ctx context.Context //nolint:containedctx // pgx.Row.Scan takes no context; the query is deferred to it
}

func (row deadlineRow) Scan(...any) error { return row.r.record(row.ctx) }

// The store methods that used to hand the caller's context to the query
// untouched, one per package that had them. A deadline-less context in must
// still produce a bounded statement out.
func TestEveryStoreMethodBoundsItsOwnStatement(t *testing.T) {
	id := uuid.New()

	calls := map[string]func(s Stores) error{
		"Users.GetByID": func(s Stores) error {
			_, err := s.Users.GetByID(context.Background(), id) //nolint:usetesting // a context with no deadline is the point
			return err
		},
		"EmailVerification.GetByHash": func(s Stores) error {
			_, err := s.EmailVerification.GetByHash(context.Background(), []byte("hash")) //nolint:usetesting // as above
			return err
		},
		"Tokens.GetRefreshToken": func(s Stores) error {
			_, err := s.Tokens.GetRefreshToken(context.Background(), []byte("hash")) //nolint:usetesting // as above
			return err
		},
		"Clients.GetByID": func(s Stores) error {
			_, err := s.Clients.GetByID(context.Background(), id) //nolint:usetesting // as above
			return err
		},
		"Courts.GetByID": func(s Stores) error {
			_, err := s.Courts.GetByID(context.Background(), id) //nolint:usetesting // as above
			return err
		},
		"Complexes.GetByID": func(s Stores) error {
			_, err := s.Complexes.GetByID(context.Background(), id) //nolint:usetesting // as above
			return err
		},
		"Payments.GetByBookingID": func(s Stores) error {
			_, err := s.Payments.GetByBookingID(context.Background(), id) //nolint:usetesting // as above
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			r := &deadlineRunner{}
			if err := call(newStores(data.NewDBOver(r, noWait), Config{})); err == nil {
				t.Fatal("want the connection failure to surface")
			}
			if r.statements == 0 {
				t.Fatal("no statement reached the runner")
			}
			if r.withDeadline != r.statements {
				t.Errorf("%d of %d statements ran with no deadline; the method passes the caller's "+
					"context straight to the query", r.statements-r.withDeadline, r.statements)
			}
		})
	}
}
