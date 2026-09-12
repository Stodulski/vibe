package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
)

const (
	// redisTokenKey prefixes the per-token revocation keys.
	//
	// It is a suffix of the real key: every key this type writes is built on
	// platformredis.KeyPrefix(env) as well, so two deployments sharing one
	// Redis cannot revoke each other's sessions (RED-01). A blacklist is
	// exactly the kind of thing that collision is worst on — the shared key
	// means a logout in staging silently signs a production user out, and
	// nothing anywhere reports it.
	redisTokenKey = "bl:token:"
	// redisUserKey prefixes the per-user "everything issued before this
	// instant is void" keys.
	redisUserKey = "bl:user:"
	// redisOpTimeout bounds every blacklist round trip. A revocation check sits
	// in the path of every authenticated request, so it may not hang on Redis.
	redisOpTimeout = 2 * time.Second
)

// TokenBlacklist revokes access tokens before they expire on their own.
//
// Access tokens are self-contained JWTs, so nothing can invalidate one by
// deleting a row; signing out, deleting an account, resetting a password and
// the response to a detected refresh-token theft all rest on this type.
//
// Redis, when configured, is what makes a revocation visible to every
// instance. The in-memory maps are not a test double: they are this instance's
// own record of what it was asked to revoke. They carry the whole blacklist
// when Redis is not configured, and they are what the blacklist falls back on
// when Redis is configured but failing — a write Redis rejects is still stored
// here, and a read Redis cannot answer is answered from here rather than by
// letting the token through.
//
// What the fallback cannot do: revocations performed by OTHER instances while
// Redis is down are not visible to this one. A session revoked on instance A
// during the outage stays live on instance B until the token expires on its
// own. The degraded answer is strictly better than treating every revoked
// token as valid, but it is not the full protection — which is why every
// fallback is logged at error level and reported to Sentry.
type TokenBlacklist struct {
	rdb    *redis.Client
	logger *slog.Logger
	// prefix namespaces every key by application and environment.
	prefix            string
	tokens            sync.Map // [32]byte → time.Time (token expiry)
	userInvalidatedAt sync.Map // uuid.UUID → time.Time (cutoff)
}

// NewTokenBlacklist returns a blacklist. A nil Redis client makes it purely
// in-memory, which revokes correctly for a single instance and for tests.
//
// env is the deployment's environment, which becomes part of every key.
func NewTokenBlacklist(rdb *redis.Client, logger *slog.Logger, env string) *TokenBlacklist {
	return &TokenBlacklist{rdb: rdb, logger: logger, prefix: platformredis.KeyPrefix(env)}
}

// BlacklistToken revokes a single access token until it naturally expires.
//
// A Redis failure is recorded in memory and returned: the revocation still
// holds on this instance, and the caller learns it was not shared.
func (b *TokenBlacklist) BlacklistToken(ctx context.Context, rawToken string, expiry time.Time) error {
	hash := sha256.Sum256([]byte(rawToken))

	if b.rdb != nil {
		ttl := time.Until(expiry)
		if ttl <= 0 {
			return nil // Already expired on its own; nothing left to revoke.
		}

		ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
		defer cancel()

		hexHash := hex.EncodeToString(hash[:])
		if err := b.rdb.Set(ctx, b.prefix+redisTokenKey+hexHash, "1", ttl).Err(); err != nil {
			b.tokens.Store(hash, expiry)
			b.degraded("failed to SET token", err)
			return fmt.Errorf("blacklist token: %w", err)
		}
		return nil
	}

	b.tokens.Store(hash, expiry)
	return nil
}

// InvalidateUserTokens voids every access token issued to the user before now.
//
// A Redis failure is recorded in memory and returned: the cutoff still holds
// on this instance, and the caller learns it was not shared.
func (b *TokenBlacklist) InvalidateUserTokens(ctx context.Context, userID uuid.UUID) error {
	cutoff := time.Now()

	if b.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
		defer cancel()

		err := b.rdb.Set(ctx, b.prefix+redisUserKey+userID.String(),
			cutoff.Format(time.RFC3339Nano), accessTokenExpiry+time.Minute).Err()
		if err != nil {
			b.userInvalidatedAt.Store(userID, cutoff)
			b.degraded("failed to SET user invalidation", err)
			return fmt.Errorf("blacklist invalidate user tokens: %w", err)
		}
		return nil
	}

	b.userInvalidatedAt.Store(userID, cutoff)
	return nil
}

