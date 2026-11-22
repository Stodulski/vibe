//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/data/slotguard"
)

// holdWindow is the payment window every store in the integration fixture runs
// on (stores.defaultPaymentExpiry).
const holdWindow = 10 * time.Minute

// takeOnlineHold puts a visitor's live storefront hold on the fixture's court
// through the real store, the way POST /book/holds does, and returns when it
// frees up if nobody pays.
func takeOnlineHold(t *testing.T, f *datatest.Fixture, date time.Time, start, end string) time.Time {
	t.Helper()
	b := f.NewBooking(datatest.BookingOptions{
		Public: true, Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid,
		StartTime: start, EndTime: end,
	})
	b.ClientID = uuid.Nil
	b.Date = date
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

// S3: a block over hours a visitor is booking online right now is refused as
// before (ErrSlotHasBooking), and the refusal says when the hold frees up.
func TestBlockOverALiveOnlineHoldIsToldWhenItFrees(t *testing.T) {
	f := datatest.Isolated(t)
	date := bookedDay(3)
	freesAt := takeOnlineHold(t, f, date, "18:00", "19:30")

	err := f.Stores.Courts.InsertBlockedSlot(f.Scoped(context.Background()), &courtstore.BlockedSlot{
		CourtID: f.CourtID, Date: date, StartTime: "18:30", EndTime: "19:30", CreatedBy: &f.UserID,
	})
	if !errors.Is(err, courtstore.ErrSlotHasBooking) {
		t.Fatalf("want ErrSlotHasBooking; got %v", err)
	}
	var held *slotguard.OnlineHoldError
	if !errors.As(err, &held) || !held.FreesAt.Equal(freesAt) {
		t.Fatalf("want an OnlineHoldError freeing at %v; got %v", freesAt, err)
	}
	if n := countBlocks(t, f); n != 0 {
		t.Errorf("the refused block must leave no row behind; found %d", n)
	}
}

// S3, previous behavior: a block over a real booking is the plain
// ErrSlotHasBooking it always was, even when an online hold sits next to it
// inside the same block.
func TestBlockOverARealBookingBesideAnOnlineHoldIsAPlainRefusal(t *testing.T) {
	f := datatest.Isolated(t)
	date := bookedDay(4)
	insertLiveBooking(t, f, date, "18:00", 60)
	takeOnlineHold(t, f, date, "19:00", "20:00")

	err := f.Stores.Courts.InsertBlockedSlot(f.Scoped(context.Background()), &courtstore.BlockedSlot{
		CourtID: f.CourtID, Date: date, StartTime: "18:30", EndTime: "19:30", CreatedBy: &f.UserID,
	})
	if !errors.Is(err, courtstore.ErrSlotHasBooking) {
		t.Fatalf("want ErrSlotHasBooking; got %v", err)
	}
	var held *slotguard.OnlineHoldError
	if errors.As(err, &held) {
		t.Fatalf("a real booking must not be reported as an online hold; got %v", err)
	}
}
