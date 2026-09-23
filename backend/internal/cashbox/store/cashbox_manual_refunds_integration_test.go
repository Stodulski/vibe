//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data/datatest"
)

// TestIntegration_ExpectedCashEndToEndSubtractsOnlyInWindowCashManualRefunds
// exercises the whole cash-manual-refunds path against a real database: a
// cash payment manually refunded inside the session window lowers expected
// cash by exactly what was handed back; a transfer refund and a refund
// outside the window do not. It reads the window through
// reportstore.Store — the same store the cashbox service calls — rather than
// re-deriving the SQL, and closes the session with the value that store
// query actually returns.
func TestIntegration_ExpectedCashEndToEndSubtractsOnlyInWindowCashManualRefunds(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	bg := context.Background()

	session := openSession(t, f, 10000)

	booking := f.CreateBooking(t, datatest.BookingOptions{})

	// In window: a cash payment, collected and then manually refunded within
	// the session's own window — this is the amount expected cash must drop
	// by.
	inWindowCash := f.CreatePayment(t, booking.ID, 50000, 0, nil)
	if _, err := f.DB.Exec(bg,
		`UPDATE payments SET status = 'refunded', refund_amount = $2,
		        manual_refund_amount = $2, manual_refunded_at = NOW()
		 WHERE id = $1`,
		inWindowCash.ID, 20000,
	); err != nil {
		t.Fatalf("marking in-window cash payment manually refunded: %v", err)
	}

	// A transfer refund of the same shape, same window: must not touch cash.
	transferRefund := f.CreatePayment(t, booking.ID, 15000, 0, nil)
	if _, err := f.DB.Exec(bg,
		`UPDATE payments SET method = 'transfer', status = 'refunded', refund_amount = $2,
		        manual_refund_amount = $2, manual_refunded_at = NOW()
		 WHERE id = $1`,
		transferRefund.ID, 15000,
	); err != nil {
		t.Fatalf("marking transfer payment manually refunded: %v", err)
	}

	// A cash refund confirmed before this session ever opened: out of window.
	outsideWindowCash := f.CreatePayment(t, booking.ID, 40000, 0, nil)
	if _, err := f.DB.Exec(bg,
		`UPDATE payments SET status = 'refunded', refund_amount = $2,
		        manual_refund_amount = $2, manual_refunded_at = $3, created_at = $3
		 WHERE id = $1`,
		outsideWindowCash.ID, 40000, session.OpenedAt.Add(-time.Hour),
	); err != nil {
		t.Fatalf("marking outside-window cash payment manually refunded: %v", err)
	}

	closedAt := time.Now()

	bookingPayments, err := f.Stores.Reports.PaymentSummaryByMethodWindow(ctx, f.ComplexID, session.OpenedAt, closedAt)
	if err != nil {
		t.Fatalf("PaymentSummaryByMethodWindow: %v", err)
	}
	var cashBookingPayments int64
	for _, p := range bookingPayments {
		if p.Method == "cash" {
			cashBookingPayments += int64(p.Amount + p.ServiceFee)
		}
	}

	manualRefunds, err := f.Stores.Reports.ManualRefundSummaryByMethodWindow(ctx, f.ComplexID, session.OpenedAt, closedAt)
	if err != nil {
		t.Fatalf("ManualRefundSummaryByMethodWindow: %v", err)
	}
	var cashManualRefunds int64
	for _, r := range manualRefunds {
		if r.Method == "cash" {
			cashManualRefunds += int64(r.Amount)
		}
	}
	if cashManualRefunds != 20000 {
		t.Fatalf("want cashManualRefunds=20000 (only the in-window cash refund); got %d (rows=%+v)", cashManualRefunds, manualRefunds)
	}

	closed, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID,
		0, cashBookingPayments, cashManualRefunds, closedAt, nil)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	wantExpected := 10000 + cashBookingPayments - cashManualRefunds
	if closed.ExpectedCash == nil || *closed.ExpectedCash != wantExpected {
		t.Fatalf("want expected_cash=%d (opening + cash booking payments - only the in-window cash manual refund); got %+v",
			wantExpected, closed.ExpectedCash)
	}

	// Rebuilding the same window after close (the same two reportstore calls
	// a fresh GET of this session's summary would make) must agree exactly
	// with what Close already wrote — "closing right after a refund stores
	// an expected cash that matches the rebuilt summary".
	rebuiltBookingPayments, err := f.Stores.Reports.PaymentSummaryByMethodWindow(ctx, f.ComplexID, session.OpenedAt, closedAt)
	if err != nil {
		t.Fatalf("rebuild PaymentSummaryByMethodWindow: %v", err)
	}
	var rebuiltCashBookingPayments int64
	for _, p := range rebuiltBookingPayments {
		if p.Method == "cash" {
			rebuiltCashBookingPayments += int64(p.Amount + p.ServiceFee)
		}
	}
	rebuiltManualRefunds, err := f.Stores.Reports.ManualRefundSummaryByMethodWindow(ctx, f.ComplexID, session.OpenedAt, closedAt)
	if err != nil {
		t.Fatalf("rebuild ManualRefundSummaryByMethodWindow: %v", err)
	}
	var rebuiltCashManualRefunds int64
	for _, r := range rebuiltManualRefunds {
		if r.Method == "cash" {
			rebuiltCashManualRefunds += int64(r.Amount)
		}
	}
	rebuiltExpected := 10000 + rebuiltCashBookingPayments - rebuiltCashManualRefunds
	if *closed.ExpectedCash != rebuiltExpected {
		t.Fatalf("closed.ExpectedCash=%d disagrees with the rebuilt summary's %d", *closed.ExpectedCash, rebuiltExpected)
	}
}
