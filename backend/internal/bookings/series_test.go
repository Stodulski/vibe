package bookings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// stubSeries is an in-memory SeriesStore. It keeps the one rule the database
// enforces with a primary key: a (series, date) pair is generated at most once.
type stubSeries struct {
	store  *stubStore
	series map[uuid.UUID]*bookingstore.Series
	dates  map[string]bookingstore.SeriesDate
}

func dateKey(id uuid.UUID, d time.Time) string { return id.String() + "|" + d.Format(time.DateOnly) }

func (s *stubSeries) InsertSeries(_ context.Context, ser *bookingstore.Series) error {
	ser.ID = uuid.New()
	ser.CreatedAt = time.Now()
	s.series[ser.ID] = ser
	return nil
}

func (s *stubSeries) GetSeries(_ context.Context, id uuid.UUID) (*bookingstore.Series, error) {
	ser, ok := s.series[id]
	if !ok {
		return nil, data.ErrRecordNotFound
	}
	copied := *ser
	return &copied, nil
}

func (s *stubSeries) ListActiveSeries(context.Context) ([]*bookingstore.Series, error) {
	var out []*bookingstore.Series
	for _, ser := range s.series {
		if ser.Status == bookingstore.SeriesStatusActive {
			copied := *ser
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (s *stubSeries) EndSeries(_ context.Context, id uuid.UUID, endsOn *time.Time, status string) (*bookingstore.Series, error) {
	ser, ok := s.series[id]
	if !ok {
		return nil, data.ErrRecordNotFound
	}
	ser.EndsOn, ser.Status = endsOn, status
	copied := *ser
	return &copied, nil
}

func (s *stubSeries) SeriesDates(_ context.Context, id uuid.UUID, from, to time.Time) ([]bookingstore.SeriesDate, error) {
	var out []bookingstore.SeriesDate
	for k, d := range s.dates {
		if strings.HasPrefix(k, id.String()) && !d.Date.Before(from) && !d.Date.After(to) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *stubSeries) RecordSeriesSkip(_ context.Context, ser *bookingstore.Series, d time.Time, reason string) error {
	if _, ok := s.dates[dateKey(ser.ID, d)]; !ok {
		s.dates[dateKey(ser.ID, d)] = bookingstore.SeriesDate{Date: d, Outcome: bookingstore.SeriesDateSkipped, Reason: &reason}
	}
	return nil
}

func (s *stubSeries) SeriesBookingsFrom(_ context.Context, id uuid.UUID, from time.Time) ([]*bookingstore.Booking, error) {
	var out []*bookingstore.Booking
	for _, b := range s.store.inserted {
		if b.SeriesID != nil && *b.SeriesID == id && !b.Date.Before(from) {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b *bookingstore.Booking) int { return a.Date.Compare(b.Date) })
	return out, nil
}

// claim is what the real insert does inside its transaction.
func (s *stubSeries) claim(b *bookingstore.Booking) error {
	if b.SeriesID == nil {
		return nil
	}
	k := dateKey(*b.SeriesID, b.Date)
	if _, ok := s.dates[k]; ok {
		return bookingstore.ErrSeriesDateGenerated
	}
	s.dates[k] = bookingstore.SeriesDate{Date: b.Date, Outcome: bookingstore.SeriesDateBooked}
	return nil
}

type seriesFx struct {
	*fixture
	series    *stubSeries
	complexID uuid.UUID
	courtID   uuid.UUID
}

func newSeriesFixture(t *testing.T) *seriesFx {
	t.Helper()
	f, complexID, courtID := staffFixture(t)
	f.complexes.complex = &complexstore.Complex{ID: complexID, CancellationHours: 24, Name: "Vibe", Slug: "vibe"}
	ss := &stubSeries{store: f.store, series: map[uuid.UUID]*bookingstore.Series{}, dates: map[string]bookingstore.SeriesDate{}}
	f.service.series = ss
	f.store.beforeInsert = func(b *bookingstore.Booking) error {
		// Overlap with a span held by someone else, like the exclusion constraint.
		start := slots.At(timezone.Day(b.Date), b.StartTime)
		end := start.Add(time.Duration(b.DurationMinutes) * time.Minute)
		for _, sp := range f.store.booked {
			if slots.OverlapAt(start, end, sp.StartsAt, sp.EndsAt) {
				return bookingstore.ErrDuplicateBooking
			}
		}
		return ss.claim(b)
	}
	return &seriesFx{fixture: f, series: ss, complexID: complexID, courtID: courtID}
}

func (f *seriesFx) day(offset int) time.Time { return today().AddDate(0, 0, offset) }

func (f *seriesFx) body(first time.Time, extra string) string {
	return fmt.Sprintf(`{"court_id":%q,"first_date":%q,"start_time":"18:00","duration_minutes":90,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000"%s}`,
		f.courtID, first.Format(time.DateOnly), extra)
}

func (f *seriesFx) create(t *testing.T, first time.Time, extra string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.CreateSeries(w, ownerRequest(t, http.MethodPost, "/", f.complexID,
		map[string]string{"id": f.complexID.String()}, f.body(first, extra)))
	return w
}

func (f *seriesFx) seriesCount() int { return len(f.series.series) }

func (f *seriesFx) occupy(d time.Time, from string, minutes int) {
	start := slots.At(timezone.Day(d), from)
	f.store.booked = append(f.store.booked, bookingstore.BookedSpan{
		CourtID: f.courtID, StartsAt: start, EndsAt: start.Add(time.Duration(minutes) * time.Minute),
	})
}

func (f *seriesFx) onlySeries(t *testing.T) *bookingstore.Series {
	t.Helper()
	for _, s := range f.series.series {
		return s
	}
	t.Fatal("no series was stored")
	return nil
}

func TestCreatingASeriesMaterializesEightWeeksWithoutNotifyingTheClient(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)

	w := f.create(t, first, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if n := len(body["bookings"].([]any)); n != 8 {
		t.Fatalf("want 8 occurrences, got %d", n)
	}
	if len(f.store.inserted) != 8 {
		t.Fatalf("want 8 stored bookings, got %d", len(f.store.inserted))
	}
	ser := f.onlySeries(t)
	if ser.Weekday != int(first.Weekday()) || ser.StartTime != "18:00" || ser.Status != bookingstore.SeriesStatusActive {
		t.Errorf("series stored wrongly: %+v", ser)
	}
	for i, b := range f.store.inserted {
		if b.SeriesID == nil || *b.SeriesID != ser.ID {
			t.Errorf("occurrence %d has no series_id", i)
		}
		if want := first.AddDate(0, 0, 7*i); !b.Date.Equal(want) {
			t.Errorf("occurrence %d on %s, want %s", i, b.Date.Format(time.DateOnly), want.Format(time.DateOnly))
		}
		if b.Price != 500_000*90/60 || b.CollectionStatus != bookingstore.CollectionStatusUnpaid {
			t.Errorf("occurrence %d priced %d / %s; want a normally priced unpaid booking", i, b.Price, b.CollectionStatus)
		}
	}
	if len(f.notify.confirmed) != 0 {
		t.Errorf("generated occurrences must not send a confirmation; %d were queued", len(f.notify.confirmed))
	}
	if got := body["bookings"].([]any)[0].(map[string]any)["series_id"]; got != ser.ID.String() {
		t.Errorf("the booking payload must carry series_id %s, got %v", ser.ID, got)
	}
}

func TestAManualPriceAppliesToEveryOccurrence(t *testing.T) {
	f := newSeriesFixture(t)
	if w := f.create(t, f.day(3), `,"price":123400`); w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	for _, b := range f.store.inserted {
		if b.Price != 123_400 {
			t.Fatalf("occurrence priced %d, want the manual 123400", b.Price)
		}
	}
}

func TestASeriesWithClashingDatesIsRefusedAndWritesNothing(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.occupy(first.AddDate(0, 0, 14), "18:30", 60) // week 3 overlaps 18:00-19:30

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	var problem struct {
		Conflicts []struct{ Date, Reason string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Conflicts) != 1 || problem.Conflicts[0].Reason != ReasonOverlap ||
		problem.Conflicts[0].Date != first.AddDate(0, 0, 14).Format(time.DateOnly) {
		t.Errorf("conflicts = %+v", problem.Conflicts)
	}
	if f.seriesCount() != 0 || len(f.store.inserted) != 0 || len(f.clients.created) != 0 || len(f.series.dates) != 0 {
		t.Error("a refused series must write nothing: no series, bookings, client or dates")
	}
}

func TestSkipConflictsCreatesTheRestAndRecordsTheSkippedDate(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.occupy(first.AddDate(0, 0, 14), "18:30", 60)

	w := f.create(t, first, `,"skip_conflicts":true`)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	skipped := body["skipped"].([]any)
	if len(f.store.inserted) != 7 || len(skipped) != 1 {
		t.Fatalf("want 7 created and 1 skipped; got %d and %d", len(f.store.inserted), len(skipped))
	}
	rec := f.series.dates[dateKey(f.onlySeries(t).ID, first.AddDate(0, 0, 14))]
	if rec.Outcome != bookingstore.SeriesDateSkipped || rec.Reason == nil || *rec.Reason != ReasonOverlap {
		t.Errorf("the skipped date must be recorded with its reason; got %+v", rec)
	}
}

func conflictReasons(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var problem struct {
		Conflicts []struct{ Date, Reason string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, c := range problem.Conflicts {
		out[c.Date] = c.Reason
	}
	return out
}

func TestABlockedSlotIsAConflict(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.courts.block(f.courtID, first.AddDate(0, 0, 7), "17:00", "19:00")

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if got := conflictReasons(t, w)[first.AddDate(0, 0, 7).Format(time.DateOnly)]; got != ReasonBlocked {
		t.Errorf("reason = %q, want %q", got, ReasonBlocked)
	}
}

func TestAClosedWeekdayMakesEveryDateAConflict(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	for _, sc := range f.complexes.schedules {
		if sc.Day == slots.DayName(first.Weekday()) {
			sc.IsClosed = true
		}
	}

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	reasons := conflictReasons(t, w)
	if len(reasons) != 8 {
		t.Fatalf("want all 8 dates refused, got %d", len(reasons))
	}
	for d, r := range reasons {
		if r != ReasonClosedDay {
			t.Errorf("%s: reason %q, want %q", d, r, ReasonClosedDay)
		}
	}
}

func TestADateNoPriceRuleCoversIsAConflictUnlessThePriceIsGiven(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.courts.prices = nil

	if w := f.create(t, first, ""); w.Code != http.StatusConflict ||
		conflictReasons(t, w)[first.Format(time.DateOnly)] != ReasonNoPrice {
		t.Fatalf("an unpriced hour must be a no_price conflict; got %d (%s)", w.Code, w.Body.String())
	}
	if w := f.create(t, first, `,"price":100000`); w.Code != http.StatusCreated {
		t.Fatalf("a manual price must book it; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestEndsOnAndWeeksBoundTheSeries(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)

	if w := f.create(t, first, `,"weeks":3`); w.Code != http.StatusCreated {
		t.Fatalf("weeks: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 3 {
		t.Fatalf("weeks=3 must create 3 occurrences, got %d", len(f.store.inserted))
	}
	if end := f.onlySeries(t).EndsOn; end == nil || !end.Equal(first.AddDate(0, 0, 14)) {
		t.Errorf("weeks=3 must end on the third occurrence, got %v", end)
	}

	g := newSeriesFixture(t)
	endsOn := first.AddDate(0, 0, 7).Format(time.DateOnly)
	if w := g.create(t, first, fmt.Sprintf(`,"ends_on":%q`, endsOn)); w.Code != http.StatusCreated {
		t.Fatalf("ends_on: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(g.store.inserted) != 2 {
		t.Fatalf("ends_on one week after the first date must create 2, got %d", len(g.store.inserted))
	}
}

func TestSeriesInputIsValidated(t *testing.T) {
	f := newSeriesFixture(t)
	cases := map[string]struct {
		first time.Time
		extra string
		field string
	}{
		"past first date": {f.day(-1), "", "first_date"},
		"both bounds":     {f.day(3), fmt.Sprintf(`,"weeks":2,"ends_on":%q`, f.day(20).Format(time.DateOnly)), "weeks"},
		"ends before":     {f.day(3), fmt.Sprintf(`,"ends_on":%q`, f.day(2).Format(time.DateOnly)), "ends_on"},
	}
	for name, c := range cases {
		w := f.create(t, c.first, c.extra)
		if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), c.field) {
			t.Errorf("%s: want 422 naming %s, got %d (%s)", name, c.field, w.Code, w.Body.String())
		}
	}
	if f.seriesCount() != 0 || len(f.store.inserted) != 0 {
		t.Error("invalid input must write nothing")
	}
}

func TestTheJobExtendsASeriesByOneWeekAndIsIdempotent(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	ctx := t.Context()

	f.service.extendSeries(ctx, f.day(0))
	if len(f.store.inserted) != 8 {
		t.Fatalf("a run on the creation day finds the window full; got %d bookings", len(f.store.inserted))
	}

	f.service.extendSeries(ctx, f.day(7))
	if len(f.store.inserted) != 9 {
		t.Fatalf("a week later the job adds exactly one occurrence; got %d", len(f.store.inserted))
	}
	last := f.store.inserted[8]
	if want := first.AddDate(0, 0, 56); !last.Date.Equal(want) {
		t.Errorf("the new occurrence is on %s, want %s", last.Date.Format(time.DateOnly), want.Format(time.DateOnly))
	}
	if last.SeriesID == nil {
		t.Error("the extension is part of the series")
	}

	f.service.extendSeries(ctx, f.day(7))
	f.service.extendSeries(ctx, f.day(7))
	if len(f.store.inserted) != 9 {
		t.Errorf("running again must not duplicate anything; got %d bookings", len(f.store.inserted))
	}
	if len(f.notify.confirmed) != 0 {
		t.Error("the job must not notify the client")
	}
}

func TestACancelledOccurrenceIsNotRecreatedByTheJob(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(3), "")

	cancelled := f.store.inserted[2]
	cancelled.Status = bookingstore.BookingStatusCancelled
	f.service.extendSeries(t.Context(), f.day(0))
	f.service.extendSeries(t.Context(), f.day(7))

	for _, b := range f.store.inserted {
		if b != cancelled && b.Date.Equal(cancelled.Date) {
			t.Fatalf("the cancelled date %s was booked again", cancelled.Date.Format(time.DateOnly))
		}
	}
	if len(f.store.inserted) != 9 {
		t.Errorf("want only the one new week, got %d bookings", len(f.store.inserted))
	}
}

func TestTheJobSkipsAndRecordsADateItCannotBook(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	f.occupy(first.AddDate(0, 0, 56), "18:00", 90)

	f.service.extendSeries(t.Context(), f.day(7))
	f.service.extendSeries(t.Context(), f.day(7))
	if len(f.store.inserted) != 8 {
		t.Fatalf("the clashing week must not be booked; got %d", len(f.store.inserted))
	}
	rec := f.series.dates[dateKey(f.onlySeries(t).ID, first.AddDate(0, 0, 56))]
	if rec.Outcome != bookingstore.SeriesDateSkipped {
		t.Errorf("the date must be recorded as skipped, got %+v", rec)
	}
}

func TestTheJobEndsASeriesWhoseLastDayPassed(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(3), `,"weeks":2`)

	f.service.extendSeries(t.Context(), f.day(30))
	if got := f.onlySeries(t).Status; got != bookingstore.SeriesStatusEnded {
		t.Errorf("status = %s, want ended", got)
	}
	if len(f.store.inserted) != 2 {
		t.Errorf("an ended series books nothing more; got %d", len(f.store.inserted))
	}
}

func (f *seriesFx) end(t *testing.T, complexID, seriesID uuid.UUID, from time.Time) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.EndSeries(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String(), "seriesID": seriesID.String()},
		fmt.Sprintf(`{"from_date":%q}`, from.Format(time.DateOnly))))
	return w
}

func TestEndingFromADateCancelsUnpaidOccurrencesAndKeepsPaidOnes(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	ser := f.onlySeries(t)
	paid := f.store.inserted[4]
	paid.CollectionStatus = bookingstore.CollectionStatusFullyPaid
	from := first.AddDate(0, 0, 21) // the fourth occurrence on

	w := f.end(t, f.complexID, ser.ID, from)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if n := len(body["cancelled"].([]any)); n != 4 {
		t.Errorf("want 4 cancelled (occurrences 3,5,6,7), got %d", n)
	}
	if n := len(body["kept"].([]any)); n != 1 {
		t.Errorf("want the paid occurrence kept, got %d", n)
	}
	for i, b := range f.store.inserted {
		wantCancelled := i >= 3 && b != paid
		if (b.Status == bookingstore.BookingStatusCancelled) != wantCancelled {
			t.Errorf("occurrence %d status %s (cancelled expected: %v)", i, b.Status, wantCancelled)
		}
	}
	if end := ser.EndsOn; end == nil || !end.Equal(from.AddDate(0, 0, -1)) {
		t.Errorf("ends_on = %v, want the day before from_date", end)
	}
	if ser.Status != bookingstore.SeriesStatusActive {
		t.Errorf("a paid occurrence still stands ahead, so the series is not ended yet; status %s", ser.Status)
	}
	if len(f.notify.cancelled) != 0 {
		t.Error("ending a series must not message the client per occurrence")
	}
}

func TestEndingASeriesWithNothingPaidMarksItEnded(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	ser := f.onlySeries(t)

	if w := f.end(t, f.complexID, ser.ID, first); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if ser.Status != bookingstore.SeriesStatusEnded || ser.EndsOn == nil || !ser.EndsOn.Equal(first.AddDate(0, 0, -1)) {
		t.Errorf("series = %+v, want ended the day before its first date", ser)
	}
	for _, b := range f.store.inserted {
		if b.Status != bookingstore.BookingStatusCancelled {
			t.Errorf("occurrence on %s survived", b.Date.Format(time.DateOnly))
		}
	}
	f.service.extendSeries(t.Context(), f.day(10))
	if len(f.store.inserted) != 8 {
		t.Error("an ended series must not be extended")
	}
}

func TestAnotherOwnerCannotReadOrEndASeries(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(3), "")
	ser := f.onlySeries(t)
	other := uuid.New()

	w := httptest.NewRecorder()
	f.handler.GetSeries(w, ownerRequest(t, http.MethodGet, "/", other,
		map[string]string{"id": other.String(), "seriesID": ser.ID.String()}, ""))
	if w.Code != http.StatusNotFound {
		t.Errorf("reading another complex's series: want 404, got %d", w.Code)
	}

	if w := f.end(t, other, ser.ID, f.day(5)); w.Code != http.StatusNotFound {
		t.Errorf("ending another complex's series: want 404, got %d", w.Code)
	}
	for _, b := range f.store.inserted {
		if b.Status == bookingstore.BookingStatusCancelled {
			t.Error("a foreign end request cancelled bookings")
		}
	}
	if ser.Status != bookingstore.SeriesStatusActive || ser.EndsOn != nil {
		t.Error("a foreign end request changed the series")
	}
}

func TestGetSeriesReturnsTheSeriesItsUpcomingOccurrencesAndSkippedDates(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.occupy(first.AddDate(0, 0, 7), "18:00", 90)
	f.create(t, first, `,"skip_conflicts":true`)
	ser := f.onlySeries(t)

	w := httptest.NewRecorder()
	f.handler.GetSeries(w, ownerRequest(t, http.MethodGet, "/", f.complexID,
		map[string]string{"id": f.complexID.String(), "seriesID": ser.ID.String()}, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if n := len(body["upcoming"].([]any)); n != 7 {
		t.Errorf("want 7 upcoming, got %d", n)
	}
	if n := len(body["skipped"].([]any)); n != 1 {
		t.Errorf("want 1 skipped, got %d", n)
	}
	if got := body["series"].(map[string]any)["weekday"]; got != float64(first.Weekday()) {
		t.Errorf("weekday = %v, want %d", got, first.Weekday())
	}
}
