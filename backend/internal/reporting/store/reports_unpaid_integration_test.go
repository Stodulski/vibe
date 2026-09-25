//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// TestPaymentSummaryByMethodAlreadyExcludesUnpaidAbandonedCheckouts is a
// regression guard, not a fix: unlike bookingstore.GetDashboardStats (which
// used to filter WHERE status != 'refunded' and quietly counted abandoned
// MercadoPago checkouts as revenue), the monthly export's
// countedPaymentStatuses already excludes 'unpaid' by construction — it is a
// positive allowlist ('deposit_paid', 'fully_paid', 'refunded',
// 'refund_pending'), not a "not refunded" exclusion. This test pins that the
// monthly report was never exposed to the same bug, so a future edit to
// countedPaymentStatuses cannot reintroduce it unnoticed.
func TestPaymentSummaryByMethodAlreadyExcludesUnpaidAbandonedCheckouts(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	paid := f.CreateBooking(t, datatest.BookingOptions{StartTime: "09:00", EndTime: "10:00"})
	f.CreatePayment(t, paid.ID, 30_000, 0, nil) // datatest's default status: deposit_paid, method: cash

	abandoned := f.CreateBooking(t, datatest.BookingOptions{StartTime: "10:00", EndTime: "11:00"})
	mpID := "mp-abandoned-1"
	unpaid := f.CreatePayment(t, abandoned.ID, 50_000, 0, &mpID) // method becomes mercadopago
	if _, err := f.DB.Exec(ctx, `UPDATE payments SET status = 'unpaid' WHERE id = $1`, unpaid.ID); err != nil {
		t.Fatalf("marking payment unpaid: %v", err)
	}

	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)

	store := &reportstore.Store{DB: f.DB}
	summaries, err := store.PaymentSummaryByMethod(f.Scoped(ctx), f.ComplexID, from, to)
	if err != nil {
		t.Fatalf("PaymentSummaryByMethod: %v", err)
	}

	for _, s := range summaries {
		if s.Method == "mercadopago" {
			t.Errorf("monthly report counted the unpaid abandoned checkout: %+v", s)
		}
	}

	var cashTotal int
	for _, s := range summaries {
		if s.Method == "cash" {
			cashTotal = s.Amount
		}
	}
	if cashTotal != 30_000 {
		t.Errorf("cash total = %d, want 30000", cashTotal)
	}
}
