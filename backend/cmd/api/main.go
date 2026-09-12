package main

import (
	"context"
	"expvar"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stodulski/vibe-server/internal/admin"
	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/auth"
	"github.com/stodulski/vibe-server/internal/bookings"
	"github.com/stodulski/vibe-server/internal/clients"
	"github.com/stodulski/vibe-server/internal/complexes"
	"github.com/stodulski/vibe-server/internal/courts"
	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/health"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/leads"
	"github.com/stodulski/vibe-server/internal/mailer"
	"github.com/stodulski/vibe-server/internal/middleware"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/notifier"
	"github.com/stodulski/vibe-server/internal/openapi"
	"github.com/stodulski/vibe-server/internal/payments"
	"github.com/stodulski/vibe-server/internal/places"
	"github.com/stodulski/vibe-server/internal/publicsite"
	"github.com/stodulski/vibe-server/internal/realtime"
	"github.com/stodulski/vibe-server/internal/reporting"
	"github.com/stodulski/vibe-server/internal/scheduler"
	"github.com/stodulski/vibe-server/internal/storage"
	"github.com/stodulski/vibe-server/internal/stores"
	"github.com/stodulski/vibe-server/internal/whatsapp"
)

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
		// statementTimeout is the server-side backstop. A Go context cancels a
		// query by sending PostgreSQL a cancel request over a second
		// connection; when the pool is exhausted or the network is the thing
		// that is broken, that request is exactly what cannot get through, and
		// the query keeps a pool connection for as long as it likes. With 25
		// connections, a handful of those is the whole instance.
		statementTimeout time.Duration
		// autoMigrate runs the embedded migration chain before the server
		// starts listening. Off by default: a process that changes the
		// schema as a side effect of booting is a surprise unless somebody
		// asked for it. Railway asks for it the other way, through
		// -migrate-only as a pre-deploy command, which is the same chain
		// run once instead of once per replica.
		autoMigrate bool
		// migratorDSN is the connection migrations run as. Empty means "the
		// same one the server uses". See migratorDSN() in migrate.go for why
		// the two are allowed to differ.
		migratorDSN string
	}
	jwt struct {
		secret string
	}
	mp struct {
		accessToken    string
		webhookSecret  string
		appID          string
		clientSecret   string
		credentialKeys string
	}
	whatsapp struct {
		token       string
		phoneID     string
		verifyToken string
		appSecret   string
	}
	brevo struct {
		apiKey string
		sender string
	}
	turnstile struct {
		// secretKey enables Cloudflare Turnstile verification on register,
		// login and forgot-password when non-empty. Optional: a self-hoster
		// is not forced to use Turnstile.
		secretKey string
	}
	leads struct {
		// abandonedWebhookURL is a Google Apps Script Web App /exec URL
		// (same spreadsheet the landing page's mailing-list signup writes
		// to). Blank disables forwarding — the endpoint still accepts and
		// validates requests, it just drops them.
		abandonedWebhookURL   string
		abandonedWebhookToken string
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
	}
	r2 struct {
		accountID  string
		accessKey  string
		secretKey  string
		bucketName string
		publicURL  string
	}
	frontendURL string
	// passwordHashCost is the bcrypt cost the auth module hashes at. Zero
	// means authstore.DefaultHashCost, the production value; the test harness
	// lowers it so a suite that registers hundreds of users does not spend a
	// quarter of a second on each one. It carries no flag: nothing about a
	// real deployment should ever set it.
	passwordHashCost int
	backendURL       string
	cookieDomain     string
	// trustedProxies is the "is there a proxy in front of us at all" answer the
	// audit-log call sites still take. trustedProxySet is the real setting: the
	// address ranges allowed to rewrite the client address. The bool is derived
	// from the set so the two cannot drift.
	trustedProxies  bool
	trustedProxySet httpx.TrustedProxies
	limiter         struct {
		enabled bool
		rps     float64
		burst   int
	}
	// requestLogSample thins the request log: 0 or 1 logs every request, N
	// logs one successful request in N. Failures and slow requests are never
	// sampled away, and the metrics behind the detailed health endpoint are
	// never sampled at all.
	requestLogSample int
	booking          struct {
		gracePeriod        time.Duration
		paymentExpiry      time.Duration
		cancellationWindow time.Duration
		slotLockTTL        time.Duration
		// linkTokenBuffer is added to a booking's end time to compute a
		// booking link token's expires_at (specs/booking-link-credential).
		linkTokenBuffer time.Duration
	}
	limits struct {
		maxComplexes int
	}
	redis struct {
		url string
	}
	sentry struct {
		dsn string
	}
	google struct {
		placesAPIKey string
		// oauthClientID enables Sign in with Google when non-empty: every
		// endpoint stays registered either way (POST /api/v1/auth/google and
		// .../google/complete answer 503 while it is empty), so the route
		// table, the OpenAPI document and the CSRF/rate-limit/tenant
		// exemption tables never depend on it.
		oauthClientID string
	}
	pprof bool
}

