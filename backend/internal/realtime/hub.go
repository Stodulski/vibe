// Package realtime pushes live updates to a complex's dashboard over
// Server-Sent Events.
//
// The hub holds the connections for one process. When Redis is configured it
// also relays every event through Pub/Sub, so a booking made on one instance
// reaches dashboards connected to any other. Without Redis it degrades to
// local-only delivery rather than failing.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// channel is the Redis Pub/Sub channel every instance publishes to and reads.
const channel = "sse:events"

// publishTimeout bounds a single Redis publish. It is deliberately short: the
// caller is usually finishing a request and must not wait on the fan-out.
const publishTimeout = 2 * time.Second

// clientBuffer is how many events a slow dashboard may fall behind before its
// events start being dropped.
const clientBuffer = 16

// maxStreamsPerComplex and maxStreams bound how many event streams this
// process will hold open.
//
// A stream costs a goroutine, a socket and a buffer for as long as it lives,
// and nothing on the client side limits how many a browser opens: a dashboard
// stuck in a reconnect loop, or a script pointed at the endpoint with one
// valid session, opens them as fast as the network allows. Without a ceiling
// the only thing that stops it is the process running out of file descriptors,
// which takes the whole API down with it — including the payment webhook.
//
// The per-complex limit is the one that matters, because it is per credential:
// ten is several tabs on several devices for one venue and still refuses a
// runaway loop. The process-wide limit is the backstop for many tenants doing
// it at once, and is deliberately far below what the process can carry so that
// streams cannot crowd out ordinary requests.
const (
	maxStreamsPerComplex = 10
	maxStreams           = 1000
)

// Event is one message delivered to a dashboard.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

// BookingChanged is the event every booking mutation broadcasts. The dashboard
// reacts by refetching, so the event carries no payload.
var BookingChanged = Event{Type: "booking_changed"}

// message is the Pub/Sub envelope, pairing an event with the complex it
// belongs to.
type message struct {
	ComplexID uuid.UUID `json:"complex_id"`
	Event     Event     `json:"event"`
}

// client is one connected dashboard.
type client struct {
	events chan Event
}

// Hub fans events out to the dashboards connected to each complex.
type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[*client]struct{}
	// total is the number of live subscriptions across every complex. It is
	// kept alongside the map rather than recomputed, because the admission
	// check runs on every connect and summing the map would make that cost
	// grow with the number of connected tenants.
	total int
	// perComplexLimit and totalLimit are fields rather than the constants
	// themselves so a test can drive the admission boundary without opening a
	// thousand streams.
	perComplexLimit int
	totalLimit      int
	rdb             *redis.Client
	logger          *slog.Logger
	shutdown        chan struct{}
	started         atomic.Bool
}

// NewHub returns a Hub. Passing a nil Redis client is supported and means
// local-only delivery — which is what tests and single-instance deployments
// run with.
//
// NewHub launches no goroutine, even when rdb is non-nil: the composition
// root that builds a Hub must not start background work on construction, so
// that building one twice in a test process — or building one that is never
// used — cannot leak a goroutine. Call Start to begin relaying.
func NewHub(rdb *redis.Client, logger *slog.Logger) *Hub {
	return &Hub{
		clients:         make(map[uuid.UUID]map[*client]struct{}),
		perComplexLimit: maxStreamsPerComplex,
		totalLimit:      maxStreams,
		rdb:             rdb,
		logger:          logger,
		shutdown:        make(chan struct{}),
	}
}

// Start begins relaying events published by other instances to this one's
// dashboards. It is a no-op when rdb is nil (local-only delivery, nothing to
// relay) and idempotent — a second call does nothing, so a caller does not
// need to track whether it already started this Hub.
func (h *Hub) Start() {
	if h.rdb == nil {
		return
	}
	if !h.started.CompareAndSwap(false, true) {
		return
	}
	go h.consume()
}

// Subscribe registers a dashboard for a complex and returns its event channel.
// The caller must Unsubscribe when the connection ends.
//
// The second return is false when this complex, or the process, is already at
// its stream limit; no client is registered in that case and there is nothing
// to unsubscribe. Refusing here — before the caller has written a byte — is
// what lets the refusal be an ordinary HTTP status instead of a stream that
// opens and then dies for reasons the client cannot read.
func (h *Hub) Subscribe(complexID uuid.UUID) (*client, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.total >= h.totalLimit || len(h.clients[complexID]) >= h.perComplexLimit {
		return nil, false
	}

	c := &client{events: make(chan Event, clientBuffer)}

	if h.clients[complexID] == nil {
		h.clients[complexID] = make(map[*client]struct{})
	}
	h.clients[complexID][c] = struct{}{}
	h.total++

	return c, true
}

// Unsubscribe removes a dashboard, dropping the complex's entry once its last
// connection is gone so the map does not grow without bound.
func (h *Hub) Unsubscribe(complexID uuid.UUID, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.clients[complexID]
	if !ok {
		return
	}
	// The count only moves for a client that was actually registered, so a
	// second Unsubscribe for the same connection cannot leak capacity away.
	if _, registered := conns[c]; !registered {
		return
	}
	delete(conns, c)
	h.total--
	if len(conns) == 0 {
		delete(h.clients, complexID)
	}
}

// broadcast delivers an event to this process's dashboards for a complex.
//
// A dashboard whose buffer is full is skipped rather than waited on: one stalled
// browser must not block delivery to every other connection.
func (h *Hub) broadcast(complexID uuid.UUID, event Event) {
	h.mu.RLock()
	conns := make([]*client, 0, len(h.clients[complexID]))
	for c := range h.clients[complexID] {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		select {
		case c.events <- event:
		default:
		}
	}
}

// Publish delivers an event to every instance's dashboards for a complex.
//
// It is deliberately detached from the caller's context and uses its own
// timeout: the broadcast must reach other connected clients even when the
// request that triggered it is cancelled. A Redis failure falls back to
// local-only delivery rather than losing the event entirely.
func (h *Hub) Publish(complexID uuid.UUID, event Event) {
	if h.rdb == nil {
		h.broadcast(complexID, event)
		return
	}

	data, err := json.Marshal(message{ComplexID: complexID, Event: event})
	if err != nil {
		h.logger.Error("realtime: failed to marshal event", "error", err)
		h.broadcast(complexID, event)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	if err := h.rdb.Publish(ctx, channel, data).Err(); err != nil {
		h.logger.Error("realtime: redis publish failed, delivering locally only", "error", err)
		h.broadcast(complexID, event)
	}
}

// PublishBookingChanged is the one event the rest of the application emits.
// It exists so callers name the thing that happened rather than assembling the
// same Event literal at every call site.
func (h *Hub) PublishBookingChanged(complexID uuid.UUID) {
	h.Publish(complexID, BookingChanged)
}

// consume relays events published by any instance to this one's dashboards.
// go-redis reconnects internally; the loop ends when Shutdown closes the
// subscription.
func (h *Hub) consume() {
	sub := h.rdb.Subscribe(context.Background(), channel)

	go func() {
		<-h.shutdown
		// Nothing can observe a close error during shutdown; closing only
		// unblocks the range below, which then returns.
		_ = sub.Close()
	}()

	for msg := range sub.Channel() {
		var m message
		if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
			h.logger.Error("realtime: failed to unmarshal redis message", "error", err)
			continue
		}
		h.broadcast(m.ComplexID, m.Event)
	}
}

// Shutdown stops the Redis subscription. It is safe to call more than once.
func (h *Hub) Shutdown() {
	select {
	case <-h.shutdown:
	default:
		close(h.shutdown)
	}
}
