// Package migrate applies the migration chain embedded in the binary.
//
// It is the same chain, byte for byte, that `goose -dir ./db/migrations
// postgres "$DATABASE_URL" up` applies from the working tree, recorded in the
// same goose_db_version table by the same library at the same version. That is
// the whole compatibility claim: a database brought to version N by the CLI can
// be finished by the binary and the other way round, because neither one knows
// which of them wrote the rows it is reading.
//
// Why a package instead of a few lines in cmd/api: the migrator opens its own
// connection, under its own DSN, with the pool deliberately pinned to one
// connection, and none of that has anything to do with how the server opens the
// pool it serves from. Keeping them apart is also what lets the round trip be
// tested without booting an application.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	// pgx's database/sql driver, registered as "pgx". goose speaks
	// database/sql; the rest of the application speaks pgx natively through
	// pgxpool. Sharing the DSN string is the only thing the two have in common.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	vibedb "github.com/stodulski/vibe-server/db"
)

// ErrNoDSN is returned when neither a migrator DSN nor a fallback was
// configured. It is a distinct error because "migrate against nothing" is a
// configuration mistake, not a database failure, and the two want different
// messages at the call site.
var ErrNoDSN = errors.New("migrate: no database DSN configured")

// Result is what one Up did. Applied is empty on a database that was already
// current, which is the normal case on every deploy that ships no migration.
type Result struct {
	// VersionBefore and VersionAfter are the goose_db_version high-water
	// marks either side of the run. Equal values mean nothing was applied.
	VersionBefore int64
	VersionAfter  int64
	// Applied names the migrations this run put on, in the order it applied
	// them, as file names ("001_init.sql").
	Applied []string
}

// Line is one row of `status`: a migration in the embedded chain and whether
// this database has it.
type Line struct {
	Version   int64
	Source    string
	State     string
	AppliedAt time.Time
}

// Applied reports whether this database has the migration.
func (l Line) Applied() bool { return l.State == string(goose.StateApplied) }

// Up applies every pending migration and reports what moved.
//
// Each migration runs in its own transaction (goose opens one per file unless
// the file opts out), so a failure leaves the database at the last migration
// that committed rather than half-way through one. The error is returned, not
// logged and swallowed: the caller decides whether a failure is fatal, and for
// both callers in this repository it is.
//
// logger may be nil. When it is not, goose's own per-migration output is
// routed through it, so the deploy log carries a line per applied file in the
// same format as everything else the process logs.
func Up(ctx context.Context, dsn string, logger *slog.Logger) (Result, error) {
	var result Result

	err := withProvider(dsn, logger, func(provider *goose.Provider) error {
		before, err := provider.GetDBVersion(ctx)
		if err != nil {
			return fmt.Errorf("migrate: reading the current version: %w", err)
		}
		result.VersionBefore = before
		result.VersionAfter = before

		results, upErr := provider.Up(ctx)
		// results is populated even on failure: goose returns what it managed
		// to apply alongside the error. Reporting those is the difference
		// between "the deploy failed" and "the deploy failed after one file and
		// before the next".
		result.Applied = make([]string, 0, len(results))
		for _, r := range results {
			result.Applied = append(result.Applied, r.Source.Path)
		}
		if upErr != nil {
			return fmt.Errorf("migrate: applying migrations (database was at version %d, applied %d): %w",
				before, len(result.Applied), upErr)
		}

		after, err := provider.GetDBVersion(ctx)
		if err != nil {
			return fmt.Errorf("migrate: reading the version after migrating: %w", err)
		}
		result.VersionAfter = after
		return nil
	})

	return result, err
}

// DownTo rolls the database back to version, running each migration's Down in
// reverse order. DownTo(ctx, dsn, 0) undoes the whole chain.
//
// Nothing in the binary calls this: no flag reaches it and the server never
// runs it. It exists so that the round trip can be tested from zero on a real
// database — which is the only way to know the chain's Downs are the inverses
// they claim to be — and so that a recovery has a supported path that is not
// "hope the CLI is installed on the box". Reversing a migration destroys the
// data the column or table held, so treat it as such.
func DownTo(ctx context.Context, dsn string, version int64, logger *slog.Logger) error {
	return withProvider(dsn, logger, func(provider *goose.Provider) error {
		if _, err := provider.DownTo(ctx, version); err != nil {
			return fmt.Errorf("migrate: rolling back to version %d: %w", version, err)
		}
		return nil
	})
}

