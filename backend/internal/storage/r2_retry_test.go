package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeTransport answers every request from respond and counts the attempts.
type fakeTransport struct {
	attempts atomic.Int64
	respond  func(*http.Request) (*http.Response, error)
}

func (f *fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.attempts.Add(1)
	return f.respond(r)
}

// status builds a bodiless response with the given status.
//
func status(code int) func(*http.Request) (*http.Response, error) {
	return func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: code,
			Status:     http.StatusText(code),
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	}
}

func clientOver(t *testing.T, transport http.RoundTripper) *R2Client {
	t.Helper()
	c, err := newR2Client("account", "AKIAEXAMPLEKEY", "examplesecret", "bucket",
		"https://cdn.example.com", &http.Client{Transport: transport, Timeout: r2RequestTimeout})
	if err != nil {
		t.Fatalf("newR2Client: %v", err)
	}
	return c
}

// TestATransientFailureIsRetriedUpToTheConfiguredBudget is half of S3-05. The
// config had no Retryer at all, so how many times an operation was sent was
// whatever the SDK version in go.sum happened to default to — which is not a
// decision, and does not survive an upgrade.
func TestATransientFailureIsRetriedUpToTheConfiguredBudget(t *testing.T) {
	//nolint:bodyclose // status only builds a response; the SDK is the caller that reads and closes it.
	transport := &fakeTransport{respond: status(http.StatusServiceUnavailable)}
	client := clientOver(t, transport)

	if err := client.DeleteObject(t.Context(), "courts/photo.jpg"); err == nil {
		t.Fatal("DeleteObject reported success against a 503")
	}
	if got := transport.attempts.Load(); got != r2MaxAttempts {
		t.Errorf("a 503 was sent %d times, want %d", got, r2MaxAttempts)
	}
}

// TestAnAnswerAboutTheObjectIsNotRetried is the other half: a retry budget
// that is spent on a refusal is three times the latency for the same answer,
// and on a delete it is three times the log noise for an object that will
// never be deletable by these credentials.
func TestAnAnswerAboutTheObjectIsNotRetried(t *testing.T) {
	for _, code := range []int{http.StatusForbidden, http.StatusNotFound} {
		//nolint:bodyclose // status only builds a response; the SDK is the caller that reads and closes it.
		transport := &fakeTransport{respond: status(code)}
		client := clientOver(t, transport)

		_ = client.DeleteObject(t.Context(), "courts/photo.jpg")

		if got := transport.attempts.Load(); got != 1 {
			t.Errorf("a %d was sent %d times, want 1", code, got)
		}
	}
}

// TestAStalledRequestIsCutOffRatherThanHanging is the timeout. Without an
// explicit HTTPClient the SDK's default one has a zero Timeout, so a
// connection that is accepted and then goes quiet holds the caller until its
// own context expires — and one caller here, the cleanup that follows a court
// delete, runs on a detached context.
func TestAStalledRequestIsCutOffRatherThanHanging(t *testing.T) {
	// A transport that never answers, standing in for a socket that accepted
	// the request and went silent.
	stalled := &fakeTransport{respond: func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}}

	// The client's own timeout is what has to fire, so the caller's context is
	// given far more room than it.
	c, err := newR2Client("account", "AKIAEXAMPLEKEY", "examplesecret", "bucket",
		"https://cdn.example.com", &http.Client{Transport: stalled, Timeout: 150 * time.Millisecond})
	if err != nil {
		t.Fatalf("newR2Client: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- c.DeleteObject(ctx, "courts/photo.jpg") }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("DeleteObject reported success against a transport that never answered")
		}
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
			t.Error("the caller's context expired rather than the client's own timeout")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("DeleteObject hung: the client has no timeout of its own")
	}
}
