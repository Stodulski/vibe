//go:build integration

package bookings

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// This file proves refund-intent-durability's most important property against
// a real database: the reconciliation sweep must never find a cancellation
// that was deliberately declined, even though its row is byte-for-byte the
// same shape a genuine crash-orphan leaves behind.

// integrationFixture wires a bookings.Handler whose Store and Complexes
// dependencies are the real, data-backed stores — the two this suite needs
// to drive PublicCancel's refund-intent marker write through a real database.
// Everything else is a stub from stubs_test.go: none of it is what
// refund-intent-durability tests.
//
// It embeds datatest.Fixture rather than opening its own pool: the handler is
// called directly (no httptest.Server, no second goroutine reaching the
// database), so every write it makes goes through the same connection and
// fits inside the fixture's one rolled-back transaction.
type integrationFixture struct {
	*datatest.Fixture
	handler *Handler
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()

	f := &integrationFixture{Fixture: datatest.Isolated(t)}

	logger := slog.New(slog.NewTextHandler(&discard{}, nil))
	svc := NewService(Dependencies{
		Facade:    NewFacade(f.Stores.Bookings),
		Store:     f.Stores.Bookings,
		Clients:   &stubClients{},
		Complexes: f.Stores.Complexes,
		Courts:    &stubCourts{},
		Payments:  &stubPayments{},
		Locks:     &stubLocks{},
		Checkout:  &stubCheckout{},
		Refunds:   &stubRefunder{},
		// f.Stores.BookingLinkTokens is the real, data-backed store — this
		// suite exercises PublicCancel through ResolveLink, and a minted
		// token has to actually resolve against it.
		LinkResolver: f.Stores.BookingLinkTokens,
		LinkTokens:   f.Stores.BookingLinkTokens,
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
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Date:            start,
		StartTime:       start.Format("15:04"),
		DurationMinutes: 90, Price: 500_000, DepositAmount: 150_000,
		Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		RefundStatus: bookingstore.RefundStatusNone,
	}
	if err := f.Stores.Bookings.Insert(f.Scoped(ctx), b); err != nil {
		t.Fatalf("creating booking: %v", err)
	}

	if _, err := f.DB.Exec(ctx,
		`UPDATE bookings SET created_at = NOW() - INTERVAL '24 hours' WHERE id = $1`, b.ID,
	); err != nil {
		t.Fatalf("backdating booking: %v", err)
	}
	return b
}

func (f *integrationFixture) readBookingState(t *testing.T, id uuid.UUID) (status, collectionStatus, refundStatus string) {
	t.Helper()
	if err := f.DB.QueryRow(context.Background(),
		`SELECT status, collection_status, refund_status FROM bookings WHERE id = $1`, id,
	).Scan(&status, &collectionStatus, &refundStatus); err != nil {
		t.Fatalf("reading booking %s: %v", id, err)
	}
	return status, collectionStatus, refundStatus
}

func (f *integrationFixture) hasFailedRefundRow(t *testing.T, bookingID uuid.UUID) bool {
	t.Helper()
	var exists bool
	if err := f.DB.QueryRow(context.Background(),
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
	plaintext, err := f.Stores.BookingLinkTokens.Mint(t.Context(), booking.ID, booking.EndsAt.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("minting a token for the out-of-window booking: %v", err)
	}

	w := httptest.NewRecorder()
	// The tenant bypass, because the public cancel route runs under it: it is in
	// middleware.CrossTenantRoutes by design, resolving its booking from a token
	// hash before any tenant is known. Calling the handler directly skips the
	// middleware, so the test declares the same scope the route would have.
	req := httptest.NewRequestWithContext(data.ContextWithTenantBypass(t.Context()), http.MethodPost, "/",
		strings.NewReader(`{"token":"`+plaintext+`"}`))
	req.Header.Set("Content-Type", "application/json")
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

	orphans, err := f.Stores.Bookings.GetRefundIntentOrphans(t.Context(), 0, 50)
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
