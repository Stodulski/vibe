package data

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stodulski/vibe-server/internal/db"
)

// maxAttempts is how many times one statement is sent before the failure is
// handed to the caller.
//
// Three, not more: the failures this retries are a connection that died and a
// server that is coming back from a failover, and both are decided within a
// couple of round trips. A longer ladder would spend the caller's whole
// deadline — every statement here runs under QueryContext's three seconds —
// waiting on something that has already answered.
const maxAttempts = 3

// retryBaseDelay is the wait before the second attempt; it doubles thereafter.
//
// It is deliberately shorter than anything a human notices, because the point
// is not to wait out an outage. A pooled connection that was severed is
// replaced on the next checkout, and a failover promotes a standby in well
// under a second. Two waits of 20ms and 40ms cost a request 60ms in the worst
// case and nothing at all in the ordinary one.
const retryBaseDelay = 20 * time.Millisecond

// DB is the database handle every store in this package holds.
//
// It is the connection pool with one behaviour added: a single statement that
// fails for a reason a later attempt could survive is sent again. Begin is
// passed straight through, which is the important half of the design — see
// retrier for why a transaction must never be retried a statement at a time.
type DB struct {
	pool *pgxpool.Pool
	r    retrier
	// begin opens the transaction Begin stamps and hands back. It is nil in
	// every build the application makes, where the pool is the only thing a
	// transaction can start on; NewDBOverTx sets it so a test can run a whole
	// package's worth of work inside one transaction it rolls back. See that
	// function.
	begin func(context.Context) (pgx.Tx, error)
}

// NewDB wraps a connection pool so that single statements retry transient
// failures.
func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool, r: retrier{db: pool, sleep: waitFor}}
}

// Pool returns the underlying pool, for the callers that need the pool itself
// rather than a way to run statements — health checks and shutdown.
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// Begin starts a transaction on the pool, with no retry wrapped around it, and
// stamps the tenant scope on it.
//
// A transaction owns a connection for its whole life, so its statements are
// not independent: replaying one of them alone would replay it inside a
// transaction the server has already aborted. Transactions are retried, when
// they are retried at all, by their caller re-running the whole unit.
//
// The stamp is the first statement inside the transaction, before the caller
// gets the handle back, and it is SET LOCAL — see tenant.go for why both this
// and the pool's checkout hook exist. Every transaction in this package is
// opened here, which is why the tenant reaches all of them without a single
// call-site change. A failed stamp rolls the transaction back and returns the
// error rather than handing back a transaction that would read the wrong
// tenant's rows.
func (d *DB) Begin(ctx context.Context) (pgx.Tx, error) {
	open := d.begin
	if open == nil {
		open = d.pool.Begin
	}
	tx, err := open(ctx)
	if err != nil {
		return nil, err
	}
	if err := stampTx(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

// WithTx runs fn inside one transaction and commits it, or rolls it back.
//
// It is the only thing this package offers that Begin does not, and the reason
// is that the three lines around every Begin — the wrapped error, the deferred
// Rollback, the trailing Commit — were written out by hand at thirteen call
// sites. Every one of them was correct, which is precisely the problem: the
// invariant "no transaction is left open, on any path, including a panic" held
// by thirteen people having remembered it rather than by construction, and the
// fourteenth call site is the one that forgets.
//
// fn gets both handles it could want: the raw pgx.Tx, for the advisory locks
// and the hand-written SQL, and a *db.Queries bound to that same transaction,
// which is what the generated queries need. They are the same transaction —
// db.New(tx) is exactly what q.WithTx(tx) produces.
//
// The deferred Rollback after a successful Commit is a no-op: pgx answers
// ErrTxClosed, which is expected and discarded. On an error return it undoes
// the work, and on a panic unwinding through here it undoes the work and lets
// the panic continue — the connection goes back to the pool with no
// transaction on it either way.
//
// fn's error is returned unwrapped. A store method's caller compares against
// that package's sentinels, and a wrapper here would make every one of those
// comparisons go through errors.Is for no benefit.
func (d *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx, q *db.Queries) error) error {
	tx, err := d.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("data: commit tx: %w", err)
	}
	return nil
}

// Exec runs a statement that returns no rows.
func (d *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return d.r.Exec(ctx, sql, args...)
}

// Query runs a statement that returns rows.
func (d *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return d.r.Query(ctx, sql, args...)
}

// QueryRow runs a statement that returns at most one row.
func (d *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return d.r.QueryRow(ctx, sql, args...)
}

// statementRunner is the part of the pool that runs one statement. It is the
// same shape sqlc generates its Queries against, so a retrier can stand
// between the two.
type statementRunner interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// retrier sends one statement, and sends it again when the failure was the
// connection or the server rather than the statement.
//
// Two rules decide whether a second attempt happens, and the difference
// between them is the whole of the safety argument:
//
//   - Any statement is retried when pgx reports the failure happened before
//     the statement reached the server. pgx knows this because it had not
//     finished writing when the connection broke, so the server never saw the
//     statement and cannot have applied it. Replaying it is not a second
//     attempt at all — it is the first one.
//
//   - A statement that only reads is retried on a transient failure even after
//     it was sent, because running a SELECT twice produces the same answer and
//     leaves nothing behind.
//
// A write that failed after it was sent is never retried. "The connection
// dropped" and "the connection dropped after the commit was applied" arrive at
// the client as the same error, and re-sending in the second case books the
// court twice, or refunds twice. Failing a request that might have succeeded
// is recoverable — the caller sees an error and can look. Silently doing it
// twice is not.
type retrier struct {
	db statementRunner
	// sleep waits between attempts. It is a field so a test can prove the
	// backoff without spending it.
	sleep func(ctx context.Context, d time.Duration) error
}

