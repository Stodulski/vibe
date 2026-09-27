package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
)

const (
	// redisGoogleCodeKey prefixes every entry this store holds.
	//
	// It is a suffix of the real key: every key this type writes carries
	// platformredis.KeyPrefix(env) in front of it as well, so an entry minted
	// in staging cannot be spent against production (RED-01).
	redisGoogleCodeKey = "gauth:"
	// googleCodeOpTimeout bounds one round trip. Both operations sit inside a
	// sign-in a person is waiting through, so neither may hang on Redis.
	googleCodeOpTimeout = 2 * time.Second
)

// GoogleCodes holds single-use, TTL-bound opaque values keyed by a random
// token. Today that is the OIDC authorization-code flow's {nonce, verifier}
// state (internal/auth/google_oidc.go), under its own key namespace.
//
// Redis is what makes an entry mintable on one instance and spendable on
// another, and — through GETDEL — what makes consuming it atomic, so two
// concurrent attempts to spend the same entry cannot both succeed.
//
// The in-memory map is not a test double: it is the whole store when no Redis
// is configured, which is a single-instance deployment and the local
// development stack. It is correct there and only there — on more than one
// instance an entry minted on A is unknown to B, and consuming it fails as if
// it had expired. That is why the composition root always hands this type the
// real client when there is one.
type GoogleCodes struct {
	rdb *redis.Client
	// prefix namespaces every key by application and environment.
	prefix string
	mem    sync.Map // string (code) → googleCode
}

// googleCode is one unspent code in the in-memory store: what it carries, and
// when it stops being worth anything.
type googleCode struct {
	payload []byte
	expires time.Time
}

// NewGoogleCodes returns the store. A nil Redis client makes it purely
// in-memory — see the type comment for what that costs.
//
// env is the deployment's environment, which becomes part of every key.
func NewGoogleCodes(rdb *redis.Client, env string) *GoogleCodes {
	return &GoogleCodes{rdb: rdb, prefix: platformredis.KeyPrefix(env)}
}

// Store records payload under code for ttl.
func (c *GoogleCodes) Store(ctx context.Context, code string, payload []byte, ttl time.Duration) error {
	if c.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, googleCodeOpTimeout)
		defer cancel()

		if err := c.rdb.Set(ctx, c.key(code), payload, ttl).Err(); err != nil {
			return fmt.Errorf("auth: storing google sign-in code: %w", err)
		}
		return nil
	}

	// Nothing else ever evicts an in-memory code that was minted and never
	// spent — the redirect that would have spent it is exactly the request
	// that failed — so the mint path is where they are swept.
	c.sweep(time.Now())
	c.mem.Store(code, googleCode{payload: payload, expires: time.Now().Add(ttl)})
	return nil
}

// Consume returns what code carries and spends it in the same operation, so a
// code is usable exactly once. An unknown, expired or already-spent code is
// ErrGoogleCodeInvalid — one error for all three, because telling them apart
// would answer questions about codes the caller never held.
func (c *GoogleCodes) Consume(ctx context.Context, code string) ([]byte, error) {
	if c.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, googleCodeOpTimeout)
		defer cancel()

		// GETDEL, not GET then DEL: two concurrent exchanges of one code have
		// to be decided by Redis, not by whichever of them read first.
		payload, err := c.rdb.GetDel(ctx, c.key(code)).Bytes()
		switch {
		case errors.Is(err, redis.Nil):
			return nil, ErrGoogleCodeInvalid
		case err != nil:
			return nil, fmt.Errorf("auth: consuming google sign-in code: %w", err)
		}
		return payload, nil
	}

	entry, ok := c.mem.LoadAndDelete(code)
	if !ok {
		return nil, ErrGoogleCodeInvalid
	}
	held, ok := entry.(googleCode)
	if !ok || time.Now().After(held.expires) {
		return nil, ErrGoogleCodeInvalid
	}
	return held.payload, nil
}

// sweep drops the in-memory codes that have expired.
func (c *GoogleCodes) sweep(now time.Time) {
	c.mem.Range(func(key, value any) bool {
		if held, ok := value.(googleCode); !ok || now.After(held.expires) {
			c.mem.Delete(key)
		}
		return true
	})
}

// key is the full Redis key for a code.
func (c *GoogleCodes) key(code string) string {
	return c.prefix + redisGoogleCodeKey + code
}
