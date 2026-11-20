package bookings

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// waitlistDays is the set of days handed to the waitlist, as dates.
func (f *seriesFx) waitlistDays() map[string]bool {
	out := map[string]bool{}
	for _, fr := range f.waitlist.freed {
		out[fr.Day.Format(time.DateOnly)] = true
	}
	return out
}

// freedStarts is the set of start instants handed to the freed-slot listener.
func (f *seriesFx) freedStarts() map[time.Time]bool {
	out := map[time.Time]bool{}
	for _, s := range f.freedSlots.slots {
		out[s.StartsAt] = true
	}
	return out
}

// An occurrence whose game is under way is not the series' to cancel: ending
// the series from today leaves it standing, and nobody is told its hours are
// free.
func TestEndingASeriesLeavesAnOccurrenceThatHasStarted(t *testing.T) {
	f := newSeriesFixture(t)
	// The fixture's now is 10:00; a 09:00 first date is booked at once and has started.
	if w := f.createAt(t, f.day(0), "09:00", ""); w.Code != http.StatusCreated {
		t.Fatalf("setup: %d (%s)", w.Code, w.Body.String())
	}
	ser := f.onlySeries(t)
	started := f.store.inserted[0]

	w := f.end(t, f.complexID, ser.ID, f.day(0))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if started.Status != bookingstore.BookingStatusConfirmed {
		t.Errorf("a started occurrence was cancelled: %s", started.Status)
	}
	if n := len(decode(t, w)["cancelled"].([]any)); n != 0 {
		t.Errorf("want nothing cancelled, got %d", n)
	}
	if f.waitlistDays()[f.day(0).Format(time.DateOnly)] || f.freedStarts()[started.StartsAt] {
		t.Error("hours already under way were announced as free")
	}
}

// Skipping a date puts its hours back on sale, so the people waiting for that
// day and the freed-slot listener hear about it, as they do for a cancelled
// booking.
func TestSkippingAFutureDateAnnouncesTheFreedSlot(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	ser := f.onlySeries(t)
	target := first.AddDate(0, 0, 7)

	if w := f.skip(t, f.complexID, ser.ID, target); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if !f.waitlistDays()[target.Format(time.DateOnly)] {
		t.Errorf("the waitlist for %s was not told; got %v", target.Format(time.DateOnly), f.waitlist.freed)
	}
	if !f.freedStarts()[slots.At(timezone.Day(target), "18:00")] {
		t.Errorf("the freed-slot listener was not told; got %v", f.freedSlots.slots)
	}
}

// The series is committed before its due first date is booked. A failure
// booking that date must not answer the owner with an error for a series that
// exists (a retry would then collide with it): the series stands, and the job
// books the date on its next run.
func TestAFailureBookingTheFirstDateLeavesTheSeriesForTheJob(t *testing.T) {
	f := newSeriesFixture(t)
	f.store.insertErr = errors.New("connection reset")

	w := f.create(t, f.day(0), "")
	if w.Code != http.StatusCreated {
		t.Fatalf("the series exists, so the answer is 201; got %d (%s)", w.Code, w.Body.String())
	}
	if f.seriesCount() != 1 {
		t.Fatalf("want the series stored; got %d", f.seriesCount())
	}

	f.store.insertErr = nil
	f.runJob(f.at(0, "10:30"))
	if len(f.store.inserted) != 1 {
		t.Errorf("the job must book the first date; got %d bookings", len(f.store.inserted))
	}
}