// waitFor sleeps for d, or returns early if ctx finishes first.
func waitFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Exec runs a statement that returns no rows.
func (r retrier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	err := r.attempt(ctx, sql, func(ctx context.Context) error {
		var err error
		tag, err = r.db.Exec(ctx, sql, args...)
		return err
	})
	return tag, err
}

// Query runs a statement that returns rows.
//
// Only the failure to start the query is retried. Once rows are being streamed
// the caller holds them, and a connection that dies mid-stream surfaces at
// rows.Err() with some of the result already consumed; re-running underneath
// the caller would hand it a second result set spliced onto the first.
func (r retrier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	var rows pgx.Rows
	err := r.attempt(ctx, sql, func(ctx context.Context) error {
		var err error
		// The rows belong to the caller, which closes them. This function is a
		// pass-through, and closing here would hand back a spent result set.
		rows, err = r.db.Query(ctx, sql, args...) //nolint:sqlclosecheck // returned to the caller
		return err
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// QueryRow runs a statement that returns at most one row.
//
// pgx defers the error to Scan, so the retry has to be deferred with it: the
// row returned here has not run anything yet.
func (r retrier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &retryRow{r: r, ctx: ctx, sql: sql, args: args}
}

// retryRow runs its statement when it is scanned, so that a transient failure
// can be retried with the destinations still in hand.
type retryRow struct {
	r   retrier
	ctx context.Context //nolint:containedctx // pgx.Row.Scan takes no context; the query is deferred to it
	sql string

	args []any
}

// Scan runs the statement and copies the row into dest, retrying a transient
// failure. A retry rewrites dest from the start, so a partly-written
// destination from the failed attempt cannot survive into the result.
func (row *retryRow) Scan(dest ...any) error {
	return row.r.attempt(row.ctx, row.sql, func(ctx context.Context) error {
		return row.r.db.QueryRow(ctx, row.sql, row.args...).Scan(dest...)
	})
}

// attempt runs op until it succeeds, until the failure is one no later attempt
// would survive, or until the attempts are spent.
func (r retrier) attempt(ctx context.Context, sql string, op func(context.Context) error) error {
	readOnly := readOnlyStatement(sql)

	var err error
	for attempt := range maxAttempts {
		err = op(ctx)
		if err == nil || !retryable(err, readOnly) {
			return err
		}
		if attempt == maxAttempts-1 {
			break
		}
		// A caller whose deadline passed while we waited gets the database
		// error, not the timeout: it is the one that says what went wrong.
		if waitErr := r.sleep(ctx, retryBaseDelay<<attempt); waitErr != nil {
			return err
		}
	}
	return err
}

// retryable reports whether err is worth another attempt.
//
// readOnly says whether the statement only reads, which is what licenses the
// second rule described on retrier.
func retryable(err error, readOnly bool) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// The caller's budget is spent. Another attempt would fail on the same
		// context before it reached the network.
		return false
	case pgconn.SafeToRetry(err):
		// pgx proved the statement never left the client.
		return true
	case !readOnly:
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return transientSQLSTATE(pgErr.Code)
	}
	return brokenConnection(err)
}

// transientSQLSTATE reports whether a PostgreSQL error code names a failure of
// the connection or of the server rather than of the statement.
//
// Class 08 is "Connection Exception" in full, so the whole class qualifies. The
// three 57P codes are the ones a failover produces: the primary being shut
// down, the primary having crashed, and a standby that is still recovering and
// is not accepting connections yet. Every one of them describes a server that
// will answer the same statement a moment later.
//
// Everything else is excluded on purpose, including 40001 serialization_failure
// and 40P01 deadlock_detected. Those are genuinely retryable, but only by
// replaying the whole transaction they aborted, which is not something a single
// statement can do from here.
func transientSQLSTATE(code string) bool {
	switch code {
	case "57P01", "57P02", "57P03":
		return true
	}
	return strings.HasPrefix(code, "08")
}

// brokenConnection reports whether err is a connection that died underneath a
// statement that had already been sent.
//
// pgx surfaces these as ordinary network errors rather than as a PgError,
// because there was no server left to send an error code. They are the exact
// shape of the "connection reset" this retries, and they are consulted only
// for a statement that reads: a write that got this far may already have been
// applied.
func brokenConnection(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}

// readOnlyStatement reports whether sql can be run twice with no consequence.
//
// The test is deliberately narrow, because it is a safety test and everything
// it is unsure about must come out false. Only a statement whose first keyword
// is SELECT qualifies. WITH does not, because a common table expression is
// allowed to contain INSERT, UPDATE or DELETE and the difference is not visible
// from the first keyword. A SELECT that takes row locks does not either: the
// lock is a side effect, and while every locking read in this package today
// runs inside a transaction — where this code never sees it — a future one that
// does not should not quietly acquire its lock twice.
func readOnlyStatement(sql string) bool {
	s := strings.TrimLeft(sql, " \t\r\n")
	for strings.HasPrefix(s, "--") {
		_, rest, found := strings.Cut(s, "\n")
		if !found {
			return false
		}
		s = strings.TrimLeft(rest, " \t\r\n")
	}

	const keyword = "SELECT"
	if len(s) <= len(keyword) || !strings.EqualFold(s[:len(keyword)], keyword) {
		return false
	}
	// "SELECTED" is not "SELECT"; the keyword has to end where it ends.
	switch s[len(keyword)] {
	case ' ', '\t', '\r', '\n', '(':
	default:
		return false
	}

	upper := strings.ToUpper(s)
	return !strings.Contains(upper, "FOR UPDATE") && !strings.Contains(upper, "FOR SHARE")
}
