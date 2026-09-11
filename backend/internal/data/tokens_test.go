package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRefreshToken_StructFields(t *testing.T) {
	now := time.Now()
	token := RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: []byte("hash-value-123"),
		ExpiresAt: now.Add(7 * 24 * time.Hour),
		CreatedAt: now,
	}

	if token.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if token.UserID == uuid.Nil {
		t.Error("UserID should not be nil")
	}
	if len(token.TokenHash) == 0 {
		t.Error("TokenHash should not be empty")
	}
	if !token.ExpiresAt.After(token.CreatedAt) {
		t.Error("ExpiresAt should be after CreatedAt")
	}
}

func TestRefreshToken_ExpiryInFuture(t *testing.T) {
	now := time.Now()
	ttl := 7 * 24 * time.Hour
	token := RefreshToken{
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	duration := token.ExpiresAt.Sub(token.CreatedAt)
	if duration != ttl {
		t.Errorf("token TTL = %v, want %v", duration, ttl)
	}
}

// TestTokenModel_RequiresDB documents that all TokenModel methods
// require a database connection.
func TestTokenModel_RequiresDB(t *testing.T) {
	t.Skip("TokenModel methods (InsertRefreshToken, GetRefreshToken, MarkRefreshTokenUsed, etc.) all require *pgxpool.Pool and *db.Queries")
}
