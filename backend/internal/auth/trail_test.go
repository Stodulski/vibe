package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// encodedValue returns an audit value as the recorder will persist it.
//
// audit.Recorder.Record marshals with encoding/json before it hands anything to
// the background write, so these bytes are exactly what reaches the audit_log
// row. Asserting on them rather than on the Go struct is the difference between
// "the field is unexported or tagged" and "the secret is not in the table".
func encodedValue(t *testing.T, v any) string {
	t.Helper()
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("an audit value must encode, or the entry is written without it: %v", err)
	}
	return string(b)
}

// storedUser returns the account signIn put in the fixture.
func storedUser(t *testing.T, f *fixture) *authstore.User {
	t.Helper()
	u, ok := f.users.byEmail["ana@example.com"]
	if !ok {
		t.Fatal("the fixture holds no account")
	}
	return u
}

// findEntry returns the single entry with the given action.
func findEntry(t *testing.T, entries []audit.Entry, action string) audit.Entry {
	t.Helper()
	var found []audit.Entry
	for _, e := range entries {
		if e.Action == action {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly 1 %q entry; got %d out of %+v", action, len(found), entries)
	}
	return found[0]
}

func TestSigningInIsRecorded(t *testing.T) {
	f := newFixture(t)
	f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

	w := httptest.NewRecorder()
	f.handler.Login(w, postJSON(t, `{"email":"ana@example.com","password":"correct-horse-battery"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in failed: %d (%s)", w.Code, w.Body.String())
	}

	e := f.audit.only(t)
	if e.Action != actionLogin {
		t.Errorf("action is %q, want %q", e.Action, actionLogin)
	}
	if e.EntityType != entityUser {
		t.Errorf("entity type is %q, want %q", e.EntityType, entityUser)
	}
	if e.EntityID == nil || *e.EntityID != storedUser(t, f).ID {
		t.Errorf("the entry names account %v, want %v", e.EntityID, storedUser(t, f).ID)
	}
	if got := encodedValue(t, e.NewValue); !strings.Contains(got, "ana@example.com") {
		t.Errorf("the value does not carry the address: %s", got)
	}
	if e.IPAddress != "192.0.2.1" {
		t.Errorf("the client address is %q, want the request's peer", e.IPAddress)
	}
}

// A sign-in that fails must leave an entry that says nothing about whether the
// address has an account.
//
// The response was fixed to answer identically for a wrong password, a locked
// account, an unverified one, a deactivated one and an address nobody owns. If
// the trail then wrote a real user_id for the four that exist and NULL for the
// one that does not — with the account's email joined in beside it by
// data.listAuditLogsSQL — the oracle would simply have moved into permanent
// storage.
func TestAFailedSignInRecordsNoAccountEvenWhenTheAccountExists(t *testing.T) {
	unverified := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	unverified.EmailVerified = false

	deactivated := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	deactivated.IsActive = false

	tests := []struct {
		name    string
		account *authstore.User
		body    string
	}{
		{"no such account", nil, `{"email":"ana@example.com","password":"correct-horse-battery"}`},
		{"wrong password", verifiedUser(t, "ana@example.com", "correct-horse-battery"), `{"email":"ana@example.com","password":"wrong-guess"}`},
		{"locked out", lockedUser(t, "ana@example.com", "correct-horse-battery"), `{"email":"ana@example.com","password":"correct-horse-battery"}`},
		{"not verified", unverified, `{"email":"ana@example.com","password":"correct-horse-battery"}`},
		{"deactivated", deactivated, `{"email":"ana@example.com","password":"correct-horse-battery"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.account != nil {
				f.users.add(tt.account)
			}

			w := httptest.NewRecorder()
			f.handler.Login(w, postJSON(t, tt.body))
			if w.Code == http.StatusOK {
				t.Fatalf("this sign-in was supposed to fail; got %d", w.Code)
			}

			e := f.audit.only(t)
			if e.Action != actionLoginFailed {
				t.Errorf("action is %q, want %q", e.Action, actionLoginFailed)
			}
			if e.UserID != nil {
				t.Errorf("a failed sign-in named an actor (%v); the trail becomes the enumeration oracle the response stopped being", *e.UserID)
			}
			if e.EntityID != nil {
				t.Errorf("a failed sign-in named an account (%v); same oracle, different column", *e.EntityID)
			}
			if got := encodedValue(t, e.NewValue); !strings.Contains(got, "ana@example.com") {
				t.Errorf("the attempted address is the only thing tying this entry to anything, and it is missing: %s", got)
			}
		})
	}
}

// Nothing in this module writes an entry a tenant can read.
//
// The tenant trail is scoped `complex_id = $1` against the complex the ownership
// guard resolved (data.listAuditLogsSQL), and NULL matches no id — so a nil
// ComplexID is what keeps an owner's sign-in history, and the addresses
// strangers typed at the login form, out of a venue's audit page.
func TestNoEntryIsReadableByATenant(t *testing.T) {
	entries := everyAuditedFlow(t).entries
	if len(entries) < 9 {
		t.Fatalf("the flows produced only %d entries; the sweep is not covering this module", len(entries))
	}

	for _, e := range entries {
		if e.ComplexID != nil {
			t.Errorf("the %q entry is scoped to complex %v, which puts it on a venue's audit page", e.Action, *e.ComplexID)
		}
		if e.EntityType != entityUser {
			t.Errorf("the %q entry has entity type %q, want %q", e.Action, e.EntityType, entityUser)
		}
	}
}

// The test that matters: no credential reaches the audit value.
//
// A password, a hash, an access or refresh token, or a single-use reset token in
// this table is that credential published — the table is long-lived, has no
// retention job, and is read by people who are not the account's owner. The
// check is on the encoded bytes because encoding is what the recorder does, so
// it catches a field that a tag was supposed to hide and an untagged one alike.
func TestNoAuditValueCarriesACredential(t *testing.T) {
	flows := everyAuditedFlow(t)

	for _, e := range flows.entries {
		values := encodedValue(t, e.OldValue) + encodedValue(t, e.NewValue)
		for name, secret := range flows.secrets {
			if secret == "" {
				t.Fatalf("the %s secret was never captured, so this test proves nothing about it", name)
			}
			if strings.Contains(values, secret) {
				t.Errorf("the %q entry's value carries the %s: %s", e.Action, name, values)
			}
		}
	}
}

func TestSigningOutIsRecordedOnlyForASessionThatExisted(t *testing.T) {
	t.Run("a signed-in sign-out is recorded", func(t *testing.T) {
		f := newFixture(t)
		refresh := signIn(t, f)
		user := storedUser(t, f)

		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		r.AddCookie(refresh)
		r = withUser(r, user)

		w := httptest.NewRecorder()
		f.handler.Logout(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}

		e := findEntry(t, f.audit.entries, actionLogout)
		if e.UserID == nil || *e.UserID != user.ID {
			t.Errorf("the sign-out entry names actor %v, want %v", e.UserID, user.ID)
		}
		if e.EntityID == nil || *e.EntityID != user.ID {
			t.Errorf("the sign-out entry names account %v, want %v", e.EntityID, user.ID)
		}
	})

	// The route has no auth guard so an expired session can still clear its
	// cookies. Recording those would let anyone write rows into an unpruned
	// table with a cookie-less curl.
	t.Run("a sign-out with no session records nothing", func(t *testing.T) {
		f := newFixture(t)

		w := httptest.NewRecorder()
		f.handler.Logout(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.audit.entries) != 0 {
			t.Errorf("a sign-out that ended nothing was recorded: %+v", f.audit.entries)
		}
	})
}

// The deletion entry cannot name its actor in audit_log.user_id: that column is
// a foreign key into users (audit_log_user_id_fkey) and the row is already gone, so
// an entry pointing at it would be refused by the database and dropped — losing
// the one act the trail most needs to have witnessed. The account is named by
// entity_id, which carries no foreign key, and the actor is said in the value.
func TestDeletingAnAccountIsRecordedWithoutAForeignKeyToTheDeletedRow(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)

	r := withUser(httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil), user)
	w := httptest.NewRecorder()
	f.handler.DeleteAccount(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	e := findEntry(t, f.audit.entries, actionAccountDelete)
	if e.UserID != nil {
		t.Errorf("the deletion entry points user_id at the row that was just deleted (%v); the insert is refused and the entry is lost", *e.UserID)
	}
	if e.EntityID == nil || *e.EntityID != user.ID {
		t.Errorf("the deletion entry names account %v, want %v", e.EntityID, user.ID)
	}
	if got := encodedValue(t, e.NewValue); !strings.Contains(got, actorSelf) {
		t.Errorf("with user_id NULL the actor has to be said in words, and is not: %s", got)
	}
}

