package slots

import (
	"slices"
	"testing"
	"time"
)

func TestValidFormat(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"00:00", true},
		{"09:30", true},
		{"23:59", true},
		{"24:00", false},
		{"23:60", false},
		{"9:30", false},
		{"09-30", false},
		{"", false},
		{"09:3", false},
		{"ab:cd", false},
	}

	for _, tt := range tests {
		if got := ValidFormat(tt.in); got != tt.want {
			t.Errorf("ValidFormat(%q) = %v; want %v", tt.in, got, tt.want)
		}
	}
}

// FINDING 88. Every string below is five bytes with a colon in the middle and
// is not HH:MM. All six were accepted by the fmt.Sscanf implementation, and
// each one is a different way its %d verb was lenient: a leading sign, a
// leading space, an interior sign, an interior space, a trailing space, and a
// trailing non-digit.
//
// The second column is what ToMinutes makes of the same string, and it is the
// reason accepting them was worse than rejecting them: the value the caller
// believed it had validated is not the value the rest of the system acts on.
// Both columns are asserted, so this test still fails if ValidFormat is
// tightened and ToMinutes silently changes underneath it.
func TestValidFormatRejectsWhatItWouldOtherwiseReinterpret(t *testing.T) {
	tests := []struct {
		in              string
		reinterpretedAs int
	}{
		{"+9:30", 570}, // 09:30
		{" 9:30", 570}, // 09:30
		{"09:+5", 545}, // 09:05
		{"09: 5", 545}, // 09:05
		{"09:5 ", 545}, // 09:05
		{"12:3x", 723}, // 12:03
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if ValidFormat(tt.in) {
				t.Errorf("ValidFormat(%q) = true; %q is not HH:MM and is read as %s",
					tt.in, tt.in, FromMinutes(ToMinutes(tt.in)))
			}
			// Naming the reinterpretation is the point: were ValidFormat to
			// accept it again, this is the time the booking would actually be
			// made for.
			if got := ToMinutes(tt.in); got != tt.reinterpretedAs {
				t.Errorf("ToMinutes(%q) = %d, want %d — the reinterpretation this test is about changed",
					tt.in, got, tt.reinterpretedAs)
			}
		})
	}
}

func TestToMinutes(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"00:00", 0},
		{"01:00", 60},
		{"09:30", 570},
		{"23:59", 1439},
	}

	for _, tt := range tests {
		if got := ToMinutes(tt.in); got != tt.want {
			t.Errorf("ToMinutes(%q) = %d; want %d", tt.in, got, tt.want)
		}
	}
}

func TestFromMinutes(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "00:00"},
		{60, "01:00"},
		{570, "09:30"},
		{1439, "23:59"},
		{1440, "00:00"}, // wraps at midnight
		{1500, "01:00"}, // and keeps wrapping
	}

	for _, tt := range tests {
		if got := FromMinutes(tt.in); got != tt.want {
			t.Errorf("FromMinutes(%d) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name    string
		start   string
		minutes int
		want    string
	}{
		{"within the day", "09:00", 90, "10:30"},
		{"exactly on the hour", "18:00", 60, "19:00"},
		{"crossing midnight", "23:30", 60, "00:30"},
		{"landing on midnight", "23:00", 60, "00:00"},
		{"zero is identity", "14:15", 0, "14:15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Add(tt.start, tt.minutes); got != tt.want {
				t.Errorf("Add(%q, %d) = %q; want %q", tt.start, tt.minutes, got, tt.want)
			}
		})
	}
}

// A slot time survives a round trip through minutes unchanged, which is what
// lets availability build a day's grid by stepping in minutes and rendering back.
func TestRoundTrip(t *testing.T) {
	for m := 0; m < minutesPerDay; m++ {
		if got := ToMinutes(FromMinutes(m)); got != m {
			t.Fatalf("round trip lost %d minutes: got %d", m, got)
		}
	}
}

func TestDayName(t *testing.T) {
	tests := map[time.Weekday]string{
		time.Monday: "monday", time.Tuesday: "tuesday", time.Wednesday: "wednesday",
		time.Thursday: "thursday", time.Friday: "friday",
		time.Saturday: "saturday", time.Sunday: "sunday",
	}

	for wd, want := range tests {
		if got := DayName(wd); got != want {
			t.Errorf("DayName(%v) = %q; want %q", wd, got, want)
		}
	}

	// Schedules are keyed by these strings, so the lowercase form is the
	// contract, not a formatting preference.
	if got := DayName(time.Monday); got == time.Monday.String() {
		t.Error("DayName must return the lowercase schedule key, not Go's own name")
	}
}

