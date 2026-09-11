package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordResetTokenExpiry_Value(t *testing.T) {
	if passwordResetTokenExpiry != 1*time.Hour {
		t.Errorf("passwordResetTokenExpiry = %v, want %v", passwordResetTokenExpiry, 1*time.Hour)
	}
}

func TestPasswordResetToken_StructFields(t *testing.T) {
	now := time.Now()
	token := PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: []byte("resetHash"),
		ExpiresAt: now.Add(passwordResetTokenExpiry),
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

func TestPasswordResetToken_ExpiryDuration(t *testing.T) {
	now := time.Now()
	token := PasswordResetToken{
		CreatedAt: now,
		ExpiresAt: now.Add(passwordResetTokenExpiry),
	}

	duration := token.ExpiresAt.Sub(token.CreatedAt)
	if duration != 1*time.Hour {
		t.Errorf("token validity duration = %v, want %v", duration, 1*time.Hour)
	}
}

func TestPasswordResetToken_ShorterThanEmailVerification(t *testing.T) {
	if passwordResetTokenExpiry >= emailVerificationTokenExpiry {
		t.Errorf("passwordResetTokenExpiry (%v) should be shorter than emailVerificationTokenExpiry (%v)",
			passwordResetTokenExpiry, emailVerificationTokenExpiry)
	}
}

// TestPasswordResetModel_RequiresDB documents that all PasswordResetModel methods
// require a database connection.
func TestPasswordResetModel_RequiresDB(t *testing.T) {
	t.Skip("PasswordResetModel methods (InsertWithCooldown, GetByHash, DeleteByUser, DeleteExpired) all require *pgxpool.Pool")
}
