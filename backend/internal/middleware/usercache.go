package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
)

const (
	// userCacheTTL is how long a cached account is served before the database
	// is consulted again.
	userCacheTTL = 10 * time.Minute
	// redisCacheTimeout bounds one cache operation.
	//
	// It was two seconds, which is longer than the database read the cache
	// exists to avoid: a slow Redis made every authenticated request slower
	// than having no cache at all, and a Redis that had stopped answering
	// added two seconds to every request in the process. A cache that cannot
	// answer in a quarter of a second has already failed at its job.
	redisCacheTimeout = 250 * time.Millisecond
	// userCacheSchema is stamped on every record. Bump it whenever the meaning
	// or the set of cached fields changes: entries written by the previous
	// build are then read as misses instead of being decoded into a shape they
	// no longer match.
	userCacheSchema = 1
	// cacheReportInterval is how often one failing cache operation earns a log
	// line. See cacheReports.
	cacheReportInterval = time.Minute
)

// The error policy for this cache, in one place, because it was three
// different policies:
//
//   - A cache failure never changes what the caller gets. A read that fails is
//     a miss and costs a database read; a write that fails costs the next
//     request the same. That part was already true and stays true.
//   - A cache failure is never silent. The read path was: redis.Nil, a
//     connection refused, a timeout and a record nothing can decode all
//     returned nil with nothing written anywhere. A Redis that has quietly
//     stopped answering is then a database taking the whole authenticated
//     request load it was given a cache to keep off it, and the only visible
//     symptom is latency nobody can attribute.
//   - A cache failure is not allowed to bury the log. It fails once per
//     request, so a line per occurrence is one error line per authenticated
//     request for as long as the incident lasts — which does not report the
//     incident, it hides it among thousands of copies of itself.
//
// So: an ordinary miss is silent because nothing happened, and everything else
// is reported at most once per operation per cacheReportInterval, carrying the
// number of occurrences that line stands for.
//
// InvalidateUser is the deliberate exception and reports every failure — see
// the comment there.

// cacheReports is the per-operation throttle behind reportCacheFailure.
type cacheReports struct {
	mu   sync.Mutex
	runs map[string]*cacheReportRun
}

// cacheReportRun is one operation's throttle state: when it may log again, and
// how many occurrences it has swallowed since it last did.
type cacheReportRun struct {
	openAt     time.Time
	suppressed int
}

func newCacheReports() *cacheReports {
	return &cacheReports{runs: make(map[string]*cacheReportRun)}
}

// admit reports whether op may write a line now, and how many occurrences were
// suppressed since it last wrote one.
func (c *cacheReports) admit(op string, now time.Time) (suppressed int, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	run, seen := c.runs[op]
	if !seen {
		c.runs[op] = &cacheReportRun{openAt: now.Add(cacheReportInterval)}
		return 0, true
	}

	if now.Before(run.openAt) {
		run.suppressed++
		return 0, false
	}

	suppressed, run.suppressed = run.suppressed, 0
	run.openAt = now.Add(cacheReportInterval)
	return suppressed, true
}

// reportCacheFailure records a cache operation that did not happen.
func (m *Middleware) reportCacheFailure(op string, err error) {
	suppressed, ok := m.cacheReports.admit(op, m.now())
	if !ok {
		return
	}

	m.logger.Error("user cache: "+op+" failed; the request was served without the cache",
		"error", err,
		"operation", op,
		"suppressed_since_last_line", suppressed,
		"report_interval", cacheReportInterval)
}

