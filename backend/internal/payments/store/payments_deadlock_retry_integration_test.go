//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	slotguard "github.com/stodulski/vibe-server/internal/data/slotguard"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A payment confirmation takes the court-day advisory lock and then the court
// row FOR SHARE; creating a weekly series takes the court row FOR UPDATE and
// then, inserting its bookings, the court-day advisory lock. Opposite orders,
// so PostgreSQL can abort the confirmation with a deadlock (40P01), which used
// to reach the client as a 500 while their money was captured. The
// confirmation now retries as a whole, like the series creation does.
//
// The competing writer is that series order played by hand: court row first,
// then the court-day lock once the confirmation is waiting on the row. Its own
// deadlock_timeout is raised so the confirmation, waiting first, is the side
// PostgreSQL aborts.
func TestAConfirmationAbortedByADeadlockWithASeriesIsRetried(t *testing.T) {
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

			series, err := f.SharedPool(t, "series").Begin(ctx)
			if err != nil {
				t.Fatalf("beginning the series transaction: %v", err)
			}
			t.Cleanup(func() { _ = series.Rollback(context.Background()) })
			if _, err := series.Exec(ctx, `SET LOCAL deadlock_timeout = '10s'`); err != nil {
				t.Fatalf("raising the series side's deadlock_timeout: %v", err)
			}
			if err := eventstore.LockCourt(ctx, series, f.CourtID); err != nil {
				t.Fatalf("locking the court: %v", err)
			}

			done := make(chan error, 1)
			go func() { done <- tc.confirm(ctx, f, b) }()

			// The confirmation now holds the court-day lock and waits on the row.
			select {
			case err := <-done:
				t.Fatalf("the confirmation finished while the court row was held (err=%v)", err)
			case <-time.After(500 * time.Millisecond):
			}

			// Closing the cycle: this waits on the confirmation's court-day lock
			// until PostgreSQL aborts the confirmation and releases it.
			if err := slotguard.LockCourtDay(ctx, series, f.CourtID, b.Date); err != nil {
				t.Fatalf("the series side was chosen as the deadlock victim; the test cannot tell anything: %v", err)
			}
			if err := series.Rollback(ctx); err != nil {
				t.Fatalf("ending the series transaction: %v", err)
			}

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("a confirmation aborted by a deadlock must be retried, not answered with an error; got %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the confirmation never finished")
			}

			if status, _, _ := f.ReadBookingState(t, b.ID); status != "confirmed" {
				t.Errorf("want the retried confirmation committed; booking status %q", status)
			}
		})
	}
}
