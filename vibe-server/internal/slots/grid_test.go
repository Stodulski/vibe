package slots

import (
	"errors"
	"testing"
)

// starts is the shape every assertion below is about: the grid as a client
// would see it listed, for a booking of the given duration.
func starts(g Grid, duration int) []string {
	out := make([]string, 0)
	for _, s := range g.Slots(duration) {
		out = append(out, s.Start)
	}
	return out
}

// The grid now offers a start every gridStep (30) minutes regardless of the
// requested duration, not every duration minutes: a 60-minute court used to
// step by 60, which is exactly what made a 60-minute booking ending at 09:00
// unreachable by one starting there.
func TestGridLaysSlotsOnA30MinuteStepRegardlessOfDuration(t *testing.T) {
	g := NewGrid("08:00", "10:00")

	got := starts(g, 60)
	want := []string{"08:00", "08:30", "09:00"}
	if len(got) != len(want) {
		t.Fatalf("want %d slots %v; got %d %v", len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot %d: want %s; got %s", i, want[i], got[i])
		}
	}
	if g.Slots(60)[2].End != "10:00" {
		t.Errorf("the last slot must end at closing; got %s", g.Slots(60)[2].End)
	}
}

// A slot that would run past closing is not a slot. The remainder of the day is
// not sold as a short session.
func TestGridStopsBeforeAPartialSlot(t *testing.T) {
	g := NewGrid("08:00", "09:30")

	got := starts(g, 60)
	want := []string{"08:00", "08:30"}
	if len(got) != len(want) {
		t.Fatalf("an hour and a half of a 60-minute booking fits two starts on the 30-minute step; want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot %d: want %s; got %s", i, want[i], got[i])
		}
	}
}

func TestGridWithNoRoomForASlotIsEmptyRatherThanNil(t *testing.T) {
	g := NewGrid("08:00", "09:00")

	got := g.Slots(120)
	if got == nil {
		t.Fatal("Slots must return an empty list a caller can render, never nil")
	}
	if len(got) != 0 {
		t.Errorf("a 120-minute booking cannot be sold in a one-hour window; got %v", starts(g, 120))
	}
}

// FINDING 2, and its resolution. A venue open past midnight had its closing
// time carried past 1440 and slots generated all the way to it, then rendered
// through a helper that wraps modulo 1440. A 23:00 start came out looking like
// an ordinary slot and the booking path refused every one of them, because a
// booking was one row on one date whose end_time had to exceed its start_time.
//
// The grid stopped at midnight for that reason and the venue lost its late
// hours. It carries a range now (the generated span) and the check that forbade
// the shape is gone, so the grid runs to closing and those hours are on
// sale — which is what a venue advertising 21:00-02:00 meant all along.
func TestGridSellsThroughMidnightToClosing(t *testing.T) {
	g := NewGrid("21:00", "02:00")

	got := starts(g, 60)
	want := []string{"21:00", "21:30", "22:00", "22:30", "23:00", "23:30", "00:00", "00:30", "01:00"}
	if len(got) != len(want) {
		t.Fatalf("a venue open 21:00-02:00 sells every hour it trades; want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot %d: want %s; got %s", i, want[i], got[i])
		}
	}

	// The ends past midnight read as the next day's clock, not as 24:00 or
	// more — a clock reading is all end_time ever was.
	last := g.Slots(60)[len(got)-1]
	if last.Start != "01:00" || last.End != "02:00" {
		t.Errorf("the last slot is 01:00-02:00; got %s-%s", last.Start, last.End)
	}
}

// A venue closing at midnight sells right up to it. This used to stop at 22:30
// on the grounds that a 23:00-00:00 row could not be stored.
func TestGridSellsTheSlotEndingAtMidnight(t *testing.T) {
	g := NewGrid("22:00", "00:00")

	got := starts(g, 60)
	want := []string{"22:00", "22:30", "23:00"}
	if len(got) != len(want) {
		t.Fatalf("22:00-00:00 sells three 60-minute starts; want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot %d: want %s; got %s", i, want[i], got[i])
		}
	}
}

// FINDING 5. Every start the storefront publishes must validate on the write
// path, and nothing else may. This is the property that makes the two agree by
// construction rather than by two implementations that happen to match.
func TestEveryPublishedSlotValidatesAndNothingBetweenThemDoes(t *testing.T) {
	g := NewGrid("08:00", "22:00")

	for _, s := range g.Slots(90) {
		if err := g.Validate(s.Start, 90); err != nil {
			t.Errorf("the grid published %s but the write path would refuse it: %v", s.Start, err)
		}
	}

	// 08:15 is not on the 30-minute grid step at all. Booking it would defeat a
	// slot lock keyed on an exact start time.
	if err := g.Validate("08:15", 90); !errors.Is(err, ErrOffGrid) {
		t.Errorf("a start not on the 30-minute step must be refused as off-grid; got %v", err)
	}
}

