package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A password changed from a signed-in session is told to the account's
// address, once per change: a second change is a second notice. A change the
// current password did not authorize tells nobody anything.
func TestChangingThePasswordTellsTheAccount(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)

	change := func(current, next string) int {
		w := httptest.NewRecorder()
		f.handler.UpdateCurrentUser(w, withUser(postJSON(t, `{"current_password":"`+current+`","new_password":"`+next+`"}`), user))
		return w.Code
	}

	if code := change("wrong-password-entirely", "a-brand-new-passphrase"); code == http.StatusOK {
		t.Fatalf("a wrong current password was accepted")
	}
	if len(f.notify.passwordChanges) != 0 {
		t.Fatalf("a refused change was announced: %+v", f.notify.passwordChanges)
	}

	if code := change("correct-horse-battery", "a-brand-new-passphrase"); code != http.StatusOK {
		t.Fatalf("want 200; got %d", code)
	}
	if code := change("a-brand-new-passphrase", "yet-another-passphrase"); code != http.StatusOK {
		t.Fatalf("want 200 for the second change; got %d", code)
	}

	got := f.notify.passwordChanges
	if len(got) != 2 {
		t.Fatalf("want one notice per change; got %d", len(got))
	}
	for _, n := range got {
		if n.email.To != "ana@example.com" {
			t.Errorf("notice sent to %q; want the account's address", n.email.To)
		}
	}
	if got[0].event == "" || got[0].event == got[1].event {
		t.Errorf("two changes share the event id %q; the queue would drop the second notice", got[0].event)
	}
}

// A password replaced through a reset link is told to the account's address:
// whoever held the link may not be the owner.
func TestResettingThePasswordTellsTheAccount(t *testing.T) {
	f := newFixture(t)
	f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

	f.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com"}`))
	token := tokenFromURL(t, f.notify.resets[0].ResetURL)

	w := httptest.NewRecorder()
	f.handler.ResetPassword(w, postJSON(t, `{"token":"`+token+`","password":"a-brand-new-password"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	if len(f.notify.passwordChanges) != 1 || f.notify.passwordChanges[0].email.To != "ana@example.com" {
		t.Fatalf("want one password-changed notice to the account; got %+v", f.notify.passwordChanges)
	}
	if f.notify.passwordChanges[0].event == "" {
		t.Error("the notice was queued without an event id")
	}
}

// A confirmed email change is told to the address the account had BEFORE the
// change, and only once the change is committed.
func TestConfirmingAnEmailChangeTellsThePreviousAddress(t *testing.T) {
	confirm := func(t *testing.T, updateErr error) (*fixture, int) {
		t.Helper()
		f := newFixture(t)
		user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
		f.users.add(user)
		f.handler.UpdateCurrentUser(httptest.NewRecorder(), withUser(postJSON(t, `{"email":"ana-new@example.com"}`), user))
		token := tokenFromURL(t, f.notify.emailChanges[0].ConfirmURL)

		f.users.updateErr = updateErr
		w := httptest.NewRecorder()
		f.handler.ConfirmEmailChange(w, postJSON(t, `{"token":"`+token+`"}`))
		return f, w.Code
	}

	t.Run("a change that did not commit tells nobody", func(t *testing.T) {
		f, _ := confirm(t, errors.New("connection reset"))
		if len(f.notify.emailChanged) != 0 {
			t.Errorf("a change that did not commit was announced: %+v", f.notify.emailChanged)
		}
	})

	t.Run("a committed change tells the previous address", func(t *testing.T) {
		f, code := confirm(t, nil)
		if code != http.StatusOK {
			t.Fatalf("want 200; got %d", code)
		}
		got := f.notify.emailChanged
		if len(got) != 1 {
			t.Fatalf("want one email-changed notice; got %d", len(got))
		}
		if got[0].email.To != "ana@example.com" || got[0].email.NewEmail != "ana-new@example.com" {
			t.Errorf("want the notice at the previous address naming the new one; got %+v", got[0].email)
		}
		if got[0].event == "" {
			t.Error("the notice was queued without the change request's id")
		}
	})
}

// A deleted account is told at the address it had, and a delete that failed
// announces nothing.
func TestDeletingTheAccountTellsItsAddress(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)

	f.users.deleteErr = errors.New("foreign key still points at a complex")
	f.handler.DeleteAccount(httptest.NewRecorder(), withUser(httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil), user))
	if len(f.notify.deletions) != 0 {
		t.Fatalf("a failed delete was announced: %+v", f.notify.deletions)
	}

	f.users.deleteErr = nil
	w := httptest.NewRecorder()
	f.handler.DeleteAccount(w, withUser(httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil), user))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	got := f.notify.deletions
	if len(got) != 1 || got[0].email.To != "ana@example.com" || got[0].event != user.ID.String() {
		t.Errorf("want one deletion notice to ana@example.com keyed on the account; got %+v", got)
	}
}
