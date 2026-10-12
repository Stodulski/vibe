package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

func TestIssueVerificationTokenStoresAHashUnderTheResendCooldown(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()

	plain, err := f.service.IssueVerificationToken(t.Context(), userID)
	if err != nil {
		t.Fatalf("IssueVerificationToken: %v", err)
	}
	if plain == "" {
		t.Fatal("no plaintext token was returned for the link")
	}
	if f.verifications.cooldowns != 1 {
		t.Fatalf("token was inserted %d times through the cooldown path, want 1", f.verifications.cooldowns)
	}
	found := false
	for _, tok := range f.verifications.tokens {
		if tok.UserID == userID {
			found = true
		}
	}
	if !found {
		t.Fatal("no verification token was stored for the user")
	}
}

func TestIssueVerificationTokenReturnsTheCooldownRefusal(t *testing.T) {
	f := newFixture(t)
	f.verifications.insertErr = data.ErrCooldownActive

	_, err := f.service.IssueVerificationToken(t.Context(), uuid.New())
	if !errors.Is(err, data.ErrCooldownActive) {
		t.Fatalf("err = %v, want ErrCooldownActive so the nudge can release its claim", err)
	}
}
