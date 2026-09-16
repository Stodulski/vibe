package main

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/platform/config"
	"github.com/stodulski/vibe-server/internal/stores"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-secret-key-for-testing-only-32b"

func newTestApplication(t *testing.T) *application {
	t.Helper()

	app, _ := newTestApplicationWithNotifications(t)
	return app
}

// newTestApplicationWithStores is the harness with the store set the caller
// wants, named before the application is built.
//
// A domain service captures its stores at construction, exactly as production
// does, so a test that needs a double has to supply it here — assigning
// app.models afterwards reaches the field the service already read past.
func newTestApplicationWithStores(t *testing.T, customize func(*stores.Stores)) (*application, *memoryQueue) {
	t.Helper()

	return newTestApplicationWith(t, slog.New(slog.NewTextHandler(io.Discard, nil)), customize)
}

// newTestApplicationWithNotifications also hands back the queue the
// application's notification service publishes to: newApplication builds a
// *memoryQueue whenever deps.rdb is nil, which the harness's mock-store,
// no-Redis application always is — the same fallback production would use if
// it ever tried to boot without Redis, which main() refuses to do.
func newTestApplicationWithNotifications(t *testing.T) (*application, *memoryQueue) {
	t.Helper()

	return newTestApplicationWithLogger(t, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// newTestApplicationWithLogger is the same harness with the application's
// logger chosen by the caller. A component that captures the logger at
// construction — the domain services do — cannot be redirected by assigning
// app.logger afterwards, so a test that reads log output has to name the
// logger up front.
func newTestApplicationWithLogger(t *testing.T, testLogger *slog.Logger) (*application, *memoryQueue) {
	t.Helper()

	return newTestApplicationWith(t, testLogger, nil)
}

// newTestApplicationWith is the one harness both wrappers above go through:
// the caller chooses the logger and, optionally, adjusts the store set before
// newApplication reads it.
func newTestApplicationWith(t *testing.T, testLogger *slog.Logger, customize func(*stores.Stores)) (*application, *memoryQueue) {
	t.Helper()

	cfg := config.Config{
		Env: "test",
		JWT: config.JWT{Secret: testJWTSecret},
		// Rate limiting is ON in the harness, and deliberately so.
		//
		// It used to be off, and middleware.Config{} below carried the
		// same default, so RateLimit returned its next handler untouched
		// and rateLimitInMemory was never entered by a single test. The
		// data race on the shared *client it keeps per address was
		// therefore invisible to the whole suite, -race included: code
		// that never runs cannot be reported.
		//
		// The ceiling is far above what any test issues, so no test is
		// throttled by being exercised through the real path — but the
		// path is real. The auth and booking limiters inside it are
		// hardcoded (10 per 6s, 3 per 20s) and not raised by this, which
		// is correct: a test that trips one has found a genuine limit on
		// the endpoint it is hammering.
		Limiter: config.Limiter{
			Enabled:   true,
			RPS:       10_000,
			Burst:     10_000,
			UserRPS:   10_000,
			UserBurst: 10_000,
		},
		FrontendURL: "http://localhost:5173",
		// bcrypt.MinCost for the same reason internal/auth's own harness uses
		// it: the routes this suite drives register and sign in users, and a
		// cost-12 hash under the race detector takes seconds each.
		PasswordHashCost: bcrypt.MinCost,
		Booking: config.Booking{
			GracePeriod:     15 * time.Minute,
			PaymentExpiry:   15 * time.Minute,
			SlotLockTTL:     20 * time.Minute,
			LinkTokenBuffer: 24 * time.Hour,
		},
		Limits: config.Limits{MaxComplexes: 4},
	}

	models := stores.Stores{
		Users:             &mockUserStore{},
		UserIdentities:    &mockUserIdentityStore{},
		Tokens:            &mockTokenStore{},
		Complexes:         &mockComplexStore{},
		Courts:            &mockCourtStore{},
		Bookings:          &mockBookingStore{},
		BookingLinkTokens: &mockBookingLinkTokenStore{},
		Clients:           &mockClientStore{},
		Payments:          &mockPaymentStore{},
		EmailVerification: &mockEmailVerificationStore{},
		PasswordReset:     &mockPasswordResetStore{},
		EmailChange:       &mockEmailChangeStore{},
		FailedRefunds:     &mockFailedRefundStore{},
		WebhookEvents:     &mockWebhookEventStore{},
		SlotLocks:         &mockSlotLockStore{},
		Reports:           &mockReportStore{},
		Admin:             &mockAdminStore{},
		Audit:             &mockAuditStore{},
		Locks:             &mockLockStore{},
	}
	if customize != nil {
		customize(&models)
	}

	app, err := newApplication(cfg, deps{
		logger: testLogger,
		models: models,
		// db and rdb stay nil: no database, in-memory blacklist/hub/limiter,
		// and the memoryQueue branch of newApplication's notifier decision.
	})
	if err != nil {
		t.Fatalf("newTestApplication: %v", err)
	}

	queue, ok := app.queue.(*memoryQueue)
	if !ok {
		t.Fatalf("newTestApplication: app.queue is %T, not *memoryQueue — deps.rdb must be nil in this harness", app.queue)
	}

	// The integration harness has always closed this; the unit one had not.
	// Rate limiting is on here, so app.routes() starts the bucket evictor, and
	// without a close every test that builds a router leaves one running for
	// the rest of the process.
	t.Cleanup(func() {
		close(app.shutdown)
	})

	return app, queue
}

func newTestServer(t *testing.T, app *application) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(app.routes())
	t.Cleanup(ts.Close)
	return ts
}
