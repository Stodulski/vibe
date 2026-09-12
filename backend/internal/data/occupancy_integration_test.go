//go:build integration

package data_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// FINDING 3. Three places decided whether an existing booking takes a court's
// hours out of circulation, and all three disagreed:
//
//	SlotTaken (slot_guard.go)              status NOT IN ('cancelled', 'no_show')
//	GetBookedSlots (db/queries)            status NOT IN ('cancelled')
//	idx_bookings_no_double (the initial schema) WHERE status NOT IN ('cancelled')
//
// A no_show therefore displayed as taken, passed the overlap check as free, and
// was refused by the index as a duplicate — permanently unbookable while shown
// as unavailable, with an error contradicting the grid the client was reading.
//
// The tests below are one round trip through all three against a real database:
// what the grid reads, what the overlap check decides, and what the index
// permits. A fix to any one of them alone leaves one of these red.

// markStatus moves a booking to a terminal status the way the owner-facing
// update does, through a plain UPDATE that the status-reversal trigger sees.
func markStatus(t *testing.T, f *testFixture, id uuid.UUID, status string) {
	t.Helper()

	_, err := f.Pool.Exec(context.Background(),
		`UPDATE bookings SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		t.Fatalf("marking booking %s: %v", status, err)
	}
}

// bookedStarts is what the availability grid reads to draw a slot as taken.
func bookedStarts(t *testing.T, f *testFixture, date time.Time) []string {
	t.Helper()

	slots, err := f.Models.Bookings.GetBookedSlotsByCourtIDs(
		context.Background(), []uuid.UUID{f.CourtID}, date)
	if err != nil {
		t.Fatalf("reading booked slots: %v", err)
	}

	// A booked span carries instants now, not a time of day — phase 2 of the
	// cross-midnight migration replaced TimeSlot with BookedSpan. Rendered in
	// Argentina, which is the clock the callers of this helper reason in.
	out := make([]string, 0, len(slots))
	for _, s := range slots {
		out = append(out, s.StartsAt.In(timezone.Argentina).Format("15:04"))
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// A live booking occupies its hours in all three places. This is the half of
// the predicate that must not be lost while fixing the other half.
func TestAConfirmedBookingOccupiesItsSlotEverywhere(t *testing.T) {
	f := newTestFixture(t)

	first := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	if err := f.Models.Bookings.InsertSafe(context.Background(), first); err != nil {
		t.Fatalf("the first booking must be accepted: %v", err)
	}

	if got := bookedStarts(t, f, first.Date); !contains(got, "18:00") {
		t.Errorf("the grid must draw 18:00 as taken; it read %v", got)
	}

	second := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	err := f.Models.Bookings.InsertSafe(context.Background(), second)
	if !errors.Is(err, data.ErrSlotUnavailable) {
		t.Errorf("the overlap check must refuse a second booking on those hours; got %v", err)
	}
}

// A cancelled booking releases its hours in all three places — the one status
// every site already agreed on.
func TestACancelledBookingReleasesItsSlotEverywhere(t *testing.T) {
	f := newTestFixture(t)

	first := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	if err := f.Models.Bookings.InsertSafe(context.Background(), first); err != nil {
		t.Fatalf("the first booking must be accepted: %v", err)
	}
	markStatus(t, f, first.ID, "cancelled")

	if got := bookedStarts(t, f, first.Date); contains(got, "18:00") {
		t.Errorf("a cancelled booking must not draw its hours as taken; the grid read %v", got)
	}

	second := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	if err := f.Models.Bookings.InsertSafe(context.Background(), second); err != nil {
		t.Errorf("the hours of a cancelled booking must be resellable: %v", err)
	}
}

// The finding itself. no_show is the status the three sites disagreed about,
// and the assertions run in the order a client hits them: what the grid shows,
// what the overlap check decides, and what the index permits.
func TestANoShowBookingReleasesItsSlotEverywhere(t *testing.T) {
	f := newTestFixture(t)

	first := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	if err := f.Models.Bookings.InsertSafe(context.Background(), first); err != nil {
		t.Fatalf("the first booking must be accepted: %v", err)
	}
	markStatus(t, f, first.ID, "no_show")

	// 1. The grid. GetBookedSlots excluded only 'cancelled', so these hours were
	//    still drawn as unavailable.
	if got := bookedStarts(t, f, first.Date); contains(got, "18:00") {
		t.Errorf("a no_show releases the court, so the grid must not draw 18:00 as taken; it read %v", got)
	}

	// 2 and 3. The overlap check already let this through; the unique index did
	//    not, and answered with a duplicate error the grid contradicted. Both
	//    have to agree for the resale to land.
	second := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	err := f.Models.Bookings.InsertSafe(context.Background(), second)
	if errors.Is(err, data.ErrDuplicateBooking) {
		t.Fatalf("idx_bookings_no_double still counts a no_show as live: the slot is shown free and cannot be sold (%v)", err)
	}
	if err != nil {
		t.Fatalf("the hours of a no_show must be resellable: %v", err)
	}

	// And the resale itself is now the live booking, occupying the hours it
	// bought — the index has not simply been switched off.
	if got := bookedStarts(t, f, first.Date); !contains(got, "18:00") {
		t.Errorf("the resold booking must draw 18:00 as taken; the grid read %v", got)
	}
	third := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	if err := f.Models.Bookings.InsertSafe(context.Background(), third); !errors.Is(err, data.ErrSlotUnavailable) {
		t.Errorf("only one live booking may hold those hours; got %v", err)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// FINDINGS 38 AND 81. The three write-path sites above were brought to one
// predicate; the queries that COUNT bookings were not, and still asked
// `status != 'cancelled'`.
//
// That is not a cosmetic disagreement, because the schema deliberately lets
// a no_show and its resale share one (court, date, start_time). Every query
// that counts "bookings that took court time" therefore counts that one hour
// twice — and both consumers clamp with min(…, 100), so the over-count reads as
// a plausible number rather than an obviously broken one.
//
// The tests below seed exactly that shape: a no_show beside a live resale on
// the same slot. It is the only shape the double-count appears in, and no unit
// test with a store double can see it — the defect is in the SQL.
// ───────────────────────────────────────────────────────────────────────────

// noShowThenResale marks the first booking absent and sells its hours again,
// returning the resale. The order matters: idx_bookings_no_double only tolerates
// the pair once the first row has left the live set.
func noShowThenResale(t *testing.T, f *testFixture, first *data.Booking, opts bookingOptions) *data.Booking {
	t.Helper()

	markStatus(t, f, first.ID, "no_show")

	opts.StartTime = first.StartTime
	// The fixture spells a length as an end time; the resale must cover exactly
	// the same hours, so it is rebuilt from the duration the first booking
	// actually holds rather than from a column that no longer exists.
	opts.EndTime = slots.Add(first.StartTime, first.DurationMinutes)
	resale := f.createBooking(t, opts)
	if !resale.Date.Equal(first.Date) {
		t.Fatalf("the resale must land on the same day as the no_show: %v vs %v", resale.Date, first.Date)
	}
	return resale
}

// The dashboard's headline counters. One court-hour was sold twice, and it is
// still one court-hour: two rows, ninety minutes, one booking on the board.
func TestTheDashboardCountsAResoldNoShowHourOnce(t *testing.T) {
	f := newTestFixture(t)

	first := f.createBooking(t, bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	noShowThenResale(t, f, first, bookingOptions{})

	// A game that was played is court time too. It is here so that a predicate
	// widened past no_show — the obvious over-correction — cannot pass by making
	// the counters agree at the cost of forgetting every finished booking.
	played := f.createBooking(t, bookingOptions{StartTime: "20:00", EndTime: "21:30"})
	markStatus(t, f, played.ID, "completed")

	stats, err := f.Models.Bookings.GetDashboardStats(context.Background(), f.ComplexID, first.Date)
	if err != nil {
		t.Fatalf("reading dashboard stats: %v", err)
	}

	if stats.TodayBookings != 2 {
		t.Errorf("the resold hour counts once and the played hour counts; want 2 bookings, the dashboard counted %d", stats.TodayBookings)
	}
	if stats.TodayBookedMinutes != 180 {
		t.Errorf("two ninety-minute hours were used, not %d minutes — this figure is the numerator of occupancy_rate", stats.TodayBookedMinutes)
	}

	// A count of one must mean "one booking occupies the hour", never "both rows
	// were dropped" — so the live resale has to be the row that survives.
	if stats.TodayBookings == 0 {
		t.Error("the live resale must still be counted; the hour was sold again, not released")
	}

	// The yesterday counter is a third subquery reading a day back. Asking the
	// same day from tomorrow's vantage point exercises it on the same fixture,
	// rather than trusting that one fix reached all three.
	dayAfter := first.Date.AddDate(0, 0, 1)
	fromTomorrow, err := f.Models.Bookings.GetDashboardStats(context.Background(), f.ComplexID, dayAfter)
	if err != nil {
		t.Fatalf("reading dashboard stats a day on: %v", err)
	}
	if fromTomorrow.YesterdayBookings != 2 {
		t.Errorf("the yesterday counter must also see two bookings, not %d", fromTomorrow.YesterdayBookings)
	}
}

// The occupancy heatmap. Its cell is (bookings in this hour × 100) / (courts ×
// weeks), clamped at 100 — so the double-count shows up as a hot cell rather
// than as an impossible number.
func TestTheOccupancyHeatmapCountsAResoldNoShowHourOnce(t *testing.T) {
	f := newTestFixture(t)

	first := f.createBooking(t, bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	noShowThenResale(t, f, first, bookingOptions{})

	// The 20:00 cell holds a played game, so a predicate widened past no_show
	// cannot pass here either — it would empty this cell instead.
	played := f.createBooking(t, bookingOptions{StartTime: "20:00", EndTime: "21:30"})
	markStatus(t, f, played.ID, "completed")

	points, err := f.Models.Bookings.GetOccupancyByHourDay(
		context.Background(), f.ComplexID, first.Date, first.Date)
	if err != nil {
		t.Fatalf("reading the occupancy heatmap: %v", err)
	}

	cells := make(map[int]int, len(points))
	for _, p := range points {
		cells[p.Hour] = p.BookingCount
	}

	if got, ok := cells[18]; !ok || got != 1 {
		t.Errorf("the 18:00 cell covers one court-hour sold twice; the heatmap counted %d (present=%v), which renders as double the occupancy", got, ok)
	}
	if got, ok := cells[20]; !ok || got != 1 {
		t.Errorf("the 20:00 cell holds a played game and must still count; the heatmap counted %d (present=%v)", got, ok)
	}
}

// The "next up" list. A booking nobody is coming to must not sit above the one
// that replaced it.
func TestTheUpcomingListShowsTheResaleAndNotTheNoShow(t *testing.T) {
	f := newTestFixture(t)

	first := f.createBooking(t, bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	resale := noShowThenResale(t, f, first, bookingOptions{})

	upcoming, err := f.Models.Bookings.GetUpcomingToday(
		context.Background(), f.ComplexID, first.Date, "00:00", 10)
	if err != nil {
		t.Fatalf("reading the upcoming list: %v", err)
	}

	if len(upcoming) != 1 {
		ids := make([]string, 0, len(upcoming))
		for _, b := range upcoming {
			ids = append(ids, b.ID.String()+"="+b.Status)
		}
		t.Fatalf("one booking holds 18:00; the list returned %d (%v)", len(upcoming), ids)
	}
	if upcoming[0].ID != resale.ID {
		t.Errorf("the list must show the resale %s, not the no_show %s", resale.ID, first.ID)
	}
}

// The court-deletion guard, walked through the whole terminal progression a
// booking can make. confirmed and completed both took the hour and both must
// keep blocking; no_show gave it back and must not.
//
// The guard's own error names the remedy — "cancel them first" — and a no_show
// has no way to reach cancelled: internal/bookings/handlers.go maps it to the
// empty set of transitions. So while a no_show counts as active, the owner is
// told to do something the product refuses, until the date falls behind
// CURRENT_DATE on its own.
func TestANoShowStopsBlockingTheDeletionOfTheCourtItReleased(t *testing.T) {
	f := newTestFixture(t)

	b := f.createBooking(t, bookingOptions{StartTime: "18:00", EndTime: "19:30"})

	assertBlocks := func(want bool, why string) {
		t.Helper()

		byCourt, err := f.Models.Bookings.HasActiveBookingsByCourt(context.Background(), f.CourtID)
		if err != nil {
			t.Fatalf("asking whether the court is busy: %v", err)
		}
		if byCourt != want {
			t.Errorf("HasActiveBookingsByCourt = %v, want %v — %s", byCourt, want, why)
		}

		byComplex, err := f.Models.Bookings.HasActiveBookings(context.Background(), f.ComplexID)
		if err != nil {
			t.Fatalf("asking whether the complex is busy: %v", err)
		}
		if byComplex != want {
			t.Errorf("HasActiveBookings = %v, want %v — %s", byComplex, want, why)
		}
	}

	assertBlocks(true, "a confirmed booking holds the court and someone is coming to it")

	markStatus(t, f, b.ID, "completed")
	assertBlocks(true, "a completed booking still took the court's hours; this half must not be lost")

	markStatus(t, f, b.ID, "no_show")
	assertBlocks(false, "a no_show gave the hours back, and it cannot be cancelled to satisfy the guard")
}

// The deliberate divergence, guarded. GetPaymentSummary reads the bookings
// table but it is a money query, and money does not behave like court time:
// hours cannot be sold twice, pesos can be collected twice. The absent client
// forfeited a deposit the venue kept, and the resale paid in full — the day's
// takings are both, and aligning this query to the occupancy predicate would
// erase the forfeit from the owner's cash view.
func TestTheDailyPaymentSummaryStillCountsWhatTheAbsentClientPaid(t *testing.T) {
	f := newTestFixture(t)

	first := f.createBooking(t, bookingOptions{
		StartTime: "18:00", EndTime: "19:30",
		CollectionStatus: data.CollectionStatusDepositPaid, RefundStatus: data.RefundStatusNone,
		Price: 500_000, DepositAmount: 150_000,
	})
	// createBooking only writes the bookings row; GetPaymentSummary INNER JOINs
	// payments, so the deposit itself has to be recorded through the payments
	// store the same way the real deposit flow would.
	f.createPayment(t, first.ID, 150_000, 0, nil)

	resale := noShowThenResale(t, f, first, bookingOptions{
		CollectionStatus: data.CollectionStatusFullyPaid, RefundStatus: data.RefundStatusNone,
		Price: 500_000, DepositAmount: 150_000,
	})
	f.createPayment(t, resale.ID, 500_000, 0, nil)

	// GetPaymentSummary is keyed by payment date, not booking date (see its doc
	// comment in bookings.go) — the dashboard always passes today. first.Date is
	// a week out, so passing it here would ask for a day nothing was paid on.
	summary, err := f.Models.Bookings.GetPaymentSummary(context.Background(), f.ComplexID, timezone.Today())
	if err != nil {
		t.Fatalf("reading the payment summary: %v", err)
	}

	forfeited := summary.ByStatus["deposit_paid"]
	if forfeited.Count != 1 || forfeited.Total != 150_000 {
		t.Errorf("the deposit the absent client forfeited is money the venue holds; the summary reported %+v", forfeited)
	}

	resold := summary.ByStatus["fully_paid"]
	if resold.Count != 1 || resold.Total != 500_000 {
		t.Errorf("the resale paid in full; the summary reported %+v", resold)
	}
}
