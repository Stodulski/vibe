//go:build integration

package data

import (
	"context"
	"strings"
	"testing"
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
	f := newTestFixture(t)
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

	booking := f.createBooking(t, bookingOptions{})
	for range rowCount {
		f.createPayment(t, booking.ID, perRow, 0, nil)
	}

	want := baseline + rowCount*perRow
	if want <= int32Max {
		t.Fatalf("the seeded revenue must exceed the int32 ceiling to exercise the bug; got %d", want)
	}

	stats, err := f.Models.Admin.GetPlatformStats(ctx)
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
	f := newTestFixture(t)
	ctx := context.Background()

	const perRow = 2_000_000_000
	const rowCount = 3

	booking := f.createBooking(t, bookingOptions{})
	for range rowCount {
		f.createPayment(t, booking.ID, perRow, 0, nil)
	}

	// This fixture's complex is freshly created, so its own payments are the only
	// ones in scope and the expected total is exact.
	want := int64(rowCount) * perRow
	if want <= int32Max {
		t.Fatalf("the seeded revenue must exceed the int32 ceiling to exercise the bug; got %d", want)
	}

	detail, err := f.Models.Admin.GetComplexDetail(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetComplexDetail over %d centavos of non-refunded revenue: %v", want, err)
	}

	if int64(detail.TotalRevenue) != want {
		t.Errorf("total_revenue must carry the full BIGINT sum; want %d, got %d", want, detail.TotalRevenue)
	}
}

// auditPlanSeedRows is how many audit rows the plan test inserts. At this size the
// planner's choice is unambiguous in both directions: with idx_audit_log_created_at
// it picks an index scan and drops the sort entirely, and without it there is no
// alternative but a seq scan plus a top-N sort.
const auditPlanSeedRows = 20_000

// TestListAuditLogsUsesTheCreatedAtIndex pins the plan for the unscoped audit-log
// page — the default superadmin view at GET /api/v1/admin/audit-log.
//
// audit_log shipped with (complex_id, created_at DESC) and (entity_type, entity_id).
// Neither leads with created_at, so with no complex_id filter nothing served the
// query: Postgres seq-scanned the table, hash-joined all of users and top-N sorted,
// measured at 4.03s over 500,000 rows against the 3s QueryContext budget. It crossed
// that budget at roughly 372,000 rows and 500'd from then on.
//
// The assertion is on the plan rather than on a wall-clock figure, because a timing
// threshold on shared CI hardware is a flake generator and says nothing about why the
// query got slow.
func TestListAuditLogsUsesTheCreatedAtIndex(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	// Rows hang off this fixture's complex so the fixture cleanup removes them, but
	// the query under test passes complex_id = NULL, so they are all in scope for the
	// unscoped plan regardless.
	_, err := f.Pool.Exec(ctx, `
		INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, created_at)
		SELECT $1, $2, 'update', 'booking', gen_random_uuid(), NOW() - (g * INTERVAL '1 second')
		FROM generate_series(1, $3) g`,
		f.UserID, f.ComplexID, auditPlanSeedRows)
	if err != nil {
		t.Fatalf("seeding audit rows: %v", err)
	}
	if _, err := f.Pool.Exec(ctx, `ANALYZE audit_log`); err != nil {
		t.Fatalf("analyzing audit_log: %v", err)
	}

	// EXPLAIN the constant the store itself issues, so this can never assert a plan
	// for a copy of the SQL that has drifted from the real one. Arguments mirror an
	// unfiltered first page: no complex, no entity type, no cursor.
	rows, err := f.Pool.Query(ctx, "EXPLAIN "+listAuditLogsSQL, nil, "", false, nil, nil, 21)
	if err != nil {
		t.Fatalf("explaining the audit-log query: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scanning the plan: %v", err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the plan: %v", err)
	}

	if !strings.Contains(plan.String(), "Index Scan using idx_audit_log_created_at on audit_log") {
		t.Errorf("the unscoped audit-log page must be served by idx_audit_log_created_at; plan was:\n%s", plan.String())
	}
	if strings.Contains(plan.String(), "Seq Scan on audit_log") {
		t.Errorf("the unscoped audit-log page must not seq-scan audit_log; plan was:\n%s", plan.String())
	}
}

// TestListAuditLogsKeepsTheComplexScopedPlan guards the other direction: the new
// index must not steal the complex-filtered query, which idx_audit_log_complex
// already serves in about a millisecond.
func TestListAuditLogsKeepsTheComplexScopedPlan(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	_, err := f.Pool.Exec(ctx, `
		INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, created_at)
		SELECT $1, $2, 'update', 'booking', gen_random_uuid(), NOW() - (g * INTERVAL '1 second')
		FROM generate_series(1, $3) g`,
		f.UserID, f.ComplexID, auditPlanSeedRows)
	if err != nil {
		t.Fatalf("seeding audit rows: %v", err)
	}
	if _, err := f.Pool.Exec(ctx, `ANALYZE audit_log`); err != nil {
		t.Fatalf("analyzing audit_log: %v", err)
	}

	complexID := f.ComplexID
	rows, err := f.Pool.Query(ctx, "EXPLAIN "+listAuditLogsSQL, complexID, "", false, nil, nil, 21)
	if err != nil {
		t.Fatalf("explaining the scoped audit-log query: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scanning the plan: %v", err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the plan: %v", err)
	}

	if strings.Contains(plan.String(), "Seq Scan on audit_log") {
		t.Errorf("the complex-scoped audit-log page must stay on an index; plan was:\n%s", plan.String())
	}
}
