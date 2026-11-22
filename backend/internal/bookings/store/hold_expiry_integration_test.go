//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
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

// The sweep's read returns each expired booking until it is reconciled or has
// failed too often, a checkout row or not: MercadoPago is searched by the
// booking id alone, and a booking whose payment row failed to save after its
// link was created could still have been paid.
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
	// Due again, so only the failure bound can leave it out.
	makeDue(t, f, failing.ID)

	ids := reconcileIDs(t, f)
	if !ids[waiting.ID] {
		t.Error("want the unreconciled booking with a checkout returned")
	}
	if !ids[noCheckout.ID] {
		t.Error("want the unreconciled booking without a checkout row returned: MercadoPago is asked by its id")
	}
	if ids[reconciled.ID] || ids[failing.ID] {
		t.Errorf("want reconciled and exhausted bookings left out; got %v", ids)
	}

	if _, err := f.DB.Exec(ctx,
		`UPDATE booking_hold_expiries SET expired_at = NOW() - INTERVAL '49 hours' WHERE booking_id = $1`, waiting.ID); err != nil {
		t.Fatalf("ageing the record: %v", err)
	}
	if reconcileIDs(t, f)[waiting.ID] {
		t.Error("want a booking past the lookback left out")
	}
}

// makeDue moves a record's next attempt into the past.
func makeDue(t *testing.T, f *datatest.Fixture, bookingID uuid.UUID) {
	t.Helper()
	if _, err := f.DB.Exec(f.Scoped(context.Background()),
		`UPDATE booking_hold_expiries SET next_attempt_at = NOW() - INTERVAL '1 second' WHERE booking_id = $1`, bookingID); err != nil {
		t.Fatalf("making the record due: %v", err)
	}
}

// A failed attempt is not retried on the very next run: the record waits five
// minutes more for every failure, at most two hours, so a MercadoPago outage
// of a few hours does not use up every attempt of the bookings expired in it.
func TestAFailedReconciliationBacksOff(t *testing.T) {
	f := datatest.Isolated(t)
	b := expiredBooking(t, f, "08:00", "09:00", true)
	ctx := f.Scoped(context.Background())

	if !reconcileIDs(t, f)[b.ID] {
		t.Fatal("a fresh record must be due at once")
	}
	for attempt := 1; attempt <= 30; attempt++ {
		if _, err := f.Stores.Bookings.RecordHoldReconcileFailure(ctx, b.ID); err != nil {
			t.Fatalf("RecordHoldReconcileFailure: %v", err)
		}
		var wait float64
		if err := f.DB.QueryRow(ctx,
			`SELECT EXTRACT(EPOCH FROM next_attempt_at - NOW()) FROM booking_hold_expiries WHERE booking_id = $1`, b.ID).Scan(&wait); err != nil {
			t.Fatalf("reading the next attempt: %v", err)
		}
		want := min(5*attempt, 120) * 60
		if wait < float64(want)-5 || wait > float64(want)+5 {
			t.Fatalf("after %d failures want the next attempt in %ds; got %.0fs", attempt, want, wait)
		}
		list, err := f.Stores.Bookings.ExpiredHoldsToReconcile(ctx, 48*time.Hour, 50, 100)
		if err != nil {
			t.Fatalf("ExpiredHoldsToReconcile: %v", err)
		}
		for _, got := range list {
			if got.ID == b.ID {
				t.Fatalf("after %d failures the record must wait for its next attempt", attempt)
			}
		}
	}
	makeDue(t, f, b.ID)
	list, err := f.Stores.Bookings.ExpiredHoldsToReconcile(ctx, 48*time.Hour, 50, 100)
	if err != nil || len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("a record whose next attempt came must be read again; got %v, %v", list, err)
	}
}

// The release sweep cancels from a read taken before it asked MercadoPago. A
// client who cancelled in between, or a payment that confirmed the booking in
// between, must not be overwritten into an expiry: the first would become a
// booking a late payment can reopen, the second a paid booking cancelled.
func TestExpireHoldLeavesABookingThatChangedSinceItWasRead(t *testing.T) {
	cases := map[string]struct {
		write      string
		wantStatus string
	}{
		"client cancelled": {`UPDATE bookings SET status = 'cancelled', notes = 'Cancelado por el cliente' WHERE id = $1`, "cancelled"},
		"paid meanwhile":   {`UPDATE bookings SET status = 'confirmed', collection_status = 'deposit_paid' WHERE id = $1`, "confirmed"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := datatest.Isolated(t)
			ctx := f.Scoped(context.Background())
			b := f.CreateBooking(t, datatest.BookingOptions{
				Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
			})
			if _, err := f.DB.Exec(ctx, tc.write, b.ID); err != nil {
				t.Fatalf("the concurrent write: %v", err)
			}
			b.Status = "cancelled"
			note := "Cancelado automáticamente: tiempo de pago expirado (10m0s)"
			b.Notes = &note

			if err := f.Stores.Bookings.ExpireHold(ctx, b); !errors.Is(err, data.ErrEditConflict) {
				t.Fatalf("want ExpireHold refused with ErrEditConflict for a booking no longer pending and unpaid; got %v", err)
			}
			if status, _, _ := f.ReadBookingState(t, b.ID); status != tc.wantStatus {
				t.Errorf("want the booking left %s; got %s", tc.wantStatus, status)
			}
			var n int
			if err := f.DB.QueryRow(ctx, `SELECT count(*) FROM booking_hold_expiries WHERE booking_id = $1`, b.ID).Scan(&n); err != nil {
				t.Fatalf("counting expiry rows: %v", err)
			}
			if n != 0 {
				t.Errorf("want no expiry recorded; got %d rows", n)
			}
			got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if got.Notes != nil && *got.Notes == note {
				t.Error("the expiry note overwrote the concurrent write")
			}
		})
	}
}