// The retry after a 409 names the dates the owner was shown. A date taken
// after that answer is a conflict the owner never saw, so it is refused again
// instead of being skipped silently.
func TestSkipDatesSkipOnlyTheConflictsTheOwnerSaw(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	seen, unseen := first.AddDate(0, 0, 7), first.AddDate(0, 0, 21)
	f.occupy(seen, "18:00", 90)
	f.occupy(unseen, "18:00", 90) // booked after the owner's 409

	skip := `,"skip_conflicts":true,"skip_dates":["` + seen.Format(time.DateOnly) + `"]`
	w := f.create(t, first, skip)
	if w.Code != http.StatusConflict {
		t.Fatalf("a conflict the owner never saw must refuse the retry; got %d (%s)", w.Code, w.Body.String())
	}
	if got := decodeConflicts(t, w); got[unseen.Format(time.DateOnly)] != ReasonOverlap {
		t.Errorf("the refusal must list the new conflict; got %v", got)
	}
	if f.seriesCount() != 0 {
		t.Fatal("nothing may be written")
	}

	both := `,"skip_conflicts":true,"skip_dates":["` + seen.Format(time.DateOnly) + `","` + unseen.Format(time.DateOnly) + `"]`
	if w := f.create(t, first, both); w.Code != http.StatusCreated {
		t.Fatalf("with every conflict seen the series is created; got %d (%s)", w.Code, w.Body.String())
	}
}

// A date the job could not book is lost to the client without anyone having
// decided it. The owner hears about it: one email per series and run, naming
// every date it lost and why.
func TestTheJobTellsTheOwnerWhichDatesASeriesLost(t *testing.T) {
	f := newSeriesFixture(t)
	f.complexes.complex.OwnerID = uuid.New()
	first := f.day(3)
	f.create(t, first, "")
	f.occupy(first, "18:30", 60) // somebody holds the hours on the day itself

	f.runJob(f.at(3, "00:05"))

	lost := f.notify.seriesDatesLost
	if len(lost) != 1 {
		t.Fatalf("want one notice for the series; got %d", len(lost))
	}
	got := lost[0]
	if got.OwnerID != f.complexes.complex.OwnerID.String() || got.SeriesID != f.onlySeries(t).ID.String() {
		t.Errorf("the notice must go to the owner about this series; got %+v", got)
	}
	if len(got.Dates) != 1 || !got.Dates[0].Date.Equal(first) || got.Dates[0].Reason != ReasonOverlap {
		t.Errorf("want the lost date with its reason; got %+v", got.Dates)
	}

	// A run that loses nothing says nothing.
	f.runJob(f.at(3, "06:00"))
	if len(f.notify.seriesDatesLost) != 1 {
		t.Errorf("a run that lost no date must not email; got %d notices", len(f.notify.seriesDatesLost))
	}
}

// A date whose booking was cancelled is no longer booked: the detail says it
// is released, and still names the booking so the owner can open it.
func TestGetSeriesDoesNotReportACancelledOccurrenceAsBooked(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(0), "") // today: booked at once
	ser := f.onlySeries(t)
	booking := f.store.inserted[0]
	booking.Status = bookingstore.BookingStatusCancelled

	detail, err := f.service.GetSeries(t.Context(), f.complexID, ser.ID)
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	today := detail.Upcoming[0]
	if today.State != SeriesStateException || today.Reason == nil || *today.Reason != ReasonCancelledByOwner {
		t.Errorf("a cancelled occurrence must read released; got %+v", today)
	}
	if today.BookingID == nil || *today.BookingID != booking.ID {
		t.Errorf("the released date must still name its booking; got %v", today.BookingID)
	}
}

// Ending a series frees the dates it held without a booking yet, and those are
// announced like any freed slot.
func TestEndingASeriesAnnouncesTheDatesItStopsHolding(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	ser := f.onlySeries(t)

	if w := f.end(t, f.complexID, ser.ID, first.AddDate(0, 0, 7)); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	days := f.waitlistDays()
	if days[first.Format(time.DateOnly)] {
		t.Error("a date the series still holds was announced")
	}
	for _, d := range []time.Time{first.AddDate(0, 0, 7), first.AddDate(0, 0, 14)} {
		if !days[d.Format(time.DateOnly)] {
			t.Errorf("the freed date %s was not announced; got %v", d.Format(time.DateOnly), f.waitlist.freed)
		}
		if !f.freedStarts()[slots.At(timezone.Day(d), "18:00")] {
			t.Errorf("the freed-slot listener did not hear about %s", d.Format(time.DateOnly))
		}
	}
}