func TestValidateNamesTheReasonItRefuses(t *testing.T) {
	tests := []struct {
		name         string
		open, closes string
		start        string
		duration     int
		want         error
	}{
		{"on the grid", "08:00", "22:00", "09:30", 90, nil},
		{"a 60-minute booking on the same grid", "08:00", "22:00", "09:30", 60, nil},
		{"a 120-minute booking on the same grid", "08:00", "22:00", "09:30", 120, nil},
		{"before opening", "08:00", "22:00", "07:00", 90, ErrOutsideSchedule},
		{"running past closing", "08:00", "22:00", "21:00", 90, ErrOutsideSchedule},
		{"off the 30-minute step", "08:00", "22:00", "08:15", 90, ErrOffGrid},
		{"reaching midnight, inside a schedule that closes there", "08:00", "00:00", "23:00", 60, nil},
		{"crossing midnight, inside a schedule that trades past it", "18:00", "02:00", "23:30", 60, nil},
		{"a longer booking crossing midnight", "18:00", "02:00", "22:00", 120, nil},
		{"past an after-midnight closing", "18:00", "02:00", "01:30", 60, ErrOutsideSchedule},
		{"zero duration", "08:00", "22:00", "08:00", 0, ErrOffGrid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewGrid(tt.open, tt.closes).Validate(tt.start, tt.duration)
			if !errors.Is(err, tt.want) {
				t.Errorf("Validate(%q, %d) on %s-%s = %v; want %v",
					tt.start, tt.duration, tt.open, tt.closes, err, tt.want)
			}
		})
	}
}

// A venue trading until 02:00 is genuinely open at 23:30, and now sells it. The
// test this replaced asserted the opposite and explained why the refusal named
// midnight rather than the schedule: the owner's hours were already correct and
// only the row shape could not hold the booking. The row holds it now.
func TestAnAfterMidnightScheduleSellsItsLateHours(t *testing.T) {
	g := NewGrid("18:00", "02:00")

	if err := g.Validate("23:30", 60); err != nil {
		t.Errorf("23:30 is inside a 18:00-02:00 schedule and must sell; got %v", err)
	}
}

// Owns answers which window an hour was sold out of, and where in that window
// it sits. The second half is the part a clock cannot give you: 00:30 reads the
// same whether it is early morning or the tail of last night.
func TestOwnsPlacesAnHourInsideItsWindow(t *testing.T) {
	night := NewGrid("08:00", "01:30") // closes after midnight
	day := NewGrid("08:00", "23:00")   // an ordinary day

	tests := []struct {
		name    string
		grid    Grid
		hhmm    string
		wantMin int
		wantOK  bool
	}{
		{"an ordinary hour is where the clock says", day, "09:30", 570, true},
		{"the same hour in a night window", night, "09:30", 570, true},
		{"after midnight is not this day's hour, however late the window runs", night, "00:30", 0, false},
		{"past the night's close", night, "02:00", 0, false},
		{"before the window opens", night, "07:00", 0, false},
		{"after midnight is nothing to a window that closed at 23:00", day, "00:30", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.grid.OwnsSameDay(tt.hhmm)
			if ok != tt.wantOK {
				t.Fatalf("OwnsSameDay(%q) ok = %v, want %v", tt.hhmm, ok, tt.wantOK)
			}
			if ok && got != tt.wantMin {
				t.Errorf("OwnsSameDay(%q) = %d, want %d", tt.hhmm, got, tt.wantMin)
			}
		})
	}
}

// The tie-break, decided at the product level: while last night's window is
// still open, an hour inside it belongs to that night — even once the new day
// has opened too. A session does not become the next day's at midnight just
// because the venue also opens then.
func TestOwnsPrefersTheNightStillOpen(t *testing.T) {
	night := NewGrid("08:00", "01:30")
	earlyDay := NewGrid("00:00", "23:00")

	// 00:30 falls inside both windows.
	nightMin, nightOK := night.OwnsNextDay("00:30")
	dayMin, dayOK := earlyDay.OwnsSameDay("00:30")
	if !nightOK || !dayOK {
		t.Fatalf("the fixture must be ambiguous; night=%v day=%v", nightOK, dayOK)
	}
	if nightMin != 1470 {
		t.Errorf("inside the night it is minute 1470; got %d", nightMin)
	}
	if dayMin != 30 {
		t.Errorf("inside its own day it is minute 30; got %d", dayMin)
	}
	// Which of the two wins is the caller's rule, not the grid's — the grid
	// only says where the hour sits in each. See priceWindow in
	// internal/bookings/grid.go, which asks the night first.
}
