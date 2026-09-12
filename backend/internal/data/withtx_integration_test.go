//go:build integration

package data_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
)

// txDB is the *data.DB every test in this file runs its scratch table through:
// an Isolated fixture's handle, whose Begin opens a savepoint on the
// fixture's own outer transaction (see NewDBOverTx) rather than a pool
// checkout. WithTx's commit/rollback contract is exactly the same either way,
// and the scratch table this file creates is undone by the fixture's own
// rollback, so scratchTable no longer needs to DROP it itself.
func txDB(t *testing.T) *data.DB {
	t.Helper()
	return datatest.Isolated(t).DB
}

// scratchTable gives one test a table of its own to write to, so the
// transaction boundary is the only thing under test and no fixture row has to
// be interpreted.
//
// Nothing here drops it: it lives inside the fixture's own outer transaction,
// and that transaction's rollback (t.Cleanup, registered by datatest.Isolated)
// takes the table with it — Postgres DDL is transactional, same as any other
// statement.
func scratchTable(t *testing.T, d *data.DB) string {
	t.Helper()

	name := "withtx_" + uuid.New().String()[:8]
	if _, err := d.Exec(t.Context(), `CREATE TABLE `+name+` (n int)`); err != nil {
		t.Fatalf("creating the scratch table: %v", err)
	}
	return name
}

func rowCount(t *testing.T, d *data.DB, table string) int {
	t.Helper()
	var n int
	if err := d.QueryRow(t.Context(), `SELECT COUNT(*)::int FROM `+table).Scan(&n); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	return n
}

func TestWithTxCommitsWhenTheFunctionSucceeds(t *testing.T) {
	d := txDB(t)
	table := scratchTable(t, d)

	err := d.WithTx(t.Context(), func(tx pgx.Tx, q *db.Queries) error {
		if q == nil {
			t.Error("WithTx must hand fn a *db.Queries bound to the same transaction")
		}
		_, err := tx.Exec(t.Context(), `INSERT INTO `+table+` (n) VALUES (1)`)
		return err
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := rowCount(t, d, table); got != 1 {
		t.Errorf("want the row committed; got %d rows", got)
	}
}

// The invariant the thirteen hand-written call sites each carried on their own.
func TestWithTxRollsBackWhenTheFunctionFails(t *testing.T) {
	d := txDB(t)
	table := scratchTable(t, d)

	sentinel := errors.New("the caller refused")
	err := d.WithTx(t.Context(), func(tx pgx.Tx, _ *db.Queries) error {
		if _, err := tx.Exec(t.Context(), `INSERT INTO `+table+` (n) VALUES (1)`); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want the function's own error back unwrapped; got %v", err)
	}

	if got := rowCount(t, d, table); got != 0 {
		t.Errorf("want the write rolled back; got %d rows", got)
	}
}

// A panic unwinding through WithTx must roll back and keep panicking: swallowing
// it would turn a programming error into a silent half-written transaction.
func TestWithTxRollsBackOnAPanic(t *testing.T) {
	d := txDB(t)
	table := scratchTable(t, d)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic must continue past WithTx")
			}
		}()
		//nolint:errcheck // the call panics; there is no error to check
		_ = d.WithTx(t.Context(), func(tx pgx.Tx, _ *db.Queries) error {
			if _, err := tx.Exec(t.Context(), `INSERT INTO `+table+` (n) VALUES (1)`); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			panic("something went wrong mid-transaction")
		})
	}()

	if got := rowCount(t, d, table); got != 0 {
		t.Errorf("want the write rolled back after the panic; got %d rows", got)
	}
}
