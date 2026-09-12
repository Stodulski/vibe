package main

import (
	"bufio"
	"bytes"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/realtime"
)

// shutdownFixture is the process-lifecycle slice of an application: the pieces
// gracefulShutdown actually touches, and a log buffer to read back what it did.
//
// It is built by hand rather than through newTestApplication because the whole
// point is the ordering between app.shutdown, srv.Shutdown and the cleanup —
// nothing here needs stores, handlers or authentication, and a full application
// would only make the timings harder to read.
type shutdownFixture struct {
	app  *application
	logs *bytes.Buffer
	mu   sync.Mutex
}

// Write serializes the log writes, which arrive from the shutdown goroutine
// while the test polls the buffer from its own.
func (f *shutdownFixture) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logs.Write(p)
}

func (f *shutdownFixture) logged(substr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Contains(f.logs.String(), substr)
}

// waitForLog polls until the shutdown sequence has logged substr, which is how
// a test observes that it reached a given step rather than returning early.
func (f *shutdownFixture) waitForLog(t *testing.T, substr string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !f.logged(substr) {
		if time.Now().After(deadline) {
			t.Fatalf("the shutdown sequence never reached %q — the cleanup steps after srv.Shutdown were skipped; log was:\n%s",
				substr, f.logs.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func newShutdownFixture(t *testing.T) *shutdownFixture {
	t.Helper()

	f := &shutdownFixture{logs: &bytes.Buffer{}}
	logger := slog.New(slog.NewTextHandler(f, nil))
	f.app = &application{
		logger:   logger,
		respond:  httpx.NewResponder(logger),
		shutdown: make(chan struct{}),
	}
	f.app.events = realtime.NewHub(nil, logger, "test")
	return f
}

// serveOn starts handler on a real listener and returns the server and its
// address. A real http.Server is required, not httptest: the behaviour under
// test is http.Server.Shutdown's interaction with a handler that is still
// running, which httptest.Server.Close does not reproduce.
func serveOn(t *testing.T, handler http.Handler) (*http.Server, string) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	return srv, ln.Addr().String()
}

// The deadlock this fix exists for.
//
// srv.Shutdown waits for in-flight requests and does not cancel their contexts.
// The SSE handler returns only on r.Context().Done() or on app.shutdown. With
// app.shutdown closed after srv.Shutdown returned, the two waited on each other
// and one open dashboard burned the whole timeout on every deploy — after which
// the timeout path took an early return and skipped every cleanup step.
func TestShutdownDoesNotWaitOnItsOwnEventStreams(t *testing.T) {
	f := newShutdownFixture(t)

	// Long lifecycle timers: this test is about the shutdown channel, and a
	// re-check or an expiry firing first would end the stream for the wrong
	// reason and pass it without proving anything.
	stream := realtime.NewHandler(f.app.events, streamAuthorizer{mw: f.app.middleware},
		f.app.respond, f.app.logger, f.app.shutdown,
		realtime.Config{RecheckInterval: time.Hour, MaxLifetime: time.Hour})
	srv, addr := serveOn(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The complex is what the stream endpoint is scoped to; the ownership
		// checks that normally put it there are not what is being tested.
		stream.Stream(w, httpx.ContextSetComplex(r, &complexstore.Complex{ID: uuid.New()}))
	}))

	// The request context stays live for the whole test on purpose. If it were
	// cancelled the stream would return through r.Context().Done() and the test
	// would pass without ever exercising the shutdown channel.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/events", nil)
	if err != nil {
		t.Fatalf("building the stream request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("opening the event stream: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Reading the greeting frame proves the handler is inside its loop, so the
	// stream is genuinely an in-flight request when the shutdown starts.
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || !strings.Contains(line, "connected") {
		t.Fatalf("the stream never connected; got %q (%v)", line, err)
	}

	const timeout = 3 * time.Second

	start := time.Now()
	shutdownErr := f.app.gracefulShutdown(srv, timeout)
	elapsed := time.Since(start)

	if shutdownErr != nil {
		t.Errorf("an open event stream must not make the shutdown fail; got %v", shutdownErr)
	}
	// A tenth of the budget: enough room for a loaded CI machine, far too
	// little to be confused with having waited the timeout out.
	if want := timeout / 10; elapsed > want {
		t.Errorf("the shutdown took %v of its %v budget draining one event stream; it must finish within %v — "+
			"app.shutdown has to be closed before srv.Shutdown, or the two wait on each other",
			elapsed, timeout, want)
	}
}

// The other half: srv.Shutdown timing out used to take an early return, so
// app.wg.Wait, the hub, the notifier queue and the Redis client were all
// skipped. The notifier queue is at-most-once and destructive-pop, so every
// worker mid-task lost its work. A shutdown that timed out needs the drain
// more, not less.
func TestShutdownDrainsEvenWhenTheServerTimesOut(t *testing.T) {
	f := newShutdownFixture(t)

	// A request that ignores app.shutdown entirely, so srv.Shutdown cannot
	// finish and must hit its deadline.
	releaseHandler := make(chan struct{})
	handlerEntered := make(chan struct{})
	srv, addr := serveOn(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(handlerEntered)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-releaseHandler
	}))

	// The request is written over a raw connection the test keeps open.
	// http.Get would not do: closing its response body drops the connection,
	// and srv.Shutdown stops tracking a connection that is gone even while its
	// handler goroutine is still running — so the timeout path would never be
	// reached and this test would silently assert nothing.
	var dialer net.Dialer
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatalf("dialling the hung request: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte("GET /hang HTTP/1.1\r\nHost: test\r\n\r\n")); err != nil {
		t.Fatalf("writing the hung request: %v", err)
	}
	<-handlerEntered

	// A tracked background task still holding work when the stop begins. If
	// app.wg.Wait() is skipped, gracefulShutdown returns while this is unfinished.
	var taskFinished atomic.Bool
	releaseTask := make(chan struct{})
	taskStarted := make(chan struct{})
	f.app.background(func() {
		close(taskStarted)
		<-releaseTask
		taskFinished.Store(true)
	})
	<-taskStarted

	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- f.app.gracefulShutdown(srv, 200*time.Millisecond) }()

	// Reaching this log line is only possible past the point the old code
	// returned from, and it means the sequence is now inside app.wg.Wait().
	f.waitForLog(t, "completing background tasks")

	close(releaseTask)
	close(releaseHandler)

	err = <-shutdownErr
	if err == nil {
		t.Fatal("this test is only meaningful on the timeout path; srv.Shutdown reported success, so the hung request did not hang")
	}
	if !taskFinished.Load() {
		t.Error("gracefulShutdown returned without waiting for a tracked background task")
	}
	if !f.logged("draining the job queue") {
		t.Errorf("the job queue was never drained on the timeout path; log was:\n%s", f.logs.String())
	}
}
