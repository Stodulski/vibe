package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LockStore provides cross-instance locks, which are how two instances agree
// that only one of them handles a given piece of work.
type LockStore interface {
	// TryAdvisory attempts to take the lock named by key. It returns whether
	// the lock was taken and a release function that is safe to call in a
	// defer whether or not it was.
	TryAdvisory(ctx context.Context, key string) (acquired bool, release func(), err error)
}

// leaseGrace is how much longer a lease lives than the context that took it.
//
// It absorbs the gap between the caller's deadline passing and its deferred
// release actually running — a scheduling delay, a slow final write — so the
// lock is not handed to somebody else while the previous holder is still
// unwinding. It is not a licence to overrun: every caller's work is bounded by
// that same context, so a holder that is still doing anything after its deadline
// is a holder whose calls are already failing.
const leaseGrace = 30 * time.Second

// defaultLeaseTTL bounds a lease taken with no deadline on its context.
//
// Nothing in this codebase does that today — the cron runs carry jobTimeout and
// the webhook worker carries webhookWorkTimeout — so this is the value for a
// future caller that forgets. It matches the scheduler's job budget, the longest
// of the deadlines that do exist, so forgetting is never worse than the worst
// honest case.
const defaultLeaseTTL = 2 * time.Minute

// abandonedLeaseRetention is how long a lease row outlives its expiry before any
// release sweeps it away.
//
// A lease whose holder died is harmless from the moment it expires: the next
// attempt on that key takes it over in the same statement. What is left is a
// dead row, and 'mp_webhook:<payment id>' keys are unbounded in number, so
// without this the table would accumulate one row per crash forever. A day is
// far past any expiry this package can produce, so the sweep can never touch a
// lease that still means something.
const abandonedLeaseRetention = 24 * time.Hour

// LockModel implements LockStore against PostgreSQL.
//
// It holds the pool and uses it the ordinary way — one statement, one checkout,
// connection straight back — which is the whole point of the design. It used to
// take a session advisory lock, which forced it to hold a pooled connection from
// acquire to release, so a lock taken around a MercadoPago call cost a
// connection for the length of that call. See job_locks in db/migrations/001_init.sql for the arithmetic
// that made that a tenant-visible outage.
type LockModel struct {
	DB *DB
	// Logger reports a failed release. It is the one place in this package that
	// logs, because it is the one place with an error no caller can be handed:
	// the release function is deferred and returns nothing, so an unreported
	// failure here is a lock nobody ever hears about.
	Logger *slog.Logger
}

// logger is the configured logger, or the default when none was given, so a
// LockModel built directly in a test still reports rather than panics.
func (m *LockModel) logger() *slog.Logger {
	if m.Logger == nil {
		return slog.Default()
	}
	return m.Logger
}

// leaseTTL is how long the lease for a call under ctx should live.
//
// Deriving it from the caller's own deadline is what keeps a lease honest: the
// work is bounded by that context, so by the time the lease lapses the holder
// has already stopped, whether it returned, timed out or died with the process.
// A fixed TTL could only be either too short — releasing a lock somebody is
// still using — or too long, blocking a key for minutes after a crash.
func leaseTTL(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return defaultLeaseTTL
	}
	return max(time.Until(deadline), 0) + leaseGrace
}

// TryAdvisory takes the lock for key if it is free.
//
// A caller that gets acquired=false must do nothing: another instance is
// already handling this work, or was until very recently. The release function
// is always non-nil, so the deferred call needs no guard.
//
// Free means "no live lease". The single statement below inserts a lease, or
// takes over an existing one whose expiry has passed; when the existing lease is
// still live the ON CONFLICT clause matches nothing, no row comes back, and the
// caller is refused. Doing it in one statement is what makes it atomic between
// instances — there is no window between reading the lease and writing it.
//
// The one case this is worse at than the session lock it replaced: a process
// that dies holding a key used to free it instantly, because the session went
// with it, and now the key stays taken until the lease lapses — a minute at
// most, given how the TTL is derived. What is behind the lock is unharmed by
// that wait. A cron run that is refused runs again on its next tick, and a
// webhook event whose worker died is reclaimed by the sweeper from its own
// 'processing' row, which takes longer than the lease does. A minute of a rare
// crash path is the price of the pool no longer being the ceiling on how many
// payment notifications the service can accept.
func (m *LockModel) TryAdvisory(ctx context.Context, key string) (bool, func(), error) {
	queryCtx, cancel := QueryContext(ctx)
	defer cancel()

	holder := uuid.New()

	var taken string
	err := m.DB.QueryRow(queryCtx, `
		INSERT INTO job_locks (key, holder, expires_at)
		VALUES ($1, $2, NOW() + $3::interval)
		ON CONFLICT (key) DO UPDATE
		   SET holder = EXCLUDED.holder,
		       acquired_at = NOW(),
		       expires_at = EXCLUDED.expires_at
		 WHERE job_locks.expires_at <= NOW()
		RETURNING key`,
		key, holder, leaseTTL(ctx).String(),
	).Scan(&taken)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// A live lease is held by somebody else.
		return false, func() {}, nil
	case err != nil:
		return false, func() {}, fmt.Errorf("taking the lock on %q: %w", key, err)
	}

	return true, func() { m.release(ctx, key, holder) }, nil
}

// release gives up a lease, and sweeps away any lease abandoned long ago.
//
// The second clause is not tidiness for its own sake. Lock keys include a
// 'mp_webhook:<payment id>' per payment, so the set of keys is unbounded, and a
// process that dies mid-lock leaves a row nothing else would ever delete. Doing
// it here means the table is kept small by the same traffic that fills it,
// rather than by a cleanup job somebody has to remember to schedule.
func (m *LockModel) release(ctx context.Context, key string, holder uuid.UUID) {
	// Detached from the caller's cancellation but not from the clock. The
	// detachment is deliberate: the caller's context may already be cancelled by
	// the time the deferred release runs — that is precisely the case of a cron
	// job that hit its budget — and a lease left in place would refuse every
	// attempt on this key until it expired. The deadline is what stops a wedged
	// PostgreSQL turning the cleanup into a goroutine blocked forever. See
	// DetachedQueryContext.
	releaseCtx, cancel := DetachedQueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(releaseCtx, `
		DELETE FROM job_locks
		 WHERE (key = $1 AND holder = $2)
		    OR expires_at < NOW() - $3::interval`,
		key, holder, abandonedLeaseRetention.String())
	if err != nil {
		// Reported rather than swallowed. It is not fatal — the lease expires on
		// its own, and everything behind these locks is retried — but it is the
		// only warning that this key will refuse every attempt until then.
		m.logger().Error("data: releasing the job lock failed", "key", key, "error", err)
	}
}
