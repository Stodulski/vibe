//go:build integration

package data_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A booking can legitimately carry more than one payment row — the MercadoPago checkout
// row and a cash row added at the desk — and the index on payments(booking_id) is not
// unique, so an unordered lookup returns whichever row the scan reaches first. The
// refund path needs the row carrying mp_payment_id: pick the cash row and the refund has
// nothing to send to the provider. Both insertion orders are exercised because the
// defect was precisely that the answer depended on them.
func TestGetPaymentByBookingIDPrefersTheMercadoPagoRow(t *testing.T) {
	tests := []struct {
		name      string
		cashFirst bool
	}{
		{name: "the cash row was inserted first", cashFirst: true},
		{name: "the MercadoPago row was inserted first", cashFirst: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestFixture(t)
			ctx := context.Background()

			booking := f.createBooking(t, bookingOptions{})
			mpPaymentID := "mp-" + uuid.NewString()

			var mpPayment, cashPayment *paymentstore.Payment
			if tt.cashFirst {
				cashPayment = f.createPayment(t, booking.ID, 150_000, 0, nil)
				mpPayment = f.createPayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)
			} else {
				mpPayment = f.createPayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)
				cashPayment = f.createPayment(t, booking.ID, 150_000, 0, nil)
			}

			found, err := f.Models.Payments.GetByBookingID(ctx, booking.ID)
			if err != nil {
				t.Fatalf("GetByBookingID: %v", err)
			}

			if found.ID == cashPayment.ID {
				t.Fatal("the lookup returned the cash row, which carries no MercadoPago id to refund against")
			}
			if found.ID != mpPayment.ID {
				t.Fatalf("want the MercadoPago payment %s; got %s", mpPayment.ID, found.ID)
			}
			if found.MPPaymentID == nil || *found.MPPaymentID != mpPaymentID {
				t.Errorf("want mp_payment_id %q; got %v", mpPaymentID, found.MPPaymentID)
			}
		})
	}
}

// ListByBookingID is the whole-ledger counterpart to GetByBookingID: callers
// that sum money across every payment row (the refund path) or show every
// payment on a booking (the detail endpoint) cannot use a query that silently
// picks one row and discards the rest.
func TestListPaymentsByBookingIDReturnsEveryRowOldestFirst(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	booking := f.createBooking(t, bookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()

	cashPayment := f.createPayment(t, booking.ID, 150_000, 0, nil)
	mpPayment := f.createPayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	found, err := f.Models.Payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("ListByBookingID: %v", err)
	}

	if len(found) != 2 {
		t.Fatalf("want 2 payments; got %d", len(found))
	}
	if found[0].ID != cashPayment.ID {
		t.Errorf("want the cash payment %s first (created first); got %s", cashPayment.ID, found[0].ID)
	}
	if found[1].ID != mpPayment.ID {
		t.Errorf("want the MercadoPago payment %s second (created second); got %s", mpPayment.ID, found[1].ID)
	}
}

// GetPaymentByMPID has no ORDER BY and no LIMIT, and it is a sqlc `:one` query —
// which means pgx takes the first row the scan reaches and silently discards the
// rest. That is only safe because the result set can never hold more than one
// row, and this is the test that says so out loud: the partial
// unique index on payments(mp_payment_id) WHERE mp_payment_id IS NOT NULL is
// what makes the lookup deterministic.
//
// It matters because that lookup is the webhook's idempotency gate. If two rows
// could ever carry the same MercadoPago payment id, a redelivery would be
// answered by whichever row Postgres happened to reach first, and the refund
// path would compute what to send back from a row that may not be the one the
// money is on.
//
// The guarantee lives in the schema rather than in the query, so it is the
// schema that has to be pinned. Drop that index and the query becomes
// non-deterministic without a single line of Go changing.
func TestOneMercadoPagoPaymentIDCannotBeOnTwoPaymentRows(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	booking := f.createBooking(t, bookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()

	recorded := f.createPayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	// A booking may legitimately carry more than one payment row, so the index on
	// payments(booking_id) does not stop this. The second row is what a redelivery
	// that re-inserted instead of reusing the first would produce, and only the
	// unique index on mp_payment_id refuses it.
	duplicate := &paymentstore.Payment{
		BookingID:   booking.ID,
		ComplexID:   f.ComplexID,
		Amount:      150_000,
		ServiceFee:  7_500,
		Method:      "mercadopago",
		Status:      "deposit_paid",
		MPPaymentID: &mpPaymentID,
	}
	if err := f.Models.Payments.Insert(ctx, duplicate); err == nil {
		t.Fatal("the database accepted a second payment row carrying the same mp_payment_id; GetPaymentByMPID can no longer be deterministic")
	}

	// And the lookup still answers with the one row that exists.
	found, err := f.Models.Payments.GetByMPPaymentID(ctx, mpPaymentID)
	if err != nil {
		t.Fatalf("GetByMPPaymentID: %v", err)
	}
	if found.ID != recorded.ID {
		t.Errorf("want the recorded payment %s; got %s", recorded.ID, found.ID)
	}
}