// Status reports the state of every migration in the embedded chain against
// this database, oldest first. It applies nothing.
func Status(ctx context.Context, dsn string) ([]Line, error) {
	var lines []Line

	err := withProvider(dsn, nil, func(provider *goose.Provider) error {
		statuses, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("migrate: reading migration status: %w", err)
		}
		lines = make([]Line, 0, len(statuses))
		for _, s := range statuses {
			lines = append(lines, Line{
				Version:   s.Source.Version,
				Source:    s.Source.Path,
				State:     string(s.State),
				AppliedAt: s.AppliedAt,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lines, nil
}

// HighestVersion is the version of the last migration in the embedded chain —
// the version a database is at once Up has nothing left to do. It reads the
// embedded filesystem only and touches no database.
func HighestVersion() (int64, error) {
	entries, err := fs.ReadDir(vibedb.Migrations, ".")
	if err != nil {
		return 0, fmt.Errorf("migrate: reading the embedded chain: %w", err)
	}

	var highest int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		v, err := goose.NumericComponent(e.Name())
		if err != nil {
			// Not a migration file name. goose ignores those too, so this
			// must not be an error, or a stray README would break the build.
			continue
		}
		if v > highest {
			highest = v
		}
	}
	if highest == 0 {
		return 0, errors.New("migrate: the embedded chain has no migrations")
	}
	return highest, nil
}

// withProvider opens the migrator connection, builds the provider over the
// embedded chain, hands it to fn, and closes the connection afterwards. Every
// exported operation in this package is one of these, and having one place
// that opens and closes is what keeps the three from drifting.
//
// Close's error is discarded deliberately: the process is finished with the
// connection either way, and returning it would replace the migration error
// the caller actually needs to read.
func withProvider(dsn string, logger *slog.Logger, fn func(*goose.Provider) error) error {
	db, err := open(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	provider, err := newProvider(db, logger)
	if err != nil {
		return err
	}
	return fn(provider)
}

// open dials the migrator DSN with a pool of exactly one connection.
//
// One connection, because the session lock newProvider takes is held by a
// PostgreSQL *session*: taken on one connection and released on another it is
// not a lock at all, and database/sql is free to hand out whichever connection
// it likes. It also costs nothing — migrations are strictly sequential.
func open(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, ErrNoDSN
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("migrate: opening the migrator connection: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

// newProvider builds the goose provider over the embedded chain.
//
// The defaults are deliberate and must stay identical to the CLI's, because
// the two write the same bookkeeping table:
//   - the table is goose_db_version (goose.DefaultTablename), unset here so it
//     cannot drift from the CLI's;
//   - out-of-order migrations stay refused, matching a CLI run without
//     --allow-missing. A version that appears below the high-water mark is a
//     branch that was merged late, and applying it silently is how two
//     databases end up with the same version number and different schemas.
func newProvider(db *sql.DB, logger *slog.Logger) (*goose.Provider, error) {
	// A session-scoped advisory lock, so that N application instances booting
	// at once with DB_AUTO_MIGRATE=true do not each try to apply the same
	// pending migration. The one that wins migrates; the others wait and then
	// find nothing to do. Without it the losers fail their startup on a
	// duplicate-object error, which on a rolling deploy is a crash loop.
	//
	// Note this does not coordinate with the goose CLI, which takes no lock:
	// it protects the case the embedded path creates (many replicas), not a
	// human running the CLI against a deploying service.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("migrate: building the migration lock: %w", err)
	}

	opts := []goose.ProviderOption{
		goose.WithSessionLocker(locker),
		// The application registers no Go migrations, and the global registry
		// is process-wide mutable state a test in another package could have
		// written to. Reading it here would make what this binary applies
		// depend on what else was linked into it.
		goose.WithDisableGlobalRegistry(true),
	}
	if logger != nil {
		opts = append(opts, goose.WithSlog(logger))
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, vibedb.Migrations, opts...)
	if err != nil {
		return nil, fmt.Errorf("migrate: building the migration provider: %w", err)
	}
	return provider, nil
}
