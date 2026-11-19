package instagram

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

const secretToken = "IGQVJsecret-long-lived-token"

// A failed GET carries the token in its query string; a transport error must
// not hand it to whoever logs the error.
func TestTransportErrorsDoNotCarryTheToken(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // nothing listens: every request fails at dial

	c := New(Config{GraphBase: "http://" + addr, HTTP: &http.Client{Timeout: time.Second}})
	_, err = c.Refresh(t.Context(), secretToken)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Refresh against a closed port = %v, want ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Fatalf("the error carries the token: %v", err)
	}
}

// Meta's 4xx is a refusal (no retry helps), its 5xx and 429 are unavailability
// (retry later), and only the latter count against the breaker.
func TestStatusesMapToRejectedOrUnavailable(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusBadRequest, ErrRejected},
		{http.StatusUnauthorized, ErrRejected},
		{http.StatusTooManyRequests, ErrUnavailable},
		{http.StatusInternalServerError, ErrUnavailable},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"error":{"message":"Error validating access token","code":190}}`))
		}))
		c := New(Config{GraphBase: srv.URL})
		_, err := c.Refresh(t.Context(), secretToken)
		srv.Close()
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d: err = %v, want %v", tc.status, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), secretToken) {
			t.Errorf("status %d: the error carries the token", tc.status)
		}
	}
}

// An open breaker stops requests before they leave: Meta is not hammered
// while it is down.
func TestOpenBreakerSendsNothing(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	cb := circuitbreaker.New(circuitbreaker.Config{Name: "ig_test", MaxFailures: 2, ResetTimeout: time.Hour, HalfOpenMaxReqs: 1})
	c := New(Config{GraphBase: srv.URL, CB: cb})
	for range 5 {
		_, _ = c.Refresh(t.Context(), secretToken)
	}
	if hits != 2 {
		t.Fatalf("Meta received %d requests, want 2 before the breaker opened", hits)
	}
}

// A 200 that does not carry a token is not a success: storing an empty token
// would look connected and publish nothing.
func TestTokenResponseWithoutATokenIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"token_type":"bearer","expires_in":5183944}`))
	}))
	defer srv.Close()
	_, err := New(Config{GraphBase: srv.URL}).LongLived(t.Context(), "short")
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("LongLived with no token in the response = %v, want ErrRejected", err)
	}
}
