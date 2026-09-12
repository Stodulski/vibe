package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// quietRedisLogger silences go-redis's package-level logger. One test points
// the client at a dead server on purpose, and its dial failures are the
// expected outcome rather than something to print eighteen times.
type quietRedisLogger struct{}

func (quietRedisLogger) Printf(context.Context, string, ...any) {}

// The Redis limiter is the only limiter that runs in production — the process
// refuses to boot without Redis — and until these tests existed no test in the
// package ever reached it. Everything below drives it for real: miniredis
// speaks RESP over a socket and runs the Lua, so the script, the key names,
// the TTLs and the arithmetic are exercised, and only the server is a stand-in.

// newRedisLimiterFixture returns a fixture wired to a live miniredis, with a
// clock the test controls. Nothing in these tests sleeps: the bucket refills
// because the clock says so.
func newRedisLimiterFixture(t *testing.T, cfg Config) (*fixture, *miniredis.Miniredis, *time.Time) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		// Fail fast: one test points the client at a dead server on purpose,
		// and the retry backoff would only slow that down.
		MaxRetries: -1,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	f := newFixtureWith(t, cfg, rdb)
	clock := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	f.mw.now = func() time.Time { return clock }

	return f, mr, &clock
}

// send drives one request through the handler and returns its status.
func send(t *testing.T, handler http.Handler, method, path string) int {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	return w.Code
}

// allowedOf counts how many of n immediate requests are let through.
func allowedOf(t *testing.T, handler http.Handler, n int, method, path string) int {
	t.Helper()
	allowed := 0
	for range n {
		if send(t, handler, method, path) == http.StatusOK {
			allowed++
		}
	}
	return allowed
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

// The defect: the Redis limiter compared a one-second counter against
// RateLimitBurst and never read RateLimitRPS at all. The configured sustained
// rate was inert in production, and the effective one was the burst.
//
// With rps=1 and burst=3, a frozen clock must admit exactly 3 and refuse the
// rest, and one second on the clock must buy exactly one more.
func TestTheRedisLimiterEnforcesTheConfiguredRateNotTheBurstPerSecond(t *testing.T) {
	f, _, clock := newRedisLimiterFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 1, RateLimitBurst: 3})
	handler := f.mw.RateLimit(okHandler())

	if got := allowedOf(t, handler, 10, http.MethodGet, "/api/v1/healthcheck"); got != 3 {
		t.Errorf("a burst of 3 must admit exactly 3 of 10 immediate requests; got %d", got)
	}

	// The old fixed window handed out RateLimitBurst afresh every second, so
	// this step used to buy three more requests instead of one.
	*clock = clock.Add(time.Second)
	if got := allowedOf(t, handler, 10, http.MethodGet, "/api/v1/healthcheck"); got != 1 {
		t.Errorf("one second at 1 rps must refill exactly one token; got %d", got)
	}

	*clock = clock.Add(10 * time.Second)
	if got := allowedOf(t, handler, 10, http.MethodGet, "/api/v1/healthcheck"); got != 3 {
		t.Errorf("ten seconds must refill to the burst ceiling of 3, not beyond it; got %d", got)
	}
}

// The auth ceiling was the worst of the two disagreements: Redis allowed 10
// requests per 6-second window — 1.67 attempts per second sustained — where
// the in-memory limiter allowed one attempt per 6 seconds. Ten times the
// credential-stuffing budget, on the login endpoint, in production only.
func TestTheRedisAuthCeilingMatchesTheInProcessOne(t *testing.T) {
	cfg := Config{RateLimitEnabled: true, RateLimitRPS: 1000, RateLimitBurst: 1000}
	f, _, clock := newRedisLimiterFixture(t, cfg)
	handler := f.mw.RateLimit(okHandler())

	if got := allowedOf(t, handler, 30, http.MethodPost, "/api/v1/auth/login"); got != authCeiling.burst {
		t.Errorf("the auth burst is %d attempts; got %d", authCeiling.burst, got)
	}

	// Six seconds is one attempt, not ten.
	*clock = clock.Add(6 * time.Second)
	if got := allowedOf(t, handler, 30, http.MethodPost, "/api/v1/auth/login"); got != 1 {
		t.Errorf("six seconds must buy exactly one login attempt; got %d", got)
	}
}

