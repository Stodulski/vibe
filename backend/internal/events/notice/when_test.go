package notice

import (
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/timezone"
)

// S12: the reminder names the start on Argentina's calendar, whatever zone the
// process runs in: at 22:30 in Argentina it is already the next day in UTC, and
// "mañana" must still mean Argentina's tomorrow.
func TestWhenReadsTheDayOnArgentinasClock(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 30, 0, 0, timezone.Argentina) // 01:30 UTC on the 21st
	cases := []struct {
		name    string
		start   time.Time
		hasTime bool
		want    string
	}{
		{"later the same day", time.Date(2026, 11, 20, 23, 30, 0, 0, timezone.Argentina), true, "hoy viernes 20/11 a las 23:30"},
		{"the next day", time.Date(2026, 11, 21, 19, 0, 0, 0, timezone.Argentina).UTC(), true, "mañana sábado 21/11 a las 19:00"},
		{"a tournament day without an hour", time.Date(2026, 11, 21, 9, 0, 0, 0, timezone.Argentina), false, "mañana sábado 21/11"},
		{"further out", time.Date(2026, 11, 23, 9, 0, 0, 0, timezone.Argentina), true, "el lunes 23/11 a las 09:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := When(tc.start, tc.hasTime, now.UTC()); got != tc.want {
				t.Errorf("When = %q, want %q", got, tc.want)
			}
		})
	}
}
