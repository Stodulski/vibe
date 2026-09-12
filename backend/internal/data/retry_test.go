package data

import (
	"context"
	"errors"
	"io"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/db"
)

// The two statements below are shaped exactly as sqlc emits them — the
// "-- name:" header on the first line, then the SQL — because that header is
// the reason readOnlyStatement cannot simply look at the first byte, and a test
// that fed it bare SQL would prove nothing about the strings that actually
// reach it.
const (
	readSQL  = "-- name: GetUserByID :one\nSELECT id, email FROM users\nWHERE id = $1\n"
	writeSQL = "-- name: InsertUser :one\nINSERT INTO users (email) VALUES ($1)\nRETURNING id\n"
	execSQL  = "-- name: DeleteUser :exec\nDELETE FROM users WHERE id = $1\n"
)

// ---------------------------------------------------------------------------
// Doubles
// ---------------------------------------------------------------------------

// fakeCall is one statement as the runner actually received it.
type fakeCall struct {
	sql  string
	args []any
	// ctxErr is the state of the context at the moment the statement ran, so a
	// test can tell "the retry happened" from "the retry happened on a context
	// that was already dead".
	ctxErr error
}

// fakeRunner stands in for the pool. It fails a fixed number of times and then
// succeeds, recording every statement, every argument and the state of the
// context it was handed.
//
// It records rather than counts on purpose: a double that only counted calls
// would stay green if the retry re-sent the statement with no arguments, or
// re-sent a different statement entirely.
type fakeRunner struct {
	failures int   // attempts that fail before one succeeds
	err      error // what those attempts return
	value    int64 // what a successful Scan writes into its destination

	calls []fakeCall
	rows  fakeRows
}

func (f *fakeRunner) record(ctx context.Context, sql string, args []any) error {
	f.calls = append(f.calls, fakeCall{sql: sql, args: args, ctxErr: ctx.Err()})
	if len(f.calls) <= f.failures {
		return f.err
	}
	return nil
}

func (f *fakeRunner) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.record(ctx, sql, args)
}

func (f *fakeRunner) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := f.record(ctx, sql, args); err != nil {
		return nil, err
	}
	return &f.rows, nil
}

func (f *fakeRunner) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &fakeRow{f: f, ctx: ctx, sql: sql, args: args}
}

// fakeRow defers to Scan the way pgx does, so the retrier's deferred-retry path
// is the one under test rather than a simplification of it.
type fakeRow struct {
	f   *fakeRunner
	ctx context.Context //nolint:containedctx // mirrors pgx.Row, whose Scan takes none
	sql string

	args []any
}

func (r *fakeRow) Scan(dest ...any) error {
	if err := r.f.record(r.ctx, r.sql, r.args); err != nil {
		return err
	}
	if len(dest) > 0 {
		if p, ok := dest[0].(*int64); ok {
			*p = r.f.value
		}
	}
	return nil
}

// fakeRows is a non-nil pgx.Rows so a test can assert the retrier handed back
// the result set rather than a zero value. Close is real so a test can close
// what it was handed; every other method would panic on the nil embedded
// interface, which is the right outcome for a test that starts reading rows
// this file never fills.
type fakeRows struct {
	pgx.Rows
	closed bool
}

func (r *fakeRows) Close() { r.closed = true }

// fakeClock records the waits instead of taking them, so a test of the backoff
// does not have to spend it.
type fakeClock struct {
	waits []time.Duration
	err   error
}

func (c *fakeClock) sleep(ctx context.Context, d time.Duration) error {
	c.waits = append(c.waits, d)
	if c.err != nil {
		return c.err
	}
	return ctx.Err()
}

// safeToRetryError is the shape pgx gives an error raised before the statement
// was written to the connection.
type safeToRetryError struct{ error }

func (safeToRetryError) SafeToRetry() bool { return true }

func newRetrier(f *fakeRunner) (retrier, *fakeClock) {
	clock := &fakeClock{}
	return retrier{db: f, sleep: clock.sleep}, clock
}