type application struct {
	config  config
	logger  *slog.Logger
	respond *httpx.Responder
	auditor *audit.Recorder
	// auditTrail serves a tenant its own trail; auditor writes to it.
	auditTrail *audit.Handler
	places     *places.Handler
	clients    *clients.Handler
	realtime   *realtime.Handler
	publicsite *publicsite.Handler
	leads      *leads.Handler
	reporting  *reporting.Handler
	admin      *admin.Handler
	health     *health.Handler
	openapi    *openapi.Handler
	// queues reports the durable work queues' backlog. It is a field so the
	// cron heartbeat and the health endpoint read the same source, and so a
	// test can supply one without a database.
	queues     health.QueueReporter
	notify     *notifications.Service
	courts     *courts.Handler
	complexes  *complexes.Handler
	// complexesService is held separately from the handler because the
	// scheduler calls it directly: the MercadoPago OAuth refresh sweep is the
	// venue domain's own credential lifecycle, not an HTTP route.
	complexesService *complexes.Service
	auth       *auth.Handler
	payments   *payments.Handler
	bookings   *bookings.Handler
	scheduler  *scheduler.Scheduler
	middleware *middleware.Middleware
	db         *pgxpool.Pool
	rdb        *redis.Client
	models     stores.Stores
	mp         *mp.MPClient
	// mpOAuth is the same provider on its own circuit breaker, used only by the
	// bulk token-refresh cron. See newApplication for why it is separate.
	mpOAuth  *mp.MPClient
	wa       *whatsapp.WAClient
	mailer   *mailer.Mailer
	notifier *notifier.Notifier
	// queue is what app.notify publishes to: notifier (wrapped by taskQueue)
	// when Redis is configured, a recording memoryQueue otherwise. It is a
	// field, distinct from notifier, so a test can read back what was
	// enqueued regardless of which implementation newApplication chose.
	queue           notifications.Queue
	blacklist       *auth.TokenBlacklist
	tokens          *auth.TokenService
	events          *realtime.Hub
	storage         storage.ObjectStorage
	whatsappEnabled bool
	wg              sync.WaitGroup
	shutdown        chan struct{}
}

func sentryRelease() string {
	if v := os.Getenv("SENTRY_RELEASE"); v != "" {
		return v
	}
	return "vibe@1.0.0"
}

