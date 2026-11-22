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

// ageHold moves a storefront booking back in time by age: both the instant
// it started holding (hold_started_at) and the one its window counts from
// (created_at). The fixture runs in one transaction, so NOW() never moves.
func ageHold(t *testing.T, f *datatest.Fixture, id uuid.UUID, age time.Duration) {
	t.Helper()
	if _, err := f.DB.Exec(context.Background(), `
		UPDATE bookings SET created_at = NOW() - make_interval(secs => $2),
		                    hold_started_at = NOW() - make_interval(secs => $2)
		WHERE id = $1`, id, age.Seconds()); err != nil {
		t.Fatalf("ageing booking %s: %v", id, err)
	}
}

// S2: an address that held hours for a whole window without paying waits
// HoldCooldown from when it stopped, whichever sweep (or none yet) noticed the
// lapse. Other hours, and other addresses, are not affected.
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
			ageHold(t, f, hold.ID, window+time.Minute)
			lapsedAt := readCreatedAt(t, f, hold.ID).Add(window)
			if swept {
				if _, err := f.Stores.Bookings.DeleteExpiredHolds(f.Scoped(context.Background()), window); err != nil {
					t.Fatalf("DeleteExpiredHolds: %v", err)
				}
			}

			err := takeHoldFrom(f, newHold(f, "18:30", "20:00"), holdIP)
			var cooldown *bookingstore.HoldCooldownError
			if !errors.As(err, &cooldown) {
				t.Fatalf("want HoldCooldownError; got %v", err)
			}
			if want := lapsedAt.Add(bookingstore.HoldCooldown); cooldown.Until.Sub(want).Abs() > time.Second {
				t.Errorf("Until = %v, want the lapse plus the cooldown (%v)", cooldown.Until, want)
			}

			if err := takeHoldFrom(f, newHold(f, "20:00", "21:30"), holdIP); err != nil {
				t.Errorf("hours the lapsed hold did not cover must not be refused: %v", err)
			}
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
	ageHold(t, f, hold.ID, window+bookingstore.HoldCooldown+time.Minute)

	again := newHold(f, "18:00", "19:30")
	if err := takeHoldFrom(f, again, holdIP); err != nil {
		t.Fatalf("after the cooldown the same hours must be held again: %v", err)
	}
	if left := time.Until(readCreatedAt(t, f, again.ID).Add(window)); left < window-time.Minute {
		t.Errorf("a hold after the cooldown starts a fresh window; %v left", left)
	}
}

// S2: letting a hold go and picking overlapping hours again ("Cambiar
// horario") spends only the time it was actually held: the new hold gets the
// rest of the window, and every reader agrees on when it ends.
func TestRetakingAReleasedHoldGetsTheRestOfTheWindow(t *testing.T) {
	f := datatest.Isolated(t)
	first := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, first)
	ageHold(t, f, first.ID, 4*time.Minute)

	if _, released, err := f.Stores.Bookings.ReleaseHold(context.Background(), hash); err != nil || !released {
		t.Fatalf("ReleaseHold = %v, %v", released, err)
	}

	again := newHold(f, "18:30", "20:00")
	if err := takeHoldFrom(f, again, holdIP); err != nil {
		t.Fatalf("the visitor's own released hours must be pickable again: %v", err)
	}
	left := time.Until(readCreatedAt(t, f, again.ID).Add(window))
	if want := window - 4*time.Minute; (left - want).Abs() > 5*time.Second {
		t.Errorf("want about %v left (the window minus the four minutes held); got %v", want, left)
	}
	if !again.CreatedAt.Equal(readCreatedAt(t, f, again.ID)) {
		t.Errorf("the hold handed back (%v) must end when the stored one does (%v)", again.CreatedAt, readCreatedAt(t, f, again.ID))
	}
}

