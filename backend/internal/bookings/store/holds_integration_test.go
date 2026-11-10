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

// holdIP is the address the holds in this file are taken from.
const holdIP = "198.51.100.7"

// newHold builds a slot hold the way POST /book/holds does: a pending, unpaid
// visitor booking that names nobody yet.
func newHold(f *datatest.Fixture, start, end string) *bookingstore.Booking {
	b := f.NewBooking(datatest.BookingOptions{
		Public: true, Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid,
		StartTime: start, EndTime: end,
	})
	b.ClientID = uuid.Nil
	return b
}

// takeHold inserts a hold through the real store and returns its token hash.
func takeHold(t *testing.T, f *datatest.Fixture, b *bookingstore.Booking) []byte {
	t.Helper()
	_, hash, err := bookingstore.NewHoldToken()
	if err != nil {
		t.Fatalf("NewHoldToken: %v", err)
	}
	if err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), b, hash, holdIP, 3); err != nil {
		t.Fatalf("InsertHold: %v", err)
	}
	return hash
}

func bookingExists(t *testing.T, f *datatest.Fixture, id uuid.UUID) bool {
	t.Helper()
	var n int
	if err := f.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM bookings WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("counting booking %s: %v", id, err)
	}
	return n == 1
}

func withContact(b *bookingstore.Booking) {
	b.ClientSnapshot = &bookingstore.ClientSnapshot{FirstName: "Ana", LastName: "Perez", Phone: "+5491100000077", Email: "ana@example.com"}
}

// Migration 025: a hold names nobody and is still accepted, and it holds its
// slot on the grid and against a second hold for the same hours.
func TestAHoldIsStoredWithNoContactAndHoldsItsSlot(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	takeHold(t, f, hold)

	var phone *string
	var hash []byte
	var ip *string
	if err := f.DB.QueryRow(context.Background(),
		`SELECT client_phone, hold_token_hash, hold_ip FROM bookings WHERE id = $1`, hold.ID).Scan(&phone, &hash, &ip); err != nil {
		t.Fatalf("reading the hold: %v", err)
	}
	if phone != nil || len(hash) == 0 || ip == nil || *ip != holdIP {
		t.Errorf("want no contact, a token hash and the address; got phone=%v hash=%d ip=%v", phone, len(hash), ip)
	}
	if !contains(bookedStarts(t, f, hold.Date), "18:00") {
		t.Error("a live hold must show as booked on the availability grid")
	}

	second := newHold(f, "18:30", "20:00")
	_, h2, _ := bookingstore.NewHoldToken()
	err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), second, h2, "203.0.113.9", 3)
	if !errors.Is(err, bookingstore.ErrSlotUnavailable) && !errors.Is(err, bookingstore.ErrDuplicateBooking) {
		t.Errorf("an overlapping second hold must be refused as taken; got %v", err)
	}
}

// Migration 025 relaxes the rule for holds only: any other booking still needs
// a client or a snapshot.
func TestANonHoldBookingStillNeedsAClientOrASnapshot(t *testing.T) {
	f := datatest.Isolated(t)
	owner := f.NewBooking(datatest.BookingOptions{Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid})
	owner.ClientID = uuid.Nil
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), owner); err == nil {
		t.Fatal("an owner booking with neither a client nor a snapshot must be refused (bookings_client_or_snapshot)")
	}
}

func TestTheActiveHoldCapPerAddress(t *testing.T) {
	f := datatest.Isolated(t)
	for i, start := range []string{"09:00", "11:00", "13:00"} {
		_, hash, _ := bookingstore.NewHoldToken()
		b := newHold(f, start, slotsEnd(start))
		if err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), b, hash, holdIP, 3); err != nil {
			t.Fatalf("hold %d must be accepted: %v", i+1, err)
		}
	}
	_, hash, _ := bookingstore.NewHoldToken()
	err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), newHold(f, "15:00", "16:30"), hash, holdIP, 3)
	if !errors.Is(err, bookingstore.ErrHoldLimit) {
		t.Fatalf("a fourth live hold from one address must answer ErrHoldLimit; got %v", err)
	}
	// Another address is not counted against this one.
	_, other, _ := bookingstore.NewHoldToken()
	if err := f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), newHold(f, "15:00", "16:30"), other, "203.0.113.50", 3); err != nil {
		t.Errorf("another address must not share the cap: %v", err)
	}
}

func slotsEnd(start string) string {
	t, _ := time.Parse("15:04", start)
	return t.Add(90 * time.Minute).Format("15:04")
}

