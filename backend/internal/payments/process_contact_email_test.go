package payments

import (
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// A returning client who typed a different email on the storefront is
// confirmed at that email, not at the one their client row keeps; a booking
// with no typed email falls back to the client's.
func TestTheConfirmationGoesToTheEmailTypedForTheBooking(t *testing.T) {
	for _, tc := range []struct {
		name, typed, want string
	}{
		{name: "typed for this booking", typed: "new@example.com", want: "new@example.com"},
		{name: "none typed", typed: "", want: "old@example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, mpPayment := pendingBooking(complexID)
			booking.ContactEmail = tc.typed
			f.bookings.booking = booking
			mpPayment.CollectorID = 111111111
			sellerID := "111111111"
			oldEmail := "old@example.com"
			f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID, Name: "Vibe", Slug: "vibe"}
			f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz", Email: &oldEmail}
			f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

			if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
				t.Fatalf("processApprovedPayment: %v", err)
			}
			if len(f.notify.confirmations) != 1 {
				t.Fatalf("want one confirmation; got %d", len(f.notify.confirmations))
			}
			if got := f.notify.confirmations[0].Email; got != tc.want {
				t.Errorf("want the confirmation sent to %q; got %q", tc.want, got)
			}
		})
	}
}