// dependency construction, route registration); splitting would fragment a single linear
// initialization sequence into arbitrarily-named helpers without clarifying it.
//
//nolint:funlen // flat sequential startup/wiring code (flag parsing, env-var overrides,
func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 8080, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 10, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")
	flag.DurationVar(&cfg.db.statementTimeout, "db-statement-timeout", 15*time.Second,
		"PostgreSQL statement_timeout: the server-side backstop for a query no context managed to cancel")
	flag.BoolVar(&cfg.db.autoMigrate, "db-auto-migrate", false,
		"Apply the embedded migration chain before serving (DB_AUTO_MIGRATE)")
	flag.StringVar(&cfg.db.migratorDSN, "db-migrator-dsn", "",
		"PostgreSQL DSN migrations run as; falls back to db-dsn/DATABASE_URL (DB_MIGRATOR_URL)")
	var migrateOnly bool
	flag.BoolVar(&migrateOnly, "migrate-only", false,
		"Apply the embedded migration chain, print the status, and exit without starting the server")

	flag.StringVar(&cfg.jwt.secret, "jwt-secret", "", "JWT secret")

	flag.StringVar(&cfg.mp.accessToken, "mp-access-token", "", "MercadoPago access token")
	flag.StringVar(&cfg.mp.webhookSecret, "mp-webhook-secret", "", "MercadoPago webhook secret")
	flag.StringVar(&cfg.mp.appID, "mp-app-id", "", "MercadoPago application ID (for OAuth)")
	flag.StringVar(&cfg.mp.clientSecret, "mp-client-secret", "", "MercadoPago client secret (for OAuth)")
	flag.StringVar(&cfg.mp.credentialKeys, "mp-credential-keys", "",
		"MercadoPago credential encryption keyring: kid:base64key[,kid:base64key...] (first entry writes, every entry opens)")

	flag.StringVar(&cfg.whatsapp.token, "whatsapp-token", "", "WhatsApp API token")
	flag.StringVar(&cfg.whatsapp.phoneID, "whatsapp-phone-id", "", "WhatsApp phone number ID")
	flag.StringVar(&cfg.whatsapp.verifyToken, "whatsapp-verify-token", "", "WhatsApp verify token")
	flag.StringVar(&cfg.whatsapp.appSecret, "whatsapp-app-secret", "", "WhatsApp app secret for webhook verification")

	flag.StringVar(&cfg.brevo.apiKey, "brevo-api-key", "", "Brevo API key")
	flag.StringVar(&cfg.brevo.sender, "brevo-sender", "Vibe <no-reply@vibe.com.ar>", "Email sender")
	flag.StringVar(&cfg.smtp.host, "smtp-host", "", "SMTP host (fallback if no Brevo API key)")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 587, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "", "SMTP password")

	flag.StringVar(&cfg.turnstile.secretKey, "turnstile-secret-key", "",
		"Cloudflare Turnstile secret key; enables verification on register, login and forgot-password (TURNSTILE_SECRET_KEY)")

	flag.StringVar(&cfg.leads.abandonedWebhookURL, "leads-abandoned-webhook-url", "",
		"Google Apps Script webhook URL for abandoned-registration email capture")
	flag.StringVar(&cfg.leads.abandonedWebhookToken, "leads-abandoned-webhook-token", "",
		"Shared token the abandoned-registration webhook expects")

	flag.StringVar(&cfg.r2.accountID, "r2-account-id", "", "Cloudflare R2 account ID")
	flag.StringVar(&cfg.r2.accessKey, "r2-access-key", "", "Cloudflare R2 access key")
	flag.StringVar(&cfg.r2.secretKey, "r2-secret-key", "", "Cloudflare R2 secret key")
	flag.StringVar(&cfg.r2.bucketName, "r2-bucket-name", "vibe", "Cloudflare R2 bucket name")
	flag.StringVar(&cfg.r2.publicURL, "r2-public-url", "", "R2 public base URL for serving images")

	flag.StringVar(&cfg.frontendURL, "frontend-url", "http://localhost:5173", "Frontend URL")
	flag.StringVar(&cfg.backendURL, "backend-url", "", "Backend public URL (for webhooks)")
	flag.StringVar(&cfg.cookieDomain, "cookie-domain", "", "Cookie domain (e.g. .skymait.com)")

	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 10, "Rate limiter requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 20, "Rate limiter maximum burst")

	flag.DurationVar(&cfg.booking.gracePeriod, "booking-grace-period", 15*time.Minute, "Grace period for refund after booking creation")
	flag.DurationVar(&cfg.booking.paymentExpiry, "booking-payment-expiry", 15*time.Minute, "Time before unpaid booking is auto-cancelled")
	flag.DurationVar(&cfg.booking.cancellationWindow, "booking-cancellation-window", 24*time.Hour, "Default cancellation window before game start")
	flag.DurationVar(&cfg.booking.slotLockTTL, "booking-slot-lock-ttl", 15*time.Minute, "TTL for slot locks during payment flow")
	flag.DurationVar(&cfg.booking.linkTokenBuffer, "booking-link-token-buffer", 24*time.Hour,
		"How long past a booking's end its access token stays valid (specs/booking-link-credential)")
	flag.IntVar(&cfg.limits.maxComplexes, "limits-max-complexes", 4, "Maximum complexes per user account")

	flag.StringVar(&cfg.google.placesAPIKey, "google-places-api-key", "", "Google Places API key")
	flag.StringVar(&cfg.google.oauthClientID, "google-oauth-client-id", "",
		"Google OAuth client id; enables Sign in with Google (GOOGLE_OAUTH_CLIENT_ID)")
	flag.BoolVar(&cfg.pprof, "pprof", false, "Enable pprof profiling endpoints")
	flag.IntVar(&cfg.requestLogSample, "request-log-sample", 1,
		"Log one successful request in N (1 logs every request; failures and slow requests are never sampled away)")
	var trustedProxySpec string
	flag.StringVar(&trustedProxySpec, "trusted-proxies", "",
		`Peers allowed to set X-Forwarded-For: "false" (default), "true" for the private ranges, `+
			`or a comma-separated list of CIDR prefixes`)

	flag.Parse()

	// Override flags with environment variables when set.
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		cfg.db.dsn = dsn
	}
	if dsn := os.Getenv("DB_MIGRATOR_URL"); dsn != "" {
		cfg.db.migratorDSN = dsn
	}
	// Captured rather than acted on: the logger does not exist yet, and a
	// DB_AUTO_MIGRATE nobody could parse must not quietly resolve to false —
	// that is the whole feature switched off with nothing said. Resolved
	// beside trusted-proxies below, once there is somewhere to say it.
	var autoMigrateErr error
	if v := os.Getenv("DB_AUTO_MIGRATE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			autoMigrateErr = fmt.Errorf("DB_AUTO_MIGRATE %q is not a boolean (true/false/1/0): %w", v, err)
		}
		cfg.db.autoMigrate = b
	}
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		cfg.jwt.secret = secret
	}
	if token := os.Getenv("MP_ACCESS_TOKEN"); token != "" {
		cfg.mp.accessToken = token
	}
	if secret := os.Getenv("MP_WEBHOOK_SECRET"); secret != "" {
		cfg.mp.webhookSecret = secret
	}
	if appID := os.Getenv("MP_APP_ID"); appID != "" {
		cfg.mp.appID = appID
	}
	if clientSecret := os.Getenv("MP_CLIENT_SECRET"); clientSecret != "" {
		cfg.mp.clientSecret = clientSecret
	}
	if keys := os.Getenv("MP_CREDENTIAL_KEYS"); keys != "" {
		cfg.mp.credentialKeys = keys
	}
	if token := os.Getenv("WHATSAPP_TOKEN"); token != "" {
		cfg.whatsapp.token = token
	}
	if id := os.Getenv("WHATSAPP_PHONE_NUMBER_ID"); id != "" {
		cfg.whatsapp.phoneID = id
	}
	if token := os.Getenv("WHATSAPP_VERIFY_TOKEN"); token != "" {
		cfg.whatsapp.verifyToken = token
	}
	if secret := os.Getenv("WHATSAPP_APP_SECRET"); secret != "" {
		cfg.whatsapp.appSecret = secret
	}
	if key := os.Getenv("BREVO_API_KEY"); key != "" {
		cfg.brevo.apiKey = key
	}
	if sender := os.Getenv("BREVO_SENDER"); sender != "" {
		cfg.brevo.sender = sender
	}
	if secret := os.Getenv("TURNSTILE_SECRET_KEY"); secret != "" {
		cfg.turnstile.secretKey = secret
	}
	if url := os.Getenv("LEADS_ABANDONED_WEBHOOK_URL"); url != "" {
		cfg.leads.abandonedWebhookURL = url
	}
	if token := os.Getenv("LEADS_ABANDONED_WEBHOOK_TOKEN"); token != "" {
		cfg.leads.abandonedWebhookToken = token
	}
	if host := os.Getenv("SMTP_HOST"); host != "" {
		cfg.smtp.host = host
	}
	if username := os.Getenv("SMTP_USERNAME"); username != "" {
		cfg.smtp.username = username
	}
	if password := os.Getenv("SMTP_PASSWORD"); password != "" {
		cfg.smtp.password = password
	}
	if id := os.Getenv("R2_ACCOUNT_ID"); id != "" {
		cfg.r2.accountID = id
	}
	if key := os.Getenv("R2_ACCESS_KEY"); key != "" {
		cfg.r2.accessKey = key
	}
	if key := os.Getenv("R2_SECRET_KEY"); key != "" {
		cfg.r2.secretKey = key
	}
	if name := os.Getenv("R2_BUCKET_NAME"); name != "" {
		cfg.r2.bucketName = name
	}
	if url := os.Getenv("R2_PUBLIC_URL"); url != "" {
		cfg.r2.publicURL = url
	}
	if env := os.Getenv("ENV"); env != "" {
		cfg.env = env
	}
	if url := os.Getenv("FRONTEND_URL"); url != "" {
		cfg.frontendURL = url
	}
	if url := os.Getenv("BACKEND_URL"); url != "" {
		cfg.backendURL = url
	}
	if domain := os.Getenv("COOKIE_DOMAIN"); domain != "" {
		cfg.cookieDomain = domain
	}
	if url := os.Getenv("REDIS_URL"); url != "" {
		cfg.redis.url = url
	}
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		cfg.sentry.dsn = dsn
	}
	if os.Getenv("PPROF_ENABLED") == "true" {
		cfg.pprof = true
	}
	if v := os.Getenv("REQUEST_LOG_SAMPLE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.requestLogSample = n
		}
	}
	if spec := os.Getenv("TRUSTED_PROXIES"); spec != "" {
		trustedProxySpec = spec
	}
	if v := os.Getenv("DB_STATEMENT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.db.statementTimeout = d
		}
	}
	if v := os.Getenv("BOOKING_GRACE_PERIOD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.booking.gracePeriod = d
		}
	}
	if v := os.Getenv("BOOKING_PAYMENT_EXPIRY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.booking.paymentExpiry = d
		}
	}
	if v := os.Getenv("BOOKING_CANCELLATION_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.booking.cancellationWindow = d
		}
	}
	if v := os.Getenv("BOOKING_SLOT_LOCK_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.booking.slotLockTTL = d
		}
	}
	if v := os.Getenv("BOOKING_LINK_TOKEN_BUFFER"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.booking.linkTokenBuffer = d
		}
	}
	if v := os.Getenv("LIMITS_MAX_COMPLEXES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.limits.maxComplexes = n
		}
	}
	if key := os.Getenv("GOOGLE_MAPS_API"); key != "" {
		cfg.google.placesAPIKey = key
	}
	if id := os.Getenv("GOOGLE_OAUTH_CLIENT_ID"); id != "" {
		cfg.google.oauthClientID = id
	}

	var logger *slog.Logger
	if cfg.env == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	if cfg.sentry.dsn != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.sentry.dsn,
			Environment:      cfg.env,
			Release:          sentryRelease(),
			TracesSampleRate: 0.1,
			EnableTracing:    true,
			// Everything below leaves the building. See scrubEvent.
			BeforeSend:       scrubEvent,
			BeforeBreadcrumb: scrubBreadcrumb,
		})
		if err != nil {
			logger.Error("sentry initialization failed", "error", err)
		} else {
			logger.Info("sentry initialized")
			defer sentry.Flush(2 * time.Second)
		}
	}

	if autoMigrateErr != nil {
		logger.Error("invalid DB_AUTO_MIGRATE setting", "error", autoMigrateErr)
		sentry.Flush(2 * time.Second)
		//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is replicated on the line above.
		os.Exit(1)
	}

	// -migrate-only is what a Railway pre-deploy command runs: apply the
	// chain, say what the database now has, and exit. It is answered here,
	// before JWT_SECRET, the MercadoPago keyring and validateBootConfig,
	// because none of those have anything to do with a schema — a pre-deploy
	// step that fails on a missing webhook secret is a step that lies about
	// what is wrong.
	if migrateOnly {
		if err := runMigrations(cfg, logger); err != nil {
			sentry.Flush(2 * time.Second)
			//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is replicated on the line above.
			os.Exit(1)
		}
		if err := logMigrationStatus(cfg, logger); err != nil {
			logger.Error("failed to read migration status", "error", err)
			sentry.Flush(2 * time.Second)
			os.Exit(1)
		}
		sentry.Flush(2 * time.Second)
		os.Exit(0)
	}

	// Resolved after the logger exists so a bad set can be reported, and before
	// anything is wired, so a bad set stops the boot rather than silently
	// trusting nobody — which on a proxied deployment collapses every client
	// into the balancer's single rate-limit bucket.
	trustedProxySet, err := httpx.ParseTrustedProxies(trustedProxySpec)
	if err != nil {
		logger.Error("invalid trusted-proxies setting", "error", err)
		sentry.Flush(2 * time.Second)
		//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is replicated above.
		os.Exit(1)
	}
	cfg.trustedProxySet = trustedProxySet
	cfg.trustedProxies = trustedProxySet.Any()
	logger.Info("trusted proxies", "set", trustedProxySet.String())
	// The audit-log call sites in internal/bookings, internal/complexes,
	// internal/courts and internal/admin still take a bool and resolve it to
	// httpx.DefaultTrustedProxies. A custom set therefore attributes audit
	// entries by a different rule than the rate limiter uses, which is worth
	// saying out loud until those call sites take the set too.
	if trustedProxySet.Any() && trustedProxySet.String() != httpx.DefaultTrustedProxies().String() {
		logger.Warn("a custom trusted-proxy set is in use, but the audit-log call sites still resolve " +
			"the default private ranges; move them to httpx.ClientIPFrom before relying on either")
	}

	if cfg.jwt.secret == "" {
		logger.Error("jwt-secret flag or JWT_SECRET env var must be set")
		// os.Exit bypasses the deferred sentry.Flush above; flush explicitly
		// so a captured init/config error is not lost on process exit. Safe
		// to call even when sentry was never initialized (no-op then).
		sentry.Flush(2 * time.Second)
		//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is already replicated
		// explicitly on the line above, so this os.Exit does not actually skip it.
		os.Exit(1)
	}
	if len(cfg.jwt.secret) < 32 {
		if cfg.env == "production" {
			logger.Error("JWT_SECRET must be at least 32 random bytes in production")
			sentry.Flush(2 * time.Second)
			os.Exit(1)
		}
		logger.Warn("JWT_SECRET is too short, use at least 32 random bytes in production")
	}
	if strings.Contains(strings.ToLower(cfg.jwt.secret), "cambiar") ||
		strings.Contains(strings.ToLower(cfg.jwt.secret), "change") ||
		strings.Contains(strings.ToLower(cfg.jwt.secret), "test") {
		if cfg.env == "production" {
			logger.Error("JWT_SECRET appears to be a placeholder, refusing to start in production")
			os.Exit(1)
		}
		logger.Warn("JWT_SECRET appears to be a placeholder, rotate it before going to production")
	}

	// Required unconditionally, matching JWT_SECRET's unconditional
	// requirement above — design.md's open question resolved against
	// gating this on cfg.env == "production". A nil keyring is not a
	// smaller hazard in development: it silently makes every MercadoPago
	// credential read/write fail with ErrNilKeyring instead of surfacing a
	// misconfiguration at boot, and design.md is explicit that "silently
	// not encrypted" — not "silently refuses" — is the failure this change
	// exists to remove. Refusing to boot is the loud alternative to both.
	mpKeyring, err := crypto.ParseKeyring(cfg.mp.credentialKeys)
	if err != nil {
		logger.Error("mp-credential-keys flag or MP_CREDENTIAL_KEYS env var is invalid", "error", err)
		sentry.Flush(2 * time.Second)
		os.Exit(1)
	}

	if err := validateBootConfig(cfg, logger); err != nil {
		logger.Error("boot configuration check failed", "error", err)
		sentry.Flush(2 * time.Second)
		os.Exit(1)
	}

	// After the boot checks and before the pool: migrating a database and then
	// refusing to start over a missing secret leaves a schema nobody asked for
	// behind a deploy that never came up. Before openDB because the server's
	// pool may be opened as a role that cannot see a table the schema has not created
	// yet, and because a failure here must stop the boot rather than let the
	// server serve a schema it does not match.
	if cfg.db.autoMigrate {
		if err := runMigrations(cfg, logger); err != nil {
			sentry.Flush(2 * time.Second)
			//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is replicated on the line above.
			os.Exit(1)
		}
	}

	db, err := openDB(cfg)
	if err != nil {
		logger.Error("failed to open database connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("database connection pool established")

	expvar.NewString("version").Set("1.0.0")
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

	// Redis (optional — when REDIS_URL is set, enables distributed rate limiting,
	// shared blacklist, persistent notification queue, and user cache).
	var rdb *redis.Client
	if cfg.redis.url != "" {
		opt, err := redis.ParseURL(cfg.redis.url)
		if err != nil {
			logger.Error("failed to parse REDIS_URL", "error", err)
			os.Exit(1)
		}

		// Explicit pool tuning: 4 BRPop workers hold connections permanently,
		// plus concurrent rate-limit, blacklist, SSE subscriber, and cache operations.
		opt.PoolSize = 10 * runtime.GOMAXPROCS(0)
		opt.MinIdleConns = 5
		opt.PoolFIFO = true // prefer recently used connections to reduce stale-conn errors
		opt.ConnMaxIdleTime = 5 * time.Minute
		opt.DialTimeout = 5 * time.Second
		opt.ReadTimeout = 3 * time.Second
		opt.WriteTimeout = 3 * time.Second
		opt.MaxRetries = 3
		// Aggressive TCP keepalive: Railway's proxy drops idle connections.
		// 30s probes keep long-lived connections (BRPop workers, SSE subscriber) alive.
		opt.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{
				Timeout:   opt.DialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, network, addr)
		}

		rdb = redis.NewClient(opt)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			logger.Warn("failed to connect to Redis, using in-memory fallback", "error", err)
			_ = rdb.Close()
			rdb = nil
		} else {
			logger.Info("redis connected")
		}
	} else {
		logger.Info("redis not configured (REDIS_URL not set), using in-memory fallback")
	}

	// Rate limiting, the token blacklist and the SSE hub all have in-memory
	// fallbacks, so Redis used to be optional for the whole process. The
	// notification queue has no fallback: without Redis every task enqueued is
	// discarded. An instance in that state answers requests normally and passes
	// its health check while delivering zero email — and because sign-in
	// requires a verified address, no account it accepts can ever be used.
	//
	// That is not a degraded deployment, it is a broken one that looks healthy,
	// so it refuses to start instead. The alternative — booting and counting the
	// drops — makes the failure visible to whoever reads the metric, which is
	// nobody on the deploy that introduced it.
	if rdb == nil {
		logger.Error("redis is required: the notification queue is Redis-backed and has no fallback, " +
			"so this instance would accept registrations and bookings while dropping every " +
			"verification email, password reset and confirmation. Set REDIS_URL to a reachable instance.")
		sentry.Flush(2 * time.Second)
		os.Exit(1)
	}

	var objectStorage storage.ObjectStorage
	if cfg.r2.accountID != "" && cfg.r2.accessKey != "" && cfg.r2.secretKey != "" {
		r2Client, err := storage.NewR2Client(cfg.r2.accountID, cfg.r2.accessKey, cfg.r2.secretKey, cfg.r2.bucketName, cfg.r2.publicURL)
		if err != nil {
			logger.Error("failed to initialize R2 storage", "error", err)
			os.Exit(1)
		}
		objectStorage = r2Client
		logger.Info("R2 storage initialized", "bucket", cfg.r2.bucketName)
	}

	app, err := newApplication(cfg, deps{
		logger: logger,
		models: stores.New(db, stores.Config{
			PaymentExpiry:   cfg.booking.paymentExpiry,
			Logger:          logger,
			Keys:            mpKeyring,
			LinkTokenBuffer: cfg.booking.linkTokenBuffer,
		}),
		db:      db,
		rdb:     rdb,
		storage: objectStorage,
	})
	if err != nil {
		logger.Error("failed to assemble application", "error", err)
		sentry.Flush(2 * time.Second)
		os.Exit(1)
	}

	// Background work starts here, in main(), never inside newApplication: the
	// constructor must be safe to call more than once in a process, which a
	// goroutine it launched itself would not be.
	app.events.Start()
	app.notify.RegisterWorkers()
	app.notifier.Start()

	logger.Info("email configured", "mode", app.mailer.Mode(), "sender", cfg.brevo.sender)

	if app.whatsappEnabled {
		logger.Info("whatsapp notifications enabled")
	} else {
		logger.Info("whatsapp notifications disabled (no WHATSAPP_TOKEN/WHATSAPP_PHONE_NUMBER_ID)")
	}

	if cfg.turnstile.secretKey != "" {
		logger.Info("turnstile verification enabled")
	} else {
		logger.Info("turnstile verification disabled (no TURNSTILE_SECRET_KEY)")
		// Not a boot failure — an owner may deliberately self-host without
		// Turnstile — but a production deployment running unprotected
		// register/login/forgot-password endpoints is worth a loud line, not
		// just the info one above that a quiet log rotation could bury.
		if cfg.env == "production" {
			logger.Warn("running in production without TURNSTILE_SECRET_KEY: " +
				"register, login and forgot-password accept no bot verification")
		}
	}

	if cfg.google.oauthClientID != "" {
		logger.Info("google sign-in enabled")
	} else {
		logger.Info("google sign-in disabled (no GOOGLE_OAUTH_CLIENT_ID)")
	}

	app.startCronJobs()

	err = app.serve()
	if err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func openDB(cfg config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.db.dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database DSN: %w", err)
	}

	//nolint:gosec // G115: db-max-open-conns is an operator-supplied CLI flag/env var (default 25), never derived
	// from request input; realistic values are far below int32 range.
	poolConfig.MaxConns = int32(cfg.db.maxOpenConns)
	//nolint:gosec // G115: db-max-idle-conns is an operator-supplied CLI flag/env var (default 10), never derived
	// from request input; realistic values are far below int32 range.
	poolConfig.MinConns = int32(cfg.db.maxIdleConns)
	poolConfig.MaxConnLifetime = 5 * time.Minute
	poolConfig.MaxConnIdleTime = cfg.db.maxIdleTime
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	// Stamp the tenant scope on every connection as it leaves the pool, from
	// the context of whoever is borrowing it. This is what gives the tenant
	// policies something to compare against on the queries that are not
	// inside a transaction — see internal/data/tenant.go for the whole
	// mechanism and for why a transaction repeats it with SET LOCAL.
	//
	// PrepareConn rather than the deprecated BeforeAcquire: it can tell a dead
	// connection (destroyed, query retried on another) from a statement that
	// failed on a live one (kept, query fails). See data.StampTenantScope.
	poolConfig.PrepareConn = data.StampTenantScope

	// Set on every connection as it is established, so it applies to every
	// query on it. It is deliberately looser than the store's own 3s and 5s
	// budgets: those are the normal path, and this only has to catch the case
	// where none of them managed to cancel anything.
	if cfg.db.statementTimeout > 0 {
		if poolConfig.ConnConfig.RuntimeParams == nil {
			poolConfig.ConnConfig.RuntimeParams = map[string]string{}
		}
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] =
			strconv.FormatInt(cfg.db.statementTimeout.Milliseconds(), 10)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}
