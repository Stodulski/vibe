//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// Converting a hold used to clear the address it was taken from, so it fell
// out of the per-address cap the moment the visitor typed any name: one
// script could take a hold, convert it with made-up details, and take the next,
// holding a venue's evening for the whole payment window. A converted booking
// still waiting for its payment counts against the address that took it.
func TestAConvertedUnpaidHoldStillCountsAgainstItsAddress(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	first := newHold(f, "10:00", "11:30")
	_, hash, _ := bookingstore.NewHoldToken()
	if err := f.Stores.Bookings.InsertHold(ctx, first, hash, holdIP, 1); err != nil {
		t.Fatalf("InsertHold: %v", err)
	}
	withContact(first)
	if err := f.Stores.Bookings.ConvertHold(ctx, first, hash); err != nil {
		t.Fatalf("ConvertHold: %v", err)
	}

	_, next, _ := bookingstore.NewHoldToken()
	err := f.Stores.Bookings.InsertHold(ctx, newHold(f, "12:00", "13:30"), next, holdIP, 1)
	if !errors.Is(err, bookingstore.ErrHoldLimit) {
		t.Fatalf("a converted, still unpaid booking must count against its address; got %v", err)
	}

	// Once its payment window is over it holds nothing, and counts for nothing.
	f.BackdateBookingCreatedAt(t, first.ID, time.Hour)
	if err := f.Stores.Bookings.InsertHold(ctx, newHold(f, "12:00", "13:30"), next, holdIP, 1); err != nil {
		t.Errorf("an expired booking must not count; got %v", err)
	}
}

// The pre-hold POST /book path (no hold token) inserts with the address and
// is held to the same cap: before, it had none at all.
func TestAStorefrontBookingWithoutAHoldIsCappedPerAddress(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	first := newHold(f, "10:00", "11:30")
	withContact(first)
	if err := f.Stores.Bookings.InsertHold(ctx, first, nil, holdIP, 1); err != nil {
		t.Fatalf("a first storefront booking must go in: %v", err)
	}
	if first.LinkToken == "" {
		t.Error("a storefront booking must carry its link token, as InsertSafe mints it")
	}

	second := newHold(f, "12:00", "13:30")
	withContact(second)
	err := f.Stores.Bookings.InsertHold(ctx, second, nil, holdIP, 1)
	if !errors.Is(err, bookingstore.ErrHoldLimit) {
		t.Fatalf("a second unpaid storefront booking from the same address must be refused; got %v", err)
	}

	other := newHold(f, "12:00", "13:30")
	withContact(other)
	if err := f.Stores.Bookings.InsertHold(ctx, other, nil, "203.0.113.50", 1); err != nil {
		t.Errorf("another address is not capped by this one; got %v", err)
	}
}

// A cap of 0 enforces none: the isolated E2E stack runs with the rate limiter
// off, every browser behind the one loopback address, and its specs leave
// unpaid storefront bookings behind from test to test.
func TestACapOfZeroLetsOneAddressHoldAnyNumber(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	for _, span := range [][2]string{{"10:00", "11:30"}, {"12:00", "13:30"}, {"14:00", "15:30"}} {
		b := newHold(f, span[0], span[1])
		withContact(b)
		if err := f.Stores.Bookings.InsertHold(ctx, b, nil, holdIP, 0); err != nil {
			t.Fatalf("uncapped, the booking at %s must go in; got %v", span[0], err)
		}
	}
}

// A booking that was paid names its client and is nobody's hold any more: it
// stops counting, and the address it came from is not kept on it.
func TestAPaidBookingNoLongerCountsOrKeepsItsAddress(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	paid := newHold(f, "10:00", "11:30")
	withContact(paid)
	if err := f.Stores.Bookings.InsertHold(ctx, paid, nil, holdIP, 1); err != nil {
		t.Fatalf("InsertHold: %v", err)
	}
	attachSnapshot(t, f, paid)

	var ip *string
	if err := f.DB.QueryRow(context.Background(), `SELECT hold_ip FROM bookings WHERE id = $1`, paid.ID).Scan(&ip); err != nil {
		t.Fatalf("reading hold_ip: %v", err)
	}
	if ip != nil {
		t.Errorf("a paid booking must not keep the address it was taken from; got %q", *ip)
	}
	next := newHold(f, "12:00", "13:30")
	withContact(next)
	if err := f.Stores.Bookings.InsertHold(ctx, next, nil, holdIP, 1); err != nil {
		t.Errorf("a paid booking must not count against its address; got %v", err)
	}
}
