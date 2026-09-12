package data_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
)

// noSleep stands in for the wait between attempts, recording what would have
// been spent so a test can prove the backoff without spending it.
func noSleep(waits *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		return nil
	}
}

// aborted returns the error PostgreSQL hands back when it rolls a transaction
// back for a conflict.
func aborted(code string) error {
	return &pgconn.PgError{Code: code, Message: "transaction aborted"}
}

// failNTimes returns a unit of work that fails with err its first n runs and
// succeeds after, plus a counter of how many times it ran.
func failNTimes(n int, err error) (func(context.Context) error, *int) {
	runs := 0
	return func(context.Context) error {
		runs++
		if runs <= n {
			return err
		}
		return nil
	}, &runs
}

func TestRetryTxReplaysAnAbortedTransaction(t *testing.T) {
	for _, code := range []string{data.SQLStateSerializationFailure, data.SQLStateDeadlockDetected} {
		t.Run(code, func(t *testing.T) {
			run, runs := failNTimes(1, aborted(code))
			var waits []time.Duration

			if err := data.RetryTxLoop(t.Context(), data.DefaultTxAttempts, noSleep(&waits), run); err != nil {
				t.Fatalf("the second attempt succeeds, so RetryTx must not return an error: %v", err)
			}
			if *runs != 2 {
				t.Errorf("want the unit run twice; got %d", *runs)
			}
			if len(waits) != 1 {
				t.Errorf("want exactly one wait between the two attempts; got %v", waits)
			}
		})
	}
}

// The attempts are a bound, not a promise: a transaction that keeps colliding
// has to reach the caller rather than be retried forever.
func TestRetryTxGivesUpAfterTheAttemptsAreSpent(t *testing.T) {
	deadlock := aborted(data.SQLStateDeadlockDetected)
	run, runs := failNTimes(99, deadlock)
	var waits []time.Duration

	err := data.RetryTxLoop(t.Context(), data.DefaultTxAttempts, noSleep(&waits), run)
	if !errors.Is(err, deadlock) {
		t.Fatalf("want the last failure back; got %v", err)
	}
	if *runs != data.DefaultTxAttempts {
		t.Errorf("want %d attempts; got %d", data.DefaultTxAttempts, *runs)
	}
	if len(waits) != data.DefaultTxAttempts-1 {
		t.Errorf("want a wait between attempts and none after the last; got %v", waits)
	}
}

// Everything else is returned on the first attempt. A write that failed for any
// other reason may or may not have been applied, and "run it again and hope" is
// how a refund happens twice.
func TestRetryTxDoesNotReplayAnythingElse(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"a unique violation", aborted(data.SQLStateUniqueViolation)},
		{"a connection exception", aborted("08006")},
		{"the caller's own sentinel", data.ErrRecordNotFound},
		{"a cancelled context", context.Canceled},
		{"an expired deadline", context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run, runs := failNTimes(99, tc.err)
			var waits []time.Duration

			err := data.RetryTxLoop(t.Context(), data.DefaultTxAttempts, noSleep(&waits), run)
			if !errors.Is(err, tc.err) {
				t.Fatalf("want the error back unchanged; got %v", err)
			}
			if *runs != 1 {
				t.Errorf("want exactly one attempt; got %d", *runs)
			}
			if len(waits) != 0 {
				t.Errorf("want no wait at all; got %v", waits)
			}
		})
	}
}

func TestRetryTxRunsTheUnitOnceWhenItSucceeds(t *testing.T) {
	run, runs := failNTimes(0, aborted(data.SQLStateDeadlockDetected))
	var waits []time.Duration

	if err := data.RetryTxLoop(t.Context(), data.DefaultTxAttempts, noSleep(&waits), run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *runs != 1 {
		t.Errorf("want exactly one attempt; got %d", *runs)
	}
}

// Zero attempts is an unset value, not a request to do nothing: running the
// transaction no times at all would be a silent no-op reported as success.
func TestRetryTxRunsAtLeastOnce(t *testing.T) {
	run, runs := failNTimes(0, nil)
	var waits []time.Duration

	if err := data.RetryTxLoop(t.Context(), 0, noSleep(&waits), run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *runs != 1 {
		t.Errorf("want the unit run once; got %d", *runs)
	}
}

// A caller whose budget ran out while waiting gets the database error, not the
// timeout: it is the one that says what went wrong.
func TestRetryTxStopsWhenTheWaitIsInterrupted(t *testing.T) {
	deadlock := aborted(data.SQLStateDeadlockDetected)
	run, runs := failNTimes(99, deadlock)

	interrupted := func(context.Context, time.Duration) error { return context.DeadlineExceeded }

	err := data.RetryTxLoop(t.Context(), data.DefaultTxAttempts, interrupted, run)
	if !errors.Is(err, deadlock) {
		t.Fatalf("want the database error rather than the timeout; got %v", err)
	}
	if *runs != 1 {
		t.Errorf("want no attempt after the interrupted wait; got %d", *runs)
	}
}

// Without jitter every writer that collided at one moment retries at that same
// moment, reproducing the contention exactly.
func TestTxBackoffIsJitteredAndGrows(t *testing.T) {
	seen := map[time.Duration]bool{}
	for range 50 {
		d := data.TxBackoffForTest(0)
		if d < 25*time.Millisecond || d >= 50*time.Millisecond {
			t.Fatalf("the first backoff must fall in [25ms, 50ms); got %v", d)
		}
		seen[d] = true
	}
	if len(seen) < 2 {
		t.Error("the backoff is not jittered: 50 draws produced one value")
	}

	for range 50 {
		if d := data.TxBackoffForTest(2); d < 100*time.Millisecond || d >= 200*time.Millisecond {
			t.Fatalf("the third backoff must fall in [100ms, 200ms); got %v", d)
		}
	}
}