// A fixed window admits double the limit across its boundary: the last
// requests of one window and the first of the next arrive together. A token
// bucket cannot, and this is the shape of the test that says so.
func TestTheRedisLimiterDoesNotDoubleAtAWindowBoundary(t *testing.T) {
	f, _, clock := newRedisLimiterFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 2, RateLimitBurst: 2})
	handler := f.mw.RateLimit(okHandler())

	// Drain the bucket just before the second ends...
	*clock = clock.Add(999 * time.Millisecond)
	first := allowedOf(t, handler, 5, http.MethodGet, "/api/v1/healthcheck")

	// ...and ask again just after it does.
	*clock = clock.Add(2 * time.Millisecond)
	second := allowedOf(t, handler, 5, http.MethodGet, "/api/v1/healthcheck")

	if total := first + second; total > 2 {
		t.Errorf("2 rps with a burst of 2 must never admit more than the burst across a boundary; got %d", total)
	}
}

// Redis buckets are keyed per address, so one address filling its bucket must
// not touch another's. This is the other half of the forgery in
// httpx.ClientIP: a shared key would let anyone lock anyone out.
func TestTheRedisLimiterKeysPerAddress(t *testing.T) {
	f, _, _ := newRedisLimiterFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 1, RateLimitBurst: 1})
	handler := f.mw.RateLimit(okHandler())

	drain := func(remoteAddr string) int {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/healthcheck", nil)
		r.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	if got := drain("203.0.113.7:1000"); got != http.StatusOK {
		t.Fatalf("the first request from an address must be allowed; got %d", got)
	}
	if got := drain("203.0.113.7:1001"); got != http.StatusTooManyRequests {
		t.Errorf("the second request from the same address must be refused; got %d", got)
	}
	if got := drain("198.51.100.4:1000"); got != http.StatusOK {
		t.Errorf("a different address has its own bucket; got %d", got)
	}
}

// Webhooks are delivered by MercadoPago and Meta, not by clients, and are
// authenticated by signature. Throttling them drops payment notifications.
func TestTheRedisLimiterLeavesWebhooksAlone(t *testing.T) {
	f, _, _ := newRedisLimiterFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 1, RateLimitBurst: 1})
	handler := f.mw.RateLimit(okHandler())

	if got := allowedOf(t, handler, 20, http.MethodPost, "/api/v1/webhooks/mercadopago"); got != 20 {
		t.Errorf("webhook deliveries must not be throttled; %d of 20 got through", got)
	}
}

// When Redis stops answering, the limiter degrades to the in-process buckets
// rather than waving everything through. Failing open here means an attacker
// who catches a Redis blip gets an unlimited login endpoint.
func TestTheRedisLimiterFallsBackToTheInProcessBucketsWhenRedisIsDown(t *testing.T) {
	// The fallback is golang.org/x/time/rate, which reads the real clock, so
	// the sustained rate is set low enough that nothing refills while the
	// failing dials take their time.
	f, mr, _ := newRedisLimiterFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 0.01, RateLimitBurst: 2})
	handler := f.mw.RateLimit(okHandler())

	mr.Close() // Redis is gone.

	if got := allowedOf(t, handler, 6, http.MethodGet, "/api/v1/healthcheck"); got != 2 {
		t.Errorf("with redis down the in-process burst of 2 must still apply; %d of 6 got through", got)
	}
	if f.logs.Len() == 0 {
		t.Error("a redis failure inside the limiter must be logged")
	}
}

// budgetHook records the deadline the limiter gave one Redis call.
type budgetHook struct{ budget time.Duration }

func (h *budgetHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *budgetHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if deadline, ok := ctx.Deadline(); ok {
			h.budget = time.Until(deadline)
		}
		return next(ctx, cmd)
	}
}

