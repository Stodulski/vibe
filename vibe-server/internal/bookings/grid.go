package bookings

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// This file is the write path's half of internal/slots.Grid.
//
// slots.Grid's own doc comment already describes the arrangement it was built
// for: "The storefront renders Slots(); the write path calls Validate on the
// value a caller asked for. Neither can offer or accept what the other would
// not." Only the first half of that was ever wired. The availability handler
// built a Grid; both booking handlers re-derived a weaker rule of their own —
// PublicBook did schedule containment with its own copy of the midnight
// arithmetic, and Create did not even do that — so the write path accepted
// start times the storefront never offered.
//
// An off-grid start is not a cosmetic problem. A 90-minute court open from
// 09:00 sells 18:00 and 19:30; a booking accepted at 18:45 runs across both,
// takes two grid positions out of circulation while occupying one row, and
// defeats the slot lock, which is keyed on an exact start time and therefore
// cannot see a hold placed at a different one.
//
// And the same two handlers never asked whether the owner had blocked the
// hours at all. blocked_slots is consulted by the storefront (courts'
// availability) and defended in one direction by the blocked-slot write, which
// refuses to block over a live booking — but nothing refused a booking over a
// live block. An owner who closed a court for maintenance could watch it sell
// anyway, and the confirmation that turns a pending booking into one holding
// its slot could not see the block either.

// errComplexClosed is the grid's answer when the venue is shut on that weekday:
// there is no grid to be on.
var errComplexClosed = errors.New("bookings: the complex is closed on the selected date")

// courtGrid returns the bookable grid the complex has on one date.
//
// It is the same value the availability handler renders from, built from the
// complex's opening hours for that weekday — the only input left now that a
// booking's length is chosen per request rather than fixed on the court — so a
// position this refuses is a position the storefront never drew.
func (h *Handler) courtGrid(ctx context.Context, complexID uuid.UUID, date time.Time) (slots.Grid, error) {
	schedules, err := h.complexes.GetSchedules(ctx, complexID)
	if err != nil {
		return slots.Grid{}, err
	}

	day := slots.DayName(date.Weekday())
	for _, s := range schedules {
		if s.Day != day {
			continue
		}
		if s.IsClosed {
			return slots.Grid{}, errComplexClosed
		}
		return slots.NewGrid(s.OpenTime, s.CloseTime), nil
	}

	// No row for the weekday is the same fact as a closed one: nothing says
	// the venue opens, so nothing may be sold.
	return slots.Grid{}, errComplexClosed
}

// slotIsBlocked reports whether the owner has taken any part of the candidate
// booking off sale on this court and date.
//
// The read is per court and date rather than per slot, so a multi-slot booking
// costs one query regardless of how many positions it spans.
//
// It takes instants rather than clock strings, which is the whole fix.
//
// The end used to arrive as a string from slots.Add, which wraps at midnight:
// a 23:00 booking of sixty minutes ends "00:00". Fed to the string comparison
// this said the booking ran from 23:00 backwards to midnight, so a block on
// 23:00–23:59 did not overlap it and the booking was sold. Nothing downstream
// caught it either — the bookings_no_overlapping_span exclusion constraint is single-table and
// does not see blocked_slots, so this check is the only thing standing between
// a blocked court and a public sale. An end derived by adding a duration to
// the start instant — or read off the booking's own span — cannot wrap,
// because it never round-trips through a clock face.
//
// A blocked slot itself can never cross midnight — blocked_slots has carried
// CHECK (start_time < end_time) since the initial schema — so placing its two times
// of day on the date it is filed under is exact.
//
// Exact, and on one timeline. blockedDays anchors every day it returns at
// midnight on the product's wall-clock, so slots.At — which carries the
// location through from the date it is given — builds the block's two instants
// in Argentina. That is not decoration: the confirmation path reaches here with
// booking.Date, and a `date` column is decoded by pgx as midnight UTC, so the
// days used to arrive on a different timeline from the candidate's own
// instants. A block filed 00:00-01:00 became the instants 21:00-22:00 the
// previous evening, three hours out, and a booking landing on it was answered
// "not blocked". Nothing downstream caught it — InsertAndConfirmBooking has no
// blocked-slot check at all — so an owner could block a court for maintenance
// and still have a pending booking on those hours confirmed onto it.
//
// The two write paths never saw it, because they build their candidate with
// slots.At from the same UTC-anchored date and were therefore wrong on both
// sides at once. They now anchor their date too, so all three callers speak
// the same wall-clock.
//
// The candidate can, though, and that is why the read is per day rather than
// for `date` alone. A booking filed on D running 23:00 for two hours meets a
// block filed on D+1 at 00:00; loading only D's blocks, this answered "not
// blocked" and left the refusal to InsertSafe's transactional guard, which
// says only "slot unavailable". The outcome was right and the sentence was
// wrong: the client was told the hours were taken rather than that the court
// is closed, and an owner reading it could not tell their own maintenance
// block from a sale. The second read costs one query, and only for the
// bookings that actually cross midnight.
func (h *Handler) slotIsBlocked(
	ctx context.Context,
	courtID uuid.UUID,
	date time.Time,
	startAt, endAt time.Time,
) (bool, error) {
	for _, day := range blockedDays(date, endAt) {
		blocked, err := h.courts.GetBlockedSlots(ctx, courtID, day)
		if err != nil {
			return false, err
		}

		for _, b := range blocked {
			if slots.OverlapAt(startAt, endAt, slots.At(day, b.StartTime), slots.At(day, b.EndTime)) {
				return true, nil
			}
		}
	}
	return false, nil
}

