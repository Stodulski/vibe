package courts

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// fullDay books the whole trading day (08:00-22:00, the openEveryDay window)
// on day, so the grid for that date has no free slot on any court.
func fullDay(courtID uuid.UUID, day time.Time) bookingstore.BookedSpan {
	return bookingstore.BookedSpan{
		CourtID:  courtID,
		StartsAt: time.Date(day.Year(), day.Month(), day.Day(), 8, 0, 0, 0, timezone.Argentina),
		EndsAt:   time.Date(day.Year(), day.Month(), day.Day(), 22, 0, 0, 0, timezone.Argentina),
	}
}

// nextAvailableFixture is one open complex with one court priced all day.
func nextAvailableFixture(booked ...bookingstore.BookedSpan) (*stubStore, *stubBookings, *stubComplexes, uuid.UUID) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: everyDayBand(courtID, "08:00", "22:00", 500_000),
	}
	bookings := &stubBookings{booked: booked}
	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	return store, bookings, complexes, courtID
}

// callNextAvailable runs the next-available handler for one raw query string.
func callNextAvailable(t *testing.T, store *stubStore, bookings *stubBookings, complexes *stubComplexes, query string) *httptest.ResponseRecorder {
	t.Helper()
	h, _ := newTestHandler(store, bookings, complexes)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?"+query, nil)
	r = withSlug(r, "vibe")
	w := httptest.NewRecorder()
	h.NextAvailable(w, r)
	return w
}

// nextDate returns the date the 200 body names, and whether it named one.
func nextDate(t *testing.T, w *httptest.ResponseRecorder) (string, bool) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if _, ok := body["date"]; !ok {
		t.Fatalf("want a date key in the body; got %s", w.Body.String())
	}
	date, ok := body["date"].(string)
	return date, ok
}

func TestNextAvailableSkipsADayWithNoFreeSlot(t *testing.T) {
	from := timezone.Today().AddDate(0, 0, 2)
	store, bookings, complexes, courtID := nextAvailableFixture()
	bookings.booked = append(bookings.booked, fullDay(courtID, from.AddDate(0, 0, 1)))

	w := callNextAvailable(t, store, bookings, complexes, "from="+from.Format("2006-01-02"))

	got, ok := nextDate(t, w)
	want := from.AddDate(0, 0, 2).Format("2006-01-02")
	if !ok || got != want {
		t.Fatalf("want the day after the full one (%s); got %q (ok=%v)", want, got, ok)
	}
}

func TestNextAvailableNeverAnswersTheDayItWasAskedFrom(t *testing.T) {
	from := timezone.Today().AddDate(0, 0, 2)
	store, bookings, complexes, _ := nextAvailableFixture()

	w := callNextAvailable(t, store, bookings, complexes, "from="+from.Format("2006-01-02"))

	got, ok := nextDate(t, w)
	want := from.AddDate(0, 0, 1).Format("2006-01-02")
	if !ok || got != want {
		t.Fatalf("the grid of from is not searched; want %s, got %q (ok=%v)", want, got, ok)
	}
}

func TestNextAvailableAnswersNullWhenTheWholeWindowIsFull(t *testing.T) {
	from := timezone.Today().AddDate(0, 0, 2)
	var booked []bookingstore.BookedSpan
	store, _, complexes, courtID := nextAvailableFixture()
	for i := 1; i <= 14; i++ {
		booked = append(booked, fullDay(courtID, from.AddDate(0, 0, i)))
	}
	bookings := &stubBookings{booked: booked}

	w := callNextAvailable(t, store, bookings, complexes, "from="+from.Format("2006-01-02"))

	if got, ok := nextDate(t, w); ok {
		t.Fatalf("every day in the 14-day window is full; want null, got %q", got)
	}
}

func TestNextAvailableLooksNoFurtherThanFourteenDays(t *testing.T) {
	from := timezone.Today().AddDate(0, 0, 2)
	var booked []bookingstore.BookedSpan
	store, _, complexes, courtID := nextAvailableFixture()
	for i := 1; i <= 14; i++ {
		booked = append(booked, fullDay(courtID, from.AddDate(0, 0, i)))
	}
	bookings := &stubBookings{booked: booked}

	w := callNextAvailable(t, store, bookings, complexes, "from="+from.Format("2006-01-02"))

	if got, ok := nextDate(t, w); ok {
		t.Fatalf("day 15 after from is outside the window; want null, got %q", got)
	}
}

func TestNextAvailableNeverPassesTheBookingHorizon(t *testing.T) {
	// from sits on the last day the booking horizon allows, so the day after
	// it is beyond the horizon and must not be offered even though it is free.
	from := timezone.Today().AddDate(0, 0, bookingstore.MaxBookingHorizonDays)
	store, bookings, complexes, _ := nextAvailableFixture()

	w := callNextAvailable(t, store, bookings, complexes, "from="+from.Format("2006-01-02"))

	if got, ok := nextDate(t, w); ok {
		t.Fatalf("the day after the horizon must not be offered; got %q", got)
	}
}

func TestNextAvailableStartsFromTodayWhenFromIsInThePast(t *testing.T) {
	today := timezone.Today()
	from := today.AddDate(0, 0, -5)
	store, bookings, complexes, courtID := nextAvailableFixture()
	bookings.booked = append(bookings.booked, fullDay(courtID, today))

	w := callNextAvailable(t, store, bookings, complexes, "from="+from.Format("2006-01-02"))

	got, ok := nextDate(t, w)
	want := today.AddDate(0, 0, 1).Format("2006-01-02")
	if !ok || got != want {
		t.Fatalf("a past from searches from today; today is full, want %s, got %q (ok=%v)", want, got, ok)
	}
}

func TestNextAvailableRequiresFrom(t *testing.T) {
	store, bookings, complexes, _ := nextAvailableFixture()

	w := callNextAvailable(t, store, bookings, complexes, "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for a missing from; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestNextAvailableRejectsAMalformedFrom(t *testing.T) {
	store, bookings, complexes, _ := nextAvailableFixture()

	w := callNextAvailable(t, store, bookings, complexes, "from=18-10-2026")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for a malformed from; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestNextAvailableRejectsAnUnpermittedDuration(t *testing.T) {
	store, bookings, complexes, _ := nextAvailableFixture()

	w := callNextAvailable(t, store, bookings, complexes, "from=2030-01-01&duration=45")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for an unpermitted duration; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestNextAvailableAnswers404ForAnUnknownComplex(t *testing.T) {
	store, bookings, complexes, _ := nextAvailableFixture()
	complexes.err = data.ErrRecordNotFound

	w := callNextAvailable(t, store, bookings, complexes, "from="+timezone.Today().Format("2006-01-02"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for an unknown slug; got %d (%s)", w.Code, w.Body.String())
	}
}
