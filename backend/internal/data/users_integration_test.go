//go:build integration

package data_test

import (
	"context"
	"testing"
)

// UserModel.Update used to leave email_verified out of its SET list, so clearing the
// flag in Go changed nothing in the database. That matters because changing an account's
// email address is supposed to un-verify it: if the flag survives, the account keeps a
// verified badge on an address nobody proved they own, and password-reset links follow
// it there. Only a re-read can tell the difference — the in-memory struct says false
// either way.
func TestUserUpdatePersistsEmailVerified(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	user, err := f.Models.Users.GetByID(ctx, f.UserID)
	if err != nil {
		t.Fatalf("loading the owner: %v", err)
	}
	if !user.EmailVerified {
		t.Fatalf("the fixture owner must start out verified, otherwise this test proves nothing")
	}

	user.EmailVerified = false
	user.FirstName = "Renamed"
	if err := f.Models.Users.Update(ctx, user); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stored, err := f.Models.Users.GetByID(ctx, f.UserID)
	if err != nil {
		t.Fatalf("re-reading the owner: %v", err)
	}
	if stored.EmailVerified {
		t.Error("clearing email_verified must reach the database; the stored user is still verified")
	}
	if stored.FirstName != "Renamed" {
		t.Errorf("the rest of the update must still land: want first name %q, got %q", "Renamed", stored.FirstName)
	}
}

// The same column in the other direction, so the test cannot pass by an Update that
// simply hardcodes false.
func TestUserUpdateCanRestoreEmailVerified(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	if _, err := f.Pool.Exec(ctx, `UPDATE users SET email_verified = false WHERE id = $1`, f.UserID); err != nil {
		t.Fatalf("un-verifying the owner: %v", err)
	}

	user, err := f.Models.Users.GetByID(ctx, f.UserID)
	if err != nil {
		t.Fatalf("loading the owner: %v", err)
	}
	if user.EmailVerified {
		t.Fatal("the owner must start out unverified for this test to mean anything")
	}

	user.EmailVerified = true
	if err := f.Models.Users.Update(ctx, user); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stored, err := f.Models.Users.GetByID(ctx, f.UserID)
	if err != nil {
		t.Fatalf("re-reading the owner: %v", err)
	}
	if !stored.EmailVerified {
		t.Error("setting email_verified must reach the database; the stored user is still unverified")
	}
}

// GetByID used to lose failed_login_attempts, locked_until and last_failed_login
// on the way out of the database: SELECT * fetched them, db.User carried them,
// and userFromDB mapped twelve of fifteen columns.
//
// The unit test in users_mapping_test.go proves the mapper. This one proves the
// path, which is a different claim: it locks a real row through the same UPDATE
// the login flow uses, reads it back through the store method the refresh
// handler, the password-reset handler and the authenticate middleware all use,
// and asserts the lock is visible there. A mapper can be correct while the query
// feeding it selects the wrong columns, and only a round trip can tell.
func TestGetByIDSeesTheLockoutState(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	unlocked, err := f.Models.Users.GetByID(ctx, f.UserID)
	if err != nil {
		t.Fatalf("loading the owner: %v", err)
	}
	if unlocked.IsLocked() {
		t.Fatal("the fixture owner must start out unlocked, otherwise this test proves nothing")
	}
	if unlocked.FailedLoginAttempts != 0 || unlocked.LastFailedLogin != nil {
		t.Fatalf("the fixture owner must start out with a clean lockout record; got attempts=%d last=%v",
			unlocked.FailedLoginAttempts, unlocked.LastFailedLogin)
	}

	// IncrementFailedAttempts is the real write path: five attempts is the first
	// tier of the progressive lockout, so this produces a genuinely locked
	// account rather than a hand-written locked_until.
	for range 5 {
		if err := f.Models.Users.IncrementFailedAttempts(ctx, f.UserID); err != nil {
			t.Fatalf("incrementing failed attempts: %v", err)
		}
	}

	locked, err := f.Models.Users.GetByID(ctx, f.UserID)
	if err != nil {
		t.Fatalf("re-loading the owner: %v", err)
	}

	if locked.FailedLoginAttempts != 5 {
		t.Errorf("GetByID must carry failed_login_attempts; want 5, got %d", locked.FailedLoginAttempts)
	}
	if locked.LockedUntil == nil {
		t.Fatal("GetByID must carry locked_until; it is nil, so IsLocked reports this locked account as unlocked")
	}
	if !locked.IsLocked() {
		t.Errorf("a user locked until %v must read as locked", *locked.LockedUntil)
	}
	if locked.LastFailedLogin == nil {
		t.Error("GetByID must carry last_failed_login; it is nil")
	}

	// The two readers must agree. They used to be two hand-written field lists,
	// and the whole defect was that only one of them was complete.
	byEmail, err := f.Models.Users.GetByEmail(ctx, locked.Email)
	if err != nil {
		t.Fatalf("loading the owner by email: %v", err)
	}
	if byEmail.FailedLoginAttempts != locked.FailedLoginAttempts {
		t.Errorf("GetByEmail and GetByID disagree on failed_login_attempts: %d vs %d",
			byEmail.FailedLoginAttempts, locked.FailedLoginAttempts)
	}
	if byEmail.LockedUntil == nil || !byEmail.LockedUntil.Equal(*locked.LockedUntil) {
		t.Errorf("GetByEmail and GetByID disagree on locked_until: %v vs %v",
			byEmail.LockedUntil, locked.LockedUntil)
	}
	if byEmail.LastFailedLogin == nil || !byEmail.LastFailedLogin.Equal(*locked.LastFailedLogin) {
		t.Errorf("GetByEmail and GetByID disagree on last_failed_login: %v vs %v",
			byEmail.LastFailedLogin, locked.LastFailedLogin)
	}

	// And the other direction, so the test cannot be satisfied by a reader that
	// hardcodes a lock. ResetFailedAttempts writes NULL to both timestamps, which
	// must arrive as nil rather than as a zero time — time.Time{} is in the past,
	// so a zero would read as "unlocked" by luck rather than by mapping.
	if err := f.Models.Users.ResetFailedAttempts(ctx, f.UserID); err != nil {
		t.Fatalf("resetting failed attempts: %v", err)
	}

	cleared, err := f.Models.Users.GetByID(ctx, f.UserID)
	if err != nil {
		t.Fatalf("re-loading the owner after reset: %v", err)
	}
	if cleared.FailedLoginAttempts != 0 {
		t.Errorf("the reset must reach failed_login_attempts; got %d", cleared.FailedLoginAttempts)
	}
	if cleared.LockedUntil != nil {
		t.Errorf("a NULL locked_until must arrive as nil; got %v", *cleared.LockedUntil)
	}
	if cleared.LastFailedLogin != nil {
		t.Errorf("a NULL last_failed_login must arrive as nil; got %v", *cleared.LastFailedLogin)
	}
	if cleared.IsLocked() {
		t.Error("a cleared account must not read as locked")
	}
}