// cachedUser is the Redis representation of an authenticated account.
//
// It exists because the obvious thing — marshalling *authstore.User straight to
// JSON — is silently lossy in exactly the way that matters. authstore.User carries
// `json:"-"` on PasswordHash, LockedUntil, FailedLoginAttempts and
// LastFailedLogin, because that struct is also the API response body and none
// of those belong in it. Reusing it as the cache format inherited those tags,
// so the cached account came back with a nil password hash. bcrypt does not
// report a nil hash as a mismatch — it returns ErrHashTooShort — so the
// change-password endpoint answered 500, for every user, from their second
// request onwards, for as long as the cache stayed warm. Which is always.
//
// The rule this type encodes: the cache returns what the database would have
// returned, or it returns nothing. A field that is present on authstore.User and
// absent here is a field some handler will read as its zero value while
// believing it read the row — and the zero values are the dangerous answers.
// LockedUntil nil reads as "not locked out". IsActive false at least fails
// safe; FailedLoginAttempts zero does not. Today the lockout check reads the
// row directly and none of that bites, but "today" is doing all the work in
// that sentence.
//
// # Why the password hash is in Redis
//
// It is credential material, and putting it in a second datastore widens the
// blast radius of a Redis compromise. That is real, and it is the smaller of
// the two risks:
//
//   - The alternative on offer is not "the hash stays in Postgres". It is "the
//     hash is nil in the object handlers are handed", which is the live 500.
//     Serving a *authstore.User that is missing a field it declares is a lie the
//     type system cannot catch.
//   - This Redis already holds the token blacklist and the notification queue:
//     an attacker reading it can already revoke sessions, forge nothing, and
//     read every password-reset task in flight. It is not an untrusted store
//     that is being upgraded to a trusted one.
//   - What is stored is a bcrypt digest at cost 12, not a password. Offline
//     attack on it costs the same whether it was taken from Postgres or Redis.
//
// The honest alternative — cache an identity-only record and have the
// password-change path load the row itself — is better, and it is not
// available from inside this package: the handler reads the hash off the
// context user, in internal/auth. It is the follow-up this comment exists to
// justify handing over.
type cachedUser struct {
	Schema              int        `json:"schema"`
	ID                  uuid.UUID  `json:"id"`
	Email               string     `json:"email"`
	PasswordHash        []byte     `json:"password_hash"`
	FirstName           string     `json:"first_name"`
	LastName            string     `json:"last_name"`
	Phone               string     `json:"phone"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	EmailVerified       bool       `json:"email_verified"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	FailedLoginAttempts int        `json:"failed_login_attempts"`
	LockedUntil         *time.Time `json:"locked_until"`
	LastFailedLogin     *time.Time `json:"last_failed_login"`
}

// newCachedUser copies an account into its cache representation.
func newCachedUser(u *authstore.User) cachedUser {
	return cachedUser{
		Schema:              userCacheSchema,
		ID:                  u.ID,
		Email:               u.Email,
		PasswordHash:        u.PasswordHash,
		FirstName:           u.FirstName,
		LastName:            u.LastName,
		Phone:               u.Phone,
		Role:                u.Role,
		IsActive:            u.IsActive,
		EmailVerified:       u.EmailVerified,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
		FailedLoginAttempts: u.FailedLoginAttempts,
		LockedUntil:         u.LockedUntil,
		LastFailedLogin:     u.LastFailedLogin,
	}
}

// user rebuilds the account, or reports that the record cannot be trusted.
//
// A record from an older schema, or one whose credential material is missing,
// is treated as a miss rather than as an account: serving a half-decoded user
// is how this went wrong the first time, and a miss costs one database read.
func (c cachedUser) user() (*authstore.User, bool) {
	if c.Schema != userCacheSchema || c.ID == uuid.Nil || len(c.PasswordHash) == 0 {
		return nil, false
	}

	return &authstore.User{
		ID:                  c.ID,
		Email:               c.Email,
		PasswordHash:        c.PasswordHash,
		FirstName:           c.FirstName,
		LastName:            c.LastName,
		Phone:               c.Phone,
		Role:                c.Role,
		IsActive:            c.IsActive,
		EmailVerified:       c.EmailVerified,
		CreatedAt:           c.CreatedAt,
		UpdatedAt:           c.UpdatedAt,
		FailedLoginAttempts: c.FailedLoginAttempts,
		LockedUntil:         c.LockedUntil,
		LastFailedLogin:     c.LastFailedLogin,
	}, true
}

// userCacheKey is where an account's cached record lives.
//
// The environment prefix is not decoration: without it two deployments sharing
// one Redis serve each other's accounts, and the symptom is a user reading
// another environment's copy of their own row rather than an error anybody
// notices (RED-01).
func (m *Middleware) userCacheKey(id uuid.UUID) string {
	return platformredis.KeyPrefix(m.cfg.Env) + "cache:user:" + id.String()
}

