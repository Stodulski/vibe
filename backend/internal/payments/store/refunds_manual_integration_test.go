//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A checkout row is written 'unpaid' with no MercadoPago payment id when a client
// starts paying online (internal/bookings/service_public.go), and the cash the
// club actually holds is a separate row. The owner's manual-refund close-out used
// to mark every unrefunded row without a MercadoPago id as refunded and add its
// full price to the returned total, so a checkout the client never paid was both
// "handed back" and subtracted from the cashbox. Only money the club holds may be
// closed out by hand.
func TestRecordManualRefundLeavesAnUnpaidCheckoutRowAlone(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// Cancelled, as both cancel paths leave it before a refund: 'partial' is a
	// cancelled booking's state, and a live booking is never refunded this way.
	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled"})
	mpPaymentID := "mp-" + uuid.NewString()
	mpPayment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)
	cashPayment := f.CreatePayment(t, booking.ID, 50_000, 0, nil)

	// The checkout row, as PublicBook leaves it: unpaid, MercadoPago method, no payment id.
	checkout := f.CreatePayment(t, booking.ID, 130_000, 0, nil)
	if _, err := f.DB.Exec(ctx,
		`UPDATE payments SET status = 'unpaid', method = 'mercadopago', mp_preference_id = $2 WHERE id = $1`,
		checkout.ID, "pref-"+uuid.NewString(),
	); err != nil {
		t.Fatalf("setup: marking the checkout row unpaid: %v", err)
	}

	claim, err := f.Stores.Payments.ClaimRefund(ctx, mpPayment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}
	manualOwed := cashPayment.Amount + cashPayment.ServiceFee - cashPayment.RefundAmount
	if _, err := f.Stores.Payments.RecordRefundSuccess(ctx, *claim, manualOwed); err != nil {
		t.Fatalf("RecordRefundSuccess: %v", err)
	}
	if _, _, refundStatus := f.ReadBookingState(t, booking.ID); refundStatus != bookingstore.RefundStatusPartial {
		t.Fatalf("setup: booking must read refund_status 'partial' before RecordManualRefund runs; got %q", refundStatus)
	}

	returned, err := f.Stores.Payments.RecordManualRefund(ctx, booking.ID)
	if err != nil {
		t.Fatalf("RecordManualRefund: %v", err)
	}
	if returned != manualOwed {
		t.Errorf("only the cash the club holds is handed back by hand; want %d, got %d", manualOwed, returned)
	}

	checkoutStatus, checkoutRefund := f.ReadPaymentState(t, checkout.ID)
	if checkoutStatus != "unpaid" {
		t.Errorf("the unpaid checkout row must stay unpaid; got %q", checkoutStatus)
	}
	if checkoutRefund != 0 {
		t.Errorf("the unpaid checkout row must not record any refund; got %d", checkoutRefund)
	}
	var manualRefundAmount *int
	if err := f.DB.QueryRow(ctx,
		`SELECT manual_refund_amount FROM payments WHERE id = $1`, checkout.ID,
	).Scan(&manualRefundAmount); err != nil {
		t.Fatalf("reading the checkout row's manual refund amount: %v", err)
	}
	if manualRefundAmount != nil {
		t.Errorf("the unpaid checkout row must not carry a manual refund amount; got %d", *manualRefundAmount)
	}
}

