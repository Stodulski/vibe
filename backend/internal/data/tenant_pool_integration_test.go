//go:build integration

package data_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stodulski/vibe-server/internal/data"
)

// TestADeadPooledConnectionIsReplacedRatherThanReported pins the PrepareConn
// contract StampTenantScope relies on. A connection the server has dropped
// while it sat idle in the pool must not surface as an error to the next
// query: the hook answers (false, nil), the pool destroys the connection and
// retries on a fresh one. Before this, the hook answered (true, err) for a
// dead socket too, and every connection retired under a checkout became a 500.
func TestADeadPooledConnectionIsReplacedRatherThanReported(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing DATABASE_URL: %v", err)
	}
	// One connection, so the query below can only be served by the one that
	// was killed or by its replacement.
	config.MaxConns = 1
	config.MinConns = 0
	config.PrepareConn = data.StampTenantScope
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	t.Cleanup(pool.Close)

	var pid int
	if err := pool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("reading the pooled connection's backend pid: %v", err)
	}

	// Drop that backend from a separate connection while the pooled one is
	// idle: the pool has no way of knowing until it hands it out.
	killer, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("opening the terminating connection: %v", err)
	}
	defer func() { _ = killer.Close(ctx) }()
	if _, err := killer.Exec(ctx, `SELECT pg_terminate_backend($1)`, pid); err != nil {
		t.Fatalf("terminating backend %d: %v", pid, err)
	}

	var replacement int
	if err := pool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&replacement); err != nil {
		t.Fatalf("the query after the dead connection failed instead of being retried on a new one: %v", err)
	}
	if replacement == pid {
		t.Fatalf("expected a fresh backend after %d was terminated, got the same pid", pid)
	}
}
