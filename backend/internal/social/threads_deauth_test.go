package social

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
)

const testStatusURL = "https://vibe.com.ar/privacidad"

// fakeDisconnecter holds at most one connection, like the table it stands for,
// and removes it only for the account that owns it.
type fakeDisconnecter struct {
	conn    ThreadsConnection
	ok      bool
	deletes int
}

func (f *fakeDisconnecter) DisconnectThreads(_ context.Context, userID string) (string, bool, error) {
	if !f.ok || f.conn.UserID != userID {
		return "", false, nil
	}
	f.ok = false
	f.deletes++
	return f.conn.Username, true, nil
}

func connectedAs(userID, username string) *fakeDisconnecter {
	return &fakeDisconnecter{ok: true, conn: ThreadsConnection{UserID: userID, Username: username, AccessToken: "secret-long-token"}}
}

func newDeauth(configured bool, store *fakeDisconnecter, q *fakeEnqueuer) *ThreadsDeauth {
	logger := slog.New(slog.DiscardHandler)
	return NewThreadsDeauth(store, q, ThreadsDeauthConfig{
		Configured: configured,
		AppSecret:  testAppSecret,
		To:         "ops@vibe.test",
		StatusURL:  testStatusURL,
	}, httpx.NewResponder(logger), logger)
}

func deauthRequest(path, signed string) *http.Request {
	form := url.Values{"signed_request": {signed}}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func payloadFor(userID string) string {
	return `{"algorithm":"HMAC-SHA256","issued_at":1760000000,"user_id":"` + userID + `"}`
}

func TestUninstallRemovesTheConnectionAndMailsOnce(t *testing.T) {
	t.Parallel()
	store := connectedAs("42", "vibe.padel")
	q := &fakeEnqueuer{}
	d := newDeauth(true, store, q)

	w := httptest.NewRecorder()
	d.Uninstall(w, deauthRequest("/api/v1/social/threads/uninstall", signedRequestFor(testAppSecret, payloadFor("42"))))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if store.deletes != 1 || store.ok {
		t.Fatalf("connection removed %d times, still stored %v; want removed once", store.deletes, store.ok)
	}
	if len(q.calls) != 1 {
		t.Fatalf("emails queued = %d; want 1", len(q.calls))
	}
	mail := q.calls[0]
	if mail.taskType != TaskReport || mail.payload.Subject != "Threads: se desconectó la cuenta" {
		t.Fatalf("email = %s %q; want the disconnect notice", mail.taskType, mail.payload.Subject)
	}
	body := strings.Join(mail.payload.Lines, "\n")
	if !strings.Contains(body, "@vibe.padel") || strings.Contains(body, "secret-long-token") {
		t.Fatalf("email body = %q; want the account named and no token", body)
	}

	// Meta may deliver the same callback again: the connection is already gone,
	// so nothing is removed and no second email is queued.
	w = httptest.NewRecorder()
	d.Uninstall(w, deauthRequest("/api/v1/social/threads/uninstall", signedRequestFor(testAppSecret, payloadFor("42"))))
	if w.Code != http.StatusOK || store.deletes != 1 || len(q.calls) != 1 {
		t.Fatalf("second delivery: status %d, removals %d, emails %d; want 200, 1, 1", w.Code, store.deletes, len(q.calls))
	}
}

func TestUninstallRefusesABadSignatureAndKeepsTheConnection(t *testing.T) {
	t.Parallel()
	store := connectedAs("42", "vibe.padel")
	q := &fakeEnqueuer{}
	d := newDeauth(true, store, q)

	w := httptest.NewRecorder()
	d.Uninstall(w, deauthRequest("/api/v1/social/threads/uninstall", signedRequestFor("forged-secret", payloadFor("42"))))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", w.Code)
	}
	if !store.ok || len(q.calls) != 0 {
		t.Fatal("a forged request must not remove the connection or mail anyone")
	}
}

