//go:build integration

package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/stores"
)

// The tests here are about the one hole the advisory lock did not cover. A public
// booking that goes unpaid past the payment expiry is carved out of the overlap
// check so it stops holding its slot — and the booking that carve-out lets in is
// worth nothing if the stale booking can still be confirmed afterwards.
//
// staleAge is older than the default fifteen-minute expiry the fixture's stores
// are built with, and younger than the longer expiry the configuration test uses,
// so the same booking is stale under one and live under the other.
const staleAge = 20 * time.Minute

// The stale booking runs 09:00-11:00 and the taker 10:30-12:00: they overlap by
// half an hour on different start times, so nothing here is caught by
// idx_bookings_no_double, which only sees an identical start_time. Both are
// durations the schema permits (60, 90 or 120 minutes); the fixture derives
// duration_minutes from these hours, and span from that.
func staleBookingOptions() datatest.BookingOptions {
	return datatest.BookingOptions{
		StartTime: "09:00", EndTime: "11:00",
		Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid,
		RefundStatus: bookingstore.RefundStatusNone, Public: true,
	}
}

func overlappingBookingOptions() datatest.BookingOptions {
	return datatest.BookingOptions{StartTime: "10:30", EndTime: "12:00", Status: "confirmed"}
}

// newStaleBooking inserts a public unpaid booking through the real insert path and
// ages it past the fixture's payment expiry.
func newStaleBooking(t *testing.T, f *datatest.Fixture) *bookingstore.Booking {
	t.Helper()

	stale := f.NewBooking(staleBookingOptions())
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), stale); err != nil {
		t.Fatalf("the first booking must be accepted: %v", err)
	}
	f.BackdateBookingCreatedAt(t, stale.ID, staleAge)
	return stale
}

// The carve-out itself, unchanged in intent: once an unpaid public booking is
// older than the payment expiry it no longer blocks the court, or a visitor who
// abandoned a checkout would hold a slot until the next cron sweep.
func TestAStalePendingBookingStopsHoldingItsSlot(t *testing.T) {
	f := datatest.Isolated(t)

	newStaleBooking(t, f)

	newcomer := f.NewBooking(overlappingBookingOptions())
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), newcomer); err != nil {
		t.Errorf("a booking overlapping only a stale pending one must be accepted: %v", err)
	}
}

