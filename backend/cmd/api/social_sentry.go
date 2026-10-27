package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/getsentry/sentry-go"

	"github.com/stodulski/vibe-server/internal/social"
)

// socialFailureHandler sends one Sentry error for each social post that ends
// partial or failed. It forwards the single record internal/social writes at
// that moment and nothing else: a general logger-to-Sentry bridge would change
// what every error line in the process reports, and this is one deliberate
// report per post.
//
// Every record, forwarded or not, still reaches the inner handler unchanged.
type socialFailureHandler struct {
	inner slog.Handler
	hub   *sentry.Hub
}

// socialFailureReporting wraps inner so that the terminal social failure record
// is also captured on hub. A hub with no bound client sends nothing.
func socialFailureReporting(inner slog.Handler, hub *sentry.Hub) slog.Handler {
	return socialFailureHandler{inner: inner, hub: hub}
}

func (h socialFailureHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h socialFailureHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError && r.Message == social.FailureLogMessage {
		h.hub.CaptureEvent(socialFailureEvent(r))
	}
	if err := h.inner.Handle(ctx, r); err != nil {
		return fmt.Errorf("social sentry log: %w", err)
	}
	return nil
}

func (h socialFailureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return socialFailureHandler{inner: h.inner.WithAttrs(attrs), hub: h.hub}
}

func (h socialFailureHandler) WithGroup(name string) slog.Handler {
	return socialFailureHandler{inner: h.inner.WithGroup(name), hub: h.hub}
}

// socialFailureEvent builds the event for one failure record. The fingerprint is
// the post id, so every report about one post groups together.
//
// The last error is scrubbed here as well as by the publisher. scrubEvent, the
// BeforeSend hook, does not walk event contexts, so this is the only pass the
// context value gets.
func socialFailureEvent(r slog.Record) *sentry.Event {
	var postID, status, kind, lastErr string
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "post":
			postID = a.Value.String()
		case "status":
			status = a.Value.String()
		case "type":
			kind = a.Value.String()
		case "error":
			lastErr = a.Value.String()
		}
		return true
	})
	return &sentry.Event{
		Level:       sentry.LevelError,
		Message:     "social post " + postID + " ended " + status,
		Fingerprint: []string{postID},
		Tags: map[string]string{
			"social.post_id": postID,
			"social.status":  status,
			"social.type":    kind,
		},
		Contexts: map[string]sentry.Context{
			"social": {"last_error": scrubText(lastErr)},
		},
	}
}
