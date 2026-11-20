//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Creating an open-play event, a fixed weekly series or a tournament match
// takes the court's row lock FOR UPDATE and then checks the hours are free; it
// takes no court-day advisory lock. The payment confirmation took only that
// advisory lock, so the two serialized on nothing: a late webhook confirming a
// stale storefront booking could commit over an event inserted at the same
// moment, each check blind to the other's uncommitted row. The confirmation
// now holds the court row too, so it waits for the event's transaction and
// then sees what it wrote.
//
// The competing writer here is the first half of eventstore.CreateEvent (the
// court lock) followed by an occupancy the confirmation guard reads, committed
// only after the confirmation has started.
func TestAConfirmationWaitsForACourtHeldByAnEventBeingCreated(t *testing.T) {
	for _, tc := range []struct {
		name    string
		confirm func(ctx context.Context, f *datatest.Fixture, b *bookingstore.Booking) error
	}{
		{
			name: "the checkout row the webhook settles",
			confirm: func(ctx context.Context, f *datatest.Fixture, b *bookingstore.Booking) error {
				mpPaymentID := "mp-" + uuid.NewString()
				checkout := &paymentstore.Payment{
					BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount,
					Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpPaymentID,
				}
				if err := f.Stores.Payments.Insert(ctx, checkout); err != nil {
					return err
				}
				b.Status = "confirmed"
				return f.Stores.Payments.ConfirmWebhookPayment(ctx, checkout, b)
			},
		},
		{
			name: "a new payment row",
			confirm: func(ctx context.Context, f *datatest.Fixture, b *bookingstore.Booking) error {
				b.Status = "confirmed"
				return f.Stores.Payments.InsertAndConfirmBooking(ctx, &paymentstore.Payment{
					BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount,
					Method: "mercadopago", Status: "deposit_paid",
				}, b)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := datatest.Shared(t)
			ctx := f.Scoped(context.Background())

			b := f.CreateBooking(t, datatest.BookingOptions{
				Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
			})
			// Past its payment window: the event's check does not count it.
			f.BackdateBookingCreatedAt(t, b.ID, time.Hour)

			event, err := f.SharedPool(t, "court lock").Begin(ctx)
			if err != nil {
				t.Fatalf("beginning the event transaction: %v", err)
			}
			t.Cleanup(func() { _ = event.Rollback(context.Background()) })
			if err := eventstore.LockCourt(ctx, event, f.CourtID); err != nil {
				t.Fatalf("locking the court: %v", err)
			}

			done := make(chan error, 1)
			go func() { done <- tc.confirm(ctx, f, b) }()

			select {
			case err := <-done:
				t.Fatalf("the confirmation finished while an event held the court (err=%v): it is serialized against nothing", err)
			case <-time.After(500 * time.Millisecond):
			}

			if _, err := event.Exec(ctx, `
				INSERT INTO blocked_slots (court_id, date, start_time, end_time, reason)
				VALUES ($1, $2::date, '17:00'::time, '21:00'::time, 'maintenance')`,
				f.CourtID, b.Date.Format("2006-01-02")); err != nil {
				t.Fatalf("writing the event's occupancy: %v", err)
			}
			if err := event.Commit(ctx); err != nil {
				t.Fatalf("committing the event: %v", err)
			}

			select {
			case err := <-done:
				if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
					t.Fatalf("the confirmation must see the hours the event took and refuse with ErrSlotUnavailable; got %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the confirmation never finished")
			}
			assertBookingStillPending(t, f, b.ID)
		})
	}
}
