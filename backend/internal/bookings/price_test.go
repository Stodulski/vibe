package bookings

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// The price the storefront shows and the price the write path charges used to
// come from two different functions. The availability grid asks
// pricing.SlotPrice, which has no fallback: a slot no band covers is omitted,
// because it is not for sale. findPrice had a ladder underneath it — the band
// covering the slot, then any band for that weekday, then whichever price row
// came back first — so the same slot was sold, at another band's price.
//
// These tests are about that disagreement, and each one is written against a
// rule set where the two answers differ. A test where they agree proves nothing:
// the exact-match tier was always the same in both.

// bookableDate returns a weekday a week out, and the day_type string that
// prices it — the same lowercase name findPrice and the availability grid both
// derive from the date.
func bookableDate() (date time.Time, dayType string) {
	date = time.Now().In(timezone.Argentina).AddDate(0, 0, 7)
	return date, slots.DayName(date.Weekday())
}

// gapPricedCourt wires a court whose bands leave 18:00 unpriced: base until
// 18:00, peak from 20:00. Both write paths book 18:00 by default, so this is a
// rule set where SlotPrice refuses and the old ladder answered anyway — with
// basePrice, because GetCourtPrices orders by (day_type, time_from) and the
// earliest band for the day is the base band.
func gapPricedCourt(f *fixture, courtID uuid.UUID) (basePrice, peakPrice int) {
	_, dayType := bookableDate()
	basePrice, peakPrice = 500_000, 900_000
	f.courts.prices = []*courtstore.CourtPrice{
		courtstore.NewCourtPriceForTest(courtID, dayType, "08:00", "18:00", basePrice),
		courtstore.NewCourtPriceForTest(courtID, dayType, "20:00", "23:00", peakPrice),
	}
	return basePrice, peakPrice
}

// The gap the fixture leaves is a gap for pricing.SlotPrice too. If this ever
// stops holding, every assertion below is testing nothing, so it is asserted
// rather than assumed.
func TestTheGapFixtureIsActuallyUnpricedForSlotPrice(t *testing.T) {
	f := newFixture(t)
	courtID := uuid.New()
	gapPricedCourt(f, courtID)
	_, dayType := bookableDate()

	if _, err := pricing.SlotPrice(f.courts.prices, dayType, at("18:00")); err == nil {
		t.Fatal("the fixture must leave 18:00 unpriced, or these tests assert nothing")
	}
	// And the band that does cover 20:00 is the peak one, so the ladder's
	// "first band for this weekday" really is the cheaper answer.
	if got, err := pricing.SlotPrice(f.courts.prices, dayType, at("20:00")); err != nil || got != 900_000 {
		t.Fatalf("SlotPrice(20:00) = (%d, %v), want (900000, nil)", got, err)
	}
}

// A client is quoted one number by the availability grid and charged another by
// the checkout. The grid omits 18:00 entirely; the public write path used to
// sell it at the base band's price, which is what "the peak band loses to the
// base band" costs at the till.
func TestPublicBookRefusesASlotNoBandPrices(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	basePrice, _ := gapPricedCourt(f, courtID)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusConflict {
		t.Fatalf("a slot no rule prices is not for sale; want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Fatalf("no booking may be created for an unpriced slot; got %d, priced at %d",
			len(f.store.inserted), f.store.inserted[0].Price)
	}
	if f.checkout.created != 0 {
		t.Error("no checkout may be created for an unpriced slot")
	}
	if len(f.locks.acquired) != 0 {
		t.Error("no slot may be held for a request that is about to be refused")
	}
	// The number that must never have been charged, named so a regression says
	// what it did rather than only that it did something.
	if len(f.store.inserted) == 1 && f.store.inserted[0].Price == basePrice {
		t.Errorf("the base band's price (%d) was charged for a slot the storefront never offered", basePrice)
	}
}