// setupPendingMercadoPagoRefund leaves a booking the way a real cancellation with
// a split refund does: one MercadoPago payment refunded in full with the cash
// balance still owed (so the booking reads 'partial'), and a second MercadoPago
// payment whose refund is claimed and still in flight (refund_pending).
func setupPendingMercadoPagoRefund(t *testing.T) (f *datatest.Fixture, bookingID uuid.UUID, pendingClaim *paymentstore.RefundClaim, pending, cash *paymentstore.Payment) {
	t.Helper()
	ctx := context.Background()
	f = datatest.Isolated(t)

	// Cancelled, as both cancel paths leave it before a refund: see the note on
	// TestRecordManualRefundLeavesAnUnpaidCheckoutRowAlone.
	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled"})
	settledID := "mp-" + uuid.NewString()
	settled := f.CreatePayment(t, booking.ID, 150_000, 7_500, &settledID)
	pendingID := "mp-" + uuid.NewString()
	pending = f.CreatePayment(t, booking.ID, 100_000, 0, &pendingID)
	cash = f.CreatePayment(t, booking.ID, 50_000, 0, nil)

	settledClaim, err := f.Stores.Payments.ClaimRefund(ctx, settled.ID)
	if err != nil {
		t.Fatalf("ClaimRefund(settled): %v", err)
	}
	cashOwed := cash.Amount + cash.ServiceFee - cash.RefundAmount
	if _, err := f.Stores.Payments.RecordRefundSuccess(ctx, *settledClaim, cashOwed); err != nil {
		t.Fatalf("RecordRefundSuccess(settled): %v", err)
	}

	pendingClaim, err = f.Stores.Payments.ClaimRefund(ctx, pending.ID)
	if err != nil {
		t.Fatalf("ClaimRefund(pending): %v", err)
	}
	if status, _ := f.ReadPaymentState(t, pending.ID); status != "refund_pending" {
		t.Fatalf("setup: the second MercadoPago payment must read refund_pending; got %q", status)
	}
	if _, _, refundStatus := f.ReadBookingState(t, booking.ID); refundStatus != bookingstore.RefundStatusPartial {
		t.Fatalf("setup: booking must read refund_status 'partial' before the manual refund; got %q", refundStatus)
	}
	return f, booking.ID, pendingClaim, pending, cash
}

// While a MercadoPago refund on the booking is still in flight, the cash the club
// holds is not the whole of what is owed: the automatic half has not settled. The
// owner's close-out must be refused, and nothing may move: the booking stays
// 'partial' and the cash row stays unrefunded. Closing the booking to 'full' now
// would read as fully returned while the MercadoPago payment is still pending.
func TestRecordManualRefundRefusesWhileAMercadoPagoRefundIsPending(t *testing.T) {
	f, bookingID, _, pending, cash := setupPendingMercadoPagoRefund(t)
	ctx := context.Background()

	returned, err := f.Stores.Payments.RecordManualRefund(ctx, bookingID)
	if !errors.Is(err, paymentstore.ErrMercadoPagoRefundPending) {
		t.Fatalf("the close-out must be refused while a MercadoPago refund is pending; got returned=%d err=%v", returned, err)
	}

	if _, _, refundStatus := f.ReadBookingState(t, bookingID); refundStatus != bookingstore.RefundStatusPartial {
		t.Errorf("the refused close-out must leave the booking 'partial'; got %q", refundStatus)
	}
	if status, refund := f.ReadPaymentState(t, cash.ID); status != "deposit_paid" || refund != 0 {
		t.Errorf("the refused close-out must leave the cash row untouched; got status %q refund %d", status, refund)
	}
	if status, _ := f.ReadPaymentState(t, pending.ID); status != "refund_pending" {
		t.Errorf("the pending MercadoPago payment must stay refund_pending; got %q", status)
	}
}

// Once the pending MercadoPago refund has settled, the close-out goes through as
// it always did: the cash balance is returned and the booking reads 'full'.
func TestRecordManualRefundProceedsOnceTheMercadoPagoRefundSettles(t *testing.T) {
	f, bookingID, pendingClaim, _, cash := setupPendingMercadoPagoRefund(t)
	ctx := context.Background()

	cashOwed := cash.Amount + cash.ServiceFee - cash.RefundAmount
	if _, err := f.Stores.Payments.RecordRefundSuccess(ctx, *pendingClaim, cashOwed); err != nil {
		t.Fatalf("RecordRefundSuccess(pending): %v", err)
	}

	returned, err := f.Stores.Payments.RecordManualRefund(ctx, bookingID)
	if err != nil {
		t.Fatalf("RecordManualRefund after the MercadoPago refund settled: %v", err)
	}
	if returned != cashOwed {
		t.Errorf("the cash balance must be returned once nothing is in flight; want %d, got %d", cashOwed, returned)
	}
	if _, _, refundStatus := f.ReadBookingState(t, bookingID); refundStatus != bookingstore.RefundStatusFull {
		t.Errorf("the booking must read a full refund after the close-out; got %q", refundStatus)
	}
}
