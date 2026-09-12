package data

import (
	"context"
	"time"
)

// MaxAttempts is how many times a retryable single statement is sent before
// the failure is returned to the caller.
const MaxAttempts = maxAttempts

// StatementRunner is what a DB sends every single statement to: the pool in a
// real deployment, and a recording stand-in in a test.
//
// It is exported for NewDBOver alone — see that function. Nothing in the
// application builds one.
type StatementRunner interface {
	statementRunner
}

// NewDBOver builds a DB whose single statements go to r, retried on the same
// schedule a real one uses, with sleep standing in for the wait between
// attempts so that a test can prove the backoff without spending it.
//
// It exists for the composition root's wiring test (internal/stores): the
// retrier is worth nothing if the stores do not go through it, and the only
// way to prove they do without a database is to build every store over a
// runner that counts what reaches it. Begin is deliberately not covered — a
// transaction is never retried a statement at a time — so a DB built here has
// no pool and must not be asked for one.
func NewDBOver(r StatementRunner, sleep func(ctx context.Context, d time.Duration) error) *DB {
	return &DB{r: retrier{db: r, sleep: sleep}}
}

// RetryTxLoop is RetryTx's retry policy over an opaque unit of work: which
// errors earn another attempt, how many attempts there are, and how long the
// waits between them last.
//
// Exported for the package's own external tests, which prove the policy against
// a stand-in that fails on demand. Provoking a real 40001 or 40P01 out of
// PostgreSQL takes two coordinated connections and a lock cycle, and a test
// that has to build one to check "three attempts, then give up" is testing the
// wrong thing.
func RetryTxLoop(
	ctx context.Context,
	attempts int,
	sleep func(ctx context.Context, d time.Duration) error,
	run func(ctx context.Context) error,
) error {
	return retryTx(ctx, attempts, sleep, run)
}

// TxBackoffForTest is the wait retryTx takes before the attempt after this one.
func TxBackoffForTest(attempt int) time.Duration { return txBackoff(attempt) }
