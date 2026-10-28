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
	// redisMPOAuthIndexKey prefixes each scope's list of live states, newest
	// first, which is what caps how many attempts a scope holds.
	redisMPOAuthIndexKey = "mpoauthidx:"
	// mpOAuthAttemptOpTimeout bounds one round trip: both operations sit in a
	// request an owner is waiting on.
	mpOAuthAttemptOpTimeout = 2 * time.Second
	// mpOAuthAttemptTTL is how long an owner has between starting the
	// MercadoPago authorization and coming back with its code.
	mpOAuthAttemptTTL = time.Hour
	// mpOAuthAttemptsPerOwner is how many attempts one owner can hold for one
	// venue at a time. A few, not one: each open tab prepares its own.
	mpOAuthAttemptsPerOwner = 5
)

// ErrOAuthAttemptInvalid says a MercadoPago OAuth attempt is unknown, expired,
// already spent, or registered by someone else for somewhere else. One error
// for all of them: telling them apart would answer questions about attempts
// the caller never held.
var ErrOAuthAttemptInvalid = errors.New("mercadopago oauth attempt is unknown or expired")

// MPOAuthAttempts holds single-use, TTL-bound MercadoPago OAuth attempts (the
// PKCE verifier, and who started it for which venue) keyed by a scope (the
// venue and the user, see mpOAuthAttemptScope) and the OAuth state. It is the
// sibling of auth.GoogleCodes, under its own namespace.
//
// It lives server-side because the OAuth callback often lands in a different
// tab, browser or installed app than the one that started it, where browser
// storage holds nothing. Redis makes an attempt registered on one instance
// spendable on another, and GETDEL makes spending it atomic.
//
// The scope is part of the key, so a request from anyone else naming a state
// looks up a key that does not exist and leaves the owner's attempt alone.
// Each scope keeps at most mpOAuthAttemptsPerOwner attempts: registering one
// more drops the oldest, so an owner cannot pile attempts up.
//
// The in-memory maps are the whole store when no Redis is configured: correct
// on a single instance (local development, the test harness) and only there.
type MPOAuthAttempts struct {
	rdb    *redis.Client
	prefix string

	mu      sync.Mutex
	entries map[string]mpOAuthAttempt // key → attempt
	order   map[string][]string       // scope → states, newest first
}

type mpOAuthAttempt struct {
	payload []byte
	expires time.Time
}

// NewMPOAuthAttempts returns the store. A nil Redis client makes it purely
// in-memory. env becomes part of every key.
func NewMPOAuthAttempts(rdb *redis.Client, env string) *MPOAuthAttempts {
	return &MPOAuthAttempts{
		rdb:     rdb,
		prefix:  platformredis.KeyPrefix(env),
		entries: map[string]mpOAuthAttempt{},
		order:   map[string][]string{},
	}
}

// Store records payload under scope and state for ttl, replacing whatever was
// there, and drops the scope's oldest attempts beyond mpOAuthAttemptsPerOwner.
func (s *MPOAuthAttempts) Store(ctx context.Context, scope, state string, payload []byte, ttl time.Duration) error {
	if s.rdb != nil {
		return s.storeRedis(ctx, scope, state, payload, ttl)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Nothing else evicts an attempt that was registered and never spent, so
	// the register path sweeps them.
	now := time.Now()
	s.sweep(now)
	s.entries[s.key(scope, state)] = mpOAuthAttempt{payload: payload, expires: now.Add(ttl)}
	states := append([]string{state}, without(s.order[scope], state)...)
	for _, dropped := range states[min(len(states), mpOAuthAttemptsPerOwner):] {
		delete(s.entries, s.key(scope, dropped))
	}
	s.order[scope] = states[:min(len(states), mpOAuthAttemptsPerOwner)]
	return nil
}

func (s *MPOAuthAttempts) storeRedis(ctx context.Context, scope, state string, payload []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, mpOAuthAttemptOpTimeout)
	defer cancel()

	index := s.indexKey(scope)
	var dropped *redis.StringSliceCmd
	_, err := s.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Set(ctx, s.key(scope, state), payload, ttl)
		p.LRem(ctx, index, 0, state)
		p.LPush(ctx, index, state)
		dropped = p.LRange(ctx, index, mpOAuthAttemptsPerOwner, -1)
		p.LTrim(ctx, index, 0, mpOAuthAttemptsPerOwner-1)
		// Every attempt in the index was stored with ttl, so the index can go
		// when the newest of them does.
		p.Expire(ctx, index, ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("complexes: storing mercadopago oauth attempt: %w", err)
	}
	if states := dropped.Val(); len(states) > 0 {
		keys := make([]string, len(states))
		for i, old := range states {
			keys[i] = s.key(scope, old)
		}
		if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("complexes: dropping old mercadopago oauth attempts: %w", err)
		}
	}
	return nil
}

// Consume returns what scope and state carry and spends it in the same
// operation. An unknown, expired or already-spent attempt is
// ErrOAuthAttemptInvalid.
func (s *MPOAuthAttempts) Consume(ctx context.Context, scope, state string) ([]byte, error) {
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, mpOAuthAttemptOpTimeout)
		defer cancel()

		payload, err := s.rdb.GetDel(ctx, s.key(scope, state)).Bytes()
		switch {
		case errors.Is(err, redis.Nil):
			return nil, ErrOAuthAttemptInvalid
		case err != nil:
			return nil, fmt.Errorf("complexes: consuming mercadopago oauth attempt: %w", err)
		}
		// The index only bounds how many attempts a scope holds; a stale entry
		// in it costs nothing, so failing to trim it is not the owner's problem.
		_ = s.rdb.LRem(ctx, s.indexKey(scope), 0, state).Err()
		return payload, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key(scope, state)
	held, ok := s.entries[key]
	delete(s.entries, key)
	s.order[scope] = without(s.order[scope], state)
	if len(s.order[scope]) == 0 {
		delete(s.order, scope)
	}
	if !ok || time.Now().After(held.expires) {
		return nil, ErrOAuthAttemptInvalid
	}
	return held.payload, nil
}

// sweep drops expired attempts. The caller holds s.mu.
func (s *MPOAuthAttempts) sweep(now time.Time) {
	for key, held := range s.entries {
		if now.After(held.expires) {
			delete(s.entries, key)
		}
	}
	for scope, states := range s.order {
		live := states[:0]
		for _, state := range states {
			if _, ok := s.entries[s.key(scope, state)]; ok {
				live = append(live, state)
			}
		}
		if len(live) == 0 {
			delete(s.order, scope)
		} else {
			s.order[scope] = live
		}
	}
}

func (s *MPOAuthAttempts) key(scope, state string) string {
	return s.prefix + redisMPOAuthAttemptKey + scope + ":" + state
}

func (s *MPOAuthAttempts) indexKey(scope string) string {
	return s.prefix + redisMPOAuthIndexKey + scope
}

func without(states []string, state string) []string {
	out := make([]string, 0, len(states))
	for _, s := range states {
		if s != state {
			out = append(out, s)
		}
	}
	return out
}
