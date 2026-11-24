//go:build integration

package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
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

// lockRowThen holds the hold's row lock in its own transaction, starts every
// racer, and only then lets go: each racer blocks on the row, so they are all
// waiting when it frees and the database alone picks who goes first. The
// second one re-reads the row as the first left it (READ COMMITTED).
func lockRowThen(t *testing.T, f *datatest.Fixture, id uuid.UUID, racers ...func()) {
	t.Helper()
	ctx := data.ContextWithTenantBypass(context.Background())
	tx, err := f.DB.Begin(ctx)
	if err != nil {
		t.Fatalf("begin the lock holder: %v", err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM bookings WHERE id = $1 FOR UPDATE`, id); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("lock the hold: %v", err)
	}
	var wg sync.WaitGroup
	for _, race := range racers {
		wg.Go(race)
	}
	time.Sleep(200 * time.Millisecond)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("release the lock: %v", err)
	}
	wg.Wait()
}

// takeUncappedHold inserts a hold from an address of its own and with no
// per-address cap: a Shared fixture keeps every earlier test's live holds, and
// they would count against holdIP's cap of three.
func takeUncappedHold(t *testing.T, f *datatest.Fixture, b *bookingstore.Booking) []byte {
	t.Helper()
	_, hash, err := bookingstore.NewHoldToken()
	if err != nil {
		t.Fatalf("NewHoldToken: %v", err)
	}
	if err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), b, hash, uuid.NewString(), 0); err != nil {
		t.Fatalf("InsertHold: %v", err)
	}
	return hash
}

// releaseLeftFor runs the sweep and counts what it freed for one complex.
func releaseLeftFor(f *datatest.Fixture) (int, error) {
	freed, err := f.Stores.Bookings.ReleaseLeftHolds(data.ContextWithTenantBypass(context.Background()), leftGrace)
	n := 0
	for _, h := range freed {
		if h.ComplexID == f.ComplexID {
			n++
		}
	}
	return n, err
}

func TestTheLeftHoldSweepRaces(t *testing.T) {
	t.Run("reclaim against the sweep: exactly one wins", func(t *testing.T) {
		for range 5 {
			f := datatest.Shared(t)
			hold := newHold(f, "18:00", "19:30")
			hash := takeUncappedHold(t, f, hold)
			if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
				t.Fatalf("RequestHoldRelease: %v", err)
			}
			backdateReleaseRequest(t, f, hold.ID, leftGrace+time.Second)

			var reclaimErr, sweepErr error
			var freed int
			lockRowThen(t, f, hold.ID,
				func() {
					_, reclaimErr = f.Stores.Bookings.ReclaimHold(data.ContextWithTenantBypass(context.Background()), hash)
				},
				func() { freed, sweepErr = releaseLeftFor(f) },
			)
			if sweepErr != nil {
				t.Fatalf("ReleaseLeftHolds: %v", sweepErr)
			}
			status, _, _ := f.ReadBookingState(t, hold.ID)
			switch {
			case reclaimErr == nil && freed == 0:
				if status != "pending" {
					t.Errorf("a reclaimed hold must stay pending; got %q", status)
				}
			case errors.Is(reclaimErr, bookingstore.ErrHoldNotFound) && freed == 1:
				if status != "cancelled" {
					t.Errorf("a released hold must be cancelled; got %q", status)
				}
			default:
				t.Fatalf("want exactly one of reclaim/sweep to win; reclaim=%v freed=%d", reclaimErr, freed)
			}
		}
	})

	t.Run("convert against the sweep: exactly one wins", func(t *testing.T) {
		for range 5 {
			f := datatest.Shared(t)
			hold := newHold(f, "18:00", "19:30")
			hash := takeUncappedHold(t, f, hold)
			if err := f.Stores.Bookings.RequestHoldRelease(context.Background(), hash); err != nil {
				t.Fatalf("RequestHoldRelease: %v", err)
			}
			backdateReleaseRequest(t, f, hold.ID, leftGrace+time.Second)

			var convertErr, sweepErr error
			var freed int
			lockRowThen(t, f, hold.ID,
				func() {
					c := *hold
					withContact(&c)
					convertErr = f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), &c, hash)
				},
				func() { freed, sweepErr = releaseLeftFor(f) },
			)
			if sweepErr != nil {
				t.Fatalf("ReleaseLeftHolds: %v", sweepErr)
			}
			status, _, _ := f.ReadBookingState(t, hold.ID)
			switch {
			case convertErr == nil && freed == 0:
				if status != "pending" || releaseRequestedAt(t, f, hold.ID) != nil {
					t.Errorf("a converted booking must stay pending with no release asked; got %q", status)
				}
			case errors.Is(convertErr, bookingstore.ErrHoldNotFound) && freed == 1:
				if status != "cancelled" {
					t.Errorf("a released hold must be cancelled; got %q", status)
				}
			default:
				t.Fatalf("want exactly one of convert/sweep to win; convert=%v freed=%d", convertErr, freed)
			}
		}
	})
}

// A leave beacon from the page that went away can reach the server after the
// reloaded page already reclaimed the hold. It asks for the release again, and
// only the page's next reclaim (it asserts the hold again a few seconds after
// loading) calls it off for good.
func TestALeaveLandingAfterTheReclaimIsCalledOffByTheNextReclaim(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	ctx := context.Background()

	if _, err := f.Stores.Bookings.ReclaimHold(ctx, hash); err != nil {
		t.Fatalf("ReclaimHold on load: %v", err)
	}
	if err := f.Stores.Bookings.RequestHoldRelease(ctx, hash); err != nil {
		t.Fatalf("the late leave: %v", err)
	}
	if releaseRequestedAt(t, f, hold.ID) == nil {
		t.Fatal("a leave after the reclaim asks for the release again")
	}

	if _, err := f.Stores.Bookings.ReclaimHold(ctx, hash); err != nil {
		t.Fatalf("ReclaimHold again: %v", err)
	}
	if releaseRequestedAt(t, f, hold.ID) != nil {
		t.Fatal("the second reclaim must clear the late request")
	}
	if freed := releaseLeft(t, f); len(freed) != 0 {
		t.Errorf("a hold reasserted after a late leave must stay; freed %+v", freed)
	}
}
