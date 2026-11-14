package bookings

import (
	"net/http"
	"testing"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

func withClientEmail(f *seriesFx, email string) {
	f.clients.client.Email = &email
}

func TestCreatingASeriesQueuesOneNoticeForTheClient(t *testing.T) {
	f := newSeriesFixture(t)
	withClientEmail(f, "ana@example.com")
	first := f.day(7) // a Monday

	if w := f.create(t, first, `,"ends_on":"`+f.day(70).Format("2006-01-02")+`"`); w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.seriesCreated) != 1 {
		t.Fatalf("want exactly one creation notice, got %d", len(f.notify.seriesCreated))
	}
	c := f.notify.seriesCreated[0]
	if c.Email != "ana@example.com" || c.CourtName != "Court 1" ||
		c.Weekday.String() != "Monday" || c.Hours != "18:00 a 19:30" ||
		c.From != first.Format("02/01") || c.Until != f.day(70).Format("02/01") || c.SeriesID != f.onlySeries(t).ID.String() {
		t.Errorf("notice = %+v", c)
	}
	if len(f.notify.confirmed) != 0 || len(f.notify.cancelled) != 0 || len(f.notify.seriesCancelled) != 0 {
		t.Error("creating a series sends only the creation notice")
	}
}

func TestACreationNoticeNeedsAnEmail(t *testing.T) {
	f := newSeriesFixture(t)
	if w := f.create(t, f.day(7), ""); w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	if len(f.notify.seriesCreated) != 0 {
		t.Errorf("a client with no email gets no creation notice; got %d", len(f.notify.seriesCreated))
	}
}

func TestARefusedSeriesSendsNoNotice(t *testing.T) {
	f := newSeriesFixture(t)
	withClientEmail(f, "ana@example.com")
	f.occupy(f.day(14), "18:00", 90)
	if w := f.create(t, f.day(7), ""); w.Code == http.StatusCreated {
		t.Fatal("setup: the series should collide")
	}
	if len(f.notify.seriesCreated) != 0 {
		t.Error("a series that was not created must not be announced")
	}
}

func TestEndingASeriesQueuesOneNoticeNotOnePerBooking(t *testing.T) {
	f := newSeriesFixture(t)
	withClientEmail(f, "ana@example.com")
	f.clients.client.Phone = "+5411555"
	f.create(t, f.day(0), "")
	ser := f.onlySeries(t)
	f.runJob(f.at(7, "00:05"))
	f.runJob(f.at(14, "00:05"))

	if w := f.end(t, f.complexID, ser.ID, f.day(7)); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.seriesCancelled) != 1 || len(f.notify.cancelled) != 0 {
		t.Fatalf("want one series notice and no per-booking one; got %d / %d", len(f.notify.seriesCancelled), len(f.notify.cancelled))
	}
	c := f.notify.seriesCancelled[0]
	if c.Email != "ana@example.com" || c.Phone != "+5411555" || c.From != f.day(7).Format("02/01") ||
		c.Weekday.String() != "Monday" || c.Hours != "18:00 a 19:30" || c.KeptPaid || c.BookPath == "" {
		t.Errorf("notice = %+v", c)
	}
}

func TestEndingASeriesWithPaidBookingsSaysTheyStand(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(0), "")
	ser := f.onlySeries(t)
	f.runJob(f.at(7, "00:05"))
	f.store.inserted[1].CollectionStatus = bookingstore.CollectionStatusFullyPaid

	f.end(t, f.complexID, ser.ID, f.day(7))
	if len(f.notify.seriesCancelled) != 1 || !f.notify.seriesCancelled[0].KeptPaid {
		t.Errorf("want one notice with KeptPaid; got %+v", f.notify.seriesCancelled)
	}
}

// The series itself was cancelled, so the client is told even when no booking
// existed yet for the dates it stops holding.
func TestEndingASeriesWithNothingBookedStillNotifies(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(7), "")
	ser := f.onlySeries(t)

	f.end(t, f.complexID, ser.ID, f.day(7))
	if len(f.notify.seriesCancelled) != 1 {
		t.Errorf("want one notice, got %d", len(f.notify.seriesCancelled))
	}
}

func TestTheCancellationNamesTheFirstDateOfTheSeriesFromTheOnePicked(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(7), "") // Mondays
	ser := f.onlySeries(t)

	f.end(t, f.complexID, ser.ID, f.day(9)) // a Wednesday
	got := f.notify.seriesCancelled[0].From
	if want := f.day(14).Format("02/01"); got != want {
		t.Errorf("From = %s, want the next Monday %s", got, want)
	}
}

func TestARefusedEndSendsNoNotice(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(7), "")
	ser := f.onlySeries(t)
	if w := f.end(t, f.complexID, ser.ID, f.day(-1)); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("setup: want 422, got %d", w.Code)
	}
	if len(f.notify.seriesCancelled) != 0 {
		t.Error("a refused end must not notify")
	}
}
