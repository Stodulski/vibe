package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestUpRefusesAnEmptyDSN pins the one configuration mistake that has no
// database to report it: DB_AUTO_MIGRATE turned on with neither
// DB_MIGRATOR_URL nor DATABASE_URL set. It must be an error and not a
// successful no-op, or startup would continue against a schema nobody checked.
func TestUpRefusesAnEmptyDSN(t *testing.T) {
	_, err := Up(context.Background(), "", nil)
	if !errors.Is(err, ErrNoDSN) {
		t.Errorf("Up with no DSN: got %v, want ErrNoDSN", err)
	}
}

// TestHighestVersionMatchesTheDirectoryOnDisk checks the number the
// integration test compares the database against against a second reading of
// the same chain — the directory in the working tree rather than the copy in
// the binary. No literal appears here on purpose: the chain grows with every
// migration, and a hard-coded 36 would either need editing on every change
// (a number nobody reads) or would break the day the chain is squashed to a
// single file.
func TestHighestVersionMatchesTheDirectoryOnDisk(t *testing.T) {
	got, err := HighestVersion()
	if err != nil {
		t.Fatalf("HighestVersion: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatalf("reading db/migrations from disk: %v", err)
	}
	var want int64
	var counted int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		counted++
		v, err := strconv.ParseInt(strings.SplitN(e.Name(), "_", 2)[0], 10, 64)
		if err != nil {
			t.Fatalf("db/migrations/%s does not start with a version number: %v", e.Name(), err)
		}
		if v > want {
			want = v
		}
	}
	if counted == 0 {
		t.Fatal("found no .sql files in db/migrations — the comparison would have proved nothing")
	}
	if got != want {
		t.Errorf("HighestVersion = %d, but db/migrations tops out at %d", got, want)
	}
}
