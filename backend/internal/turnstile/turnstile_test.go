package turnstile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

func TestVerifySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("siteverify request body did not parse: %v", err)
		}
		if got := r.PostForm.Get("secret"); got != "test-secret" {
			t.Errorf("secret = %q, want %q", got, "test-secret")
		}
		if got := r.PostForm.Get("response"); got != "good-token" {
			t.Errorf("response = %q, want %q", got, "good-token")
		}
		if got := r.PostForm.Get("remoteip"); got != "203.0.113.7" {
			t.Errorf("remoteip = %q, want %q", got, "203.0.113.7")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	cb := circuitbreaker.New(circuitbreaker.Config{Name: "turnstile", MaxFailures: 1, ResetTimeout: time.Hour})
	c := New(Config{SecretKey: "test-secret", CB: cb})
	c.apiURL = srv.URL

	if err := c.Verify(context.Background(), "good-token", "203.0.113.7"); err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
	if cb.Status().ConsecutiveFailures != 0 {
		t.Error("a successful verification must not count against the breaker")
	}
}

func TestVerifyInvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response","timeout-or-duplicate"]}`))
	}))
	defer srv.Close()

	cb := circuitbreaker.New(circuitbreaker.Config{Name: "turnstile", MaxFailures: 1, ResetTimeout: time.Hour})
	c := New(Config{SecretKey: "test-secret", CB: cb})
	c.apiURL = srv.URL

	err := c.Verify(context.Background(), "bad-token", "")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() = %v, want an error wrapping ErrInvalidToken", err)
	}
	if got := err.Error(); got == ErrInvalidToken.Error() {
		t.Error("the error-codes Cloudflare returned should be in the wrapped message")
	}
	// Cloudflare answered on time with a well-formed refusal — that is not an
	// infrastructure failure, and must not open the breaker for everybody else.
	if cb.Status().ConsecutiveFailures != 0 {
		t.Error("an invalid token must not count against the breaker")
	}
}

func TestVerifyRejectsAnEmptyTokenWithoutCallingCloudflare(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := New(Config{SecretKey: "test-secret"})
	c.apiURL = srv.URL

	err := c.Verify(context.Background(), "", "")
	if !errors.Is(err, ErrMissingToken) {
		t.Fatalf("Verify(\"\") = %v, want ErrMissingToken", err)
	}
	if called {
		t.Error("an empty token must never reach Cloudflare")
	}
}

func TestVerifyTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	cb := circuitbreaker.New(circuitbreaker.Config{
		Name: "turnstile", MaxFailures: 1, ResetTimeout: time.Hour,
	})
	c := New(Config{SecretKey: "test-secret", CB: cb})
	c.apiURL = srv.URL
	c.httpClient = &http.Client{Timeout: 20 * time.Millisecond}

	err := c.Verify(context.Background(), "good-token", "")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify() on a slow server = %v, want an error wrapping ErrUnavailable", err)
	}
	if cb.Status().ConsecutiveFailures == 0 {
		t.Error("a timeout is an infrastructure failure and must count against the breaker")
	}
}

func TestVerifyCanceledContextDoesNotRecordFailure(t *testing.T) {
	cb := circuitbreaker.New(circuitbreaker.Config{Name: "turnstile", MaxFailures: 1, ResetTimeout: time.Hour})
	c := New(Config{SecretKey: "test-secret", CB: cb})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Verify(ctx, "good-token", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify() with a canceled context = %v, want ErrUnavailable", err)
	}
	if cb.Status().ConsecutiveFailures != 0 {
		t.Error("a caller-canceled context must not count against the breaker")
	}
}

func TestVerifyServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cb := circuitbreaker.New(circuitbreaker.Config{Name: "turnstile", MaxFailures: 1, ResetTimeout: time.Hour})
	c := New(Config{SecretKey: "test-secret", CB: cb})
	c.apiURL = srv.URL

	err := c.Verify(context.Background(), "good-token", "")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify() on a 500 = %v, want an error wrapping ErrUnavailable", err)
	}
	if cb.Status().ConsecutiveFailures == 0 {
		t.Error("a 500 from Cloudflare is an infrastructure failure and must count against the breaker")
	}
}

func TestVerifyOpenBreakerIsReportedAsUnavailable(t *testing.T) {
	cb := circuitbreaker.New(circuitbreaker.Config{Name: "turnstile", MaxFailures: 1, ResetTimeout: time.Hour})
	cb.RecordFailure() // opens it

	c := New(Config{SecretKey: "test-secret", CB: cb})

	err := c.Verify(context.Background(), "good-token", "")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify() through an open breaker = %v, want an error wrapping ErrUnavailable", err)
	}
	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Errorf("Verify() through an open breaker = %v, want it to also wrap circuitbreaker.ErrOpen", err)
	}
}

func TestEnabled(t *testing.T) {
	if (&Client{}).Enabled() {
		t.Error("a client with no secret key must report disabled")
	}
	if !New(Config{SecretKey: "x"}).Enabled() {
		t.Error("a client with a secret key must report enabled")
	}
}
