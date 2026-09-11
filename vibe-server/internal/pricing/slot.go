package pricing

import (
	"errors"
	"math"

	"github.com/stodulski/vibe-server/internal/data"
)

// What a slot costs used to be answered twice, differently.
//
// The availability grid scanned the day's price bands and left the price at 0
// when none matched, so a court whose bands do not cover its whole opening
// hours advertised free tennis. The booking handler ran a three-tier fallback —
// exact band, then any band for that weekday, then whatever price row happened
// to be first — and charged a real amount for the same slot. A day with no
// bands at all produced no slots on the storefront and a successful booking at
// another weekday's price.
//
// SlotPrice is that answer, once, and it has no fallback. A slot no rule covers
// is not priced, and a slot that is not priced is not for sale: the storefront
// omits it and the write path refuses it, which is the same sentence in two
// places instead of two different sentences.

// ErrNoPriceRule means no price rule covers the slot. The court is configured,
// the complex is open, and this particular slot has no price — which makes it
// unbookable rather than free.
var ErrNoPriceRule = errors.New("pricing: no price rule covers this slot")

// SlotPrice returns the hourly rate in effect at `min` on the given weekday, in
// centavos per 60 minutes.
//
// `min` is minutes from that weekday's own midnight and may exceed 1440: an
// hour sold out of Thursday's 08:00-01:30 window at 00:30 is Thursday minute
// 1470, and Thursday's bands are laid out across the same span. Minutes rather
// than a clock string because a clock reading cannot say which day it belongs
// to — "00:30" is the tail of one night and the small hours of the next, and
// the two are priced from different cards.
//
// court_prices.price used to be the price of the court's one fixed slot
// length; now that any court may be booked for 60, 90 or 120 minutes, it is an
// hourly rate instead, and this function's return value is priced accordingly.
// Callers pricing a whole booking use BookingPrice below, which is built on
// this same band lookup.
//
// Bands are half-open, matching the exclusion constraint on court_prices. That
// constraint is what makes the first match here the only match: at most one
// rule may cover any given minute for one court and weekday, so this loop is
// correct rather than merely repeatable.
func SlotPrice(rules []*data.CourtPrice, day string, min int) (int, error) {
	for _, p := range rules {
		if p == nil {
			continue
		}
		if p.DayType == day && p.Covers(min) {
			return p.Price, nil
		}
	}
	return 0, ErrNoPriceRule
}

// bookingBlockMinutes is the granularity a booking is priced at.
//
// A booking may now run 60, 90 or 120 minutes and cross a price band boundary
// partway through — a 120-minute booking starting at 17:00 can run into an
// 18:00 peak band the same way three consecutive 60-minute slots used to, each
// charged at its own band. 30 minutes is the finest grain slots.Grid offers
// start times on, so walking a booking in 30-minute blocks and pricing each at
// its own band's hourly rate reproduces that behaviour exactly, rather than
// flattening the whole booking to whatever band its start time happens to
// land in.
const bookingBlockMinutes = 30

// BookingPrice returns what a booking of durationMinutes beginning at start
// costs in total, in centavos.
//
// It walks the booking in bookingBlockMinutes blocks, prices each block's own
// start against the hourly rate SlotPrice reports for it, and sums the rates
// before dividing once — rather than rounding each block's own price and
// summing those roundings. A half-hour block costs rate/2, and rounding that
// on every block independently would compound: on an odd hourly rate, two
// half-hour blocks at the same rate could round to more (or less) than the
// hour they make up. Summing the whole-number rates first and dividing once
// bounds the total error to under half a centavo regardless of how many
// blocks the booking spans.
//
// The booking is priced by the opening window it was sold out of, which is what
// `day` and `startMin` together name: the weekday whose window produced the
// hour, and the position inside that window. Both come from one resolution the
// caller does once (priceWindow, internal/bookings), so a booking crossing
// midnight is one session on one rate card rather than two halves on two.
//
// This function walked instants and derived the weekday per block for a while,
// which billed a Thursday-night session half at Friday's card. Correct by the
// calendar, wrong by the counter: the hours came out of Thursday's window and
// Thursday is what the venue and the player both call them.
//
// If any block's start falls outside every price rule, the whole booking is
// refused with ErrNoPriceRule — the same failure mode SlotPrice already has,
// so a booking that runs off the end of the priced window is refused exactly
// where the storefront would have stopped offering it.
func BookingPrice(rules []*data.CourtPrice, day string, startMin, durationMinutes int) (int, error) {
	if durationMinutes <= 0 {
		return 0, ErrNoPriceRule
	}

	blocks := durationMinutes / bookingBlockMinutes

	rateSum := 0
	for i := 0; i < blocks; i++ {
		rate, err := SlotPrice(rules, day, startMin+i*bookingBlockMinutes)
		if err != nil {
			return 0, err
		}
		rateSum += rate
	}

	// rateSum is the sum of hourly rates over `blocks` half-hour blocks; the
	// total is half of that, rounded once to the nearest centavo.
	return int(math.Round(float64(rateSum) / 2)), nil
}
