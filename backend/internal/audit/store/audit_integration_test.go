//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// The audit trail used to be empty in every environment that ran the real
// pool: a []byte bound to a JSONB parameter under QueryExecModeExec is typed as
// bytea, Postgres refuses to read it as json, and Record only logged the
// failure. This pins the insert against that same exec mode with the two
// shapes every caller produces — a payload and no payload at all.
//
// datatest.Isolated already connects with QueryExecModeExec (fixture.go,
// connectForTx), the same mode cmd/api's pool uses, so this reproduces the
// defect without a pool of its own.
func TestInsertAuditLogPersistsUnderQueryExecModeExec(t *testing.T) {
	f := datatest.Isolated(t)
	model := &auditstore.Store{DB: f.DB}
	ctx := context.Background()

	action := "integration-audit-" + uuid.NewString()

	entityID := uuid.New()
	if err := model.InsertAuditLog(ctx, nil, nil, action, "booking", &entityID, nil, []byte(`{"refund_status":"full"}`), "127.0.0.1"); err != nil {
		t.Fatalf("insert with a payload and no old value: %v", err)
	}
	if err := model.InsertAuditLog(ctx, nil, nil, action, "user", nil, nil, nil, ""); err != nil {
		t.Fatalf("insert with no payloads at all: %v", err)
	}

	var rows int
	var status *string
	err := f.DB.QueryRow(ctx, `
		SELECT COUNT(*), MAX(new_value->>'refund_status')
		FROM audit_log WHERE action = $1`, action).Scan(&rows, &status)
	if err != nil {
		t.Fatalf("reading audit rows back: %v", err)
	}
	if rows != 2 {
		t.Fatalf("expected both audit rows to be persisted, found %d", rows)
	}
	if status == nil || *status != "full" {
		t.Fatalf("expected new_value to round-trip as jsonb, got %v", status)
	}
}
