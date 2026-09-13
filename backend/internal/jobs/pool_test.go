package jobs

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestConfigLeaseFloorGrowsWithAPerTypeTimeout proves the Lease floor is
// derived from the longest attempt timeout in effect — a per-type Timeouts
// entry included — not from JobTimeout alone. A type given more time than
// JobTimeout must still be able to finish a claim before another worker
// reclaims it.
func TestConfigLeaseFloorGrowsWithAPerTypeTimeout(t *testing.T) {
	cfg := Config{
		JobTimeout: 10 * time.Second,
		Timeouts: map[string]time.Duration{
			"export:payments": 60 * time.Second,
		},
	}
	cfg.applyDefaults()

	if want := 6 * 60 * time.Second; cfg.Lease != want {
		t.Errorf("Lease = %s, want %s (6x the longest timeout, not 6x JobTimeout)", cfg.Lease, want)
	}
}

// TestConfigLeaseFloorIgnoresAShorterPerTypeTimeout proves a Timeouts entry
// shorter than JobTimeout never shrinks the floor below 6x JobTimeout.
func TestConfigLeaseFloorIgnoresAShorterPerTypeTimeout(t *testing.T) {
	cfg := Config{
		JobTimeout: 10 * time.Second,
		Timeouts: map[string]time.Duration{
			"quick": 2 * time.Second,
		},
	}
	cfg.applyDefaults()

	if want := 6 * 10 * time.Second; cfg.Lease != want {
		t.Errorf("Lease = %s, want %s (6x JobTimeout, unaffected by a shorter override)", cfg.Lease, want)
	}
}

// TestConfigLeaseFloorRespectsAnExplicitLeaseAboveTheFloor proves an
// explicitly configured Lease that already clears the derived floor is left
// alone.
func TestConfigLeaseFloorRespectsAnExplicitLeaseAboveTheFloor(t *testing.T) {
	cfg := Config{
		JobTimeout: 10 * time.Second,
		Lease:      120 * time.Second,
	}
	cfg.applyDefaults()

	if cfg.Lease != 120*time.Second {
		t.Errorf("Lease = %s, want the explicit 120s left untouched", cfg.Lease)
	}
}

// TestTimeoutForAppliesOnlyToItsOwnType proves timeoutFor — what Pool.run
// bounds a handler's context with — resolves a type's own Timeouts entry and
// falls back to JobTimeout for every type that has none, rather than the
// override leaking to jobs it was never registered for.
func TestTimeoutForAppliesOnlyToItsOwnType(t *testing.T) {
	p := NewPool(nil, Config{
		JobTimeout: 5 * time.Second,
		Timeouts: map[string]time.Duration{
			"export:payments": 45 * time.Second,
		},
	})

	if got := p.timeoutFor("export:payments"); got != 45*time.Second {
		t.Errorf("timeoutFor(export:payments) = %s, want 45s", got)
	}
	if got := p.timeoutFor("notifications:email"); got != 5*time.Second {
		t.Errorf("timeoutFor(notifications:email) = %s, want the shared 5s JobTimeout", got)
	}
	if got := p.timeoutFor("zero-override"); got != 5*time.Second {
		t.Errorf("timeoutFor with no entry = %s, want the shared 5s JobTimeout", got)
	}
}

// TestRunBoundsTheHandlerContextByTheTypesOwnTimeout proves the override
// actually reaches the context a handler runs under, not only Config's own
// bookkeeping. A "long" job outlives the shared JobTimeout because its own
// Timeouts entry covers it; a job of any other type is cancelled at
// JobTimeout, same as before this per-type override existed.
func TestRunBoundsTheHandlerContextByTheTypesOwnTimeout(t *testing.T) {
	p := NewPool(nil, Config{
		JobTimeout: 20 * time.Millisecond,
		Timeouts: map[string]time.Duration{
			"long": 200 * time.Millisecond,
		},
	})

	// Between the two timeouts: long enough that a handler bounded by the
	// shared 20ms JobTimeout is cancelled first, short enough that a handler
	// bounded by "long"'s own 200ms timeout is still running.
	const between = 80 * time.Millisecond

	stillRunning := func(ctx context.Context) bool {
		select {
		case <-time.After(between):
			return true
		case <-ctx.Done():
			return false
		}
	}

	longRan := make(chan bool, 1)
	_ = p.run(&Job{Type: "long"}, func(ctx context.Context, _ json.RawMessage) error {
		longRan <- stillRunning(ctx)
		return nil
	})
	if !<-longRan {
		t.Error("a \"long\" job was cancelled before its own 200ms Timeouts entry elapsed")
	}

	otherRan := make(chan bool, 1)
	_ = p.run(&Job{Type: "other"}, func(ctx context.Context, _ json.RawMessage) error {
		otherRan <- stillRunning(ctx)
		return nil
	})
	if <-otherRan {
		t.Error("a job with no Timeouts entry ran past the shared 20ms JobTimeout instead of being cancelled")
	}
}