func connectionFailure() error {
	return &pgconn.PgError{Code: "08006", Message: "connection failure"}
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

// A connection that dies under a SELECT is the failure this whole file exists
// for: nothing was written, the answer does not change, and the next attempt
// gets a fresh connection out of the pool.
func TestAReadSurvivesAConnectionThatDiesUnderIt(t *testing.T) {
	f := &fakeRunner{failures: 2, err: connectionFailure(), value: 42}
	r, clock := newRetrier(f)

	id := int64(0)
	err := r.QueryRow(t.Context(), readSQL, "the-id").Scan(&id)
	if err != nil {
		t.Fatalf("the third attempt should have succeeded; got %v", err)
	}
	if id != 42 {
		t.Errorf("the successful attempt's row did not reach the destination: got %d, want 42", id)
	}
	if len(f.calls) != 3 {
		t.Fatalf("want 3 attempts, got %d", len(f.calls))
	}
	for i, call := range f.calls {
		if call.sql != readSQL {
			t.Errorf("attempt %d ran a different statement", i+1)
		}
		if len(call.args) != 1 || call.args[0] != "the-id" {
			t.Errorf("attempt %d lost its arguments: %v", i+1, call.args)
		}
	}
	want := []time.Duration{retryBaseDelay, 2 * retryBaseDelay}
	if len(clock.waits) != len(want) || clock.waits[0] != want[0] || clock.waits[1] != want[1] {
		t.Errorf("backoff: got %v, want %v", clock.waits, want)
	}
}

// A failover: the primary is shutting down, or the standby has not finished
// recovering. Both answer normally a moment later.
func TestAReadSurvivesAFailover(t *testing.T) {
	for _, code := range []string{"57P01", "57P02", "57P03", "08000", "08003", "08006"} {
		t.Run(code, func(t *testing.T) {
			f := &fakeRunner{failures: 1, err: &pgconn.PgError{Code: code}}
			r, _ := newRetrier(f)

			id := int64(0)
			if err := r.QueryRow(t.Context(), readSQL, "the-id").Scan(&id); err != nil {
				t.Fatalf("SQLSTATE %s should have been retried; got %v", code, err)
			}
			if len(f.calls) != 2 {
				t.Errorf("want 2 attempts, got %d", len(f.calls))
			}
		})
	}
}

// A read whose connection was severed mid-statement never gets a SQLSTATE back,
// because there is no server left to send one.
func TestAReadSurvivesASeveredConnection(t *testing.T) {
	for name, err := range map[string]error{
		"reset": syscall.ECONNRESET,
		"pipe":  syscall.EPIPE,
		"eof":   io.ErrUnexpectedEOF,
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeRunner{failures: 1, err: err}
			r, _ := newRetrier(f)

			id := int64(0)
			if qerr := r.QueryRow(t.Context(), readSQL, "the-id").Scan(&id); qerr != nil {
				t.Fatalf("%s should have been retried; got %v", name, qerr)
			}
			if len(f.calls) != 2 {
				t.Errorf("want 2 attempts, got %d", len(f.calls))
			}
		})
	}
}

