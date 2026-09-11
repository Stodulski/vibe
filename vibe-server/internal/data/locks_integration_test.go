//go:build integration

package data

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newLockModel returns a lock store over pool whose log output the test can read.
func newLockModel(pool *pgxpool.Pool) (*LockModel, *bytes.Buffer) {
	logs := &bytes.Buffer{}
	return &LockModel{DB: NewDB(pool), Logger: slog.New(slog.NewTextHandler(logs, nil))}, logs
}

// lockKey returns a key no other test can collide with, and deletes whatever
// rows the test leaves behind on it.
func lockKey(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()

	key := name + ":" + uuid.NewString()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM job_locks WHERE key = $1`, key); err != nil {
			t.Errorf("cleaning up lock %q: %v", key, err)
		}
	})
	return key
}

// poolOf opens a pool with exactly maxConns connections, which is how a test
// makes the connection budget observable: a lock that pins a connection cannot
// hide inside a large pool.
func poolOf(t *testing.T, maxConns int32) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing DSN: %v", err)
	}
	config.MaxConns = maxConns
	config.MinConns = 0

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging database: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// leaseExpiry reads back when a lock's lease runs out.
func leaseExpiry(t *testing.T, pool *pgxpool.Pool, key string) time.Time {
	t.Helper()

	var expires time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT expires_at FROM job_locks WHERE key = $1`, key).Scan(&expires)
	if err != nil {
		t.Fatalf("reading the lease on %q: %v", key, err)
	}
	return expires
}

// FINDING 4. The lock must not hold a pooled connection while its caller works.
//
// It used to: a session advisory lock has to be taken and released on the same
// connection, so TryAdvisory checked one out and kept it until release. Four
// cron jobs and every inbound webhook call MercadoPago while holding that lock,
// each needing a second connection to do their own work, so the pool — 25
// connections in production — was the real limit on how many payment
// notifications the service could take: about 1.9 a second, past which every
// tenant's ordinary requests failed to get a connection.
//
// One connection and three simultaneous locks is that arithmetic in miniature:
// it cannot pass while a lock costs a connection, and the number of locks a pool
// can hold at once is now unbounded.
func TestALockCostsNoConnectionWhileItIsHeld(t *testing.T) {
	pool := poolOf(t, 1)
	locks, _ := newLockModel(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := range 3 {
		key := lockKey(t, pool, "concurrent")

		acquired, release, err := locks.TryAdvisory(ctx, key)
		if err != nil {
			t.Fatalf("taking lock %d of 3 on a single-connection pool: %v", i+1, err)
		}
		if !acquired {
			t.Fatalf("a freshly generated key must be free; lock %d was refused", i+1)
		}
		defer release()

		if held := pool.Stat().AcquiredConns(); held != 0 {
			t.Fatalf("holding %d lock(s) pins %d pooled connection(s); a lock must pin none", i+1, held)
		}
	}
}

// The lock still has to be a lock: two instances must not both believe they hold
// the same key, or three of them send the same reminder three times.
func TestOnlyOneCallerHoldsALockAtATime(t *testing.T) {
	pool := setupTestDB(t)
	locks, _ := newLockModel(pool)
	key := lockKey(t, pool, "exclusive")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	acquired, release, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("taking the lock: %v", err)
	}
	if !acquired {
		t.Fatal("a freshly generated key must be free")
	}

	second, releaseSecond, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("the second attempt errored rather than being refused: %v", err)
	}
	defer releaseSecond()
	if second {
		t.Error("two callers hold the same lock at once")
	}

	release()

	third, releaseThird, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("the third attempt: %v", err)
	}
	defer releaseThird()
	if !third {
		t.Error("the lock was not free after being released, so every later run of that job is skipped")
	}
}

// The release runs on a context detached from the caller's.
//
// This is the property a naive bound would break: a lock whose caller has
// already given up — which is every timed-out cron job — would never be
// released, and every attempt on that key would be refused until its lease
// lapsed.
func TestTheLockIsReleasedAfterItsCallerIsCancelled(t *testing.T) {
	pool := setupTestDB(t)
	locks, logs := newLockModel(pool)
	key := lockKey(t, pool, "release-after-cancel")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	acquired, release, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		cancel()
		t.Fatalf("taking the lock: %v", err)
	}
	if !acquired {
		cancel()
		t.Fatal("a freshly generated key must be free")
	}

	// The caller gives up before the deferred release runs — a cron job that hit
	// its jobTimeout, which is precisely when the lock must still be freed.
	cancel()
	release()

	free, releaseFree, err := locks.TryAdvisory(context.Background(), key)
	if err != nil {
		t.Fatalf("re-taking the lock: %v", err)
	}
	defer releaseFree()
	if !free {
		t.Error("the lock was not released once its caller's context was done")
	}

	if logs.Len() != 0 {
		t.Errorf("a successful release must report nothing; got %s", logs.String())
	}
}