// A refusal is not a deletion, and must not be recorded as one.
func TestARefusedDeletionIsNotRecorded(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)
	f.complexes.owned = []*complexstore.Complex{{ID: uuid.New()}}
	f.bookings.hasActive = true

	r := withUser(httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil), user)
	w := httptest.NewRecorder()
	f.handler.DeleteAccount(w, r)
	if w.Code == http.StatusOK {
		t.Fatalf("this deletion was supposed to be refused; got %d", w.Code)
	}
	if len(f.audit.entries) != 0 {
		t.Errorf("an account that still exists was recorded as deleted: %+v", f.audit.entries)
	}
}

// Changing the address moves the account's recovery channel, and overwrites the
// only record of where it used to point.
func TestChangingTheAddressRecordsBothOfThem(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)

	r := withUser(postJSON(t, `{"email":"someone-else@example.com"}`), user)
	w := httptest.NewRecorder()
	f.handler.UpdateCurrentUser(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	e := findEntry(t, f.audit.entries, actionEmailChange)
	if got := encodedValue(t, e.OldValue); !strings.Contains(got, "ana@example.com") {
		t.Errorf("the address the account is leaving is nowhere else after this request, and not in the entry either: %s", got)
	}
	if got := encodedValue(t, e.NewValue); !strings.Contains(got, "someone-else@example.com") {
		t.Errorf("the new address is missing from the entry: %s", got)
	}
}

