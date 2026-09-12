package middleware

import (
	"context"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"github.com/stodulski/vibe-server/internal/httpx"
	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
)

// ---------------------------------------------------------------------------
// The ceilings
// ---------------------------------------------------------------------------

// ceiling is one rate limit: a sustained rate, and the burst above it that is
// tolerated before requests are refused.
//
// It exists because the two backends used to disagree about what a limit even
// was. Redis counted requests inside a fixed one-second window and compared
// the count against RateLimitBurst — it never read RateLimitRPS at all, so
// `-limiter-rps` was inert in production, and the effective sustained rate was
// the burst: 20 rps where 10 was configured. Auth was worse: a 6-second window
// admitting 10 requests is 1.67 rps sustained, against the in-memory limiter's
// one attempt per 6 seconds — a factor of ten on the login endpoint.
//
// A fixed window is also wrong on its own terms: ten requests at 5.9s and ten
// more at 6.1s are twenty requests in 200ms, and the counter is happy.
//
// One definition, one shape of enforcement, both backends. A token bucket is
// what golang.org/x/time/rate already implements, so making Redis match it
// means the fallback and the distributed path enforce the same thing rather
// than two things that happen to be named alike.
type ceiling struct {
	// name is the key suffix and the log label.
	name string
	// rps is the sustained refill rate, in tokens per second.
	rps float64
	// burst is the bucket size: how far ahead of the sustained rate a client
	// may run before it is refused.
	burst int
}

// retryAfter is how long a refused client should wait before trying again:
// the time one token takes to refill, rounded up to whole seconds because
// that is what the Retry-After header carries. A 429 without it leaves a
// well-behaved client guessing, and a guess that is too short is just more
// refused requests.
func (c ceiling) retryAfter() time.Duration {
	if c.rps <= 0 {
		return time.Second
	}
	wait := time.Duration(math.Ceil(1/c.rps)) * time.Second
	if wait < time.Second {
		return time.Second
	}
	return wait
}

// The auth and booking ceilings are not configurable and are stated here as
// rate-and-burst rather than as window sizes. They reproduce exactly what the
// in-memory limiter has always enforced, which is the stricter of the two
// previous readings: the login endpoint is where credential stuffing lands,
// and 1.67 attempts per second per address is not a limit.
var (
	authCeiling    = ceiling{name: "auth", rps: 1.0 / 6.0, burst: 10}
	bookingCeiling = ceiling{name: "book", rps: 1.0 / 20.0, burst: 3}
)

// ttlMillis is how long an idle bucket is kept: long enough to refill
// completely, after which a missing key and a full bucket are the same thing.
//
// A non-positive rate never refills, so there is no such moment; the bucket is
// kept for the same three minutes the in-process one uses. That configuration
// blocks every address after its first burst and is a mistake either way, but
// it must not produce an infinite TTL.
func (c ceiling) ttlMillis() int64 {
	if c.rps <= 0 {
		return int64(clientTTL / time.Millisecond)
	}
	return int64(math.Ceil(float64(c.burst)/c.rps))*1000 + 1000
}

// generalCeiling is the operator-configured limit that applies to every
// non-webhook request.
func (m *Middleware) generalCeiling() ceiling {
	return ceiling{name: "gen", rps: m.cfg.RateLimitRPS, burst: m.cfg.RateLimitBurst}
}

// userCeiling is the operator-configured limit keyed on the authenticated
// account rather than on the address.
//
// Every other ceiling here counts by client address, which is the wrong unit
// for the thing an account can do. One signed-in owner script running from a
// dozen addresses — a cloud function, a rotating proxy, a phone moving between
// networks — stays under every address bucket while costing the database a
// dozen times what one client should, and the audit trail shows one account
// doing it. Conversely a NAT collapses a whole office into one address bucket
// and throttles them all together. The two keys answer different questions and
// this repository only had one of them.
func (m *Middleware) userCeiling() ceiling {
	return ceiling{name: "user", rps: m.cfg.RateLimitUserRPS, burst: m.cfg.RateLimitUserBurst}
}

// ceilingsFor returns the limits that apply to a request, cheapest first. The
// slices are package-level so the hot path allocates nothing.
var (
	generalOnly    = []int{indexGeneral}
	generalAndAuth = []int{indexGeneral, indexAuth}
	generalAndBook = []int{indexGeneral, indexBooking}
)

const (
	indexGeneral = iota
	indexAuth
	indexBooking
	// indexUser is not in any ceilingsFor slice: it is enforced by
	// RateLimitUser, which runs further in than the others because the account
	// it keys on is not known until Authenticate has resolved it.
	indexUser

	ceilingCount
)

