package courts

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// gridFor runs the availability handler over one court, for the default
// 90-minute duration, and returns that court's slots as they were serialised
// to the client.
func gridFor(t *testing.T, store *stubStore, bookings *stubBookings, complexes *stubComplexes, date string) []map[string]any {
	t.Helper()
	return gridForDuration(t, store, bookings, complexes, date, 0)
}

// gridForDuration is gridFor with an explicit `duration` query param. A zero
// duration omits the query param entirely, exercising the default.
func gridForDuration(t *testing.T, store *stubStore, bookings *stubBookings, complexes *stubComplexes, date string, duration int) []map[string]any {
	t.Helper()

	h, _ := newTestHandler(store, bookings, complexes)

	target := "/?date=" + date
	if duration != 0 {
		target += "&duration=" + strconv.Itoa(duration)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	grid, _ := decode(t, w)["availability"].(map[string]any)
	courtsOut, _ := grid["courts"].([]any)
	if len(courtsOut) != 1 {
		t.Fatalf("want one court in the grid; got %d (%s)", len(courtsOut), w.Body.String())
	}
	first, _ := courtsOut[0].(map[string]any)
	raw, _ := first["slots"].([]any)

	out := make([]map[string]any, 0, len(raw))
	for _, s := range raw {
		m, _ := s.(map[string]any)
		out = append(out, m)
	}
	return out
}

// activeComplex is the fixture every public read now needs: a venue that has
// not switched itself off.
func activeComplex(id uuid.UUID) *complexstore.Complex {
	return &complexstore.Complex{ID: id, Name: "Vibe", Slug: "vibe", IsActive: true}
}

// FINDING 1. The grid used to price a slot no band covered at 0 and publish it
// as bookable, while the booking path charged that slot a real amount from a
// fallback rule. An unpriced slot is not for sale, so it is not on the grid.
func TestAvailabilityOmitsSlotsNoPriceRuleCovers(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()

	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		// Open 08:00-22:00, but priced only until 12:00.
		prices: everyDayBand(courtID, "08:00", "12:00", 500_000),
	}

	got := gridForDuration(t, store, &stubBookings{}, complexes, futureDate(), 60)

	// Starts land every 30 minutes now, not every 60: 08:00, 08:30, ... 11:00 —
	// seven starts whose full 60 minutes fit inside the 08:00-12:00 band, the
	// last of which (11:30) would run its second half-hour block past 12:00
	// and is excluded.
	if len(got) != 7 {
		t.Fatalf("only 08:00-12:00 is priced for a 60-minute booking, on a 30-minute grid step; want 7 slots, got %d: %v", len(got), got)
	}
	for _, s := range got {
		if s["price"] == float64(0) {
			t.Errorf("a slot published at price 0 is a slot the write path would charge for; got %v", s)
		}
	}
	if got[len(got)-1]["start_time"] != "11:00" {
		t.Errorf("the last priced slot starts at 11:00; got %v", got[len(got)-1]["start_time"])
	}
}

// A court with no band at all for that weekday sells nothing that day, rather
// than selling the whole day at another weekday's price.
func TestAvailabilityPublishesNoSlotsForAnUnpricedDay(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()

	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: []*courtstore.CourtPrice{{CourtID: courtID, DayType: "monday", TimeFrom: "08:00", TimeTo: "22:00", Price: 500_000}},
	}

	// A Tuesday, so the only configured band is the wrong weekday.
	got := gridFor(t, store, &stubBookings{}, complexes, nextWeekday(t, "tuesday"))

	if len(got) != 0 {
		t.Errorf("no Tuesday band is configured, so nothing may be offered; got %d slots: %v", len(got), got)
	}
}

// The price the storefront shows is the price the write path resolves, because
// both call pricing.SlotPrice. This is the property, not the arithmetic.
func TestAvailabilityShowsThePriceTheBookingPathWouldCharge(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()

	rules := append(
		everyDayBand(courtID, "08:00", "18:00", 400_000),
		everyDayBand(courtID, "18:00", "22:00", 900_000)...,
	)
	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: rules,
	}

	date := futureDate()
	const duration = 120 // long enough to cross the 18:00 band boundary from some starts
	got := gridForDuration(t, store, &stubBookings{}, complexes, date, duration)
	day := slots.DayName(mustParseDate(t, date).Weekday())

	if len(got) == 0 {
		t.Fatal("the fixture must publish at least one slot, or this test asserts nothing")
	}
	for _, s := range got {
		start, _ := s["start_time"].(string)
		// The window's own day and the slot's minute inside it — for a venue
		// closing before midnight those are just the weekday and the clock.
		want, err := pricing.BookingPrice(rules, day, slots.ToMinutes(start), duration)
		if err != nil {
			t.Fatalf("the grid published %s, which has no price rule: %v", start, err)
		}
		if s["price"] != float64(want) {
			t.Errorf("slot %s: grid shows %v, the booking path resolves %d", start, s["price"], want)
		}
	}
}