// A lease outlives its caller's deadline, and not by much.
//
// Both halves matter. Shorter than the deadline and the lock would be handed to
// a second worker while the first is still calling MercadoPago; much longer and
// a crashed holder would block its key for minutes after its work had stopped.
func TestALeaseIsBoundedByItsCallersDeadline(t *testing.T) {
	pool := setupTestDB(t)
	locks, _ := newLockModel(pool)
	key := lockKey(t, pool, "lease-window")

	const budget = 2 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	acquired, release, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("taking the lock: %v", err)
	}
	defer release()
	if !acquired {
		t.Fatal("a freshly generated key must be free")
	}

	lease := time.Until(leaseExpiry(t, pool, key))
	if lease <= budget {
		t.Errorf("the lease (%v) expires before its caller's deadline (%v), so a second worker "+
			"can take the lock while the first is still working", lease, budget)
	}
	if lease > budget+leaseGrace+5*time.Second {
		t.Errorf("the lease (%v) outlives its caller's deadline (%v) by more than the grace (%v)",
			lease, budget, leaseGrace)
	}
}

// A holder that died leaves its lease behind, and the next attempt takes it over
// once that lease has lapsed. This is what a session lock got for free and a
// lease has to earn.
func TestAnExpiredLeaseIsTakenOver(t *testing.T) {
	pool := setupTestDB(t)
	locks, _ := newLockModel(pool)
	key := lockKey(t, pool, "expired-lease")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	acquired, _, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("taking the lock: %v", err)
	}
	if !acquired {
		t.Fatal("a freshly generated key must be free")
	}
	// The holder is gone without releasing: the process was redeployed, OOM
	// killed, or its context died mid-flight.

	if _, err := pool.Exec(ctx,
		`UPDATE job_locks SET expires_at = NOW() - INTERVAL '1 second' WHERE key = $1`, key); err != nil {
		t.Fatalf("expiring the lease: %v", err)
	}

	free, release, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("re-taking the lock: %v", err)
	}
	defer release()
	if !free {
		t.Error("an expired lease was never taken over, so this key is locked forever")
	}
}

// The previous holder must not be able to delete the lock somebody else now
// holds — which is what an unscoped delete would do the moment a lease lapsed
// and a late release ran.
func TestALateReleaseCannotFreeTheNewHoldersLock(t *testing.T) {
	pool := setupTestDB(t)
	locks, _ := newLockModel(pool)
	key := lockKey(t, pool, "late-release")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, staleRelease, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("taking the lock: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE job_locks SET expires_at = NOW() - INTERVAL '1 second' WHERE key = $1`, key); err != nil {
		t.Fatalf("expiring the lease: %v", err)
	}

	taken, release, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("taking over the lock: %v", err)
	}
	defer release()
	if !taken {
		t.Fatal("the expired lease was not taken over")
	}

	// The first holder finally gets round to its deferred release.
	staleRelease()

	stillHeld, releaseThird, err := locks.TryAdvisory(ctx, key)
	if err != nil {
		t.Fatalf("probing the lock: %v", err)
	}
	defer releaseThird()
	if stillHeld {
		t.Error("a stale release freed the current holder's lock, so two workers can now run the same job")
	}
}

// Lock keys include one per MercadoPago payment, so the set of keys is
// unbounded and a crash leaves a row nothing would otherwise delete. Ordinary
// traffic has to clear them, or this table repeats the growth defect it was
// meant to avoid.
func TestALongAbandonedLeaseIsSweptAwayByOrdinaryTraffic(t *testing.T) {
	pool := setupTestDB(t)
	locks, _ := newLockModel(pool)
	abandoned := lockKey(t, pool, "abandoned")
	unrelated := lockKey(t, pool, "unrelated")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, _, err := locks.TryAdvisory(ctx, abandoned); err != nil {
		t.Fatalf("taking the abandoned lock: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE job_locks SET expires_at = NOW() - $2::interval WHERE key = $1`,
		abandoned, (abandonedLeaseRetention + time.Hour).String()); err != nil {
		t.Fatalf("ageing the abandoned lease: %v", err)
	}

	// Any other lock, released normally, does the sweeping.
	_, release, err := locks.TryAdvisory(ctx, unrelated)
	if err != nil {
		t.Fatalf("taking the unrelated lock: %v", err)
	}
	release()

	var remaining int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM job_locks WHERE key = $1`, abandoned).Scan(&remaining); err != nil {
		t.Fatalf("counting abandoned leases: %v", err)
	}
	if remaining != 0 {
		t.Error("a lease abandoned long ago is never deleted, so the lock table grows without bound")
	}
}
