package pricing

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

func rule(day, from, to string, price int) *data.CourtPrice {
	return data.NewCourtPriceForTest(uuid.Nil, day, from, to, price)
}

func TestSlotPriceReturnsTheRuleCoveringTheSlot(t *testing.T) {
	rules := []*data.CourtPrice{
		rule("monday", "08:00", "18:00", 10_000),
		rule("monday", "18:00", "23:00", 15_000),
		rule("tuesday", "08:00", "23:00", 12_000),
	}

	got, err := SlotPrice(rules, "monday", at("19:30"))
	if err != nil {
		t.Fatalf("19:30 on Monday is covered by the peak rule: %v", err)
	}
	if got != 15_000 {
		t.Errorf("want the peak price 15000; got %d", got)
	}
}

// Bounds are half-open, matching the exclusion constraint on court_prices:
// adjacent rules 08:00-18:00 and 18:00-23:00 are the normal way to price a peak
// window, and 18:00 belongs to the second.
func TestSlotPriceTreatsRuleBoundsAsHalfOpen(t *testing.T) {
	rules := []*data.CourtPrice{
		rule("monday", "08:00", "18:00", 10_000),
		rule("monday", "18:00", "23:00", 15_000),
	}

	if got, _ := SlotPrice(rules, "monday", at("08:00")); got != 10_000 {
		t.Errorf("time_from is inclusive; want 10000 at 08:00, got %d", got)
	}
	if got, _ := SlotPrice(rules, "monday", at("18:00")); got != 15_000 {
		t.Errorf("time_to is exclusive; 18:00 belongs to the peak rule, got %d", got)
	}
}

// FINDING 1. The booking path used to fall back to any band for that weekday,
// and then to prices[0] — so a slot outside every band was charged some other
// slot's price, or another weekday's, while the storefront showed it at 0.
// There is no fallback: an uncovered slot has no price.
func TestSlotPriceHasNoFallbackToAnotherBand(t *testing.T) {
	rules := []*data.CourtPrice{rule("monday", "08:00", "18:00", 10_000)}

	if _, err := SlotPrice(rules, "monday", at("20:00")); !errors.Is(err, ErrNoPriceRule) {
		t.Errorf("20:00 is outside every Monday band and must have no price; got %v", err)
	}
}

func TestSlotPriceHasNoFallbackToAnotherWeekday(t *testing.T) {
	rules := []*data.CourtPrice{rule("monday", "08:00", "23:00", 10_000)}

	if _, err := SlotPrice(rules, "sunday", at("10:00")); !errors.Is(err, ErrNoPriceRule) {
		t.Errorf("a Monday band must not price a Sunday slot; got %v", err)
	}
}

func TestSlotPriceOnACourtWithNoRulesAtAll(t *testing.T) {
	if _, err := SlotPrice(nil, "monday", at("10:00")); !errors.Is(err, ErrNoPriceRule) {
		t.Errorf("want ErrNoPriceRule; got %v", err)
	}
}

// An unpriced slot resolves to zero *and* an error. Nothing may read the value
// without reading the error, so the price the storefront shows and the price
// the write path charges cannot diverge into 0-versus-something.
func TestSlotPriceReturnsZeroWithItsError(t *testing.T) {
	got, err := SlotPrice(nil, "monday", at("10:00"))
	if err == nil {
		t.Fatal("an uncovered slot must be an error, not a free slot")
	}
	if got != 0 {
		t.Errorf("want 0 alongside the error; got %d", got)
	}
}

// A 120-minute booking starting at 17:00 crosses from an off-peak band into a
// peak one at 18:00, the same way three consecutive 60-minute slots used to be
// charged each at its own band. Two 30-minute blocks price at the 400/hour
// off-peak rate, two more at the 800/hour peak rate.
func TestBookingPriceChargesEachBlockAtItsOwnBand(t *testing.T) {
	rules := []*data.CourtPrice{
		rule("monday", "08:00", "18:00", 400_00),
		rule("monday", "18:00", "23:00", 800_00),
	}

	got, err := BookingPrice(rules, "monday", at("17:00"), 120)
	if err != nil {
		t.Fatalf("17:00-19:00 is fully priced across the two bands: %v", err)
	}
	// 30 min off-peak + 30 min off-peak + 30 min peak + 30 min peak
	// = 400_00/2 + 400_00/2 + 800_00/2 + 800_00/2 = 200_00+200_00+400_00+400_00
	want := 200_00 + 200_00 + 400_00 + 400_00
	if got != want {
		t.Errorf("BookingPrice(17:00, 120) = %d, want %d", got, want)
	}
}