// blockedDays lists the dates whose blocks can touch a candidate that starts on
// date and ends at endAt.
//
// The end is read on the product's wall-clock, because that is the calendar
// blocked_slots.date is filed on, and endAt reaches here as an instant — from
// slots.At on the two write paths, and off a booking's span on the
// confirmation path, where it carries whatever zone pgx decoded it in.
//
// Every day returned is anchored at midnight in Argentina by timezone.Day, and
// that is what puts the block instants slots.At derives from them on the same
// timeline as endAt. The date arriving here is a calendar day however it was
// obtained — parsed from the request, or decoded by pgx as midnight UTC — so
// only its year, month and day are read, which is all a `date` column ever
// meant.
//
// A booking ending exactly at midnight is not on the next day: both ranges
// being compared are half-open, so hours that end where a day ends belong to
// the day they started in.
func blockedDays(date time.Time, endAt time.Time) []time.Time {
	first := timezone.Day(date)
	days := []time.Time{first}

	last := endAt.Add(-time.Nanosecond).In(timezone.Argentina)
	y, m, d := first.Date()
	if ly, lm, ld := last.Date(); ly != y || lm != m || ld != d {
		days = append(days, first.AddDate(0, 0, 1))
	}
	return days
}

// Messages for the three refusals a grid can issue, kept beside each other so
// that the wording a client reads is chosen once rather than at each handler.
//
// The midnight text is load-bearing: it is what distinguishes "we cannot sell
// hours that cross midnight" from the schedule refusal that would otherwise
// swallow it, and midnight_test.go asserts on the word.
const (
	outsideScheduleMessage = "the selected time is outside the complex schedule"
	offGridMessage         = "the selected time is not one of the court's start times"
	closedMessage          = "the complex is closed on the selected date"
	blockedSlotMessage     = "the selected time is not available on this court"
)

// offBoundaryMessage is what the owner/staff write path answers with when a
// start time does not fall on the package's 30-minute grid step. Unlike
// offGridMessage it says nothing about the court's own start times — the
// owner path does not consult those — only that the position itself cannot be
// represented or reconciled against the storefront's grid.
const offBoundaryMessage = "must fall on a 30-minute boundary (e.g. 10:00 or 10:30)"

// durationMessage is the refusal both write paths answer with when a booking
// asks for a length that is not one of the grid's permitted durations. It is
// derived from slots.PermittedDurations rather than typed out, the same reason
// internal/courts' own durationMessage exists — so the set the grid accepts
// and the sentence a client reads cannot drift apart.
var durationMessage = func() string {
	d := slots.PermittedDurations()
	parts := make([]string, len(d))
	for i, m := range d {
		parts[i] = strconv.Itoa(m)
	}
	return "must be " + strings.Join(parts[:len(parts)-1], ", ") + ", or " + parts[len(parts)-1]
}()

// gridRefusal turns a slots.Grid error into the sentence a client is given.
//
// An unrecognised error is reported as the schedule refusal rather than being
// dropped: Validate has exactly two failure modes today, and a third added
// later must still refuse the booking, not let it through. It had three until
// A booking may cross midnight now, which retired that verdict
// along with the sentinel that carried it.
func gridRefusal(err error) string {
	switch {
	case errors.Is(err, slots.ErrOffGrid):
		return offGridMessage
	case errors.Is(err, errComplexClosed):
		return closedMessage
	default:
		return outsideScheduleMessage
	}
}