func TestReleaseDeletesAnUntouchedHoldAndIsIdempotent(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)

	freed, released, err := f.Stores.Bookings.ReleaseHold(context.Background(), hash)
	if err != nil || !released || freed.ComplexID != f.ComplexID {
		t.Fatalf("ReleaseHold = (%+v, %v, %v), want complex %v, true, nil", freed, released, err, f.ComplexID)
	}
	if freed.Day.Format(time.DateOnly) != hold.Date.Format(time.DateOnly) || freed.ComplexSlug == "" || freed.ComplexName == "" {
		t.Errorf("want the freed slot's day and venue for the waitlist; got %+v (hold on %v)", freed, hold.Date)
	}
	if bookingExists(t, f, hold.ID) {
		t.Error("a released untouched hold must be gone")
	}
	if contains(bookedStarts(t, f, hold.Date), "18:00") {
		t.Error("a released hold must free its slot")
	}
	if _, released, err := f.Stores.Bookings.ReleaseHold(context.Background(), hash); err != nil || released {
		t.Errorf("a second release must be a no-op; got released=%v err=%v", released, err)
	}
}

func TestConvertWritesTheSnapshotAndReleaseNeverTouchesTheConvertedBooking(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)

	got, err := f.Stores.Bookings.GetHold(context.Background(), hash)
	if err != nil || got.ID != hold.ID {
		t.Fatalf("GetHold = (%v, %v), want the hold", got, err)
	}

	withContact(hold)
	if err := f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), hold, hash); err != nil {
		t.Fatalf("ConvertHold: %v", err)
	}
	if hold.LinkToken == "" {
		t.Error("a converted hold must carry a fresh link token")
	}
	read, err := f.Stores.Bookings.GetByID(f.Scoped(context.Background()), hold.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if read.ClientSnapshot == nil || read.ClientSnapshot.Phone != "+5491100000077" {
		t.Errorf("want the snapshot written; got %+v", read.ClientSnapshot)
	}

	if _, released, err := f.Stores.Bookings.ReleaseHold(context.Background(), hash); err != nil || released {
		t.Errorf("release after conversion must not touch the booking; released=%v err=%v", released, err)
	}
	if !bookingExists(t, f, hold.ID) {
		t.Fatal("the converted booking must survive a release")
	}
	if err := f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), hold, hash); !errors.Is(err, bookingstore.ErrHoldNotFound) {
		t.Errorf("a second conversion must answer ErrHoldNotFound; got %v", err)
	}
	if _, err := f.Stores.Bookings.GetHold(context.Background(), hash); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a converted hold's token must resolve nothing; got %v", err)
	}
}

func TestReleaseNeverTouchesAPaidBooking(t *testing.T) {
	f := datatest.Isolated(t)
	paid := f.CreateBooking(t, datatest.BookingOptions{Public: true})
	// A paid booking whose row somehow still carried a token: the predicate,
	// not the token, is what protects it.
	_, hash, _ := bookingstore.NewHoldToken()
	if _, err := f.DB.Exec(context.Background(), `UPDATE bookings SET hold_token_hash = $2 WHERE id = $1`, paid.ID, hash); err != nil {
		t.Fatalf("tagging the paid booking: %v", err)
	}
	if _, released, err := f.Stores.Bookings.ReleaseHold(context.Background(), hash); err != nil || released {
		t.Errorf("a paid booking must never be released; released=%v err=%v", released, err)
	}
	if !bookingExists(t, f, paid.ID) {
		t.Fatal("the paid booking must survive")
	}
}

func TestAnExpiredHoldCannotBeConvertedAndTheSweepDeletesIt(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, hold)
	f.BackdateBookingCreatedAt(t, hold.ID, time.Hour)

	withContact(hold)
	if err := f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), hold, hash); !errors.Is(err, bookingstore.ErrHoldNotFound) {
		t.Errorf("an expired hold must not convert; got %v", err)
	}

	expired, err := f.Stores.Bookings.GetExpiredPendingEnriched(f.Scoped(context.Background()), 15*time.Minute)
	if err != nil {
		t.Fatalf("GetExpiredPendingEnriched: %v", err)
	}
	for _, cb := range expired {
		if cb.ID == hold.ID {
			t.Error("an untouched hold is the hold sweep's, not the notice sweep's")
		}
	}

	freed, err := f.Stores.Bookings.DeleteExpiredHolds(f.Scoped(context.Background()), 15*time.Minute)
	if err != nil {
		t.Fatalf("DeleteExpiredHolds: %v", err)
	}
	if len(freed) != 1 || freed[0].ComplexID != f.ComplexID || freed[0].Day.Format(time.DateOnly) != hold.Date.Format(time.DateOnly) || freed[0].ComplexSlug == "" {
		t.Errorf("want the hold's venue and day reported once; got %+v", freed)
	}
	if bookingExists(t, f, hold.ID) {
		t.Error("an expired untouched hold must be deleted")
	}
}

