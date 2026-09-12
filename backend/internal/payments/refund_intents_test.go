package payments

import (
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
)

// The sweep never calls MercadoPago for a payment with no MercadoPago id — a
// cash or transfer booking stays routed to the existing owedManually alert,
// not to an automatic provider call (refund-intent-durability spec, "the
// sweep never calls MercadoPago for a payment with no MercadoPago id"). The
// marker is cleared exactly once, by AutoRefundIfPaid's own exit (Phase 15),
// never by the sweep itself.
func TestSweepNeverCallsMercadoPagoForACashBooking(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	now := time.Now()
	booking.RefundIntentAt = &now
	payment.MPPaymentID = nil
	payment.Method = "cash"
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
	f.refundIntents.orphans = []*bookingstore.Booking{booking}

	f.handler.SweepOrphanedRefundIntents(t.Context())

	if len(f.refundIntents.claimed) != 1 {
		t.Fatalf("want the orphan claimed exactly once; got %d", len(f.refundIntents.claimed))
	}
	if len(f.provider.refunds) != 0 {
		t.Error("a cash booking must never reach RefundPayment")
	}
	if len(f.refundIntents.cleared) != 1 {
		t.Errorf("want the marker cleared exactly once (by AutoRefundIfPaid's owedManually exit, not the sweep); got %d", len(f.refundIntents.cleared))
	}
}

// A sweep run that loses the claim race must not touch the row at all: no
// second AutoRefundIfPaid call, and therefore no second refund attempt.
func TestSweepLeavesAnAlreadyClaimedOrphanAlone(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	now := time.Now()
	booking.RefundIntentAt = &now
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")
	f.refundIntents.orphans = []*bookingstore.Booking{booking}
	f.refundIntents.notFoundFor = map[uuid.UUID]bool{booking.ID: true}

	f.handler.SweepOrphanedRefundIntents(t.Context())

	if len(f.payments.claimed) != 0 {
		t.Error("a lost claim race must never reach ClaimRefund")
	}
	if len(f.refundIntents.cleared) != 0 {
		t.Error("a lost claim race must never clear a marker this instance does not own")
	}
}