// A rename is data about a person; it is not control over an account.
func TestARenameIsNotRecorded(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	user.Phone = "+541112345678"
	f.users.add(user)

	r := withUser(postJSON(t, `{"last_name":"Gomez"}`), user)
	w := httptest.NewRecorder()
	f.handler.UpdateCurrentUser(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.audit.entries) != 0 {
		t.Errorf("a profile edit was written to the trail: %+v", f.audit.entries)
	}
}

// The two ways a password changes are different claims, and the trail has to
// keep them apart: one request carried a session this module authenticated, the
// other carried a token out of an email.
func TestAPasswordChangeAndAPasswordResetAreDifferentEntries(t *testing.T) {
	t.Run("changed from an authenticated session", func(t *testing.T) {
		f := newFixture(t)
		user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
		f.users.add(user)

		r := withUser(postJSON(t, `{"current_password":"correct-horse-battery","new_password":"a-brand-new-passphrase"}`), user)
		w := httptest.NewRecorder()
		f.handler.UpdateCurrentUser(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}

		e := findEntry(t, f.audit.entries, actionPasswordChange)
		if e.UserID == nil || *e.UserID != user.ID {
			t.Errorf("a change made from a session must name that session's account; got %v", e.UserID)
		}
	})

	t.Run("reset by whoever held the emailed link", func(t *testing.T) {
		f := newFixture(t)
		user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
		f.users.add(user)

		f.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com"}`))
		token := tokenFromURL(t, f.notify.resets[0].ResetURL)

		w := httptest.NewRecorder()
		f.handler.ResetPassword(w, postJSON(t, `{"token":"`+token+`","password":"a-brand-new-passphrase"}`))
		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}

		e := findEntry(t, f.audit.entries, actionPasswordReset)
		if e.UserID != nil {
			t.Errorf("this route is public and nothing authenticated the caller, yet the entry names actor %v", *e.UserID)
		}
		if e.EntityID == nil || *e.EntityID != user.ID {
			t.Errorf("the reset entry names account %v, want %v", e.EntityID, user.ID)
		}
	})
}