// The defect this whole change exists for.
//
// A stale pending booking is excluded from the overlap check, so a second client
// books over it and is confirmed. The first booking's webhook then arrives. Before
// the confirmation-time slot guard, nothing looked at the court again: the stale
// booking was confirmed too, and the court held two confirmed bookings ninety
// minutes on top of each other.
func TestConfirmingAStalePendingBookingIsRefusedWhenItsSlotWasTaken(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	stale := newStaleBooking(t, f)

	taken := f.NewBooking(overlappingBookingOptions())
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(ctx), taken); err != nil {
		t.Fatalf("the overlapping booking must be accepted while the first one is stale: %v", err)
	}

	// The refusal has to be a REFUNDABLE one, and that is the whole assertion:
	// internal/payments/process.go sends the client's money back on exactly
	// ErrSlotUnavailable and ErrBookingCancelled, and requeues on anything
	// else. This case used to expect ErrSlotUnavailable alone, which was the
	// only refundable answer there was; guardBookingConfirmable now re-reads
	// the row first and finds it cancelled — the taker's insert having
	// released it — so the refundable answer for this interleaving is
	// ErrBookingCancelled instead. H-23 is what happened when that split was
	// made without the refund branch following it: the money stopped going
	// back and nothing here said so, because this file was not being run.
	err := f.ConfirmBooking(f.Stores, stale)
	if !errors.Is(err, bookingstore.ErrBookingCancelled) && !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Errorf("confirming a stale booking whose slot was taken must be refused with an error the "+
			"webhook refunds on (ErrBookingCancelled or ErrSlotUnavailable); got %v", err)
	}

	// The taker's insert released the stale booking (ReleaseStalePendingOverlaps),
	// so it is cancelled by the time its payment arrives; what the refusal must
	// not do is turn it back into a confirmed one or record its payment.
	status, collectionStatus, _ := f.ReadBookingState(t, stale.ID)
	if status != "cancelled" || collectionStatus != bookingstore.CollectionStatusUnpaid {
		t.Errorf("a refused confirmation must leave the released booking as it was; got status=%q collection_status=%q", status, collectionStatus)
	}
	if confirmed := f.CountBookings(t, "confirmed"); confirmed != 1 {
		t.Errorf("the court must hold exactly one confirmed booking for those hours; found %d", confirmed)
	}

	// The refusal is a rollback, not a partial write: no payment row may survive it
	// either, or the money would read as recorded against a booking nothing confirmed.
	var payments int
	if err := f.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE booking_id = $1`, stale.ID).Scan(&payments); err != nil {
		t.Fatalf("counting payments: %v", err)
	}
	if payments != 0 {
		t.Errorf("a refused confirmation must write no payment row; found %d", payments)
	}
}

// The counterpart, and the reason the guard asks about the slot rather than about
// the clock: a client whose payment merely arrived late, into hours nobody else
// took, still gets their booking. Refusing them for being old would be a new
// defect wearing the fix's clothes.
func TestConfirmingAStalePendingBookingWhoseSlotIsFreeSucceeds(t *testing.T) {
	f := datatest.Isolated(t)

	stale := newStaleBooking(t, f)

	if err := f.ConfirmBooking(f.Stores, stale); err != nil {
		t.Fatalf("a late payment into a free slot must still confirm the booking: %v", err)
	}

	status, collectionStatus, _ := f.ReadBookingState(t, stale.ID)
	if status != "confirmed" || collectionStatus != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("the booking must be confirmed and paid; got status=%q collection_status=%q", status, collectionStatus)
	}
}

// The threshold is the configured payment expiry, not a literal in the SQL.
//
// It has to be, because the only thing that makes it safe to stop holding a slot
// is the cancellation cron cancelling that booking on the same schedule. When the
// two disagree, the gap between them is a window in which a booking holds no slot
// and nothing has cancelled it.
func TestTheStaleCarveOutFollowsTheConfiguredPaymentExpiry(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// Stores configured to hold a slot for a full hour, against the fixture's
	// default fifteen minutes.
	longHold := stores.NewOver(f.DB, stores.Config{PaymentExpiry: time.Hour})

	newStaleBooking(t, f)

	newcomer := f.NewBooking(overlappingBookingOptions())
	err := longHold.Bookings.InsertSafe(f.Scoped(ctx), newcomer)
	if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("under an hour-long payment expiry a %v-old booking still holds its slot; got %v", staleAge, err)
	}

	// Same booking, same age, stores configured with the fifteen-minute default:
	// now it is stale and the slot is free. Only the configured value differs.
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(ctx), newcomer); err != nil {
		t.Errorf("under the default expiry the same slot must be free: %v", err)
	}
}

// The defect this test guards against: GetBookedSlots, the query behind the
// public availability grid, used to hardcode INTERVAL '15 minutes' for this
// same carve-out while SlotTaken (InsertSafe's collision guard, exercised
// above) took the configured payment expiry as a parameter. With any expiry
// other than fifteen minutes the two disagreed — a stale booking between the
// hardcoded value and the configured one showed as free on the grid and was
// refused at booking time, or the reverse. GetBookedSlots now takes the same
// Config.PaymentExpiry every other copy of this carve-out reads.
func TestAvailabilityFollowsTheConfiguredPaymentExpiry(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	stale := newStaleBooking(t, f)

	// Under the fixture's default fifteen-minute expiry, staleAge (20m) is well
	// past the hold: the grid must show the slot free.
	freeSlots := bookedStarts(t, f, stale.Date)
	if contains(freeSlots, stale.StartTime) {
		t.Errorf("a %v-old pending booking must not block its slot under the default 15m expiry; booked=%v", staleAge, freeSlots)
	}

	// Under a 30-minute configured expiry the same 20-minute-old booking has not
	// expired yet: the grid must show the slot taken.
	longHold := stores.NewOver(f.DB, stores.Config{PaymentExpiry: 30 * time.Minute})
	takenSlots, err := longHold.Bookings.GetBookedSlotsByCourtIDs(ctx, []uuid.UUID{f.CourtID}, stale.Date)
	if err != nil {
		t.Fatalf("reading booked slots under a 30m expiry: %v", err)
	}
	var startsAt []string
	for _, s := range takenSlots {
		startsAt = append(startsAt, s.StartsAt.Format(time.RFC3339))
	}
	found := false
	for _, s := range takenSlots {
		if s.StartsAt.Equal(stale.StartsAt) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("a %v-old pending booking must still block its slot under a 30m expiry; booked=%v want %v", staleAge, startsAt, stale.StartsAt)
	}
}

// The two paths that can put a booking on a court — an insert and a payment
// confirmation — now contend for the same advisory lock, so they have to be raced
// against each other and not only against their own kind. Exactly one may win.
//
// Both orders are played out, because only one of them is dangerous. When the
// confirmation gets there first the insert is refused by a check that always
// existed; when the insert gets there first, nothing but the confirmation-time
// guard stands between the client and a court sold twice. A test that let the
// database pick the order would pass on the harmless half of the coin.
//
// The order is chosen without weakening the race: both transactions are live and
// genuinely queued on the same lock, held by the test until they are both waiting
// for it. Postgres grants it in the order it was asked for.
func TestAnInsertAndAConfirmationRacingForTheSameSlotLeaveOneWinner(t *testing.T) {
	tests := []struct {
		name string
		// insertFirst decides which of the two transactions reaches the lock first.
		insertFirst bool
	}{
		{name: "the insert gets there first", insertFirst: true},
		{name: "the confirmation gets there first", insertFirst: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := datatest.Shared(t)

			stale := newStaleBooking(t, f)
			newcomer := f.NewBooking(overlappingBookingOptions())

			insert := func() error {
				return f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), newcomer)
			}
			confirm := func() error {
				return f.ConfirmBooking(f.Stores, stale)
			}
			first, second := insert, confirm
			if !tt.insertFirst {
				first, second = confirm, insert
			}

			release := f.BlockCourtDay(t, stale.Date)

			results := make([]error, 2)
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				results[0] = first()
			}()
			// Both goroutines are inside a real transaction blocked on the lock before
			// it is released, so neither can run to completion ahead of the other.
			f.WaitForLockWaiters(t, 1)

			go func() {
				defer wg.Done()
				results[1] = second()
			}()
			f.WaitForLockWaiters(t, 2)

			release()
			wg.Wait()

			var accepted, rejected int
			for i, err := range results {
				switch {
				case err == nil:
					accepted++
				// Either refundable refusal counts; see the note in
				// TestConfirmingAStalePendingBookingIsRefusedWhenItsSlotWasTaken
				// for why the confirmation's loser can now answer with the
				// cancelled one.
				case errors.Is(err, bookingstore.ErrSlotUnavailable), errors.Is(err, bookingstore.ErrBookingCancelled):
					rejected++
				default:
					t.Errorf("attempt %d failed for an unexpected reason: %v", i, err)
				}
			}

			if accepted != 1 {
				t.Errorf("exactly one of the insert and the confirmation must be accepted; %d were", accepted)
			}
			if rejected != 1 {
				t.Errorf("the loser must be turned away with a refundable refusal; %d were", rejected)
			}
			if results[0] != nil {
				t.Errorf("the transaction that reached the lock first must be the one accepted; it got %v", results[0])
			}

			// Whoever won, the court is sold once: either the newcomer was inserted
			// confirmed and the stale booking stayed pending, or the stale booking was
			// confirmed and the newcomer never existed.
			if confirmed := f.CountBookings(t, "confirmed"); confirmed != 1 {
				t.Errorf("the court must hold exactly one confirmed booking for those hours; found %d", confirmed)
			}
		})
	}
}
