package realtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// mustSubscribe registers a client and fails the test if the hub refused it,
// so a test that is not about admission cannot silently proceed with a nil
// client.
func mustSubscribe(t *testing.T, h *Hub, complexID uuid.UUID) *client {
	t.Helper()
	c, ok := h.Subscribe(complexID)
	if !ok {
		t.Fatalf("hub refused a subscription for %s", complexID)
	}
	return c
}

// allowAll is the authorizer for tests that are not about authorization.
type allowAll struct{}

func (allowAll) Authorize(context.Context, *http.Request, uuid.UUID) error { return nil }

// recordingAuthorizer answers whatever the test queued and records what it was
// actually asked.
//
// It records the complex id and the caller's credential rather than only
// counting calls: a stub that discarded its arguments could not tell a re-check
// bound to the stream's own complex from one bound to whatever the client last
// put in the URL, and could not tell a re-check that carries the caller's
// session from one that carries nothing and would therefore pass or fail for
// the wrong reason.
type recordingAuthorizer struct {
	mu       sync.Mutex
	verdicts []error // consumed in order; the last one repeats
	calls    int
	complex  []uuid.UUID
	creds    []string
}

func (a *recordingAuthorizer) Authorize(_ context.Context, r *http.Request, complexID uuid.UUID) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.calls++
	a.complex = append(a.complex, complexID)
	cred := ""
	if c, err := r.Cookie("access_token"); err == nil {
		cred = c.Value
	}
	a.creds = append(a.creds, cred)

	if len(a.verdicts) == 0 {
		return nil
	}
	v := a.verdicts[0]
	if len(a.verdicts) > 1 {
		a.verdicts = a.verdicts[1:]
	}
	return v
}

func (a *recordingAuthorizer) observed() (calls int, complexes []uuid.UUID, creds []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls, append([]uuid.UUID(nil), a.complex...), append([]string(nil), a.creds...)
}

// streamFixture runs one Stream over a real HTTP server, as an SSE client sees
// it.
type streamFixture struct {
	hub       *Hub
	complexID uuid.UUID
	resp      *http.Response

	mu   sync.Mutex
	body strings.Builder
	done chan struct{}
}

// startStream opens a stream and reads it in the background until the server
// closes it. The request carries a session cookie so the authorizer stub has a
// credential to observe.
func startStream(t *testing.T, auth Authorizer, cfg Config) *streamFixture {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	hub := NewHub(nil, logger, "test")
	t.Cleanup(hub.Shutdown)

	f := &streamFixture{hub: hub, complexID: uuid.New(), done: make(chan struct{})}
	h := NewHandler(hub, auth, httpx.NewResponder(logger), logger, make(chan struct{}), cfg)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Stream(w, httpx.ContextSetComplex(r, &complexstore.Complex{ID: f.complexID}))
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "the-session"})

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	f.resp = resp
	t.Cleanup(func() { _ = resp.Body.Close() })

	go func() {
		defer close(f.done)
		buf := make([]byte, 256)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				f.mu.Lock()
				f.body.Write(buf[:n])
				f.mu.Unlock()
			}
			if rerr != nil {
				return
			}
		}
	}()

	return f
}

func (f *streamFixture) received() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.body.String()
}

// waitForClose waits for the server to end the stream.
func (f *streamFixture) waitForClose(t *testing.T) {
	t.Helper()
	select {
	case <-f.done:
	case <-time.After(3 * time.Second):
		t.Fatalf("the stream was never closed; received so far: %q", f.received())
	}
}

// A stream authorises once, at connect, and then outlives that decision. An
// owner who is deactivated, signed out, or no longer owns the venue must stop
// receiving its booking events.
func TestStreamEndsWhenAuthorizationIsRevoked(t *testing.T) {
	auth := &recordingAuthorizer{verdicts: []error{ErrStreamUnauthorized}}

	f := startStream(t, auth, Config{RecheckInterval: 20 * time.Millisecond, MaxLifetime: time.Hour})
	f.waitForClose(t)

	body := f.received()
	if !strings.Contains(body, "event: unauthorized") {
		t.Errorf("a revoked stream must say so before closing; got %q", body)
	}

	calls, complexes, creds := auth.observed()
	if calls == 0 {
		t.Fatal("authorization was never re-checked")
	}
	// The re-check must be bound to the stream's own complex, not to anything
	// the client can influence after connecting.
	for _, got := range complexes {
		if got != f.complexID {
			t.Errorf("re-checked complex %s; want the stream's own %s", got, f.complexID)
		}
	}
	// And it must be handed the caller's credential, or it is deciding about
	// nobody.
	for _, got := range creds {
		if got != "the-session" {
			t.Errorf("authorizer saw credential %q; want the caller's own", got)
		}
	}
}

