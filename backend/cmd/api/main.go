package main

import (
	"context"
	"errors"
	"expvar"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
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
	"github.com/stodulski/vibe-server/internal/platform/config"
	platformdb "github.com/stodulski/vibe-server/internal/platform/db"
	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
	"github.com/stodulski/vibe-server/internal/publicsite"
	"github.com/stodulski/vibe-server/internal/realtime"
	"github.com/stodulski/vibe-server/internal/reporting"
	"github.com/stodulski/vibe-server/internal/scheduler"
	"github.com/stodulski/vibe-server/internal/storage"
	"github.com/stodulski/vibe-server/internal/stores"
	"github.com/stodulski/vibe-server/internal/whatsapp"
)

type application struct {
	config  config.Config
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
	queues    health.QueueReporter
	notify    *notifications.Service
	courts    *courts.Handler
	complexes *complexes.Handler
	// complexesService is held separately from the handler because the
	// scheduler calls it directly: the MercadoPago OAuth refresh sweep is the
	// venue domain's own credential lifecycle, not an HTTP route.
	complexesService *complexes.Service
	auth             *auth.Handler
	payments         *payments.Handler
	// paymentsService is held separately from the handler because the scheduler
	// calls it directly: retrying refunds, sweeping the webhook inbox and
	// reconciling refund intents are the money domain's own background work,
	// not HTTP routes.
	paymentsService *payments.Service
	bookings        *bookings.Handler
	// bookingsService is held separately from the handler because the scheduler
	// calls it directly: the reminder, the expiry sweep, the completion sweep
	// and the link-token sweep are the booking domain's own rules, not HTTP
	// routes.
	bookingsService *bookings.Service
	scheduler       *scheduler.Scheduler
	middleware      *middleware.Middleware
	db              *platformdb.Pool
	rdb             *platformredis.Client
	models          stores.Stores
	mp              *mp.MPClient
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

// sentryRelease is what Sentry groups this build's errors under: SENTRY_RELEASE
// when a deployment sets one, and otherwise the build's own version — which the
// Dockerfile stamps in with -X main.version, so an unset variable still names
// the commit rather than a literal nobody has changed since it was written.
func sentryRelease(cfg config.Config) string {
	if cfg.Sentry.Release != "" {
		return cfg.Sentry.Release
	}
	return "vibe@" + version
}

// baseLogger is the logger every line in the process descends from.
//
// The three attributes on it are the ones a log aggregator groups by and that
// no individual call site knows: which service wrote the line, which
// deployment it was, and which build. Without them a line in a shared log
// stream says what happened and nothing about where — and "which build" is
// the first question asked of an error that started this afternoon.
func baseLogger(cfg config.Config, w io.Writer) *slog.Logger {
	var handler slog.Handler
	if cfg.Env == "production" {
		handler = slog.NewJSONHandler(w, nil)
	} else {
		handler = slog.NewTextHandler(w, nil)
	}
	return slog.New(handler).With(
		"service", serviceName,
		"env", cfg.Env,
		"version", version,
	)
}

// serviceName is what this process calls itself in its own logs.
const serviceName = "vibe-api"

// main is config → dependencies → serve.
//
// The configuration itself lives in internal/platform/config, the pool in
// internal/platform/db and the Redis client in internal/platform/redis, so
// what is left here is the order the boot happens in: what must be true before
// the next thing is built, and what is refused outright.
//
// initialization sequence into arbitrarily-named helpers without clarifying it.
//
//nolint:funlen // flat sequential startup/wiring code; splitting would fragment a single linear
func main() {
	cfg, err := config.Load(os.Args[1:], config.OSLookup)
	if errors.Is(err, config.ErrHelp) {
		// -h. Asking what the flags are is not a misconfiguration.
		config.Usage(os.Stdout)
		os.Exit(0)
	}
	if err != nil {
		// Written before there is a configured logger, because the thing that
		// failed is the configuration a logger would be built from.
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger := baseLogger(cfg, os.Stdout)

	if cfg.Sentry.DSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.Sentry.DSN,
			Environment:      cfg.Env,
			Release:          sentryRelease(cfg),
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

	// -migrate-only is what a Railway pre-deploy command runs: apply the
	// chain, say what the database now has, and exit. It is answered here,
	// before JWT_SECRET, the MercadoPago keyring and validateBootConfig,
	// because none of those have anything to do with a schema — a pre-deploy
	// step that fails on a missing webhook secret is a step that lies about
	// what is wrong.
	if cfg.MigrateOnly {
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
	//
	// It is parsed here rather than in internal/platform/config because the
	// parsed type belongs to internal/httpx, and a platform package holds no
	// domain types.
	trustedProxySet, err := httpx.ParseTrustedProxies(cfg.TrustedProxiesSpec)
	if err != nil {
		logger.Error("invalid trusted-proxies setting", "error", err)
		sentry.Flush(2 * time.Second)
		//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is replicated above.
		os.Exit(1)
	}
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

	if names := cfg.Features.Names(); len(names) > 0 {
		logger.Info("feature flags enabled", "flags", strings.Join(names, ","))
	}

	if cfg.JWT.Secret == "" {
		logger.Error("jwt-secret flag or JWT_SECRET env var must be set")
		// os.Exit bypasses the deferred sentry.Flush above; flush explicitly
		// so a captured init/config error is not lost on process exit. Safe
		// to call even when sentry was never initialized (no-op then).
		sentry.Flush(2 * time.Second)
		//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is already replicated
		// explicitly on the line above, so this os.Exit does not actually skip it.
		os.Exit(1)
	}
	if len(cfg.JWT.Secret) < 32 {
		if cfg.Env == "production" {
			logger.Error("JWT_SECRET must be at least 32 random bytes in production")
			sentry.Flush(2 * time.Second)
			os.Exit(1)
		}
		logger.Warn("JWT_SECRET is too short, use at least 32 random bytes in production")
	}
	if strings.Contains(strings.ToLower(cfg.JWT.Secret), "cambiar") ||
		strings.Contains(strings.ToLower(cfg.JWT.Secret), "change") ||
		strings.Contains(strings.ToLower(cfg.JWT.Secret), "test") {
		if cfg.Env == "production" {
			logger.Error("JWT_SECRET appears to be a placeholder, refusing to start in production")
			os.Exit(1)
		}
		logger.Warn("JWT_SECRET appears to be a placeholder, rotate it before going to production")
	}

	// Required unconditionally, matching JWT_SECRET's unconditional
	// requirement above — design.md's open question resolved against
	// gating this on cfg.Env == "production". A nil keyring is not a
	// smaller hazard in development: it silently makes every MercadoPago
	// credential read/write fail with ErrNilKeyring instead of surfacing a
	// misconfiguration at boot, and design.md is explicit that "silently
	// not encrypted" — not "silently refuses" — is the failure this change
	// exists to remove. Refusing to boot is the loud alternative to both.
	mpKeyring, err := crypto.ParseKeyring(cfg.MP.CredentialKeys)
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
	// behind a deploy that never came up. Before the pool because the server's
	// pool may be opened as a role that cannot see a table the schema has not created
	// yet, and because a failure here must stop the boot rather than let the
	// server serve a schema it does not match.
	if cfg.DB.AutoMigrate {
		if err := runMigrations(cfg, logger); err != nil {
			sentry.Flush(2 * time.Second)
			//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is replicated on the line above.
			os.Exit(1)
		}
	}

	db, err := platformdb.Open(context.Background(), platformdb.Config{
		DSN:                cfg.DB.DSN,
		MaxOpenConns:       cfg.DB.MaxOpenConns,
		MaxIdleConns:       cfg.DB.MaxIdleConns,
		MaxIdleTime:        cfg.DB.MaxIdleTime,
		StatementTimeout:   cfg.DB.StatementTimeout,
		SlowQueryThreshold: cfg.DB.SlowQueryThreshold,
		PrepareConn:        data.StampTenantScope,
		Logger:             logger,
	})
	if err != nil {
		logger.Error("failed to open database connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("database connection pool established")

	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

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
	rdb, err := platformredis.Open(context.Background(), platformredis.Config{URL: cfg.Redis.URL})
	if err != nil {
		logger.Error("redis is required: the notification queue is Redis-backed and has no fallback, "+
			"so this instance would accept registrations and bookings while dropping every "+
			"verification email, password reset and confirmation. Set REDIS_URL to a reachable instance.",
			"error", err)
		sentry.Flush(2 * time.Second)
		//nolint:gocritic // exitAfterDefer: the deferred sentry.Flush is replicated on the line above.
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()
	logger.Info("redis connected")

	var objectStorage storage.ObjectStorage
	if cfg.R2.AccountID != "" && cfg.R2.AccessKey != "" && cfg.R2.SecretKey != "" {
		r2Client, err := storage.NewR2Client(cfg.R2.AccountID, cfg.R2.AccessKey, cfg.R2.SecretKey,
			cfg.R2.BucketName, cfg.R2.PublicURL)
		if err != nil {
			logger.Error("failed to initialize R2 storage", "error", err)
			os.Exit(1)
		}
		objectStorage = r2Client
		logger.Info("R2 storage initialized", "bucket", cfg.R2.BucketName)
	}

	app, err := newApplication(cfg, deps{
		logger: logger,
		models: stores.New(db, stores.Config{
			PaymentExpiry:   cfg.Booking.PaymentExpiry,
			Logger:          logger,
			Keys:            mpKeyring,
			LinkTokenBuffer: cfg.Booking.LinkTokenBuffer,
		}),
		db:             db,
		rdb:            rdb,
		storage:        objectStorage,
		trustedProxies: trustedProxySet,
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

	logger.Info("email configured", "mode", app.mailer.Mode(), "sender", cfg.Brevo.Sender)

	if app.whatsappEnabled {
		logger.Info("whatsapp notifications enabled")
	} else {
		logger.Info("whatsapp notifications disabled (no WHATSAPP_TOKEN/WHATSAPP_PHONE_NUMBER_ID)")
	}

	if cfg.Turnstile.SecretKey != "" {
		logger.Info("turnstile verification enabled")
	} else {
		logger.Info("turnstile verification disabled (no TURNSTILE_SECRET_KEY)")
		// Not a boot failure — an owner may deliberately self-host without
		// Turnstile — but a production deployment running unprotected
		// register/login/forgot-password endpoints is worth a loud line, not
		// just the info one above that a quiet log rotation could bury.
		if cfg.Env == "production" {
			logger.Warn("running in production without TURNSTILE_SECRET_KEY: " +
				"register, login and forgot-password accept no bot verification")
		}
	}

	if cfg.Google.OAuthClientID != "" {
		logger.Info("google sign-in enabled")
	} else {
		logger.Info("google sign-in disabled (no GOOGLE_OAUTH_CLIENT_ID)")
	}

	app.startCronJobs()

	if err := app.serve(); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
