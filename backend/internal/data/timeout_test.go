package data

import (
	"context"
	"testing"
	"time"
)

func TestDefaultTimeout_Value(t *testing.T) {
	if defaultTimeout != 3*time.Second {
		t.Errorf("defaultTimeout = %v, want %v", defaultTimeout, 3*time.Second)
	}
}

func TestTxTimeout_Value(t *testing.T) {
	if txTimeout != 5*time.Second {
		t.Errorf("txTimeout = %v, want %v", txTimeout, 5*time.Second)
	}
}

func TestQueryContext_SetsDeadline(t *testing.T) {
	parent := context.Background()
	before := time.Now()

	ctx, cancel := QueryContext(parent)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("QueryContext should set a deadline")
	}

	expectedMin := before.Add(defaultTimeout - 10*time.Millisecond)
	expectedMax := before.Add(defaultTimeout + 100*time.Millisecond)
	if deadline.Before(expectedMin) || deadline.After(expectedMax) {
		t.Errorf("deadline = %v, want between %v and %v", deadline, expectedMin, expectedMax)
	}
}

func TestTxContext_SetsDeadline(t *testing.T) {
	parent := context.Background()
	before := time.Now()

	ctx, cancel := TxContext(parent)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("TxContext should set a deadline")
	}

	expectedMin := before.Add(txTimeout - 10*time.Millisecond)
	expectedMax := before.Add(txTimeout + 100*time.Millisecond)
	if deadline.Before(expectedMin) || deadline.After(expectedMax) {
		t.Errorf("deadline = %v, want between %v and %v", deadline, expectedMin, expectedMax)
	}
}

func TestQueryContext_RespectsParentCancellation(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel() // Cancel parent immediately.

	ctx, cancel := QueryContext(parent)
	defer cancel()

	select {
	case <-ctx.Done():
		// Expected: child context should be done because parent is cancelled.
	default:
		t.Error("QueryContext child should be done when parent is cancelled")
	}
}

func TestTxContext_RespectsParentCancellation(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel()

	ctx, cancel := TxContext(parent)
	defer cancel()

	select {
	case <-ctx.Done():
		// Expected.
	default:
		t.Error("TxContext child should be done when parent is cancelled")
	}
}

func TestQueryContext_RespectsParentShorterDeadline(t *testing.T) {
	// If parent has a shorter deadline, QueryContext should inherit it.
	shortTimeout := 1 * time.Millisecond
	parent, parentCancel := context.WithTimeout(context.Background(), shortTimeout)
	defer parentCancel()

	ctx, cancel := QueryContext(parent)
	defer cancel()

	// The child deadline should be at most the parent's deadline.
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("QueryContext should have a deadline")
	}

	// Since parent timeout (1ms) < defaultTimeout (3s), deadline should be close to now.
	if time.Until(deadline) > defaultTimeout {
		t.Errorf("child deadline should not exceed parent's shorter deadline")
	}
}

func TestTxContext_TxTimeoutLongerThanDefault(t *testing.T) {
	if txTimeout <= defaultTimeout {
		t.Errorf("txTimeout (%v) should be longer than defaultTimeout (%v)", txTimeout, defaultTimeout)
	}
}

// The property the advisory-lock release was missing.
//
// context.WithoutCancel strips the parent's deadline along with its
// cancellation, so a cleanup query built on it alone had no bound at all —
// there is no statement_timeout anywhere in this repository to fall back on.
// Both halves are asserted here: detached from the parent's cancellation, and
// bounded by its own clock.
func TestDetachedQueryContext_IsBoundedButNotCancelledByItsParent(t *testing.T) {
	parent, parentCancel := context.WithTimeout(context.Background(), time.Millisecond)
	parentCancel()

	before := time.Now()
	ctx, cancel := DetachedQueryContext(parent)
	defer cancel()

	if err := ctx.Err(); err != nil {
		t.Errorf("a release must still run after its caller is done; the context was already %v", err)
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("DetachedQueryContext must carry a deadline: nothing else bounds the query it is handed to, " +
			"so without one a wedged database blocks the caller forever")
	}
	if latest := before.Add(defaultTimeout + 100*time.Millisecond); deadline.After(latest) {
		t.Errorf("deadline = %v, want no later than %v (defaultTimeout is %v)", deadline, latest, defaultTimeout)
	}
}
