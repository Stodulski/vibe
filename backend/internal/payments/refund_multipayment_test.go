package payments

import (
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// A booking can carry more than one payment row: the deposit paid online
// through MercadoPago, then the remaining balance ConfirmPayment recorded in
// cash at the desk (internal/bookings/actions.go). Before ListByBookingID,
// refundable() read a single row through GetByBookingID — which prefers the
// MercadoPago row when more than one exists (db/queries/payments.sql) — so
// the cash balance was invisible to the refund path entirely: on
// cancellation, the MP deposit refunded and the cash balance was refunded
// from nowhere. owedManually never ran for it, and the "MANUAL REFUND OWED"
// Sentry alert — the only mechanism that tells an operator money is owed by
// hand — never fired.
//
// This drives that exact shape: one MP row, one cash row, both unrefunded.
func TestAutoRefundIssuesTheMercadoPagoRowAndAlertsOnTheCashRemainder(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()
	mpID := "mp-" + uuid.NewString()

	booking := &data.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "confirmed", CollectionStatus: data.CollectionStatusFullyPaid,
		RefundStatus: data.RefundStatusNone, Price: 500_000, DepositAmount: 150_000,
	}
	deposit := &data.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 150_000, ServiceFee: 7_500, Status: "deposit_paid", MPPaymentID: &mpID,
	}
	cash := &data.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 350_000, ServiceFee: 0, Status: "deposit_paid", Method: "cash",
	}
	f.payments.byBookingAll = []*data.Payment{deposit, cash}
	f.payments.claimAmount = 157_500
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.provider.refundAmount = 1_575.00

	sentryEvents := withCapturedSentryEvents(t)

	outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.payments.claimed) != 1 || f.payments.claimed[0] != deposit.ID {
		t.Fatalf("only the MercadoPago row may be claimed; got claimed=%v", f.payments.claimed)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("the MercadoPago row must be refunded exactly once; got %d", len(f.payments.recordedSuccess))
	}
	if want := cash.Amount + cash.ServiceFee; len(f.payments.recordedSuccessManualOwed) != 1 || f.payments.recordedSuccessManualOwed[0] != want {
		t.Errorf("RecordRefundSuccess must be told the cash row's balance, or the booking is written 'refunded' with cash still owed; want %d, got %v",
			want, f.payments.recordedSuccessManualOwed)
	}

	if outcome.Result != data.RefundIssued {
		t.Errorf("the automatic half succeeded, so the result describes it; want %q, got %q (reason %q)",
			data.RefundIssued, outcome.Result, outcome.Reason)
	}
	if outcome.AmountCentavos != 157_500 {
		t.Errorf("AmountCentavos must be what the MercadoPago row returned; want 157500, got %d", outcome.AmountCentavos)
	}
	if want := cash.Amount + cash.ServiceFee; outcome.ManualAmountCentavos != want {
		t.Errorf("ManualAmountCentavos must carry the cash row's full remainder; want %d, got %d", want, outcome.ManualAmountCentavos)
	}
	if !outcome.NeedsAHuman() {
		t.Error("a split outcome still needs a person for the manual half, even though the automatic half succeeded")
	}

	messages := sentryEvents.messages()
	alert, found := findMessage(messages, "MANUAL REFUND OWED")
	if !found {
		t.Fatalf("the cash remainder must alert as money owed by hand; captured %q", messages)
	}
	wantAmount := cash.Amount + cash.ServiceFee
	if !strings.Contains(alert, strconv.Itoa(wantAmount)) {
		t.Errorf("the alert must carry the cash row's own amount (%d), not the deposit's; got %q", wantAmount, alert)
	}
}

// Two payment rows, neither carrying a MercadoPago id — a booking paid twice
// in cash, or a cash deposit followed by a cash balance. Both are summed into
// one manual outcome and one alert, not one silent loss per extra row.
func TestAutoRefundSumsMultipleCashRowsIntoOneManualOutcome(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()

	booking := &data.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "confirmed", CollectionStatus: data.CollectionStatusFullyPaid,
		RefundStatus: data.RefundStatusNone, Price: 500_000, DepositAmount: 100_000,
	}
	deposit := &data.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 100_000, ServiceFee: 0, Status: "deposit_paid", Method: "cash",
	}
	balance := &data.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 50_000, ServiceFee: 0, Status: "deposit_paid", Method: "cash",
	}
	f.payments.byBookingAll = []*data.Payment{deposit, balance}

	sentryEvents := withCapturedSentryEvents(t)

	outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.payments.claimed) != 0 {
		t.Fatalf("neither row carries a mercadopago id, so nothing may be claimed; got claimed=%v", f.payments.claimed)
	}
	if outcome.Result != data.RefundManual {
		t.Fatalf("no automatic row exists, so this is a plain manual outcome; got %q", outcome.Result)
	}
	want := deposit.Amount + balance.Amount
	if outcome.AmountCentavos != want {
		t.Errorf("the two cash rows must be summed; want %d, got %d", want, outcome.AmountCentavos)
	}
	if outcome.ManualAmountCentavos != 0 {
		t.Errorf("a pure manual outcome carries its sum in AmountCentavos, not a second ManualAmountCentavos; got %d",
			outcome.ManualAmountCentavos)
	}

	messages := sentryEvents.messages()
	alert, found := findMessage(messages, "MANUAL REFUND OWED")
	if !found {
		t.Fatalf("the summed cash owed must alert exactly once; captured %q", messages)
	}
	if !strings.Contains(alert, strconv.Itoa(want)) {
		t.Errorf("the alert must carry the summed amount; got %q", alert)
	}
}