func ceilingsFor(r *http.Request) []int {
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/v1/auth/"):
		return generalAndAuth
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/book":
		return generalAndBook
	default:
		return generalOnly
	}
}

// rateLimitExemptRoutes names the exact routes that are outside rate limiting,
// each with the reason. The key is "METHOD /path", written exactly as the route
// is registered.
//
// The reason is the same for all three, and it is a deliberate exemption rather
// than an oversight: these are called by MercadoPago and Meta, not by clients.
// Their delivery bursts are not ours to throttle, and both providers stop
// redelivering after enough refusals — a throttled webhook is a payment we never
// hear about again.
//
// It was "every path under /api/v1/webhooks/", which granted the exemption to
// routes nobody had written yet. The direction of that mistake is the opposite
// of the CSRF one and just as invisible: the next webhook added here inherits
// no throttling silently, and a webhook added under some other prefix inherits
// throttling just as silently and drops deliveries.
//
// TestEveryWebhookRouteDeclaresItsRateLimitPosition in cmd/api is what forces
// the next one to be a decision.
var rateLimitExemptRoutes = map[string]string{
	"POST /api/v1/webhooks/mercadopago": "payment notifications; MercadoPago gives up after enough refusals",
	"GET /api/v1/webhooks/whatsapp":     "Meta's verification handshake",
	"POST /api/v1/webhooks/whatsapp":    "message delivery from Meta, at Meta's pace",
}

// RateLimitExemptRoutes returns the routes outside rate limiting, keyed by
// "METHOD /path" with the reason as the value.
//
// It is exported for the route audit in cmd/api, which is the only place that
// can see the registered route table and this list at the same time.
func RateLimitExemptRoutes() map[string]string { return maps.Clone(rateLimitExemptRoutes) }

// rateLimitExempt reports whether a route is outside rate limiting entirely.
func rateLimitExempt(method, path string) bool {
	_, exempt := rateLimitExemptRoutes[method+" "+path]
	return exempt
}

// RateLimit throttles by client address, backed by Redis when it is
// configured so the limit is shared across instances, and by an in-process
// limiter otherwise.
func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	if !m.cfg.RateLimitEnabled {
		return next
	}

	buckets := newLocalBuckets(m.ceilings(), m.logger)
	go buckets.evict(m.shutdown)

	if m.rdb != nil {
		return m.rateLimitRedis(next, buckets)
	}
	return m.rateLimitLocal(next, buckets)
}

// ceilings returns the limits in index order.
func (m *Middleware) ceilings() [ceilingCount]ceiling {
	return [ceilingCount]ceiling{m.generalCeiling(), authCeiling, bookingCeiling, m.userCeiling()}
}

// ---------------------------------------------------------------------------
// Redis backend
// ---------------------------------------------------------------------------

// redisTokenBucket is the same token bucket golang.org/x/time/rate implements,
// as one atomic script: refill by elapsed time, cap at the burst, spend one.
//
// Fractional tokens are written with tostring because redis.call converts a
// Lua number argument to an integer, which would truncate every partial refill
// to zero and stall the bucket below one token per whole second.
var redisTokenBucket = redis.NewScript(`
	local key    = KEYS[1]
	local rate   = tonumber(ARGV[1])
	local burst  = tonumber(ARGV[2])
	local now    = tonumber(ARGV[3])
	local ttl    = tonumber(ARGV[4])

	local state  = redis.call("HMGET", key, "tokens", "at")
	local tokens = tonumber(state[1])
	local at     = tonumber(state[2])

	if tokens == nil or at == nil then
		tokens = burst
		at = now
	end

	local elapsed = now - at
	if elapsed < 0 then
		elapsed = 0
	end

	tokens = tokens + elapsed * rate
	if tokens > burst then
		tokens = burst
	end

	local allowed = 0
	if tokens >= 1 then
		tokens = tokens - 1
		allowed = 1
	end

	redis.call("HSET", key, "tokens", tostring(tokens), "at", tostring(now))
	redis.call("PEXPIRE", key, ttl)
	return allowed
`)

// redisLimiterTimeout bounds one bucket check.
//
// The client is configured with a 3s read timeout and 3 retries, so a Redis
// that accepts connections and then stops answering stalls a request for
// something close to thirteen seconds — inside the rate limiter, which every
// request passes through, while http.Server's 10s WriteTimeout has already
// abandoned the response. A budget of 150ms is roughly fifty times the round
// trip this call actually takes; anything slower is a Redis that is not
// serving, and waiting longer does not make it serve.
const redisLimiterTimeout = 150 * time.Millisecond

