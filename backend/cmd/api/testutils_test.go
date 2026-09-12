package main

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/stores"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-secret-key-for-testing-only-32b"

func newTestApplication(t *testing.T) *application {
	t.Helper()

	app, _ := newTestApplicationWithNotifications(t)
	return app
}

// newTestApplicationWithNotifications also hands back the queue the
// application's notification service publishes to: newApplication builds a
// *memoryQueue whenever deps.rdb is nil, which the harness's mock-store,
// no-Redis application always is — the same fallback production would use if
// it ever tried to boot without Redis, which main() refuses to do.
func newTestApplicationWithNotifications(t *testing.T) (*application, *memoryQueue) {
	t.Helper()

	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := config{
		env: "test",
		jwt: struct{ secret string }{
			secret: testJWTSecret,
		},
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
		limiter: struct {
			enabled bool
			rps     float64
			burst   int
		}{
			enabled: true,
			rps:     10_000,
			burst:   10_000,
		},
		frontendURL: "http://localhost:5173",
		// bcrypt.MinCost for the same reason internal/auth's own harness uses
		// it: the routes this suite drives register and sign in users, and a
		// cost-12 hash under the race detector takes seconds each.
		passwordHashCost: bcrypt.MinCost,
		booking: struct {
			gracePeriod        time.Duration
			paymentExpiry      time.Duration
			cancellationWindow time.Duration
			slotLockTTL        time.Duration
			linkTokenBuffer    time.Duration
		}{
			gracePeriod:     15 * time.Minute,
			paymentExpiry:   15 * time.Minute,
			slotLockTTL:     20 * time.Minute,
			linkTokenBuffer: 24 * time.Hour,
		},
		limits: struct {
			maxComplexes int
		}{
			maxComplexes: 4,
		},
	}

	app, err := newApplication(cfg, deps{
		logger: testLogger,
		models: stores.Stores{
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
			FailedRefunds:     &mockFailedRefundStore{},
			WebhookEvents:     &mockWebhookEventStore{},
			SlotLocks:         &mockSlotLockStore{},
			Reports:           &mockReportStore{},
			Admin:             &mockAdminStore{},
			Audit:             &mockAuditStore{},
			Locks:             &mockLockStore{},
		},
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