// S8: a hold whose payment is in process is extended, and the extension is
// what every reader honours: the hours stay taken for a new booking, and the
// booking stays pending. A booking no longer pending is not touched.
func TestExtendHoldKeepsTheHoursTaken(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	stale := newStaleBooking(t, f)
	withCheckout(t, f, stale)

	if err := f.Stores.Bookings.ExtendHold(ctx, stale.ID, stale.ComplexID, 6*time.Minute, 10*time.Minute); err != nil {
		t.Fatalf("ExtendHold: %v", err)
	}

	err := f.Stores.Bookings.InsertSafe(ctx, f.NewBooking(overlappingBookingOptions()))
	if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("want the extended hold to keep its hours taken; got %v", err)
	}
	got, err := f.Stores.Bookings.GetByID(ctx, stale.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "pending" {
		t.Errorf("want the extended booking still pending; got %q", got.Status)
	}

	got.Status = "cancelled"
	if err := f.Stores.Bookings.Update(ctx, got); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := f.Stores.Bookings.ExtendHold(ctx, stale.ID, stale.ComplexID, 6*time.Minute, 10*time.Minute); !errors.Is(err, data.ErrEditConflict) {
		t.Errorf("a cancelled booking must not be extended; got %v", err)
	}
}

// S8: ExtendHold never moves created_at into the future. With a window shorter
// than the extension asked for, the hold is kept for the whole window from now
// instead: a created_at in the future would sort, age and expire wrongly
// everywhere it is read.
func TestExtendHoldNeverMovesTheHoldIntoTheFuture(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	stale := newStaleBooking(t, f)
	withCheckout(t, f, stale)

	if err := f.Stores.Bookings.ExtendHold(ctx, stale.ID, stale.ComplexID, 6*time.Minute, 2*time.Minute); err != nil {
		t.Fatalf("ExtendHold: %v", err)
	}
	var future bool
	if err := f.DB.QueryRow(context.Background(),
		`SELECT created_at > NOW() FROM bookings WHERE id = $1`, stale.ID).Scan(&future); err != nil {
		t.Fatalf("reading created_at: %v", err)
	}
	if future {
		t.Error("created_at must never be set in the future")
	}
	if err := f.Stores.Bookings.InsertSafe(ctx, f.NewBooking(overlappingBookingOptions())); !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Errorf("the clamped extension must still keep the hours taken; got %v", err)
	}
}

// S8: the sweep looks ahead at the storefront bookings with a checkout whose
// window ends before its next run, so a payment in process is extended before
// the hours read as free. One already lapsed and still pending is read too
// (the sweep that expires it may not have run yet); one with no checkout, an
// untouched hold, a paid booking and one with time to spare are not.
func TestPaymentsLapsingSoonReadsCheckoutsAboutToLapse(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	insert := func(start, end string) *bookingstore.Booking {
		b := f.NewBooking(datatest.BookingOptions{
			StartTime: start, EndTime: end, Status: "pending",
			CollectionStatus: bookingstore.CollectionStatusUnpaid, RefundStatus: bookingstore.RefundStatusNone, Public: true,
		})
		if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
			t.Fatalf("insert %s: %v", start, err)
		}
		return b
	}
	soon := insert("08:00", "09:00")
	withCheckout(t, f, soon)
	f.BackdateBookingCreatedAt(t, soon.ID, window-time.Minute)

	lapsed := insert("10:00", "11:00")
	withCheckout(t, f, lapsed)
	f.BackdateBookingCreatedAt(t, lapsed.ID, window+time.Minute)

	spare := insert("12:00", "13:00")
	withCheckout(t, f, spare)
	f.BackdateBookingCreatedAt(t, spare.ID, time.Minute)

	noCheckout := insert("14:00", "15:00")
	f.BackdateBookingCreatedAt(t, noCheckout.ID, window-time.Minute)

	untouched := newHold(f, "16:00", "17:00")
	takeHold(t, f, untouched)
	f.BackdateBookingCreatedAt(t, untouched.ID, window-time.Minute)

	paid := insert("18:00", "19:00")
	withCheckout(t, f, paid)
	f.BackdateBookingCreatedAt(t, paid.ID, window-time.Minute)
	if _, err := f.DB.Exec(context.Background(), `UPDATE bookings SET status = 'confirmed', collection_status = 'deposit_paid' WHERE id = $1`, paid.ID); err != nil {
		t.Fatalf("paying: %v", err)
	}

	got, err := f.Stores.Bookings.PaymentsLapsingSoon(ctx, window, 2*time.Minute, time.Hour, 50)
	if err != nil {
		t.Fatalf("PaymentsLapsingSoon: %v", err)
	}
	ids := map[uuid.UUID]bool{}
	for _, b := range got {
		ids[b.ID] = true
	}
	if !ids[soon.ID] || !ids[lapsed.ID] || len(ids) != 2 {
		t.Errorf("want only the booking about to lapse and the lapsed one; got %v (soon %v, lapsed %v)", ids, soon.ID, lapsed.ID)
	}
	// Reading is all it does: nothing is expired or moved.
	b, err := f.Stores.Bookings.GetByID(ctx, soon.ID)
	if err != nil || b.Status != "pending" {
		t.Errorf("reading must not touch the booking; got %v, %v", b, err)
	}
}
