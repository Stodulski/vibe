//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
)

// takeOnlineHold puts a visitor's live storefront hold on the fixture's court on
// start() through the real store, the way POST /book/holds does, and returns
// when it frees up if nobody pays (the fixture's window is ten minutes).
func takeOnlineHold(t *testing.T, f *fixture, from, to string) time.Time {
	t.Helper()
	b := f.NewBooking(datatest.BookingOptions{
		Public: true, Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid,
		StartTime: from, EndTime: to,
	})
	b.ClientID = uuid.Nil
	b.Date = start()
	_, hash, err := bookingstore.NewHoldToken()
	if err != nil {
		t.Fatalf("NewHoldToken: %v", err)
	}
	if err := f.Stores.Bookings.InsertHold(f.ctx, b, hash, "198.51.100.7", 3); err != nil {
		t.Fatalf("InsertHold: %v", err)
	}
	var createdAt time.Time
	if err := f.DB.QueryRow(context.Background(), `SELECT created_at FROM bookings WHERE id = $1`, b.ID).Scan(&createdAt); err != nil {
		t.Fatalf("reading the hold's created_at: %v", err)
	}
	return createdAt.Add(10 * time.Minute)
}

// S3: scheduling a match over hours a visitor is booking online right now is
// refused as an online hold that says when it frees up.
func TestSchedulingOverALiveOnlineHoldSaysWhenItFrees(t *testing.T) {
	f := newFixture(t)
	_, _, first := drawn(t, f)
	freesAt := takeOnlineHold(t, f, "18:00", "19:30")

	err := f.schedule(first[0], f.CourtID, "19:00")
	var conflict *eventstore.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != eventstore.ConflictOnlineHold {
		t.Fatalf("want an online-hold conflict; got %v", err)
	}
	if !conflict.FreesAt.Equal(freesAt) {
		t.Errorf("frees at %v, want %v", conflict.FreesAt, freesAt)
	}
}

// S3, previous behavior: scheduling over a real booking is a booking conflict.
func TestSchedulingOverARealBookingIsABookingConflict(t *testing.T) {
	f := newFixture(t)
	_, _, first := drawn(t, f)
	booking := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})
	booking.Date = start()
	if err := f.Stores.Bookings.InsertSafe(f.ctx, booking); err != nil {
		t.Fatal(err)
	}

	requireConflict(t, f.schedule(first[0], f.CourtID, "19:00"), eventstore.ConflictBooking)
}
