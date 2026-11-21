package bookings

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// Nobody can have failed to show up for hours that have not started: a
// no-show on a future booking is refused, and the client's counter stays put.
func TestNoShowIsRefusedBeforeTheBookingStarts(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking

	noShow := string(bookingstore.BookingStatusNoShow)
	_, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &noShow})

	var verr *ValidationError
	if !errors.As(err, &verr) || verr.Errors["status"] == "" {
		t.Fatalf("a no-show before the start must be a 422 on status; got %v", err)
	}
	if len(f.store.updated) != 0 {
		t.Error("nothing may be written")
	}
	if len(f.clients.noShows) != 0 {
		t.Errorf("the client's no-show counter moved: %v", f.clients.noShows)
	}
}

// Once the hours have started the owner can mark the absence, and the client's
// counter goes up once.
func TestNoShowAfterTheStartCountsOnce(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.StartsAt = time.Now().Add(-10 * time.Minute)
	booking.EndsAt = booking.StartsAt.Add(90 * time.Minute)
	f.store.booking = booking

	noShow := string(bookingstore.BookingStatusNoShow)
	if _, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &noShow}); err != nil {
		t.Fatalf("a no-show on a started booking must go through; got %v", err)
	}
	if len(f.clients.noShows) != 1 {
		t.Errorf("want the counter moved once; got %d", len(f.clients.noShows))
	}
}

// Saving a booking that already reads no_show (a retried request, or an edit
// of its notes that resends the status) must not count the absence again.
func TestARepeatedNoShowDoesNotCountTwice(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := endedBooking(complexID)
	booking.Status = bookingstore.BookingStatusNoShow
	f.store.booking = booking

	noShow := string(bookingstore.BookingStatusNoShow)
	if _, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &noShow}); err != nil {
		t.Fatalf("resending the same status must not fail; got %v", err)
	}
	if len(f.clients.noShows) != 0 {
		t.Errorf("a booking already absent was counted again: %v", f.clients.noShows)
	}
}
