// Package middleware holds the chain every request passes through before it
// reaches a handler: panic recovery, request ids, security headers, rate
// limiting, CSRF, and authentication.
//
// The order matters and is fixed by the application: the request id first so
// every later log line can be joined to the id the client holds, request
// logging immediately inside it so every request leaves a line whatever
// happens below, recovery next so it catches everything under it, then the
// checks that can reject. Authentication is last, because the cheaper
// rejections should not cost a database read. See Wrap.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/auth"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// UserReader loads the account a session belongs to.
type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*authstore.User, error)
}

// ComplexReader loads the complex an ownership-scoped route names.
type ComplexReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error)
}

// TokenVerifier verifies the access and CSRF tokens on a request.
type TokenVerifier interface {
	ValidateAccessToken(tokenString string) (*auth.Claims, error)
	ValidateCSRFToken(accessToken, csrfToken string) bool
}

// Blacklist reports whether an otherwise-valid access token has been revoked.
type Blacklist interface {
	IsBlacklisted(ctx context.Context, rawToken string, userID uuid.UUID, issuedAt time.Time) bool
}

// Config is what the chain needs from application configuration.
type Config struct {
	// TrustedProxies names the peers allowed to rewrite the client address
	// through X-Forwarded-For. The zero value trusts nobody, which is correct
	// with no proxy in front: the header is then attacker-supplied, and
	// trusting it lets anyone forge the address rate limiting is keyed on and
	// the audit trail records.
	TrustedProxies httpx.TrustedProxies
	// RateLimitEnabled turns rate limiting on.
	RateLimitEnabled bool
	// RateLimitRPS and RateLimitBurst shape the general limiter: a sustained
	// rate and the burst above it that is tolerated. Both backends enforce
	// exactly these — see ratelimit.go.
	RateLimitRPS   float64
	RateLimitBurst int
	// RequestLogSample thins the request log: 0 or 1 logs every request, N
	// logs one successful request in N. Failures and slow requests are never
	// sampled away — see shouldLog. The counters behind Metrics are never
	// sampled at all, so raising this costs detail, never accuracy.
	RequestLogSample int
}

// Middleware builds the chain.
type Middleware struct {
	users     UserReader
	complexes ComplexReader
	tokens    TokenVerifier
	blacklist Blacklist
	rdb       *redis.Client
	respond   *httpx.Responder
	logger    *slog.Logger
	cfg       Config
	// shutdown stops the in-memory limiter's eviction loop.
	shutdown <-chan struct{}
	// now is the clock the Redis token bucket, the cache-failure reporter,
	// and request-latency logging read. It is a field so a test can advance
	// time without sleeping; production always gets time.Now.
	now func() time.Time
	// cacheReports throttles user-cache failure reporting — see usercache.go.
	cacheReports *cacheReports
	// warnedUntrustedForward makes the "forwarded header from an untrusted
	// peer" warning fire once per process rather than once per request.
	warnedUntrustedForward sync.Once
	// metrics is the request surface LogRequests fills in and Metrics reads.
	// It is a pointer so that Middleware itself stays copyable-by-mistake-safe
	// in the same way as before: the atomics live behind it, not in it.
	metrics *metrics
}

// Dependencies groups what New needs.
type Dependencies struct {
	Users     UserReader
	Complexes ComplexReader
	Tokens    TokenVerifier
	Blacklist Blacklist
	// Redis enables distributed rate limiting and the user cache. A nil client
	// falls back to in-process behaviour, which is correct for one instance.
	Redis   *redis.Client
	Respond *httpx.Responder
	Logger  *slog.Logger
	// Shutdown stops background goroutines during a graceful stop.
	Shutdown <-chan struct{}
}

// New returns a Middleware.
func New(d Dependencies, cfg Config) *Middleware {
	return &Middleware{
		users:     d.Users,
		complexes: d.Complexes,
		tokens:    d.Tokens,
		blacklist: d.Blacklist,
		rdb:       d.Redis,
		respond:   d.Respond,
		logger:    d.Logger,
		cfg:       cfg,
		shutdown:  d.Shutdown,
		now:       time.Now,
		metrics:   &metrics{},

		cacheReports: newCacheReports(),
	}
}

// clientIP resolves the address this request is attributed to, and says so
// once if a forwarded header arrived from a peer the trusted-proxy set does
// not cover.
//
// That combination is either an attempted forgery or — far more likely, and
// far more damaging — a trusted-proxy set that does not match the deployment.
// In the second case every client collapses into the load balancer's single
// bucket and the whole tenant base starts seeing 429s, which is a confusing
// outage unless something named the cause. This is that line.
func (m *Middleware) clientIP(r *http.Request) string {
	if r.Header.Get("X-Forwarded-For") != "" && !m.cfg.TrustedProxies.TrustsPeer(r) {
		m.warnedUntrustedForward.Do(func() {
			m.logger.Warn("X-Forwarded-For arrived from a peer outside the trusted-proxy set and was ignored; "+
				"if there is a proxy in front of this process, add its address range to TRUSTED_PROXIES",
				"peer", r.RemoteAddr, "trusted", m.cfg.TrustedProxies.String())
		})
	}
	return httpx.ClientIPFrom(r, m.cfg.TrustedProxies)
}

// Guards returns the per-route access checks domain modules ask for by name.
func (m *Middleware) Guards() httpx.Guards {
	return httpx.Guards{
		RequireAuth:         m.RequireAuth,
		RequireComplexOwner: m.RequireComplexOwner,
		RequireSuperAdmin:   m.RequireRole("superadmin"),
	}
}

// Wrap puts the router behind the full chain, outermost first.
//
// RequestID is outermost and RecoverPanic sits directly inside it, which is
// the opposite of the obvious order and is deliberate. RecoverPanic reports
// through the Responder, which stamps every log line with the request id from
// the context — so with recovery on the outside, the one log line that matters
// most carried request_id="" while the client was already holding the id from
// the X-Request-ID response header. The two could not be joined up, which is
// precisely when you need them joined up.
//
// The cost of the swap is that a panic inside RequestID itself is no longer
// caught. RequestID mints a UUID and sets one header; the only way it panics
// is the kernel refusing to produce randomness, which no 500 page survives
// anyway.
//
// Authenticate is innermost of the checks, so a request rejected by rate
// limiting or CSRF never costs a database read.
func (m *Middleware) Wrap(router http.Handler, cors func(http.Handler) http.Handler) http.Handler {
	return m.RequestID(
		m.LogRequests(
			m.RecoverPanic(
				m.SecurityHeaders(
					cors(
						m.RateLimit(
							m.CSRFProtect(
								m.Authenticate(router))))))))
}
