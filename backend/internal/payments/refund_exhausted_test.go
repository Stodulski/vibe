package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A refund that spends its whole retry budget against MercadoPago used to end in
// a Sentry alert and an audit entry, and nothing that reached the client. The
// client had been told at cancellation that the refund was in progress, so they
// were left waiting for a message that would never come. They must now hear, once,
// that the refund is taking longer than expected and is still being processed. It
// must not invite the club to pay by hand: MercadoPago can still pay it.
func TestAnExhaustedRefundTellsTheClientTheRefundIsDelayed(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	f.bookings.booking = booking
	f.payments.byBooking = payment
	f.payments.claimAmount = 150_000
	f.provider.refundErr = errProvider
	f.payments.exhausted = true
	f.complexes.complex = linkedComplex(complexID, "")
	f.complexes.complex.Name = "Vibe Palermo"
	email := "ana@example.com"
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", Email: &email}

	f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.notify.refundDelayed) != 1 {
		t.Fatalf("the client must be told once that the refund is still being processed; got %d notices", len(f.notify.refundDelayed))
	}
	got := f.notify.refundDelayed[0]
	if got.Email != email || got.ComplexName != "Vibe Palermo" {
		t.Errorf("the notice must go to the client about this complex; got email %q complex %q", got.Email, got.ComplexName)
	}
	if want := "$1.500"; got.Amount != want {
		t.Errorf("the notice must name the amount that is still being processed; want %q, got %q", want, got.Amount)
	}
	if got.BookingID != booking.ID.String() || got.RefundID == "" {
		t.Errorf("the notice must identify its booking and refund so a redelivery is one notice; got booking %q refund %q", got.BookingID, got.RefundID)
	}
	if len(f.notify.refunds) != 0 {
		t.Errorf("the client must not hear that a deposit is on its way back; got %d deposit notices", len(f.notify.refunds))
	}
}

// Once the automatic half has failed for good, the cash the club still holds is the
// only money a person can return. The owner's manual-refund action is offered on
// refund_status 'partial' alone, so the booking must be left there for them.
func TestAnExhaustedRefundLeavesTheCashBalanceOpenForTheOwner(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()
	mpID := "mp-" + uuid.NewString()

	booking := &bookingstore.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		RefundStatus: bookingstore.RefundStatusNone, Price: 500_000, DepositAmount: 150_000,
	}
	mpRow := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 150_000, Status: "deposit_paid", MPPaymentID: &mpID,
	}
	cash := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 50_000, Status: "deposit_paid", Method: "cash",
	}
	f.bookings.booking = booking
	f.payments.byBookingAll = []*paymentstore.Payment{mpRow, cash}
	f.provider.refundErr = errProvider
	f.payments.exhausted = true
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
	withCapturedSentryEvents(t)

	outcome := f.service.AutoRefundIfPaid(t.Context(), booking)

	if !outcome.NeedsAHuman() {
		t.Fatalf("the exhausted refund and the cash balance both need a person; got %+v", outcome)
	}
	if f.bookings.updated == nil {
		t.Fatal("the booking must be written with its cash balance owed by hand; nothing was written")
	}
	if got := f.bookings.updated.RefundStatus; got != bookingstore.RefundStatusPartial {
		t.Errorf("want refund_status %q so the owner can mark the manual refund; got %q",
			bookingstore.RefundStatusPartial, got)
	}
}
