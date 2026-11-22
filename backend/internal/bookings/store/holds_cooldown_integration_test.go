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

// window is the payment window every store in the integration fixture runs on
// (stores.defaultPaymentExpiry).
const window = 10 * time.Minute

// readCreatedAt is the created_at the database holds for a booking: the
// instant every "is this hold still live" predicate counts its window from.
func readCreatedAt(t *testing.T, f *datatest.Fixture, id uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := f.DB.QueryRow(context.Background(), `SELECT created_at FROM bookings WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatalf("reading created_at of %s: %v", id, err)
	}
	return at
}

func takeHoldFrom(f *datatest.Fixture, b *bookingstore.Booking, ip string) error {
	_, hash, err := bookingstore.NewHoldToken()
	if err != nil {
		return err
	}
	return f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), b, hash, ip, 3)
}

// S2: a hold that ran out unconverted cannot be taken again by the same
// address for HoldCooldown, whichever sweep (or none yet) noticed it ran out.
func TestALapsedHoldCannotBeRetakenByTheSameAddressDuringTheCooldown(t *testing.T) {
	for _, swept := range []bool{false, true} {
		name := "before the sweep ran"
		if swept {
			name = "after the sweep ran"
		}
		t.Run(name, func(t *testing.T) {
			f := datatest.Isolated(t)
			hold := newHold(f, "18:00", "19:30")
			takeHold(t, f, hold)
			f.BackdateBookingCreatedAt(t, hold.ID, window+time.Minute)
			lapsedAt := readCreatedAt(t, f, hold.ID).Add(window)
			if swept {
				if _, err := f.Stores.Bookings.DeleteExpiredHolds(f.Scoped(context.Background()), window); err != nil {
					t.Fatalf("DeleteExpiredHolds: %v", err)
				}
			}

			// Overlapping hours on the same court, same address: refused, and told when.
			err := takeHoldFrom(f, newHold(f, "18:30", "20:00"), holdIP)
			var cooldown *bookingstore.HoldCooldownError
			if !errors.As(err, &cooldown) {
				t.Fatalf("want HoldCooldownError; got %v", err)
			}
			if want := lapsedAt.Add(bookingstore.HoldCooldown); cooldown.Until.Sub(want).Abs() > time.Second {
				t.Errorf("Until = %v, want the lapse plus the cooldown (%v)", cooldown.Until, want)
			}

			// Other hours on the same court, from the same address: a fresh hold.
			if err := takeHoldFrom(f, newHold(f, "20:00", "21:30"), holdIP); err != nil {
				t.Errorf("hours the lapsed hold did not cover must not be refused: %v", err)
			}
			// The same hours from another address: unaffected.
			if err := takeHoldFrom(f, newHold(f, "18:00", "19:30"), "203.0.113.77"); err != nil {
				t.Errorf("another address must not inherit the cooldown: %v", err)
			}
		})
	}
}

// S2: the cooldown ends. Once it has passed the same address holds the same
// hours afresh, with a full window of its own.
func TestTheCooldownEnds(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	takeHold(t, f, hold)
	f.BackdateBookingCreatedAt(t, hold.ID, window+bookingstore.HoldCooldown+time.Minute)

	again := newHold(f, "18:00", "19:30")
	if err := takeHoldFrom(f, again, holdIP); err != nil {
		t.Fatalf("after the cooldown the same hours must be held again: %v", err)
	}
	if left := time.Until(readCreatedAt(t, f, again.ID).Add(window)); left < window-time.Minute {
		t.Errorf("a hold after the cooldown starts a fresh window; %v left", left)
	}
}

// S2: letting a hold go and picking overlapping hours again ("Cambiar
// horario") is allowed, but the clock does not start over: the new hold ends
// when the released one would have. Releasing and retaking cannot hold a slot
// for longer than one window.
func TestRetakingAReleasedHoldKeepsItsClock(t *testing.T) {
	f := datatest.Isolated(t)
	first := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, first)
	f.BackdateBookingCreatedAt(t, first.ID, window/2)
	started := readCreatedAt(t, f, first.ID)

	if _, released, err := f.Stores.Bookings.ReleaseHold(context.Background(), hash); err != nil || !released {
		t.Fatalf("ReleaseHold = %v, %v", released, err)
	}

	again := newHold(f, "18:30", "20:00")
	if err := takeHoldFrom(f, again, holdIP); err != nil {
		t.Fatalf("the visitor's own released hours must be pickable again: %v", err)
	}
	if !again.CreatedAt.Equal(started) || !readCreatedAt(t, f, again.ID).Equal(started) {
		t.Errorf("the retaken hold must keep the released one's clock (%v); got %v in memory, %v stored",
			started, again.CreatedAt, readCreatedAt(t, f, again.ID))
	}

	// A window that is already over by the time the hours are retaken is a lapse.
	f.BackdateBookingCreatedAt(t, again.ID, window+time.Second)
	err := takeHoldFrom(f, newHold(f, "18:00", "19:30"), holdIP)
	var cooldown *bookingstore.HoldCooldownError
	if !errors.As(err, &cooldown) {
		t.Fatalf("a slot held for its whole window must cool down; got %v", err)
	}
}

// S2, previous behavior: the cap still holds alongside the cooldown, and a
// first hold on fresh hours is untouched by it.
func TestTheCapStillAppliesBesideTheCooldown(t *testing.T) {
	f := datatest.Isolated(t)
	for _, start := range []string{"09:00", "11:00", "13:00"} {
		if err := takeHoldFrom(f, newHold(f, start, slotsEnd(start)), holdIP); err != nil {
			t.Fatalf("hold at %s: %v", start, err)
		}
	}
	if err := takeHoldFrom(f, newHold(f, "15:00", "16:30"), holdIP); !errors.Is(err, bookingstore.ErrHoldLimit) {
		t.Fatalf("a fourth live hold must still answer ErrHoldLimit; got %v", err)
	}
}

// S7: submitting the details leaves at least MinPaymentWindow to pay, and
// every reader of the window (the grid, the expiry sweep) agrees.
func TestConvertingLateLeavesTheMinimumPaymentWindow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	f.BackdateBookingCreatedAt(t, hold.ID, window-30*time.Second)

	withContact(hold)
	if err := f.Stores.Bookings.ConvertHold(ctx, hold, hash); err != nil {
		t.Fatalf("ConvertHold: %v", err)
	}
	left := time.Until(hold.CreatedAt.Add(window))
	if left < bookingstore.MinPaymentWindow-5*time.Second || left > bookingstore.MinPaymentWindow+5*time.Second {
		t.Errorf("want about %v left to pay; got %v", bookingstore.MinPaymentWindow, left)
	}
	if stored := readCreatedAt(t, f, hold.ID); !stored.Equal(hold.CreatedAt) {
		t.Errorf("the extension must be stored (%v), not only returned (%v)", stored, hold.CreatedAt)
	}

	if !contains(bookedStarts(t, f, hold.Date), "18:00") {
		t.Error("the converted booking must still hold its slot on the grid")
	}
	expired, err := f.Stores.Bookings.GetExpiredPendingEnriched(ctx, window)
	if err != nil {
		t.Fatalf("GetExpiredPendingEnriched: %v", err)
	}
	for _, cb := range expired {
		if cb.ID == hold.ID {
			t.Error("the expiry sweep must not release a booking inside its extended window")
		}
	}
}

// S7 with a window longer than the minimum (BOOKING_PAYMENT_EXPIRY set above
// MinPaymentWindow): a hold converted with more than the minimum left keeps
// its own clock. With the default window the minimum equals it, so every
// conversion starts a full minimum (TestConvertingLateLeavesTheMinimumPaymentWindow).
func TestConvertingEarlyKeepsTheHoldsClock(t *testing.T) {
	if window <= bookingstore.MinPaymentWindow {
		t.Skipf("window %v is not longer than the minimum %v: no early conversion exists", window, bookingstore.MinPaymentWindow)
	}
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	f.BackdateBookingCreatedAt(t, hold.ID, 2*time.Minute)
	started := readCreatedAt(t, f, hold.ID)

	withContact(hold)
	if err := f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), hold, hash); err != nil {
		t.Fatalf("ConvertHold: %v", err)
	}
	if !readCreatedAt(t, f, hold.ID).Equal(started) || !hold.CreatedAt.Equal(started) {
		t.Errorf("a hold with time to spare keeps its clock %v; got %v", started, readCreatedAt(t, f, hold.ID))
	}
}

// S3: the owner sees the hours a visitor is booking online, and until when,
// and nothing else of the visitor's.
func TestLiveHoldsListsTheOnlineHoldsOfTheDay(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	untouched := newHold(f, "09:00", "10:30")
	takeHold(t, f, untouched)
	converted := newHold(f, "12:00", "13:30")
	hash := takeHold(t, f, converted)
	withContact(converted)
	if err := f.Stores.Bookings.ConvertHold(ctx, converted, hash); err != nil {
		t.Fatalf("ConvertHold: %v", err)
	}
	lapsed := newHold(f, "15:00", "16:30")
	takeHold(t, f, lapsed)
	f.BackdateBookingCreatedAt(t, lapsed.ID, window+time.Minute)
	f.CreateBooking(t, datatest.BookingOptions{StartTime: "20:00", EndTime: "21:30"})

	store, ok := f.Stores.Bookings.(*bookingstore.Store)
	if !ok {
		t.Fatalf("the fixture's bookings store is %T", f.Stores.Bookings)
	}
	holds, err := store.LiveHolds(ctx, f.ComplexID, untouched.Date)
	if err != nil {
		t.Fatalf("LiveHolds: %v", err)
	}
	if len(holds) != 2 {
		t.Fatalf("want the untouched and the converted hold; got %+v", holds)
	}
	for i, want := range []*bookingstore.Booking{untouched, converted} {
		got := holds[i]
		if got.CourtID != f.CourtID || got.StartTime != want.StartTime || got.DurationMinutes != want.DurationMinutes {
			t.Errorf("hold %d = %+v, want court %v at %s for %d min", i, got, f.CourtID, want.StartTime, want.DurationMinutes)
		}
		if freesAt := readCreatedAt(t, f, want.ID).Add(window); !got.FreesAt.Equal(freesAt) {
			t.Errorf("hold %d frees at %v, want %v", i, got.FreesAt, freesAt)
		}
	}
}

// S3: an owner booking refused by a visitor's live hold says so, and when the
// hold frees up; the refusal stays ErrSlotUnavailable for every caller.
func TestAnOwnerBookingOverALiveHoldIsToldWhenItFrees(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	takeHold(t, f, hold)
	freesAt := readCreatedAt(t, f, hold.ID).Add(window)

	err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), f.NewBooking(datatest.BookingOptions{StartTime: "18:30", EndTime: "19:30"}))
	if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("want ErrSlotUnavailable; got %v", err)
	}
	var blocked *bookingstore.HoldBlockedError
	if !errors.As(err, &blocked) || !blocked.FreesAt.Equal(freesAt) {
		t.Fatalf("want HoldBlockedError freeing at %v; got %v", freesAt, err)
	}
}

// S3, previous behavior: an owner booking refused by a real booking is the
// plain ErrSlotUnavailable it always was.
func TestAnOwnerBookingOverARealBookingIsAPlainRefusal(t *testing.T) {
	f := datatest.Isolated(t)
	f.CreateBooking(t, datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})

	err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), f.NewBooking(datatest.BookingOptions{StartTime: "18:30", EndTime: "19:30"}))
	var blocked *bookingstore.HoldBlockedError
	if !errors.Is(err, bookingstore.ErrSlotUnavailable) || errors.As(err, &blocked) {
		t.Fatalf("want a plain ErrSlotUnavailable; got %v", err)
	}
}