// The frame matters as much as the closing: a stream that just stops looks
// exactly like a dropped connection, and an EventSource answers that by
// reconnecting forever.
func TestRevokedStreamNamesTheReasonItClosed(t *testing.T) {
	auth := &recordingAuthorizer{verdicts: []error{ErrStreamUnauthorized}}

	f := startStream(t, auth, Config{RecheckInterval: 20 * time.Millisecond, MaxLifetime: time.Hour})
	f.waitForClose(t)

	body := f.received()
	if !strings.Contains(body, `"reason"`) {
		t.Errorf("the closing frame must carry a reason the frontend can act on; got %q", body)
	}
	if strings.Contains(body, "event: expired") {
		t.Errorf("a revocation must not be reported as a routine expiry; got %q", body)
	}
}

// An authorized caller must keep their stream. This is the mutation guard on
// the test above: a handler that closed every stream would pass it.
func TestStreamSurvivesRepeatedSuccessfulRechecks(t *testing.T) {
	auth := &recordingAuthorizer{} // no queued verdict: always allowed

	f := startStream(t, auth, Config{RecheckInterval: 20 * time.Millisecond, MaxLifetime: time.Hour})

	select {
	case <-f.done:
		t.Fatalf("an authorized stream was closed; received %q", f.received())
	case <-time.After(200 * time.Millisecond):
	}

	if calls, _, _ := auth.observed(); calls < 2 {
		t.Errorf("want the stream re-checked repeatedly; got %d calls", calls)
	}
	if body := f.received(); !strings.Contains(body, "event: connected") {
		t.Errorf("want the opening frame; got %q", body)
	}
}

// No stream outlives the credential that opened it.
func TestStreamEndsAtItsMaximumLifetime(t *testing.T) {
	f := startStream(t, allowAll{}, Config{RecheckInterval: time.Hour, MaxLifetime: 50 * time.Millisecond})
	f.waitForClose(t)

	body := f.received()
	if !strings.Contains(body, "event: expired") {
		t.Errorf("want an expiry frame telling the client to reconnect; got %q", body)
	}
	if strings.Contains(body, "event: unauthorized") {
		t.Errorf("a routine expiry must not read as a revocation; got %q", body)
	}
}

// A check that cannot reach a verdict is not a denial: the database being
// briefly unreachable must not disconnect every dashboard at once.
func TestStreamToleratesAnUnverifiableRecheck(t *testing.T) {
	auth := &recordingAuthorizer{verdicts: []error{errors.New("database unreachable"), nil}}

	f := startStream(t, auth, Config{RecheckInterval: 20 * time.Millisecond, MaxLifetime: time.Hour})

	select {
	case <-f.done:
		t.Fatalf("one failed re-check closed the stream; received %q", f.received())
	case <-time.After(200 * time.Millisecond):
	}
}

// Tolerated, but not forever: an outage must not become the way to keep a
// stream nobody can vouch for.
func TestStreamEndsAfterRepeatedUnverifiableRechecks(t *testing.T) {
	auth := &recordingAuthorizer{verdicts: []error{fmt.Errorf("database unreachable")}}

	f := startStream(t, auth, Config{RecheckInterval: 20 * time.Millisecond, MaxLifetime: time.Hour})
	f.waitForClose(t)

	if calls, _, _ := auth.observed(); calls < maxRecheckFailures {
		t.Errorf("want at least %d attempts before giving up; got %d", maxRecheckFailures, calls)
	}
	if body := f.received(); !strings.Contains(body, "event: expired") {
		t.Errorf("want the client told to reconnect, where a status code can be returned; got %q", body)
	}
}

