package db

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// TestEmbeddedChainMatchesDisk is the guard on the one failure mode embedding
// introduces: db/migrations/ is the directory everybody edits, and the binary
// ships whatever was there at build time. A migration added to the repository
// but excluded from the embed pattern — a name the glob does not match, a
// file that is not committed — produces a binary whose `up` reports success
// having skipped it, and the schema silently drifts from the chain.
//
// The comparison is on names, not on a count: a count matches whenever two
// mistakes cancel out, and it cannot say which file is missing.
func TestEmbeddedChainMatchesDisk(t *testing.T) {
	onDisk := sqlNames(t, os.DirFS("migrations"))
	embedded := sqlNames(t, Migrations)

	for name := range onDisk {
		if !embedded[name] {
			t.Errorf("db/migrations/%s is on disk but not in the embedded chain: "+
				"a binary built from this tree would skip it", name)
		}
	}
	for name := range embedded {
		if !onDisk[name] {
			t.Errorf("%s is embedded but no longer on disk in db/migrations/", name)
		}
	}
	if len(onDisk) == 0 {
		t.Fatal("read no .sql files from db/migrations/ — the test compared two empty sets and proved nothing")
	}
	if len(onDisk) != len(embedded) {
		t.Errorf("db/migrations/ has %d .sql files, the embedded chain has %d", len(onDisk), len(embedded))
	}
}

// TestEmbeddedChainIsFlat states what the embed contract assumes: goose reads
// the root of the filesystem it is given, so a migration filed in a
// subdirectory is invisible to it. Today there are none; this fails the day
// somebody adds one, rather than letting it be skipped in production.
func TestEmbeddedChainIsFlat(t *testing.T) {
	entries, err := fs.ReadDir(Migrations, ".")
	if err != nil {
		t.Fatalf("reading the embedded chain: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("db/migrations/%s/ is a directory; goose only reads the root of the embedded "+
				"filesystem, so nothing inside it would ever be applied", e.Name())
		}
	}
}

// sqlNames returns the set of .sql file names at the root of fsys.
func sqlNames(t *testing.T, fsys fs.FS) map[string]bool {
	t.Helper()

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatalf("reading migration directory: %v", err)
	}

	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names[e.Name()] = true
	}
	return names
}