// ForgotPassword answers identically for an address with an account, one
// without, one that is unverified and one in cooldown. An entry written only on
// the exit that sends mail would rebuild that oracle inside the trail.
func TestAResetRequestIsRecordedWhetherOrNotTheAccountExists(t *testing.T) {
	known := newFixture(t)
	known.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
	known.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com"}`))

	unknown := newFixture(t)
	unknown.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"nobody@example.com"}`))

	for name, f := range map[string]*fixture{"an address with an account": known, "an address with none": unknown} {
		e := findEntry(t, f.audit.entries, actionPasswordResetRequest)
		if e.UserID != nil {
			t.Errorf("%s: the request entry names actor %v", name, *e.UserID)
		}
		if e.EntityID != nil {
			t.Errorf("%s: the request entry names account %v, which says the address exists", name, *e.EntityID)
		}
	}

	if got := encodedValue(t, findEntry(t, known.audit.entries, actionPasswordResetRequest).NewValue); !strings.Contains(got, "ana@example.com") {
		t.Errorf("the requested address is what the entry is for, and it is missing: %s", got)
	}
}

// Presenting an already-rotated refresh token destroys every session on the
// account, and nobody asked for it. The account is the party it happened to;
// who presented the token is exactly what is not known, so naming the account in
// the actor column would file the theft as something its victim did.
func TestRefreshTokenReuseIsRecordedAgainstTheAccountNotAsItsAct(t *testing.T) {
	f := newFixture(t)
	old := signIn(t, f)
	user := storedUser(t, f)

	first := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	first.AddCookie(old)
	f.handler.Refresh(httptest.NewRecorder(), first)
	// A replay moments after rotation reads as a sibling tab and is not
	// recorded; this one arrives long after (see refreshReuseGrace).
	f.tokens.ageUsedTokens(2 * refreshReuseGrace)

	replay := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	replay.AddCookie(old)
	w := httptest.NewRecorder()
	f.handler.Refresh(w, replay)
	if w.Code == http.StatusOK {
		t.Fatalf("a replayed token must be refused; got %d", w.Code)
	}

	e := findEntry(t, f.audit.entries, actionRefreshReuse)
	if e.UserID != nil {
		t.Errorf("the reuse entry attributes the act to %v; the presenter is unknown by definition", *e.UserID)
	}
	if e.EntityID == nil || *e.EntityID != user.ID {
		t.Errorf("the reuse entry names account %v, want %v", e.EntityID, user.ID)
	}
	if got := encodedValue(t, e.NewValue); !strings.Contains(got, actorUnknown) {
		t.Errorf("with user_id NULL the entry has to say the actor is unknown: %s", got)
	}
}

// An ordinary rotation is not recorded. It happens every few minutes per open
// session, and burying the entries above under it would cost more than it buys
// — audit_log has no retention job (see idx_audit_log_created_at in db/migrations/001_init.sql).
func TestAnOrdinaryRotationIsNotRecorded(t *testing.T) {
	f := newFixture(t)
	old := signIn(t, f)
	before := len(f.audit.entries)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	r.AddCookie(old)
	w := httptest.NewRecorder()
	f.handler.Refresh(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.audit.entries) != before {
		t.Errorf("a routine rotation wrote to the trail: %+v", f.audit.entries[before:])
	}
}

// A failed sign-in records unvalidated request input, one row per attempt, into
// a table with no retention job. Bodies run to 1 MiB, so uncapped, one request
// buys a megabyte of permanent storage and a script buys as much as it likes.
func TestARecordedAddressCannotBeUsedToInflateTheTable(t *testing.T) {
	f := newFixture(t)
	huge := strings.Repeat("a", 4000) + "@example.com"

	w := httptest.NewRecorder()
	f.handler.Login(w, postJSON(t, `{"email":"`+huge+`","password":"anything"}`))
	if w.Code == http.StatusOK {
		t.Fatalf("this sign-in was supposed to fail; got %d", w.Code)
	}

	value, ok := f.audit.only(t).NewValue.(accountEvent)
	if !ok {
		t.Fatalf("the entry's value is not an accountEvent: %#v", f.audit.entries[0].NewValue)
	}
	if len(value.Email) > maxRecordedAddress {
		t.Errorf("the recorded address is %d bytes; SMTP carries at most %d, so anything longer is not an address being kept, it is storage being bought", len(value.Email), maxRecordedAddress)
	}
	if !strings.HasPrefix(value.Email, "aaaa") {
		t.Errorf("what was attempted is the useful part of the entry and was dropped: %q", value.Email)
	}
}

// The trail records where an act came from, and which address that is depends on
// whether the deployment sits behind a proxy — the same one flag every other
// module's trail reads.
func TestTheForwardedAddressIsRecordedOnlyWhenProxiesAreTrusted(t *testing.T) {
	for _, tt := range []struct {
		name    string
		trust   bool
		wantIP  string
		forward string
	}{
		{"untrusted proxies", false, "127.0.0.1", "203.0.113.9"},
		{"trusted proxies", true, "203.0.113.9", "203.0.113.9"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.handler.cfg.TrustProxies = tt.trust
			f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

			r := postJSON(t, `{"email":"ana@example.com","password":"correct-horse-battery"}`)
			r.RemoteAddr = "127.0.0.1:41234"
			r.Header.Set("X-Forwarded-For", tt.forward)

			w := httptest.NewRecorder()
			f.handler.Login(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
			}
			if got := f.audit.only(t).IPAddress; got != tt.wantIP {
				t.Errorf("recorded %q, want %q", got, tt.wantIP)
			}
		})
	}
}

// A handler built without a recorder is refused at construction rather than left
// to fail at the first sign-in, in production, for everyone.
func TestNewHandlerRefusesAHandlerWithNoRecorder(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a handler with no audit recorder was accepted; the first sign-in would panic instead of the deploy failing")
		}
	}()

	NewHandler(Dependencies{
		Users:  newStubUsers(),
		Tokens: newStubTokens(),
	}, Config{JWTSecret: testJWTSecret, PasswordHashCost: bcrypt.MinCost})
}

// auditRun is every entry this module writes, alongside every secret that passed
// through the requests that wrote them.
type auditRun struct {
	entries []audit.Entry
	secrets map[string]string
}

// everyAuditedFlow drives each path in this module that writes to the trail.
//
// It exists so the two sweeping assertions — no credential in any value, no
// entry a tenant can read — cover the module rather than whichever handler a
// test happened to call. A flow added without being listed here is a flow those
// two never see, which is why TestNoEntryIsReadableByATenant also asserts the
// count.
//
//nolint:funlen // a list of flows; splitting it hides what is and is not covered
func everyAuditedFlow(t *testing.T) auditRun {
	t.Helper()

	const (
		password    = "correct-horse-battery"
		newPassword = "a-brand-new-passphrase"
	)
	run := auditRun{secrets: map[string]string{
		"password":     password,
		"new password": newPassword,
	}}
	collect := func(f *fixture) { run.entries = append(run.entries, f.audit.entries...) }

	// A sign-in, and the three tokens it mints.
	signedIn := newFixture(t)
	signedIn.users.add(verifiedUser(t, "ana@example.com", password))
	w := httptest.NewRecorder()
	signedIn.handler.Login(w, postJSON(t, `{"email":"ana@example.com","password":"`+password+`"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in failed: %d (%s)", w.Code, w.Body.String())
	}
	refresh := findCookie(w.Header(), "refresh_token")
	access := findCookie(w.Header(), "access_token")
	if refresh == nil || access == nil {
		t.Fatal("the sign-in issued no session cookies")
	}
	run.secrets["refresh token"] = refresh.Value
	run.secrets["access token"] = access.Value
	csrf, _ := decode(t, w)["csrf_token"].(string)
	run.secrets["csrf token"] = csrf
	run.secrets["password hash"] = string(storedUser(t, signedIn).PasswordHash)

	// A sign-out of that session.
	out := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	out.AddCookie(refresh)
	out.AddCookie(access)
	signedIn.handler.Logout(httptest.NewRecorder(), withUser(out, storedUser(t, signedIn)))
	collect(signedIn)

	// A failed sign-in against a real account, guessing with the other secret.
	failed := newFixture(t)
	failed.users.add(verifiedUser(t, "ana@example.com", password))
	failed.handler.Login(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com","password":"`+newPassword+`"}`))
	collect(failed)

	// An address change and a password change, from a session.
	edited := newFixture(t)
	editUser := verifiedUser(t, "ana@example.com", password)
	edited.users.add(editUser)
	editW := httptest.NewRecorder()
	edited.handler.UpdateCurrentUser(editW, withUser(postJSON(t,
		`{"email":"moved@example.com","current_password":"`+password+`","new_password":"`+newPassword+`"}`), editUser))
	if editW.Code != http.StatusOK {
		t.Fatalf("the edit failed: %d (%s)", editW.Code, editW.Body.String())
	}
	collect(edited)

	// A reset request and the reset it leads to, with the emailed token.
	reset := newFixture(t)
	reset.users.add(verifiedUser(t, "ana@example.com", password))
	reset.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com"}`))
	resetToken := tokenFromURL(t, reset.notify.resets[0].ResetURL)
	run.secrets["reset token"] = resetToken
	resetW := httptest.NewRecorder()
	reset.handler.ResetPassword(resetW, postJSON(t, `{"token":"`+resetToken+`","password":"`+newPassword+`"}`))
	if resetW.Code != http.StatusOK {
		t.Fatalf("the reset failed: %d (%s)", resetW.Code, resetW.Body.String())
	}
	collect(reset)

	// A replayed refresh token.
	replayed := newFixture(t)
	spent := signIn(t, replayed)
	firstUse := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	firstUse.AddCookie(spent)
	replayed.handler.Refresh(httptest.NewRecorder(), firstUse)
	replayed.tokens.ageUsedTokens(2 * refreshReuseGrace) // past the sibling-tab grace window
	replay := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	replay.AddCookie(spent)
	replayed.handler.Refresh(httptest.NewRecorder(), replay)
	collect(replayed)

	// A deletion.
	deleted := newFixture(t)
	deletedUser := verifiedUser(t, "ana@example.com", password)
	deleted.users.add(deletedUser)
	delW := httptest.NewRecorder()
	deleted.handler.DeleteAccount(delW, withUser(
		httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil), deletedUser))
	if delW.Code != http.StatusOK {
		t.Fatalf("the deletion failed: %d (%s)", delW.Code, delW.Body.String())
	}
	collect(deleted)

	seen := map[string]bool{}
	for _, e := range run.entries {
		seen[e.Action] = true
	}
	for _, action := range []string{
		actionLogin, actionLoginFailed, actionLogout, actionEmailChange, actionPasswordChange,
		actionPasswordResetRequest, actionPasswordReset, actionAccountDelete, actionRefreshReuse,
	} {
		if !seen[action] {
			t.Fatalf("no flow here produces a %q entry, so nothing sweeping it is tested", action)
		}
	}
	return run
}

// The application wires the real recorder in through this interface, so a
// signature drift in either would break the deploy and not a test.
var _ Recorder = (*audit.Recorder)(nil)
