package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// The tests below drive the Redis branch of TokenBlacklist — the only branch
// that runs in production, and the one every other blacklist test skips by
// passing a nil client.
//
// They run against miniredis rather than a container so that `make test`
// covers them with no external service: docker-compose.e2e.yml deliberately
// has no Redis, and the degradation being tested is a *connection* failure,
// which is reproduced faithfully by closing the server out from under the
// client. miniredis speaks the real RESP protocol over a real socket, so the
// pipeline, the key names, the TTLs and the RFC3339Nano encoding are all
// exercised for real; only the server implementation is a stand-in.

// newRedisBlacklist returns a blacklist wired to a live miniredis, plus the
// server so a test can kill it mid-flight.
func newRedisBlacklist(t *testing.T) (*TokenBlacklist, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		// Fail fast: these tests deliberately point the client at a dead
		// server, and the retry backoff would only slow that down.
		MaxRetries: -1,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewTokenBlacklist(rdb, slog.New(slog.NewTextHandler(io.Discard, nil))), mr
}

func tokenHash(raw string) [32]byte {
	return sha256.Sum256([]byte(raw))
}

// TestBlacklistRedisDown covers the defect this whole change exists for: when
// Redis breaks, a revoked token must stay revoked on this instance.
func TestBlacklistRedisDown(t *testing.T) {
	t.Run("revoked token is still rejected when the redis read fails", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		mr.Close() // Redis is gone for both the write and the read.

		if err := bl.BlacklistToken(t.Context(), "stolen-token", time.Now().Add(15*time.Minute)); err == nil {
			t.Fatal("expected BlacklistToken to report the redis failure")
		}

		if !bl.IsBlacklisted(t.Context(), "stolen-token", uuid.New(), time.Now()) {
			t.Error("expected a revoked token to stay revoked while redis is down, got allowed")
		}
	})

	t.Run("user invalidation still rejects old tokens when redis is down", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		mr.Close()

		userID := uuid.New()
		issuedAt := time.Now().Add(-5 * time.Minute)

		if err := bl.InvalidateUserTokens(t.Context(), userID); err == nil {
			t.Fatal("expected InvalidateUserTokens to report the redis failure")
		}

		if !bl.IsBlacklisted(t.Context(), "some-token", userID, issuedAt) {
			t.Error("expected a token issued before the cutoff to stay revoked while redis is down, got allowed")
		}
		if bl.IsBlacklisted(t.Context(), "fresh-token", userID, time.Now().Add(2*time.Second)) {
			t.Error("expected a token issued after the cutoff to be allowed even while degraded")
		}
	})

	t.Run("an unrevoked token is still allowed when the redis read fails", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		mr.Close()

		if bl.IsBlacklisted(t.Context(), "innocent-token", uuid.New(), time.Now()) {
			t.Error("degrading must not reject tokens nobody revoked")
		}
	})

	// This is the limit of the degradation, asserted so nobody mistakes it for
	// full protection: a revocation that reached Redis before the outage lives
	// only in Redis. Once Redis is unreachable this instance cannot see it —
	// and by the same token, cannot see what other instances revoke meanwhile.
	t.Run("a revocation that only reached redis is lost during the outage", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)

		if err := bl.BlacklistToken(t.Context(), "written-while-up", time.Now().Add(15*time.Minute)); err != nil {
			t.Fatalf("BlacklistToken while redis is up: %v", err)
		}
		if !bl.IsBlacklisted(t.Context(), "written-while-up", uuid.New(), time.Now()) {
			t.Fatal("expected the token to be revoked while redis is up")
		}

		mr.Close()

		if bl.IsBlacklisted(t.Context(), "written-while-up", uuid.New(), time.Now()) {
			t.Error("this assertion documents a known gap; if it now passes, the gap closed and this test should be rewritten")
		}
	})
}