func TestUninstallForAnotherAccountIsAHarmlessNoOp(t *testing.T) {
	t.Parallel()
	store := connectedAs("42", "vibe.padel")
	q := &fakeEnqueuer{}
	d := newDeauth(true, store, q)

	w := httptest.NewRecorder()
	d.Uninstall(w, deauthRequest("/api/v1/social/threads/uninstall", signedRequestFor(testAppSecret, payloadFor("99"))))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 so Meta does not retry", w.Code)
	}
	if !store.ok || len(q.calls) != 0 {
		t.Fatal("a valid request for another account must not remove our connection")
	}
}

func TestDeleteAnswersWithTheStatusPageAndAConfirmationCode(t *testing.T) {
	t.Parallel()
	store := connectedAs("42", "vibe.padel")
	q := &fakeEnqueuer{}
	d := newDeauth(true, store, q)

	w := httptest.NewRecorder()
	d.Delete(w, deauthRequest("/api/v1/social/threads/delete", signedRequestFor(testAppSecret, payloadFor("42"))))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	var out struct {
		URL  string `json:"url"`
		Code string `json:"confirmation_code"`
	}
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.URL != testStatusURL || out.Code == "" {
		t.Fatalf("response = %+v; want the status URL and a confirmation code", out)
	}
	if store.ok || store.deletes != 1 || len(q.calls) != 1 {
		t.Fatalf("delete removed %d, stored %v, emails %d; want the connection gone and one email", store.deletes, store.ok, len(q.calls))
	}
	if strings.Contains(w.Body.String(), "secret-long-token") {
		t.Fatal("the delete response must never carry the token")
	}
}

func TestDeleteWithNothingStoredStillConfirms(t *testing.T) {
	t.Parallel()
	store := &fakeDisconnecter{}
	q := &fakeEnqueuer{}
	d := newDeauth(true, store, q)

	w := httptest.NewRecorder()
	d.Delete(w, deauthRequest("/api/v1/social/threads/delete", signedRequestFor(testAppSecret, payloadFor("42"))))
	var out map[string]string
	if w.Code != http.StatusOK || json.NewDecoder(w.Body).Decode(&out) != nil || out["confirmation_code"] == "" {
		t.Fatalf("status %d, body %s; want 200 with a confirmation code", w.Code, w.Body.String())
	}
	if len(q.calls) != 0 {
		t.Fatalf("emails = %d; want none when nothing was removed", len(q.calls))
	}
}

func TestDeleteRefusesABadSignature(t *testing.T) {
	t.Parallel()
	store := connectedAs("42", "vibe.padel")
	d := newDeauth(true, store, &fakeEnqueuer{})

	w := httptest.NewRecorder()
	d.Delete(w, deauthRequest("/api/v1/social/threads/delete", "bogus.payload"))
	if w.Code != http.StatusBadRequest || !store.ok {
		t.Fatalf("status = %d, kept = %v; want 400 and the connection kept", w.Code, store.ok)
	}
}

func TestDeauthCallbacksNeedTheAppConfigured(t *testing.T) {
	t.Parallel()
	store := connectedAs("42", "vibe.padel")
	d := newDeauth(false, store, &fakeEnqueuer{})

	for name, handle := range map[string]func(http.ResponseWriter, *http.Request){
		"uninstall": d.Uninstall,
		"delete":    d.Delete,
	} {
		w := httptest.NewRecorder()
		handle(w, deauthRequest("/api/v1/social/threads/"+name, signedRequestFor(testAppSecret, payloadFor("42"))))
		if w.Code != http.StatusNotImplemented {
			t.Fatalf("%s status = %d; want 501 when Threads is not configured", name, w.Code)
		}
	}
	if !store.ok {
		t.Fatal("an unconfigured app must not touch the connection")
	}
}

func TestDeauthRequiresASignedRequestField(t *testing.T) {
	t.Parallel()
	d := newDeauth(true, connectedAs("42", "vibe.padel"), &fakeEnqueuer{})

	w := httptest.NewRecorder()
	d.Uninstall(w, deauthRequest("/api/v1/social/threads/uninstall", ""))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400 for an empty signed_request", w.Code)
	}
}
