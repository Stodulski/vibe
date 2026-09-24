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

// ---------------------------------------------------------------------------
// Auth ceiling scope
// ---------------------------------------------------------------------------

// TestAuthCeilingOffRoutesAreNotBoundByTheAuthCeiling proves the three
// session/token routes named in authCeilingOffRoutes are throttled by the
// general ceiling only, not by authCeiling's much stricter burst of 10. The
// general ceiling here is set far above what the loop below issues, so every
// one of the 20 requests must succeed; a route still on authCeiling would be
// refused after its 10th.
func TestAuthCeilingOffRoutesAreNotBoundByTheAuthCeiling(t *testing.T) {
	tests := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/auth/refresh"},
		{http.MethodPost, "/api/v1/auth/logout"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			f := newFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 10_000, RateLimitBurst: 10_000})
			handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			allowed := 0
			for range authCeiling.burst * 2 {
				r := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
				r.RemoteAddr = "203.0.113.11:1000"
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code == http.StatusOK {
					allowed++
				}
			}

			if want := authCeiling.burst * 2; allowed != want {
				t.Errorf("%s %s must not be bound by authCeiling (burst %d); got %d/%d through",
					tt.method, tt.path, authCeiling.burst, allowed, want)
			}
		})
	}
}

// TestGetAuthMeDoesNotDrainTheBucketLoginNeeds is the acceptance scenario:
// draining what would have been the auth bucket with GET /auth/me must not
// make POST /auth/login answer 429, because the two no longer share a
// ceiling.
func TestGetAuthMeDoesNotDrainTheBucketLoginNeeds(t *testing.T) {
	f := newFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 10_000, RateLimitBurst: 10_000})
	handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	const addr = "203.0.113.12:1000"

	// Hammer GET /auth/me well past what the old shared auth ceiling
	// (burst 10) would have tolerated.
	for range authCeiling.burst * 3 {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
		r.RemoteAddr = addr
		handler.ServeHTTP(httptest.NewRecorder(), r)
	}

	// Login still gets its full, untouched auth-ceiling burst.
	allowed := 0
	for range authCeiling.burst {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", nil)
		r.RemoteAddr = addr
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code == http.StatusOK {
			allowed++
		}
	}

	if allowed != authCeiling.burst {
		t.Errorf("POST /auth/login got %d/%d of its own burst after GET /auth/me was hammered from the same address; "+
			"the two must not share a bucket", allowed, authCeiling.burst)
	}
}

// TestPostAuthLoginStillHitsTheAuthCeiling is the other half: login itself
// must still be bound by authCeiling, not accidentally exempted along with
// the three session routes.
func TestPostAuthLoginStillHitsTheAuthCeiling(t *testing.T) {
	f := newFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 10_000, RateLimitBurst: 10_000})
	handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	allowed := 0
	for range authCeiling.burst + 5 {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", nil)
		r.RemoteAddr = "203.0.113.13:1000"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code == http.StatusOK {
			allowed++
		}
	}

	if allowed != authCeiling.burst {
		t.Errorf("POST /auth/login must still be bound by authCeiling's burst of %d; got %d through",
			authCeiling.burst, allowed)
	}
}

// ---------------------------------------------------------------------------
// Google redirect rate-limit contract
// ---------------------------------------------------------------------------

// TestRateLimitedGoogleRedirectAnswers303ToFrontendLogin is
// authGoogleRedirect's documented contract (openapi.yaml): every failure,
// rate limiting included, is a 303 to the frontend, never a JSON 429 — this
// route's only caller is a browser following a top-level form navigation
// Google itself posted.
func TestRateLimitedGoogleRedirectAnswers303ToFrontendLogin(t *testing.T) {
	f := newFixture(t, Config{
		RateLimitEnabled: true, RateLimitRPS: 0, RateLimitBurst: 1,
		FrontendURL: "http://localhost:5173",
	})
	handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	const addr = "203.0.113.14:1000"

	req := func() *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/google/redirect", nil)
		r.RemoteAddr = addr
		return r
	}

	// The one-token burst is spent by the first request.
	if w := httptest.NewRecorder(); true {
		handler.ServeHTTP(w, req())
		if w.Code != http.StatusOK {
			t.Fatalf("first request: want 200 to spend the only token; got %d", w.Code)
		}
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req())

	if w.Code != http.StatusSeeOther {
		t.Fatalf("rate-limited google/redirect: want 303; got %d", w.Code)
	}
	if got, want := w.Header().Get("Location"), "http://localhost:5173/login?error=google_rate_limited"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got := w.Header().Get("Retry-After"); got == "" {
		t.Error("want Retry-After set even on the redirect path")
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store — the Location carries a one-time-use error state", got)
	}
}

// TestRateLimitedGoogleRedirectFallsBackTo429WithoutAFrontendURL is the
// degenerate deployment state: with no FrontendURL configured there is
// nowhere to redirect to, so the ordinary 429 problem document stands —
// matching the handler's own FrontendURL-missing behaviour (501) in spirit.
func TestRateLimitedGoogleRedirectFallsBackTo429WithoutAFrontendURL(t *testing.T) {
	f := newFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 0, RateLimitBurst: 1})
	handler := f.mw.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	const addr = "203.0.113.15:1000"

	req := func() *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/google/redirect", nil)
		r.RemoteAddr = addr
		return r
	}

	handler.ServeHTTP(httptest.NewRecorder(), req())

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req())

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("without FrontendURL: want 429; got %d", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got == "" {
		t.Error("want Retry-After set")
	}
}