// TestBlacklistWriteFailureLandsInMemory is what makes the degradation real: a
// read can only fall back to memory if the failed write went there.
func TestBlacklistWriteFailureLandsInMemory(t *testing.T) {
	t.Run("BlacklistToken stores the token locally", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		mr.Close()

		expiry := time.Now().Add(15 * time.Minute)
		if err := bl.BlacklistToken(t.Context(), "my-token", expiry); err == nil {
			t.Fatal("expected an error from the failed redis write")
		}

		stored, ok := bl.tokens.Load(tokenHash("my-token"))
		if !ok {
			t.Fatal("expected the failed redis write to be recorded in the in-memory map")
		}
		if !stored.(time.Time).Equal(expiry) {
			t.Errorf("stored expiry = %v, want %v", stored, expiry)
		}
	})

	t.Run("InvalidateUserTokens stores the cutoff locally", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		mr.Close()

		userID := uuid.New()
		before := time.Now()
		if err := bl.InvalidateUserTokens(t.Context(), userID); err == nil {
			t.Fatal("expected an error from the failed redis write")
		}
		after := time.Now()

		stored, ok := bl.userInvalidatedAt.Load(userID)
		if !ok {
			t.Fatal("expected the failed redis write to be recorded in the in-memory map")
		}
		cutoff := stored.(time.Time)
		if cutoff.Before(before) || cutoff.After(after) {
			t.Errorf("stored cutoff = %v, want between %v and %v", cutoff, before, after)
		}
	})

	t.Run("a successful redis write does not touch the in-memory map", func(t *testing.T) {
		bl, _ := newRedisBlacklist(t)

		if err := bl.BlacklistToken(t.Context(), "my-token", time.Now().Add(15*time.Minute)); err != nil {
			t.Fatalf("BlacklistToken: %v", err)
		}
		userID := uuid.New()
		if err := bl.InvalidateUserTokens(t.Context(), userID); err != nil {
			t.Fatalf("InvalidateUserTokens: %v", err)
		}

		if _, ok := bl.tokens.Load(tokenHash("my-token")); ok {
			t.Error("in-memory map should stay empty while redis accepts the write")
		}
		if _, ok := bl.userInvalidatedAt.Load(userID); ok {
			t.Error("in-memory map should stay empty while redis accepts the write")
		}
	})

	t.Run("an already-expired token is not written anywhere", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		mr.Close()

		if err := bl.BlacklistToken(t.Context(), "expired", time.Now().Add(-time.Second)); err != nil {
			t.Errorf("revoking an already-expired token is a no-op, got error: %v", err)
		}
		if _, ok := bl.tokens.Load(tokenHash("expired")); ok {
			t.Error("an already-expired token has nothing left to revoke and should not be stored")
		}
	})
}

// TestBlacklistRedisHappyPath pins the wire format: the key names other
// instances read, and the timestamp encoding IsBlacklisted has to parse back.
func TestBlacklistRedisHappyPath(t *testing.T) {
	t.Run("BlacklistToken writes the bl:token: key with a ttl", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)

		hash := tokenHash("my-token")
		expiry := time.Now().Add(15 * time.Minute)
		if err := bl.BlacklistToken(t.Context(), "my-token", expiry); err != nil {
			t.Fatalf("BlacklistToken: %v", err)
		}

		key := "bl:token:" + hex.EncodeToString(hash[:])
		if !mr.Exists(key) {
			t.Fatalf("expected redis key %q to exist; keys present: %v", key, mr.Keys())
		}
		if ttl := mr.TTL(key); ttl <= 0 || ttl > 15*time.Minute {
			t.Errorf("ttl = %v, want a positive value no greater than the token lifetime", ttl)
		}

		if !bl.IsBlacklisted(t.Context(), "my-token", uuid.New(), time.Now()) {
			t.Error("expected the token written to redis to read back as revoked")
		}
		if bl.IsBlacklisted(t.Context(), "another-token", uuid.New(), time.Now()) {
			t.Error("expected an unrelated token to be allowed")
		}
	})

	t.Run("InvalidateUserTokens writes an RFC3339Nano cutoff IsBlacklisted parses back", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		userID := uuid.New()

		before := time.Now()
		if err := bl.InvalidateUserTokens(t.Context(), userID); err != nil {
			t.Fatalf("InvalidateUserTokens: %v", err)
		}
		after := time.Now()

		key := "bl:user:" + userID.String()
		raw, err := mr.Get(key)
		if err != nil {
			t.Fatalf("expected redis key %q to exist: %v; keys present: %v", key, err, mr.Keys())
		}

		cutoff, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			t.Fatalf("stored cutoff %q is not RFC3339Nano — IsBlacklisted would silently fail to parse it: %v", raw, err)
		}
		if cutoff.Before(before.Truncate(time.Second)) || cutoff.After(after.Add(time.Second)) {
			t.Errorf("cutoff = %v, want around %v", cutoff, before)
		}
		if ttl := mr.TTL(key); ttl <= accessTokenExpiry {
			t.Errorf("ttl = %v, want longer than an access token's %v lifetime", ttl, accessTokenExpiry)
		}

		// Both directions of the comparison, through redis.
		if !bl.IsBlacklisted(t.Context(), "old-token", userID, cutoff.Add(-time.Second)) {
			t.Error("expected a token issued before the cutoff to be rejected")
		}
		if bl.IsBlacklisted(t.Context(), "new-token", userID, cutoff.Add(time.Second)) {
			t.Error("expected a token issued after the cutoff to be allowed")
		}
		if bl.IsBlacklisted(t.Context(), "other-user-token", uuid.New(), cutoff.Add(-time.Hour)) {
			t.Error("expected another user's token to be unaffected by this cutoff")
		}
	})

	t.Run("a clean redis with nothing revoked allows the token", func(t *testing.T) {
		bl, _ := newRedisBlacklist(t)

		if bl.IsBlacklisted(t.Context(), "fresh-token", uuid.New(), time.Now()) {
			t.Error("expected an untouched token to be allowed")
		}
	})
}