// FINDING 2, and what became of it. A venue open past midnight had its closing
// time carried past 1440 and slots generated all the way to it, then rendered
// through a helper that wraps modulo 1440. Those slots showed a price of 0,
// were marked available, and the booking path refused every one of them.
//
// The grid was capped at midnight to stop publishing what could not be sold,
// which cost a 20:00-02:00 venue its last four trading hours. Both halves are
// settled now: a booking carries a range (the generated span) and the check that
// forbade the shape is gone, so the grid runs to closing and everything
// it publishes is priced and sellable.
func TestAvailabilityPublishesTheHoursPastMidnightTheVenueTrades(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()

	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: everyDaySchedule("20:00", "02:00")}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		// The band covers the window, wrapping the same way it does. A
		// 00:00-23:59 band would not: its minutes stop at 1439 and the hours
		// after the rollover are 1440 and up, so every slot past midnight would
		// be unpriced and therefore unpublished — which is the rule working,
		// not a gap.
		prices: everyDayBand(courtID, "20:00", "02:00", 500_000),
	}

	got := gridForDuration(t, store, &stubBookings{}, complexes, futureDate(), 90)

	// 20:00 through 00:30 on the 30-minute step: ten starts, the last ending
	// exactly at closing.
	if len(got) != 10 {
		t.Fatalf("20:00-02:00 sells ten 90-minute starts on the 30-minute grid; got %d: %v", len(got), got)
	}
	if got[0]["start_time"] != "20:00" {
		t.Errorf("the first slot starts at 20:00; got %v", got[0]["start_time"])
	}
	last := got[len(got)-1]
	if last["start_time"] != "00:30" || last["end_time"] != "02:00" {
		t.Errorf("the last slot is 00:30-02:00; got %v-%v", last["start_time"], last["end_time"])
	}

	// Every published slot is priced and on sale — the condition the cap was
	// protecting, now met without withholding anything.
	for _, sl := range got {
		if sl["price"] == float64(0) {
			t.Errorf("a slot published at price 0 is one the write path would charge for; got %v", sl)
		}
		if sl["available"] != true {
			t.Errorf("nothing is booked, so every slot must be available; got %v", sl)
		}
	}
}

// The ends past midnight read as the next day's clock rather than as 24:00 or
// more. `end_time` was always a clock reading; what changed is that a smaller
// one than `start_time` is no longer a contradiction.
func TestAvailabilityRendersPastMidnightEndsAsTheNextDaysClock(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()

	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: everyDaySchedule("22:00", "02:00")}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: everyDayBand(courtID, "22:00", "02:00", 500_000),
	}

	got := gridForDuration(t, store, &stubBookings{}, complexes, futureDate(), 60)

	ends := map[string]string{}
	for _, sl := range got {
		start, _ := sl["start_time"].(string)
		end, _ := sl["end_time"].(string)
		ends[start] = end
	}
	for start, want := range map[string]string{"23:00": "00:00", "23:30": "00:30", "01:00": "02:00"} {
		if ends[start] != want {
			t.Errorf("%s should end at %s; got %q", start, want, ends[start])
		}
	}
}

// FINDING 4. A deactivated venue is closed. Only the booking write used to
// check is_active, so the grid stayed live and every booking attempt against it
// answered a bare 404.
func TestAvailabilityIsClosedForADeactivatedComplex(t *testing.T) {
	complexID := uuid.New()
	complexes := &stubComplexes{
		complex:   &complexstore.Complex{ID: complexID, Slug: "vibe", IsActive: false},
		schedules: openEveryDay(),
	}
	store := &stubStore{courts: []*courtstore.Court{
		{ID: uuid.New(), ComplexID: complexID, Name: "Court 1", IsActive: true},
	}}

	h, _ := newTestHandler(store, &stubBookings{}, complexes)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+futureDate(), nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("a deactivated venue must not publish a booking grid; got %d (%s)", w.Code, w.Body.String())
	}
}

// A booking marks its hours taken. The obstacle and the grid slot are compared
// as instants, so this holds whether or not either of them spans midnight.
func TestAvailabilityMarksBookedSlotsUnavailable(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	date := futureDate()
	day := mustParseDate(t, date)

	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: pricedEveryDay(courtID),
	}
	bookings := &stubBookings{booked: []data.BookedSpan{
		{CourtID: courtID, StartsAt: slots.At(day, "10:00"), EndsAt: slots.At(day, "11:00")},
	}}

	got := gridFor(t, store, bookings, complexes, date)

	for _, s := range got {
		if s["start_time"] == "10:00" && s["available"] != false {
			t.Errorf("10:00 is booked and must not be offered; got %v", s)
		}
		if s["start_time"] == "11:00" && s["available"] != true {
			t.Errorf("11:00 is free and must stay bookable; got %v", s)
		}
	}
}