// getCachedUser returns the cached account, or nil for a miss.
//
// Every path out of here is a miss the caller absorbs with a database read.
// What the paths do not share is whether anything went wrong: an expired entry
// is the cache working, and a Redis that refused the connection is the cache
// being gone. They were the same silent nil.
func (m *Middleware) getCachedUser(ctx context.Context, id uuid.UUID) *authstore.User {
	if m.rdb == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, redisCacheTimeout)
	defer cancel()

	raw, err := m.rdb.Get(ctx, m.userCacheKey(id)).Result()
	switch {
	case errors.Is(err, redis.Nil):
		// The entry expired, or this account has not been seen in ten
		// minutes. Nothing failed, so nothing is reported.
		return nil
	case err != nil:
		m.reportCacheFailure("GET", err)
		return nil
	}

	var record cachedUser
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		// Not the stale-format case: a record written by an older build still
		// decodes into this struct and is rejected below on its schema stamp.
		// Bytes that do not decode at all mean something other than this code
		// is writing to the key, which no redeploy fixes.
		m.reportCacheFailure("decode", err)
		return nil
	}

	user, ok := record.user()
	if !ok {
		// A record from a previous schema, or one with no credential material.
		// Expected across a deploy, self-healing on the next write, and it
		// would report once per request for ten minutes if it were an error.
		return nil
	}
	return user
}

// cacheUser stores an account for userCacheTTL.
func (m *Middleware) cacheUser(ctx context.Context, user *authstore.User) {
	if m.rdb == nil {
		return
	}
	raw, err := json.Marshal(newCachedUser(user))
	if err != nil {
		m.reportCacheFailure("encode", err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, redisCacheTimeout)
	defer cancel()
	if err := m.rdb.Set(ctx, m.userCacheKey(user.ID), raw, userCacheTTL).Err(); err != nil {
		m.reportCacheFailure("SET", err)
	}
}

// invalidateAttempts is how many times a failed delete is retried before the
// account is left to expire on its own.
const invalidateAttempts = 3

// InvalidateUser drops a user's cached record.
//
// The cache carries is_active and the role, so a stale entry keeps a
// deactivated account working and a demoted one privileged until it expires —
// ten minutes during which a revoked account is a live one.
//
// Two things make the delete more than a hopeful call.
//
// The context is detached from the request. It was the request's, and the
// callers are handlers that return immediately afterwards: a client that
// disconnects as it logs out cancelled the delete it had just asked for.
//
// A failed delete is retried, because a single DEL against a Redis that
// dropped one connection is not an answer. What it still cannot do is tell the
// caller: the InvalidateUser method on the cache interfaces in internal/auth
// and internal/admin returns nothing, so a caller has no way to refuse to
// deactivate an account it could not evict. Giving it an error return is the
// follow-up; it is a signature change across two packages this change does not
// own.
//
// This is the one cache operation whose failures are reported every time
// rather than through the throttle the read and write paths use. The two
// reasons the throttle exists do not apply: a revocation happens once per
// logout or deactivation rather than once per request, so there is no flood to
// contain, and the line names the account that stayed usable — collapsing
// twenty of those into "and nineteen more" would throw away the only part an
// operator needs, which is which nineteen.
func (m *Middleware) InvalidateUser(ctx context.Context, id uuid.UUID) {
	if m.rdb == nil {
		return
	}

	var err error
	for attempt := 1; attempt <= invalidateAttempts; attempt++ {
		err = m.deleteCachedUser(ctx, id)
		if err == nil {
			return
		}
	}

	m.logger.Error("user cache: DEL failed, a revoked account stays usable until the entry expires",
		"error", err, "user_id", id, "attempts", invalidateAttempts, "ttl", userCacheTTL)
}

// deleteCachedUser is one delete attempt, on a context that outlives the
// request that asked for it.
func (m *Middleware) deleteCachedUser(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), redisCacheTimeout)
	defer cancel()
	return m.rdb.Del(ctx, m.userCacheKey(id)).Err()
}
