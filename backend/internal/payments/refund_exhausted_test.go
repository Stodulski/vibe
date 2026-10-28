package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A refund that spends its whole retry budget is not abandoned: the retry job keeps
// re-selecting exhausted rows, so MercadoPago can still pay it. The client hears
// about a refund only once it is actually done, so an exhausted attempt sends them
// nothing, and in particular never that a deposit is on its way back.
func TestAnExhaustedRefundSendsTheClientNothingUntilTheMoneyIsBack(t *testing.T) {
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
	email := "ana@example.com"
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", Email: &email}

	f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.notify.refunds) != 0 {
		t.Errorf("the client must hear about a refund only once it is done; got %d refund notices", len(f.notify.refunds))
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
	if len(f.bookings.markedOwed) != 1 || f.bookings.markedOwed[0] != booking.ID {
		t.Fatalf("the booking must be marked with its balance owed by hand (refund_status partial) so the owner can mark the manual refund; marked %v", f.bookings.markedOwed)
	}
	// A whole-row write from the snapshot read before the refund would revert
	// any change another writer (an owner edit) made in between.
	if f.bookings.updated != nil {
		t.Errorf("the balance must be marked without rewriting the whole booking; got a full write %+v", f.bookings.updated)
	}
}