// The same slot, the same rule set, the owner's dashboard instead of the public
// page. Both write paths share findPrice, but they no longer share what an
// uncovered slot means: the public path still refuses it outright (see
// TestPublicBookRefusesASlotNoBandPrices), while the owner's dashboard answers
// with a field-level validation error on price, naming the one thing that
// would let this exact booking through — CodePriceRequired, which the
// frontend uses to show the manual price input.
func TestStaffCreateRequiresAnExplicitPriceWhenNoBandPricesTheSlot(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := uuid.New(), uuid.New()
	basePrice, _ := gapPricedCourt(f, courtID)
	f.courts.court = &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}
	f.clients.client = &clientstore.Client{ID: uuid.New(), FirstName: "Ana"}
	// The schedule is set, and the price bands gapPricedCourt installed are
	// left alone, so 18:00 is a real position on this court's grid and the only
	// thing that can refuse this booking is the missing price band. The owner
	// path no longer consults the schedule at all, but the fixture is kept
	// realistic rather than relying on that.
	scheduleOnly(f, "09:00", "23:00")

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, staffBookBody(courtID, "18:00")))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a slot no rule prices requires an explicit price; want 422, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"price"`) {
		t.Errorf("the refusal must name the price field; got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), httpx.CodePriceRequired) {
		t.Errorf("the refusal must carry %s; got %s", httpx.CodePriceRequired, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Fatalf("no booking may be created without a price; got %d, priced at %d (base band is %d)",
			len(f.store.inserted), f.store.inserted[0].Price, basePrice)
	}
}

// With an explicit price the same booking succeeds, and the submitted price
// is used verbatim rather than the missing band's.
func TestStaffCreateAcceptsAnExplicitPriceWhenNoBandPricesTheSlot(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := uuid.New(), uuid.New()
	gapPricedCourt(f, courtID)
	f.courts.court = &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}
	f.clients.client = &clientstore.Client{ID: uuid.New(), FirstName: "Ana"}
	scheduleOnly(f, "09:00", "23:00")

	const manualPrice = 650_000
	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, staffBookBodyWithPrice(courtID, "18:00", 90, manualPrice)))

	if w.Code != http.StatusCreated {
		t.Fatalf("an explicit price must let an otherwise-unpriced booking through; want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("want the booking stored; got %d", len(f.store.inserted))
	}
	if f.store.inserted[0].Price != manualPrice {
		t.Errorf("the booking was priced %d, want the submitted %d", f.store.inserted[0].Price, manualPrice)
	}
}

// The ladder's last rung: with no band at all for this weekday it returned
// prices[0].Price, so a Tuesday booking was charged whatever Monday costs. On
// the owner's dashboard the refusal is now the price-required validation
// error rather than a flat 409, same as the gap case above.
func TestOwnerBookingRequiresAnExplicitPriceForAWeekdayWithNoBandsAtAll(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := uuid.New(), uuid.New()
	date, dayType := bookableDate()
	// Every day but the one being booked, so the only rows present belong to
	// another weekday entirely.
	f.courts.prices = nil
	for _, d := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"} {
		if d == dayType {
			continue
		}
		f.courts.prices = append(f.courts.prices,
			courtstore.NewCourtPriceForTest(courtID, d, "08:00", "23:00", 111_000))
	}
	f.courts.court = &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}
	f.clients.client = &clientstore.Client{ID: uuid.New(), FirstName: "Ana"}
	// Same reason as above: the venue is open and 18:00 is on the grid, so the
	// only thing left to refuse this booking is the weekday nothing prices.
	scheduleOnly(f, "09:00", "23:00")

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, staffBookBodyOn(courtID, date, "18:00")))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for a weekday nothing prices; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), httpx.CodePriceRequired) {
		t.Errorf("the refusal must carry %s; got %s", httpx.CodePriceRequired, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Errorf("another weekday's price (%d) was charged; got %d booking(s)",
			111_000, len(f.store.inserted))
	}
}

// A booking is refused whole when any one of its 30-minute blocks is unpriced.
// The old ladder priced the uncovered tail from the base band, so a booking
// that ran past the peak window's end was charged as if it had not.
func TestABookingIsRefusedWhenOnlyItsTailIsUnpriced(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	_, dayType := bookableDate()
	// A 120-minute booking from 18:00 walks blocks 18:00, 18:30, 19:00, 19:30.
	// The band only covers the first two.
	f.courts.prices = []*courtstore.CourtPrice{
		courtstore.NewCourtPriceForTest(courtID, dayType, "08:00", "19:00", 500_000),
	}

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 120)))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 when a block in the run is unpriced; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("no part of a booking whose tail is unpriced may be created")
	}
}

// staffBookBody builds a dashboard booking a week out at startTime.
func staffBookBody(courtID uuid.UUID, startTime string) string {
	date, _ := bookableDate()
	return staffBookBodyOn(courtID, date, startTime)
}

func staffBookBodyOn(courtID uuid.UUID, date time.Time, startTime string) string {
	return staffBookBodyOnFor(courtID, date, startTime, 90)
}

// staffBookBodyOnFor is staffBookBodyOn with an explicit duration, for tests
// about a booking length rather than a start time.
func staffBookBodyOnFor(courtID uuid.UUID, date time.Time, startTime string, durationMinutes int) string {
	return fmt.Sprintf(`{"court_id":%q,"date":%q,"start_time":%q,"duration_minutes":%d,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000"}`,
		courtID, date.Format("2006-01-02"), startTime, durationMinutes)
}

// staffBookBodyWithPrice builds a dashboard booking a week out at startTime,
// with an explicit price override in centavos — for tests about the owner
// path's manual price, in particular hours no price rule covers.
func staffBookBodyWithPrice(courtID uuid.UUID, startTime string, durationMinutes, priceCentavos int) string {
	date, _ := bookableDate()
	return fmt.Sprintf(`{"court_id":%q,"date":%q,"start_time":%q,"duration_minutes":%d,"price":%d,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000"}`,
		courtID, date.Format("2006-01-02"), startTime, durationMinutes, priceCentavos)
}

// scheduleOnly opens the complex every day between the given hours and touches
// nothing else — in particular not the price bands, which the tests that call
// it are about.
func scheduleOnly(f *fixture, opensAt, closesAt string) {
	f.complexes.schedules = nil
	for _, d := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"} {
		f.complexes.schedules = append(f.complexes.schedules, &complexstore.Schedule{Day: d, OpenTime: opensAt, CloseTime: closesAt})
	}
}

// at turns a clock reading into minutes from its own day's midnight, which is
// what pricing takes now. A window running past midnight is laid out beyond
// 1440, so an hour on the far side is `at("00:30") + 24*60`.
func at(hhmm string) int {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		panic("test fixture: " + hhmm + " is not HH:MM")
	}
	return h*60 + m
}
