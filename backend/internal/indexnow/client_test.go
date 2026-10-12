package indexnow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/jobs"
)

func TestSubmitPostsTheExactIndexNowBody(t *testing.T) {
	var got map[string]any
	var method, contentType string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("body is not JSON: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	urls := []string{ProductionOrigin + "/c/club-norte", ProductionOrigin + "/canchas"}
	if err := c.Submit(t.Context(), urls); err != nil {
		t.Fatalf("Submit() = %v, want nil", err)
	}

	if method != http.MethodPost {
		t.Errorf("method = %s, want POST", method)
	}
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("content type = %q", contentType)
	}
	want := map[string]any{
		"host":        "vibe.com.ar",
		"key":         "469b1eba6dac3b5f5da13d17ea885719",
		"keyLocation": "https://vibe.com.ar/469b1eba6dac3b5f5da13d17ea885719.txt",
		"urlList":     []any{urls[0], urls[1]},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %v, want %v", got, want)
	}
}

func TestTheStatusCodeDecidesWhatTheQueueDoesNext(t *testing.T) {
	tests := []struct {
		status int
		want   jobs.Outcome
	}{
		{http.StatusOK, jobs.OutcomeDone},
		{http.StatusAccepted, jobs.OutcomeDone},
		{http.StatusBadRequest, jobs.OutcomePermanent},
		{http.StatusForbidden, jobs.OutcomePermanent},
		{http.StatusUnprocessableEntity, jobs.OutcomePermanent},
		{http.StatusTooManyRequests, jobs.OutcomeRetry},
		{http.StatusInternalServerError, jobs.OutcomeRetry},
		{http.StatusServiceUnavailable, jobs.OutcomeRetry},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			})
			err := c.Submit(t.Context(), []string{ProductionOrigin + "/canchas"})
			if got := jobs.Classify(err); got != tt.want {
				t.Fatalf("status %d: outcome %v, want %v (err %v)", tt.status, got, tt.want, err)
			}
		})
	}
}

func TestANetworkFailureIsRetried(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	c := NewClient(ProductionOrigin, circuitbreaker.New(circuitbreaker.Config{Name: "indexnow-test", MaxFailures: 3, ResetTimeout: time.Minute}))
	c.endpoint = url

	if got := jobs.Classify(c.Submit(t.Context(), []string{ProductionOrigin + "/canchas"})); got != jobs.OutcomeRetry {
		t.Fatalf("a dropped connection must be retried, got outcome %v", got)
	}
}

func TestAnOpenBreakerIsNotAttempted(t *testing.T) {
	hits := 0
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	})
	c.cb = circuitbreaker.New(circuitbreaker.Config{Name: "indexnow-test", MaxFailures: 1, ResetTimeout: time.Minute})

	// The first failure opens the circuit; the second call must not reach the endpoint.
	_ = c.Submit(t.Context(), []string{ProductionOrigin + "/canchas"})
	err := c.Submit(t.Context(), []string{ProductionOrigin + "/canchas"})

	if hits != 1 {
		t.Fatalf("endpoint was hit %d times, want 1: an open circuit must not send", hits)
	}
	if got := jobs.Classify(err); got != jobs.OutcomeNotAttempted {
		t.Fatalf("open circuit: outcome %v, want NotAttempted", got)
	}
}

func TestTheClientHasAnExplicitTimeout(t *testing.T) {
	c := NewClient(ProductionOrigin, nil)
	if c.http.Timeout != requestTimeout || requestTimeout <= 0 {
		t.Fatalf("timeout = %v, want the explicit %v", c.http.Timeout, requestTimeout)
	}
}