// IsBlacklisted reports whether the token should be rejected.
func (b *TokenBlacklist) IsBlacklisted(ctx context.Context, rawToken string, userID uuid.UUID, issuedAt time.Time) bool {
	hash := sha256.Sum256([]byte(rawToken))

	if b.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
		defer cancel()

		hexHash := hex.EncodeToString(hash[:])

		// Pipeline both checks into a single round trip.
		pipe := b.rdb.Pipeline()
		existsCmd := pipe.Exists(ctx, b.prefix+redisTokenKey+hexHash)
		getCmd := pipe.Get(ctx, b.prefix+redisUserKey+userID.String())
		_, pipeErr := pipe.Exec(ctx)

		// redis.Nil is expected: it only means the user has no invalidation
		// cutoff recorded. Redis answered.
		if pipeErr == nil || errors.Is(pipeErr, redis.Nil) {
			if existsCmd.Val() > 0 {
				return true
			}
			if val, err := getCmd.Result(); err == nil {
				if cutoff, parseErr := time.Parse(time.RFC3339Nano, val); parseErr == nil && issuedAt.Before(cutoff) {
					return true
				}
			}
			// Redis answered "not revoked", but Redis only knows the writes it
			// accepted. A revocation it rejected during an earlier outage was
			// recorded in this instance's own maps and nowhere else, precisely
			// so this instance could keep enforcing it. Returning false here
			// would throw that away the moment Redis came back — the session a
			// user signed out of mid-outage would start working again, for the
			// rest of its access-token lifetime, with no further alarm because
			// degraded() already fired at write time. So fall through to the
			// maps below in both cases; they only ever hold revocations this
			// instance was actually asked for, and Cleanup evicts them once
			// every token they cover has expired.
		} else {
			// Redis is unreachable or broken. Returning false here would make
			// every revoked token valid again for the rest of its 15 minutes —
			// logout, account deletion, password reset and the response to a
			// detected token theft would all silently stop working. So fall
			// through to the in-memory maps instead: everything this instance
			// was asked to revoke, including the writes that failed against
			// this same broken Redis, is still there and still enforced.
			//
			// This is a degraded answer, not a correct one. Revocations
			// performed by other instances during the outage are not in these
			// maps and will not be found. It is strictly better than allowing
			// all of them.
			b.degraded("redis pipeline error", pipeErr)
		}
	}

	if _, ok := b.tokens.Load(hash); ok {
		return true
	}
	if val, ok := b.userInvalidatedAt.Load(userID); ok {
		if issuedAt.Before(val.(time.Time)) {
			return true
		}
	}
	return false
}

// Cleanup removes expired entries from the in-memory maps.
//
// It runs even when Redis is configured, because a Redis outage leaves failed
// writes behind in those maps and nothing else would ever evict them.
func (b *TokenBlacklist) Cleanup() {
	now := time.Now()

	b.tokens.Range(func(key, value any) bool {
		if now.After(value.(time.Time)) {
			b.tokens.Delete(key)
		}
		return true
	})

	b.userInvalidatedAt.Range(func(key, value any) bool {
		// A cutoff stays useful until every token that predates it has expired
		// anyway, which is one access-token lifetime after the cutoff itself.
		cutoff := value.(time.Time)
		if now.After(cutoff.Add(accessTokenExpiry)) {
			b.userInvalidatedAt.Delete(key)
		}
		return true
	})
}

// degraded reports a fall back to the in-memory blacklist.
//
// It is loud on two channels on purpose. The error log is for whoever is
// watching this instance; Sentry is so that a security control running in
// degraded mode raises an alert rather than quietly becoming the normal state
// of the system. A revocation that only half works is exactly the kind of
// thing nobody notices until it matters.
func (b *TokenBlacklist) degraded(what string, err error) {
	b.logger.Error("SECURITY: token blacklist degraded to in-memory — revocations from other instances are not enforced here",
		"what", what, "error", err)
	sentry.CaptureMessage(fmt.Sprintf(
		"TOKEN BLACKLIST DEGRADED (redis unavailable, revocations from other instances not enforced): %s: %v",
		what, err))
}
