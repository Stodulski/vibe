package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

// socialFailureLogMessage is the text internal/social logs for a post that
// ended partial or failed. internal/social pins the same text in its tests.
const socialFailureLogMessage = "social: post did not fully publish"

// recordingTransport keeps every event the client sends. A custom Transport
// makes the SDK send synchronously, so a test can read the events right after
// the log call returns.
type recordingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (r *recordingTransport) Flush(time.Duration) bool              { return true }
func (r *recordingTransport) FlushWithContext(context.Context) bool { return true }
func (r *recordingTransport) Configure(sentry.ClientOptions)        {}
func (r *recordingTransport) Close()                                {}

func (r *recordingTransport) SendEvent(event *sentry.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingTransport) sent() []*sentry.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

// testHub returns a hub bound to a client that records what it would send. The
// client runs the production BeforeSend, so a test sees what would leave.
func testHub(t *testing.T) (*sentry.Hub, *recordingTransport) {
	t.Helper()
	transport := &recordingTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:        "https://public@example.com/1",
		Transport:  transport,
		BeforeSend: scrubEvent,
	})
	if err != nil {
		t.Fatalf("sentry.NewClient() = %v", err)
	}
	return sentry.NewHub(client, sentry.NewScope()), transport
}

// socialTestLogger builds the logger the way baseLogger does: the JSON handler
// wrapped by the social reporter, with With-attributes applied on top.
func socialTestLogger(hub *sentry.Hub) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	handler := socialFailureReporting(slog.NewJSONHandler(&buf, nil), hub)
	return slog.New(handler).With("service", "vibe-api"), &buf
}

func TestAPartialPostIsSentAsOneErrorGroupedByPost(t *testing.T) {
	t.Parallel()
	hub, transport := testHub(t)
	logger, _ := socialTestLogger(hub)

	logger.Error(socialFailureLogMessage, "post", "p-1", "status", "partial", "type", "story", "error", "instagram: no publicada")

	events := transport.sent()
	if len(events) != 1 {
		t.Fatalf("events = %d, want exactly one", len(events))
	}
	ev := events[0]
	if ev.Level != sentry.LevelError || ev.Message != "social post p-1 ended partial" {
		t.Fatalf("event = level %q message %q, want an error naming the post and its status", ev.Level, ev.Message)
	}
	if !slices.Equal(ev.Fingerprint, []string{"p-1"}) {
		t.Fatalf("fingerprint = %v, want the post id so repeats group together", ev.Fingerprint)
	}
	wantTags := map[string]string{"social.post_id": "p-1", "social.status": "partial", "social.type": "story"}
	for key, want := range wantTags {
		if got := ev.Tags[key]; got != want {
			t.Fatalf("tag %s = %q, want %q", key, got, want)
		}
	}
	if got := ev.Contexts["social"]["last_error"]; got != "instagram: no publicada" {
		t.Fatalf("social context last_error = %v, want the last error", got)
	}
}

func TestAFailedPostIsSentAsOneError(t *testing.T) {
	t.Parallel()
	hub, transport := testHub(t)
	logger, _ := socialTestLogger(hub)

	logger.Error(socialFailureLogMessage, "post", "p-2", "status", "failed", "type", "image", "error", "meta 400: invalid image")

	events := transport.sent()
	if len(events) != 1 {
		t.Fatalf("events = %d, want exactly one", len(events))
	}
	if events[0].Message != "social post p-2 ended failed" || events[0].Tags["social.status"] != "failed" {
		t.Fatalf("event = message %q tags %v, want the failed post", events[0].Message, events[0].Tags)
	}
}

func TestTheSentErrorIsScrubbedEvenThoughTheRecordIsNot(t *testing.T) {
	t.Parallel()
	hub, transport := testHub(t)
	logger, _ := socialTestLogger(hub)

	logger.Error(socialFailureLogMessage, "post", "p-3", "status", "failed", "type", "image",
		"error", "contact juan@example.com on +54 11 5555-1234")

	events := transport.sent()
	if len(events) != 1 {
		t.Fatalf("events = %d, want exactly one", len(events))
	}
	got := fmt.Sprint(events[0].Contexts["social"]["last_error"])
	if strings.Contains(got, "juan@example.com") || strings.Contains(got, "5555") || !strings.Contains(got, "[redacted]") {
		t.Fatalf("social context last_error = %q, want the email and phone redacted", got)
	}
}

func TestOnlyTheFailureLineIsSentButEveryRecordIsLogged(t *testing.T) {
	t.Parallel()
	hub, transport := testHub(t)
	logger, buf := socialTestLogger(hub)

	logger.Error("social: could not record the publish result", "post", "p-4", "error", "db down")
	logger.Warn(socialFailureLogMessage, "post", "p-4", "status", "partial")
	logger.Info(socialFailureLogMessage, "post", "p-4", "status", "partial")

	if n := len(transport.sent()); n != 0 {
		t.Fatalf("events = %d, want none: only the error-level failure line is forwarded", n)
	}
	for _, want := range []string{"could not record the publish result", socialFailureLogMessage} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("log output is missing %q", want)
		}
	}
}

func TestWithoutASentryClientNothingIsSentAndTheLineIsStillWritten(t *testing.T) {
	t.Parallel()
	logger, buf := socialTestLogger(sentry.NewHub(nil, sentry.NewScope()))

	logger.Error(socialFailureLogMessage, "post", "p-5", "status", "failed", "type", "image", "error", "meta 400")

	if !strings.Contains(buf.String(), socialFailureLogMessage) {
		t.Fatal("the log line must still be written when Sentry is not configured")
	}
}
