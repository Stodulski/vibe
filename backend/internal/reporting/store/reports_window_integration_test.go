//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// TestIntegration_PaymentSummaryByMethodWindowFiltersByPreciseTimestamps pins
// PaymentSummaryByMethodWindow (added for pos-cashbox's cash session
// reconciliation, internal/cashbox) against a real database: a session's
// window is [opened_at, closed_at-or-now), not a calendar day, and this is
// the query that decides which booking payments a cash session counts as
// "collected".
func TestIntegration_PaymentSummaryByMethodWindowFiltersByPreciseTimestamps(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})

	// One payment inside the window, one before it, one exactly at `from` and
	// one exactly at `to` — the window is half-open, [from, to), so the first
	// three must be counted and the fourth must not.
	inWindow := f.CreatePayment(t, booking.ID, 30000, 0, nil)
	beforeWindow := f.CreatePayment(t, booking.ID, 40000, 0, nil)
	atStart := f.CreatePayment(t, booking.ID, 7000, 0, nil)
	atEnd := f.CreatePayment(t, booking.ID, 50000, 0, nil)

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(1 * time.Hour)

	// Fixed instants, not time.Now() plus a duration computed after from/to
	// were: two separate time.Now() calls a moment apart used to make
	// "exactly at `to`" actually land a few microseconds past it, which
	// happened to still be excluded but for the wrong reason — this pins the
	// boundary itself, not a coincidence next to it.
	backdate := func(id string, at time.Time) {
		if _, err := f.DB.Exec(ctx, `UPDATE payments SET created_at = $2 WHERE id = $1`, id, at); err != nil {
			t.Fatalf("backdating payment %s: %v", id, err)
		}
	}
	backdate(inWindow.ID.String(), time.Now())
	backdate(beforeWindow.ID.String(), from.Add(-time.Hour))
	backdate(atStart.ID.String(), from) // exactly at `from`, which must be included
	backdate(atEnd.ID.String(), to)     // exactly at `to`, which must be excluded

	store := &reportstore.Store{DB: f.DB}
	summaries, err := store.PaymentSummaryByMethodWindow(f.Scoped(ctx), f.ComplexID, from, to)
	if err != nil {
		t.Fatalf("PaymentSummaryByMethodWindow: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("want exactly one method summary (cash); got %d: %+v", len(summaries), summaries)
	}
	got := summaries[0]
	if got.Method != "cash" {
		t.Fatalf("want method=cash; got %s", got.Method)
	}
	if got.Count != 2 {
		t.Errorf("want count=2 (the in-window payment and the one exactly at `from`); got %d", got.Count)
	}
	if got.Amount != 30000+7000 {
		t.Errorf("want amount=37000 (in-window plus at-`from`, not before or at-`to`); got %d", got.Amount)
	}
}

// TestIntegration_ManualRefundSummaryByMethodWindowFiltersOnManualRefundedAt
// pins ManualRefundSummaryByMethodWindow (cash-manual-refunds): unlike
// PaymentSummaryByMethodWindow, it filters on manual_refunded_at — when the
// money actually left the till — not on created_at, so a refund confirmed
// today against a payment taken long ago still lands in today's window, and
// a payment that was never manually refunded contributes nothing.
func TestIntegration_ManualRefundSummaryByMethodWindowFiltersOnManualRefundedAt(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})

	inWindow := f.CreatePayment(t, booking.ID, 30000, 0, nil)
	outsideWindow := f.CreatePayment(t, booking.ID, 40000, 0, nil)
	neverRefunded := f.CreatePayment(t, booking.ID, 90000, 0, nil)
	_ = neverRefunded

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(1 * time.Hour)

	markManuallyRefunded := func(id string, amount int, at time.Time) {
		if _, err := f.DB.Exec(ctx,
			`UPDATE payments SET status = 'refunded', manual_refund_amount = $2, manual_refunded_at = $3 WHERE id = $1`,
			id, amount, at,
		); err != nil {
			t.Fatalf("marking payment %s manually refunded: %v", id, err)
		}
	}
	markManuallyRefunded(inWindow.ID.String(), 30000, time.Now())
	markManuallyRefunded(outsideWindow.ID.String(), 40000, from.Add(-time.Hour))

	store := &reportstore.Store{DB: f.DB}
	summaries, err := store.ManualRefundSummaryByMethodWindow(f.Scoped(ctx), f.ComplexID, from, to)
	if err != nil {
		t.Fatalf("ManualRefundSummaryByMethodWindow: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("want exactly one method summary (cash); got %d: %+v", len(summaries), summaries)
	}
	got := summaries[0]
	if got.Method != "cash" {
		t.Fatalf("want method=cash; got %s", got.Method)
	}
	if got.Count != 1 {
		t.Errorf("want count=1 (only the in-window refund); got %d", got.Count)
	}
	if got.Amount != 30000 {
		t.Errorf("want amount=30000 (the in-window refund only); got %d", got.Amount)
	}
}
