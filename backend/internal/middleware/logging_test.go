package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// logFixture is a Middleware whose logger writes JSON into a buffer, so a test
// can assert on the fields of a line rather than on a substring of it.
//
// Substring assertions are what let a middleware that logs `duration_ms=0`
// pass a test for "the duration is logged".
type logFixture struct {
	mw   *Middleware
	logs *bytes.Buffer
}

func newLogFixture(t *testing.T, cfg Config) *logFixture {
	t.Helper()

	f := &logFixture{logs: &bytes.Buffer{}}
	logger := slog.New(slog.NewJSONHandler(f.logs, nil))
	f.mw = New(Dependencies{
		Users: &stubUsers{}, Complexes: &stubComplexes{},
		Tokens: &stubTokens{csrfValid: true}, Blacklist: &stubBlacklist{},
		Respond: httpx.NewResponder(logger), Logger: logger, Shutdown: make(chan struct{}),
	}, cfg)
	return f
}

// requestLines returns every "request" record the fixture logged, decoded.
func (f *logFixture) requestLines(t *testing.T) []map[string]any {
	t.Helper()

	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(f.logs.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not JSON: %v\n%s", err, line)
		}
		if record["msg"] == "request" {
			out = append(out, record)
		}
	}
	return out
}

// onlyLine returns the single request line, failing if there is not exactly one.
func (f *logFixture) onlyLine(t *testing.T) map[string]any {
	t.Helper()

	lines := f.requestLines(t)
	if len(lines) != 1 {
		t.Fatalf("want exactly one request line; got %d\n%s", len(lines), f.logs.String())
	}
	return lines[0]
}

// num reads a numeric field, failing when it is missing or not a number.
func num(t *testing.T, record map[string]any, key string) float64 {
	t.Helper()

	value, present := record[key]
	if !present {
		t.Fatalf("the log line has no %q field: %v", key, record)
	}
	n, ok := value.(float64)
	if !ok {
		t.Fatalf("want %q to be a number; got %T (%v)", key, value, value)
	}
	return n
}

// str reads a string field.
func str(t *testing.T, record map[string]any, key string) string {
	t.Helper()

	value, present := record[key]
	if !present {
		t.Fatalf("the log line has no %q field: %v", key, record)
	}
	s, ok := value.(string)
	if !ok {
		t.Fatalf("want %q to be a string; got %T (%v)", key, value, value)
	}
	return s
}

// serve sends one request through handler and returns the recorder.
func serve(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return w
}

// ---------------------------------------------------------------------------
// The line itself
// ---------------------------------------------------------------------------

// The failure this replaces: a request that merely took ten seconds, or queued
// behind an exhausted pool and eventually succeeded, produced no output at all.
func TestEveryRequestLeavesOneLineNamingWhatHappened(t *testing.T) {
	f := newLogFixture(t, Config{})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	serve(t, handler, http.MethodPost, "/api/v1/complexes")

	line := f.onlyLine(t)
	if got := str(t, line, "method"); got != http.MethodPost {
		t.Errorf("want method POST; got %q", got)
	}
	if got := str(t, line, "route"); got != "/api/v1/complexes" {
		t.Errorf("want the route; got %q", got)
	}
	if got := num(t, line, "status"); got != http.StatusCreated {
		t.Errorf("want the status the handler wrote (201); got %v", got)
	}
	if got := num(t, line, "bytes"); got != 10 {
		t.Errorf("want the 10 bytes the handler wrote; got %v", got)
	}
}

// A duration field that is always zero passes any test for "the duration is
// logged", so this asserts the number rather than the field.
//
// It is also why the field is a float: this API's fast paths answer in tens of
// microseconds, and an integer-millisecond duration reports every one of them
// as 0 — indistinguishable from a middleware that never measured anything.
func TestTheLoggedDurationIsTheTimeTheRequestActuallyTook(t *testing.T) {
	f := newLogFixture(t, Config{})

	const slept = 25 * time.Millisecond
	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(slept)
		w.WriteHeader(http.StatusOK)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/healthcheck")

	got := num(t, f.onlyLine(t), "duration_ms")
	if got < float64(slept.Milliseconds()) {
		t.Errorf("want at least %d ms, the time the handler actually took; got %v", slept.Milliseconds(), got)
	}
	if got > 5000 {
		t.Errorf("want a duration in milliseconds; got %v, which looks like another unit", got)
	}
}

// A sub-millisecond request must not be logged as zero. Same argument as
// above, from the other end.
func TestASubMillisecondRequestIsNotLoggedAsZero(t *testing.T) {
	f := newLogFixture(t, Config{})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/healthcheck")

	if got := num(t, f.onlyLine(t), "duration_ms"); got <= 0 {
		t.Errorf("want a non-zero duration for a request that took real time; got %v", got)
	}
}

