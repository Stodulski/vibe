package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// startedBooking is a paid booking whose game began ten minutes ago and runs
// for another eighty. Its link is still live: the link outlives the end.
func startedBooking(complexID uuid.UUID) *bookingstore.Booking {
	b := futureBooking(complexID)
	start := time.Now().In(timezone.Argentina).Add(-10 * time.Minute).Truncate(time.Minute)
	b.Date = timezone.Day(start)
	b.StartTime = start.Format("15:04")
	b.StartsAt = start
	b.EndsAt = start.Add(90 * time.Minute)
	b.CreatedAt = start.Add(-48 * time.Hour)
	return b
}

// A player could cancel from their link while the game was being played, and a
// complex with no cancellation window (or a booking still inside its grace
// period) refunded them in full for hours they had used. Once a booking has
// started, the public link no longer cancels it.
func TestPublicCancelRefusesABookingThatHasStarted(t *testing.T) {
	for _, hours := range []int{0, 24} {
		f := newFixture(t)
		complexID := uuid.New()
		booking := startedBooking(complexID)
		f.store.booking = booking
		f.linkResolver.booking = booking
		f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: hours}
		f.clients.client = &clientstore.Client{ID: booking.ClientID}
		f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

		w := httptest.NewRecorder()
		f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

		if w.Code != http.StatusBadRequest {
			t.Errorf("cancellation_hours=%d: a started booking must be refused with 400; got %d (%s)", hours, w.Code, w.Body.String())
		}
		if len(f.store.updated) != 0 {
			t.Errorf("cancellation_hours=%d: a refused cancellation must write nothing; got %d writes", hours, len(f.store.updated))
		}
		if len(f.refunds.refunded) != 0 {
			t.Errorf("cancellation_hours=%d: a started booking must not be refunded; got %v", hours, f.refunds.refunded)
		}
		if booking.Status != "confirmed" {
			t.Errorf("cancellation_hours=%d: the booking must stay confirmed; got %q", hours, booking.Status)
		}
	}
}

// The cancel page reads can_cancel and can_refund from the same rule, so it
// never offers a cancellation the cancel route then refuses.
func TestCancelInfoDoesNotOfferCancellingAStartedBooking(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := startedBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 0}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1", Sport: "padel", CourtType: "indoor"}

	w := httptest.NewRecorder()
	f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet, "/?token=test-token", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["can_cancel"] != false {
		t.Errorf("a started booking must not offer cancelling; got can_cancel=%v", body["can_cancel"])
	}
	if body["can_refund"] != false {
		t.Errorf("a started booking must not promise a refund; got can_refund=%v", body["can_refund"])
	}
}
