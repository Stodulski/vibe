package bookings

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// horizonFixture opens a venue that trades 20:00-02:00 every day, so the
// after-midnight starts these tests need are on the grid.
func horizonFixture(f *fixture) (complexID, courtID uuid.UUID) {
	complexID, courtID = preparePublicBooking(f)
	f.complexes.schedules = nil
	f.courts.prices = nil
	openAllWeek(f, courtID, "20:00", "02:00")
	return complexID, courtID
}

// horizonBody is a public booking for a start on a storefront grid day.
func horizonBody(complexID, courtID uuid.UUID, gridDay time.Time, start string) string {
	return fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":%q,"duration_minutes":60,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000",`+
		`"client_email":"ana@example.com"}`, complexID, courtID, gridDay.Format("2006-01-02"), start)
}

// argentinaDay is the calendar day daysAhead days from today in the venue's
// timezone, as PublicBook reads "today".
func argentinaDay(daysAhead int) time.Time {
	return timezone.Day(time.Now().In(timezone.Argentina).AddDate(0, 0, daysAhead))
}

// The last bookable grid day is the horizon day, and its 00:30 slot starts on
// the calendar day after it: one day past the horizon. The horizon is a rule
// about the day a booking is stored under, so this slot must be refused.
func TestPublicBookRefusesAnAfterMidnightSlotOfTheLastBookableDay(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := horizonFixture(f)
	lastGridDay := argentinaDay(bookingstore.MaxBookingHorizonDays)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", horizonBody(complexID, courtID, lastGridDay, "00:30")))

	want := fmt.Sprintf("cannot book more than %d days in advance", bookingstore.MaxBookingHorizonDays)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), want) {
		t.Errorf("00:30 on the last grid day starts one day past the horizon: want 409 %q; got %d (%s)",
			want, w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking past the horizon must not be persisted")
	}
}

// The bound sits on the calendar day, not a day earlier: the horizon day's
// before-midnight slots are still inside it.
func TestPublicBookAcceptsTheLastBookableDayBeforeMidnight(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := horizonFixture(f)
	lastGridDay := argentinaDay(bookingstore.MaxBookingHorizonDays)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", horizonBody(complexID, courtID, lastGridDay, "20:30")))

	if w.Code != http.StatusCreated {
		t.Fatalf("20:30 on the horizon day is inside the horizon: want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("the booking must be recorded; got %d", len(f.store.inserted))
	}
	if got, want := f.store.inserted[0].Date.Format("2006-01-02"), lastGridDay.Format("2006-01-02"); got != want {
		t.Errorf("20:30 stays on the horizon day: want %s, got %s", want, got)
	}
}

// Yesterday's grid carries after-midnight slots that start today. Its 00:30
// starts today at 00:30, which a client at 00:10 can still book, so the early
// date guard must not refuse the grid day. Whether that start has passed is
// the instant check's call. The only refusal this test allows is that one, so
// the calendar-day refusal is what fails it.
func TestPublicBookJudgesYesterdaysAfterMidnightSlotByItsStart(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := horizonFixture(f)
	yesterday := argentinaDay(-1)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", horizonBody(complexID, courtID, yesterday, "00:30")))

	if strings.Contains(w.Body.String(), "cannot book a past date") {
		t.Fatalf("yesterday's 00:30 slot starts today at 00:30, so it is refused only once that instant has passed; "+
			"got the calendar-day refusal (%d, %s)", w.Code, w.Body.String())
	}
}

// Relaxing the early guard to yesterday must not let yesterday's own calendar
// day through: a 20:30 start yesterday is over, and it is refused as a past
// date, as it always was.
func TestPublicBookStillRefusesYesterdaysOwnCalendarDay(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := horizonFixture(f)
	yesterday := argentinaDay(-1)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", horizonBody(complexID, courtID, yesterday, "20:30")))

	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "cannot book a past date") {
		t.Errorf("20:30 yesterday is over: want 409 %q; got %d (%s)", "cannot book a past date", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking for a past day must not be persisted")
	}
}
