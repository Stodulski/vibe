//go:build integration

package store_test

import (
	"context"
	"testing"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// TestGetDashboardStatsExcludesUnpaidAbandonedCheckouts pins the money bug
// found in review: every public booking inserts a payments row with
// status='unpaid', method='mercadopago' the instant checkout starts
// (internal/bookings/service_public.go), and that row is never cleaned up if
// the player abandons — payment_status has no "expired"/"cancelled" value.
// TodayRevenue used to filter WHERE status != 'refunded', which reads as
// "everything except a refund" but also let every abandoned checkout count as
// revenue. It must now filter on paymentstatus.CollectedStatuses instead.
func TestGetDashboardStatsExcludesUnpaidAbandonedCheckouts(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	today := timezone.Today()
	todayNoon := argentinaNoonOffset(0)

	paid := f.CreateBooking(t, datatest.BookingOptions{StartTime: "09:00", EndTime: "10:00"})
	paidPayment := f.CreatePayment(t, paid.ID, 30_000, 0, nil) // datatest's default status: deposit_paid
	backdatePaymentCreatedAt(t, f, paidPayment.ID, todayNoon)

	abandoned := f.CreateBooking(t, datatest.BookingOptions{StartTime: "10:00", EndTime: "11:00"})
	insertPaymentWithStatus(t, f, abandoned.ID, 50_000, "mercadopago", "unpaid", todayNoon)

	stats, err := f.Stores.Bookings.GetDashboardStats(ctx, f.ComplexID, today)
	if err != nil {
		t.Fatalf("GetDashboardStats: %v", err)
	}

	if stats.TodayRevenue != 30_000 {
		t.Errorf("TodayRevenue = %d, want 30000 — the 50000 unpaid abandoned checkout must not count as revenue", stats.TodayRevenue)
	}
}

// TestGetPaymentSummaryExcludesUnpaidAbandonedCheckouts covers the same fix
// in GetPaymentSummary's by-status and by-method breakdowns, which the
// dashboard's payment-status/method cards read.
func TestGetPaymentSummaryExcludesUnpaidAbandonedCheckouts(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	today := timezone.Today()
	todayNoon := argentinaNoonOffset(0)

	paid := f.CreateBooking(t, datatest.BookingOptions{StartTime: "09:00", EndTime: "10:00"})
	paidPayment := f.CreatePayment(t, paid.ID, 30_000, 0, nil)
	backdatePaymentCreatedAt(t, f, paidPayment.ID, todayNoon)

	abandoned := f.CreateBooking(t, datatest.BookingOptions{StartTime: "10:00", EndTime: "11:00"})
	insertPaymentWithStatus(t, f, abandoned.ID, 50_000, "mercadopago", "unpaid", todayNoon)

	summary, err := f.Stores.Bookings.GetPaymentSummary(ctx, f.ComplexID, today)
	if err != nil {
		t.Fatalf("GetPaymentSummary: %v", err)
	}

	if got, ok := summary.ByMethod["mercadopago"]; ok && got != 0 {
		t.Errorf("ByMethod[mercadopago] = %d, want 0 or absent — the only mercadopago row today is unpaid", got)
	}
	if got, ok := summary.ByStatus["unpaid"]; ok && got.Total != 0 {
		t.Errorf("ByStatus[unpaid].Total = %d, want 0 — an unpaid booking's own payment must never carry a collected amount", got.Total)
	}
	if summary.ByMethod["cash"] != 30_000 {
		t.Errorf("ByMethod[cash] = %d, want 30000", summary.ByMethod["cash"])
	}
}