func (h *budgetHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

// The client is configured with a 3s read timeout and 3 retries, so a Redis
// that accepts connections and then stops answering stalls a request for
// something close to thirteen seconds — inside the rate limiter, which every
// request passes through, and long after http.Server's 10s WriteTimeout has
// abandoned the response.
func TestTheRedisLimiterCallIsBounded(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	hook := &budgetHook{}
	rdb.AddHook(hook)

	f := newFixtureWith(t, Config{RateLimitEnabled: true, RateLimitRPS: 10, RateLimitBurst: 10}, rdb)
	send(t, f.mw.RateLimit(okHandler()), http.MethodGet, "/api/v1/healthcheck")

	if hook.budget <= 0 {
		t.Fatal("the limiter's Redis call ran on a context with no deadline at all")
	}
	if hook.budget > redisLimiterTimeout {
		t.Errorf("the budget must be at most %s; got %s", redisLimiterTimeout, hook.budget)
	}
}

// The limiter's key comes from the trusted-proxy set, not from whatever the
// caller typed. This is the attack joined up end to end: rotate the header,
// get a fresh bucket, and every limiter in the chain is a formality.
func TestAForgedForwardedHeaderCannotMintBucketsThroughTheLimiter(t *testing.T) {
	f, _, _ := newRedisLimiterFixture(t, Config{RateLimitEnabled: true, RateLimitRPS: 1, RateLimitBurst: 1})
	handler := f.mw.RateLimit(okHandler())

	// No proxy is trusted, so the header is exactly what an attacker on an
	// exposed deployment controls.
	rotate := func(forwarded string) int {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/healthcheck", nil)
		r.RemoteAddr = "203.0.113.7:1000"
		r.Header.Set("X-Forwarded-For", forwarded)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	if got := rotate("1.1.1.1"); got != http.StatusOK {
		t.Fatalf("the first request must be allowed; got %d", got)
	}
	if got := rotate("2.2.2.2"); got != http.StatusTooManyRequests {
		t.Errorf("a rotated forwarded header must not mint a fresh bucket; got %d", got)
	}
}

// And the other direction: behind a proxy the set does name, two clients
// sharing that proxy must not share a bucket.
func TestTwoClientsBehindATrustedProxyGetSeparateBuckets(t *testing.T) {
	cfg := Config{
		TrustedProxies:   httpx.DefaultTrustedProxies(),
		RateLimitEnabled: true, RateLimitRPS: 1, RateLimitBurst: 1,
	}
	f, _, _ := newRedisLimiterFixture(t, cfg)
	handler := f.mw.RateLimit(okHandler())

	fromClient := func(client string) int {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/healthcheck", nil)
		r.RemoteAddr = "10.0.0.1:443"
		r.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	if got := fromClient("203.0.113.7"); got != http.StatusOK {
		t.Fatalf("the first client must be allowed; got %d", got)
	}
	if got := fromClient("203.0.113.7"); got != http.StatusTooManyRequests {
		t.Errorf("the same client must be throttled; got %d", got)
	}
	if got := fromClient("198.51.100.4"); got != http.StatusOK {
		t.Errorf("a different client behind the same proxy has its own bucket; got %d", got)
	}
}

// ---------------------------------------------------------------------------
// The per-account ceiling
// ---------------------------------------------------------------------------

// asUser drives one request that already carries an authenticated account,
// which is the state Authenticate leaves the request in before RateLimitUser
// sees it.
func asUser(t *testing.T, handler http.Handler, id uuid.UUID) int {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/complexes", nil)
	handler.ServeHTTP(w, httpx.ContextSetUser(r, &authstore.User{ID: id, IsActive: true}))
	return w.Code
}

func allowedAsUser(t *testing.T, handler http.Handler, id uuid.UUID, n int) int {
	t.Helper()
	allowed := 0
	for range n {
		if asUser(t, handler, id) == http.StatusOK {
			allowed++
		}
	}
	return allowed
}

// Every other ceiling counts by address, so one account arriving from many
// addresses was never limited at all. The account id is the key here, which is
// what makes those requests one bucket.
func TestThePerAccountCeilingCountsOneAccountAcrossAddresses(t *testing.T) {
	f, _, clock := newRedisLimiterFixture(t, Config{
		RateLimitEnabled: true, RateLimitRPS: 10_000, RateLimitBurst: 10_000,
		RateLimitUserRPS: 1, RateLimitUserBurst: 3,
	})
	handler := f.mw.RateLimitUser(okHandler())
	user := uuid.New()

	// The address changes on every request as far as any address-keyed bucket
	// is concerned — httptest gives them all the same RemoteAddr, and it makes
	// no difference, because this ceiling never reads it.
	if got := allowedAsUser(t, handler, user, 10); got != 3 {
		t.Errorf("a burst of 3 must admit exactly 3 of 10 immediate requests; got %d", got)
	}

	*clock = clock.Add(time.Second)
	if got := allowedAsUser(t, handler, user, 5); got != 1 {
		t.Errorf("one second at 1 rps must buy exactly one more request; got %d", got)
	}
}

// Two accounts behind one address are two buckets, which is the half of this
// the address-keyed limiter gets wrong in the other direction.
func TestThePerAccountCeilingKeepsAccountsApart(t *testing.T) {
	f, _, _ := newRedisLimiterFixture(t, Config{
		RateLimitEnabled: true, RateLimitUserRPS: 1, RateLimitUserBurst: 2,
	})
	handler := f.mw.RateLimitUser(okHandler())

	first, second := uuid.New(), uuid.New()
	if got := allowedAsUser(t, handler, first, 5); got != 2 {
		t.Fatalf("the first account must spend its own burst of 2; got %d", got)
	}
	if got := allowedAsUser(t, handler, second, 5); got != 2 {
		t.Errorf("the second account must have a full burst of its own; got %d", got)
	}
}

// An anonymous request is not exempt from throttling — it is throttled by
// address, further out in the chain — so this middleware must not refuse it.
func TestThePerAccountCeilingIgnoresAnonymousRequests(t *testing.T) {
	f, _, _ := newRedisLimiterFixture(t, Config{
		RateLimitEnabled: true, RateLimitUserRPS: 1, RateLimitUserBurst: 1,
	})
	handler := f.mw.RateLimitUser(okHandler())

	for range 20 {
		if got := send(t, handler, http.MethodGet, "/api/v1/complexes"); got != http.StatusOK {
			t.Fatalf("an anonymous request must pass this ceiling untouched; got %d", got)
		}
	}
}

// Redis is where the shared count lives, so the key has to carry the
// environment: a staging deployment pointed at a production Redis would
// otherwise spend production's tokens and 429 real customers.
func TestRateLimitKeysAreNamespacedByEnvironment(t *testing.T) {
	f, mr, _ := newRedisLimiterFixture(t, Config{
		RateLimitEnabled: true, Env: "staging",
		RateLimitUserRPS: 1, RateLimitUserBurst: 1,
	})
	handler := f.mw.RateLimitUser(okHandler())
	user := uuid.New()

	if got := asUser(t, handler, user); got != http.StatusOK {
		t.Fatalf("the first request must be admitted; got %d", got)
	}

	want := "vibe:staging:rl:user:" + user.String()
	if !mr.Exists(want) {
		t.Errorf("want the bucket at %q; got keys %v", want, mr.Keys())
	}
}

// The fallback matters more here than anywhere: with Redis unreachable the
// per-account ceiling still has to exist, or an account that can make Redis
// slow is an account with no ceiling at all.
func TestThePerAccountCeilingFallsBackToTheInProcessBuckets(t *testing.T) {
	// The in-process buckets read the wall clock rather than the fixture's,
	// and each refused request spends the 150ms Redis budget first, so the
	// refill rate is set low enough that no bucket refills while the test runs.
	f, mr, _ := newRedisLimiterFixture(t, Config{
		RateLimitEnabled: true, RateLimitUserRPS: 0.001, RateLimitUserBurst: 2,
	})
	mr.Close()

	handler := f.mw.RateLimitUser(okHandler())
	if got := allowedAsUser(t, handler, uuid.New(), 6); got != 2 {
		t.Errorf("the in-process fallback must still enforce the burst of 2; got %d", got)
	}
}

// A Config that never set the knob gets no ceiling rather than a ceiling of
// zero, which would refuse every signed-in request.
func TestThePerAccountCeilingIsOffWhenUnconfigured(t *testing.T) {
	f, _, _ := newRedisLimiterFixture(t, Config{RateLimitEnabled: true})
	handler := f.mw.RateLimitUser(okHandler())

	if got := allowedAsUser(t, handler, uuid.New(), 20); got != 20 {
		t.Errorf("an unconfigured per-account ceiling must admit everything; got %d of 20", got)
	}
}