// Every booking has its own UUID, so logging the raw path gives one distinct
// "route" per booking and nothing groups. The template is what an operator
// asks about: "which endpoint is slow".
func TestTheLoggedRouteCollapsesEntityIdentifiers(t *testing.T) {
	f := newLogFixture(t, Config{})
	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const path = "/api/v1/complexes/6ba7b810-9dad-11d1-80b4-00c04fd430c8/bookings/12345"
	serve(t, handler, http.MethodGet, path)

	line := f.onlyLine(t)
	if got, want := str(t, line, "route"), "/api/v1/complexes/{id}/bookings/{id}"; got != want {
		t.Errorf("want the route template %q; got %q", want, got)
	}
	// The raw path is kept as well: the template groups, the path is what you
	// need to find the one booking an incident is about.
	if got := str(t, line, "path"); got != path {
		t.Errorf("want the raw path preserved; got %q", got)
	}
}

// The client is handed an id in X-Request-ID and quotes it in a bug report.
// A log line that does not carry the same id cannot be found from it.
func TestTheLoggedRequestIDIsTheOneTheClientWasGiven(t *testing.T) {
	f := newLogFixture(t, Config{})

	handler := f.mw.RequestID(f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	w := serve(t, handler, http.MethodGet, "/api/v1/healthcheck")

	header := w.Header().Get("X-Request-ID")
	if header == "" {
		t.Fatal("setup: no X-Request-ID header")
	}
	if got := str(t, f.onlyLine(t), "request_id"); got != header {
		t.Errorf("want the log line to carry the id the client holds (%q); got %q", header, got)
	}
}

// LogRequests sits outside RecoverPanic so it observes the 500 the client
// received, not the unwritten response the panicking handler left behind.
func TestAPanickingHandlerIsLoggedAsTheStatusTheClientReceived(t *testing.T) {
	f := newLogFixture(t, Config{})

	handler := f.mw.LogRequests(f.mw.RecoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	w := serve(t, handler, http.MethodGet, "/api/v1/bookings")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("setup: want a 500 on the wire; got %d", w.Code)
	}
	if got := num(t, f.onlyLine(t), "status"); got != http.StatusInternalServerError {
		t.Errorf("want the panic logged as the 500 the client got; got %v", got)
	}
}

// A path is caller-supplied and %0a decodes to a newline. Nobody outside this
// process gets to decide where our log lines end.
func TestControlCharactersInThePathCannotForgeALogLine(t *testing.T) {
	f := newLogFixture(t, Config{})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/x%0amsg=forged%0d")

	got := str(t, f.onlyLine(t), "path")
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("the logged path still carries a line break: %q", got)
	}
	if !strings.Contains(got, "msg=forged") {
		t.Errorf("the attempt should still be visible, just defanged; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Sampling
// ---------------------------------------------------------------------------

// Sampling exists for volume, so it may thin successes.
func TestSamplingThinsSuccessfulRequests(t *testing.T) {
	f := newLogFixture(t, Config{RequestLogSample: 5})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for range 10 {
		serve(t, handler, http.MethodGet, "/api/v1/healthcheck")
	}

	if got := len(f.requestLines(t)); got != 2 {
		t.Errorf("want 1 line in 5 from 10 successes, so 2; got %d", got)
	}
}

// ...but a sampler that can drop a failure puts the original problem back.
func TestSamplingNeverDropsAFailedRequest(t *testing.T) {
	f := newLogFixture(t, Config{RequestLogSample: 1000})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	for range 3 {
		serve(t, handler, http.MethodGet, "/api/v1/bookings")
	}

	if got := len(f.requestLines(t)); got != 3 {
		t.Errorf("every failure must be logged whatever the sample rate; want 3, got %d", got)
	}
}

// The ten-second request is the one this whole file exists for. It is a
// success, so only the slow-request rule keeps it.
func TestSamplingNeverDropsASlowRequest(t *testing.T) {
	f := newLogFixture(t, Config{RequestLogSample: 1000})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(slowRequest + 10*time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/complexes")

	if got := len(f.requestLines(t)); got != 1 {
		t.Errorf("a request slower than %v must be logged whatever the sample rate; got %d lines",
			slowRequest, got)
	}
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

func TestMetricsCountEveryRequestByStatusClass(t *testing.T) {
	f := newLogFixture(t, Config{})

	status := http.StatusOK
	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))

	for _, code := range []int{200, 200, 201, 404, 500} {
		status = code
		serve(t, handler, http.MethodGet, "/api/v1/healthcheck")
	}

	m := f.mw.Metrics()
	for key, want := range map[string]int64{
		"requests_total": 5,
		"responses_2xx":  3,
		"responses_4xx":  1,
		"responses_5xx":  1,
	} {
		if got := m[key]; got != want {
			t.Errorf("want %s = %d; got %v", key, want, got)
		}
	}
}

// Sampling thins the log, never the counters: a percentile computed from a
// tenth of the requests is a different number wearing the same name.
func TestSamplingDoesNotThinTheCounters(t *testing.T) {
	f := newLogFixture(t, Config{RequestLogSample: 1000})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for range 20 {
		serve(t, handler, http.MethodGet, "/api/v1/healthcheck")
	}

	if got := len(f.requestLines(t)); got != 0 {
		t.Fatalf("setup: want the log fully sampled away; got %d lines", got)
	}
	if got := f.mw.Metrics()["requests_total"]; got != int64(20) {
		t.Errorf("want all 20 requests counted despite the sampling; got %v", got)
	}
}

// "What is the p99" was unanswerable. It is the number that has to move when
// a slow tail appears, and not move when it does not.
func TestTheLatencyDistributionReflectsTheSlowTail(t *testing.T) {
	f := newLogFixture(t, Config{RequestLogSample: 1000})

	// A real time.Sleep here made this test flaky: LogRequests measures wall
	// time, and scheduling jitter (worse under -race or a loaded CI box) can
	// push a "fast" handler with no sleep at all past the 5ms bucket boundary
	// on an unlucky run. f.mw.now is the same test seam ratelimit_redis_test.go
	// and usercache_test.go already use to move the clock without sleeping —
	// the handler advances it by exactly 30ms for the slow requests instead of
	// actually blocking, so the elapsed duration LogRequests reads is exact
	// and the test no longer depends on the scheduler at all.
	clock := time.Now()
	f.mw.now = func() time.Time { return clock }

	slow := false
	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if slow {
			clock = clock.Add(30 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
	}))

	for range 98 {
		serve(t, handler, http.MethodGet, "/api/v1/healthcheck")
	}
	slow = true
	for range 2 {
		serve(t, handler, http.MethodGet, "/api/v1/healthcheck")
	}

	m := f.mw.Metrics()
	if got := m["latency_ms_p50"]; got != 5.0 {
		t.Errorf("98 of 100 requests were fast, so the p50 must stay in the first bucket; got %v", got)
	}
	if got := m["latency_ms_p99"]; got != 50.0 {
		t.Errorf("two requests of 30ms must push the p99 into the 50ms bucket; got %v", got)
	}
	if got, ok := m["latency_ms_max"].(float64); !ok || got < 30 {
		t.Errorf("want the observed maximum to be at least the 30ms request; got %v", m["latency_ms_max"])
	}

	histogram, _ := m["latency_histogram_le_ms"].(map[string]int64)
	if histogram["5"] != 98 {
		t.Errorf("want 98 requests at or under 5ms; got %v", histogram["5"])
	}
	if histogram["inf"] != 100 {
		t.Errorf("want every measured request in the cumulative total; got %v", histogram["inf"])
	}
}

// The figure that names pool starvation: requests piled up waiting on a
// dependency, none of them finished, so no completion counter moves at all.
func TestRequestsInFlightIsVisibleWhileTheRequestIsStillRunning(t *testing.T) {
	f := newLogFixture(t, Config{})

	entered := make(chan struct{})
	release := make(chan struct{})
	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	wg.Go(func() { serve(t, handler, http.MethodGet, "/api/v1/complexes/{id}/events") })

	<-entered
	if got := f.mw.Metrics()["requests_in_flight"]; got != int64(1) {
		t.Errorf("want 1 request in flight while the handler is blocked; got %v", got)
	}
	if got := f.mw.Metrics()["requests_total"]; got != int64(0) {
		t.Errorf("an unfinished request must not be counted as completed; got %v", got)
	}

	close(release)
	wg.Wait()

	if got := f.mw.Metrics()["requests_in_flight"]; got != int64(0) {
		t.Errorf("want nothing in flight once the handler returned; got %v", got)
	}
}

// An SSE stream is open for as long as the dashboard tab is. Left in the
// histogram, a handful of them make every percentile read "the API is slow"
// when what happened is that somebody left a tab open.
func TestStreamedResponsesAreCountedButKeptOutOfTheLatencyHistogram(t *testing.T) {
	f := newLogFixture(t, Config{})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: hello\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(30 * time.Millisecond)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/complexes/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events")

	m := f.mw.Metrics()
	if got := m["streamed_total"]; got != int64(1) {
		t.Errorf("want the stream counted; got %v", got)
	}
	if got := m["requests_total"]; got != int64(1) {
		t.Errorf("want the stream in the request total; got %v", got)
	}
	histogram, _ := m["latency_histogram_le_ms"].(map[string]int64)
	if histogram["inf"] != 0 {
		t.Errorf("a streamed response must not enter the latency histogram; cumulative total is %v",
			histogram["inf"])
	}
	if got := m["latency_ms_p99"]; got != 0.0 {
		t.Errorf("with no measured request the p99 has no value to report; got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Route normalisation, directly
// ---------------------------------------------------------------------------

func TestRouteOf(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/api/v1/healthcheck", "/api/v1/healthcheck"},
		{"/api/v1/complexes/6ba7b810-9dad-11d1-80b4-00c04fd430c8", "/api/v1/complexes/{id}"},
		{"/api/v1/complexes/6ba7b810-9dad-11d1-80b4-00c04fd430c8/courts/7", "/api/v1/complexes/{id}/courts/{id}"},
		// Not an identifier: a fixed segment that merely looks unusual must be
		// left alone, or two different endpoints merge into one route.
		{"/api/v1/auth/reset-password", "/api/v1/auth/reset-password"},
		{"/api/v1/public/complexes/some-slug", "/api/v1/public/complexes/some-slug"},
		{"/", "/"},
	} {
		if got := routeOf(tc.path); got != tc.want {
			t.Errorf("routeOf(%q) = %q; want %q", tc.path, got, tc.want)
		}
	}
}

// A caller can put a kilobyte in a URL; nobody outside this process decides
// how wide our log lines are.
func TestAnOverlongPathIsTruncated(t *testing.T) {
	long := "/api/v1/" + strings.Repeat("a", 4096)

	got := routeOf(long)
	if len(got) > maxRouteLength+len("...") {
		t.Errorf("want the route bounded at %d characters; got %d", maxRouteLength, len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("want the truncation to be visible; got %q", got)
	}
}

func TestBucketOf(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want int
	}{
		{0, 0},
		{5 * time.Millisecond, 0},
		{6 * time.Millisecond, 1},
		{time.Second, 7},
		{time.Minute, numLatencyBounds},
	} {
		if got := bucketOf(tc.d); got != tc.want {
			t.Errorf("bucketOf(%v) = %d; want %d", tc.d, got, tc.want)
		}
	}
	if got, want := len(latencyBounds), numLatencyBounds; got != want {
		t.Errorf("numLatencyBounds is %d but there are %d bounds", want, got)
	}
	if got := bucketLabel(numLatencyBounds); got != "inf" {
		t.Errorf("want the overflow bucket labelled inf; got %q", got)
	}
	if got := bucketLabel(0); got != fmt.Sprint(latencyBounds[0].Milliseconds()) {
		t.Errorf("want the bucket labelled by its bound in ms; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// The actor
// ---------------------------------------------------------------------------

// TestAnAuthenticatedRequestIsLoggedWithItsUserID closes the gap that made the
// request log useless for "what did this account do": the line was written by
// a middleware sitting outside authentication, so it knew the address and the
// route but never who was asking.
func TestAnAuthenticatedRequestIsLoggedWithItsUserID(t *testing.T) {
	f := newLogFixture(t, Config{})

	userID := uuid.New()
	// The shape of the real chain: authentication runs inside LogRequests and
	// replaces the request it was handed.
	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = httpx.ContextSetUser(r, &authstore.User{ID: userID, Role: "owner"})
		if _, ok := httpx.ContextGetAuthenticatedUser(r); !ok {
			t.Error("the user did not survive into the request")
		}
		w.WriteHeader(http.StatusOK)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/auth/me")

	if got := str(t, f.onlyLine(t), "user_id"); got != userID.String() {
		t.Errorf("user_id = %q, want %q", got, userID)
	}
}

// TestAnAnonymousRequestHasNoUserIDField keeps the field out of the lines it
// would only ever be empty on.
func TestAnAnonymousRequestHasNoUserIDField(t *testing.T) {
	f := newLogFixture(t, Config{})

	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/healthcheck")

	if _, present := f.onlyLine(t)["user_id"]; present {
		t.Error("an unauthenticated request carries a user_id field")
	}
}

// TestTheRequestLogCarriesNoPersonalDataBeyondTheID is the limit on the field
// above: an id identifies an account to whoever can query the database, an
// email address identifies a person to whoever can read the logs.
func TestTheRequestLogCarriesNoPersonalDataBeyondTheID(t *testing.T) {
	f := newLogFixture(t, Config{})

	user := &authstore.User{ID: uuid.New(), Email: "someone@example.com", FirstName: "Some", LastName: "One", Phone: "+5491122334455", Role: "owner"}
	handler := f.mw.LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = httpx.ContextSetUser(r, user)
		w.WriteHeader(http.StatusOK)
	}))
	serve(t, handler, http.MethodGet, "/api/v1/auth/me")

	logged := f.logs.String()
	for _, personal := range []string{user.Email, user.LastName, user.Phone} {
		if strings.Contains(logged, personal) {
			t.Errorf("the request log carries %q", personal)
		}
	}
}
