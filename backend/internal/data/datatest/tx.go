//go:build integration

package datatest

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// connectForTx opens one connection of its own — not a pooled one — for a
// transactional fixture to live in.
//
// A pool is the wrong shape here for a reason that is easy to miss: a pool
// hands out whichever connection is free, and a transaction lives on exactly
// one. A fixture that began its transaction on a pooled connection would see
// its own uncommitted rows only while it happened to be handed that same
// connection back, and nothing in pgxpool promises that. One connection, one
// transaction, one test.
func connectForTx(t *testing.T) *pgx.Conn {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing DSN: %v", err)
	}
	// The exec mode the API pool uses, for the reason SetupTestDB gives: the
	// default statement-cache mode knows column types the application's mode
	// has to infer, and it hid a real jsonb defect.
	config.DefaultQueryExecMode = pgx.QueryExecModeExec

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}

	// Runs after the rollback the caller registers, t.Cleanup being LIFO: the
	// transaction is undone first, and only then is the connection given back.
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		if err := conn.Close(closeCtx); err != nil {
			t.Errorf("closing the fixture connection: %v", err)
		}
	})

	return conn
}

// savepointRunner sends every single statement to tx wrapped in its own
// savepoint.
//
// The savepoint is what makes a rolled-back transaction usable as test
// isolation at all. PostgreSQL aborts a transaction the moment a statement in
// it fails: every statement after that answers 25P02 until the transaction
// ends. A great many tests here deliberately provoke a refusal — the whole of
// schema_constraints_integration_test.go is nothing else — and without a
// savepoint around each statement the first expected error would take the rest
// of the test with it, reporting a transaction-aborted error in place of the
// assertion that was about to pass.
//
// So each statement gets `SAVEPOINT`, and its failure gets `ROLLBACK TO`. The
// test still sees the real PgError, with its real SQLSTATE and constraint name;
// the transaction it is running in survives to be asserted against.
//
// Query is the exception: it returns rows the caller reads after this function
// has returned, so there is no moment at which the savepoint could be released
// without cutting the result set short. A failing Query therefore poisons the
// transaction as it would in production, which is the honest behaviour — a test
// that expects a failing read is asserting on Scan, which goes through QueryRow.
type savepointRunner struct {
	tx pgx.Tx
}

// Exec runs a statement inside its own savepoint, releasing it on success and
// rolling back to it on failure.
func (r savepointRunner) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	sp, err := r.tx.Begin(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}

	tag, err := sp.Exec(ctx, sql, args...)
	if err != nil {
		_ = sp.Rollback(ctx)
		return tag, err
	}
	return tag, sp.Commit(ctx)
}

// Query runs a statement straight on the transaction; see savepointRunner.
func (r savepointRunner) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return r.tx.Query(ctx, sql, args...) //nolint:sqlclosecheck // returned to the caller
}

// QueryRow defers to Scan, which is where the statement actually runs and so
// where the savepoint has to be.
func (r savepointRunner) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return savepointRow{r: r, ctx: ctx, sql: sql, args: args}
}

// savepointRow is one row read, run inside its own savepoint when it is scanned.
type savepointRow struct {
	r   savepointRunner
	ctx context.Context //nolint:containedctx // pgx.Row.Scan takes no context; the query is deferred to it
	sql string

	args []any
}

// Scan runs the statement and copies the row into dest.
//
// pgx.ErrNoRows is passed through without rolling the savepoint back: no row is
// a result, not a failure, and the transaction is perfectly healthy after one.
// Rolling back regardless would be harmless but would hide that distinction
// from anybody reading this.
func (row savepointRow) Scan(dest ...any) error {
	sp, err := row.r.tx.Begin(row.ctx)
	if err != nil {
		return err
	}

	scanErr := sp.QueryRow(row.ctx, row.sql, row.args...).Scan(dest...)
	if scanErr != nil && !isNoRows(scanErr) {
		_ = sp.Rollback(row.ctx)
		return scanErr
	}
	if err := sp.Commit(row.ctx); err != nil {
		return err
	}
	return scanErr
}

// isNoRows reports whether err is pgx saying the statement matched nothing.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// tenantStampSQL is Isolated's initial stamp and restampingTx.Commit's
// restamp: the same statement, on this fixture's tenant.
const tenantStampSQL = `SELECT set_config('app.complex_id', $1, true), set_config('app.bypass_tenant', 'off', true)`

// tenantPinnedTx wraps the fixture's outer transaction so DB.Begin's
// savepoint (a store's own transaction) comes back as a restampingTx.
type tenantPinnedTx struct {
	pgx.Tx
	complexID string
}

func (t tenantPinnedTx) Begin(ctx context.Context) (pgx.Tx, error) {
	sp, err := t.Tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return restampingTx{Tx: sp, complexID: t.complexID}, nil
}

// restampingTx is a store's own transaction (data.DB.Begin SET LOCALs its
// ctx's tenant here). RELEASE SAVEPOINT does not undo that — PostgreSQL keeps
// it until the enclosing transaction ends (tx_integration_test.go confirms
// this) — so a foreign-tenant store call would otherwise leak its tenant for
// the rest of the fixture. Commit restamps the fixture's own tenant right
// after; a rollback needs no fix, since PostgreSQL reverts the SET LOCAL with
// it. Nested Begins are untouched: their SET LOCAL must outlive this savepoint.
type restampingTx struct {
	pgx.Tx
	complexID string
}

func (t restampingTx) Commit(ctx context.Context) error {
	if err := t.Tx.Commit(ctx); err != nil {
		return err
	}
	_, err := t.Tx.Conn().Exec(ctx, tenantStampSQL, t.complexID)
	return err
}
