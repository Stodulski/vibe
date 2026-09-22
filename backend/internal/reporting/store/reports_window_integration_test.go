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

	// One payment inside the window, one before it, one at-or-after the
	// window's end (the window is half-open: [from, to)).
	inWindow := f.CreatePayment(t, booking.ID, 30000, 0, nil)
	beforeWindow := f.CreatePayment(t, booking.ID, 40000, 0, nil)
	atEnd := f.CreatePayment(t, booking.ID, 50000, 0, nil)

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(1 * time.Hour)

	backdate := func(id string, delta time.Duration) {
		if _, err := f.DB.Exec(ctx, `UPDATE payments SET created_at = $2 WHERE id = $1`, id, time.Now().Add(delta)); err != nil {
			t.Fatalf("backdating payment %s: %v", id, err)
		}
	}
	backdate(inWindow.ID.String(), 0)
	backdate(beforeWindow.ID.String(), -2*time.Hour)
	backdate(atEnd.ID.String(), 1*time.Hour) // exactly at `to`, which must be excluded

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
	if got.Count != 1 {
		t.Errorf("want count=1 (only the in-window payment); got %d", got.Count)
	}
	if got.Amount != 30000 {
		t.Errorf("want amount=30000 (the in-window payment only, not before or at-end); got %d", got.Amount)
	}
}
