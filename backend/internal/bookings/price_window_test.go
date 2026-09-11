package bookings

import (
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
)

// 2026-08-27 is a Thursday and 2026-08-28 a Friday.
var (
	thursday = time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	friday   = time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
)

func week(rows map[string][2]string) []*data.Schedule {
	all := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	out := make([]*data.Schedule, 0, len(all))
	for _, d := range all {
		hours, ok := rows[d]
		if !ok {
			out = append(out, &data.Schedule{Day: d, IsClosed: true})
			continue
		}
		out = append(out, &data.Schedule{Day: d, OpenTime: hours[0], CloseTime: hours[1]})
	}
	return out
}

func TestPriceWindowPicksTheWindowTheHourWasSoldFrom(t *testing.T) {
	// Thursday trades into Friday morning; Friday is an ordinary day.
	schedules := week(map[string][2]string{
		"thursday": {"08:00", "01:30"},
		"friday":   {"08:00", "23:00"},
	})

	tests := []struct {
		name    string
		date    time.Time
		start   string
		wantDay string
		wantMin int
	}{
		{"an ordinary Thursday hour", thursday, "20:00", "thursday", 1200},
		{"the hour that crosses", thursday, "23:30", "thursday", 1410},
		{"Friday 00:30 belongs to Thursday night", friday, "00:30", "thursday", 1470},
		{"Friday 01:00, the last of that night", friday, "01:00", "thursday", 1500},
		// No window covers it, so it falls back to its own weekday's card —
		// staff book outside opening hours and a rate card is per weekday.
		{"Friday 02:00, after that night closed and before Friday opens", friday, "02:00", "friday", 120},
		{"an ordinary Friday hour", friday, "10:00", "friday", 600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := priceWindow(schedules, tt.date, tt.start)
			if got.Day != tt.wantDay || got.StartMin != tt.wantMin {
				t.Errorf("got {%s %d}, want {%s %d}", got.Day, got.StartMin, tt.wantDay, tt.wantMin)
			}
		})
	}
}

// The tie-break the product chose. Friday opens at 00:00, so 00:30 is inside
// Friday's own window too — and it is still Thursday's, because Thursday's
// window has not closed. Once it has, the same clock reading is Friday's.
func TestPriceWindowPrefersTheNightStillOpen(t *testing.T) {
	schedules := week(map[string][2]string{
		"thursday": {"08:00", "01:30"},
		"friday":   {"00:00", "23:00"},
	})

	got := priceWindow(schedules, friday, "00:30")
	if got.Day != "thursday" || got.StartMin != 1470 {
		t.Errorf("while Thursday is still open, 00:30 is Thursday minute 1470; got {%s %d}", got.Day, got.StartMin)
	}

	after := priceWindow(schedules, friday, "02:00")
	if after.Day != "friday" || after.StartMin != 120 {
		t.Errorf("Thursday closed at 01:30, so 02:00 is Friday minute 120; got {%s %d}", after.Day, after.StartMin)
	}
}

// A closed day has no window, and its hours still have a card. Staff book days
// the venue is shut, and court_prices is keyed by weekday rather than by
// opening — so the hour prices from Thursday's own rules, at the clock reading
// it was booked at.
func TestPriceWindowFallsBackToTheWeekdayWhenNoWindowClaimsTheHour(t *testing.T) {
	schedules := week(map[string][2]string{"friday": {"08:00", "23:00"}})

	got := priceWindow(schedules, thursday, "20:00")
	if got.Day != "thursday" || got.StartMin != 1200 {
		t.Errorf("a shut Thursday still prices from Thursday's card at 20:00; got {%s %d}", got.Day, got.StartMin)
	}
}