// Query must hand back the result set it got, not a zero value that a caller
// would then range over as if it were empty.
func TestARetriedQueryReturnsTheResultSet(t *testing.T) {
	f := &fakeRunner{failures: 1, err: connectionFailure()}
	r, _ := newRetrier(f)

	rows, err := r.Query(t.Context(), readSQL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rows.Close()

	if rows != &f.rows {
		t.Error("the rows from the successful attempt were not returned")
	}
}

// ---------------------------------------------------------------------------
// Writes
// ---------------------------------------------------------------------------

// The safety line. "The connection dropped" and "the connection dropped after
// the row was committed" arrive as the same error, so a write that was already
// sent is failed rather than repeated.
//
// It is a QueryRow, not an Exec, because sqlc runs INSERT ... RETURNING through
// QueryRow: a policy that decided read from write by looking at which method
// was called would retry this one.
func TestAWriteIsNotRepeatedOnceItReachedTheServer(t *testing.T) {
	f := &fakeRunner{failures: 1, err: connectionFailure()}
	r, clock := newRetrier(f)

	id := int64(0)
	err := r.QueryRow(t.Context(), writeSQL, "someone@example.com").Scan(&id)
	if err == nil {
		t.Fatal("the write should have failed rather than been repeated")
	}
	if len(f.calls) != 1 {
		t.Errorf("the write was sent %d times; it may have committed the first time", len(f.calls))
	}
	if len(clock.waits) != 0 {
		t.Errorf("no backoff should have been taken; got %v", clock.waits)
	}
}

// The same rule for a plain DELETE.
func TestAnExecIsNotRepeatedOnceItReachedTheServer(t *testing.T) {
	f := &fakeRunner{failures: 1, err: connectionFailure()}
	r, _ := newRetrier(f)

	if _, err := r.Exec(t.Context(), execSQL, "the-id"); err == nil {
		t.Fatal("the statement should have failed rather than been repeated")
	}
	if len(f.calls) != 1 {
		t.Errorf("the statement was sent %d times; it may have applied the first time", len(f.calls))
	}
}

// The one case where repeating a write is safe: pgx reports it never finished
// writing the statement to the connection, so the server never saw it.
func TestAWriteIsRepeatedWhenPgxProvedItNeverLeftTheClient(t *testing.T) {
	f := &fakeRunner{failures: 1, err: safeToRetryError{errors.New("write failed before send")}}
	r, _ := newRetrier(f)

	if _, err := r.Exec(t.Context(), execSQL, "the-id"); err != nil {
		t.Fatalf("a statement that never left the client is safe to send; got %v", err)
	}
	if len(f.calls) != 2 {
		t.Errorf("want 2 attempts, got %d", len(f.calls))
	}
}

// ---------------------------------------------------------------------------
// What is not transient
// ---------------------------------------------------------------------------

func TestFailuresThatNoLaterAttemptWouldSurviveAreNotRetried(t *testing.T) {
	tests := map[string]error{
		"no rows":              pgx.ErrNoRows,
		"unique violation":     &pgconn.PgError{Code: "23505"},
		"foreign key":          &pgconn.PgError{Code: "23503"},
		"syntax error":         &pgconn.PgError{Code: "42601"},
		"serialization":        &pgconn.PgError{Code: "40001"},
		"deadlock":             &pgconn.PgError{Code: "40P01"},
		"caller cancelled":     context.Canceled,
		"caller out of budget": context.DeadlineExceeded,
	}

	for name, err := range tests {
		t.Run(name, func(t *testing.T) {
			f := &fakeRunner{failures: 1, err: err}
			r, _ := newRetrier(f)

			id := int64(0)
			if scanErr := r.QueryRow(t.Context(), readSQL).Scan(&id); scanErr == nil {
				t.Fatal("want the failure to be returned")
			}
			if len(f.calls) != 1 {
				t.Errorf("want 1 attempt, got %d", len(f.calls))
			}
		})
	}
}

// The ladder is finite. A database that is down stays down, and the caller gets
// the last error rather than a timeout from somewhere else.
func TestThePersistentFailureIsReturnedAfterTheAttemptsAreSpent(t *testing.T) {
	want := connectionFailure()
	f := &fakeRunner{failures: 100, err: want}
	r, clock := newRetrier(f)

	id := int64(0)
	err := r.QueryRow(t.Context(), readSQL, "the-id").Scan(&id)
	if !errors.Is(err, want) {
		t.Fatalf("want the database's own error, got %v", err)
	}
	if len(f.calls) != maxAttempts {
		t.Errorf("want %d attempts, got %d", maxAttempts, len(f.calls))
	}
	if len(clock.waits) != maxAttempts-1 {
		t.Errorf("want %d waits, got %d", maxAttempts-1, len(clock.waits))
	}
}

// A caller whose deadline passes during the backoff is not made to wait for the
// rest of the ladder, and is handed the database's error rather than the clock's.
func TestTheLadderStopsWhenTheCallersBudgetIsSpent(t *testing.T) {
	want := connectionFailure()
	f := &fakeRunner{failures: 100, err: want}
	clock := &fakeClock{err: context.DeadlineExceeded}
	r := retrier{db: f, sleep: clock.sleep}

	id := int64(0)
	err := r.QueryRow(t.Context(), readSQL, "the-id").Scan(&id)
	if !errors.Is(err, want) {
		t.Fatalf("want the database's own error, got %v", err)
	}
	if len(f.calls) != 1 {
		t.Errorf("want 1 attempt once the budget is spent, got %d", len(f.calls))
	}
}

// ---------------------------------------------------------------------------
// Statement classification
// ---------------------------------------------------------------------------

func TestReadOnlyStatement(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{"sqlc select", readSQL, true},
		{"sqlc insert returning", writeSQL, false},
		{"sqlc delete", execSQL, false},
		{"bare select", "SELECT 1", true},
		{"lowercase select", "select id from users", true},
		{"leading whitespace", "\n\t  SELECT id FROM users", true},
		{"parenthesised", "SELECT(1)", true},
		{"stacked comments", "-- one\n-- two\nSELECT id FROM users", true},
		{"comment with no newline", "-- name: X :one", false},
		{"a word starting with select", "SELECTED id FROM users", false},
		{"with cte", "WITH t AS (SELECT 1) SELECT * FROM t", false},
		{"with cte that writes", "WITH d AS (DELETE FROM users RETURNING id) SELECT * FROM d", false},
		{"locking read", "SELECT id FROM users WHERE id = $1 FOR UPDATE", false},
		{"shared lock", "SELECT id FROM users FOR SHARE", false},
		{"update", "UPDATE users SET email = $1", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readOnlyStatement(tt.sql); got != tt.want {
				t.Errorf("readOnlyStatement(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

// The retrier is worth nothing if the stores do not go through it.
//
// This drives a real store method — one whose statement lives in a .sql file
// and is run by sqlc's generated code — and asserts the attempts arrived at the
// runner. It is the assertion that dies if db.New is ever handed the raw pool
// instead of the retrying handle, which would compile and pass every other test
// in this file.
func TestAGeneratedQueryRunsThroughTheRetrier(t *testing.T) {
	f := &fakeRunner{failures: 100, err: connectionFailure()}
	clock := &fakeClock{}
	m := newModels(&DB{r: retrier{db: f, sleep: clock.sleep}}, Config{})

	if _, err := m.Users.GetByID(t.Context(), uuid.New()); err == nil {
		t.Fatal("want the connection failure to surface")
	}
	if len(f.calls) != maxAttempts {
		t.Fatalf("UserModel.GetByID reached the runner %d times, want %d; "+
			"one attempt means the generated queries are not behind the retrier",
			len(f.calls), maxAttempts)
	}
}

// A store that writes its own SQL rather than going through sqlc shares the
// same handle, so it gets the same treatment.
func TestAHandWrittenReadRunsThroughTheRetrier(t *testing.T) {
	f := &fakeRunner{failures: 100, err: connectionFailure()}
	clock := &fakeClock{}
	m := newModels(&DB{r: retrier{db: f, sleep: clock.sleep}}, Config{})

	if _, err := m.FailedRefunds.GetPendingDue(t.Context()); err == nil {
		t.Fatal("want the connection failure to surface")
	}
	if len(f.calls) != maxAttempts {
		t.Fatalf("FailedRefundModel.GetPendingDue reached the runner %d times, want %d",
			len(f.calls), maxAttempts)
	}
}

// And the write rule reaches a hand-written statement too. TryAdvisory's
// statement is an INSERT with a RETURNING clause, so it looks like a read from
// the method it is run through and is a write in every way that matters: a
// second attempt would take a lease the first attempt may already have taken.
func TestAHandWrittenWriteIsNotRepeated(t *testing.T) {
	f := &fakeRunner{failures: 100, err: connectionFailure()}
	clock := &fakeClock{}
	m := newModels(&DB{r: retrier{db: f, sleep: clock.sleep}}, Config{})

	if _, _, err := m.Locks.TryAdvisory(t.Context(), "some-job"); err == nil {
		t.Fatal("want the connection failure to surface")
	}
	if len(f.calls) != 1 {
		t.Fatalf("LockModel.TryAdvisory was sent %d times; the lease may already be taken",
			len(f.calls))
	}
}

// A transaction must not be retried a statement at a time, so Begin goes to the
// pool untouched.
func TestBeginBypassesTheRetrier(t *testing.T) {
	var _ db.DBTX = (*DB)(nil)

	f := &fakeRunner{failures: 100, err: connectionFailure()}
	d := &DB{r: retrier{db: f, sleep: (&fakeClock{}).sleep}}

	// A nil pool panics rather than retrying, which is the evidence wanted:
	// Begin never reaches the runner the retrier drives.
	defer func() {
		if recover() == nil {
			t.Fatal("Begin should have gone to the pool")
		}
		if len(f.calls) != 0 {
			t.Errorf("Begin ran %d statements through the retrier; it must not", len(f.calls))
		}
	}()
	_, _ = d.Begin(t.Context())
}
