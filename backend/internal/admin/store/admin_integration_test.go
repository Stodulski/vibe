//go:build integration

package store_test

import (
	"context"
	"testing"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// int32Max is the ceiling an ::int cast silently imposes on a SUM(). Postgres does
// not truncate past it — it raises "integer out of range" and the whole query fails.
const int32Max = 2_147_483_647

// TestGetPlatformStatsSurvivesRevenueBeyondInt32 pins the overflow that permanently
// 500s the admin dashboard.
//
// payments.amount is INTEGER, so SUM(amount) returns BIGINT. Casting it back with
// ::int made GetPlatformStats fail the moment cumulative non-refunded payments
// crossed 2,147,483,647. The unit is centavos — internal/pricing documents
// feeMinCentavos = 100_000 as "1000 ARS, in centavos" and internal/mp divides by 100
// for MercadoPago's unit price — so the ceiling is only ~21,474,836 ARS platform-wide,
// about 1,400 payments at a 15,000 ARS average. Once crossed the endpoint never
// recovers, because the sum only grows.
func TestGetPlatformStatsSurvivesRevenueBeyondInt32(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	// GetPlatformStats is platform-wide, so it also counts whatever the shared E2E
	// database already holds. Read that baseline rather than assuming this test's
	// rows are the only ones.
	var baseline int64
	err := f.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::bigint FROM payments WHERE status != 'refunded'`,
	).Scan(&baseline)
	if err != nil {
		t.Fatalf("reading the revenue baseline: %v", err)
	}

	// payments.amount is INTEGER, so no single row can carry more than int32Max.
	// Three rows near the per-row ceiling put the SUM comfortably past it, which is
	// the whole point — a handful of large rows, not a million small ones.
	const perRow = 2_000_000_000
	const rowCount = 3

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	for range rowCount {
		f.CreatePayment(t, booking.ID, perRow, 0, nil)
	}

	want := baseline + rowCount*perRow
	if want <= int32Max {
		t.Fatalf("the seeded revenue must exceed the int32 ceiling to exercise the bug; got %d", want)
	}

	stats, err := f.Stores.Admin.GetPlatformStats(ctx)
	if err != nil {
		t.Fatalf("GetPlatformStats over %d centavos of non-refunded revenue: %v", want, err)
	}

	if int64(stats.TotalRevenue) != want {
		t.Errorf("total_revenue must carry the full BIGINT sum; want %d, got %d", want, stats.TotalRevenue)
	}
	if int64(stats.TotalRevenue) <= int32Max {
		t.Errorf("total_revenue was not summed past the int32 ceiling; got %d", stats.TotalRevenue)
	}
}

// TestGetComplexDetailSurvivesRevenueBeyondInt32 covers the second ::int cast over
// the same column. It is scoped to one complex rather than the platform, so it takes
// longer to reach — but it is the same defect one busy tenant away, and it fails the
// same way: the whole detail query errors out, not just the revenue figure.
func TestGetComplexDetailSurvivesRevenueBeyondInt32(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	const perRow = 2_000_000_000
	const rowCount = 3

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	for range rowCount {
		f.CreatePayment(t, booking.ID, perRow, 0, nil)
	}

	// This fixture's complex is freshly created, so its own payments are the only
	// ones in scope and the expected total is exact.
	want := int64(rowCount) * perRow
	if want <= int32Max {
		t.Fatalf("the seeded revenue must exceed the int32 ceiling to exercise the bug; got %d", want)
	}

	detail, err := f.Stores.Admin.GetComplexDetail(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetComplexDetail over %d centavos of non-refunded revenue: %v", want, err)
	}

	if int64(detail.TotalRevenue) != want {
		t.Errorf("total_revenue must carry the full BIGINT sum; want %d, got %d", want, detail.TotalRevenue)
	}
}