// An unwired authorizer must show up as streams that end, not as streams
// nobody is re-authorizing.
func TestStreamWithNoAuthorizerFailsClosed(t *testing.T) {
	f := startStream(t, nil, Config{RecheckInterval: 20 * time.Millisecond, MaxLifetime: time.Hour})
	f.waitForClose(t)

	if body := f.received(); !strings.Contains(body, "event: unauthorized") {
		t.Errorf("want a stream with no authorizer refused; got %q", body)
	}
}

// The cap is refused before the response is committed, so the client gets a
// status code instead of a stream that opens and dies.
func TestStreamRefusesAConnectionOverTheCap(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	hub := NewHub(nil, logger, "test")
	t.Cleanup(hub.Shutdown)
	hub.perComplexLimit = 1

	complexID := uuid.New()
	mustSubscribe(t, hub, complexID) // the only slot this complex has

	h := NewHandler(hub, allowAll{}, httpx.NewResponder(logger), logger, make(chan struct{}), Config{})

	// The request carries its own short deadline so that a handler which
	// admitted the connection returns and is judged on what it wrote, instead
	// of blocking this test in its event loop for as long as the suite allows.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	w := httptest.NewRecorder()
	r := httpx.ContextSetComplex(
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil),
		&complexstore.Complex{ID: complexID})
	h.Stream(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("want 429 once the complex is at its stream cap; got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); strings.Contains(got, "text/event-stream") {
		t.Errorf("a refused connection must not open a stream; content type %q", got)
	}
	if strings.Contains(w.Body.String(), "event: connected") {
		t.Errorf("a refused connection must not be sent the connected frame; got %q", w.Body.String())
	}
}

func TestHubRefusesBeyondThePerComplexLimit(t *testing.T) {
	hub := NewHub(nil, testLogger(), "test")
	t.Cleanup(hub.Shutdown)
	hub.perComplexLimit = 2

	complexID := uuid.New()
	mustSubscribe(t, hub, complexID)
	second := mustSubscribe(t, hub, complexID)

	if _, ok := hub.Subscribe(complexID); ok {
		t.Fatal("the hub admitted a third stream past a limit of two")
	}

	// Another complex is unaffected: the limit is per tenant.
	if _, ok := hub.Subscribe(uuid.New()); !ok {
		t.Error("one complex at its limit blocked a different complex")
	}

	// And capacity comes back when a connection ends.
	hub.Unsubscribe(complexID, second)
	if _, ok := hub.Subscribe(complexID); !ok {
		t.Error("a disconnected stream did not free its slot")
	}
}

func TestHubRefusesBeyondTheProcessLimit(t *testing.T) {
	hub := NewHub(nil, testLogger(), "test")
	t.Cleanup(hub.Shutdown)
	hub.totalLimit = 2

	first := mustSubscribe(t, hub, uuid.New())
	firstComplex := onlyComplex(t, hub)
	mustSubscribe(t, hub, uuid.New())

	if _, ok := hub.Subscribe(uuid.New()); ok {
		t.Fatal("the hub admitted a third stream past a process limit of two")
	}

	hub.Unsubscribe(firstComplex, first)
	if _, ok := hub.Subscribe(uuid.New()); !ok {
		t.Error("a disconnected stream did not free process capacity")
	}
}

// A repeated Unsubscribe must not hand back capacity that was never taken.
func TestUnsubscribingTwiceDoesNotInventCapacity(t *testing.T) {
	hub := NewHub(nil, testLogger(), "test")
	t.Cleanup(hub.Shutdown)
	hub.totalLimit = 1

	complexID := uuid.New()
	c := mustSubscribe(t, hub, complexID)
	hub.Unsubscribe(complexID, c)
	hub.Unsubscribe(complexID, c)

	if _, ok := hub.Subscribe(uuid.New()); !ok {
		t.Fatal("the single slot was not free")
	}
	if _, ok := hub.Subscribe(uuid.New()); ok {
		t.Error("the double unsubscribe invented a second slot")
	}
}

// onlyComplex returns the id of the hub's single registered complex.
func onlyComplex(t *testing.T, h *Hub) uuid.UUID {
	t.Helper()
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) != 1 {
		t.Fatalf("want exactly one registered complex; got %d", len(h.clients))
	}
	for id := range h.clients {
		return id
	}
	return uuid.Nil
}
