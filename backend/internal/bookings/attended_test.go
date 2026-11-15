package bookings

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// endedBooking is a confirmed booking whose hours ended an hour ago, which the
// completion sweep has not reached yet.
func endedBooking(complexID uuid.UUID) *bookingstore.Booking {
	b := futureBooking(complexID)
	b.EndsAt = time.Now().Add(-time.Hour)
	b.StartsAt = b.EndsAt.Add(-90 * time.Minute)
	return b
}

func markAttended(t *testing.T, f *fixture, complexID uuid.UUID, b *bookingstore.Booking) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.MarkAttended(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": b.ID.String()}, ""))
	return w
}

func TestMarkAttendedCompletesAConfirmedBookingWhoseHoursAreOver(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := endedBooking(complexID)
	f.store.booking = booking

	w := markAttended(t, f, complexID, booking)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if booking.Status != bookingstore.BookingStatusCompleted || booking.AttendedAt == nil {
		t.Fatalf("want the booking completed and attended; got status %q attended_at %v", booking.Status, booking.AttendedAt)
	}
	got, _ := decode(t, w)["booking"].(map[string]any)
	if got["status"] != "completed" || got["attended_at"] == nil {
		t.Errorf("want the envelope's booking completed with attended_at; got %v", got)
	}
	if len(f.realtime.published) != 1 || f.realtime.published[0] != complexID {
		t.Errorf("want one booking-changed broadcast for the complex; got %v", f.realtime.published)
	}
}

func TestMarkAttendedMarksACompletedBookingNobodyMarked(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := endedBooking(complexID)
	booking.Status = bookingstore.BookingStatusCompleted
	f.store.booking = booking

	w := markAttended(t, f, complexID, booking)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if booking.AttendedAt == nil {
		t.Errorf("want attended_at set on the completed booking")
	}
}

func TestMarkAttendedRefusesABookingThatHasNotFinished(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking

	w := markAttended(t, f, complexID, booking)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if bookingFieldError(decode(t, w), "status") != "the booking has not finished yet" {
		t.Errorf("want the status field error; got %s", w.Body.String())
	}
	if booking.Status != bookingstore.BookingStatusConfirmed || booking.AttendedAt != nil {
		t.Errorf("the booking must not change; got status %q attended_at %v", booking.Status, booking.AttendedAt)
	}
	if len(f.realtime.published) != 0 {
		t.Errorf("a refusal must not broadcast; got %v", f.realtime.published)
	}
}

func TestMarkAttendedRefusesCancelledNoShowAndPendingBookings(t *testing.T) {
	for _, status := range []bookingstore.BookingStatus{
		bookingstore.BookingStatusCancelled, bookingstore.BookingStatusNoShow, bookingstore.BookingStatusPending,
	} {
		t.Run(string(status), func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := endedBooking(complexID)
			booking.Status = status
			f.store.booking = booking

			w := markAttended(t, f, complexID, booking)

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if bookingFieldError(decode(t, w), "status") == "" {
				t.Errorf("want a status field error; got %s", w.Body.String())
			}
			if booking.Status != status || booking.AttendedAt != nil {
				t.Errorf("the booking must not change; got status %q attended_at %v", booking.Status, booking.AttendedAt)
			}
		})
	}
}

func TestMarkAttendedIsIdempotentOnAnAttendedBooking(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := endedBooking(complexID)
	booking.Status = bookingstore.BookingStatusCompleted
	attendedAt := time.Now().Add(-30 * time.Minute)
	booking.AttendedAt = &attendedAt
	f.store.booking = booking
	f.store.markAttendedErr = errors.New("an attended booking must not be written again")

	w := markAttended(t, f, complexID, booking)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if !booking.AttendedAt.Equal(attendedAt) {
		t.Errorf("attended_at must keep the first mark; got %v", booking.AttendedAt)
	}
	if len(f.realtime.published) != 0 {
		t.Errorf("nothing changed, so nothing is broadcast; got %v", f.realtime.published)
	}
}

func TestMarkAttendedAnswersNotFoundForAnotherComplexsBooking(t *testing.T) {
	f := newFixture(t)
	booking := endedBooking(uuid.New())
	f.store.booking = booking

	w := markAttended(t, f, uuid.New(), booking)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

// The owner's completed -> no_show correction must drop the attendance mark:
// the database refuses an attended booking that is not completed.
func TestUpdateToNoShowClearsAttendance(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := endedBooking(complexID)
	booking.Status = bookingstore.BookingStatusCompleted
	attendedAt := time.Now()
	booking.AttendedAt = &attendedAt
	f.store.booking = booking

	noShow := string(bookingstore.BookingStatusNoShow)
	got, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &noShow})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Status != bookingstore.BookingStatusNoShow || got.AttendedAt != nil {
		t.Errorf("want no_show with attended_at cleared; got status %q attended_at %v", got.Status, got.AttendedAt)
	}
}
