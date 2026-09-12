package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// The tests below drive the Redis-relay branch of Hub, split out of NewHub in
// this change so that the composition root can build a Hub without launching
// a goroutine and decide separately, in main(), when relaying actually
// starts. They run against miniredis so `make test` covers the split with no
// external service, the same reasoning internal/auth/blacklist_redis_test.go
// documents.

// newHubDeps returns two clients pointed at the same miniredis instance: one
// for the Hub under test, and a second a test uses to publish directly on the
// wire, standing in for another instance's Hub.
func newHubDeps(t *testing.T) (hub *redis.Client, publisher *redis.Client) {
	t.Helper()

	mr := miniredis.RunT(t)

	hub = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = hub.Close() })

	publisher = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = publisher.Close() })

	return hub, publisher
}

// publishRaw publishes directly on the Pub/Sub channel Hub.consume reads,
// bypassing Hub.Publish entirely — this is what "another instance" does.
func publishRaw(t *testing.T, h *Hub, rdb *redis.Client, complexID uuid.UUID) {
	t.Helper()

	payload, err := json.Marshal(message{ComplexID: complexID, Event: Event{Type: "test_event"}})
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Publish(ctx, h.channel, payload).Err(); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestNewHubStartsNoSubscriber is the RED case this phase introduces: before
// this change NewHub started consume() itself whenever rdb was non-nil.
func TestNewHubStartsNoSubscriber(t *testing.T) {
	hubClient, publisher := newHubDeps(t)
	h := NewHub(hubClient, testLogger(), "test")
	t.Cleanup(h.Shutdown)

	complexID := uuid.New()
	c := mustSubscribe(t, h, complexID)
	defer h.Unsubscribe(complexID, c)

	// Give miniredis a moment to prove a subscription that should not exist.
	time.Sleep(50 * time.Millisecond)
	publishRaw(t, h, publisher, complexID)

	select {
	case <-c.events:
		t.Fatal("received an event before Start() was called; NewHub must not start consume()")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHubStartLaunchesExactlyOneSubscriber(t *testing.T) {
	hubClient, publisher := newHubDeps(t)
	h := NewHub(hubClient, testLogger(), "test")
	t.Cleanup(h.Shutdown)

	h.Start()
	// Let the SUBSCRIBE reach miniredis before publishing.
	time.Sleep(50 * time.Millisecond)

	complexID := uuid.New()
	c := mustSubscribe(t, h, complexID)
	defer h.Unsubscribe(complexID, c)

	publishRaw(t, h, publisher, complexID)

	select {
	case ev := <-c.events:
		if ev.Type != "test_event" {
			t.Errorf("event type = %q; want %q", ev.Type, "test_event")
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive an event after Start()")
	}
}

// TestHubStartIsIdempotent asserts a second Start() call does not launch a
// second subscriber: exactly one event is delivered per publish, not two.
func TestHubStartIsIdempotent(t *testing.T) {
	hubClient, publisher := newHubDeps(t)
	h := NewHub(hubClient, testLogger(), "test")
	t.Cleanup(h.Shutdown)

	h.Start()
	h.Start()
	time.Sleep(50 * time.Millisecond)

	complexID := uuid.New()
	c := mustSubscribe(t, h, complexID)
	defer h.Unsubscribe(complexID, c)

	publishRaw(t, h, publisher, complexID)

	received := 0
	timeout := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case <-c.events:
			received++
		case <-timeout:
			break loop
		}
	}
	if received != 1 {
		t.Fatalf("want exactly 1 event delivered (one subscriber); got %d", received)
	}
}

// TestHubStartIsNoopWithoutRedis covers the local-only path: Start must not
// panic when rdb is nil.
func TestHubStartIsNoopWithoutRedis(t *testing.T) {
	h := NewHub(nil, testLogger(), "test")
	t.Cleanup(h.Shutdown)

	h.Start()
	h.Start()
}

// TestTheEventChannelCarriesTheEnvironment is RED-01 for the relay. Two
// deployments sharing one Redis would otherwise deliver each other's booking
// events to each other's dashboards, which reads as a dashboard refreshing for
// a booking that does not exist in it.
func TestTheEventChannelCarriesTheEnvironment(t *testing.T) {
	staging := NewHub(nil, testLogger(), "staging")
	production := NewHub(nil, testLogger(), "production")

	if staging.channel == production.channel {
		t.Fatal("two environments publish booking events on the same channel")
	}
	if !strings.HasPrefix(staging.channel, "vibe:staging:") {
		t.Errorf("channel %q is not namespaced by application and environment", staging.channel)
	}
}
