package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The race that no test could reach.
//
// Every request from one address writes lastSeen on the same *client, and the
// eviction goroutine reads it. sync.Map synchronises the map, not the struct
// behind the pointer it stores, so those writes were unsynchronised — and
// time.Time is three words, so a torn value hands the evictor a nonsense age:
// either it drops a live client, resetting its buckets and admitting a burst
// that should have been throttled, or it never drops it at all.
//
// The whole suite missed it because cmd/api's harness built the chain with
// middleware.Config{}: RateLimitEnabled was false, RateLimit returned its next
// handler untouched, and the in-process limiter was never entered by any test.
//
// This test is only meaningful under -race, which is what `make test` and the
// verification commands run.
func TestTheInMemoryRateLimiterIsSafeForConcurrentRequestsFromOneAddress(t *testing.T) {
	// The limits are set high enough that nothing is throttled: the subject
	// here is the shared bookkeeping every request performs, not the buckets.
	f := newFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 10_000, RateLimitBurst: 10_000})

	var served atomic.Int64
	handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served.Add(1)
		w.WriteHeader(http.StatusOK)
	}))

	// Every request carries the same RemoteAddr, so they all resolve to one
	// client entry — one *client, written by all of them at once. Different
	// addresses would touch different structs and race with nothing.
	const requests = 64

	var wg sync.WaitGroup
	start := make(chan struct{})
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			handler.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/healthcheck", nil))
		}()
	}
	close(start)
	wg.Wait()

	if got := served.Load(); got != requests {
		t.Errorf("with a burst of 10000 none of the %d requests may be throttled; %d reached the handler", requests, got)
	}
}

// The limiter still limits. Making lastSeen atomic must not have turned the
// bookkeeping into the whole behaviour: a burst past the configured ceiling is
// still refused, from the same address, through the same middleware.
func TestTheInMemoryRateLimiterStillRefusesABurst(t *testing.T) {
	f := newFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 1, RateLimitBurst: 2})

	handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var limited bool
	for range 10 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/healthcheck", nil))
		if w.Code == http.StatusTooManyRequests {
			limited = true
			// One token per second refills, so the client is told to wait one.
			if got := w.Header().Get("Retry-After"); got != "1" {
				t.Errorf("a 429 must say when to retry; Retry-After = %q, want \"1\"", got)
			}
			break
		}
	}

	if !limited {
		t.Error("ten immediate requests against a burst of 2 must be throttled")
	}
}

// The wait is the time one token takes to refill, rounded up to whole seconds
// because that is the unit Retry-After carries.
func TestRetryAfterFollowsTheCeilingsRefillRate(t *testing.T) {
	cases := map[string]struct {
		c    ceiling
		want time.Duration
	}{
		"auth: one token per six seconds": {authCeiling, 6 * time.Second},
		"booking: one per twenty":         {bookingCeiling, 20 * time.Second},
		"general: faster than a second":   {ceiling{name: "gen", rps: 50, burst: 100}, time.Second},
		"disabled rate":                   {ceiling{name: "off", rps: 0, burst: 1}, time.Second},
	}
	for name, tc := range cases {
		if got := tc.c.retryAfter(); got != tc.want {
			t.Errorf("%s: retryAfter = %s, want %s", name, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Rate-limit exemptions
// ---------------------------------------------------------------------------

// The exemption was the prefix "/api/v1/webhooks/", which granted itself to
// every webhook route anybody would ever add. The direction of that mistake is
// the opposite of the CSRF one: a webhook that should have been throttled is
// silently not, and — worse — a provider added under some other path is
// silently throttled, which drops deliveries nobody hears about again.
func TestARateLimitExemptionCoversOneExactRouteAndNothingUnderIt(t *testing.T) {
	tests := []struct {
		method string
		path   string
		exempt bool
		why    string
	}{
		{http.MethodPost, "/api/v1/webhooks/mercadopago", true, "payment notifications"},
		{http.MethodPost, "/api/v1/webhooks/whatsapp", true, "message delivery from Meta"},
		{http.MethodGet, "/api/v1/webhooks/whatsapp", true, "Meta's verification handshake"},
		{http.MethodPost, "/api/v1/webhooks/stripe", false, "the next provider must be added deliberately"},
		{http.MethodPut, "/api/v1/webhooks/mercadopago", false, "the exemption is for the method that is registered"},
		{http.MethodPost, "/api/v1/webhooksomething", false, "not a webhook route at all"},
		{http.MethodGet, "/api/v1/healthcheck", false, "an ordinary route"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			// One token, no refill: the first request spends it, so anything
			// still getting through afterwards got through by exemption.
			f := newFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 0, RateLimitBurst: 1})
			handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			allowed := 0
			for range 5 {
				r := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
				r.RemoteAddr = "203.0.113.9:1000"
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code == http.StatusOK {
					allowed++
				}
			}

			if tt.exempt && allowed != 5 {
				t.Errorf("%s %s must be exempt (%s); only %d of 5 got through",
					tt.method, tt.path, tt.why, allowed)
			}
			if !tt.exempt && allowed != 1 {
				t.Errorf("%s %s must be throttled (%s); %d of 5 got through",
					tt.method, tt.path, tt.why, allowed)
			}
		})
	}
}