// The rounding rule: rates are summed across all half-hour blocks first, and
// divided by two once at the end, rather than rounding each block on its own.
// An odd hourly rate makes the two orders disagree if either block is rounded
// independently.
func TestBookingPriceRoundsOnceOverTheWholeBooking(t *testing.T) {
	rules := []*data.CourtPrice{rule("monday", "08:00", "23:00", 100_001)}

	// Two half-hour blocks at the same odd rate: summing first gives
	// round(200_002 / 2) = 100_001 exactly, with no rounding error at all.
	got, err := BookingPrice(rules, "monday", at("09:00"), 60)
	if err != nil {
		t.Fatalf("09:00-10:00 is fully priced: %v", err)
	}
	if got != 100_001 {
		t.Errorf("BookingPrice(09:00, 60) = %d, want 100001", got)
	}
}

// A booking whose tail runs past the end of the priced window is refused
// whole, the same way SlotPrice refuses a single uncovered slot — a booking
// that runs off the end of the priced window is not offered by the
// storefront, and the write path must not sell it either.
func TestBookingPriceRefusesWhenAnyBlockIsUnpriced(t *testing.T) {
	rules := []*data.CourtPrice{rule("monday", "08:00", "19:00", 500_00)}

	if _, err := BookingPrice(rules, "monday", at("18:00"), 120); !errors.Is(err, ErrNoPriceRule) {
		t.Errorf("18:00-20:00 runs past the 19:00 band boundary and must be refused; got %v", err)
	}
}

// The rule the product chose: a booking is priced by the window it was sold
// out of. A Friday night session running to 01:00 is Friday's, all four blocks
// of it, and Friday's card is laid out across the whole window — 18:00 to
// 02:00, stored as minutes [1080, 1560) so its end is genuinely after its start
// (the span_min generated column in db/migrations/001_init.sql).
//
// The version this replaced derived the weekday per block and billed the last
// two at Saturday's rate. Correct by the calendar and wrong by the counter:
// nobody at the club calls 00:30 on a Friday night "Saturday".
func TestBookingPricePricesTheWholeNightAtTheWindowsDay(t *testing.T) {
	rules := []*data.CourtPrice{
		rule("friday", "18:00", "02:00", 400_00),
		rule("saturday", "08:00", "23:00", 800_00),
	}

	// 23:00 for two hours: minutes 1380, 1410, 1440, 1470 of Friday.
	got, err := BookingPrice(rules, "friday", at("23:00"), 120)
	if err != nil {
		t.Fatalf("the Friday band covers the whole session: %v", err)
	}
	want := 400_00 * 4 / 2
	if got != want {
		t.Errorf("BookingPrice(fri 23:00, 120) = %d, want %d — Saturday's card must not be consulted", got, want)
	}
}

// A band that stops at midnight does not cover the hours past it, and the
// booking is refused rather than billed from whatever rule happens to be near.
// This is the owner's signal that the window and the band disagree.
func TestBookingPriceRefusesWhenTheBandStopsShortOfTheWindow(t *testing.T) {
	rules := []*data.CourtPrice{
		rule("friday", "18:00", "23:59", 400_00),
	}

	if _, err := BookingPrice(rules, "friday", at("23:00"), 120); !errors.Is(err, ErrNoPriceRule) {
		t.Errorf("the band ends at 23:59 and the session runs to 01:00; want ErrNoPriceRule, got %v", err)
	}
}

// The minutes past midnight are the same band, so the rate does not change
// halfway through the night.
func TestSlotPriceCoversTheHoursPastMidnightOfAWrappingBand(t *testing.T) {
	rules := []*data.CourtPrice{rule("friday", "18:00", "02:00", 400_00)}

	for _, min := range []int{at("18:00"), at("23:30"), at("00:30") + 24*60, at("01:30") + 24*60} {
		if _, err := SlotPrice(rules, "friday", min); err != nil {
			t.Errorf("minute %d is inside [18:00, 02:00) of Friday; got %v", min, err)
		}
	}
	if _, err := SlotPrice(rules, "friday", at("02:00")+24*60); !errors.Is(err, ErrNoPriceRule) {
		t.Errorf("the band is half-open and ends at 02:00; got %v", err)
	}
}

// at turns a clock reading into minutes from its own day's midnight, which is
// what the pricing functions now take. A band running past midnight is laid out
// beyond 1440, and `at("00:30") + 1440` is how a fixture names an hour on the
// far side of it.
func at(hhmm string) int {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		panic("test fixture: " + hhmm + " is not HH:MM")
	}
	return h*60 + m
}
