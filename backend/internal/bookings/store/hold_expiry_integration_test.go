//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// withCheckout gives b the checkout row its MercadoPago link was created with.
func withCheckout(t *testing.T, f *datatest.Fixture, b *bookingstore.Booking) {
	t.Helper()
	preferenceID := "pref-" + b.ID.String()
	if err := f.Stores.Payments.Insert(f.Scoped(context.Background()), &paymentstore.Payment{
		BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount, ServiceFee: 10_500,
		Method: "mercadopago", Status: "unpaid", MPPreferenceID: &preferenceID,
	}); err != nil {
		t.Fatalf("creating the checkout row: %v", err)
	}
}

func reconcileIDs(t *testing.T, f *datatest.Fixture) map[uuid.UUID]bool {
	t.Helper()
	list, err := f.Stores.Bookings.ExpiredHoldsToReconcile(f.Scoped(context.Background()), 48*time.Hour, 50, 3)
	if err != nil {
		t.Fatalf("ExpiredHoldsToReconcile: %v", err)
	}
	ids := make(map[uuid.UUID]bool, len(list))
	for _, b := range list {
		ids[b.ID] = true
	}
	return ids
}

// S1: the inline release a new booking runs over a stale checkout used to
// leave nothing behind but the note, so nobody ever asked MercadoPago about
// it. It now records the expiry in the same statement, and the sweep's read
// returns it.
func TestTheInlineReleaseRecordsTheExpiryForTheSweep(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	stale := newStaleBooking(t, f)
	withCheckout(t, f, stale)

	if err := f.Stores.Bookings.InsertSafe(ctx, f.NewBooking(overlappingBookingOptions())); err != nil {
		t.Fatalf("the overlapping booking must be accepted: %v", err)
	}

	got, err := f.Stores.Bookings.GetByID(ctx, stale.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "cancelled" || !got.CancelledByHoldExpiry {
		t.Errorf("want the stale booking cancelled by hold expiry; got status=%q expired=%v", got.Status, got.CancelledByHoldExpiry)
	}
	if !reconcileIDs(t, f)[stale.ID] {
		t.Error("want the released booking handed to the sweep")
	}
}

// An untouched slot hold has no checkout to ask about: the inline release
// cancels it without a record.
func TestTheInlineReleaseRecordsNothingForAnUntouchedHold(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	hold := newHold(f, "09:00", "11:00")
	takeHold(t, f, hold)
	f.BackdateBookingCreatedAt(t, hold.ID, staleAge)

	if err := f.Stores.Bookings.InsertSafe(ctx, f.NewBooking(overlappingBookingOptions())); err != nil {
		t.Fatalf("the overlapping booking must be accepted: %v", err)
	}
	got, err := f.Stores.Bookings.GetByID(ctx, hold.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "cancelled" || got.CancelledByHoldExpiry {
		t.Errorf("want the hold cancelled with no expiry record; got status=%q expired=%v", got.Status, got.CancelledByHoldExpiry)
	}
}

// expiredBooking is a storefront booking the release sweep cancelled
// (ExpireHold), with a checkout row when checkout is true.
func expiredBooking(t *testing.T, f *datatest.Fixture, start, end string, checkout bool) *bookingstore.Booking {
	t.Helper()
	b := f.CreateBooking(t, datatest.BookingOptions{
		Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
		StartTime: start, EndTime: end,
	})
	if checkout {
		withCheckout(t, f, b)
	}
	b.Status = "cancelled"
	if err := f.Stores.Bookings.ExpireHold(f.Scoped(context.Background()), b); err != nil {
		t.Fatalf("ExpireHold: %v", err)
	}
	return b
}

// The sweep's read returns each expired booking with a checkout until it is
// reconciled or has failed too often, and nothing without a checkout.
func TestExpiredHoldsToReconcileReturnsOnlyWhatIsStillToAsk(t *testing.T) {
	f := datatest.Isolated(t)
	reconciled := expiredBooking(t, f, "08:00", "09:00", true)
	failing := expiredBooking(t, f, "10:00", "11:00", true)
	waiting := expiredBooking(t, f, "12:00", "13:00", true)
	noCheckout := expiredBooking(t, f, "14:00", "15:00", false)
	ctx := f.Scoped(context.Background())

	if status, _, _ := f.ReadBookingState(t, waiting.ID); status != "cancelled" {
		t.Fatalf("ExpireHold must cancel the booking; got %s", status)
	}
	if err := f.Stores.Bookings.MarkHoldReconciled(ctx, reconciled.ID); err != nil {
		t.Fatalf("MarkHoldReconciled: %v", err)
	}
	for want := 1; want <= 3; want++ {
		got, err := f.Stores.Bookings.RecordHoldReconcileFailure(ctx, failing.ID)
		if err != nil || got != want {
			t.Fatalf("RecordHoldReconcileFailure: want %d; got %d, %v", want, got, err)
		}
	}

	ids := reconcileIDs(t, f)
	if !ids[waiting.ID] {
		t.Error("want the unreconciled booking with a checkout returned")
	}
	if ids[reconciled.ID] || ids[failing.ID] || ids[noCheckout.ID] {
		t.Errorf("want reconciled, exhausted and checkout-less bookings left out; got %v", ids)
	}

	if _, err := f.DB.Exec(ctx,
		`UPDATE booking_hold_expiries SET expired_at = NOW() - INTERVAL '49 hours' WHERE booking_id = $1`, waiting.ID); err != nil {
		t.Fatalf("ageing the record: %v", err)
	}
	if reconcileIDs(t, f)[waiting.ID] {
		t.Error("want a booking past the lookback left out")
	}
}
