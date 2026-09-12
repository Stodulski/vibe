//go:build integration

package bookings

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/stores"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// This file proves refund-intent-durability's most important property against
// a real database: the reconciliation sweep must never find a cancellation
// that was deliberately declined, even though its row is byte-for-byte the
// same shape a genuine crash-orphan leaves behind.

// setupIntegrationDB opens a pool against DATABASE_URL, skipping the test
// when it is unset — the same convention every integration suite in this
// repository follows (see internal/data/datatest).
func setupIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// integrationFixture wires a bookings.Handler whose Store and Complexes
// dependencies are the real, data-backed stores — the two this suite needs
// to drive PublicCancel's refund-intent marker write through a real database.
// Everything else is a stub from stubs_test.go: none of it is what
// refund-intent-durability tests.
type integrationFixture struct {
	pool      *pgxpool.Pool
	models    stores.Stores
	handler   *Handler
	complexID uuid.UUID
	courtID   uuid.UUID
	clientID  uuid.UUID
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()

	pool := setupIntegrationDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()

	f := &integrationFixture{pool: pool, models: stores.New(pool, stores.Config{PaymentExpiry: 15 * time.Minute})}

	var ownerID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, $2, 'Owner', 'Test', '+5491100000000', 'owner', true)
		RETURNING id`,
		"owner-"+suffix+"@example.test", []byte("not-a-real-hash"),
	).Scan(&ownerID); err != nil {
		t.Fatalf("creating owner: %v", err)
	}

	// cancellation_hours defaults to 24 (complexes.cancellation_hours), matching the
	// window every unit test in this package already assumes.
	if err := pool.QueryRow(ctx, `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, 'Test Complex', $2, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000001')
		RETURNING id`,
		ownerID, "test-complex-"+suffix,
	).Scan(&f.complexID); err != nil {
		t.Fatalf("creating complex: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM failed_refunds WHERE complex_id = $1`, f.complexID)
		_, _ = pool.Exec(ctx, `DELETE FROM payments WHERE complex_id = $1`, f.complexID)
		_, _ = pool.Exec(ctx, `DELETE FROM bookings WHERE complex_id = $1`, f.complexID)
		_, _ = pool.Exec(ctx, `DELETE FROM clients WHERE complex_id = $1`, f.complexID)
		_, _ = pool.Exec(ctx, `DELETE FROM courts WHERE complex_id = $1`, f.complexID)
		_, _ = pool.Exec(ctx, `DELETE FROM complexes WHERE id = $1`, f.complexID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, ownerID)
	})

	if err := pool.QueryRow(ctx, `
		INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 1') RETURNING id`,
		f.complexID,
	).Scan(&f.courtID); err != nil {
		t.Fatalf("creating court: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO clients (complex_id, first_name, last_name, phone) VALUES ($1, 'Ana', 'Diaz', $2) RETURNING id`,
		f.complexID, "+54911"+suffix[:8],
	).Scan(&f.clientID); err != nil {
		t.Fatalf("creating client: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(&discard{}, nil))
	svc := NewService(Dependencies{
		Store:     f.models.Bookings,
		Clients:   &stubClients{},
		Complexes: f.models.Complexes,
		Courts:    &stubCourts{},
		Payments:  &stubPayments{},
		Locks:     &stubLocks{},
		Checkout:  &stubCheckout{},
		Refunds:   &stubRefunder{},
		// f.models.BookingLinkTokens is the real, data-backed store — this
		// suite exercises PublicCancel through ResolveLink, and a minted
		// token has to actually resolve against it.
		LinkResolver: f.models.BookingLinkTokens,
		LinkTokens:   f.models.BookingLinkTokens,
		Notify:       &stubNotifier{},
		Realtime:     &stubBroadcaster{},
		Audit:        &stubRecorder{},
		Logger:       logger,
		Run:          func(fn func()) { fn() },
	}, Config{
		FrontendURL: "https://vibe.test", BackendURL: "https://api.vibe.test",
		Environment: "test", GracePeriod: 15 * time.Minute,
		PaymentExpiry: 15 * time.Minute, SlotLockTTL: 15 * time.Minute,
	})
	f.handler = NewHandler(svc, &stubWhatsApp{}, httpx.NewResponder(logger), logger, false)
	return f
}

// discard is a minimal io.Writer that keeps test logs out of the way,
// avoiding an extra "io" import for the one call site above.
type discard struct{}

func (*discard) Write(p []byte) (int, error) { return len(p), nil }

// createOutOfWindowBooking inserts a confirmed, deposit-paid booking whose
// start time is two hours out against the complex's 24-hour window, created
// long enough ago that both the window and the grace period have passed —
// the real-database twin of cancel_test.go's outOfWindowBooking.
func (f *integrationFixture) createOutOfWindowBooking(t *testing.T) *bookingstore.Booking {
	t.Helper()
	ctx := context.Background()

	// The start is an instant, and the date comes from that same instant rather
	// than from now — otherwise a game two hours out on a late evening is filed
	// under today with a start time belonging to tomorrow.
	//
	// And the fixture keeps its hours inside one local day. A booking crossing
	// midnight is perfectly legal now, and with no end_time column there is
	// no stored end to disagree with the span — but every assertion here reads
	// the booking's date and start time as one pair, so when the 90-minute slot
	// would run past 00:00 the fixture moves to the next morning rather than
	// filing a start under a date the caller then has to reason about. That
	// branch only fires after roughly 22:30 local, and 08:00 the next day is
	// still well inside the complex's 24-hour window, which is the only
	// property this fixture actually needs.
	//
	// Before this, the pair was built with now.Add(...).Format("15:04") and the
	// date left at now, so the test failed for about ninety minutes every
	// evening and passed the rest of the day.
	now := time.Now().In(timezone.Argentina)
	start := now.Add(2 * time.Hour)
	if end := start.Add(90 * time.Minute); end.Day() != start.Day() {
		start = time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, timezone.Argentina).AddDate(0, 0, 1)
	}

	b := &bookingstore.Booking{
		ComplexID: f.complexID, CourtID: f.courtID, ClientID: f.clientID,
		Date:            start,
		StartTime:       start.Format("15:04"),
		DurationMinutes: 90, Price: 500_000, DepositAmount: 150_000,
		Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		RefundStatus: bookingstore.RefundStatusNone,
	}
	if err := f.models.Bookings.Insert(ctx, b); err != nil {
		t.Fatalf("creating booking: %v", err)
	}

	if _, err := f.pool.Exec(ctx,
		`UPDATE bookings SET created_at = NOW() - INTERVAL '24 hours' WHERE id = $1`, b.ID,
	); err != nil {
		t.Fatalf("backdating booking: %v", err)
	}
	return b
}

func (f *integrationFixture) readBookingState(t *testing.T, id uuid.UUID) (status, collectionStatus, refundStatus string) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status, collection_status, refund_status FROM bookings WHERE id = $1`, id,
	).Scan(&status, &collectionStatus, &refundStatus); err != nil {
		t.Fatalf("reading booking %s: %v", id, err)
	}
	return status, collectionStatus, refundStatus
}

func (f *integrationFixture) hasFailedRefundRow(t *testing.T, bookingID uuid.UUID) bool {
	t.Helper()
	var exists bool
	if err := f.pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM failed_refunds WHERE booking_id = $1)`, bookingID,
	).Scan(&exists); err != nil {
		t.Fatalf("checking failed_refunds for booking %s: %v", bookingID, err)
	}
	return exists
}

