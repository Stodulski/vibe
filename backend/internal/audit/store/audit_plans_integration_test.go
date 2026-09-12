//go:build integration

package store_test

import (
	"context"
	"strings"
	"testing"

	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// auditPlanSeedRows is how many audit rows the plan test inserts. At this size the
// planner's choice is unambiguous in both directions: with idx_audit_log_created_at
// it picks an index scan and drops the sort entirely, and without it there is no
// alternative but a seq scan plus a top-N sort.
const auditPlanSeedRows = 20_000

// The audit-log page query is measured here rather than asserted on: both of
// these EXPLAIN the exact statement the store runs and fail if the planner
// stops using the index the trail depends on.
func TestListAuditLogsUsesTheCreatedAtIndex(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// Rows hang off this fixture's complex so the fixture cleanup removes them, but
	// the query under test passes complex_id = NULL, so they are all in scope for the
	// unscoped plan regardless.
	_, err := f.DB.Exec(ctx, `
		INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, created_at)
		SELECT $1, $2, 'update', 'booking', gen_random_uuid(), NOW() - (g * INTERVAL '1 second')
		FROM generate_series(1, $3) g`,
		f.UserID, f.ComplexID, auditPlanSeedRows)
	if err != nil {
		t.Fatalf("seeding audit rows: %v", err)
	}
	if _, err := f.DB.Exec(ctx, `ANALYZE audit_log`); err != nil {
		t.Fatalf("analyzing audit_log: %v", err)
	}

	// EXPLAIN the constant the store itself issues, so this can never assert a plan
	// for a copy of the SQL that has drifted from the real one. Arguments mirror an
	// unfiltered first page: no complex, no entity type, no cursor.
	rows, err := f.DB.Query(ctx, "EXPLAIN "+auditstore.ListAuditLogsSQLForTest, nil, "", false, nil, nil, 21)
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
	f := datatest.Isolated(t)
	ctx := context.Background()

	_, err := f.DB.Exec(ctx, `
		INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, created_at)
		SELECT $1, $2, 'update', 'booking', gen_random_uuid(), NOW() - (g * INTERVAL '1 second')
		FROM generate_series(1, $3) g`,
		f.UserID, f.ComplexID, auditPlanSeedRows)
	if err != nil {
		t.Fatalf("seeding audit rows: %v", err)
	}
	if _, err := f.DB.Exec(ctx, `ANALYZE audit_log`); err != nil {
		t.Fatalf("analyzing audit_log: %v", err)
	}

	complexID := f.ComplexID
	rows, err := f.DB.Query(ctx, "EXPLAIN "+auditstore.ListAuditLogsSQLForTest, complexID, "", false, nil, nil, 21)
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
