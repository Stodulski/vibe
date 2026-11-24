//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// Migration 049: leaving the confirm page asks for the hold's release, which
// the sweep makes once the grace has passed, unless the visitor reclaims it.

const leftGrace = time.Minute

// releaseRequestedAt reads a booking's hold_release_requested_at.
func releaseRequestedAt(t *testing.T, f *datatest.Fixture, id uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	if err := f.DB.QueryRow(context.Background(),
		`SELECT hold_release_requested_at FROM bookings WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatalf("reading hold_release_requested_at: %v", err)
	}
	return at
}

// backdateReleaseRequest moves a hold's release request by ago into the past.
func backdateReleaseRequest(t *testing.T, f *datatest.Fixture, id uuid.UUID, ago time.Duration) {
	t.Helper()
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE bookings SET hold_release_requested_at = NOW() - make_interval(secs => $2) WHERE id = $1`,
		id, ago.Seconds()); err != nil {
		t.Fatalf("backdating the release request: %v", err)
	}
}

// releaseLeft runs the sweep and returns what it freed for this fixture's complex.
func releaseLeft(t *testing.T, f *datatest.Fixture) []bookingstore.ReleasedHold {
	t.Helper()
	freed, err := f.Stores.Bookings.ReleaseLeftHolds(context.Background(), leftGrace)
	if err != nil {
		t.Fatalf("ReleaseLeftHolds: %v", err)
	}
	var mine []bookingstore.ReleasedHold
	for _, h := range freed {
		if h.ComplexID == f.ComplexID {
			mine = append(mine, h)
		}
	}
	return mine
}

func TestALeftHoldStaysHeldWithinTheGrace(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)

	if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
		t.Fatalf("RequestHoldRelease: %v", err)
	}
	if releaseRequestedAt(t, f, hold.ID) == nil {
		t.Fatal("leaving must record when the visitor left")
	}

	if freed := releaseLeft(t, f); len(freed) != 0 {
		t.Errorf("a hold left less than the grace ago must stay; freed %+v", freed)
	}
	if status, _, _ := f.ReadBookingState(t, hold.ID); status != "pending" {
		t.Errorf("want the hold still pending; got %q", status)
	}
	if !contains(bookedStarts(t, f, hold.Date), "18:00") {
		t.Error("a hold within its grace must still hold its slot")
	}
}

func TestALeftHoldIsReleasedAfterTheGrace(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
		t.Fatalf("RequestHoldRelease: %v", err)
	}
	backdateReleaseRequest(t, f, hold.ID, leftGrace+time.Second)

	freed := releaseLeft(t, f)
	if len(freed) != 1 || freed[0].Day.Format(time.DateOnly) != hold.Date.Format(time.DateOnly) || freed[0].ComplexSlug == "" {
		t.Fatalf("want the hold's venue and day reported once; got %+v", freed)
	}
	if status, _, _ := f.ReadBookingState(t, hold.ID); status != "cancelled" {
		t.Errorf("a released left hold must be cancelled, as ReleaseHold leaves it; got %q", status)
	}
	if contains(bookedStarts(t, f, hold.Date), "18:00") {
		t.Error("a released left hold must free its slot")
	}
	if _, err := f.Stores.Bookings.ReclaimHold(context.Background(), hash); !errors.Is(err, bookingstore.ErrHoldNotFound) {
		t.Errorf("a released hold cannot be reclaimed; got %v", err)
	}
	if again := releaseLeft(t, f); len(again) != 0 {
		t.Errorf("a second sweep must free nothing; got %+v", again)
	}
}

func TestReclaimingCallsTheReleaseOffAndKeepsTheWindow(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	before := readCreatedAt(t, f, hold.ID)
	if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
		t.Fatalf("RequestHoldRelease: %v", err)
	}
	backdateReleaseRequest(t, f, hold.ID, 30*time.Second)

	createdAt, err := f.Stores.Bookings.ReclaimHold(context.Background(), hash)
	if err != nil {
		t.Fatalf("ReclaimHold: %v", err)
	}
	if !createdAt.Equal(before) || !readCreatedAt(t, f, hold.ID).Equal(before) {
		t.Errorf("a reclaim must never move the window: created_at %v before, %v answered", before, createdAt)
	}
	if releaseRequestedAt(t, f, hold.ID) != nil {
		t.Error("a reclaim must clear the release request")
	}
	time.Sleep(10 * time.Millisecond)
	if freed, err := f.Stores.Bookings.ReleaseLeftHolds(context.Background(), 0); err != nil {
		t.Fatalf("ReleaseLeftHolds: %v", err)
	} else {
		for _, h := range freed {
			if h.ComplexID == f.ComplexID {
				t.Fatalf("a reclaimed hold must not be released; freed %+v", h)
			}
		}
	}
	if status, _, _ := f.ReadBookingState(t, hold.ID); status != "pending" {
		t.Errorf("want the reclaimed hold pending; got %q", status)
	}
}

func TestLeavingAgainNeverPushesTheReleaseLater(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
		t.Fatalf("RequestHoldRelease: %v", err)
	}
	backdateReleaseRequest(t, f, hold.ID, 45*time.Second)
	first := releaseRequestedAt(t, f, hold.ID)

	if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
		t.Fatalf("RequestHoldRelease again: %v", err)
	}
	if again := releaseRequestedAt(t, f, hold.ID); again == nil || !again.Equal(*first) {
		t.Errorf("a second leave must keep the first instant %v; got %v", first, again)
	}
}

func TestConvertingAHoldWithAPendingReleaseKeepsIt(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
		t.Fatalf("RequestHoldRelease: %v", err)
	}
	backdateReleaseRequest(t, f, hold.ID, 30*time.Second)

	withContact(hold)
	if err := f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), hold, hash); err != nil {
		t.Fatalf("a hold with a pending release must still convert: %v", err)
	}
	if releaseRequestedAt(t, f, hold.ID) != nil {
		t.Error("converting must call the pending release off")
	}
	backdateReleaseRequest(t, f, hold.ID, leftGrace+time.Second) // even were it still set
	if freed := releaseLeft(t, f); len(freed) != 0 {
		t.Errorf("a converted booking is never released by the sweep; freed %+v", freed)
	}
	if status, _, _ := f.ReadBookingState(t, hold.ID); status != "pending" {
		t.Errorf("want the converted booking still pending its payment; got %q", status)
	}
}

func TestAnExpiredHoldCannotBeReclaimedOrLeft(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	f.BackdateBookingCreatedAt(t, hold.ID, time.Hour)

	if _, err := f.Stores.Bookings.ReclaimHold(context.Background(), hash); !errors.Is(err, bookingstore.ErrHoldNotFound) {
		t.Errorf("a hold past its window cannot be reclaimed; got %v", err)
	}
	if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
		t.Fatalf("RequestHoldRelease: %v", err)
	}
	if releaseRequestedAt(t, f, hold.ID) != nil {
		t.Error("leaving a hold past its window records nothing")
	}
	if _, err := f.Stores.Bookings.ReclaimHold(context.Background(), []byte("unknown")); !errors.Is(err, bookingstore.ErrHoldNotFound) {
		t.Errorf("an unknown token cannot be reclaimed; got %v", err)
	}
}