func TestToHours(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"00:00", 0},
		{"08:00", 8},
		{"08:30", 8.5},
		{"23:45", 23.75},
		{"garbage", 0},
	}

	for _, tt := range tests {
		if got := ToHours(tt.in); got != tt.want {
			t.Errorf("ToHours(%q) = %v; want %v", tt.in, got, tt.want)
		}
	}
}

// Overlap moved here from internal/courts, where it was slotsCollide, when a
// third copy of the same expression was about to be written in the booking
// write paths. The table is the one that guarded it there, plus the two
// boundaries the write paths depend on: touching ranges do not overlap, so a
// booking may start exactly when a blocked slot ends.
func TestOverlap(t *testing.T) {
	tests := []struct {
		name                       string
		startA, endA, startB, endB string
		want                       bool
	}{
		{"overlapping", "08:00", "09:30", "09:00", "10:30", true},
		{"adjacent no collision", "08:00", "09:00", "09:00", "10:00", false},
		{"adjacent the other way", "09:00", "10:00", "08:00", "09:00", false},
		{"contained", "08:00", "12:00", "09:00", "10:00", true},
		{"identical", "08:00", "09:30", "08:00", "09:30", true},
		{"no overlap", "08:00", "09:00", "10:00", "11:00", false},
		{"B before A", "10:00", "11:00", "08:00", "09:00", false},
		{"partial tail", "08:00", "09:30", "09:29", "10:00", true},
		{"one minute of contact", "08:00", "09:01", "09:00", "10:00", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Overlap(tt.startA, tt.endA, tt.startB, tt.endB)
			if got != tt.want {
				t.Errorf("Overlap(%q-%q, %q-%q) = %v, want %v",
					tt.startA, tt.endA, tt.startB, tt.endB, got, tt.want)
			}
		})
	}
}

// The permitted slot lengths are the grid's intervals, and a caller that could
// alter the set for everyone else would change what every court in the product
// is sold in.
func TestPermittedDurationsCannotBeAlteredByACaller(t *testing.T) {
	first := PermittedDurations()
	first[0] = 7

	if second := PermittedDurations(); second[0] == 7 {
		t.Error("PermittedDurations handed out a slice a caller could write through")
	}
	if !slices.Contains(PermittedDurations(), DefaultDuration) {
		t.Errorf("the default slot length %d is not one of the permitted ones %v",
			DefaultDuration, PermittedDurations())
	}
}

// At is the non-wrapping counterpart to FromMinutes, and these are the cases
// that distinguish them.
func TestAtRollsIntoTheNextDayInsteadOfWrapping(t *testing.T) {
	// 2026-08-28 is a Friday.
	friday := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		hhmm      string
		add       time.Duration
		wantDay   time.Weekday
		wantClock string
	}{
		{"start of day", "00:00", 0, time.Friday, "00:00"},
		{"late evening", "23:00", 0, time.Friday, "23:00"},
		{"half an hour past 23:45 lands tomorrow", "23:45", 30 * time.Minute, time.Saturday, "00:15"},
		{"a two-hour booking from 23:00 ends on Saturday", "23:00", 2 * time.Hour, time.Saturday, "01:00"},
		{"exactly midnight is already the next day", "23:30", 30 * time.Minute, time.Saturday, "00:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := At(friday, tt.hhmm).Add(tt.add)
			if got.Weekday() != tt.wantDay {
				t.Errorf("weekday = %v, want %v", got.Weekday(), tt.wantDay)
			}
			if clock := got.Format("15:04"); clock != tt.wantClock {
				t.Errorf("clock = %s, want %s", clock, tt.wantClock)
			}
		})
	}
}

// The contrast, stated as a test so nobody "simplifies" At back into it:
// FromMinutes answers what a clock reads and therefore wraps, which is why it
// cannot be used to walk a span.
func TestFromMinutesWrapsWhereAtDoesNot(t *testing.T) {
	if got := FromMinutes(24 * 60); got != "00:00" {
		t.Errorf("FromMinutes(1440) = %s, want 00:00 — it is modular on purpose", got)
	}
}
