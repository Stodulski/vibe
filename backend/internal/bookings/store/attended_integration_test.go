//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// pastBooking inserts a booking with the given status whose hours ended three
// days ago: the span is generated from date, so moving the date moves it.
func pastBooking(t *testing.T, f *datatest.Fixture, status bookingstore.BookingStatus) *bookingstore.Booking {
	t.Helper()
	b := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE bookings SET date = CURRENT_DATE - 3 WHERE id = $1`, b.ID); err != nil {
		t.Fatalf("backdating the booking: %v", err)
	}
	if status != bookingstore.BookingStatusConfirmed {
		f.MarkStatus(t, b.ID, string(status))
	}
	return b
}

func readBooking(t *testing.T, f *datatest.Fixture, id uuid.UUID) *bookingstore.Booking {
	t.Helper()
	b, err := f.Stores.Bookings.GetByID(f.Scoped(context.Background()), id)
	if err != nil {
		t.Fatalf("reading the booking: %v", err)
	}
	return b
}

func TestMarkAttendedCompletesAFinishedBookingOnce(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	b := pastBooking(t, f, bookingstore.BookingStatusConfirmed)

	marked, err := f.Stores.Bookings.MarkAttended(ctx, f.ComplexID, b.ID)
	if err != nil || !marked {
		t.Fatalf("want the finished booking marked; got marked=%v err=%v", marked, err)
	}
	first := readBooking(t, f, b.ID)
	if first.Status != bookingstore.BookingStatusCompleted || first.AttendedAt == nil {
		t.Fatalf("want completed with attended_at; got status %q attended_at %v", first.Status, first.AttendedAt)
	}

	marked, err = f.Stores.Bookings.MarkAttended(ctx, f.ComplexID, b.ID)
	if err != nil || marked {
		t.Fatalf("a second mark must match no row; got marked=%v err=%v", marked, err)
	}
	if again := readBooking(t, f, b.ID); !again.AttendedAt.Equal(*first.AttendedAt) {
		t.Errorf("attended_at must keep the first mark; got %v want %v", again.AttendedAt, first.AttendedAt)
	}
}

func TestMarkAttendedMarksACompletedBookingNobodyMarked(t *testing.T) {
	f := datatest.Isolated(t)
	b := pastBooking(t, f, bookingstore.BookingStatusCompleted)

	marked, err := f.Stores.Bookings.MarkAttended(f.Scoped(context.Background()), f.ComplexID, b.ID)
	if err != nil || !marked {
		t.Fatalf("want the completed booking marked; got marked=%v err=%v", marked, err)
	}
	if got := readBooking(t, f, b.ID); got.AttendedAt == nil {
		t.Errorf("want attended_at set")
	}
}

func TestMarkAttendedLeavesUnfinishedAndClosedBookingsAlone(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	future := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed", StartTime: "10:00", EndTime: "11:30"})
	cancelled := pastBooking(t, f, bookingstore.BookingStatusCancelled)
	noShow := pastBooking(t, f, bookingstore.BookingStatusNoShow)

	for name, b := range map[string]*bookingstore.Booking{"future": future, "cancelled": cancelled, "no_show": noShow} {
		before := readBooking(t, f, b.ID)
		marked, err := f.Stores.Bookings.MarkAttended(ctx, f.ComplexID, b.ID)
		if err != nil || marked {
			t.Errorf("%s: want no row marked; got marked=%v err=%v", name, marked, err)
		}
		if after := readBooking(t, f, b.ID); after.Status != before.Status || after.AttendedAt != nil {
			t.Errorf("%s: want the booking unchanged; got status %q attended_at %v", name, after.Status, after.AttendedAt)
		}
	}
}

func TestMarkAttendedRefusesAnotherComplexsBooking(t *testing.T) {
	f := datatest.Isolated(t)
	b := pastBooking(t, f, bookingstore.BookingStatusConfirmed)

	other := uuid.New()
	marked, err := f.Stores.Bookings.MarkAttended(f.Scoped(context.Background()), other, b.ID)
	if err == nil && marked {
		t.Fatalf("a booking of another complex must not be marked")
	}
	if got := readBooking(t, f, b.ID); got.AttendedAt != nil {
		t.Errorf("want the booking unmarked; got attended_at %v", got.AttendedAt)
	}
}

// The owner's completed -> no_show correction goes through the whole-row write,
// which must drop the attendance mark: bookings_attended_only_completed refuses
// an attended booking that is not completed.
func TestNoShowCorrectionClearsAttendance(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	b := pastBooking(t, f, bookingstore.BookingStatusConfirmed)
	if _, err := f.Stores.Bookings.MarkAttended(ctx, f.ComplexID, b.ID); err != nil {
		t.Fatalf("marking: %v", err)
	}

	if _, err := f.DB.Exec(context.Background(),
		`UPDATE bookings SET status = 'no_show' WHERE id = $1`, b.ID); err == nil {
		t.Fatalf("want the CHECK to refuse a no_show that keeps attended_at")
	}

	snapshot := readBooking(t, f, b.ID)
	snapshot.Status = bookingstore.BookingStatusNoShow
	if err := f.Stores.Bookings.UpdateIfUnchanged(ctx, snapshot); err != nil {
		t.Fatalf("the no_show correction must land: %v", err)
	}
	if got := readBooking(t, f, b.ID); got.Status != bookingstore.BookingStatusNoShow || got.AttendedAt != nil {
		t.Errorf("want no_show with attended_at cleared; got status %q attended_at %v", got.Status, got.AttendedAt)
	}
}
