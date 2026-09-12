//go:build integration

package payments

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// integrationFixture wires a payments.Service whose Bookings, Payments,
// RefundIntents and Complexes dependencies are the real, data-backed stores —
// what SweepOrphanedRefundIntents and AutoRefundIfPaid actually read and
// write. Everything else (Clients, Courts, FailedRefunds, WebhookEvents,
// Locks, Provider, Notify, Realtime) is a stub from stubs_test.go: this suite
// never reaches the success path that would need any of them, because the
// fixture's complex has no MercadoPago credential connected, so every claimed
// refund refuses at sellerCredential rather than calling a real provider —
// exactly the same "committed attempt, refused before the provider" shape
// Phase 5's tests already exercise, here proven to also hold when the claim
// came from the sweep rather than a request.
//
// It embeds datatest.Fixture: Isolated by default, since every test but the
// concurrency one below drives the sweep sequentially and fits inside one
// rolled-back transaction; Shared only for the test that races two real
// goroutines against the database at once, which one transaction cannot do.
type integrationFixture struct {
	*datatest.Fixture
	service *Service
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()
	return buildIntegrationFixture(t, datatest.Isolated(t))
}

// newSharedIntegrationFixture is for TestTwoConcurrentSweepsClaimTheOrphanExactlyOnce,
// the one test that needs two genuinely concurrent database sessions.
func newSharedIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()
	return buildIntegrationFixture(t, datatest.Shared(t))
}

func buildIntegrationFixture(t *testing.T, base *datatest.Fixture) *integrationFixture {
	t.Helper()

	f := &integrationFixture{Fixture: base}

	logger := slog.New(slog.NewTextHandler(&discard{}, nil))
	f.service = NewService(Dependencies{
		Payments:      f.Stores.Payments,
		Bookings:      f.Stores.Bookings,
		Clients:       &stubClients{},
		Complexes:     f.Stores.Complexes,
		Courts:        &stubCourts{},
		FailedRefunds: &stubFailedRefunds{},
		WebhookEvents: &stubWebhookEvents{},
		RefundIntents: f.Stores.Bookings,
		Audit:         &stubRecorder{},
		Locks:         &stubLocks{},
		Provider:      &stubProvider{},
		Notify:        &stubNotifier{},
		Realtime:      &stubBroadcaster{},
		Logger:        logger,
		Run:           func(fn func()) { fn() },
	}, Config{FrontendURL: "https://vibe.test", CancellationGracePeriod: 15 * time.Minute})
	return f
}

type discard struct{}

func (*discard) Write(p []byte) (int, error) { return len(p), nil }

// createOrphan inserts a cancelled, deposit-paid booking with a real payment
// row and a refund-intent marker aged past the sweep's grace period — the
// row a genuine crash between the cancel commit and ClaimRefund leaves
// behind.
func (f *integrationFixture) createOrphan(t *testing.T) (*bookingstore.Booking, *paymentstore.Payment) {
	t.Helper()
	ctx := context.Background()

	b := &bookingstore.Booking{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Date:            time.Now().AddDate(0, 0, 7),
		StartTime:       "18:00",
		DurationMinutes: 90, Price: 500_000, DepositAmount: 150_000,
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		RefundStatus: bookingstore.RefundStatusNone,
	}
	if err := f.Stores.Bookings.Insert(f.Scoped(ctx), b); err != nil {
		t.Fatalf("creating booking: %v", err)
	}

	mpPaymentID := "mp-" + uuid.NewString()
	payment := &paymentstore.Payment{
		BookingID: b.ID, ComplexID: f.ComplexID, Amount: 150_000, ServiceFee: 7_500,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpPaymentID,
	}
	if err := f.Stores.Payments.Insert(f.Scoped(ctx), payment); err != nil {
		t.Fatalf("creating payment: %v", err)
	}

	if _, err := f.DB.Exec(ctx,
		`UPDATE bookings SET refund_intent_at = NOW() - INTERVAL '10 minutes' WHERE id = $1`, b.ID,
	); err != nil {
		t.Fatalf("backdating refund_intent_at: %v", err)
	}

	return b, payment
}

