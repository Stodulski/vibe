package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
)

// seedVerificationLink adds an account with the given role and verification
// state, plus one live verification link for it, and returns the account and
// the link's plaintext token.
func seedVerificationLink(t *testing.T, f *fixture, role string, verified bool) (*authstore.User, string) {
	t.Helper()

	u := &authstore.User{
		ID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", LastName: "Perez",
		Role: role, IsActive: true, EmailVerified: verified,
	}
	f.users.add(u)

	const link = "verification-link-token"
	if err := f.verifications.Insert(t.Context(), u.ID, hashToken(link)); err != nil {
		t.Fatalf("seeding the verification link: %v", err)
	}
	return u, link
}

// The welcome is queued the first time an owner's address becomes verified, and
// it names the owner, so the queue can dedupe it by account.
func TestVerifyingAnOwnersEmailQueuesTheWelcomeOnce(t *testing.T) {
	f := newFixture(t)
	u, link := seedVerificationLink(t, f, "owner", false)

	if err := f.service.VerifyEmail(t.Context(), link); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if len(f.notify.welcomes) != 1 {
		t.Fatalf("want one welcome after verifying; got %d", len(f.notify.welcomes))
	}
	got := f.notify.welcomes[0]
	if got.userID != u.ID.String() {
		t.Errorf("welcome queued under %q, want the owner %q", got.userID, u.ID)
	}
	if got.email.To != "ana@example.com" || got.email.FirstName != "Ana" {
		t.Errorf("welcome addressed to %+v, want Ana at ana@example.com", got.email)
	}

	// The same link is spent: presenting it again is refused and queues nothing.
	if err := f.service.VerifyEmail(t.Context(), link); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("second VerifyEmail err = %v, want ErrInvalidToken", err)
	}
	if len(f.notify.welcomes) != 1 {
		t.Errorf("a second verification queued another welcome; got %d", len(f.notify.welcomes))
	}
}

// An account that was already verified does not become usable again, so a
// verification that arrives after the fact (a link issued before the address
// was proven by another route) queues nothing.
func TestVerifyingAnAlreadyVerifiedOwnerQueuesNoWelcome(t *testing.T) {
	f := newFixture(t)
	_, link := seedVerificationLink(t, f, "owner", true)

	if err := f.service.VerifyEmail(t.Context(), link); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if n := len(f.notify.welcomes); n != 0 {
		t.Errorf("re-verifying a verified owner queued %d welcome(s); want none", n)
	}
}

// Superadmin accounts are internal; the owner welcome is never sent to them.
func TestVerifyingASuperadminQueuesNoWelcome(t *testing.T) {
	f := newFixture(t)
	_, link := seedVerificationLink(t, f, "superadmin", false)

	if err := f.service.VerifyEmail(t.Context(), link); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if n := len(f.notify.welcomes); n != 0 {
		t.Errorf("a superadmin verification queued %d welcome(s); want none", n)
	}
}

// A first-time Google sign-up creates an owner that is already verified, and
// its welcome is queued once, under the new account.
func TestGoogleSignupQueuesTheWelcomeOnce(t *testing.T) {
	f := newFixture(t)
	claims := &GoogleProfileClaims{Email: "ana@example.com", GivenName: "Ana", FamilyName: "Perez"}

	if _, err := f.service.GoogleComplete(t.Context(), Actor{}, claims, GoogleCompleteInput{
		FirstName: "Ana", LastName: "Perez", Phone: "+5491112345678",
	}); err != nil {
		t.Fatalf("GoogleComplete: %v", err)
	}

	created := f.users.byEmail["ana@example.com"]
	if created == nil {
		t.Fatal("GoogleComplete did not create the account")
	}
	if len(f.notify.welcomes) != 1 || f.notify.welcomes[0].userID != created.ID.String() {
		t.Fatalf("want one welcome under the new owner; got %+v", f.notify.welcomes)
	}
}

// When the address is already taken, the Google sign-up creates nothing and
// queues no welcome for the existing account.
func TestGoogleSignupOnATakenAddressQueuesNoWelcome(t *testing.T) {
	f := newFixture(t)
	seedVerificationLink(t, f, "owner", true)
	claims := &GoogleProfileClaims{Email: "ana@example.com", GivenName: "Ana", FamilyName: "Perez"}

	_, err := f.service.GoogleComplete(t.Context(), Actor{}, claims, GoogleCompleteInput{
		FirstName: "Ana", LastName: "Perez", Phone: "+5491112345678",
	})
	if !errors.Is(err, ErrAccountExists) {
		t.Fatalf("GoogleComplete err = %v, want ErrAccountExists", err)
	}
	if n := len(f.notify.welcomes); n != 0 {
		t.Errorf("a refused Google sign-up queued %d welcome(s); want none", n)
	}
}

// A Google sign-in on a registered but never-verified owner is the moment that
// account becomes usable, so it gets the welcome too.
func TestClaimingAnUnverifiedOwnerWithGoogleQueuesTheWelcome(t *testing.T) {
	f := newFixture(t)
	u, _ := seedVerificationLink(t, f, "owner", false)

	if err := f.service.claimUnverifiedAccount(t.Context(), Actor{}, u); err != nil {
		t.Fatalf("claimUnverifiedAccount: %v", err)
	}
	if len(f.notify.welcomes) != 1 || f.notify.welcomes[0].userID != u.ID.String() {
		t.Fatalf("want one welcome under the claimed owner; got %+v", f.notify.welcomes)
	}
}
