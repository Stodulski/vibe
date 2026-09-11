package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// ---------------------------------------------------------------------------
// RecoverPanic
// ---------------------------------------------------------------------------

// The panic log line is the one that matters most, and it was the one line
// that could not be joined to anything: RecoverPanic sat outside RequestID, so
// the id was not in the context yet when the Responder stamped the entry —
// while the client was already holding it from the X-Request-ID header.
func TestAPanicIsLoggedWithTheRequestIDTheClientWasGiven(t *testing.T) {
	f := newFixture(t, Config{})

	handler := f.mw.RequestID(f.mw.RecoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("something went wrong")
	})))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("the response must carry a request id")
	}
	if got := f.logs.String(); !strings.Contains(got, "request_id="+id) {
		t.Errorf("the panic log line must carry the id the client was given (%s); got %s", id, got)
	}
}

// The log line is only half of the recovered path. An operator is paged on the
// Sentry event, and it went out through the process-global hub with no request
// scope at all — so the id the client is holding in X-Request-ID, and quoting
// in the ticket, matched nothing in the place the panic is actually triaged.
func TestAPanicIsReportedToSentryWithTheRequestIDTheClientWasGiven(t *testing.T) {
	events := captureSentryEvents(t)
	f := newFixture(t, Config{})

	handler := f.mw.RequestID(f.mw.RecoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("something went wrong")
	})))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("the response must carry a request id")
	}

	got := events.all()
	if len(got) != 1 {
		t.Fatalf("the panic must be reported to Sentry exactly once; got %d events", len(got))
	}
	if got[0].Tags["request_id"] != id {
		t.Errorf("the Sentry event must be findable by the id the client was given (%s); tags were %v",
			id, got[0].Tags)
	}
}

// sentry.CurrentHub is one object shared by every request in flight, so the id
// has to be attached to a clone. Tagging the global scope leaks one request's
// id onto whatever is reported next — a correlation id pointing at the wrong
// request is worse than no id, because it gets believed.
func TestAPanicDoesNotLeaveItsRequestIDOnTheProcessGlobalSentryScope(t *testing.T) {
	events := captureSentryEvents(t)
	f := newFixture(t, Config{})

	handler := f.mw.RequestID(f.mw.RecoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("something went wrong")
	})))
	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	// Anything reported afterwards belongs to no request at all.
	sentry.CaptureMessage("an unrelated report from elsewhere in the process")

	got := events.all()
	if len(got) != 2 {
		t.Fatalf("want the panic and the unrelated report; got %d events", len(got))
	}
	if id, tagged := got[1].Tags["request_id"]; tagged {
		t.Errorf("the id belongs to the request that panicked, not to the process; it stuck as %q", id)
	}
}

// captureSentryEvents binds a client that keeps every event in memory to the
// hub RecoverPanic reports through, and restores what was there afterwards.
func captureSentryEvents(t *testing.T) *sentryTransport {
	t.Helper()

	transport := &sentryTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	if err != nil {
		t.Fatalf("sentry test client: %v", err)
	}

	hub := sentry.CurrentHub()
	previous := hub.Client()
	hub.BindClient(client)
	t.Cleanup(func() { hub.BindClient(previous) })

	return transport
}

// sentryTransport records the events a test's panic produced.
type sentryTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (tr *sentryTransport) SendEvent(event *sentry.Event) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.events = append(tr.events, event)
}

func (tr *sentryTransport) all() []*sentry.Event {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.events)
}

func (tr *sentryTransport) Configure(sentry.ClientOptions)        {}
func (tr *sentryTransport) Flush(time.Duration) bool              { return true }
func (tr *sentryTransport) FlushWithContext(context.Context) bool { return true }
func (tr *sentryTransport) Close()                                {}

