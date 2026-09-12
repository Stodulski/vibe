//go:build integration

package data_test

import (
	"context"
	"errors"
	"testing"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// The stale-pending carve-out reads the collection axis, and this is the test
// for the half of it nothing covered.
//
// The predicate — status 'pending', nothing collected, created by nobody, older
// than the configured hold — is written four times: db/queries/bookings.sql's
// GetBookedSlots, and SlotTaken, SpanTaken and ReleaseStalePendingOverlaps in
// slot_guard.go. The payment_status split rewrote the money clause in every one of them
// from `payment_status = 'unpaid'` to `collection_status = 'unpaid'`.
//
// The tests beside this one exercise the AGE dimension in both directions and
// through both surfaces, so a carve-out that stopped working outright would
// fail them. What they cannot see is the money clause going wrong: drop it, or
// point it at the refund axis where every fresh booking also reads its default,
// and every one of those tests still passes — while a booking that paid a
// deposit and is waiting for its game silently stops holding its court, and the
// storefront sells those hours to somebody else. That is a real booking losing
// a real slot, and it is the failure this file exists to catch.
//
// Both surfaces are asserted because they are two separate spellings of one
// predicate and have disagreed before (see
// TestAvailabilityFollowsTheConfiguredPaymentExpiry, which exists because
// GetBookedSlots once hardcoded its own interval).

// depositPaidStaleBookingOptions is staleBookingOptions with the deposit taken.
// Everything else is identical — same hours, same public origin, same age once
// backdated — so the only thing that can change the answer is the money.
func depositPaidStaleBookingOptions() bookingOptions {
	opts := staleBookingOptions()
	opts.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	return opts
}

// TestTheCarveOutFreesAnUnpaidPendingAndNeverADepositPaidOne runs both bookings
// through the same two surfaces in one test, because the pair is the assertion:
// either alone can be satisfied by a predicate that answers the same way for
// everything.
func TestTheCarveOutFreesAnUnpaidPendingAndNeverADepositPaidOne(t *testing.T) {
	// context.Background() is spelled at each call rather than held in a
	// variable up here: contextcheck reads a context in an enclosing scope that
	// the t.Run closures do not pass down as the defect it is named for.
	t.Run("through GetBookedSlots", func(t *testing.T) {
		t.Run("an unpaid pending stops blocking once it is stale", func(t *testing.T) {
			f := newTestFixture(t)
			stale := newStaleBooking(t, f)

			if booked := bookedStarts(t, f, stale.Date); contains(booked, stale.StartTime) {
				t.Errorf("a stale unpaid pending booking must not appear as booked; booked=%v", booked)
			}
		})

		t.Run("a deposit-paid pending of the same age still blocks", func(t *testing.T) {
			f := newTestFixture(t)

			paid := f.newBooking(depositPaidStaleBookingOptions())
			if err := f.Models.Bookings.InsertSafe(context.Background(), paid); err != nil {
				t.Fatalf("inserting the deposit-paid booking: %v", err)
			}
			f.backdateBookingCreatedAt(t, paid.ID, staleAge)

			booked := bookedStarts(t, f, paid.Date)
			if !contains(booked, paid.StartTime) {
				t.Errorf("a pending booking that already took a deposit must keep its slot however old it is — "+
					"the carve-out frees abandoned checkouts, not paid ones; booked=%v want %v",
					booked, paid.StartTime)
			}
		})
	})

	t.Run("through InsertSafe", func(t *testing.T) {
		t.Run("an unpaid pending lets a newcomer overlap it", func(t *testing.T) {
			f := newTestFixture(t)
			newStaleBooking(t, f)

			newcomer := f.newBooking(overlappingBookingOptions())
			if err := f.Models.Bookings.InsertSafe(context.Background(), newcomer); err != nil {
				t.Errorf("a booking overlapping only a stale unpaid pending one must be accepted: %v", err)
			}
		})

		t.Run("a deposit-paid pending refuses the same newcomer", func(t *testing.T) {
			f := newTestFixture(t)

			paid := f.newBooking(depositPaidStaleBookingOptions())
			if err := f.Models.Bookings.InsertSafe(context.Background(), paid); err != nil {
				t.Fatalf("inserting the deposit-paid booking: %v", err)
			}
			f.backdateBookingCreatedAt(t, paid.ID, staleAge)

			newcomer := f.newBooking(overlappingBookingOptions())
			err := f.Models.Bookings.InsertSafe(context.Background(), newcomer)
			if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
				t.Errorf("selling hours a paid pending booking already holds must be refused with "+
					"ErrSlotUnavailable; got %v", err)
			}
		})
	})
}
