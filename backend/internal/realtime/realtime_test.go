package realtime

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

func testHub(t *testing.T) *Hub {
	t.Helper()
	h := NewHub(nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	t.Cleanup(h.Shutdown)
	return h
}

// receive waits briefly for one event, so a delivery failure fails the test
// instead of hanging it.
func receive(t *testing.T, c *client) (Event, bool) {
	t.Helper()
	select {
	case e := <-c.events:
		return e, true
	case <-time.After(time.Second):
		return Event{}, false
	}
}

func TestPublishReachesSubscribersOfThatComplex(t *testing.T) {
	hub := testHub(t)
	complexID := uuid.New()

	c := mustSubscribe(t, hub, complexID)
	hub.PublishBookingChanged(complexID)

	event, ok := receive(t, c)
	if !ok {
		t.Fatal("the subscriber never received the event")
	}
	if event.Type != BookingChanged.Type {
		t.Errorf("want %q; got %q", BookingChanged.Type, event.Type)
	}
}

// Events are scoped per complex: one owner's dashboard must never see another
// complex's bookings.
func TestPublishDoesNotLeakAcrossComplexes(t *testing.T) {
	hub := testHub(t)
	mine, theirs := uuid.New(), uuid.New()

	c := mustSubscribe(t, hub, mine)
	hub.PublishBookingChanged(theirs)

	if _, ok := receive(t, c); ok {
		t.Error("received an event published to another complex")
	}
}

func TestEverySubscriberOfAComplexReceivesTheEvent(t *testing.T) {
	hub := testHub(t)
	complexID := uuid.New()

	first, second := mustSubscribe(t, hub, complexID), mustSubscribe(t, hub, complexID)
	hub.PublishBookingChanged(complexID)

	if _, ok := receive(t, first); !ok {
		t.Error("the first subscriber missed the event")
	}
	if _, ok := receive(t, second); !ok {
		t.Error("the second subscriber missed the event")
	}
}

func TestUnsubscribedClientStopsReceiving(t *testing.T) {
	hub := testHub(t)
	complexID := uuid.New()

	c := mustSubscribe(t, hub, complexID)
	hub.Unsubscribe(complexID, c)
	hub.PublishBookingChanged(complexID)

	if _, ok := receive(t, c); ok {
		t.Error("an unsubscribed client still received an event")
	}
}

// The hub must forget a complex once its last dashboard disconnects, or the
// map grows for the lifetime of the process.
func TestHubForgetsAComplexWithNoSubscribers(t *testing.T) {
	hub := testHub(t)
	complexID := uuid.New()

	c := mustSubscribe(t, hub, complexID)
	hub.Unsubscribe(complexID, c)

	hub.mu.RLock()
	_, still := hub.clients[complexID]
	hub.mu.RUnlock()

	if still {
		t.Error("the complex entry outlived its last subscriber")
	}
}

// A dashboard that stops reading must not stall delivery to the others: its
// events are dropped once its buffer fills.
func TestASlowSubscriberDoesNotBlockPublish(t *testing.T) {
	hub := testHub(t)
	complexID := uuid.New()

	slow := mustSubscribe(t, hub, complexID)
	for range clientBuffer {
		slow.events <- BookingChanged
	}
	fast := mustSubscribe(t, hub, complexID)

	done := make(chan struct{})
	go func() {
		hub.PublishBookingChanged(complexID)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on the subscriber that stopped reading")
	}

	if _, ok := receive(t, fast); !ok {
		t.Error("the responsive subscriber was starved by the slow one")
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	hub := NewHub(nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	hub.Shutdown()
	hub.Shutdown() // must not panic on a already-closed channel
}

func testHandler(t *testing.T, hub *Hub, shutdown <-chan struct{}) *Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	// An allowing authorizer and long lifecycle timers: these tests are about
	// delivery, not about the stream's lifecycle. stream_authz_test.go drives
	// that with intervals it can wait for.
	return NewHandler(hub, allowAll{}, httpx.NewResponder(logger), logger, shutdown,
		Config{RecheckInterval: time.Hour, MaxLifetime: time.Hour})
}

func TestStreamRequiresTheComplexInContext(t *testing.T) {
	h := testHandler(t, testHub(t), make(chan struct{}))

	w := httptest.NewRecorder()
	h.Stream(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 with no complex in context; got %d", w.Code)
	}
}

func TestStreamSendsHeadersAndTheConnectedEvent(t *testing.T) {
	hub := testHub(t)
	complexID := uuid.New()
	shutdown := make(chan struct{})
	h := testHandler(t, hub, shutdown)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Stream(w, httpx.ContextSetComplex(r, &data.Complex{ID: complexID}))
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("want an event-stream content type; got %q", got)
	}
	if got := resp.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("nginx buffering must be disabled or events are held back; got %q", got)
	}

	buf := make([]byte, 64)
	n, err := resp.Body.Read(buf)
	if err != nil {
		t.Fatalf("reading the opening frame: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "event: connected") {
		t.Errorf("want an opening connected frame; got %q", string(buf[:n]))
	}

	// Closing the application's shutdown channel must end the stream.
	close(shutdown)
}

func TestStreamDeliversPublishedEvents(t *testing.T) {
	hub := testHub(t)
	complexID := uuid.New()
	h := testHandler(t, hub, make(chan struct{}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Stream(w, httpx.ContextSetComplex(r, &data.Complex{ID: complexID}))
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	buf := make([]byte, 64)
	if _, err := resp.Body.Read(buf); err != nil { // consume the connected frame
		t.Fatalf("reading the opening frame: %v", err)
	}

	// Wait for the handler to register before publishing, otherwise the event
	// is broadcast to an empty room.
	deadline := time.Now().Add(time.Second)
	for {
		hub.mu.RLock()
		registered := len(hub.clients[complexID]) > 0
		hub.mu.RUnlock()
		if registered || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	hub.PublishBookingChanged(complexID)

	read := make([]byte, 128)
	n, err := resp.Body.Read(read)
	if err != nil {
		t.Fatalf("reading the event frame: %v", err)
	}
	if !strings.Contains(string(read[:n]), "event: booking_changed") {
		t.Errorf("want a booking_changed frame; got %q", string(read[:n]))
	}
}
