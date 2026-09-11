package auth

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokenBlacklist(t *testing.T) {
	t.Run("blacklisted token is rejected", func(t *testing.T) {
		bl := NewTokenBlacklist(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		_ = bl.BlacklistToken(t.Context(), "my-token", time.Now().Add(15*time.Minute))

		if !bl.IsBlacklisted(t.Context(), "my-token", uuid.New(), time.Now()) {
			t.Error("expected token to be blacklisted")
		}
	})

	t.Run("non-blacklisted token is allowed", func(t *testing.T) {
		bl := NewTokenBlacklist(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

		if bl.IsBlacklisted(t.Context(), "other-token", uuid.New(), time.Now()) {
			t.Error("expected token to not be blacklisted")
		}
	})

	t.Run("user invalidation rejects old tokens", func(t *testing.T) {
		bl := NewTokenBlacklist(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		userID := uuid.New()

		issuedAt := time.Now().Add(-5 * time.Minute)
		_ = bl.InvalidateUserTokens(t.Context(), userID)

		if !bl.IsBlacklisted(t.Context(), "some-token", userID, issuedAt) {
			t.Error("expected token issued before invalidation to be blacklisted")
		}
	})

	t.Run("user invalidation allows new tokens", func(t *testing.T) {
		bl := NewTokenBlacklist(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		userID := uuid.New()

		_ = bl.InvalidateUserTokens(t.Context(), userID)
		// JWT iat has second precision — token issued 2 seconds later is clearly after cutoff.
		issuedAt := time.Now().Add(2 * time.Second)

		if bl.IsBlacklisted(t.Context(), "new-token", userID, issuedAt) {
			t.Error("expected token issued after invalidation to be allowed")
		}
	})

	t.Run("cleanup removes expired entries", func(t *testing.T) {
		bl := NewTokenBlacklist(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		_ = bl.BlacklistToken(t.Context(), "expired-token", time.Now().Add(-1*time.Second))

		bl.Cleanup()

		if bl.IsBlacklisted(t.Context(), "expired-token", uuid.New(), time.Now()) {
			t.Error("expected expired token to be cleaned up")
		}
	})
}
