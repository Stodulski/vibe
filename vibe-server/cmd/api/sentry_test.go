package main

import (
	neturl "net/url"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"

	"github.com/stodulski/vibe-server/internal/booklink"
)

// send runs an event through the BeforeSend hook, as the SDK would.
func send(t *testing.T, event *sentry.Event) *sentry.Event {
	t.Helper()

	out := scrubEvent(event, nil)
	if out == nil {
		t.Fatal("scrubEvent dropped the event; a silent drop trades one invisible failure for another")
	}
	return out
}

// ---------------------------------------------------------------------------
// Free text
// ---------------------------------------------------------------------------

// Each case asserts both halves: the secret is gone, and enough of the
// surrounding message survives to still be diagnostic. A scrubber that
// replaced everything would pass the first half alone.
func TestScrubTextRemovesSecretsAndKeepsTheDiagnosis(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		gone []string
		kept []string
	}{
		{
			// internal/mp's APIError embeds the provider's raw response body.
			name: "mercadopago response body",
			in: `failed to create preference: mp: API error 400: ` +
				`{"message":"invalid card","cause":[{"code":3034,"data":"4509953566233704"}],` +
				`"payer":{"email":"ana@example.com"}}`,
			gone: []string{"4509953566233704", "ana@example.com", "invalid card"},
			kept: []string{"failed to create preference", "mp: API error 400"},
		},
		{
			name: "access token in a query string",
			in:   "GET /api/v1/auth/refresh?access_token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.abcdEFGH failed",
			gone: []string{"eyJhbGciOiJIUzI1NiJ9", "abcdEFGH"},
			kept: []string{"/api/v1/auth/refresh", "failed"},
		},
		{
			name: "bare jwt in a panic value",
			in:   "invalid token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhYmMifQ.Zm9vYmFyYmF6 for user",
			gone: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "Zm9vYmFyYmF6"},
			kept: []string{"invalid token", "for user"},
		},
		{
			name: "seller access token",
			in:   "refresh failed for token APP_USR-1234567890abcdef-081512-abcdef on complex Padel Norte",
			gone: []string{"APP_USR-1234567890abcdef-081512-abcdef"},
			kept: []string{"refresh failed", "Padel Norte"},
		},
		{
			name: "authorization header value",
			in:   `upstream rejected: Authorization: Bearer sk_live_9f8e7d6c5b4a3210`,
			gone: []string{"sk_live_9f8e7d6c5b4a3210"},
			kept: []string{"upstream rejected"},
		},
		{
			name: "named secret in a json body",
			in:   `login failed: {"email":"ana@example.com","password":"hunter2trombone"}`,
			gone: []string{"hunter2trombone", "ana@example.com"},
			kept: []string{"login failed"},
		},
		{
			name: "client contact details",
			in:   "reminder failed for ana.perez@gmail.com / +54 9 11 5555 4444 on booking",
			gone: []string{"ana.perez@gmail.com", "5555 4444"},
			kept: []string{"reminder failed", "on booking"},
		},
		{
			name: "card-length digit run",
			in:   "declined for 4509953566233704 at gateway",
			gone: []string{"4509953566233704"},
			kept: []string{"declined for", "at gateway"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := scrubText(tc.in)

			for _, secret := range tc.gone {
				if strings.Contains(got, secret) {
					t.Errorf("%q survived scrubbing:\n%s", secret, got)
				}
			}
			for _, keep := range tc.kept {
				if !strings.Contains(got, keep) {
					t.Errorf("%q was removed, leaving nothing to diagnose from:\n%s", keep, got)
				}
			}
			if !strings.Contains(got, "redacted") {
				t.Errorf("want a visible marker where something was removed; got:\n%s", got)
			}
		})
	}
}

// A UUID is this schema's identifier for everything, and it is exactly what a
// support request is traced by. Scrubbing it would make the events useless.
func TestScrubTextKeepsTheIdentifiersAnIncidentIsTracedBy(t *testing.T) {
	const in = "booking 6ba7b810-9dad-11d1-80b4-00c04fd430c8 request_id=a1b2c3d4 failed"

	if got := scrubText(in); got != in {
		t.Errorf("nothing here is a secret; want it untouched:\nin:  %s\ngot: %s", in, got)
	}
}