// TestBlacklistWriteFailureIsReported pins the third change: the caller finds
// out. Before this, Logout answered "successfully logged out" with the token
// still fully valid.
func TestBlacklistWriteFailureIsReported(t *testing.T) {
	bl, mr := newRedisBlacklist(t)

	if err := bl.BlacklistToken(t.Context(), "tok", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("BlacklistToken with redis up: %v", err)
	}
	if err := bl.InvalidateUserTokens(t.Context(), uuid.New()); err != nil {
		t.Fatalf("InvalidateUserTokens with redis up: %v", err)
	}

	mr.Close()

	if err := bl.BlacklistToken(t.Context(), "tok", time.Now().Add(time.Minute)); err == nil {
		t.Error("BlacklistToken swallowed a redis write failure")
	}
	if err := bl.InvalidateUserTokens(t.Context(), uuid.New()); err == nil {
		t.Error("InvalidateUserTokens swallowed a redis write failure")
	}
}

// TestBlacklistCleanup covers both maps. The pre-existing cleanup test only
// ever stored a token, so removing the userInvalidatedAt eviction entirely
// left the suite green.
func TestBlacklistCleanup(t *testing.T) {
	t.Run("evicts expired tokens and keeps live ones", func(t *testing.T) {
		bl := NewTokenBlacklist(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		// The in-memory path cannot fail, so these errors are always nil.
		_ = bl.BlacklistToken(t.Context(), "expired-token", time.Now().Add(-time.Second))
		_ = bl.BlacklistToken(t.Context(), "live-token", time.Now().Add(15*time.Minute))

		bl.Cleanup()

		if _, ok := bl.tokens.Load(tokenHash("expired-token")); ok {
			t.Error("expected the expired token to be evicted")
		}
		if _, ok := bl.tokens.Load(tokenHash("live-token")); !ok {
			t.Error("expected a token that has not expired yet to survive cleanup")
		}
	})

	t.Run("evicts user invalidations only once they can no longer matter", func(t *testing.T) {
		bl := NewTokenBlacklist(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

		// A cutoff stops mattering one access-token lifetime after itself:
		// by then every token it could reject has expired on its own.
		staleUser := uuid.New()
		staleCutoff := time.Now().Add(-accessTokenExpiry - time.Minute)
		bl.userInvalidatedAt.Store(staleUser, staleCutoff)

		// This one is older than nothing has expired yet, so evicting it would
		// resurrect every session it was meant to kill.
		recentUser := uuid.New()
		recentCutoff := time.Now().Add(-time.Minute)
		bl.userInvalidatedAt.Store(recentUser, recentCutoff)

		bl.Cleanup()

		if _, ok := bl.userInvalidatedAt.Load(staleUser); ok {
			t.Error("expected a cutoff older than the access-token lifetime to be evicted")
		}
		if _, ok := bl.userInvalidatedAt.Load(recentUser); !ok {
			t.Fatal("expected a cutoff still inside the access-token lifetime to survive cleanup")
		}
		if !bl.IsBlacklisted(t.Context(), "tok", recentUser, recentCutoff.Add(-time.Second)) {
			t.Error("cleanup resurrected a session that is still meant to be revoked")
		}
	})

	t.Run("prunes entries left behind by a redis outage", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		mr.Close()

		// Redis is configured, so before this change Cleanup returned early and
		// nothing ever evicted what the failed writes left behind in memory.
		_ = bl.BlacklistToken(t.Context(), "expired-token", time.Now().Add(15*time.Minute))
		if _, ok := bl.tokens.Load(tokenHash("expired-token")); !ok {
			t.Fatal("expected the failed write to be recorded in memory")
		}

		// Wind it past its expiry, the way another fifteen minutes would.
		bl.tokens.Store(tokenHash("expired-token"), time.Now().Add(-time.Second))
		bl.userInvalidatedAt.Store(uuid.New(), time.Now().Add(-accessTokenExpiry-time.Minute))

		bl.Cleanup()

		count := 0
		bl.tokens.Range(func(any, any) bool { count++; return true })
		bl.userInvalidatedAt.Range(func(any, any) bool { count++; return true })
		if count != 0 {
			t.Errorf("expected cleanup to prune the degraded-mode leftovers, %d entries remain", count)
		}
	})
}

// TestBlacklistSurvivesRedisRecovery is the other half of the outage, and the
// half that was missing: what happens once Redis comes back.
//
// The write failed, so the revocation exists only in this instance's maps —
// that is the whole point of storing it there. But IsBlacklisted used to
// consult those maps only while Redis was still broken, and answered from
// Redis alone the moment it recovered. Redis has never heard of the token, so
// it says "not revoked", and the session a user signed out of during the blip
// starts working again for the rest of its access-token lifetime. Nothing
// alarms at that point: degraded() already fired back when the write failed,
// and its message says the revocation is "enforced on this instance only",
// which by then is no longer true.
func TestBlacklistSurvivesRedisRecovery(t *testing.T) {
	t.Run("a token revoked during the outage stays revoked after recovery", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)

		// Redis is answering, but failing every command — a broken server
		// rather than a dead socket, so the client stays connected and the
		// recovery below needs no reconnect.
		mr.SetError("LOADING Redis is loading the dataset in memory")
		if err := bl.BlacklistToken(t.Context(), "signed-out-token", time.Now().Add(15*time.Minute)); err == nil {
			t.Fatal("expected BlacklistToken to report the redis failure")
		}

		mr.SetError("") // Redis recovers, without the write that never landed.

		if !bl.IsBlacklisted(t.Context(), "signed-out-token", uuid.New(), time.Now()) {
			t.Error("a revocation this instance recorded was discarded when redis came back")
		}
	})

	t.Run("a user cutoff recorded during the outage survives recovery", func(t *testing.T) {
		bl, mr := newRedisBlacklist(t)
		userID := uuid.New()
		issuedAt := time.Now().Add(-5 * time.Minute)

		mr.SetError("LOADING Redis is loading the dataset in memory")
		if err := bl.InvalidateUserTokens(t.Context(), userID); err == nil {
			t.Fatal("expected InvalidateUserTokens to report the redis failure")
		}

		mr.SetError("")

		if !bl.IsBlacklisted(t.Context(), "pre-cutoff-token", userID, issuedAt) {
			t.Error("a user cutoff this instance recorded was discarded when redis came back")
		}
		if bl.IsBlacklisted(t.Context(), "post-cutoff-token", userID, time.Now().Add(2*time.Second)) {
			t.Error("the recovered path must still allow a token issued after the cutoff")
		}
	})

	t.Run("a healthy redis that revoked nothing still allows the token", func(t *testing.T) {
		bl, _ := newRedisBlacklist(t)

		// The guard against the fix over-reaching: falling through to the
		// in-memory maps must not turn an empty map into a rejection.
		if bl.IsBlacklisted(t.Context(), "innocent-token", uuid.New(), time.Now()) {
			t.Error("nothing was revoked anywhere, yet the token was rejected")
		}
	})
}
