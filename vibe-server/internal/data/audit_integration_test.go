//go:build integration

package data

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// execModePool opens a pool the way cmd/api does: QueryExecModeExec, which
// never asks the server for parameter types and infers them from the Go
// values instead. The shared fixture uses pgx's default mode, under which
// the audit insert always worked — the failure only exists in the mode the
// application actually runs in, so this test has to open its own pool.
func execModePool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing DSN: %v", err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// The audit trail used to be empty in every environment that ran the real
// pool: a []byte bound to a JSONB parameter in exec mode is typed as bytea,
// Postgres refuses to read it as json, and Record only logged the failure.
// This pins the insert against the same exec mode with the two shapes every
// caller produces — a payload and no payload at all.
func TestInsertAuditLogPersistsUnderQueryExecModeExec(t *testing.T) {
	pool := execModePool(t)
	model := &AdminModel{DB: NewDB(pool)}
	ctx := context.Background()

	action := "integration-audit-" + uuid.NewString()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM audit_log WHERE action = $1`, action); err != nil {
			t.Errorf("cleaning up audit rows: %v", err)
		}
	})

	entityID := uuid.New()
	if err := model.InsertAuditLog(ctx, nil, nil, action, "booking", &entityID, nil, []byte(`{"refund_status":"full"}`), "127.0.0.1"); err != nil {
		t.Fatalf("insert with a payload and no old value: %v", err)
	}
	if err := model.InsertAuditLog(ctx, nil, nil, action, "user", nil, nil, nil, ""); err != nil {
		t.Fatalf("insert with no payloads at all: %v", err)
	}

	var rows int
	var status *string
	err := pool.QueryRow(ctx, `
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
