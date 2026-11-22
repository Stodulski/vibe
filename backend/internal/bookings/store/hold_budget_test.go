package store

import (
	"testing"
	"time"
)

// S2: the budget an address has on some court hours is the time it actually
// held them, not the wall time since it first did.
func TestBudgetFrom(t *testing.T) {
	const window = 10 * time.Minute
	now := time.Date(2026, 11, 21, 18, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	span := func(from, to time.Duration) heldSpan { return heldSpan{start: ago(from), end: ago(to)} }

	tests := []struct {
		name      string
		spans     []heldSpan
		remaining time.Duration
		until     time.Time // zero: not refused
	}{
		{name: "never held: a whole window", remaining: window},
		{
			name:      "let go after two minutes and back seven later: the eight it did not use",
			spans:     []heldSpan{span(9*time.Minute, 7*time.Minute)},
			remaining: 8 * time.Minute,
		},
		{
			name:      "released and retaken twice: what is left of the window",
			spans:     []heldSpan{span(6*time.Minute, 3*time.Minute), span(3*time.Minute, time.Minute)},
			remaining: 5 * time.Minute,
		},
		{
			name:  "held a whole window that ended a minute ago: waits the cooldown from its end",
			spans: []heldSpan{span(11*time.Minute, time.Minute)},
			until: ago(time.Minute).Add(HoldCooldown),
		},
		{
			name: "converted past a window and released: waits the cooldown from the release",
			// Nine minutes as a hold, then ten more after submitting the details.
			spans: []heldSpan{span(19*time.Minute+30*time.Second, 30*time.Second)},
			until: ago(30 * time.Second).Add(HoldCooldown),
		},
		{
			name:  "less than a minute left: no hold too short to use",
			spans: []heldSpan{span(10*time.Minute, 30*time.Second)},
			until: ago(30 * time.Second).Add(HoldCooldown),
		},
		{
			name:      "the cooldown has passed since it last held them: afresh",
			spans:     []heldSpan{span(25*time.Minute, 15*time.Minute)},
			remaining: window,
		},
		{
			name: "a pause of a cooldown between holds starts the count afresh",
			spans: []heldSpan{
				span(40*time.Minute, 31*time.Minute),
				span(8*time.Minute, 5*time.Minute),
			},
			remaining: 7 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := budgetFrom(tt.spans, now, window)
			if tt.until.IsZero() {
				if got.refused || got.remaining != tt.remaining {
					t.Errorf("want %v left; got %+v", tt.remaining, got)
				}
				return
			}
			if !got.refused || !got.until.Equal(tt.until) {
				t.Errorf("want refused until %v; got %+v", tt.until, got)
			}
		})
	}
}
