//go:build integration

// Package middleware_test holds the one test in this repository that talks to
// a real Redis server (RED-06).
//
// Everything else about Redis here is covered against miniredis, which speaks
// real RESP over a real socket and is faithful about keys, TTLs and pipelines.
// What it is not faithful about is Lua: miniredis reimplements EVAL in Go, so
// the token bucket below has never been executed by the thing that executes it
// in production. That matters for this particular script — it does arithmetic
// on fractional tokens and writes them back with tostring precisely because
// redis.call converts a Lua number to an integer, and a difference of opinion
// between the two implementations about that conversion is a rate limiter that
// either never refills or never limits.
//
// The file is an external test package on purpose: it drives the middleware
// through its exported surface, the way cmd/api wires it, so it cannot pass by
// reaching into a helper the real chain does not call.
package middleware_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/auth"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/middleware"
	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
)

const testJWTSecret = "integration-test-secret-key-32-bytes!"

// redisClient opens a client against the E2E stack's Redis, skipping when
// REDIS_URL is unset — the same shape as datatest.SetupTestDB's DATABASE_URL
// skip, so a developer without the stack up sees a skip rather than a failure.
func redisClient(t *testing.T) *platformredis.Client {
	t.Helper()

	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set, skipping the real-Redis integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rdb, err := platformredis.Open(ctx, platformredis.Config{URL: url})
	if err != nil {
		t.Fatalf("connecting to REDIS_URL: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// countingUsers is the database behind the cache, counting the reads the cache
// is supposed to prevent.
type countingUsers struct {
	user  *authstore.User
	reads atomic.Int64
}

func (c *countingUsers) GetByID(context.Context, uuid.UUID) (*authstore.User, error) {
	c.reads.Add(1)
	copied := *c.user
	return &copied, nil
}

type noComplexes struct{}

func (noComplexes) GetByID(context.Context, uuid.UUID) (*complexstore.Complex, error) {
	return nil, data.ErrRecordNotFound
}

type allowAll struct{}

func (allowAll) IsBlacklisted(context.Context, string, uuid.UUID, time.Time) bool { return false }

// newChain builds a Middleware over a real Redis, in its own environment
// namespace so that two runs — or this file and the rest of the suite — cannot
// read each other's keys.
func newChain(t *testing.T, rdb *platformredis.Client, users middleware.UserReader, cfg middleware.Config) *middleware.Middleware {
	t.Helper()

	cfg.Env = "itest-" + uuid.NewString()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	shutdown := make(chan struct{})
	t.Cleanup(func() { close(shutdown) })

	return middleware.New(middleware.Dependencies{
		Users:     users,
		Complexes: noComplexes{},
		Tokens:    auth.NewTokenService(auth.TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"}),
		Blacklist: allowAll{},
		Redis:     rdb,
		Respond:   httpx.NewResponder(logger),
		Logger:    logger,
		Shutdown:  shutdown,
	}, cfg)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

// TestTheTokenBucketLuaRunsOnARealServer is the script's first execution by a
// real Redis. It asserts the two halves of a token bucket separately, because
// a broken script usually gets exactly one of them right: the burst is spent
// and then refused, and a wait refills it.
func TestTheTokenBucketLuaRunsOnARealServer(t *testing.T) {
	rdb := redisClient(t)

	const burst = 3
	chain := newChain(t, rdb, &countingUsers{user: activeUser()}, middleware.Config{
		RateLimitEnabled: true,
		// One token a second, so the refill below is a wait a test can afford
		// and still long enough that a bucket which never refills is visibly
		// different from one that does.
		RateLimitRPS:   1,
		RateLimitBurst: burst,
	})
	handler := chain.RateLimit(okHandler())

	// A fresh address per run: the rate-limit keys are not namespaced by
	// environment yet (that prefix belongs to another change), so the bucket
	// has to be made unique by its other half.
	addr := fmt.Sprintf("10.%d.%d.%d:5000", uuid.New().ID()%200+1, uuid.New().ID()%250, uuid.New().ID()%250)

	send := func() int {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/complexes", nil)
		r.RemoteAddr = addr
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	for i := range burst {
		if code := send(); code != http.StatusOK {
			t.Fatalf("request %d of the burst got %d, want 200; the script is refusing tokens the bucket holds", i+1, code)
		}
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Fatalf("the request past the burst got %d, want 429; the script is not spending tokens", code)
	}

	// The refill is the half a Lua/Go disagreement about number conversion
	// breaks: a script that truncates its fractional tokens to zero never
	// climbs back to one and the address stays limited forever.
	time.Sleep(1100 * time.Millisecond)
	if code := send(); code != http.StatusOK {
		t.Errorf("after waiting out one token the request got %d, want 200; the bucket never refilled", code)
	}
}

// TestTheUserCacheRoundTripsThroughARealServer covers the other Redis path
// that only miniredis had seen. The record is a struct this package encodes
// itself, so what is being tested is that it survives a real server and its
// own schema check — and that a stale entry is genuinely evicted rather than
// merely overwritten on the next read.
func TestTheUserCacheRoundTripsThroughARealServer(t *testing.T) {
	rdb := redisClient(t)

	users := &countingUsers{user: activeUser()}
	chain := newChain(t, rdb, users, middleware.Config{})

	var seen atomic.Value
	handler := chain.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, ok := httpx.ContextGetAuthenticatedUser(r); ok && user != nil {
			seen.Store(*user)
		}
		w.WriteHeader(http.StatusOK)
	}))

	tokens := auth.NewTokenService(auth.TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	token, err := tokens.GenerateAccessToken(users.user.ID, users.user.Role)
	if err != nil {
		t.Fatalf("minting an access token: %v", err)
	}

	send := func() {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/complexes", nil)
		r.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("authenticated request got %d, want 200", w.Code)
		}
	}

	send()
	if users.reads.Load() != 1 {
		t.Fatalf("the first request made %d database reads, want 1", users.reads.Load())
	}
	first, _ := seen.Load().(authstore.User)
	if len(first.PasswordHash) == 0 {
		t.Error("the account reaching the handler has no password hash; the record is lossy, which is the 500 this cache caused once")
	}

	send()
	if got := users.reads.Load(); got != 1 {
		t.Errorf("the second request made the database read again (%d total); the cache did not survive a real round trip", got)
	}
	second, _ := seen.Load().(authstore.User)
	if second.ID != first.ID || second.Email != first.Email || string(second.PasswordHash) != string(first.PasswordHash) {
		t.Errorf("the cached account differs from the one the database returned:\n db:    %+v\n cache: %+v", first, second)
	}

	chain.InvalidateUser(t.Context(), users.user.ID)
	send()
	if got := users.reads.Load(); got != 2 {
		t.Errorf("after InvalidateUser the database was read %d times in total, want 2; a revoked account stays usable", got)
	}
}

func activeUser() *authstore.User {
	return &authstore.User{
		ID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", LastName: "Perez",
		PasswordHash: []byte("$2a$04$abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRS"),
		Role:         "owner", IsActive: true, EmailVerified: true,
	}
}