// A `duration` outside slots.PermittedDurations() is refused as input rather
// than silently coerced to the default — the same treatment the write paths
// give an out-of-set duration_minutes.
func TestAvailabilityRejectsAnUnpermittedDuration(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: pricedEveryDay(courtID),
	}

	h, _ := newTestHandler(store, &stubBookings{}, complexes)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+futureDate()+"&duration=45", nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for a duration that is not 60, 90 or 120; got %d (%s)", w.Code, w.Body.String())
	}
}

// FINDING 1. A day that has already ended has nothing left to book, so the
// grid refuses it the same way POST /api/v1/book refuses a past date — as a
// validation error on `date`, not a bookable-looking 200.
func TestAvailabilityRejectsADateBeforeToday(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: pricedEveryDay(courtID),
	}

	h, _ := newTestHandler(store, &stubBookings{}, complexes)

	yesterday := timezone.Today().AddDate(0, 0, -1).Format("2006-01-02")
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+yesterday, nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for a date before today; got %d (%s)", w.Code, w.Body.String())
	}

	body := decode(t, w)
	errs, _ := body["error"].(map[string]any)
	if _, ok := errs["date"]; !ok {
		t.Errorf("want a validation error on the `date` field; got %v", body)
	}
}

// Today itself is still bookable — the past-date rule only excludes days that
// have already fully elapsed, not the one in progress.
func TestAvailabilityAcceptsToday(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: pricedEveryDay(courtID),
	}

	h, _ := newTestHandler(store, &stubBookings{}, complexes)

	today := timezone.Today().Format("2006-01-02")
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+today, nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code == http.StatusUnprocessableEntity {
		t.Errorf("today must not be rejected as a past date; got 422 (%s)", w.Body.String())
	}
}

// A duration long enough to run past closing time is refused for that start
// rather than published: 21:30 plus 120 minutes runs to 23:30, past the
// fixture's 22:00 close.
func TestAvailabilityOmitsADurationThatRunsPastClosing(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: openEveryDay()}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: pricedEveryDay(courtID),
	}

	got := gridForDuration(t, store, &stubBookings{}, complexes, futureDate(), 120)

	for _, s := range got {
		if s["start_time"] == "21:30" {
			t.Errorf("a 120-minute booking at 21:30 would end at 23:30, past the 22:00 close, and must not be offered; got %v", s)
		}
	}
	if len(got) == 0 {
		t.Fatal("the fixture must still offer earlier 120-minute starts, or this test proves nothing")
	}
	last, _ := got[len(got)-1]["start_time"].(string)
	if last != "20:00" {
		t.Errorf("the last 120-minute start that still fits before 22:00 is 20:00; got %v", last)
	}
}

// The reason the obstacle is a pair of instants rather than a pair of clock
// readings. Last night's booking ran into this morning, and this morning's grid
// has to know.
//
// Under the string comparison this replaced, "00:00" and "00:30" were both
// below the booking's "23:00" start and the test `start < end` never fired, so
// every slot the booking was still occupying was published as free — the same
// shape that sold one court to two clients before the span existed (see db/migrations/001_init.sql).
func TestAvailabilityMarksSlotsTakenByLastNightsBooking(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	date := futureDate()
	day := mustParseDate(t, date)

	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: everyDaySchedule("00:00", "22:00")}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: everyDayBand(courtID, "00:00", "22:00", 500_000),
	}
	// 23:00 yesterday through 01:00 today.
	bookings := &stubBookings{booked: []data.BookedSpan{{
		CourtID:  courtID,
		StartsAt: slots.At(day.AddDate(0, 0, -1), "23:00"),
		EndsAt:   slots.At(day, "01:00"),
	}}}

	got := gridForDuration(t, store, bookings, complexes, date, 60)

	if len(got) == 0 {
		t.Fatal("the fixture must publish slots, or this test asserts nothing")
	}
	taken := map[string]bool{"00:00": true, "00:30": true}
	free := map[string]bool{"01:00": true, "01:30": true}
	seen := 0
	for _, s := range got {
		start, _ := s["start_time"].(string)
		switch {
		case taken[start]:
			seen++
			if s["available"] != false {
				t.Errorf("%s is still inside last night's booking and must not be offered; got %v", start, s)
			}
		case free[start]:
			seen++
			if s["available"] != true {
				t.Errorf("%s is after the booking ends and must stay bookable; got %v", start, s)
			}
		}
	}
	if seen != len(taken)+len(free) {
		t.Fatalf("the grid did not publish the four slots this test is about; got %d of them", seen)
	}
}
