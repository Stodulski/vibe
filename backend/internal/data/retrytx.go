package data

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/db"
)

// This file is the half of retrying that retry.go deliberately does not do.
//
// The statement retrier excludes 40001 serialization_failure and 40P01
// deadlock_detected on purpose, and says so: both are genuinely retryable, but
// only by replaying the whole transaction the server aborted, and a retrier
// that sees one statement cannot replay a transaction it never opened.
//
// Nothing replayed them, so PostgreSQL's own answer to a deadlock — abort one
// side, the other commits, the aborted one tries again — stopped halfway. The
// side PostgreSQL chose to abort surfaced as a 500 to whoever was holding the
// request. And the deadlocks here are reachable rather than theoretical: two
// writers reach the court-day advisory lock and the booking row from opposite
// directions (see the ordering comment in paymentstore.InsertAndConfirmBooking),
// and the refund flows take payment and booking row locks in a fixed order that
// a concurrent booking update does not share.
//
// The lock order is still the first line of defence, and it is not weakened by
// this: a retry does not make a deadlock acceptable, it makes the one that
// still happens invisible to the caller instead of fatal.

// The SQLSTATEs that abort a transaction but say nothing about whether it would
// succeed if run again.
//
// 40001 is what SERIALIZABLE and REPEATABLE READ raise when concurrent
// transactions cannot be ordered; 40P01 is a deadlock the server broke by
// aborting this side. In both cases the transaction is gone — every statement
// in it was rolled back — so replaying the whole unit is the only correct
// response, and it is also a safe one: there is nothing left of the first
// attempt to duplicate.
const (
	SQLStateSerializationFailure = "40001"
	SQLStateDeadlockDetected     = "40P01"
)

// DefaultTxAttempts is how many times RetryTx runs a transaction before the
// failure is the caller's.
//
// Three, for the same reason the statement retrier stops at three: a deadlock
// is decided in microseconds and a serialization conflict in one round trip.
// Two contending writers are resolved by the first retry; a ladder longer than
// this is waiting out contention that a fourth attempt will not clear either,
// while holding the caller's request open.
const DefaultTxAttempts = 3

// txRetryBaseDelay is the wait before the second attempt; it doubles after
// each one, with jitter.
//
// Longer than the statement retrier's 20ms and for a different reason: that one
// waits for a connection to be replaced, this one waits for the transaction it
// collided with to finish. Backing off instantly reproduces the collision,
// which is how a retry turns one deadlock into a livelock.
const txRetryBaseDelay = 50 * time.Millisecond

// RetryTx runs fn inside a transaction and runs it again, up to attempts times,
// when PostgreSQL aborts the transaction with a conflict a later attempt could
// survive.
//
// fn must be safe to run more than once. That is not a burden the caller has to
// reason about hard: an aborted transaction left nothing behind, so what fn has
// to be is free of side effects OUTSIDE the transaction — no provider call, no
// queue push, no mutation of a struct the caller reads afterwards on a path the
// retry does not redo. The transaction's own writes are already undone.
//
// attempts below one is treated as one: a caller asking for zero attempts means
// a caller passing an unset value, and running the transaction no times at all
// would be a silent no-op rather than an error.
func (d *DB) RetryTx(ctx context.Context, attempts int, fn func(tx pgx.Tx, q *db.Queries) error) error {
	return retryTx(ctx, attempts, d.r.sleep, func(ctx context.Context) error {
		return d.WithTx(ctx, fn)
	})
}

// retryTx is RetryTx's loop, over an opaque unit of work.
//
// It is separated from the transaction it normally runs so that the retry
// policy — which errors are retried, how many times, how long between — can be
// proved against a stand-in that fails on demand, rather than only against a
// real PostgreSQL that has to be provoked into a deadlock.
func retryTx(
	ctx context.Context,
	attempts int,
	sleep func(ctx context.Context, d time.Duration) error,
	run func(ctx context.Context) error,
) error {
	if attempts < 1 {
		attempts = 1
	}

	var err error
	for attempt := range attempts {
		err = run(ctx)
		if err == nil || !retryableTx(err) {
			return err
		}
		if attempt == attempts-1 {
			break
		}
		// A caller whose deadline passed while we waited gets the database
		// error rather than the timeout: it is the one that says what went
		// wrong. Same choice the statement retrier makes.
		if waitErr := sleep(ctx, txBackoff(attempt)); waitErr != nil {
			return err
		}
	}
	return err
}

// txBackoff returns the wait before the attempt after this one: the base delay
// doubled per attempt, then spread over a random half of that window.
//
// The jitter is not decoration. Without it every transaction that collided at
// the same moment retries at the same moment, so the contention that caused the
// first collision is reproduced exactly — the pattern that turns a retry into a
// thundering herd. Full jitter over [d/2, d) keeps the schedule bounded while
// making two retrying writers almost certainly land apart.
func txBackoff(attempt int) time.Duration {
	d := txRetryBaseDelay << attempt
	//nolint:gosec // G404: this picks a backoff delay, not a secret; math/rand is the right tool.
	return d/2 + time.Duration(rand.Int64N(int64(d/2)))
}

// retryableTx reports whether err is PostgreSQL aborting a transaction for a
// reason that says nothing about whether it would succeed again.
//
// Deliberately narrower than the statement retrier's test: a connection that
// died mid-transaction is NOT retried here. "The connection dropped" and "the
// connection dropped after the COMMIT was applied" reach the client as the same
// error, and replaying in the second case books the court twice or refunds
// twice. The two SQLSTATEs below carry no such ambiguity — the server is the one
// telling us it rolled the transaction back.
func retryableTx(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == SQLStateSerializationFailure || pgErr.Code == SQLStateDeadlockDetected
}
