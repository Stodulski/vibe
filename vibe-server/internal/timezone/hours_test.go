package timezone

import (
	"testing"
	"time"
)

// The product renders a booking's hours in six places — two confirmation
// paths, two cancellations, the 2h reminder and the exported spreadsheet — and
// they all go through HoursLabel. These are its cases.

func TestHoursLabel(t *testing.T) {
	t.Parallel()

	// The instants are built in UTC on purpose. Every one of them is the same
	// moment as the Argentina reading in the want column, and a function that
	// formatted on the process's clock would answer three hours out for all of
	// them — which is precisely the failure this exists to prevent, since the
	// server runs in UTC and the person reading the string is at the court.
	cases := []struct {
		name       string
		start, end time.Time
		want       string
	}{
		{
			"an ordinary booking inside one day",
			time.Date(2026, 3, 18, 21, 0, 0, 0, time.UTC),  // 18:00 ART
			time.Date(2026, 3, 18, 22, 30, 0, 0, time.UTC), // 19:30 ART
			"18:00 a 19:30",
		},
		{
			"a booking that crosses midnight",
			time.Date(2026, 3, 19, 2, 0, 0, 0, time.UTC), // 23:00 ART on the 18th
			time.Date(2026, 3, 19, 4, 0, 0, 0, time.UTC), // 01:00 ART on the 19th
			"23:00 a 01:00 (día sig.)",
		},
		{
			"a booking that ends exactly at midnight",
			time.Date(2026, 3, 19, 2, 0, 0, 0, time.UTC), // 23:00 ART
			time.Date(2026, 3, 19, 3, 0, 0, 0, time.UTC), // 00:00 ART, next day
			"23:00 a 00:00 (día sig.)",
		},
		{
			// The near miss the marker must not fire on: 22:00 to 23:30 in
			// Argentina is 01:00 to 02:30 UTC on the following calendar day.
			// Compared in UTC this booking would be marked as ending tomorrow.
			"a late booking that stays inside its own day",
			time.Date(2026, 3, 19, 1, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 19, 2, 30, 0, 0, time.UTC),
			"22:00 a 23:30",
		},
		{"no span at all", time.Time{}, time.Time{}, ""},
		{"half a span", time.Date(2026, 3, 18, 21, 0, 0, 0, time.UTC), time.Time{}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := HoursLabel(tc.start, tc.end); got != tc.want {
				t.Errorf("HoursLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

// Day is what puts a `date` column on the same timeline as an instant. Both
// arrive in Go as time.Time and neither the compiler nor an eyeball on the
// calendar fields can tell them apart, which is how a block filed 00:00-01:00
// came to be compared as 21:00-22:00 the previous evening.
func TestDayAnchorsACalendarDateAtTheVenue(t *testing.T) {
	t.Parallel()

	// The shape pgx hands back for a `date` column, and the shape time.Parse
	// gives for the same string.
	utcMidnight := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)

	day := Day(utcMidnight)
	if y, m, d := day.Date(); y != 2026 || m != time.March || d != 18 {
		t.Fatalf("Day changed the calendar date: got %s", day.Format(time.RFC3339))
	}
	if day.Location() != Argentina {
		t.Errorf("Day = %s, want it anchored in %s", day.Format(time.RFC3339), Argentina)
	}
	if day.Equal(utcMidnight) {
		t.Error("Day returned the same instant it was given; the anchoring is the whole point " +
			"— midnight in Argentina is three hours after midnight UTC")
	}

	// Idempotent: a value already anchored here comes back unchanged, so a
	// caller cannot make things worse by anchoring twice.
	if again := Day(day); !again.Equal(day) {
		t.Errorf("Day is not idempotent: %s then %s", day.Format(time.RFC3339), again.Format(time.RFC3339))
	}
}
