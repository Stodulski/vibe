package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// A password is opaque text. ReadJSON trims every other string in a request,
// so these tests hold the four password inputs to the exact bytes the client
// sent: a password with a space at each edge is stored and checked as typed.
// The behaviour was already correct when these were written (the password
// fields carry trim:"-"); they guard it against a future change to that tag.

const edgeSpacedPassword = " pass word "

// requireHashMatches fails unless hash verifies want exactly, and does not
// verify the trimmed form of want, which is the difference a trimmed password
// would make.
func requireHashMatches(t *testing.T, hash []byte, want string) {
	t.Helper()
	if hash == nil {
		t.Fatal("no password hash was stored")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(want)); err != nil {
		t.Errorf("the stored hash does not verify %q: %v", want, err)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(strings.TrimSpace(want))) == nil {
		t.Errorf("the stored hash also verifies the trimmed form of %q", want)
	}
}

func TestRegisterStoresThePasswordWithItsEdgeSpacesIntact(t *testing.T) {
	f := newFixture(t)
	body := `{"email":"ana@example.com","password":" pass word ","first_name":"Ana","last_name":"Perez","phone":"+541100000000"}`

	w := httptest.NewRecorder()
	f.handler.Register(w, postJSON(t, body))

	if f.users.inserted == nil {
		t.Fatalf("the account was not created (%d: %s)", w.Code, w.Body.String())
	}
	requireHashMatches(t, f.users.inserted.PasswordHash, edgeSpacedPassword)
}

func TestLoginMatchesThePasswordWithItsEdgeSpacesIntact(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "the password as registered",
			body: `{"email":"ana@example.com","password":" pass word "}`,
			want: http.StatusOK,
		},
		{
			// Control: the trimmed text is a different password, so it must not
			// sign in. Without it the first case could pass for the wrong reason.
			name: "the password without its spaces",
			body: `{"email":"ana@example.com","password":"pass word"}`,
			want: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.users.add(verifiedUser(t, "ana@example.com", edgeSpacedPassword))

			w := httptest.NewRecorder()
			f.handler.Login(w, postJSON(t, tt.body))

			if w.Code != tt.want {
				t.Fatalf("want %d; got %d (%s)", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestChangePasswordKeepsTheEdgeSpacesOfBothPasswords(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", edgeSpacedPassword)
	f.users.add(user)

	body := `{"current_password":" pass word ","new_password":"  another pass  "}`
	w := httptest.NewRecorder()
	f.handler.UpdateCurrentUser(w, withUser(postJSON(t, body), user))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	requireHashMatches(t, f.users.passwordHash, "  another pass  ")
}

func TestResetPasswordStoresThePasswordWithItsEdgeSpacesIntact(t *testing.T) {
	f := newFixture(t)
	f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

	f.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com"}`))
	if len(f.notify.resets) != 1 {
		t.Fatal("no reset email was sent")
	}
	token := tokenFromURL(t, f.notify.resets[0].ResetURL)

	w := httptest.NewRecorder()
	f.handler.ResetPassword(w, postJSON(t, `{"token":"`+token+`","password":" pass word "}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	requireHashMatches(t, f.users.passwordHash, edgeSpacedPassword)
}
