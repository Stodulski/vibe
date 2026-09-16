package store

import (
	"testing"
	"time"
)

func TestEmailChangeTokenExpiry_Value(t *testing.T) {
	if EmailChangeTokenExpiry != 1*time.Hour {
		t.Errorf("EmailChangeTokenExpiry = %v, want %v", EmailChangeTokenExpiry, 1*time.Hour)
	}
}

// The email-change link follows the password-reset TTL convention exactly —
// see internal/auth/store/email_change.go's own comment on why.
func TestEmailChangeTokenExpiry_MatchesPasswordReset(t *testing.T) {
	if EmailChangeTokenExpiry != passwordResetTokenExpiry {
		t.Errorf("EmailChangeTokenExpiry = %v, passwordResetTokenExpiry = %v; the convention requires they match",
			EmailChangeTokenExpiry, passwordResetTokenExpiry)
	}
}

// Real coverage of Put's upsert, Peek/Consume's atomic-enough consumption and
// DeleteExpired's filtering lives in email_change_integration_test.go, under
// this package's usual //go:build integration + datatest.Isolated pattern
// (see internal/auth/store/users_integration_test.go) — every EmailChanges
// method needs a real *pgxpool.Pool, and a struct-literal or skipped test here
// proved nothing beyond what the Go compiler already checks.
