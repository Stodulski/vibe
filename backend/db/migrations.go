// Package db carries the migration chain into the API binary.
//
// The embed declaration has to live in this directory and nowhere else,
// because an embed directive cannot name a path outside its own package
// directory: a file under cmd/api or internal/migrate can never reach
// db/migrations. This file exists only to be that carrier — the logic that
// applies the chain lives in internal/migrate.
//
// Nothing here reads or parses SQL. The files are shipped byte for byte, so
// the embedded chain and the one `goose -dir ./db/migrations` reads are the
// same bytes, which is what lets a database migrated by either continue with
// the other.
package db

import (
	"embed"
	"io/fs"
)

// migrationsFS holds every db/migrations/*.sql file as of the build. A file
// that is not on disk when the binary is compiled is not in the binary — the
// deployed image carries exactly the chain the code in it was written
// against, and TestEmbeddedChainMatchesDisk is what keeps a forgotten file
// from shipping.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations is the chain rooted at the .sql files themselves rather than at
// the repository, so a reader sees "001_init.sql" and not
// "migrations/001_init.sql". goose's provider walks the root of the
// filesystem it is handed, which is why the sub-tree and not migrationsFS is
// the exported value.
var Migrations = mustSub(migrationsFS, "migrations")

// mustSub panics rather than returning an error because the only way it can
// fail is a malformed embed path, which is a compile-time-constant mistake in
// this file: it would fail identically on every run of every build, so
// discovering it at init is discovering it as early as it can be discovered.
func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("db: embedded migrations are not rooted at " + dir + ": " + err.Error())
	}
	return sub
}
