package complexes

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
	// redisMPOAuthAttemptKey prefixes every attempt this store holds. Like
	// auth.GoogleCodes' "gauth:", it sits behind platformredis.KeyPrefix(env),
	// so an attempt registered in staging cannot be spent in production.
	redisMPOAuthAttemptKey = "mpoauth:"
	// mpOAuthAttemptOpTimeout bounds one round trip: both operations sit in a
	// request an owner is waiting on.
	mpOAuthAttemptOpTimeout = 2 * time.Second
	// mpOAuthAttemptTTL is how long an owner has between starting the
	// MercadoPago authorization and coming back with its code.
	mpOAuthAttemptTTL = time.Hour
)

// ErrOAuthAttemptInvalid says a MercadoPago OAuth attempt is unknown, expired,
// already spent, or registered by someone else for somewhere else. One error
// for all of them: telling them apart would answer questions about attempts
// the caller never held.
var ErrOAuthAttemptInvalid = errors.New("mercadopago oauth attempt is unknown or expired")

// MPOAuthAttempts holds single-use, TTL-bound MercadoPago OAuth attempts (the
// PKCE verifier, and who started it for which venue) keyed by the OAuth
// state. It is the sibling of auth.GoogleCodes, under its own namespace.
//
// It lives server-side because the OAuth callback often lands in a different
// tab, browser or installed app than the one that started it, where browser
// storage holds nothing. Redis makes an attempt registered on one instance
// spendable on another, and GETDEL makes spending it atomic.
//
// The in-memory map is the whole store when no Redis is configured: correct on
// a single instance (local development, the test harness) and only there.
type MPOAuthAttempts struct {
	rdb    *redis.Client
	prefix string
	mem    sync.Map // string (state) → mpOAuthAttempt
}

type mpOAuthAttempt struct {
	payload []byte
	expires time.Time
}

// NewMPOAuthAttempts returns the store. A nil Redis client makes it purely
// in-memory. env becomes part of every key.
func NewMPOAuthAttempts(rdb *redis.Client, env string) *MPOAuthAttempts {
	return &MPOAuthAttempts{rdb: rdb, prefix: platformredis.KeyPrefix(env)}
}

// Store records payload under state for ttl, replacing whatever was there.
func (s *MPOAuthAttempts) Store(ctx context.Context, state string, payload []byte, ttl time.Duration) error {
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, mpOAuthAttemptOpTimeout)
		defer cancel()

		if err := s.rdb.Set(ctx, s.key(state), payload, ttl).Err(); err != nil {
			return fmt.Errorf("complexes: storing mercadopago oauth attempt: %w", err)
		}
		return nil
	}

	// Nothing else evicts an attempt that was registered and never spent, so
	// the register path sweeps them.
	now := time.Now()
	s.sweep(now)
	s.mem.Store(state, mpOAuthAttempt{payload: payload, expires: now.Add(ttl)})
	return nil
}

// Consume returns what state carries and spends it in the same operation. An
// unknown, expired or already-spent state is ErrOAuthAttemptInvalid.
func (s *MPOAuthAttempts) Consume(ctx context.Context, state string) ([]byte, error) {
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, mpOAuthAttemptOpTimeout)
		defer cancel()

		payload, err := s.rdb.GetDel(ctx, s.key(state)).Bytes()
		switch {
		case errors.Is(err, redis.Nil):
			return nil, ErrOAuthAttemptInvalid
		case err != nil:
			return nil, fmt.Errorf("complexes: consuming mercadopago oauth attempt: %w", err)
		}
		return payload, nil
	}

	entry, ok := s.mem.LoadAndDelete(state)
	if !ok {
		return nil, ErrOAuthAttemptInvalid
	}
	held, ok := entry.(mpOAuthAttempt)
	if !ok || time.Now().After(held.expires) {
		return nil, ErrOAuthAttemptInvalid
	}
	return held.payload, nil
}

func (s *MPOAuthAttempts) sweep(now time.Time) {
	s.mem.Range(func(key, value any) bool {
		if held, ok := value.(mpOAuthAttempt); !ok || now.After(held.expires) {
			s.mem.Delete(key)
		}
		return true
	})
}

func (s *MPOAuthAttempts) key(state string) string {
	return s.prefix + redisMPOAuthAttemptKey + state
}
