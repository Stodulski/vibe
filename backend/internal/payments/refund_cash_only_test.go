package payments

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A booking paid only at the counter has no MercadoPago row, so nothing automatic
// can send the money back and the cash balance is the whole of what the client
// is owed. The owner's "mark manual refund" action is offered only on
// refund_status 'partial' (ManualRefund in internal/bookings/service.go), and the
// mixed MercadoPago-plus-cash path writes 'partial' for exactly this remainder.
// The cash-only path used to leave the booking at 'none', so the owner had no
// action that could close the debt.
func TestCancellingACashOnlyBookingLeavesItOpenForTheManualRefund(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()

	booking := &bookingstore.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		RefundStatus: bookingstore.RefundStatusNone, Price: 500_000, DepositAmount: 150_000,
	}
	cash := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 150_000, Status: "deposit_paid", Method: "cash",
	}
	f.payments.byBookingAll = []*paymentstore.Payment{cash}
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
	withCapturedSentryEvents(t)

	outcome := f.service.AutoRefundIfPaid(t.Context(), booking)

	if outcome.Result != paymentstore.RefundManual || outcome.AmountCentavos != 150_000 {
		t.Fatalf("the cash balance is owed by hand; want %q for 150000, got %q for %d (reason %q)",
			paymentstore.RefundManual, outcome.Result, outcome.AmountCentavos, outcome.Reason)
	}
	if len(f.bookings.markedOwed) != 1 || f.bookings.markedOwed[0] != booking.ID {
		t.Fatalf("the cash-only booking must be marked with its balance owed by hand (refund_status partial) so the owner can mark the manual refund; marked %v", f.bookings.markedOwed)
	}
	// A whole-row write from the snapshot read before the refund would revert
	// any change another writer (an owner edit) made in between.
	if f.bookings.updated != nil {
		t.Errorf("the balance must be marked without rewriting the whole booking; got a full write %+v", f.bookings.updated)
	}
	if booking.RefundStatus != bookingstore.RefundStatusPartial {
		t.Errorf("the caller's booking must carry the status that was written; got %q", booking.RefundStatus)
	}
}

// PublicBook inserts a checkout row with status 'unpaid' and no MercadoPago
// payment id before the client has paid anything (internal/bookings/service_public.go).
// The manual balance used to count that row as money collected, so a booking the
// client paid 10,000 of at the counter read as owing 13,000 by hand. Only rows
// whose status says money reached the club may be summed.
func TestAnUnpaidCheckoutRowIsNotCountedAsMoneyTheClubHolds(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()
	preference := "pref-" + uuid.NewString()

	booking := &bookingstore.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		RefundStatus: bookingstore.RefundStatusNone, Price: 1_500_000, DepositAmount: 1_000_000,
	}
	cash := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 1_000_000, Status: "deposit_paid", Method: "cash",
	}
	checkout := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 1_300_000, Status: "unpaid", Method: "mercadopago", MPPreferenceID: &preference,
	}
	f.payments.byBookingAll = []*paymentstore.Payment{cash, checkout}
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
	sentryEvents := withCapturedSentryEvents(t)

	outcome := f.service.AutoRefundIfPaid(t.Context(), booking)

	if want := cash.Amount; outcome.AmountCentavos != want {
		t.Fatalf("only the 10,000 paid at the counter is owed back; want %d, got %d", want, outcome.AmountCentavos)
	}
	alert, found := findMessage(sentryEvents.messages(), "MANUAL REFUND OWED")
	if !found {
		t.Fatalf("the cash remainder must alert as money owed by hand; captured %q", sentryEvents.messages())
	}
	if want := fmt.Sprintf("amount=%d", cash.Amount); !strings.Contains(alert, want) {
		t.Errorf("the alert must carry the collected amount (%s); got %q", want, alert)
	}
	if strings.Contains(alert, fmt.Sprintf("amount=%d", cash.Amount+checkout.Amount)) {
		t.Errorf("the unpaid checkout row must not be in the alert; got %q", alert)
	}
}