// rateLimitRedis enforces the ceilings through the shared token buckets, and
// falls back to the in-process buckets when Redis cannot answer.
//
// The fallback is the point. Failing open on a Redis blip means an attacker
// who can make Redis slow — or who simply arrives during one — gets an
// unlimited login endpoint. Failing closed means a Redis blip is a full
// outage. Degrading to per-instance limiting keeps a real ceiling in place;
// it is looser than the shared one by the number of instances, which is a
// price worth paying for a limit that still exists.
func (m *Middleware) rateLimitRedis(next http.Handler, fallback *localBuckets) http.Handler {
	ceilings := m.ceilings()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rateLimitExempt(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ip := m.clientIP(r)

		for _, index := range ceilingsFor(r) {
			c := ceilings[index]

			allowed, err := m.allowRedis(r.Context(), ip, c)
			if err != nil {
				m.logger.Error("rate limit: redis unavailable, falling back to the in-process limiter",
					"error", err, "ceiling", c.name)
				allowed = fallback.allow(ip, index)
			}
			if !allowed {
				m.respond.RateLimitExceededAfter(w, r, c.retryAfter())
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// allowRedis spends one token from the shared bucket for subject — a client
// address for the address-keyed ceilings, an account id for the per-user one.
func (m *Middleware) allowRedis(ctx context.Context, subject string, c ceiling) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, redisLimiterTimeout)
	defer cancel()

	key := m.rateLimitKey(c.name, subject)

	allowed, err := redisTokenBucket.Run(ctx, m.rdb, []string{key},
		strconv.FormatFloat(c.rps/1000, 'f', -1, 64), // tokens per millisecond
		strconv.Itoa(c.burst),
		strconv.FormatInt(m.now().UnixMilli(), 10),
		strconv.FormatInt(c.ttlMillis(), 10),
	).Int64()
	if err != nil {
		return false, err
	}
	return allowed == 1, nil
}

// rateLimitKey is the Redis key one bucket lives under.
//
// The environment is in the key for the same reason it is in the idempotency
// keys (see idempotency.go): a staging deployment pointed at a production
// Redis would otherwise spend production's tokens, and the first sign of it is
// real customers getting 429s for traffic they never sent.
func (m *Middleware) rateLimitKey(name, key string) string {
	return platformredis.KeyPrefix(m.cfg.Env) + "rl:" + name + ":" + key
}

// ---------------------------------------------------------------------------
// In-process backend
// ---------------------------------------------------------------------------

// clientTTL is how long an address's buckets are kept after its last request.
const clientTTL = 3 * time.Minute

// maxTrackedClients bounds the in-process map.
//
// It was unbounded, keyed on a string an attacker could choose freely — see
// httpx.ClientIP. The key is a parsed address now, so filling the map costs
// real source addresses, but "expensive" is not "bounded": every distinct
// address seen in a three-minute window held three rate.Limiters until the
// sweeper ran.
//
// At capacity this sheds addresses it has not seen before, rather than
// evicting one it has. Evicting to make room is what an attacker wants: a
// rotating flood would push every real client out of the map and hand each of
// them a fresh full bucket. Shedding is a visible refusal for new addresses
// while the flood lasts; the alternative is an out-of-memory kill, which
// refuses everybody permanently.
//
// 20k entries is roughly 8MB. This backend is the fallback — the process
// refuses to boot without Redis — so in production it holds only the addresses
// seen while Redis was failing.
const maxTrackedClients = 20_000

// clientBuckets is one key's token buckets, one per ceiling.
type clientBuckets struct {
	limiters [ceilingCount]*rate.Limiter
	// lastSeen is Unix nanoseconds behind an atomic rather than a time.Time,
	// because it is the one field here that is genuinely shared: every request
	// from an address writes it, on the same clientBuckets, while the eviction
	// goroutine below reads it.
	//
	// sync.Map synchronises the map, not the struct its values point at, so a
	// plain time.Time field was an unsynchronised write/write and read/write
	// race — confirmed by the detector on two concurrent same-IP requests.
	// time.Time is three words wide, so a torn read hands the evictor a
	// nonsense age: either it drops a live client, resetting its buckets and
	// admitting a burst that should have been throttled, or it never drops it
	// at all.
	lastSeen atomic.Int64
}

// localBuckets is the in-process limiter state.
type localBuckets struct {
	clients sync.Map // string -> *clientBuckets
	size    atomic.Int64
	// max is maxTrackedClients, as a field so a test can reach the shedding
	// behaviour without allocating twenty thousand entries to get there.
	max      int
	ceilings [ceilingCount]ceiling
	logger   *slog.Logger
	// warnedFull reports the capacity shed once rather than per request.
	warnedFull sync.Once
}

func newLocalBuckets(ceilings [ceilingCount]ceiling, logger *slog.Logger) *localBuckets {
	return &localBuckets{max: maxTrackedClients, ceilings: ceilings, logger: logger}
}

// allow spends one token from the address's bucket for the given ceiling, and
// reports false when the address is refused — either because the bucket is
// empty or because the map is full and this address is not in it.
func (b *localBuckets) allow(ip string, index int) bool {
	buckets := b.get(ip)
	if buckets == nil {
		return false
	}
	return buckets.limiters[index].Allow()
}

// get returns the address's buckets, creating them if there is room.
func (b *localBuckets) get(ip string) *clientBuckets {
	now := time.Now().UnixNano()

	if existing, ok := b.clients.Load(ip); ok {
		buckets, _ := existing.(*clientBuckets)
		buckets.lastSeen.Store(now)
		return buckets
	}

	if b.size.Load() >= int64(b.max) {
		b.warnedFull.Do(func() {
			b.logger.Warn("rate limit: the in-process client table is full; "+
				"addresses not already tracked are being refused until it drains",
				"capacity", b.max)
		})
		return nil
	}

	fresh := &clientBuckets{}
	for i, c := range b.ceilings {
		fresh.limiters[i] = rate.NewLimiter(rate.Limit(c.rps), c.burst)
	}
	// Stamped before it is published, not after: a client stored with a zero
	// lastSeen is, for as long as it takes to reach the line below, infinitely
	// old to the evictor.
	fresh.lastSeen.Store(now)

	stored, loaded := b.clients.LoadOrStore(ip, fresh)
	if !loaded {
		b.size.Add(1)
	}
	buckets, _ := stored.(*clientBuckets)
	buckets.lastSeen.Store(now)
	return buckets
}

// evict drops addresses that have gone quiet, until shutdown.
func (b *localBuckets) evict(shutdown <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixNano()
			b.clients.Range(func(key, value any) bool {
				buckets, _ := value.(*clientBuckets)
				if now-buckets.lastSeen.Load() > int64(clientTTL) {
					if _, loaded := b.clients.LoadAndDelete(key); loaded {
						b.size.Add(-1)
					}
				}
				return true
			})
		case <-shutdown:
			return
		}
	}
}

