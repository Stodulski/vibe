package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/admin"
	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/auth"
	"github.com/stodulski/vibe-server/internal/bookings"
	"github.com/stodulski/vibe-server/internal/cashbox"
	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/clients"
	"github.com/stodulski/vibe-server/internal/complexes"
	"github.com/stodulski/vibe-server/internal/courts"
	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/health"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/leads"
	"github.com/stodulski/vibe-server/internal/mailer"
	"github.com/stodulski/vibe-server/internal/middleware"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
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
	db *platformdb.Pool
	// rdb backs Redis-dependent features; a nil value falls back to the
	// in-memory blacklist, hub, rate limiter and notification queue.
	rdb *platformredis.Client
	// storage MAY be nil (no R2 configured).
	storage storage.ObjectStorage
	// privateStorage is the bucket no public domain serves, read only through
	// presigned GETs. It MAY be nil (no R2_PRIVATE_BUCKET_NAME), and the
	// payments-export endpoints then answer 501.
	privateStorage storage.ObjectStorage
	// trustedProxies is the parsed -trusted-proxies/TRUSTED_PROXIES set: the
	// peers allowed to rewrite the client address. It arrives through deps
	// rather than through cfg because parsing it produces an internal/httpx
	// type, and the configuration package holds no domain types.
	trustedProxies httpx.TrustedProxies
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
		{"models.EmailChange", d.models.EmailChange != nil},
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
func newApplication(cfg config.Config, d deps) (*application, error) {
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

	// Request validation against that same document, for the environments
	// where a mismatch should stop in front of the person who can fix it.
	// Production is deliberately excluded: the conformance suite in CI runs
	// the same check at no runtime cost (API-03).
	var specValidator *middleware.SpecValidator
	if cfg.Env != "production" && cfg.OpenAPIValidateRequests {
		specValidator, err = middleware.NewSpecValidator(openapiHandler.Document(), respond, d.logger)
		if err != nil {
			return nil, fmt.Errorf("newApplication: %w", err)
		}
		d.logger.Info("openapi: validating every request against the embedded document", "env", cfg.Env)
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

	blacklist := auth.NewTokenBlacklist(d.rdb, d.logger, cfg.Env)
	events := realtime.NewHub(d.rdb, d.logger, cfg.Env)

	mpClient := mp.NewMPClient(cfg.MP.AccessToken, cfg.MP.WebhookSecret, cfg.MP.AppID, cfg.MP.ClientSecret, mpCB)
	mpOAuthClient := mp.NewMPClient(cfg.MP.AccessToken, cfg.MP.WebhookSecret, cfg.MP.AppID, cfg.MP.ClientSecret, mpOAuthCB)
	waClient := whatsapp.NewWAClient(cfg.WhatsApp.Token, cfg.WhatsApp.PhoneID, cfg.WhatsApp.VerifyToken, cfg.WhatsApp.AppSecret, waCB)
	mailerClient := mailer.New(mailer.Config{
		BrevoAPIKey:  cfg.Brevo.APIKey,
		SMTPHost:     cfg.SMTP.Host,
		SMTPPort:     cfg.SMTP.Port,
		SMTPUsername: cfg.SMTP.Username,
		SMTPPassword: cfg.SMTP.Password,
		Sender:       cfg.Brevo.Sender,
		LogoURL:      cfg.FrontendURL + "/logo.png",
		AppURL:       cfg.FrontendURL,
		CB:           mailerCB,
	})
	whatsappEnabled := cfg.WhatsApp.Token != "" && cfg.WhatsApp.PhoneID != ""
	turnstileClient := turnstile.New(turnstile.Config{
		SecretKey: cfg.Turnstile.SecretKey,
		CB:        turnstileCB,
	})
	googleVerifier := googleid.NewVerifier(googleid.Config{
		ClientID: cfg.Google.OAuthClientID,
		CB:       googleCB,
	})

	// The queue is the jobs table, so it is built from the store rather than
	// from the Redis client. A nil Jobs store is the unit suite, which builds
	// stores.Stores by hand with no database behind it: it gets the recording
	// memoryQueue instead. stores.New always builds a real one, so a
	// deployment cannot reach that branch.
	//
	// The expvar map is created here and injected (CON-07). It used to be a
	// package-level expvar.NewMap inside the queue, which registers into the
	// process's global namespace as an import side effect — so a second queue
	// in one process panicked on the duplicate name, and a test binary that
	// merely imported the package published counters. The name is unchanged:
	// /debug/vars still reads "notifier".
	var pool *jobs.Pool
	var queue notifications.Queue
	var jobRetention jobRetentionStore
	if d.models.Jobs != nil {
		jobRetention = d.models.Jobs
		metrics := queueMetrics()
		pool = jobs.NewPool(d.models.Jobs, jobs.Config{
			Workers:    4,
			JobTimeout: 10 * time.Second,
			// The payments export gets its own ceiling instead of running
			// under the shared 10s JobTimeout above: it does everything the
			// synchronous route does (see exportBudgetFor) plus an R2
			// upload the synchronous route never has to do. See
			// exportUploadAllowance and reporting.RegisterExportWorker.
			Timeouts: map[string]time.Duration{
				reporting.TaskExportPayments: exportBudgetFor(cfg.HTTP.WriteTimeout) + exportUploadAllowance,
			},
			Metrics: metrics,
			Logger:  d.logger,
		})
		queue = taskQueue{
			pool: pool,
			enqueuer: &jobs.Enqueuer{
				Store:   d.models.Jobs,
				Logger:  d.logger,
				Metrics: metrics,
			},
		}
	} else {
		queue = &memoryQueue{}
	}

	auditor := audit.NewRecorder(d.models.Audit, d.logger, app.background)
	auditService := audit.NewService(d.models.Audit, auditor)
	auditTrailHandler := audit.NewHandler(auditService, respond, d.trustedProxies.Any())

	tokens := auth.NewTokenService(auth.TokenServiceConfig{
		JWTSecret:         cfg.JWT.Secret,
		JWTKeyID:          cfg.JWT.KeyID,
		JWTSecretPrevious: cfg.JWT.SecretPrevious,
		JWTKeyIDPrevious:  cfg.JWT.KeyIDPrevious,
		CookieDomain:      cfg.CookieDomain,
		Environment:       cfg.Env,
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
		TrustedProxies:   d.trustedProxies,
		RateLimitEnabled: cfg.Limiter.Enabled,
		RateLimitRPS:     cfg.Limiter.RPS,
		RateLimitBurst:   cfg.Limiter.Burst,

		RateLimitUserRPS:   cfg.Limiter.UserRPS,
		RateLimitUserBurst: cfg.Limiter.UserBurst,
		RequestLogSample:   cfg.RequestLogSample,
		Env:                cfg.Env,
	})
	cache := userCache{mw: mw}

	placesHandler := places.NewHandler(places.Config{APIKey: cfg.Google.PlacesAPIKey}, respond)

	// The booking domain's cross-domain entry point, built over the booking
	// store alone and therefore available here, before any domain service
	// exists. Every domain that reads bookings — clients, complexes, courts,
	// auth, reporting, payments — takes this instead of d.models.Bookings, so
	// no domain outside internal/bookings holds a booking store. The booking
	// service still has to be built last, because it depends on those domains;
	// it embeds this same facade (see bookings.Dependencies.Facade below).
	bookingsFacade := bookings.NewFacade(d.models.Bookings)

	clientsService := clients.NewService(d.models.Clients, bookingsFacade)
	clientsHandler := clients.NewHandler(clientsService, respond)
	// The stream re-authorizes through the same chain that admitted it: see
	// streamAuthorizer. Its cadence and lifetime are the package's defaults.
	realtimeHandler := realtime.NewHandler(events, streamAuthorizer{mw: mw}, respond, d.logger,
		app.shutdown, realtime.Config{})
	leadsHandler := leads.NewHandler(respond, leads.Config{
		WebhookURL: cfg.Leads.AbandonedWebhookURL,
		Token:      cfg.Leads.AbandonedWebhookToken,
	})
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
	}, health.Config{Environment: cfg.Env, Version: version})

	// Domain services hold the rules; their handlers only decode, validate and
	// map errors. A service is passed wherever another domain reads this one,
	// so the entry point into a domain is its service rather than its store —
	// which is what lets a rule added later (authorization, caching) land in
	// one place.
	//
	// The order below follows the dependency edges, and the locals-then-publish
	// rule above makes a wrong order a compile error rather than a runtime
	// surprise. Two domains read each other, so the edges cannot all be
	// construction arguments:
	//
	//   - courts and complexes: complexes is built with no court port, courts
	//     takes the complexes service, and SetCourts closes the loop below —
	//     once, here, before the router exists.
	//   - bookings and payments: bookings takes the payments service, and
	//     payments takes bookingsFacade. The booking service cannot exist yet
	//     — it takes payments — so the facade is what stands in for it.
	//
	// Everything upstream of bookings (clients, complexes, courts, auth,
	// reporting) reads bookings through bookingsFacade for the same reason:
	// the booking service is built last, because it depends on all of them.
	complexesConfig := complexes.Config{
		FrontendURL:  cfg.FrontendURL,
		TrustProxies: d.trustedProxies.Any(),
		MPAppID:      cfg.MP.AppID,
	}
	// complexesService is built before courtsService because the court domain
	// reads venues and their opening hours through it, and with no court port
	// at all: the reverse edge — the public venue page reading that venue's
	// courts — is closed by SetCourts a few lines below, which is the only way
	// two mutually reading services can both end up holding the other.
	complexesService := complexes.NewService(complexes.Dependencies{
		Store:    d.models.Complexes,
		Courts:   nil,
		Bookings: bookingsFacade,
		Payments: mpClient,
		OAuth:    mpOAuthClient,
		Storage:  d.storage,
		Audit:    auditor,
		Logger:   d.logger,
		Run:      app.background,
	}, complexesConfig)
	complexesHandler := complexes.NewHandler(complexesService, respond, complexesConfig)

	courtsService := courts.NewService(d.models.Courts, bookingsFacade, complexesService, auditor)
	courtsHandler := courts.NewHandler(courtsService, respond, d.trustedProxies.Any())

	// cashbox reads booking payments through d.models.Reports (the same
	// aggregate store the monthly report uses), not through payments or
	// bookings directly: reportstore.Store.PaymentSummaryByMethodWindow is
	// what already owns "what counts as collected money", and cashbox's cash
	// reconciliation must never disagree with the monthly report about that.
	cashboxService := cashbox.NewService(d.models.Cashbox, d.models.Reports, auditor)
	cashboxHandler := cashbox.NewHandler(cashboxService, respond, d.trustedProxies.Any())

	// The one edge that cannot be a constructor argument, closed the moment the
	// other side exists: before any handler is built, before the router is
	// built, and therefore before a request can reach the public venue page
	// that reads it. complexes.Service.publicCourts panics on a nil port rather
	// than answering a venue page with no courts on it, so moving this line
	// below the router fails loudly instead of shipping an empty list.
	complexesService.SetCourts(courtsService)

	// Built before the handlers that capture it: auth, payments and bookings
	// all take notify, and none of them can compile before this line runs.
	notify := notifications.NewService(queue, mailerClient, waClient, d.models.Users, d.logger, whatsappEnabled)

	authConfig := auth.Config{
		JWTSecret:         cfg.JWT.Secret,
		JWTKeyID:          cfg.JWT.KeyID,
		JWTSecretPrevious: cfg.JWT.SecretPrevious,
		JWTKeyIDPrevious:  cfg.JWT.KeyIDPrevious,
		CookieDomain:      cfg.CookieDomain,
		Environment:       cfg.Env,
		FrontendURL:       cfg.FrontendURL,
		TrustProxies:      d.trustedProxies.Any(),
		PasswordHashCost:  cfg.PasswordHashCost,
	}

	authService := auth.NewService(auth.Dependencies{
		Users:         d.models.Users,
		Tokens:        d.models.Tokens,
		Verifications: d.models.EmailVerification,
		Resets:        d.models.PasswordReset,
		EmailChanges:  d.models.EmailChange,
		Complexes:     complexesService,
		Bookings:      bookingsFacade,
		Blacklist:     blacklist,
		Notify:        notify,
		Cache:         cache,
		Audit:         auditor,
		Turnstile:     turnstileClient,
		Google:        googleVerifier,
		Identities:    d.models.UserIdentities,
		// The redirect-mode one-time codes are minted on whichever instance
		// Google's post landed on and spent on whichever one the frontend's
		// exchange reaches, so they have to live in Redis and not in a
		// process — see auth.GoogleCodes.
		GoogleCodes: auth.NewGoogleCodes(d.rdb, cfg.Env),
		Respond:     respond,
		Logger:      d.logger,
	}, authConfig)
	authHandler := auth.NewHandler(authService, respond, d.logger, authConfig)

	adminService := admin.NewService(d.models.Admin, auditService, cache, auditor)
	adminHandler := admin.NewHandler(adminService, respond, d.trustedProxies.Any())

	// The export dependencies are assigned only when they exist, because a
	// typed nil in an interface is not a nil interface: handing over a nil
	// *jobs.Store would make ExportsConfigured say yes and the first call
	// panic. A deployment with no jobs table, and the unit suite, leave both
	// nil and the export endpoints answer 501.
	var exportDeps reporting.ExportDeps
	if d.models.Jobs != nil {
		exportDeps.Store = d.models.Jobs
	}
	if d.privateStorage != nil {
		exportDeps.Storage = d.privateStorage
	}

	reportingService := reporting.NewService(bookingsFacade, clientsService, courtsService,
		complexesService, d.models.Reports, exportDeps, exportBudgetFor(cfg.HTTP.WriteTimeout))
	reportingHandler := reporting.NewHandler(reportingService, respond)

	// Registered here rather than beside notify.RegisterWorkers() in main.go
	// because the export worker needs the reporting service, which is a local
	// of this constructor and is not published on the application. Both
	// registrations still happen before main starts the pool, which is the
	// only ordering that matters: a claimed job whose type has no handler is
	// released rather than run.
	reportingService.RegisterExportWorker(queue)

	publicsiteService := publicsite.NewService(complexesService, cfg.FrontendURL)
	publicsiteHandler := publicsite.NewHandler(publicsiteService, respond)

	paymentsService := payments.NewService(payments.Dependencies{
		Payments:  d.models.Payments,
		Clients:   clientsService,
		Complexes: complexesService,
		Courts:    courtsService,
		// Bookings and RefundIntents both take the facade: the booking service
		// takes this one (AutoRefundIfPaid), so it cannot exist yet. See the
		// note on the two mutually reading domains above.
		Bookings:      bookingsFacade,
		FailedRefunds: d.models.FailedRefunds,
		WebhookEvents: d.models.WebhookEvents,
		// The same facade satisfies payments.RefundIntentStore, whose three
		// methods it carries for the reconciliation sweep — no second entry
		// point into the booking domain.
		RefundIntents: bookingsFacade,
		// d.models.BookingLinkTokens is typed stores.BookingLinkTokenStore, which
		// already structurally satisfies payments.LinkMinter's one method — no
		// new store instance is constructed.
		LinkTokens: d.models.BookingLinkTokens,
		Locks:      d.models.Locks,
		Provider:   mpClient,
		Notify:     notify,
		Realtime:   events,
		Audit:      auditor,
		Logger:     d.logger,
		Run:        app.background,
	}, payments.Config{
		FrontendURL:             cfg.FrontendURL,
		CancellationGracePeriod: cfg.Booking.GracePeriod,
		LinkTokenBuffer:         cfg.Booking.LinkTokenBuffer,
	})
	// The webhook handler verifies MercadoPago's signature itself, so it takes
	// the provider alongside the service: the signature is computed over the
	// request, which never reaches the service.
	paymentsHandler := payments.NewHandler(paymentsService, mpClient, respond, d.logger)

	// bookings.Dependencies.Refunds takes paymentsService, the local above —
	// not app.payments. Move this block above paymentsService's and
	// `undefined: paymentsService` fails the build, before any test runs.
	bookingsService := bookings.NewService(bookings.Dependencies{
		// The same facade every other domain took above, so the cross-domain
		// reads have one implementation whoever the caller is.
		Facade:    bookingsFacade,
		Store:     d.models.Bookings,
		Clients:   clientsService,
		Complexes: complexesService,
		Courts:    courtsService,
		Payments:  paymentsService,
		Locks:     d.models.SlotLocks,
		Checkout:  mpClient,
		Refunds:   paymentsService,
		// d.models.BookingLinkTokens is typed stores.BookingLinkTokenStore, which
		// already structurally satisfies both bookings.LinkResolver's one
		// method and bookings.LinkTokenStore's two — no new store instance is
		// constructed for either.
		LinkResolver: d.models.BookingLinkTokens,
		LinkTokens:   d.models.BookingLinkTokens,
		Notify:       notify,
		Realtime:     events,
		Audit:        auditor,
		Logger:       d.logger,
		Run:          app.background,
	}, bookings.Config{
		FrontendURL:     cfg.FrontendURL,
		BackendURL:      cfg.BackendURL,
		Environment:     cfg.Env,
		GracePeriod:     cfg.Booking.GracePeriod,
		PaymentExpiry:   cfg.Booking.PaymentExpiry,
		SlotLockTTL:     cfg.Booking.SlotLockTTL,
		TrustProxies:    d.trustedProxies.Any(),
		WhatsAppEnabled: whatsappEnabled,
		LinkTokenBuffer: cfg.Booking.LinkTokenBuffer,
	})
	// The WhatsApp webhook verifies Meta's own handshake and signature, so the
	// handler takes the client alongside the service: both are computed over
	// the request, which never reaches the service.
	bookingsHandler := bookings.NewHandler(bookingsService, waClient, respond, d.logger, d.trustedProxies.Any())

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
	app.specValidator = specValidator
	app.courts = courtsHandler
	app.cashbox = cashboxHandler
	app.tokens = tokens
	app.middleware = mw
	app.notify = notify
	app.auth = authHandler
	app.payments = paymentsHandler
	app.paymentsService = paymentsService
	app.bookings = bookingsHandler
	app.bookingsService = bookingsService
	app.complexes = complexesHandler
	app.complexesService = complexesService
	app.scheduler = sched
	app.mp = mpClient
	app.mpOAuth = mpOAuthClient
	app.wa = waClient
	app.mailer = mailerClient
	app.jobs = pool
	app.jobRetention = jobRetention
	app.queue = queue
	app.blacklist = blacklist
	app.events = events
	app.whatsappEnabled = whatsappEnabled

	return app, nil
}
