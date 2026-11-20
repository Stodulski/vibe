package payments

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// An approved payment the expiry sweep found with no webhook behind it is
// recorded in the inbox and worked like a delivery of it, so the booking is
// confirmed exactly as the lost notification would have confirmed it.
func TestAReconciledPaymentConfirmsTheBookingLikeItsWebhook(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.ExternalReference = booking.ID.String()
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111

	f.provider.payment = mpPayment
	f.bookings.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID, Name: "Vibe", Slug: "vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	if err := f.service.ReconcileApprovedPayment(t.Context(), "mp-123"); err != nil {
		t.Fatalf("ReconcileApprovedPayment: %v", err)
	}

	if len(f.webhookEvents.inserted) != 1 {
		t.Fatalf("the payment must be recorded in the inbox once; got %d rows", len(f.webhookEvents.inserted))
	}
	recorded := f.webhookEvents.inserted[0]
	if recorded.ExternalID != "mp-123" || recorded.EventType != "payment" {
		t.Errorf("the row must be a payment event for mp-123; got %q %q", recorded.EventType, recorded.ExternalID)
	}
	if len(f.webhookEvents.processed) != 1 || f.webhookEvents.processed[0] != recorded.ID {
		t.Errorf("the recorded event must be worked to processed; got %v", f.webhookEvents.processed)
	}
	if booking.Status != "confirmed" || booking.CollectionStatus != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("the booking must be confirmed; got status=%q collection_status=%q", booking.Status, booking.CollectionStatus)
	}
}

// A payment that cannot be recorded is reported, so the sweep keeps the
// booking for its next run instead of expiring it.
func TestAReconciledPaymentThatCannotBeRecordedIsReported(t *testing.T) {
	f := newFixture(t)
	f.webhookEvents.insertErr = errors.New("database is down")

	if err := f.service.ReconcileApprovedPayment(t.Context(), "mp-123"); err == nil {
		t.Fatal("a payment that was not recorded must be reported")
	}
	if f.payments.inserted != nil {
		t.Error("nothing may be processed from an event that was never recorded")
	}
}