// rateLimitLocal enforces the ceilings in this process only. It is correct for
// a single instance and is what runs when Redis is not configured.
func (m *Middleware) rateLimitLocal(next http.Handler, buckets *localBuckets) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rateLimitExempt(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ip := m.clientIP(r)

		for _, index := range ceilingsFor(r) {
			if !buckets.allow(ip, index) {
				m.respond.RateLimitExceededAfter(w, r, buckets.ceilings[index].retryAfter())
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Per-account ceiling
// ---------------------------------------------------------------------------

// RateLimitUser applies the per-account ceiling to a request that carries an
// authenticated user, and does nothing to one that does not.
//
// It is a separate middleware from RateLimit because it has to sit inside
// Authenticate — see Wrap. An anonymous request is not exempt from throttling;
// it is throttled by address, further out, before it cost anything.
//
// The account id is the key, so the same account is counted together however
// many addresses it arrives from, and two accounts behind one NAT are counted
// apart.
func (m *Middleware) RateLimitUser(next http.Handler) http.Handler {
	if !m.cfg.RateLimitEnabled {
		return next
	}

	c := m.userCeiling()
	// A non-positive rate is "not configured", not "refuse everything". The
	// environment loader already rejects LIMITER_USER_RPS <= 0, so the only
	// way here is a Config built in code or an explicit -limiter-user-rps=0;
	// the general ceiling's zero-burst reading ("refuse everything") would
	// turn either of those into a total outage for signed-in callers, which is
	// not a failure a new knob should be able to cause.
	if c.rps <= 0 || c.burst <= 0 {
		m.logger.Warn("rate limit: the per-account ceiling is not configured and is off; " +
			"set LIMITER_USER_RPS and LIMITER_USER_BURST to bound one account across many addresses")
		return next
	}

	buckets := newLocalBuckets(m.ceilings(), m.logger)
	go buckets.evict(m.shutdown)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := httpx.ContextGetAuthenticatedUser(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		key := user.ID.String()

		allowed := buckets.allow(key, indexUser)
		if m.rdb != nil {
			shared, err := m.allowRedis(r.Context(), key, c)
			if err != nil {
				// Same degradation as the address-keyed limiter: a Redis blip
				// must not leave an account with no ceiling, so the answer
				// already computed from the in-process buckets stands.
				m.logger.Error("rate limit: redis unavailable, falling back to the in-process limiter",
					"error", err, "ceiling", c.name)
			} else {
				allowed = shared
			}
		}

		if !allowed {
			m.respond.RateLimitExceededAfter(w, r, c.retryAfter())
			return
		}
		next.ServeHTTP(w, r)
	})
}
