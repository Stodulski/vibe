package data

import (
	"context"
	"testing"
	"time"
)

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
