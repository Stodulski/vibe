//go:build integration

// Package datatest_test proves the transactional harness, not a store.
package datatest_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/datatest"
)

func must(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}
func dsnOrSkip(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	return dsn
}
func scratchTable(t *testing.T, f *datatest.Fixture, ddl string) string {
	t.Helper()
	table := "tx_harness_" + uuid.New().String()[:8]
	_, err := f.DB.Exec(context.Background(), "CREATE TABLE "+table+" "+ddl)
	must(t, err, "creating scratch table")
	return table
}

// TestSavepointHarnessBehavior is findings 1 and 2: a refusal and ErrNoRows
// must leave the fixture usable, and SET LOCAL must land on the fixture's own tenant even under a foreign one.
func TestSavepointHarnessBehavior(t *testing.T) {
	f := datatest.Isolated(t)
	bg := context.Background()
	table := scratchTable(t, f, "(n int CHECK (n > 0))")
	_, err := f.DB.Exec(bg, `INSERT INTO `+table+` (n) VALUES (-1)`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("want a CHECK violation (23514), got %v", err)
	}
	var n int
	if err := f.DB.QueryRow(bg, `SELECT n FROM `+table+` WHERE false`).Scan(&n); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("want pgx.ErrNoRows, got %v", err)
	}
	_, err = f.DB.Exec(bg, `INSERT INTO `+table+` (n) VALUES (1)`)
	must(t, err, "statement after two refusals")
	if err := f.DB.QueryRow(bg, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil || n != 1 {
		t.Errorf("want 1 row surviving both refusals, got %d, err %v", n, err)
	}
	assertTenant := func(want string) {
		t.Helper()
		var got string
		must(t, f.DB.QueryRow(bg, `SELECT current_setting('app.complex_id', true)`).Scan(&got), "reading app.complex_id")
		if got != want {
			t.Errorf("want app.complex_id = %q, got %q", want, got)
		}
	}
	ownTx, err := f.DB.Begin(f.Scoped(bg))
	must(t, err, "DB.Begin (own tenant)")
	must(t, ownTx.Commit(bg), "commit (own tenant)")
	assertTenant(f.ComplexID.String())
	foreignTx, err := f.DB.Begin(data.ContextWithTenant(bg, uuid.New()))
	must(t, err, "DB.Begin (foreign tenant)")
	must(t, foreignTx.Commit(bg), "commit (foreign tenant)")
	assertTenant(f.ComplexID.String())
}
func queryExists(t *testing.T, dsn, sql string, args ...any) bool {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	must(t, err, "opening a second connection")
	defer func() { _ = conn.Close(ctx) }()
	var exists bool
	must(t, conn.QueryRow(ctx, sql, args...).Scan(&exists), "checking visibility")
	return exists
}

// TestFixtureTransactionBoundary: a committed savepoint is visible to the
// fixture, invisible elsewhere, and undone by the outer rollback — checked after the subtest's t.Cleanup runs.
func TestFixtureTransactionBoundary(t *testing.T) {
	dsn := dsnOrSkip(t)
	var table string
	t.Run("isolated_savepoint_commit_stays_private", func(t *testing.T) {
		f := datatest.Isolated(t)
		ctx := context.Background()
		table = scratchTable(t, f, "(n int)")
		tx, err := f.DB.Begin(ctx)
		must(t, err, "DB.Begin")
		_, err = tx.Exec(ctx, `INSERT INTO `+table+` (n) VALUES (1)`)
		must(t, err, "insert inside the savepoint")
		must(t, tx.Commit(ctx), "committing the savepoint")
		var n int
		if err := f.DB.QueryRow(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil || n != 1 {
			t.Errorf("want the commit visible to the fixture; got %d rows, err %v", n, err)
		}
		if queryExists(t, dsn, `SELECT to_regclass($1) IS NOT NULL`, table) {
			t.Error("want the uncommitted table invisible to a second connection")
		}
	})
	if queryExists(t, dsn, `SELECT to_regclass($1) IS NOT NULL`, table) {
		t.Error("want the table gone after the outer rollback")
	}
	t.Run("shared_writes_are_visible_to_a_second_connection", func(t *testing.T) {
		f := datatest.Shared(t)
		if !queryExists(t, dsn, `SELECT EXISTS (SELECT 1 FROM complexes WHERE id = $1)`, f.ComplexID) {
			t.Error("want Shared's seeded complex visible to a second connection")
		}
	})
}
