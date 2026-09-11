package migrate

import (
	"context"
	"os"
	"testing"
	"time"
)

// dsnEnv gates every test in this file.
//
// It is deliberately NOT DATABASE_URL and deliberately NOT the `integration`
// build tag. These tests roll a database back to version 0 and replay the
// whole chain, so pointing them at the shared E2E database would delete
// whatever another package was mid-way through. A separate variable makes
// that impossible by accident: `make test/integration` sets DATABASE_URL and
// this suite skips, and the only way to run it is to name a database on
// purpose.
//
//	psql "$ADMIN_DSN" -c 'CREATE DATABASE vibe_automigrate'
//	AUTOMIGRATE_TEST_DSN='postgres://.../vibe_automigrate?sslmode=disable' \
//	  go test ./internal/migrate/ -run TestUp -v
const dsnEnv = "AUTOMIGRATE_TEST_DSN"

// TestUpFromZeroReachesTheHighestEmbeddedVersion is the claim the whole
// feature rests on: a binary pointed at an empty database brings it to the
// schema the code in that binary was written against, and pointed at a current
// one changes nothing.
//
// Both halves matter. The first is what a fresh environment does. The second
// is what every deploy that ships no migration does, and a migrator that is
// not a no-op on that path is a migrator that reruns DDL on every restart.
func TestUpFromZeroReachesTheHighestEmbeddedVersion(t *testing.T) {
	dsn := requireDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	want, err := HighestVersion()
	if err != nil {
		t.Fatalf("HighestVersion: %v", err)
	}

	resetToZero(ctx, t, dsn)

	first, err := Up(ctx, dsn, nil)
	if err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if first.VersionBefore != 0 {
		t.Errorf("first Up started at version %d, want 0 — the reset did not empty the database", first.VersionBefore)
	}
	if first.VersionAfter != want {
		t.Errorf("first Up left the database at version %d, want %d (the last file in the embedded chain)",
			first.VersionAfter, want)
	}
	if len(first.Applied) == 0 {
		t.Error("first Up applied nothing from an empty database")
	}

	second, err := Up(ctx, dsn, nil)
	if err != nil {
		t.Fatalf("second Up: %v", err)
	}
	if len(second.Applied) != 0 {
		t.Errorf("second Up applied %v; running up on a current database must be a no-op", second.Applied)
	}
	if second.VersionBefore != want || second.VersionAfter != want {
		t.Errorf("second Up moved the version from %d to %d, want %d both sides",
			second.VersionBefore, second.VersionAfter, want)
	}
}

// TestStatusSeesEveryEmbeddedMigrationApplied checks the other half of what
// -migrate-only prints: after a full up, nothing in the chain is still
// pending. A version number alone cannot say that — goose's high-water mark
// would read the same with a gap below it.
func TestStatusSeesEveryEmbeddedMigrationApplied(t *testing.T) {
	dsn := requireDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if _, err := Up(ctx, dsn, nil); err != nil {
		t.Fatalf("Up: %v", err)
	}

	lines, err := Status(ctx, dsn)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("Status returned no lines — the assertion below would have proved nothing")
	}
	for _, line := range lines {
		if !line.Applied() {
			t.Errorf("migration %d (%s) is %s after a full up", line.Version, line.Source, line.State)
		}
	}
}

// TestUpFailsOnAnUnreachableDatabase is here because every other test in this
// file passes trivially if Up never reaches a database. An error that says
// nothing and an error that says the wrong thing both fail this.
func TestUpFailsOnAnUnreachableDatabase(t *testing.T) {
	requireDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := Up(ctx, "postgres://nobody:nobody@127.0.0.1:1/nothing?sslmode=disable&connect_timeout=5", nil)
	if err == nil {
		t.Fatal("Up against a database that does not exist returned no error")
	}
}

// resetToZero rolls the chain all the way back, so the test that follows
// starts from nothing. It uses the chain's own Down migrations rather than
// dropping the schema: the Downs are the inverses the repository claims they
// are, and running them is the only thing that checks it.
func resetToZero(ctx context.Context, t *testing.T, dsn string) {
	t.Helper()
	if err := DownTo(ctx, dsn, 0, nil); err != nil {
		t.Fatalf("rolling the chain back to 0: %v", err)
	}
}

func requireDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skipf("%s is not set; see the comment on dsnEnv for how to run this", dsnEnv)
	}
	return dsn
}
