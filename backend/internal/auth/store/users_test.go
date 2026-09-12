package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestUser_SetPassword(t *testing.T) {
	u := &User{}
	err := u.SetPassword("mysecretpassword", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("SetPassword() returned error: %v", err)
	}
	if len(u.PasswordHash) == 0 {
		t.Error("PasswordHash should not be empty after SetPassword")
	}
}

func TestUser_SetPassword_DifferentInputs(t *testing.T) {
	u1 := &User{}
	u2 := &User{}

	if err := u1.SetPassword("password1", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword(password1) error: %v", err)
	}
	if err := u2.SetPassword("password2", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword(password2) error: %v", err)
	}

	// Bcrypt generates unique hashes each time, even for same input.
	if string(u1.PasswordHash) == string(u2.PasswordHash) {
		t.Error("different passwords should produce different hashes")
	}
}

func TestUser_SetPassword_SameInputProducesDifferentHashes(t *testing.T) {
	u1 := &User{}
	u2 := &User{}

	if err := u1.SetPassword("samepassword", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword error: %v", err)
	}
	if err := u2.SetPassword("samepassword", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword error: %v", err)
	}

	// Bcrypt uses random salt, so same input produces different hashes.
	if string(u1.PasswordHash) == string(u2.PasswordHash) {
		t.Error("bcrypt should produce unique hashes even for same password")
	}
}

func TestUser_PasswordMatches_Correct(t *testing.T) {
	u := &User{}
	password := "correcthorsebatterystaple"
	if err := u.SetPassword(password, bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword error: %v", err)
	}

	matches, err := u.PasswordMatches(password)
	if err != nil {
		t.Fatalf("PasswordMatches() returned error: %v", err)
	}
	if !matches {
		t.Error("PasswordMatches should return true for correct password")
	}
}

func TestUser_PasswordMatches_Incorrect(t *testing.T) {
	u := &User{}
	if err := u.SetPassword("rightpassword", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword error: %v", err)
	}

	matches, err := u.PasswordMatches("wrongpassword")
	if err != nil {
		t.Fatalf("PasswordMatches() returned error: %v", err)
	}
	if matches {
		t.Error("PasswordMatches should return false for incorrect password")
	}
}

func TestUser_PasswordMatches_EmptyPassword(t *testing.T) {
	u := &User{}
	if err := u.SetPassword("somepassword", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword error: %v", err)
	}

	matches, err := u.PasswordMatches("")
	if err != nil {
		t.Fatalf("PasswordMatches() returned error: %v", err)
	}
	if matches {
		t.Error("PasswordMatches should return false for empty password")
	}
}

func TestUser_IsLocked_NilLockedUntil(t *testing.T) {
	u := &User{LockedUntil: nil}
	if u.IsLocked() {
		t.Error("IsLocked should return false when LockedUntil is nil")
	}
}

func TestUser_IsLocked_FutureTime(t *testing.T) {
	future := time.Now().Add(1 * time.Hour)
	u := &User{LockedUntil: &future}
	if !u.IsLocked() {
		t.Error("IsLocked should return true when LockedUntil is in the future")
	}
}

func TestUser_IsLocked_PastTime(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	u := &User{LockedUntil: &past}
	if u.IsLocked() {
		t.Error("IsLocked should return false when LockedUntil is in the past")
	}
}

func TestComparePassword_Match(t *testing.T) {
	u := &User{}
	if err := u.SetPassword("testpassword", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword error: %v", err)
	}

	err := ComparePassword(u.PasswordHash, "testpassword")
	if err != nil {
		t.Errorf("ComparePassword should not return error for matching password: %v", err)
	}
}

func TestComparePassword_Mismatch(t *testing.T) {
	u := &User{}
	if err := u.SetPassword("testpassword", bcrypt.MinCost); err != nil {
		t.Fatalf("SetPassword error: %v", err)
	}

	err := ComparePassword(u.PasswordHash, "wrongpassword")
	if err == nil {
		t.Error("ComparePassword should return error for mismatched password")
	}
}

func TestUser_StructFields(t *testing.T) {
	now := time.Now()
	lockedUntil := now.Add(15 * time.Minute)
	lastFailed := now.Add(-5 * time.Minute)
	u := User{
		ID:                  uuid.New(),
		Email:               "test@example.com",
		PasswordHash:        []byte("hash"),
		FirstName:           "John",
		LastName:            "Doe",
		Phone:               "+5491155551234",
		Role:                "owner",
		IsActive:            true,
		EmailVerified:       true,
		CreatedAt:           now,
		UpdatedAt:           now,
		FailedLoginAttempts: 3,
		LockedUntil:         &lockedUntil,
		LastFailedLogin:     &lastFailed,
	}

	if u.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", u.Email, "test@example.com")
	}
	if u.Role != "owner" {
		t.Errorf("Role = %q, want %q", u.Role, "owner")
	}
	if !u.IsActive {
		t.Error("IsActive should be true")
	}
	if !u.EmailVerified {
		t.Error("EmailVerified should be true")
	}
	if u.FailedLoginAttempts != 3 {
		t.Errorf("FailedLoginAttempts = %d, want %d", u.FailedLoginAttempts, 3)
	}
}

// TestUserModel_RequiresDB documents that Users methods
// require a database connection.
func TestUserModel_RequiresDB(t *testing.T) {
	t.Skip("Users methods all require *pgxpool.Pool and *db.Queries")
}
