package auth

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
)

// TestServiceLogin covers the rule this module is built around: every refusal
// of a sign-in answers the same thing, whatever caused it, and every one of
// them is written to the trail identically. Telling them apart is what would
// turn the endpoint into an oracle for which addresses have accounts.
func TestServiceLogin(t *testing.T) {
	const (
		email    = "ana@example.com"
		password = "correct-horse-battery"
	)

	// account builds a stored user in the state each case is about.
	account := func(t *testing.T, apply func(*authstore.User)) *authstore.User {
		t.Helper()
		u := &authstore.User{
			Email:         email,
			Role:          "owner",
			EmailVerified: true,
			IsActive:      true,
		}
		if err := u.SetPassword(password, bcrypt.MinCost); err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
		if apply != nil {
			apply(u)
		}
		return u
	}

	tests := []struct {
		name       string
		apply      func(*authstore.User)
		noAccount  bool
		password   string
		wantErr    error
		wantAction string
	}{
		{
			name:       "correct credentials establish a session",
			password:   password,
			wantAction: actionLogin,
		},
		{
			name:       "an unknown address is refused",
			noAccount:  true,
			password:   password,
			wantErr:    ErrInvalidCredentials,
			wantAction: actionLoginFailed,
		},
		{
			name:       "a wrong password is refused",
			password:   "not-the-password",
			wantErr:    ErrInvalidCredentials,
			wantAction: actionLoginFailed,
		},
		{
			name:       "a locked account is refused the same way a wrong password is",
			apply:      func(u *authstore.User) { until := time.Now().Add(time.Hour); u.LockedUntil = &until },
			password:   password,
			wantErr:    ErrInvalidCredentials,
			wantAction: actionLoginFailed,
		},
		{
			name:       "an unverified account is refused",
			apply:      func(u *authstore.User) { u.EmailVerified = false },
			password:   password,
			wantErr:    ErrInvalidCredentials,
			wantAction: actionLoginFailed,
		},
		{
			name:       "a deactivated account is refused",
			apply:      func(u *authstore.User) { u.IsActive = false },
			password:   password,
			wantErr:    ErrInvalidCredentials,
			wantAction: actionLoginFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if !tt.noAccount {
				f.users.add(account(t, tt.apply))
			}

			session, err := f.service.Login(t.Context(), Actor{IP: "1.2.3.4"}, email, tt.password)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if session != nil {
					t.Error("a refused sign-in still handed back a session")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if session == nil || session.AccessToken == "" || session.RefreshToken == "" {
					t.Fatal("a successful sign-in handed back no tokens")
				}
			}

			entry := f.audit.only(t)
			if entry.Action != tt.wantAction {
				t.Errorf("recorded %q, want %q", entry.Action, tt.wantAction)
			}
			if entry.IPAddress != "1.2.3.4" {
				t.Errorf("recorded IP %q, want the actor's", entry.IPAddress)
			}
			// Every refusal names the address that was typed and nothing else:
			// the actor is unknown and deliberately unnamed.
			if tt.wantErr != nil && entry.UserID != nil {
				t.Error("a refused sign-in named an actor; the trail must not say whether the address has an account")
			}
		})
	}
}
