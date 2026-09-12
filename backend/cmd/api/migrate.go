package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/stodulski/vibe-server/internal/migrate"
	"github.com/stodulski/vibe-server/internal/platform/config"
)

// migrationTimeout bounds a whole migrate run: the wait for the advisory lock
// plus every statement in every pending migration.
//
// It exists because the alternative is a process that hangs forever at boot
// behind a lock somebody else is holding, which on Railway looks exactly like a
// deploy that is "still starting". Ten minutes is far past anything this chain
// does on a database of this size (a full 001..N replay on an empty database is
// seconds) and past goose's own five-minute ceiling on waiting for the lock, so
// hitting it means something is genuinely stuck rather than merely slow.
const migrationTimeout = 10 * time.Minute

// migratorDSN is the DSN migrations run as: DB_MIGRATOR_URL when set, the
// application's own DATABASE_URL otherwise.
//
// The two exist separately because they are not the same privilege. Migrations
// are DDL and must run as the role that owns the schema; the server itself
// wants a role that can only read and write rows, so that a defect in a handler
// cannot drop a table and so that row-level security actually applies to it —
// PostgreSQL exempts a table's owner from its own policies unless the table is
// FORCEd, and a DML-only role is the honest way to hold that line. Until those
// roles exist, DB_MIGRATOR_URL is unset and both are the same connection
// string, which is why the fallback is silent rather than a warning.
func migratorDSN(cfg config.Config) string {
	if cfg.DB.MigratorDSN != "" {
		return cfg.DB.MigratorDSN
	}
	return cfg.DB.DSN
}

// runMigrations applies the embedded chain and logs the version either side of
// it. The returned error is the caller's to make fatal; both call sites do.
func runMigrations(cfg config.Config, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	dsn := migratorDSN(cfg)
	logger.Info("running database migrations",
		"source", "embedded", "migrator_dsn_configured", cfg.DB.MigratorDSN != "")

	result, err := migrate.Up(ctx, dsn, logger)
	if err != nil {
		// result carries what did get applied before the failure; saying so
		// is the difference between "the deploy failed" and knowing which
		// migration to look at.
		logger.Error("database migrations failed",
			"error", err,
			"version_before", result.VersionBefore,
			"applied", result.Applied)
		return err
	}

	logger.Info("database migrations complete",
		"version_before", result.VersionBefore,
		"version_after", result.VersionAfter,
		"applied", result.Applied,
		"count", len(result.Applied))
	return nil
}

// logMigrationStatus writes one line per migration in the embedded chain. It is
// what -migrate-only prints after migrating, so the pre-deploy log answers
// "what does this database have" without a psql session.
func logMigrationStatus(cfg config.Config, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	lines, err := migrate.Status(ctx, migratorDSN(cfg))
	if err != nil {
		return err
	}

	pending := 0
	for _, line := range lines {
		if !line.Applied() {
			pending++
		}
		logger.Info("migration",
			"version", line.Version,
			"source", line.Source,
			"state", line.State,
			"applied_at", line.AppliedAt)
	}
	logger.Info("migration status", "total", len(lines), "pending", pending)
	return nil
}