// The false-positive proof (design.md's "single most important deliverable",
// tasks.md Phase 17): an out-of-window public cancellation of a paid booking
// produces the exact row shape a genuine crash-orphan produces — status
// 'cancelled', collection_status still 'deposit_paid' with no refund under
// way, no failed_refunds row —
// but it must never carry the refund-intent marker that makes a row eligible
// for the reconciliation sweep.
//
// Mutation, run and recorded: add `OR (status='cancelled' AND
// collection_status IN ('deposit_paid','fully_paid'))` to
// GetRefundIntentOrphans' WHERE clause
// — the query starts returning this exact row. Removed after recording.
func TestAnOutOfWindowPublicCancelIsNeverFoundByTheSweep(t *testing.T) {
	f := newIntegrationFixture(t)
	booking := f.createOutOfWindowBooking(t)

	// Insert (the test-only variant used above) mints no token, so this
	// suite mints its own — the real-database twin of the stub fixtures'
	// linkResolver.booking assignment.
	plaintext, err := f.models.BookingLinkTokens.Mint(t.Context(), booking.ID, booking.EndsAt.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("minting a token for the out-of-window booking: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/",
		strings.NewReader(`{"token":"`+plaintext+`"}`))
	f.handler.PublicCancel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	status, collectionStatus, refundStatus := f.readBookingState(t, booking.ID)
	if status != "cancelled" || collectionStatus != bookingstore.CollectionStatusDepositPaid ||
		refundStatus != bookingstore.RefundStatusNone {
		t.Fatalf("want the orphan's exact row shape (cancelled/deposit_paid/none); got %s/%s/%s",
			status, collectionStatus, refundStatus)
	}
	if f.hasFailedRefundRow(t, booking.ID) {
		t.Fatal("an out-of-window cancellation must never write a failed_refunds row")
	}

	orphans, err := f.models.Bookings.GetRefundIntentOrphans(t.Context(), 0, 50)
	if err != nil {
		t.Fatalf("GetRefundIntentOrphans: %v", err)
	}
	for _, o := range orphans {
		if o.ID == booking.ID {
			t.Fatal("a deliberately declined cancellation must never be found by the reconciliation sweep — " +
				"this is money the venue is entitled to keep, and the sweep would pay it out")
		}
	}
}
