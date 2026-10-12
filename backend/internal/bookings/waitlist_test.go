package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// A client's cancellation frees the day for the waitlist, once it is saved.
func TestAPublicCancelFreesItsSlotForTheWaitlist(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", Slug: "vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, Phone: "+5491155551234"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.waitlist.freed) != 1 {
		t.Fatalf("want the freed day handed to the waitlist once; got %d", len(f.waitlist.freed))
	}
	got := f.waitlist.freed[0]
	if got.ComplexID != complexID || got.ComplexName != "Vibe" || got.ComplexSlug != "vibe" {
		t.Errorf("freed complex = %s %q %q; want %s Vibe vibe", got.ComplexID, got.ComplexName, got.ComplexSlug, complexID)
	}
	if got.Day.Format("2006-01-02") != booking.Date.Format("2006-01-02") {
		t.Errorf("freed day = %s; want the booking's day %s", got.Day.Format("2006-01-02"), booking.Date.Format("2006-01-02"))
	}
}

// An expired, unpaid booking frees its day too: the expiry sweep is the other
// way a slot goes back on sale.
func TestTheExpirySweepFreesTheSlotForTheWaitlist(t *testing.T) {
	f := newFixture(t)
	candidate := cronCandidate("pending")
	f.store.expiredPending = []*bookingstore.CronBooking{candidate}

	f.service.ReleaseExpiredPayments(t.Context())

	if len(f.waitlist.freed) != 1 {
		t.Fatalf("want the expired day handed to the waitlist once; got %d", len(f.waitlist.freed))
	}
	got := f.waitlist.freed[0]
	if got.ComplexID != candidate.ComplexID || got.ComplexSlug != candidate.ComplexSlug {
		t.Errorf("freed complex = %s %q; want %s %q", got.ComplexID, got.ComplexSlug, candidate.ComplexID, candidate.ComplexSlug)
	}
	if got.Day.Format("2006-01-02") != candidate.Date.Format("2006-01-02") {
		t.Errorf("freed day = %s; want the booking's day %s", got.Day.Format("2006-01-02"), candidate.Date.Format("2006-01-02"))
	}
}
