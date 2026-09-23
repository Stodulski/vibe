//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/cashbox"
	"github.com/stodulski/vibe-server/internal/data/datatest"
)

// noopRecorder discards every audit entry. This test exercises cashbox's
// money arithmetic end to end; it has nothing to say about the audit trail.
type noopRecorder struct{}

func (noopRecorder) Record(audit.Entry) {}

// TestIntegration_ExpectedCashEndToEndSubtractsOnlyInWindowCashManualRefunds
// exercises the whole cash-manual-refunds path against a real database,
// through cashbox.Service itself — the exact summing code (buildSummary,
// sumCashBookingPayments, sumCashManualRefunds) a real request runs — rather
// than re-deriving the per-method totals with a second, inline loop over the
// same reportstore rows. A cash payment manually refunded inside the
// session's window lowers expected cash by exactly what was handed back; a
// transfer refund and a refund outside the window do not.
func TestIntegration_ExpectedCashEndToEndSubtractsOnlyInWindowCashManualRefunds(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	bg := context.Background()

	svc := cashbox.NewService(f.Stores.Cashbox, f.Stores.Reports, noopRecorder{})

	session := openSession(t, f, 10000)

	booking := f.CreateBooking(t, datatest.BookingOptions{})

	// In window: a cash payment, collected and then manually refunded within
	// the session's own window — this is the amount expected cash must drop
	// by. The refund is partial (20000 of the 50000 paid) so the test also
	// pins the manual_refund_amount/refund_amount split (T1): only the owed
	// difference feeds the cashbox, never the row's whole amount.
	inWindowCash := f.CreatePayment(t, booking.ID, 50000, 0, nil)
	if _, err := f.DB.Exec(bg,
		`UPDATE payments SET status = 'refunded', refund_amount = amount + service_fee,
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

	// Live summary, through the service, before close: 10000 (opening) +
	// 50000 (the in-window cash booking payment, still counted — 'refunded'
	// is a counted status) - 20000 (only the in-window cash manual refund) =
	// 40000.
	current, err := svc.Current(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Summary.CashManualRefunds != 20000 {
		t.Fatalf("want CashManualRefunds=20000 (only the in-window cash refund); got %d (rows=%+v)",
			current.Summary.CashManualRefunds, current.Summary.ManualRefunds)
	}
	const wantExpected int64 = 40000
	if current.Summary.ExpectedCash != wantExpected {
		t.Fatalf("want live expected_cash=%d; got %d", wantExpected, current.Summary.ExpectedCash)
	}

	closed, err := svc.Close(ctx, f.ComplexID, session.ID, cashbox.Actor{UserID: &f.UserID}, f.UserID, cashbox.CloseInput{CountedCash: 0})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closed.ExpectedCash == nil || *closed.ExpectedCash != wantExpected {
		t.Fatalf("want stored expected_cash=%d (opening + in-window cash booking payment - only the in-window cash manual refund); got %+v",
			wantExpected, closed.ExpectedCash)
	}

	// Rebuilding the same window after close — the same service call a fresh
	// GET of this session's summary would make — must agree exactly with what
	// Close already wrote: "closing right after a refund stores an expected
	// cash that matches the rebuilt summary".
	detail, err := svc.Get(ctx, f.ComplexID, session.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if detail.Summary.ExpectedCash != *closed.ExpectedCash {
		t.Fatalf("closed.ExpectedCash=%d disagrees with the rebuilt summary's %d", *closed.ExpectedCash, detail.Summary.ExpectedCash)
	}
}
