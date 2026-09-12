// Package db owns the PostgreSQL connection pool: how it is built, tuned and
// instrumented. It is the only place outside the generated queries that names
// pgx, so the composition root wires a pool without importing a driver.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the connection pool the application holds. It is an alias rather
// than a wrapper: every store already takes pgx's own interfaces, and hiding
// the type would buy nothing but a translation layer.
type Pool = pgxpool.Pool

// PrepareConnFunc runs on every connection as it leaves the pool. It is a
// parameter of Config because what the application wants stamped on a
// connection — the tenant scope — is a domain rule, and this package holds
// none.
type PrepareConnFunc = func(ctx context.Context, conn *pgx.Conn) (bool, error)

// connectTimeout bounds opening the pool and the first ping: a database that
// is not answering must fail the boot rather than hang it.
const connectTimeout = 5 * time.Second

// maxConnLifetime recycles a connection this often regardless of use, so a
// rolling database restart or a proxy that quietly drops long-lived
// connections cannot leave the pool full of dead ones.
const maxConnLifetime = 5 * time.Minute

// Config is what Open needs to build the pool.
type Config struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  time.Duration
	// StatementTimeout is set as a runtime parameter on every connection, so
	// it applies to every query on it. It is deliberately looser than the
	// stores' own budgets: those are the normal path, and this only has to
	// catch the case where none of them managed to cancel anything.
	StatementTimeout time.Duration
	// SlowQueryThreshold arms the query tracer. Zero (or a nil Logger)
	// leaves the pool untraced.
	SlowQueryThreshold time.Duration
	// PrepareConn is pgxpool's own hook, passed straight through.
	PrepareConn PrepareConnFunc
	Logger      *slog.Logger
}

// Open builds the pool, verifies it with a ping, and returns it ready to use.
func Open(ctx context.Context, cfg Config) (*Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: unable to parse database DSN: %w", err)
	}

	//nolint:gosec // G115: db-max-open-conns is an operator-supplied CLI flag/env var (default 25), never derived
	// from request input; realistic values are far below int32 range.
	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	//nolint:gosec // G115: db-max-idle-conns is an operator-supplied CLI flag/env var (default 10), never derived
	// from request input; realistic values are far below int32 range.
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = maxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxIdleTime
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	// Stamp the tenant scope on every connection as it leaves the pool, from
	// the context of whoever is borrowing it. This is what gives the tenant
	// policies something to compare against on the queries that are not
	// inside a transaction — see internal/data/tenant.go for the whole
	// mechanism and for why a transaction repeats it with SET LOCAL.
	//
	// PrepareConn rather than the deprecated BeforeAcquire: it can tell a dead
	// connection (destroyed, query retried on another) from a statement that
	// failed on a live one (kept, query fails). See data.StampTenantScope.
	poolConfig.PrepareConn = cfg.PrepareConn

	if cfg.StatementTimeout > 0 {
		if poolConfig.ConnConfig.RuntimeParams == nil {
			poolConfig.ConnConfig.RuntimeParams = map[string]string{}
		}
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] =
			strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	}

	if tracer := NewSlowQueryTracer(cfg.SlowQueryThreshold, cfg.Logger); tracer != nil {
		poolConfig.ConnConfig.Tracer = tracer
	}

	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("db: unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: unable to ping database: %w", err)
	}

	return pool, nil
}
