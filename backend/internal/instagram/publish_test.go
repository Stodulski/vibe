package instagram

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// The publisher tells Meta's refusals apart by their Graph code: 9007 and 24
// are retried, 190 asks the owner to reconnect. The code must survive on both
// kinds of answer, and the token must not reach the error.
func TestPublishingErrorsCarryTheGraphCode(t *testing.T) {
	cases := []struct {
		status int
		body   string
		kind   error
		code   int
	}{
		{http.StatusBadRequest, `{"error":{"message":"Media ID is not available","code":9007,"error_subcode":2207027}}`, ErrRejected, CodeMediaNotReady},
		{http.StatusBadRequest, `{"error":{"message":"Invalid OAuth access token","code":190}}`, ErrRejected, CodeInvalidToken},
		{http.StatusInternalServerError, `{"error":{"message":"An unexpected error has occurred","code":2}}`, ErrUnavailable, 2},
		{http.StatusBadGateway, `<html>bad gateway</html>`, ErrUnavailable, 0},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		_, err := New(Config{GraphBase: srv.URL}).PublishContainer(t.Context(), "1784", secretToken, "c1")
		srv.Close()
		if !errors.Is(err, tc.kind) {
			t.Errorf("%d %s: err = %v, want %v", tc.status, tc.body, err, tc.kind)
		}
		if got := GraphCode(err); got != tc.code {
			t.Errorf("%d %s: GraphCode = %d, want %d", tc.status, tc.body, got, tc.code)
		}
		if err != nil && strings.Contains(err.Error(), secretToken) {
			t.Errorf("the error carries the token: %v", err)
		}
	}
}

// One account being throttled (429) must not open the breaker every complex
// and the connect flow share: only outages count against it.
func TestThrottlingDoesNotOpenTheSharedBreaker(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if strings.HasPrefix(r.URL.Path, "/throttled") {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"Application request limit reached","code":4}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"media-1"}`))
	}))
	defer srv.Close()
	cb := circuitbreaker.New(circuitbreaker.Config{Name: "ig_throttle_test", MaxFailures: 2, ResetTimeout: time.Hour, HalfOpenMaxReqs: 1})
	c := New(Config{GraphBase: srv.URL, CB: cb})
	for range 3 {
		if _, err := c.PublishContainer(t.Context(), "throttled", secretToken, "c1"); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("a 429 = %v, want ErrUnavailable", err)
		}
	}
	if _, err := c.PublishContainer(t.Context(), "other", secretToken, "c2"); err != nil {
		t.Fatalf("another account after three 429s: %v", err)
	}
	if hits.Load() != 4 {
		t.Fatalf("Meta received %d requests, want 4", hits.Load())
	}
}