// The business-level half of the proposal's success criterion — "proven with
// two concurrent sweepers" — end to end: two real SweepOrphanedRefundIntents
// invocations racing on the same orphan must leave exactly one refund
// attempt claimed, never zero (the orphan lost) and never two (the client
// refunded twice). internal/data's TestTwoConcurrentSweepersOnlyOneClaimsAnOrphan
// proves the compare-and-swap directly; this proves what depends on it.
func TestTwoConcurrentSweepsClaimTheOrphanExactlyOnce(t *testing.T) {
	f := newSharedIntegrationFixture(t)
	_, payment := f.createOrphan(t)

	var start sync.WaitGroup
	start.Add(1)
	var done sync.WaitGroup
	done.Add(2)

	for range 2 {
		go func() {
			defer done.Done()
			start.Wait() // release both goroutines as close to together as possible
			f.service.SweepOrphanedRefundIntents(context.Background())
		}()
	}
	start.Done()
	done.Wait()

	var attempts int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM failed_refunds WHERE payment_id = $1`, payment.ID,
	).Scan(&attempts); err != nil {
		t.Fatalf("counting queued attempts: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("exactly one refund attempt may be claimed, or the client is refunded twice; got %d", attempts)
	}
}

// TestAPaymentForACancelledBookingCommitsAMarkerTheSweepCanFind is the
// database-level half of change 5's remainder, and it is here rather than
// beside its unit twin because two of the three things it proves cannot be
// proven against a stub.
//
// recordPaymentOwedARefund used to commit refund_status='pending'
// before any claim existed. Between that commit and ClaimRefund's, nothing
// was in flight, but refundable() reads that value as "a claim is already
// committed against this booking's payment" — so a crash or a database
// refusal in that window left a cancelled, paid booking that every later
// refund path declined, with no failed_refunds row for the retry job, no
// marker for the sweep, and no alert. The webhook event's own retry could not
// repair it either: on redelivery the payment now carries the MercadoPago id,
// and processPaymentWebhook returns at "payment already processed, skipping".
//
// What a stub cannot show:
//
//  1. the bookings_refund_intent_only_when_cancelled CHECK
//     accepts this write at all. A marker set on a booking that is not
//     'cancelled' is refused by PostgreSQL, and the whole flow would fail on
//     the write rather than on anything a mock could model.
//  2. GetRefundIntentOrphans' predicate — real SQL against the real column —
//     matches the row this path leaves. The unit test asserts the marker is
//     set; only this one asserts the sweep can see it.
//
// Mutation: restore the old write in recordPaymentOwedARefund —
//
//	booking.RefundStatus = bookingstore.RefundStatusPending   // and drop the marker
//
// — and re-run. Every assertion below fails: the committed row reads a
// pending refund, refund_intent_at is NULL, and the sweep claims nothing.
func TestAPaymentForACancelledBookingCommitsAMarkerTheSweepCanFind(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := context.Background()

	// A booking cancelled while its payment was still in flight: the expiry
	// cron got there first, and MercadoPago's approval is about to arrive.
	booking := &bookingstore.Booking{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Date:            time.Now().AddDate(0, 0, 7),
		StartTime:       "20:00",
		DurationMinutes: 90, Price: 500_000, DepositAmount: 150_000,
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusUnpaid,
		RefundStatus: bookingstore.RefundStatusNone,
	}
	if err := f.Stores.Bookings.Insert(f.Scoped(ctx), booking); err != nil {
		t.Fatalf("creating the cancelled booking: %v", err)
	}
	// Read back rather than reusing the struct Insert filled: this is the shape
	// the webhook has anyway — it always works from a booking it fetched.
	booking, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the cancelled booking back: %v", err)
	}

	mpPaymentID := "mp-" + uuid.NewString()
	mpPayment := &mp.Payment{ID: 123, Status: "approved", TransactionAmount: 1_575.00}

	// Under the tenant bypass, because that is what the MercadoPago webhook
	// runs under: it is in middleware.CrossTenantRoutes by design, since it has
	// to resolve which tenant a payment belongs to before it can be scoped to
	// one. Calling the flow directly skips the middleware, so the test declares
	// the same scope the route would have.
	webhookCtx := data.ContextWithTenantBypass(ctx)
	if _, err := f.service.recordPaymentOwedARefund(webhookCtx, booking, mpPayment, mpPaymentID); err != nil {
		t.Fatalf("recording the payment owed a refund: %v", err)
	}

	// Read the committed row rather than the in-memory struct: the question is
	// what survives the process, not what the flow left on the heap.
	var refundStatus string
	var refundIntentAt *time.Time
	if err := f.DB.QueryRow(ctx,
		`SELECT refund_status, refund_intent_at FROM bookings WHERE id = $1`, booking.ID,
	).Scan(&refundStatus, &refundIntentAt); err != nil {
		t.Fatalf("reading the booking back: %v", err)
	}

	if refundStatus == bookingstore.RefundStatusPending {
		t.Errorf("no claim exists yet, so the committed row must not assert one is in flight — " +
			"refundable() declines on exactly this value")
	}
	if refundIntentAt == nil {
		t.Fatalf("the commit that records the payment must carry the refund-intent marker, or nothing " +
			"looks for this booking again")
	}

	// Age the marker past the grace period, exactly as a real crash would have,
	// and let the sweep do what it exists for.
	if _, err := f.DB.Exec(ctx,
		`UPDATE bookings SET refund_intent_at = NOW() - INTERVAL '10 minutes' WHERE id = $1`, booking.ID,
	); err != nil {
		t.Fatalf("backdating the marker: %v", err)
	}

	f.service.SweepOrphanedRefundIntents(ctx)

	// The fixture's complex has no MercadoPago credential, so the claim commits
	// its attempt row and then refuses at sellerCredential — the attempt is the
	// proof the orphan was found and claimed.
	var attempts int
	if err := f.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM failed_refunds WHERE booking_id = $1`, booking.ID,
	).Scan(&attempts); err != nil {
		t.Fatalf("counting queued attempts: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("the sweep must find and claim this orphan exactly once, or the money stays with nothing "+
			"behind it; got %d queued attempts", attempts)
	}
}
