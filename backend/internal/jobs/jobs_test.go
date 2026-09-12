package jobs_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/jobs"
)

// notAttempted is the structural form of "never reached the provider" —
// internal/mailer answers this way rather than importing the queue.
type notAttempted struct{ error }

func (notAttempted) NotAttempted() bool { return true }

type permanent struct{ error }

func (permanent) Permanent() bool { return true }

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want jobs.Outcome
	}{
		{"no error is done", nil, jobs.OutcomeDone},
		{"an unrecognised failure is retried", errors.New("smtp: 451"), jobs.OutcomeRetry},
		{"the not-attempted sentinel costs no attempt", fmt.Errorf("send: %w", jobs.ErrNotAttempted), jobs.OutcomeNotAttempted},
		{"an open breaker costs no attempt", fmt.Errorf("mp: %w", circuitbreaker.ErrOpen), jobs.OutcomeNotAttempted},
		{"a structural not-attempted costs no attempt", notAttempted{errors.New("breaker")}, jobs.OutcomeNotAttempted},
		{"the permanent sentinel dead-letters", fmt.Errorf("send: %w", jobs.ErrPermanent), jobs.OutcomePermanent},
		{"a structural permanent dead-letters", permanent{errors.New("invalid address")}, jobs.OutcomePermanent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jobs.Classify(tt.err); got != tt.want {
				t.Errorf("Classify(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestBackoffStaysInsideItsJitterBand is OUT-02 stated as a property. The
// point of the jitter is that a backlog of rows that failed against one
// provider outage does not come back at the same instant, so what matters is
// that the delay moves and that it stays within a fifth of the table entry
// either way — a spread wider than that would be a different schedule, and one
// narrower would not spread anything.
func TestBackoffStaysInsideItsJitterBand(t *testing.T) {
	table := []time.Duration{time.Minute, 10 * time.Minute}

	for attempts := 1; attempts <= 3; attempts++ {
		want := table[min(attempts-1, len(table)-1)]
		low := time.Duration(float64(want) * (1 - jobs.JitterFraction))
		high := time.Duration(float64(want) * (1 + jobs.JitterFraction))

		seen := make(map[time.Duration]struct{})
		for range 200 {
			got := jobs.Backoff(table, attempts)
			if got < low || got > high {
				t.Fatalf("attempt %d: Backoff = %v, outside [%v, %v]", attempts, got, low, high)
			}
			seen[got] = struct{}{}
		}
		if len(seen) < 2 {
			t.Errorf("attempt %d: 200 calls produced %d distinct delays; the table is still fixed and a backlog still stampedes",
				attempts, len(seen))
		}
	}
}

// TestBackoffPastTheEndOfTheTableWaitsTheLastEntry pins what happens to a job
// with a budget longer than the ladder.
func TestBackoffPastTheEndOfTheTableWaitsTheLastEntry(t *testing.T) {
	table := []time.Duration{time.Second, time.Hour}
	got := jobs.Backoff(table, 9)
	if got < time.Duration(float64(time.Hour)*(1-jobs.JitterFraction)) {
		t.Errorf("Backoff past the table = %v, want about the last entry (%v)", got, time.Hour)
	}
}

// TestDedupKeySeparatesWhatMustNotCollide covers the three ways one key would
// silently drop somebody's notification.
func TestDedupKeySeparatesWhatMustNotCollide(t *testing.T) {
	base := jobs.DedupKey("email:booking_confirmation", "ana@example.com", "booking-1")

	if again := jobs.DedupKey("email:booking_confirmation", "ana@example.com", "booking-1"); again != base {
		t.Error("the same delivery produced two keys; a redelivery would be recorded as new work")
	}
	for _, other := range []string{
		jobs.DedupKey("wa:booking_confirmation", "ana@example.com", "booking-1"),
		jobs.DedupKey("email:booking_confirmation", "beto@example.com", "booking-1"),
		jobs.DedupKey("email:booking_confirmation", "ana@example.com", "booking-2"),
	} {
		if other == base {
			t.Error("two different deliveries share a key; one of them is never sent")
		}
	}

	// The separator is what stops ("a", "bc") and ("ab", "c") hashing alike.
	if jobs.DedupKey("t", "a", "bc") == jobs.DedupKey("t", "ab", "c") {
		t.Error("the key parts are concatenated without a separator")
	}
}
