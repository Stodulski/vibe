package bookings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

// captureTransport is a sentry.Transport that records every event handed to
// it instead of sending anything over the network. sentry.Client.processEvent
// calls Transport.SendEvent synchronously (no background worker involved),
// so an event recorded here is visible to the test immediately after the
// call to sentry.CaptureMessage returns.
type captureTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (c *captureTransport) Flush(_ time.Duration) bool              { return true }
func (c *captureTransport) FlushWithContext(_ context.Context) bool { return true }
func (c *captureTransport) Configure(_ sentry.ClientOptions)        {}
func (c *captureTransport) Close()                                  {}
func (c *captureTransport) SendEvent(event *sentry.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *captureTransport) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

// messages returns the message of every captured event, so a test can assert
// on what an operator would actually read rather than on a bare count.
func (c *captureTransport) messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.events))
	for _, e := range c.events {
		out = append(out, e.Message)
	}
	return out
}

// findMessage returns the first captured message containing needle.
func findMessage(messages []string, needle string) (string, bool) {
	for _, m := range messages {
		if strings.Contains(m, needle) {
			return m, true
		}
	}
	return "", false
}

// withCapturedSentryEvents binds a client backed by captureTransport to the
// current hub for the duration of the test, and restores whatever client was
// bound before (nil in every test today, since nothing else in this suite
// initializes Sentry). It returns the transport so the test can assert on
// what was captured.
func withCapturedSentryEvents(t *testing.T) *captureTransport {
	t.Helper()

	tr := &captureTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Dsn: "", Transport: tr})
	if err != nil {
		t.Fatalf("building a test sentry client: %v", err)
	}

	hub := sentry.CurrentHub()
	previous := hub.Client()
	hub.BindClient(client)
	t.Cleanup(func() { hub.BindClient(previous) })

	return tr
}

// errStoreUnavailable stands in for the database being unreachable while a
// public route is resolving a token — distinct from data.ErrRecordNotFound,
// which resolveLink maps to 404, not a captured error.
var errStoreUnavailable = errors.New("database unavailable")

// TestPublicRoutesCaptureNothing is task 12.2 (design.md Decision 4(b),
// specs/booking-link-credential's last requirement): a server error on any of
// the three public routes must reach Sentry through nothing. Every capture
// site in this package lives in PublicBook/createMPPreferenceWithRetry, and
// none of it is on these three routes' path.
//
// Mutation, run and recorded: add
// `sentry.CaptureMessage(fmt.Sprintf("... booking_id=%s", token))` to any one
// of the three routes (or to resolveLink, which all three share) — re-run,
// and the failing route's count() != 0 assertion must fail.
func TestPublicRoutesCaptureNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(f *fixture) *httptest.ResponseRecorder
	}{
		{"status", func(f *fixture) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=x", ""))
			return w
		}},
		{"cancel-info", func(f *fixture) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet, "/?token=x", ""))
			return w
		}},
		{"cancel", func(f *fixture) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"x"}`))
			return w
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.linkResolver.err = errStoreUnavailable
			tr := withCapturedSentryEvents(t)

			w := tc.call(f)

			if w.Code != http.StatusInternalServerError {
				t.Fatalf("want the forced store failure to surface as 500; got %d (%s)", w.Code, w.Body.String())
			}
			if tr.count() != 0 {
				t.Errorf("a store failure on this route must capture nothing to Sentry; got %d event(s): %v", tr.count(), tr.messages())
			}
		})
	}
}
