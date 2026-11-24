package payments

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A booking whose deposit was paid through two MercadoPago rows refunds each of
// them (autoRefundOnePayment runs once per row). The booking-level result used to
// be whatever the last row reported, so a refund still queued behind an in-flight
// claim on the first row was hidden the moment the second row went through and
// the client was told the money was sent. The worst row's answer has to win.
func TestAMultiRowRefundReportsTheWorstRowNotTheLastOne(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()
	mpA, mpB := "mp-a-"+uuid.NewString(), "mp-b-"+uuid.NewString()

	booking := &bookingstore.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusFullyPaid,
		RefundStatus: bookingstore.RefundStatusNone, Price: 1_500_000, DepositAmount: 1_500_000,
	}
	rowA := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 100_000, Status: "deposit_paid", MPPaymentID: &mpA,
	}
	rowB := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 50_000, Status: "deposit_paid", MPPaymentID: &mpB,
	}
	f.payments.byBookingAll = []*paymentstore.Payment{rowA, rowB}
	// Row A's claim is already held by another path, so its refund is only queued.
	f.payments.claimed = []uuid.UUID{rowA.ID}
	f.payments.claimAmount = 50_000
	f.provider.refundAmount = 500.00
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	outcome := f.service.AutoRefundIfPaid(t.Context(), booking)

	if outcome.Result != paymentstore.RefundQueued {
		t.Fatalf("row A is still owed and only queued, so the booking's result must be %q, not the last row's; got %q (reason %q)",
			paymentstore.RefundQueued, outcome.Result, outcome.Reason)
	}
}

// The real claim clears the refund-intent marker inside its own transaction.
// When the first row's claim failed for a moment and stood the marker up again
// on its own, the second row's committed claim then erased it: the first row
// was left with no attempt, no marker and no alert. The marker has to be stood
// up after the last claim of the call.
func TestAFailedClaimKeepsItsMarkerWhenALaterRowIsClaimed(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()
	mpA, mpB := "mp-a-"+uuid.NewString(), "mp-b-"+uuid.NewString()

	booking := &bookingstore.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusFullyPaid,
		RefundStatus: bookingstore.RefundStatusNone, Price: 1_500_000, DepositAmount: 1_500_000,
	}
	rowA := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 100_000, Status: "deposit_paid", MPPaymentID: &mpA,
	}
	rowB := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 50_000, Status: "deposit_paid", MPPaymentID: &mpB,
	}
	f.payments.byBookingAll = []*paymentstore.Payment{rowA, rowB}
	f.payments.claimErrFor = map[uuid.UUID]error{rowA.ID: errors.New("connection reset")}
	f.payments.claimAmount = 50_000
	f.provider.refundAmount = 500.00
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	standing := true
	f.payments.onClaim = func() { standing = false }
	f.refundIntents.onMark = func() { standing = true }

	outcome := f.service.AutoRefundIfPaid(t.Context(), booking)

	if !standing {
		t.Fatal("row A's claim failed for a moment, so the marker must still stand for the sweep once row B is claimed")
	}
	if outcome.Result != paymentstore.RefundQueued || outcome.NeedsAHuman() {
		t.Errorf("row A is retried by the sweep, so the booking reads queued; got %+v", outcome)
	}
}