// http.ErrAbortHandler is the documented way for a handler to abandon a
// response on purpose. net/http suppresses its own log line for it and closes
// the connection; recovering it turns a deliberate abort into a 500 and hides
// it from the runtime.
func TestRecoverPanicDoesNotSwallowErrAbortHandler(t *testing.T) {
	f := newFixture(t, Config{})

	handler := f.mw.RecoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	w := httptest.NewRecorder()

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	}()

	if err, ok := recovered.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
		t.Errorf("ErrAbortHandler must reach net/http unchanged; got %v", recovered)
	}
	if w.Body.Len() != 0 {
		t.Errorf("an aborted handler must not have an error body written for it; got %s", w.Body.String())
	}
}

// A panic after the response has started cannot be answered with a 500: the
// status line and part of the body are already on the wire, and appending a
// JSON error produces a response that parses as a success with garbage stapled
// to the end. The only honest signal left is closing the connection, which is
// what re-panicking with ErrAbortHandler does.
func TestAPanicAfterTheResponseStartedDoesNotAppendAnErrorBody(t *testing.T) {
	f := newFixture(t, Config{})

	handler := f.mw.RecoverPanic(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"bookings":[`))
		panic("the store gave up half way through")
	}))

	w := httptest.NewRecorder()

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	}()

	if err, ok := recovered.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
		t.Errorf("a panic mid-response must abort the connection; got %v", recovered)
	}
	if got := w.Body.String(); got != `{"bookings":[` {
		t.Errorf("nothing may be appended to a response already in flight; got %s", got)
	}
	if w.Code != http.StatusOK {
		t.Errorf("the status the handler already sent stands; got %d", w.Code)
	}
	if f.logs.Len() == 0 {
		t.Error("a panic that could not be answered must still be logged")
	}
}

// The wrapper RecoverPanic puts around the writer must not break the event
// stream, which type-asserts its writer to http.Flusher and answers 500 when
// the assertion fails.
func TestTheRecoveryWrapperStaysFlushable(t *testing.T) {
	f := newFixture(t, Config{})

	var flushable bool
	handler := f.mw.RecoverPanic(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, flushable = w.(http.Flusher)
	}))

	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if !flushable {
		t.Error("the SSE handler asserts http.Flusher on the writer it is given")
	}
}

// ---------------------------------------------------------------------------
// CSRF exemptions
// ---------------------------------------------------------------------------

// The exemptions were subtrees, and one of them was a bare string prefix. Both
// grant themselves to routes that do not exist yet: whoever adds the next
// endpoint under "/api/v1/book" — or, with the bare prefix, "/api/v1/bookmarks"
// — inherits the exemption without ever opening chain.go, and the symptom is a
// CSRF hole rather than a failing test.
//
// One exact route per entry, method included, so nothing is inherited.
func TestACSRFExemptionCoversOneExactRouteAndNothingUnderIt(t *testing.T) {
	tests := []struct {
		method string
		path   string
		exempt bool
		why    string
	}{
		{http.MethodPost, "/api/v1/book", true, "public booking, no cookie in play"},
		{http.MethodPost, "/api/v1/book/cancel", true, "same flow, its own link token"},
		{http.MethodPost, "/api/v1/book/reschedule", false, "the route under it that does not exist yet"},
		{http.MethodPost, "/api/v1/book/status", false, "registered as a GET; a POST there is not an exempted route"},
		{http.MethodPost, "/api/v1/bookmarks", false, "a different route that happens to share five characters"},
		{http.MethodPost, "/api/v1/bookings/reassign", false, "a different route that happens to share five characters"},

		{http.MethodPost, "/api/v1/webhooks/mercadopago", true, "signature-authenticated, no cookie"},
		{http.MethodPost, "/api/v1/webhooks/whatsapp", true, "signature-authenticated, no cookie"},
		{http.MethodPost, "/api/v1/webhooks/stripe", false, "the next webhook must be added deliberately, not inherit"},
		{http.MethodPost, "/api/v1/webhooksomething", false, "not a webhook route at all"},

		{http.MethodPost, "/api/v1/public/complexes/x", false, "the public tree is read-only; a write under it is not pre-approved"},
		{http.MethodPost, "/api/v1/publications", false, "not under the public tree"},

		{http.MethodPost, "/api/v1/auth/login", true, "mints the token it would otherwise have to echo"},
		{http.MethodPut, "/api/v1/auth/login", false, "the exemption is for the method that is registered, not for the path"},
		{http.MethodPost, "/api/v1/auth/login/extra", false, "the login exemption is that exact route"},
		{http.MethodDelete, "/api/v1/auth/me", false, "an ordinary cookie-authenticated write"},
		{http.MethodPost, "/api/v1/complexes/x/courts", false, "an ordinary cookie-authenticated write"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			f := newFixture(t, Config{})
			f.tokens.csrfValid = false // no valid token is presented

			var reached bool
			r := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			r.AddCookie(&http.Cookie{Name: "access_token", Value: "a-session"})

			w := httptest.NewRecorder()
			f.mw.CSRFProtect(ok(&reached)).ServeHTTP(w, r)

			if tt.exempt && !reached {
				t.Errorf("%s %s must be exempt (%s); got %d", tt.method, tt.path, tt.why, w.Code)
			}
			if !tt.exempt && reached {
				t.Errorf("%s %s must be challenged (%s); it was let through", tt.method, tt.path, tt.why)
			}
		})
	}
}

// A read is exempt by method, everywhere, so no read belongs in the table.
// An entry for one is either dead weight or a misunderstanding of what the
// table does.
func TestNoExemptionIsWrittenForAMethodThatNeverReachesTheCheck(t *testing.T) {
	for route := range CSRFExemptRoutes() {
		method, _, _ := strings.Cut(route, " ")
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			t.Errorf("%q is exempt by method before the table is consulted; remove the entry", route)
		}
	}
}

// Both tables match a request path literally, and a request path is always
// concrete. An entry carrying a router parameter would match nothing and read,
// to whoever added it, exactly like an exemption that works.
//
// This is the guard on the shape of the table rather than on its contents: it
// fails the moment somebody needs an exemption the matcher cannot express,
// which is the moment to build parameter matching rather than to discover
// later that the route was never exempt.
func TestNoExemptionNamesARouteParameter(t *testing.T) {
	for name, table := range map[string]map[string]string{
		"CSRF":       CSRFExemptRoutes(),
		"rate limit": RateLimitExemptRoutes(),
	} {
		for route := range table {
			if strings.Contains(route, "/:") || strings.Contains(route, "/*") {
				t.Errorf("%s exemption %q names a router parameter, which no request path contains, "+
					"so the entry matches nothing", name, route)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Caching of credentialed responses
// ---------------------------------------------------------------------------

// A response derived from a credential is per-identity. Vary alone does not
// stop a browser restoring the previous account's page from the back-forward
// cache after a logout.
func TestACredentialedRequestGetsNoStore(t *testing.T) {
	f := newFixture(t, Config{})
	userID := uuid.New()
	f.tokens.claims = validClaims(userID)
	f.users.user = &data.User{ID: userID, Role: "owner", IsActive: true}

	var reached bool
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer some-token")

	w := httptest.NewRecorder()
	f.mw.Authenticate(ok(&reached)).ServeHTTP(w, r)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("an authenticated response must not be stored; Cache-Control was %q", got)
	}
}

// An anonymous request to a public route is cacheable, and marking it no-store
// would throw away the public site's caching for nothing.
func TestAnAnonymousRequestIsNotMarkedNoStore(t *testing.T) {
	f := newFixture(t, Config{})

	var reached bool
	w := httptest.NewRecorder()
	f.mw.Authenticate(ok(&reached)).ServeHTTP(w,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/public/complexes/x", nil))

	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("a public anonymous response must stay cacheable; Cache-Control was %q", got)
	}
}

// A 401 stops being true the moment the user signs in again; a cached one
// locks them out of a session they have already fixed.
func TestErrorResponsesAreNotStored(t *testing.T) {
	f := newFixture(t, Config{})

	var reached bool
	w := httptest.NewRecorder()
	f.mw.RequireAuth(ok(&reached))(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401; got %d", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("an error response must not be stored; Cache-Control was %q", got)
	}
}

// ---------------------------------------------------------------------------
// Deadlines
// ---------------------------------------------------------------------------

// Both reads the chain performs before a handler runs used the bare request
// context, which has no deadline: http.Server's WriteTimeout closes the
// connection but does not cancel the handler, so a stalled read held a pool
// connection with nothing left to notice it.
func TestTheChainsDatabaseReadsCarryADeadline(t *testing.T) {
	t.Run("the account behind a session", func(t *testing.T) {
		f := newFixture(t, Config{})
		userID := uuid.New()
		f.tokens.claims = validClaims(userID)
		f.users.user = &data.User{ID: userID, Role: "owner", IsActive: true}

		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer some-token")
		f.mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
			ServeHTTP(httptest.NewRecorder(), r)

		assertBounded(t, f.users.deadline)
	})

	t.Run("the complex an ownership-scoped route names", func(t *testing.T) {
		f := newFixture(t, Config{})
		ownerID, complexID := uuid.New(), uuid.New()
		f.complexes.complex = &data.Complex{ID: complexID, OwnerID: ownerID}

		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		r = httpx.ContextSetUser(r, &data.User{ID: ownerID, Role: "owner"})
		r = withComplexParam(r, complexID)

		f.mw.RequireComplexOwner(func(http.ResponseWriter, *http.Request) {})(httptest.NewRecorder(), r)

		assertBounded(t, f.complexes.deadline)
	})
}

// assertBounded fails when a read ran on a context with no deadline at all.
func assertBounded(t *testing.T, got time.Duration) {
	t.Helper()
	if got <= 0 {
		t.Fatal("the read ran on a context with no deadline at all")
	}
	if got > identityQueryTimeout {
		t.Errorf("the budget must be at most %s; got %s", identityQueryTimeout, got)
	}
}

// ---------------------------------------------------------------------------
// The in-process client table
// ---------------------------------------------------------------------------

// The map was unbounded. Its key is a parsed address now, so filling it costs
// real source addresses — but expensive is not bounded, and the failure mode
// of unbounded is an out-of-memory kill, which refuses everybody permanently.
//
// At capacity it sheds addresses it has not seen, rather than evicting one it
// has: evicting to make room is what a rotating flood wants, since it pushes
// every real client out and hands each of them a fresh full bucket.
func TestTheInProcessClientTableIsBounded(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	buckets := newLocalBuckets([3]ceiling{
		{name: "gen", rps: 1000, burst: 1000},
		authCeiling,
		bookingCeiling,
	}, logger)
	buckets.max = 2

	for _, ip := range []string{"203.0.113.1", "203.0.113.2"} {
		if !buckets.allow(ip, indexGeneral) {
			t.Fatalf("%s is within capacity and must be tracked", ip)
		}
	}

	if buckets.allow("203.0.113.3", indexGeneral) {
		t.Error("an address arriving at a full table must be shed, not admitted")
	}
	// The addresses already tracked keep working: shedding protects them, it
	// does not join them.
	if !buckets.allow("203.0.113.1", indexGeneral) {
		t.Error("an address already in the table must keep its bucket while the table is full")
	}
	if got := buckets.size.Load(); got != 2 {
		t.Errorf("the table must not grow past its capacity; holds %d", got)
	}
}

// The panic log line has to carry the request id through the chain the
// application actually builds, not through a composition a test wrote. Wrap is
// where the order lives, and the order is the fix.
func TestTheFullChainLogsAPanicWithTheClientsRequestID(t *testing.T) {
	f := newFixture(t, Config{})

	noCORS := func(next http.Handler) http.Handler { return next }
	handler := f.mw.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("the store gave up")
	}), noCORS)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/healthcheck", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500; got %d", w.Code)
	}

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("the client must be given a request id to quote")
	}
	if got := f.logs.String(); !strings.Contains(got, "request_id="+id) {
		t.Errorf("the panic the client is holding an id for must be findable by that id (%s); got %s", id, got)
	}
}