// S2: a converted booking's time spends the budget too. Hold, submit the
// details (a fresh payment window), let it go just before it ends, and the
// same hours are refused rather than held again: the cycle could otherwise
// keep a slot off sale indefinitely.
func TestTimeHeldAfterConvertingSpendsTheBudget(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	withContact(hold)
	if err := f.Stores.Bookings.ConvertHold(ctx, hold, hash); err != nil {
		t.Fatalf("ConvertHold: %v", err)
	}
	// Nine minutes as a hold before the details, nine and a half since.
	if _, err := f.DB.Exec(context.Background(), `
		UPDATE bookings SET hold_started_at = NOW() - interval '18 minutes 30 seconds',
		                    created_at = NOW() - interval '9 minutes 30 seconds',
		                    status = 'cancelled'
		WHERE id = $1`, hold.ID); err != nil {
		t.Fatalf("releasing the converted booking: %v", err)
	}

	err := takeHoldFrom(f, newHold(f, "18:00", "19:30"), holdIP)
	var cooldown *bookingstore.HoldCooldownError
	if !errors.As(err, &cooldown) {
		t.Fatalf("want HoldCooldownError after a window and more held; got %v", err)
	}
	if want := time.Now().Add(bookingstore.HoldCooldown); cooldown.Until.Sub(want).Abs() > time.Minute {
		t.Errorf("want the cooldown counted from the release (%v); got %v", want, cooldown.Until)
	}
}

// S2, finding 1: POST /book without a hold token is held to the same budget.
// It used to skip it, so the same address could hold the hours again the
// moment its hold lapsed.
func TestABookingWithoutAHoldTokenIsHeldToTheBudget(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	hold := newHold(f, "18:00", "19:30")
	takeHold(t, f, hold)
	ageHold(t, f, hold.ID, window+time.Minute)

	direct := newHold(f, "18:00", "19:30")
	withContact(direct)
	err := f.Stores.Bookings.InsertHold(ctx, direct, nil, holdIP, 3)
	var cooldown *bookingstore.HoldCooldownError
	if !errors.As(err, &cooldown) {
		t.Fatalf("want HoldCooldownError for the token-less booking; got %v", err)
	}
}

// S2: a token-less booking with budget left is taken with the whole window to
// pay, as a converted hold is, and its time counts against the next hold.
func TestABookingWithoutAHoldTokenGetsTheWholeWindowAndSpendsIt(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	ageHold(t, f, hold.ID, 5*time.Minute)
	if _, released, err := f.Stores.Bookings.ReleaseHold(context.Background(), hash); err != nil || !released {
		t.Fatalf("ReleaseHold = %v, %v", released, err)
	}

	direct := newHold(f, "18:00", "19:30")
	withContact(direct)
	if err := f.Stores.Bookings.InsertHold(ctx, direct, nil, holdIP, 3); err != nil {
		t.Fatalf("a token-less booking with budget left must be taken: %v", err)
	}
	if left := time.Until(readCreatedAt(t, f, direct.ID).Add(window)); left < window-time.Minute {
		t.Errorf("a token-less booking gets the whole window to pay; %v left", left)
	}

	// Its window runs out unpaid: five minutes held before, ten since.
	ageHold(t, f, direct.ID, window+time.Second)
	err := takeHoldFrom(f, newHold(f, "18:00", "19:30"), holdIP)
	var cooldown *bookingstore.HoldCooldownError
	if !errors.As(err, &cooldown) {
		t.Fatalf("the token-less booking's time must spend the budget; got %v", err)
	}
}

// S2, previous behavior: a paid booking on the same hours from the same
// address never spends the budget.
func TestAPaidBookingDoesNotSpendTheBudget(t *testing.T) {
	f := datatest.Isolated(t)
	paid := newHold(f, "18:00", "19:30")
	withContact(paid)
	if err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), paid, nil, holdIP, 3); err != nil {
		t.Fatalf("InsertHold: %v", err)
	}
	ageHold(t, f, paid.ID, window+time.Minute)
	if _, err := f.DB.Exec(context.Background(), `
		UPDATE bookings SET status = 'cancelled', collection_status = 'deposit_paid' WHERE id = $1`, paid.ID); err != nil {
		t.Fatalf("paying: %v", err)
	}

	if err := takeHoldFrom(f, newHold(f, "18:00", "19:30"), holdIP); err != nil {
		t.Errorf("a paid booking must not spend the address's budget: %v", err)
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
