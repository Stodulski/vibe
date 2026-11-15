package bookings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

type freeCourtOut struct {
	CourtID   string `json:"court_id"`
	CourtName string `json:"court_name"`
}

// decodeFreeCourts is the free courts the 409 names for each conflict date.
func decodeFreeCourts(t *testing.T, w *httptest.ResponseRecorder) map[string][]freeCourtOut {
	t.Helper()
	var problem struct {
		Conflicts []struct {
			Date       string         `json:"date"`
			FreeCourts []freeCourtOut `json:"free_courts"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	out := map[string][]freeCourtOut{}
	for _, c := range problem.Conflicts {
		out[c.Date] = c.FreeCourts
	}
	return out
}

// altCourt adds a court of the fixture's complex, a clone of the chosen one
// unless the caller changes it.
func (f *seriesFx) altCourt(name string, change func(*courtstore.Court)) *courtstore.Court {
	c := *f.courts.court
	c.ID, c.Name = uuid.New(), name
	if change != nil {
		change(&c)
	}
	f.courts.all = append(f.courts.all, &c)
	return &c
}

func (f *seriesFx) occupyCourt(courtID uuid.UUID, d time.Time) {
	id := f.courtID
	f.courtID = courtID
	f.occupy(d, "18:30", 60)
	f.courtID = id
}

func dateCourts(d time.Time, courtID uuid.UUID) string {
	return `,"skip_conflicts":true,"date_courts":[{"date":"` + d.Format(time.DateOnly) + `","court_id":"` + courtID.String() + `"}]`
}

func TestAConflictedSeriesNamesTheCourtsFreeOnEachDateRankedBySimilarity(t *testing.T) {
	f := newSeriesFixture(t)
	grass, clay := "synthetic_grass", "clay"
	f.courts.court.Sport, f.courts.court.CourtType, f.courts.court.Surface = "padel", "outdoor", &grass
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	d14, d21 := first.AddDate(0, 0, 14), first.AddDate(0, 0, 21)
	f.occupy(d14, "18:30", 60)
	f.occupy(d21, "18:30", 60)

	other := f.altCourt("Court 2", func(c *courtstore.Court) { c.Surface = &clay })
	same := f.altCourt("Court 3", nil)
	f.occupyCourt(same.ID, d14)
	busyBoth := f.altCourt("Court 4", nil)
	f.occupyCourt(busyBoth.ID, d14)
	f.occupyCourt(busyBoth.ID, d21)
	f.altCourt("Court 5", func(c *courtstore.Court) { c.IsActive = false })
	f.altCourt("Court 6", func(c *courtstore.Court) { c.Sport = "tennis" })

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	got := decodeFreeCourts(t, w)
	want := map[string][]freeCourtOut{
		d14.Format(time.DateOnly): {{other.ID.String(), "Court 2"}},
		d21.Format(time.DateOnly): {{same.ID.String(), "Court 3"}, {other.ID.String(), "Court 2"}},
	}
	for date, courts := range want {
		if len(got[date]) != len(courts) {
			t.Fatalf("free courts on %s = %+v, want %+v", date, got[date], courts)
		}
		for i := range courts {
			if got[date][i] != courts[i] {
				t.Errorf("free courts on %s [%d] = %+v, want %+v", date, i, got[date][i], courts[i])
			}
		}
	}
}

func TestADateWithNoFreeCourtCarriesAnEmptyList(t *testing.T) {
	f := newSeriesFixture(t)
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	clash := first.AddDate(0, 0, 14)
	f.occupy(clash, "18:30", 60)
	busy := f.altCourt("Court 2", nil)
	f.occupyCourt(busy.ID, clash)

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", w.Code)
	}
	var raw struct {
		Conflicts []map[string]json.RawMessage `json:"conflicts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Conflicts) != 1 || string(raw.Conflicts[0]["free_courts"]) != "[]" {
		t.Errorf("free_courts must be an empty list; got %s", w.Body.String())
	}
	if json.Valid(w.Body.Bytes()) && containsKey(w.Body.Bytes(), "alternatives") {
		t.Errorf("the whole-series alternatives are gone; got %s", w.Body.String())
	}
}

func containsKey(body []byte, key string) bool {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(body, &m)
	_, ok := m[key]
	return ok
}

func TestDateCourtsBookThatDateOnTheOtherCourtAndLeaveTheRestAsExceptions(t *testing.T) {
	f := newSeriesFixture(t)
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	moved, left := first.AddDate(0, 0, 14), first.AddDate(0, 0, 21)
	f.occupy(moved, "18:30", 60)
	f.occupy(left, "18:30", 60)
	other := f.altCourt("Court 2", nil)

	w := f.create(t, first, dateCourts(moved, other.ID))
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	series := f.onlySeries(t)
	if series.CourtID != f.courtID {
		t.Errorf("the series stays on the chosen court; got %s", series.CourtID)
	}
	var onOther []*bookingstore.Booking
	for _, b := range f.store.inserted {
		if b.CourtID == other.ID {
			onOther = append(onOther, b)
		}
	}
	if len(onOther) != 1 || !onOther[0].Date.Equal(moved) || onOther[0].SeriesID == nil || *onOther[0].SeriesID != series.ID {
		t.Fatalf("want one booking of the series on the other court that day; got %+v", onOther)
	}
	if onOther[0].ClientID != f.clients.client.ID || onOther[0].StartTime != "18:00" || onOther[0].DurationMinutes != 90 {
		t.Errorf("the moved booking is the series' occurrence; got %+v", onOther[0])
	}
	if rec := f.ledger(t, moved); rec.Outcome != bookingstore.SeriesDateBooked {
		t.Errorf("the moved date is booked in the ledger; got %+v", rec)
	}
	for _, o := range f.series.occurrences(nil, []uuid.UUID{f.courtID}, moved, moved) {
		t.Errorf("the series must not hold the original court on the moved date: %+v", o)
	}
	if rec := f.ledger(t, left); rec.Outcome != bookingstore.SeriesDateException || rec.Reason == nil || *rec.Reason != ReasonOverlap {
		t.Errorf("a date left without a court becomes an exception; got %+v", rec)
	}
	body := decode(t, w)
	if n := len(body["bookings"].([]any)); n != 1 {
		t.Errorf("the moved booking is returned; got %d", n)
	}
	if n := len(body["skipped"].([]any)); n != 1 {
		t.Errorf("only the date left out is skipped; got %d", n)
	}
}

func TestDateCourtsRefuseWhenThePickedCourtBecameBusy(t *testing.T) {
	f := newSeriesFixture(t)
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	clash := first.AddDate(0, 0, 14)
	f.occupy(clash, "18:30", 60)
	other := f.altCourt("Court 2", nil)
	f.occupyCourt(other.ID, clash)

	w := f.create(t, first, dateCourts(clash, other.ID))
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if f.seriesCount() != 0 || len(f.store.inserted) != 0 {
		t.Errorf("nothing is written; %d series, %d bookings", f.seriesCount(), len(f.store.inserted))
	}
}

func TestDateCourtsRefuseABusyCourtTheStoreOnlySees(t *testing.T) {
	f := newSeriesFixture(t)
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	clash := first.AddDate(0, 0, 14)
	f.occupy(clash, "18:30", 60)
	other := f.altCourt("Court 2", nil)
	f.series.insertErr = bookingstore.ErrDuplicateBooking

	w := f.create(t, first, dateCourts(clash, other.ID))
	if w.Code != http.StatusConflict || f.seriesCount() != 0 {
		t.Fatalf("a booking that landed first is a 409 and writes nothing; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestDateCourtsMustNameAConflictDateAndACourtOfTheSameSport(t *testing.T) {
	f := newSeriesFixture(t)
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	clash := first.AddDate(0, 0, 14)
	f.occupy(clash, "18:30", 60)
	other := f.altCourt("Court 2", nil)
	tennis := f.altCourt("Court 3", func(c *courtstore.Court) { c.Sport = "tennis" })

	pick := `{"date":"` + clash.Format(time.DateOnly) + `","court_id":"` + other.ID.String() + `"}`
	twice := `,"skip_conflicts":true,"date_courts":[` + pick + `,` + pick + `]`

	for name, extra := range map[string]string{
		"a date nothing holds":   dateCourts(first.AddDate(0, 0, 7), other.ID),
		"another sport":          dateCourts(clash, tennis.ID),
		"the series' own court":  dateCourts(clash, f.courtID),
		"a court of nobody":      dateCourts(clash, uuid.New()),
		"the same date twice":    twice,
		"a date off the weekday": dateCourts(clash.AddDate(0, 0, 1), other.ID),
	} {
		w := f.create(t, first, extra)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %d (%s)", name, w.Code, w.Body.String())
		}
		if f.seriesCount() != 0 {
			t.Errorf("%s: nothing is written", name)
		}
	}
}
