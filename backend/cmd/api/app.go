package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/admin"
	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/auth"
	"github.com/stodulski/vibe-server/internal/bookings"
	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/clients"
	"github.com/stodulski/vibe-server/internal/complexes"
	"github.com/stodulski/vibe-server/internal/courts"
	"github.com/stodulski/vibe-server/internal/googleid"
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
	"github.com/stodulski/vibe-server/internal/turnstile"
	"github.com/stodulski/vibe-server/internal/whatsapp"
)

// deps holds the values newApplication cannot derive from cfg: the
// substitution boundary between a real deployment and a test. Everything
// else the application needs is built inside the constructor from cfg alone.
type deps struct {
	logger *slog.Logger
	// models is the full set of stores. All 18 are required — see
	// validateDeps.
	models stores.Stores
	// db backs the database health probe only; it MAY be nil (health then
	// reports "not configured" instead of failing).
	db *pgxpool.Pool
	// rdb backs Redis-dependent features; a nil value falls back to the
	// in-memory blacklist, hub, rate limiter and notification queue.
	rdb *redis.Client
	// storage MAY be nil (no R2 configured).
	storage storage.ObjectStorage
}

// validateDeps checks that every store stores.Stores composes is present,
// naming the ones that are not.
//
// This runs before any constructor below is called — the gap the checklist
// it replaces (application.unwiredDependencies, cmd/api/main.go) could not
// close: that guard ran after eight handlers were already built, so it could
// not cover a store a handler captures before the guard's own call site. A
// store is a leaf dependency with nothing upstream of it in this
// constructor, so checking it first has no such gap.
func validateDeps(d deps) error {
	stores := []struct {
		name    string
		present bool
	}{
		{"models.Users", d.models.Users != nil},
		{"models.UserIdentities", d.models.UserIdentities != nil},
		{"models.Complexes", d.models.Complexes != nil},
		{"models.Courts", d.models.Courts != nil},
		{"models.Bookings", d.models.Bookings != nil},
		{"models.BookingLinkTokens", d.models.BookingLinkTokens != nil},
		{"models.Tokens", d.models.Tokens != nil},
		{"models.Clients", d.models.Clients != nil},
		{"models.Payments", d.models.Payments != nil},
		{"models.EmailVerification", d.models.EmailVerification != nil},
		{"models.PasswordReset", d.models.PasswordReset != nil},
		{"models.FailedRefunds", d.models.FailedRefunds != nil},
		{"models.WebhookEvents", d.models.WebhookEvents != nil},
		{"models.SlotLocks", d.models.SlotLocks != nil},
		{"models.Admin", d.models.Admin != nil},
		{"models.Audit", d.models.Audit != nil},
		{"models.Reports", d.models.Reports != nil},
		{"models.Locks", d.models.Locks != nil},
	}

	var missing []string
	for _, s := range stores {
		if !s.present {
			missing = append(missing, s.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("newApplication: missing required store(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// newApplication assembles an *application from cfg and deps.
//
// Assembly runs in three phases under one rule: phase 1 (above, via
// validateDeps) validates before anything is built. Phase 2 constructs every
// component into a local variable; a dependent takes the local (`paymentsHandler`),
// never a field of app (`app.payments`). Phase 3 is a write-only block that
// publishes those locals into the struct.
//
// That is what turns the wrong-order mistake this constructor exists to
// remove into a compile error: reorder the bookings block above the payments
// block it depends on, and `paymentsHandler` is undefined at that point in
// the file, so `go build` fails. A field of app can be read in the wrong
// order and still compile — a local cannot be referenced before its `:=`.
//
// Three exceptions capture app itself, not one of its fields, and are safe by
// construction: app.background (a method value — only wg and logger, which
// are set on the shell below before anything else runs) and
// processMetrics{app: app} (a struct field read at request time, long after
// this function has returned a fully published app).
//
// newApplication launches no goroutine, opens no connection and registers no
// process-global. It can be called more than once in the same process.
//
// Splitting it into helpers would either move the ordering guarantee out of
// this function's own compiler-checked scope, or reintroduce the
// grouped-sub-struct blind spot design.md's "Alternatives rejected" section
// already rules out as the primary safeguard. main() carries the same funlen
// exemption for the same reason.
//
//nolint:funlen // flat sequential locals-then-publish wiring, see above.
func newApplication(cfg config, d deps) (*application, error) {
	if err := validateDeps(d); err != nil {
		return nil, err
	}

	// The phase-0 shell. Only fields with no construction-order dependency —
	// raw inputs, not values this function builds — are set here, so that
	// app.background can be taken as a method value below without depending
	// on anything phase 2 constructs.
	app := &application{
		config:   cfg,
		logger:   d.logger,
		db:       d.db,
		rdb:      d.rdb,
		models:   d.models,
		storage:  d.storage,
		shutdown: make(chan struct{}),
	}

	// --- phase 2: construct into locals -------------------------------------

	respond := httpx.NewResponder(d.logger)

	// The embedded OpenAPI document is parsed and schema-validated here, once,
	// rather than per request. A broken document is a deploy-time mistake, not
	// a runtime one, so it fails the boot instead of serving nothing (or
	// garbage) on the first request to reach /api/v1/openapi.json.
	openapiHandler, err := openapi.NewHandler(respond)
	if err != nil {
		return nil, fmt.Errorf("newApplication: %w", err)
	}

	cbStateChange := func(name string, from, to circuitbreaker.State) {
		d.logger.Warn("circuit breaker state change", "service", name, "from", from.String(), "to", to.String())
	}
	mpCB := circuitbreaker.New(circuitbreaker.Config{
		Name:            "mercadopago",
		MaxFailures:     5,
		ResetTimeout:    30 * time.Second,
		HalfOpenMaxReqs: 2,
		OnStateChange:   cbStateChange,
	})
	// The bulk OAuth refresh gets its own breaker, and that separation is the
	// point rather than tidiness.
	//
	// cronRefreshMPTokens walks every connected complex on one client. Five
	// venues with a stale or unreadable refresh token are enough to spend
	// mpCB's failure budget — and mpCB gates CreatePreference, GetPayment and
	// RefundPayment, so a sweep over other people's expired credentials would
	// take checkout down for every tenant at once, twelve hours after anybody
	// last touched it. The symptom an operator sees first is checkout 503s with
	// no payment-side cause.
	//
	// Same platform credentials, same provider, different blast radius.
	mpOAuthCB := circuitbreaker.New(circuitbreaker.Config{
		Name:            "mercadopago-oauth",
		MaxFailures:     5,
		ResetTimeout:    30 * time.Second,
		HalfOpenMaxReqs: 2,
		OnStateChange:   cbStateChange,
	})
	waCB := circuitbreaker.New(circuitbreaker.Config{
		Name:            "whatsapp",
		MaxFailures:     3,
		ResetTimeout:    60 * time.Second,
		HalfOpenMaxReqs: 1,
		OnStateChange:   cbStateChange,
	})
	mailerCB := circuitbreaker.New(circuitbreaker.Config{
		Name:            "mailer",
		MaxFailures:     3,
		ResetTimeout:    60 * time.Second,
		HalfOpenMaxReqs: 1,
		OnStateChange:   cbStateChange,
	})
	turnstileCB := circuitbreaker.New(circuitbreaker.Config{
		Name:            "turnstile",
		MaxFailures:     3,
		ResetTimeout:    60 * time.Second,
		HalfOpenMaxReqs: 1,
		OnStateChange:   cbStateChange,
	})
	googleCB := circuitbreaker.New(circuitbreaker.Config{
		Name:            "googleid",
		MaxFailures:     3,
		ResetTimeout:    60 * time.Second,
		HalfOpenMaxReqs: 1,
		OnStateChange:   cbStateChange,
	})

	blacklist := auth.NewTokenBlacklist(d.rdb, d.logger)
	events := realtime.NewHub(d.rdb, d.logger)

	mpClient := mp.NewMPClient(cfg.mp.accessToken, cfg.mp.webhookSecret, cfg.mp.appID, cfg.mp.clientSecret, mpCB)
	mpOAuthClient := mp.NewMPClient(cfg.mp.accessToken, cfg.mp.webhookSecret, cfg.mp.appID, cfg.mp.clientSecret, mpOAuthCB)
	waClient := whatsapp.NewWAClient(cfg.whatsapp.token, cfg.whatsapp.phoneID, cfg.whatsapp.verifyToken, cfg.whatsapp.appSecret, waCB)
	mailerClient := mailer.New(mailer.Config{
		BrevoAPIKey:  cfg.brevo.apiKey,
		SMTPHost:     cfg.smtp.host,
		SMTPPort:     cfg.smtp.port,
		SMTPUsername: cfg.smtp.username,
		SMTPPassword: cfg.smtp.password,
		Sender:       cfg.brevo.sender,
		LogoURL:      cfg.frontendURL + "/logo.png",
		AppURL:       cfg.frontendURL,
		CB:           mailerCB,
	})
	whatsappEnabled := cfg.whatsapp.token != "" && cfg.whatsapp.phoneID != ""
	turnstileClient := turnstile.New(turnstile.Config{
		SecretKey: cfg.turnstile.secretKey,
		CB:        turnstileCB,
	})
	googleVerifier := googleid.NewVerifier(googleid.Config{
		ClientID: cfg.google.oauthClientID,
		CB:       googleCB,
	})

	// notifier.Enqueue is not nil-safe, so a nil rdb builds the recording
	// memoryQueue instead of a notifier.Notifier pointed at nothing. nf stays
	// nil in that case: app.notifier is only ever started by main(), which
	// never runs without Redis (see the boot guard in main.go), so a nil
	// *notifier.Notifier here is never asked to Start.
	var nf *notifier.Notifier
	var queue notifications.Queue
	if d.rdb != nil {
		nf = notifier.New(d.rdb, d.logger, notifier.Config{
			Workers:     4,
			TaskTimeout: 10 * time.Second,
		})
		queue = taskQueue{n: nf}
	} else {
		queue = &memoryQueue{}
	}

	auditor := audit.NewRecorder(d.models.Audit, d.logger, app.background)
	auditTrailHandler := audit.NewHandler(d.models.Audit, auditor, respond, cfg.trustedProxies)

	tokens := auth.NewTokenService(auth.TokenServiceConfig{
		JWTSecret:    cfg.jwt.secret,
		CookieDomain: cfg.cookieDomain,
		Environment:  cfg.env,
	})
	mw := middleware.New(middleware.Dependencies{
		Users:     d.models.Users,
		Complexes: d.models.Complexes,
		Tokens:    tokens,
		Blacklist: blacklist,
		Redis:     d.rdb,
		Respond:   respond,
		Logger:    d.logger,
		Shutdown:  app.shutdown,
	}, middleware.Config{
		TrustedProxies:   cfg.trustedProxySet,
		RateLimitEnabled: cfg.limiter.enabled,
		RateLimitRPS:     cfg.limiter.rps,
		RateLimitBurst:   cfg.limiter.burst,
		RequestLogSample: cfg.requestLogSample,
	})
	cache := userCache{mw: mw}

	placesHandler := places.NewHandler(places.Config{APIKey: cfg.google.placesAPIKey}, respond)
	clientsService := clients.NewService(d.models.Clients, d.models.Bookings)
	clientsHandler := clients.NewHandler(clientsService, respond)
	// The stream re-authorizes through the same chain that admitted it: see
	// streamAuthorizer. Its cadence and lifetime are the package's defaults.
	realtimeHandler := realtime.NewHandler(events, streamAuthorizer{mw: mw}, respond, d.logger,
		app.shutdown, realtime.Config{})
	publicsiteHandler := publicsite.NewHandler(d.models.Complexes, respond, cfg.frontendURL)
	leadsHandler := leads.NewHandler(respond, leads.Config{
		WebhookURL: cfg.leads.abandonedWebhookURL,
		Token:      cfg.leads.abandonedWebhookToken,
	})
	reportingHandler := reporting.NewHandler(d.models.Bookings, d.models.Clients, d.models.Courts,
		d.models.Complexes, d.models.Reports, respond)
	adminHandler := admin.NewHandler(d.models.Admin, d.models.Audit, cache, auditor, respond, cfg.trustedProxies)

	var queues health.QueueReporter
	if d.db != nil {
		queues = queueProbe{pool: d.db}
	}
	database, dbCache, dbPool, cachePool := healthProbes(d.db, d.rdb)
	healthHandler := health.NewHandler(health.Dependencies{
		Database:  database,
		Cache:     dbCache,
		DBPool:    dbPool,
		CachePool: cachePool,
		// Without these the check is blind to the payment stack: with
		// MercadoPago down, no client on any tenant can pay and the endpoint
		// still answered "available".
		Breakers: []health.Breaker{
			breakerProbe{cb: mpCB, blocksRevenue: true},
			breakerProbe{cb: waCB},
			breakerProbe{cb: mailerCB},
		},
		Queues: queues,
		// Reads app.middleware, which phase 3 publishes below. processMetrics
		// holds the application rather than the middleware, so the order is
		// not a trap: this is read at request time, long after app is fully
		// published.
		Metrics: processMetrics{app: app},
		Respond: respond,
	}, health.Config{Environment: cfg.env, Version: appVersion})

	// Domain services hold the rules; the handlers below only decode, validate
	// and map errors. A service is passed wherever another domain reads this
	// one, so the entry point into a domain is its service rather than its
	// store. The bookings and payments modules are the exception until their
	// own services land: they still receive stores.
	complexesConfig := complexes.Config{
		MaxComplexes: cfg.limits.maxComplexes,
		FrontendURL:  cfg.frontendURL,
		TrustProxies: cfg.trustedProxies,
		MPAppID:      cfg.mp.appID,
	}
	// complexesService is built before courtsService because the court domain
	// reads venues and their opening hours through it. The reverse edge — the
	// public venue page reading that venue's courts — is the one place a store
	// is still passed between two converted domains: the two services cannot
	// both be constructed second.
	complexesService := complexes.NewService(complexes.Dependencies{
		Store:    d.models.Complexes,
		Courts:   d.models.Courts,
		Bookings: d.models.Bookings,
		Payments: mpClient,
		OAuth:    mpOAuthClient,
		Storage:  d.storage,
		Audit:    auditor,
		Logger:   d.logger,
		Run:      app.background,
	}, complexesConfig)
	complexesHandler := complexes.NewHandler(complexesService, respond, complexesConfig)

	courtsService := courts.NewService(d.models.Courts, d.models.Bookings, complexesService, auditor)
	courtsHandler := courts.NewHandler(courtsService, respond, cfg.trustedProxies)

	// Built before the handlers that capture it: auth, payments and bookings
	// all take notify, and none of them can compile before this line runs.
	notify := notifications.NewService(queue, mailerClient, waClient, d.models.Users, d.logger, whatsappEnabled)

	authConfig := auth.Config{
		JWTSecret:        cfg.jwt.secret,
		CookieDomain:     cfg.cookieDomain,
		Environment:      cfg.env,
		FrontendURL:      cfg.frontendURL,
		TrustProxies:     cfg.trustedProxies,
		PasswordHashCost: cfg.passwordHashCost,
	}

	authService := auth.NewService(auth.Dependencies{
		Users:         d.models.Users,
		Tokens:        d.models.Tokens,
		Verifications: d.models.EmailVerification,
		Resets:        d.models.PasswordReset,
		Complexes:     complexesService,
		Bookings:      d.models.Bookings,
		Blacklist:     blacklist,
		Notify:        notify,
		Cache:         cache,
		Audit:         auditor,
		Turnstile:     turnstileClient,
		Google:        googleVerifier,
		Identities:    d.models.UserIdentities,
		Respond:       respond,
		Logger:        d.logger,
	}, authConfig)
	authHandler := auth.NewHandler(authService, respond, d.logger, authConfig)

	paymentsHandler := payments.NewHandler(payments.Dependencies{
		Payments:      d.models.Payments,
		Bookings:      d.models.Bookings,
		Clients:       d.models.Clients,
		Complexes:     d.models.Complexes,
		Courts:        d.models.Courts,
		FailedRefunds: d.models.FailedRefunds,
		WebhookEvents: d.models.WebhookEvents,
		// d.models.Bookings is typed stores.BookingStore, which composes
		// BookingRefundIntentManager, so it already structurally satisfies
		// payments.RefundIntentStore — no new store instance is constructed.
		RefundIntents: d.models.Bookings,
		// d.models.BookingLinkTokens is typed stores.BookingLinkTokenStore, which
		// already structurally satisfies payments.LinkMinter's one method — no
		// new store instance is constructed.
		LinkTokens: d.models.BookingLinkTokens,
		Locks:      d.models.Locks,
		Provider:   mpClient,
		Notify:     notify,
		Realtime:   events,
		Audit:      auditor,
		Respond:    respond,
		Logger:     d.logger,
		Run:        app.background,
	}, payments.Config{
		FrontendURL:             cfg.frontendURL,
		CancellationGracePeriod: cfg.booking.gracePeriod,
		LinkTokenBuffer:         cfg.booking.linkTokenBuffer,
	})

	// bookings.Dependencies.Refunds takes paymentsHandler, the local above —
	// not app.payments. Move this block above paymentsHandler's and
	// `undefined: paymentsHandler` fails the build, before any test runs.
	bookingsHandler := bookings.NewHandler(bookings.Dependencies{
		Store:     d.models.Bookings,
		Clients:   d.models.Clients,
		Complexes: d.models.Complexes,
		Courts:    d.models.Courts,
		Payments:  d.models.Payments,
		Locks:     d.models.SlotLocks,
		Checkout:  mpClient,
		WhatsApp:  waClient,
		Refunds:   paymentsHandler,
		// d.models.BookingLinkTokens is typed stores.BookingLinkTokenStore, which
		// already structurally satisfies bookings.LinkResolver's one method —
		// no new store instance is constructed.
		LinkResolver: d.models.BookingLinkTokens,
		Notify:       notify,
		Realtime:     events,
		Audit:        auditor,
		Respond:      respond,
		Logger:       d.logger,
		Run:          app.background,
	}, bookings.Config{
		FrontendURL:     cfg.frontendURL,
		BackendURL:      cfg.backendURL,
		Environment:     cfg.env,
		GracePeriod:     cfg.booking.gracePeriod,
		PaymentExpiry:   cfg.booking.paymentExpiry,
		SlotLockTTL:     cfg.booking.slotLockTTL,
		TrustProxies:    cfg.trustedProxies,
		WhatsAppEnabled: whatsappEnabled,
	})

	// The locker is wrapped so a run that never happened still leaves a line:
	// the scheduler calls a job only when it took the lock, so on a
	// multi-instance deployment the instances that skipped were silent.
	sched := scheduler.New(observedLocker{inner: d.models.Locks, logger: d.logger}, d.logger, app.background)

	// --- phase 3: publish (write-only) --------------------------------------

	app.respond = respond
	app.auditor = auditor
	app.auditTrail = auditTrailHandler
	app.places = placesHandler
	app.clients = clientsHandler
	app.realtime = realtimeHandler
	app.publicsite = publicsiteHandler
	app.leads = leadsHandler
	app.reporting = reportingHandler
	app.admin = adminHandler
	app.queues = queues
	app.health = healthHandler
	app.openapi = openapiHandler
	app.courts = courtsHandler
	app.tokens = tokens
	app.middleware = mw
	app.notify = notify
	app.auth = authHandler
	app.payments = paymentsHandler
	app.bookings = bookingsHandler
	app.complexes = complexesHandler
	app.complexesService = complexesService
	app.scheduler = sched
	app.mp = mpClient
	app.mpOAuth = mpOAuthClient
	app.wa = waClient
	app.mailer = mailerClient
	app.notifier = nf
	app.queue = queue
	app.blacklist = blacklist
	app.events = events
	app.whatsappEnabled = whatsappEnabled

	return app, nil
}
