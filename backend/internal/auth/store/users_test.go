package store

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

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
