package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEmailVerificationTokenExpiry_Value(t *testing.T) {
	if emailVerificationTokenExpiry != 24*time.Hour {
		t.Errorf("emailVerificationTokenExpiry = %v, want %v", emailVerificationTokenExpiry, 24*time.Hour)
	}
}

func TestEmailVerificationToken_StructFields(t *testing.T) {
	now := time.Now()
	token := EmailVerificationToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: []byte("somehash"),
		ExpiresAt: now.Add(emailVerificationTokenExpiry),
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

func TestEmailVerificationToken_ExpiryDuration(t *testing.T) {
	now := time.Now()
	token := EmailVerificationToken{
		CreatedAt: now,
		ExpiresAt: now.Add(emailVerificationTokenExpiry),
	}

	duration := token.ExpiresAt.Sub(token.CreatedAt)
	if duration != 24*time.Hour {
		t.Errorf("token validity duration = %v, want %v", duration, 24*time.Hour)
	}
}

// TestEmailVerificationModel_RequiresDB documents that all EmailVerificationModel methods
// require a database connection and cannot be unit tested without one.
func TestEmailVerificationModel_RequiresDB(t *testing.T) {
	t.Skip("EmailVerificationModel methods (Insert, InsertWithCooldown, GetByHash, DeleteByUser, DeleteExpired) all require *pgxpool.Pool and *db.Queries")
}