func TestTheSweepLeavesALiveHoldAndAConvertedOne(t *testing.T) {
	f := datatest.Isolated(t)
	live := newHold(f, "09:00", "10:30")
	takeHold(t, f, live)
	converted := newHold(f, "18:00", "19:30")
	hash := takeHold(t, f, converted)
	withContact(converted)
	if err := f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), converted, hash); err != nil {
		t.Fatalf("ConvertHold: %v", err)
	}
	f.BackdateBookingCreatedAt(t, converted.ID, time.Hour)

	if _, err := f.Stores.Bookings.DeleteExpiredHolds(f.Scoped(context.Background()), 15*time.Minute); err != nil {
		t.Fatalf("DeleteExpiredHolds: %v", err)
	}
	if !bookingExists(t, f, live.ID) || !bookingExists(t, f, converted.ID) {
		t.Error("the sweep must delete only expired untouched holds")
	}
}

func TestDisconnectDeletesTheComplexsHolds(t *testing.T) {
	f := datatest.Isolated(t)
	hold := newHold(f, "18:00", "19:30")
	takeHold(t, f, hold)
	holds, err := f.Stores.Bookings.GetUnpaidPublicPendingEnriched(f.Scoped(context.Background()), f.ComplexID)
	if err != nil {
		t.Fatalf("GetUnpaidPublicPendingEnriched: %v", err)
	}
	if len(holds) != 0 {
		t.Errorf("an untouched hold has no checkout to close; want it absent, got %d", len(holds))
	}
	if err := f.Stores.Bookings.DeleteComplexHolds(f.Scoped(context.Background()), f.ComplexID); err != nil {
		t.Fatalf("DeleteComplexHolds: %v", err)
	}
	if bookingExists(t, f, hold.ID) {
		t.Error("the complex's hold must be gone")
	}
}

// Two visitors taking overlapping holds at once: exactly one wins.
func TestOverlappingHoldsRaceExactlyOneWins(t *testing.T) {
	f := datatest.Shared(t)
	a, b := newHold(f, "18:00", "19:30"), newHold(f, "18:30", "20:00")
	_, ha, _ := bookingstore.NewHoldToken()
	_, hb, _ := bookingstore.NewHoldToken()

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, pair := range []struct {
		b    *bookingstore.Booking
		hash []byte
		ip   string
	}{{a, ha, "203.0.113.1"}, {b, hb, "203.0.113.2"}} {
		wg.Go(func() {
			errs[i] = f.Stores.Bookings.InsertHold(f.Scoped(context.Background()), pair.b, pair.hash, pair.ip, 3)
		})
	}
	wg.Wait()

	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, bookingstore.ErrSlotUnavailable), errors.Is(err, bookingstore.ErrDuplicateBooking):
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("exactly one overlapping hold must win; %d did (%v)", wins, errs)
	}
}

// Converting twice at once, and converting while the visitor releases: one
// outcome each, never both.
func TestConvertRacesConvertAndRelease(t *testing.T) {
	t.Run("double convert", func(t *testing.T) {
		f := datatest.Shared(t)
		hold := newHold(f, "18:00", "19:30")
		hash := takeHold(t, f, hold)

		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i := range 2 {
			wg.Go(func() {
				c := *hold
				withContact(&c)
				errs[i] = f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), &c, hash)
			})
		}
		wg.Wait()
		ok, lost := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, bookingstore.ErrHoldNotFound):
				lost++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}
		if ok != 1 || lost != 1 {
			t.Fatalf("want one conversion and one ErrHoldNotFound; got %v", errs)
		}
	})

	t.Run("convert against release", func(t *testing.T) {
		f := datatest.Shared(t)
		hold := newHold(f, "18:00", "19:30")
		hash := takeHold(t, f, hold)

		var convertErr, releaseErr error
		var released bool
		var wg sync.WaitGroup
		wg.Go(func() {
			c := *hold
			withContact(&c)
			convertErr = f.Stores.Bookings.ConvertHold(f.Scoped(context.Background()), &c, hash)
		})
		wg.Go(func() {
			_, released, releaseErr = f.Stores.Bookings.ReleaseHold(data.ContextWithTenantBypass(context.Background()), hash)
		})
		wg.Wait()
		if releaseErr != nil {
			t.Fatalf("ReleaseHold: %v", releaseErr)
		}
		switch {
		case convertErr == nil && !released:
			if !bookingExists(t, f, hold.ID) {
				t.Error("a converted booking must survive the release")
			}
		case errors.Is(convertErr, bookingstore.ErrHoldNotFound) && released:
			if bookingExists(t, f, hold.ID) {
				t.Error("a released hold must be gone")
			}
		default:
			t.Fatalf("want exactly one of convert/release to win; convert=%v released=%v", convertErr, released)
		}
	})
}
