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

// holdWindow is the payment window every store in the integration fixture runs
// on (stores.defaultPaymentExpiry).
const holdWindow = 10 * time.Minute

// takeOnlineHold puts a visitor's live storefront hold on the fixture's court on
// eventDay through the real store, the way POST /book/holds does, and returns
// when it frees up if nobody pays.
func takeOnlineHold(t *testing.T, f *datatest.Fixture, start, end string) time.Time {
	t.Helper()
	b := f.NewBooking(datatest.BookingOptions{
		Public: true, Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid,
		StartTime: start, EndTime: end,
	})
	b.ClientID = uuid.Nil
	b.Date = eventDay()
	_, hash, err := bookingstore.NewHoldToken()
	if err != nil {
		t.Fatalf("NewHoldToken: %v", err)
	}
	if err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), b, hash, "198.51.100.7", 3); err != nil {
		t.Fatalf("InsertHold: %v", err)
	}
	var createdAt time.Time
	if err := f.DB.QueryRow(context.Background(), `SELECT created_at FROM bookings WHERE id = $1`, b.ID).Scan(&createdAt); err != nil {
		t.Fatalf("reading the hold's created_at: %v", err)
	}
	return createdAt.Add(holdWindow)
}

// S3: an event over hours a visitor is booking online right now is refused,
// as an online hold that says when it frees up, and nothing is written.
func TestCreateEventOverALiveOnlineHoldSaysWhenItFrees(t *testing.T) {
	f := datatest.Isolated(t)
	freesAt := takeOnlineHold(t, f, "18:00", "19:30")

	err := f.Stores.Events.CreateEvent(f.Scoped(context.Background()), newEvent(f, "19:00", 120), []uuid.UUID{f.CourtID})
	var conflict *eventstore.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != eventstore.ConflictOnlineHold || conflict.CourtID != f.CourtID {
		t.Fatalf("want an online-hold conflict on the court; got %v", err)
	}
	if !conflict.FreesAt.Equal(freesAt) {
		t.Errorf("frees at %v, want %v", conflict.FreesAt, freesAt)
	}
	var n int
	if err := f.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM open_play_events WHERE complex_id = $1`, f.ComplexID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a refused event must leave no row; %d (%v)", n, err)
	}
}

// S3, previous behavior: a real booking still answers a booking conflict, even
// when an online hold also sits inside the event's hours.
func TestCreateEventOverARealBookingBesideAnOnlineHoldIsABookingConflict(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	takeOnlineHold(t, f, "18:00", "19:00")
	booking := f.NewBooking(datatest.BookingOptions{StartTime: "19:00", EndTime: "20:00"})
	booking.Date = eventDay()
	if err := f.Stores.Bookings.InsertSafe(ctx, booking); err != nil {
		t.Fatal(err)
	}

	requireConflict(t, f.Stores.Events.CreateEvent(ctx, newEvent(f, "18:00", 180), []uuid.UUID{f.CourtID}), eventstore.ConflictBooking)
}
