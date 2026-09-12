package bookings

import (
	"time"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/slots"
)

// PriceWindow is the opening window an hour was sold out of, and where in that
// window the hour sits.
//
// Both halves are needed to price it. `Day` selects the rate card — a booking
// is priced by the window it came from, not by the date the calendar had rolled
// over to — and `StartMin` is minutes from that window's own day's midnight,
// which exceeds 1440 for an hour past the rollover. Thursday's 00:30 slot is
// {Day: "thursday", StartMin: 1470}, and Thursday's bands are laid out across
// that same span (the span_min generated column in db/migrations/001_init.sql).
type PriceWindow struct {
	Day      string
	StartMin int
}

// priceWindow resolves which day's rate card prices an hour.
//
// The night still open wins. If Thursday trades 08:00 to 01:30 and Friday opens
// at 00:00, a Friday 00:30 booking falls inside both windows — and it is
// Thursday's, because a session does not become the next day's at midnight just
// because the venue also opened then. Only once Thursday's window has closed
// does an hour belong to Friday.
//
// The previous day is therefore asked first, and its answer is taken whenever
// it has one. A venue whose days do not touch never reaches that branch.
//
// It always resolves. An hour no window claims still has a rate card — the one
// for its own weekday — because staff book days the venue is shut and a card is
// per weekday, not per opening. Returning "no window" would have made every
// such booking unpriceable, which is a different rule than the one chosen.
func priceWindow(schedules []*complexstore.Schedule, date time.Time, startTime string) PriceWindow {
	// Last night first, asked as "are you still open at this hour tomorrow".
	previous := slots.DayName(date.AddDate(0, 0, -1).Weekday())
	if grid, ok := gridForDay(schedules, previous); ok {
		if min, owned := grid.OwnsNextDay(startTime); owned {
			return PriceWindow{Day: previous, StartMin: min}
		}
	}

	// Then the date's own window, asked about its own hours.
	own := slots.DayName(date.Weekday())
	if grid, ok := gridForDay(schedules, own); ok {
		if min, owned := grid.OwnsSameDay(startTime); owned {
			return PriceWindow{Day: own, StartMin: min}
		}
	}

	// No window claims the hour. That is not the same as "unpriceable": staff
	// may book a day the venue is shut, and a weekday's rate card exists
	// whether or not its doors opened. So the hour falls back to its own
	// calendar day, read straight off the clock.
	//
	// The public path never reaches this — it is gated by the grid, which only
	// offers hours a window produced — so the fallback answers for the owner's
	// dashboard alone, which is the only caller that books outside opening
	// hours in the first place.
	return PriceWindow{Day: slots.DayName(date.Weekday()), StartMin: slots.ToMinutes(startTime)}
}

// gridForDay finds one weekday's window. A closed day has no window, and a
// weekday with no row at all is the same fact — nothing says the venue opens.
func gridForDay(schedules []*complexstore.Schedule, day string) (slots.Grid, bool) {
	for _, s := range schedules {
		if s.Day != day {
			continue
		}
		if s.IsClosed {
			return slots.Grid{}, false
		}
		return slots.NewGrid(s.OpenTime, s.CloseTime), true
	}
	return slots.Grid{}, false
}
