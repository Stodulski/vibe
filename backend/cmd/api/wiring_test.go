package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// These tests exist because of a defect that shipped: main() constructed the
// auth, payments and bookings handlers before assigning app.notify, so each
// captured a nil *notifications.Service. The field is interface-typed at the
// far end, and a nil pointer in an interface is not itself nil, so no check
// could see it — every registration panicked on the enqueue and came back as a
// 500. The test harness reproduced the same order, which is why no test caught
// it.
//
// Driving the real router is the point: a handler-level test builds the
// dependency itself and can never observe how main() ordered its wiring.

// TestRegisterCreatesAccountAndEnqueuesVerification drives the real router and
// asserts both halves: the caller gets a 201, and the verification email was
// actually published. The status code alone would also pass against a
// notification service that silently swallowed the call, which is the wrong
// fix for the defect above.
func TestRegisterCreatesAccountAndEnqueuesVerification(t *testing.T) {
	app, queue := newTestApplicationWithNotifications(t)
	ts := newTestServer(t, app)

	const email = "nueva.cuenta@example.com"
	status, body := postJSON(t, ts, "/api/v1/auth/register", `{
		"email": "`+email+`",
		"password": "un-password-valido",
		"first_name": "Ana",
		"last_name": "Gomez",
		"phone": "+5491112345678"
	}`)

	if status != http.StatusCreated {
		t.Fatalf("register: want 201; got %d\nbody: %s", status, body)
	}

	payloads := queue.payloadsOf(notifications.TaskEmailVerification)
	if len(payloads) != 1 {
		t.Fatalf("register: want 1 %q task enqueued; got %d (enqueued: %v)",
			notifications.TaskEmailVerification, len(payloads), queue.taskTypes())
	}

	verification, ok := payloads[0].(notifications.VerificationEmail)
	if !ok {
		t.Fatalf("register: want a notifications.VerificationEmail payload; got %T", payloads[0])
	}
	if verification.To != email {
		t.Errorf("register: verification addressed to %q; want %q", verification.To, email)
	}
	if verification.VerifyURL == "" {
		t.Error("register: verification enqueued without a verify URL")
	}
}

// TestRegisterDuplicateEmailEnqueuesDuplicateNotice covers the other branch of
// Register. It answers 201 as well — telling the caller the address is taken
// would make the endpoint an account oracle — and notifies the existing owner
// instead, through the same service, so it panicked for the same reason.
func TestRegisterDuplicateEmailEnqueuesDuplicateNotice(t *testing.T) {
	app, queue := newTestApplicationWithNotifications(t)

	const email = "ya.registrada@example.com"

	users, ok := app.models.Users.(*mockUserStore)
	if !ok {
		t.Fatalf("test application user store is %T, not *mockUserStore", app.models.Users)
	}
	// Set before the server starts so the handler goroutine sees it.
	users.InsertFn = func(context.Context, *authstore.User) error { return authstore.ErrDuplicateEmail }
	// The account that already owns this address, whose real first name is not
	// the one the registration form was filled in with.
	users.GetByEmailFn = func(context.Context, string) (*authstore.User, error) {
		return &authstore.User{Email: email, FirstName: "Mariana"}, nil
	}

	ts := newTestServer(t, app)

	status, body := postJSON(t, ts, "/api/v1/auth/register", `{
		"email": "`+email+`",
		"password": "un-password-valido",
		"first_name": "Ana",
		"last_name": "Gomez",
		"phone": "+5491112345678"
	}`)

	if status != http.StatusCreated {
		t.Fatalf("register (duplicate): want 201; got %d\nbody: %s", status, body)
	}

	payloads := queue.payloadsOf(notifications.TaskEmailDuplicateRegistration)
	if len(payloads) != 1 {
		t.Fatalf("register (duplicate): want 1 %q task enqueued; got %d (enqueued: %v)",
			notifications.TaskEmailDuplicateRegistration, len(payloads), queue.taskTypes())
	}

	duplicate, ok := payloads[0].(notifications.DuplicateRegistrationEmail)
	if !ok {
		t.Fatalf("register (duplicate): want a notifications.DuplicateRegistrationEmail payload; got %T", payloads[0])
	}
	if duplicate.To != email {
		t.Errorf("register (duplicate): notice addressed to %q; want %q", duplicate.To, email)
	}

	// The greeting is the account holder's own name, read back from the store,
	// and never the one typed at the form. This email goes to the person who
	// already owns the address and its whole subject is that a stranger just
	// touched their account; greeting them with the stranger's chosen words put
	// an attacker's text in Vibe's voice, addressed to the victim.
	if duplicate.FirstName == "Ana" {
		t.Error("register (duplicate): the notice greets the account holder with the name the attempted registration typed")
	}
	if duplicate.FirstName != "Mariana" {
		t.Errorf("register (duplicate): want the account's own first name; got %q", duplicate.FirstName)
	}

	if enqueued := queue.payloadsOf(notifications.TaskEmailVerification); len(enqueued) != 0 {
		t.Errorf("register (duplicate): enqueued %d verification email(s); a duplicate must not create one",
			len(enqueued))
	}
}

// postJSON sends a JSON body to the test server and returns the status and the
// response body.
func postJSON(t *testing.T, ts *httptest.Server, path, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	return resp.StatusCode, string(responseBody)
}