// A provider that answers with a kilobyte of JSON does not get to decide how
// much of the quota, or of a reviewer's attention, its error consumes.
func TestAnOverlongValueIsTruncated(t *testing.T) {
	long := "context: " + strings.Repeat("x", 4096)

	got := scrubText(long)
	if len(got) > maxCapturedValue+64 {
		t.Errorf("want the value bounded near %d bytes; got %d", maxCapturedValue, len(got))
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("want the truncation stated; got the tail %q", got[max(0, len(got)-40):])
	}
	if !strings.HasPrefix(got, "context: ") {
		t.Error("want the beginning kept, which is where the message is")
	}
}

// ---------------------------------------------------------------------------
// The event
// ---------------------------------------------------------------------------

// A panic value is arbitrary: whatever the panicking code happened to hold.
func TestAPanicValueIsScrubbedBeforeItLeaves(t *testing.T) {
	event := send(t, &sentry.Event{
		Exception: []sentry.Exception{{
			Type:  "*errors.errorString",
			Value: `mp: API error 401: {"message":"invalid_token","token":"APP_USR-secretsecret-1"}`,
		}},
	})

	got := event.Exception[0].Value
	if strings.Contains(got, "APP_USR-secretsecret-1") || strings.Contains(got, "invalid_token") {
		t.Errorf("the provider body survived: %s", got)
	}
	if !strings.Contains(got, "mp: API error 401") {
		t.Errorf("want the status code kept, which is the diagnostic part: %s", got)
	}
}

// The session itself travels in these two headers.
func TestTheSessionHeadersNeverLeave(t *testing.T) {
	event := send(t, &sentry.Event{
		Request: &sentry.Request{
			Headers: map[string]string{
				"Cookie":        "access_token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig; csrf=abc",
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
				"X-CSRF-Token":  "abcdef0123456789",
				"User-Agent":    "Mozilla/5.0",
				"Content-Type":  "application/json",
			},
			Cookies:     "access_token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
			Data:        `{"email":"ana@example.com","password":"hunter2"}`,
			QueryString: "token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
		},
	})

	for _, banned := range []string{"Cookie", "Authorization", "X-CSRF-Token"} {
		if _, present := event.Request.Headers[banned]; present {
			t.Errorf("%s must not be sent to a third party", banned)
		}
	}
	for _, kept := range []string{"User-Agent", "Content-Type"} {
		if _, present := event.Request.Headers[kept]; !present {
			t.Errorf("%s carries no credential and is worth keeping", kept)
		}
	}
	if event.Request.Cookies != "" {
		t.Errorf("want the cookie jar dropped; got %q", event.Request.Cookies)
	}
	if event.Request.Data != "" {
		t.Errorf("want the request body dropped; got %q", event.Request.Data)
	}
	if strings.Contains(event.Request.QueryString, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("want the query string scrubbed; got %q", event.Request.QueryString)
	}
}

// The allowlist is written in canonical case; a header that arrives in any
// other case must be matched against the same entry rather than dropped for
// spelling, or silently kept for it.
func TestHeaderMatchingIsCaseInsensitive(t *testing.T) {
	event := send(t, &sentry.Event{
		Request: &sentry.Request{Headers: map[string]string{
			"user-agent":    "curl/8",
			"authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
		}},
	})

	if _, present := event.Request.Headers["user-agent"]; !present {
		t.Error("a lowercase User-Agent is still User-Agent")
	}
	if _, present := event.Request.Headers["authorization"]; present {
		t.Error("a lowercase Authorization is still the session")
	}
}

// The user id joins to our own records, which is where a support request
// starts. The email identifies a person to a third party for no gain.
func TestTheUserIsReducedToAnOpaqueID(t *testing.T) {
	event := send(t, &sentry.Event{User: sentry.User{
		ID:       "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		Email:    "ana@example.com",
		Username: "ana",
		Name:     "Ana Pérez",
	}})

	if event.User.ID == "" {
		t.Error("want the id kept: it is what joins the event to our own records")
	}
	if event.User.Email != "" || event.User.Username != "" || event.User.Name != "" {
		t.Errorf("want the person unidentifiable; got %+v", event.User)
	}
}

