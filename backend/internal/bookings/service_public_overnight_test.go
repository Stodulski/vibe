package bookings

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// overnightPublicBooking wires a complex that trades 20:00-02:00 every day and
// returns the day a week out, which the public storefront lists.
func overnightPublicBooking(f *fixture) (complexID, courtID uuid.UUID, day time.Time) {
	complexID, courtID = preparePublicBooking(f)
	// preparePublicBooking opens the week 09:00-23:00. This venue trades past
	// midnight, so its schedules and rate bands are replaced, not appended to.
	f.complexes.schedules = nil
	f.courts.prices = nil
	openAllWeek(f, courtID, "20:00", "02:00")
	day = timezone.Day(time.Now().In(timezone.Argentina).AddDate(0, 0, 7))

	// The night before the grid's day is priced differently, so the charge shows
	// which night the 00:30 slot was priced from.
	nightBefore := slots.DayName(day.AddDate(0, 0, -1).Weekday())
	for i, p := range f.courts.prices {
		if p.DayType == nightBefore {
			f.courts.prices[i] = courtstore.NewCourtPriceForTest(courtID, p.DayType, "20:00", "02:00", 700_000)
		}
	}
	return complexID, courtID, day
}

// The storefront lists the 00:30 slot under the day its window was sold from:
// that day's window runs past midnight, so 00:30 is minute 1470 of it. The
// booking is filed under the calendar day it actually starts on, the day after,
// and it is charged the rate of the window it was sold from.
func TestPublicBookFilesAnAfterMidnightSlotUnderTheCalendarDayItStartsOn(t *testing.T) {
	f := newFixture(t)
	complexID, courtID, day := overnightPublicBooking(f)

	body := fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":"00:30","duration_minutes":60,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000",`+
		`"client_email":"ana@example.com"}`, complexID, courtID, day.Format("2006-01-02"))

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", body))

	if w.Code != http.StatusCreated {
		t.Fatalf("the storefront lists 00:30 on this day; want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("the booking must be recorded; got %d", len(f.store.inserted))
	}
	stored := f.store.inserted[0]

	wantDate := day.AddDate(0, 0, 1).Format("2006-01-02")
	if got := stored.Date.Format("2006-01-02"); got != wantDate {
		t.Errorf("00:30 falls on the calendar day after the grid's day: want it filed under %s, got %s", wantDate, got)
	}
	if stored.StartTime != "00:30" {
		t.Errorf("the start is the clock reading the storefront showed; want 00:30, got %s", stored.StartTime)
	}

	opening := slots.DayName(day.Weekday())
	want, err := pricing.BookingPrice(f.courts.prices, opening, 1470, 60)
	if err != nil {
		t.Fatalf("the window the slot was sold from has a rate for it: %v", err)
	}
	if stored.Price != want {
		t.Errorf("the slot is charged the %s rate at minute 1470, the rate the storefront shows: want %d, got %d",
			opening, want, stored.Price)
	}
}
