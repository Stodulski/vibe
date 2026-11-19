package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// A client's cancellation hands the exact slot it freed (complex, court, start,
// end) to the freed-slot listener, which queues the Instagram story.
func TestAPublicCancelHandsTheFreedSlotToTheListener(t *testing.T) {
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
	assertFreedSlot(t, f, complexID, "vibe", booking.CourtID, booking.StartsAt, booking.EndsAt)
}

// The owner cancelling a booking frees its slot the same way.
func TestAnOwnerCancelHandsTheFreedSlotToTheListener(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	complex := &complexstore.Complex{ID: complexID, Name: "Vibe", Slug: "vibe", CancellationHours: 24}
	f.complexes.complex = complex
	f.clients.client = &clientstore.Client{ID: booking.ClientID, Phone: "+5491155551234"}

	if _, err := f.service.Cancel(t.Context(), complex, Actor{}, booking.ID, "lluvia"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	assertFreedSlot(t, f, complexID, "vibe", booking.CourtID, booking.StartsAt, booking.EndsAt)
}

func assertFreedSlot(t *testing.T, f *fixture, complexID uuid.UUID, slug string, courtID uuid.UUID, startsAt, endsAt time.Time) {
	t.Helper()
	if len(f.freedSlots.slots) != 1 {
		t.Fatalf("want the freed slot handed to the listener once; got %d", len(f.freedSlots.slots))
	}
	got := f.freedSlots.slots[0]
	if got.ComplexID != complexID || got.Slug != slug || got.CourtID != courtID || !got.StartsAt.Equal(startsAt) || !got.EndsAt.Equal(endsAt) {
		t.Fatalf("freed slot = %+v; want complex %s slug %s court %s %s-%s", got, complexID, slug, courtID, startsAt, endsAt)
	}
}