// Breadcrumbs are attached to whatever event comes next, so they leave the
// building on the same trip.
func TestBreadcrumbsAreScrubbed(t *testing.T) {
	crumb := scrubBreadcrumb(&sentry.Breadcrumb{
		Message: "POST /api/v1/auth/login for ana@example.com",
		Data:    map[string]any{"body": `{"password":"hunter2"}`, "status": 401},
	}, nil)

	if strings.Contains(crumb.Message, "ana@example.com") {
		t.Errorf("want the address gone; got %q", crumb.Message)
	}
	if body, _ := crumb.Data["body"].(string); strings.Contains(body, "hunter2") {
		t.Errorf("want the password gone; got %q", body)
	}
	if crumb.Data["status"] != 401 {
		t.Errorf("want non-string data untouched; got %v", crumb.Data["status"])
	}
}

// An event with nothing sensitive must come through intact, or the hook is
// just an event filter.
func TestAnOrdinaryEventSurvivesIntact(t *testing.T) {
	event := send(t, &sentry.Event{
		Message:     "slot lock expired before payment",
		Transaction: "POST /api/v1/book/:id",
		Tags:        map[string]string{"complex_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
	})

	if event.Message != "slot lock expired before payment" {
		t.Errorf("want the message untouched; got %q", event.Message)
	}
	if event.Transaction != "POST /api/v1/book/:id" {
		t.Errorf("want the transaction untouched; got %q", event.Transaction)
	}
	if event.Tags["complex_id"] != "6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Errorf("want the tag untouched; got %q", event.Tags["complex_id"])
	}
}

// ---------------------------------------------------------------------------
// The booking link credential (specs/booking-link-credential)
// ---------------------------------------------------------------------------

// TestBookingLinkURLIsScrubbed is task 10.3, the load-bearing regression
// design.md's Decision 4(a) names: the URL is built through booklink.Cancel,
// not a hardcoded string, so renaming internal/booklink's QueryParam changes
// the URL this test actually exercises — a literal "?token=..." in the test
// body would keep passing after a rename that silently undoes the scrubbing.
//
// Mutation, run and recorded: rename QueryParam to "booking_token" — the URL
// booklink.Cancel builds changes, the named-secret rule's `\btoken\b`
// alternative stops matching (an underscore is a word character), the
// plaintext survives scrubbing, and this test must fail. Restore
// QueryParam = "token" — re-run, passes.
func TestBookingLinkURLIsScrubbed(t *testing.T) {
	const knownToken = "3xzP9vLg7hK2mQ8wR5tY1nB4cD6fH0jS-AbCdEfGhIj"

	link := booklink.Cancel("https://vibe.test", "my-complex", knownToken)
	parsed, err := neturl.Parse(link)
	if err != nil {
		t.Fatalf("booklink.Cancel produced an unparseable URL: %v", err)
	}

	event := send(t, &sentry.Event{
		Request: &sentry.Request{
			URL:         link,
			QueryString: parsed.RawQuery,
		},
	})

	if strings.Contains(event.Request.URL, knownToken) {
		t.Errorf("the plaintext token survived scrubbing in Request.URL: %s", event.Request.URL)
	}
	if strings.Contains(event.Request.QueryString, knownToken) {
		t.Errorf("the plaintext token survived scrubbing in Request.QueryString: %s", event.Request.QueryString)
	}
}

// TestDifferentlyNamedTokenParameterIsNotScrubbed is task 10.4, the
// companion negative test: it proves *why* 10.3's mutation must fail. The
// value is a string literal standing in for what booklink.Cancel would build
// if QueryParam were ever renamed to "booking_token" — the underscore makes
// the named-secret rule's `\btoken\b` alternative not match inside it, so
// nothing in scrubText's rule list catches this parameter name.
func TestDifferentlyNamedTokenParameterIsNotScrubbed(t *testing.T) {
	const value = "3xzP9vLg7hK2mQ8wR5tY1nB4cD6fH0jS-AbCdEfGhIjKlMnOpQrStUvWxYz012345"
	in := "https://vibe.test/my-complex/book/status?booking_token=" + value

	got := scrubText(in)

	if !strings.Contains(got, value) {
		t.Errorf("want booking_token's value left untouched — this proves the parameter must be named "+
			"exactly %q, not a longer variant; got %q", booklink.QueryParam, got)
	}
}
