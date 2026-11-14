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

// stubSeries is an in-memory SeriesStore. It keeps the rule the database
// enforces with a primary key — a (series, date) pair is settled at most once —
// and expands the rule into the dates a series holds the way the SQL function
// booking_series_occurrences does (the store's integration tests hold the two
// to the same answer).
type stubSeries struct {
	store  *stubStore
	series map[uuid.UUID]*bookingstore.Series
	dates  map[string]bookingstore.SeriesDate
	// insertErr fails InsertSeries, the way the in-transaction recheck does when
	// a booking slipped in.
	insertErr error
}

func dateKey(id uuid.UUID, d time.Time) string { return id.String() + "|" + d.Format(time.DateOnly) }

func (s *stubSeries) InsertSeries(_ context.Context, ser *bookingstore.Series, exceptions []bookingstore.SeriesException) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	ser.ID = uuid.New()
	ser.CreatedAt = time.Now()
	s.series[ser.ID] = ser
	for _, e := range exceptions {
		reason := e.Reason
		s.dates[dateKey(ser.ID, e.Date)] = bookingstore.SeriesDate{Date: e.Date, Outcome: bookingstore.SeriesDateException, Reason: &reason}
	}
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

func (s *stubSeries) EndExpiredSeries(_ context.Context, day time.Time) (int64, error) {
	var n int64
	for _, ser := range s.series {
		if ser.Status == bookingstore.SeriesStatusActive && ser.EndsOn != nil && ser.EndsOn.Before(day) {
			ser.Status = bookingstore.SeriesStatusEnded
			n++
		}
	}
	return n, nil
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
	slices.SortFunc(out, func(a, b bookingstore.SeriesDate) int { return a.Date.Compare(b.Date) })
	return out, nil
}

func (s *stubSeries) RecordSeriesException(_ context.Context, ser *bookingstore.Series, d time.Time, reason string) (bool, error) {
	if _, ok := s.dates[dateKey(ser.ID, d)]; ok {
		return false, nil
	}
	s.dates[dateKey(ser.ID, d)] = bookingstore.SeriesDate{Date: d, Outcome: bookingstore.SeriesDateException, Reason: &reason}
	return true, nil
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

// occurrences is SeriesOccurrences: every date of an active series in the
// window that has no ledger row.
func (s *stubSeries) occurrences(_ *uuid.UUID, courtIDs []uuid.UUID, from, to time.Time) []bookingstore.SeriesOccurrence {
	var out []bookingstore.SeriesOccurrence
	for _, ser := range s.series {
		if ser.Status != bookingstore.SeriesStatusActive {
			continue
		}
		if len(courtIDs) > 0 && !slices.Contains(courtIDs, ser.CourtID) {
			continue
		}
		for _, d := range ruleDates(ser, from, to) {
			if _, settled := s.dates[dateKey(ser.ID, d)]; settled {
				continue
			}
			start := slots.At(timezone.Day(d), ser.StartTime)
			out = append(out, bookingstore.SeriesOccurrence{
				SeriesID: ser.ID, ComplexID: ser.ComplexID, CourtID: ser.CourtID, Date: d,
				StartsAt: start, EndsAt: start.Add(time.Duration(ser.DurationMinutes) * time.Minute),
				ClientName: "Ana Perez",
			})
		}
	}
	slices.SortFunc(out, func(a, b bookingstore.SeriesOccurrence) int { return a.StartsAt.Compare(b.StartsAt) })
	return out
}

// seriesNow is the instant every series test runs at: a Monday, 10:00 at the
// venue. Fixed so that "today", "due" and "already over" mean the same on any
// day the suite runs.
var seriesNow = time.Date(2026, time.November, 16, 10, 0, 0, 0, timezone.Argentina)

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
	f.service.now = func() time.Time { return seriesNow }
	f.store.occurrencesFn = ss.occurrences
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

// day is the date offset days from the fixture's today (Monday 2026-11-16).
func (f *seriesFx) day(offset int) time.Time { return f.service.today().AddDate(0, 0, offset) }

// at is the instant offset days from now, at the given clock time.
func (f *seriesFx) at(offset int, hhmm string) time.Time {
	return slots.At(timezone.Day(f.day(offset)), hhmm)
}

func (f *seriesFx) body(first time.Time, startTime, extra string) string {
	return fmt.Sprintf(`{"court_id":%q,"first_date":%q,"start_time":%q,"duration_minutes":90,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000"%s}`,
		f.courtID, first.Format(time.DateOnly), startTime, extra)
}

func (f *seriesFx) createAt(t *testing.T, first time.Time, startTime, extra string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.CreateSeries(w, ownerRequest(t, http.MethodPost, "/", f.complexID,
		map[string]string{"id": f.complexID.String()}, f.body(first, startTime, extra)))
	return w
}

func (f *seriesFx) create(t *testing.T, first time.Time, extra string) *httptest.ResponseRecorder {
	t.Helper()
	return f.createAt(t, first, "18:00", extra)
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

func (f *seriesFx) ledger(t *testing.T, d time.Time) bookingstore.SeriesDate {
	t.Helper()
	return f.series.dates[dateKey(f.onlySeries(t).ID, d)]
}

// runJob runs the materialization job at the given instant.
func (f *seriesFx) runJob(at time.Time) { f.service.materializeSeries(context.Background(), at) }

func decodeConflicts(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
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

func TestRuleDatesFollowTheWeekdayWithinTheSeriesLife(t *testing.T) {
	ser := &Series{Weekday: int(time.Wednesday), StartsOn: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)}
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 0, 0, 0, 0, time.UTC) }

	got := ruleDates(ser, d(11, 1), d(12, 10))
	want := []time.Time{d(11, 18), d(11, 25), d(12, 2), d(12, 9)}
	if !slices.EqualFunc(got, want, time.Time.Equal) {
		t.Errorf("never before starts_on, always on the weekday; got %v, want %v", got, want)
	}

	end := d(12, 2)
	ser.EndsOn = &end
	if got := ruleDates(ser, d(11, 19), d(12, 31)); !slices.EqualFunc(got, []time.Time{d(11, 25), d(12, 2)}, time.Time.Equal) {
		t.Errorf("never after ends_on, and the window may start mid-series; got %v", got)
	}
	if onRule(ser, d(12, 9)) || !onRule(ser, d(12, 2)) || onRule(ser, d(11, 24)) {
		t.Error("onRule must follow ends_on and the weekday")
	}
}

func TestCreatingASeriesBooksNothingAheadAndHoldsItsDates(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)

	w := f.create(t, first, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if n := len(body["bookings"].([]any)); n != 0 {
		t.Errorf("nothing is booked ahead; got %d bookings", n)
	}
	if len(f.store.inserted) != 0 {
		t.Fatalf("no booking row may exist for future dates; got %d", len(f.store.inserted))
	}
	ser := f.onlySeries(t)
	if ser.Weekday != int(first.Weekday()) || ser.StartTime != "18:00" || ser.Status != bookingstore.SeriesStatusActive ||
		ser.EndsOn != nil || !ser.StartsOn.Equal(first) {
		t.Errorf("series stored wrongly: %+v", ser)
	}
	if len(f.notify.confirmed) != 0 {
		t.Errorf("creating a series must not send a confirmation; %d were queued", len(f.notify.confirmed))
	}
	// Held for as far ahead as anyone looks.
	held := f.series.occurrences(nil, nil, f.day(20*7), f.day(20*7+6))
	if len(held) != 1 || !held[0].Date.Equal(first.AddDate(0, 0, 20*7)) {
		t.Errorf("the rule must hold its dates months ahead; got %+v", held)
	}
}

func TestAFirstDateThatIsDueIsBookedAtOnce(t *testing.T) {
	f := newSeriesFixture(t)

	w := f.create(t, f.day(0), "") // today 18:00, now is 10:00
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if n := len(decode(t, w)["bookings"].([]any)); n != 1 || len(f.store.inserted) != 1 {
		t.Fatalf("today's occurrence must be booked at once; got %d / %d", n, len(f.store.inserted))
	}
	b := f.store.inserted[0]
	if b.SeriesID == nil || *b.SeriesID != f.onlySeries(t).ID || !b.Date.Equal(f.day(0)) {
		t.Errorf("booking = %+v", b)
	}
	if got := f.ledger(t, f.day(0)); got.Outcome != bookingstore.SeriesDateBooked {
		t.Errorf("the date must be settled as booked; got %+v", got)
	}
	if len(f.notify.confirmed) != 0 {
		t.Error("the occurrence must not notify the client")
	}
}

func TestAFirstOccurrenceThatIsAlreadyOverIsRefused(t *testing.T) {
	f := newSeriesFixture(t)
	w := f.createAt(t, f.day(0), "09:00", "") // 09:00-10:30 and it is 10:00... ends 10:30 > now: allowed
	if w.Code != http.StatusCreated {
		t.Fatalf("an occurrence still running is fine; got %d (%s)", w.Code, w.Body.String())
	}
	g := newSeriesFixture(t)
	w = g.createAt(t, g.day(0), "08:00", "") // ended 09:30
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "first_date") {
		t.Fatalf("want 422 on first_date; got %d (%s)", w.Code, w.Body.String())
	}
	if g.seriesCount() != 0 {
		t.Error("a refused series writes nothing")
	}
}

func TestAManualPriceAppliesToEveryOccurrence(t *testing.T) {
	f := newSeriesFixture(t)
	if w := f.create(t, f.day(0), `,"price":123400`); w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	f.runJob(f.at(7, "00:30")) // the next Monday's evening is due on its own day
	f.runJob(f.at(7, "10:00"))
	if len(f.store.inserted) != 2 {
		t.Fatalf("want today's and next week's occurrence; got %d", len(f.store.inserted))
	}
	for _, b := range f.store.inserted {
		if b.Price != 123_400 {
			t.Fatalf("occurrence priced %d, want the manual 123400", b.Price)
		}
	}
}

func TestASeriesCollidingWithABookingTenWeeksAheadIsRefusedAndWritesNothing(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.occupy(first.AddDate(0, 0, 70), "18:30", 60) // week 11 overlaps 18:00-19:30

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	conflicts := decodeConflicts(t, w)
	if len(conflicts) != 1 || conflicts[first.AddDate(0, 0, 70).Format(time.DateOnly)] != ReasonOverlap {
		t.Errorf("conflicts = %+v", conflicts)
	}
	if f.seriesCount() != 0 || len(f.store.inserted) != 0 || len(f.clients.created) != 0 || len(f.series.dates) != 0 {
		t.Error("a refused series must write nothing: no series, bookings, client or dates")
	}
}

func TestASeriesCollidingWithABlockedSlotThirtyWeeksAheadIsRefused(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	blockedDay := first.AddDate(0, 0, 30*7)
	f.courts.block(f.courtID, blockedDay, "17:00", "19:00")

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if got := decodeConflicts(t, w)[blockedDay.Format(time.DateOnly)]; got != ReasonBlocked {
		t.Errorf("reason = %q, want %q", got, ReasonBlocked)
	}
}

func TestAnotherSeriesHoldingTheHoursIsAConflictToo(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	start := slots.At(timezone.Day(first.AddDate(0, 0, 14)), "18:00")
	f.store.booked = append(f.store.booked, bookingstore.BookedSpan{
		CourtID: f.courtID, StartsAt: start, EndsAt: start.Add(90 * time.Minute), Fixed: true,
	})
	w := f.create(t, first, "")
	if w.Code != http.StatusConflict || decodeConflicts(t, w)[first.AddDate(0, 0, 14).Format(time.DateOnly)] != ReasonOverlap {
		t.Fatalf("a hold by another series must conflict; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestSkipConflictsCreatesTheSeriesWithTheCollidingDatesAsExceptions(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	clash := first.AddDate(0, 0, 70)
	f.occupy(clash, "18:30", 60)

	w := f.create(t, first, `,"skip_conflicts":true`)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if n := len(decode(t, w)["skipped"].([]any)); n != 1 {
		t.Fatalf("want 1 skipped, got %d", n)
	}
	rec := f.ledger(t, clash)
	if rec.Outcome != bookingstore.SeriesDateException || rec.Reason == nil || *rec.Reason != ReasonOverlap {
		t.Errorf("the colliding date must be recorded as an exception with its reason; got %+v", rec)
	}
	// The exception frees the date: the rule no longer holds it.
	for _, o := range f.series.occurrences(nil, nil, clash, clash) {
		t.Errorf("an excepted date must not hold its hours: %+v", o)
	}
}

func TestABookingMadeWhileTheSeriesIsBeingCreatedIsAConflict(t *testing.T) {
	f := newSeriesFixture(t)
	f.series.insertErr = bookingstore.ErrSlotUnavailable

	w := f.create(t, f.day(3), "")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "try again") {
		t.Fatalf("want 409 asking to retry; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestAClosedWeekdayOrAnUnpricedHourRefusesTheSeriesAsAWhole(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	for _, sc := range f.complexes.schedules {
		if sc.Day == slots.DayName(first.Weekday()) {
			sc.IsClosed = true
		}
	}
	if w := f.create(t, first, ""); w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "first_date") {
		t.Fatalf("a closed weekday must be a 422 on first_date; got %d (%s)", w.Code, w.Body.String())
	}

	g := newSeriesFixture(t)
	g.courts.prices = nil
	if w := g.create(t, g.day(3), ""); w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "price") {
		t.Fatalf("an unpriced hour must be a 422 on price; got %d (%s)", w.Code, w.Body.String())
	}
	if w := g.create(t, g.day(3), `,"price":100000`); w.Code != http.StatusCreated {
		t.Fatalf("a manual price must create it; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestEndsOnAndWeeksBoundTheSeries(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)

	if w := f.create(t, first, `,"weeks":3`); w.Code != http.StatusCreated {
		t.Fatalf("weeks: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if end := f.onlySeries(t).EndsOn; end == nil || !end.Equal(first.AddDate(0, 0, 14)) {
		t.Errorf("weeks=3 must end on the third occurrence, got %v", end)
	}
	if held := f.series.occurrences(nil, nil, first, first.AddDate(1, 0, 0)); len(held) != 3 {
		t.Errorf("weeks=3 holds 3 dates, got %d", len(held))
	}

	g := newSeriesFixture(t)
	endsOn := first.AddDate(0, 0, 7).Format(time.DateOnly)
	if w := g.create(t, first, fmt.Sprintf(`,"ends_on":%q`, endsOn)); w.Code != http.StatusCreated {
		t.Fatalf("ends_on: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if held := g.series.occurrences(nil, nil, first, first.AddDate(1, 0, 0)); len(held) != 2 {
		t.Errorf("ends_on one week after the first date holds 2, got %d", len(held))
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
		"too far ahead":   {f.day(bookingstore.MaxBookingHorizonDays + 1), "", "first_date"},
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

// ─── What the series holds, seen by everyone who sells or blocks hours ───────

func (f *seriesFx) ownerBook(t *testing.T, date time.Time, startTime string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", f.complexID,
		map[string]string{"id": f.complexID.String()}, staffBookBodyOn(f.courtID, date, startTime)))
	return w
}

func TestTheOwnerCannotBookHoursAFixedBookingHoldsButCanOnceTheDateIsSkipped(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	months := first.AddDate(0, 0, 20*7)

	w := f.ownerBook(t, months, "18:00")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "fixed weekly booking") {
		t.Fatalf("want 409 naming the fixed booking; got %d (%s)", w.Code, w.Body.String())
	}
	if w := f.ownerBook(t, months, "19:30"); w.Code != http.StatusCreated {
		t.Errorf("hours right after the series' are free; got %d (%s)", w.Code, w.Body.String())
	}
	if w := f.ownerBook(t, months.AddDate(0, 0, 1), "18:00"); w.Code != http.StatusCreated {
		t.Errorf("another weekday is free; got %d (%s)", w.Code, w.Body.String())
	}

	skip := f.skip(t, f.complexID, f.onlySeries(t).ID, months)
	if skip.Code != http.StatusOK {
		t.Fatalf("skip: want 200, got %d (%s)", skip.Code, skip.Body.String())
	}
	if w := f.ownerBook(t, months, "18:00"); w.Code != http.StatusCreated {
		t.Errorf("a skipped date is free again; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestTheOwnerCanBookAfterTheSeriesEnds(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, `,"weeks":2`) // holds first and first+7
	if w := f.ownerBook(t, first.AddDate(0, 0, 7), "18:00"); w.Code != http.StatusConflict {
		t.Fatalf("the last date is held; got %d", w.Code)
	}
	if w := f.ownerBook(t, first.AddDate(0, 0, 14), "18:00"); w.Code != http.StatusCreated {
		t.Errorf("after ends_on the hours are free; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestTheStorefrontCannotSellHoursAFixedBookingHolds(t *testing.T) {
	f := newSeriesFixture(t)
	complexID, courtID := preparePublicBooking(f.fixture)
	f.complexID, f.courtID = complexID, courtID
	date, _ := bookableDate()
	date = seriesDay(date)
	f.service.now = time.Now // the storefront path reads the real clock too

	if w := f.createAt(t, date, "18:00", ""); w.Code != http.StatusCreated {
		t.Fatalf("series: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	req := publicSlotRequest{ComplexID: complexID, CourtID: courtID, Date: date, StartTime: "18:00", DurationMinutes: 90}
	complex, court := f.complexes.complex, f.courts.court
	if _, err := f.service.publicOffer(t.Context(), complex, court, req); err == nil {
		t.Fatal("the storefront sold hours a fixed booking holds")
	}
	req.Date = date.AddDate(0, 0, 1)
	if _, err := f.service.publicOffer(t.Context(), complex, court, req); err != nil {
		t.Errorf("another day must stay on sale: %v", err)
	}

	f.series.dates[dateKey(f.onlySeries(t).ID, date)] = bookingstore.SeriesDate{Date: date, Outcome: bookingstore.SeriesDateException}
	req.Date = date
	if _, err := f.service.publicOffer(t.Context(), complex, court, req); err != nil {
		t.Errorf("a skipped date must be on sale again: %v", err)
	}
}

// ─── The materialization job ────────────────────────────────────────────────

func TestTheJobBooksOnlyTodaysOccurrenceAtTheCourtPricesOfThatDay(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")

	// The owner reprices the court between creating the series and its day.
	f.courts.prices = nil
	openAllWeek(f.fixture, f.courtID, "09:00", "23:00")
	for _, p := range f.courts.prices {
		p.Price = 600_000
	}

	f.runJob(f.at(2, "10:00")) // the day before: 18:00 is not due yet
	if len(f.store.inserted) != 0 {
		t.Fatalf("the day before nothing is booked; got %d", len(f.store.inserted))
	}
	f.runJob(f.at(3, "00:05")) // the first run of its day
	if len(f.store.inserted) != 1 {
		t.Fatalf("on its own day exactly one booking; got %d", len(f.store.inserted))
	}
	b := f.store.inserted[0]
	if !b.Date.Equal(first) || b.SeriesID == nil || b.Price != 600_000*90/60 {
		t.Errorf("booking = date %s series %v price %d; want today's price 900000", b.Date.Format(time.DateOnly), b.SeriesID, b.Price)
	}
	if b.Status != bookingstore.BookingStatusConfirmed || b.CollectionStatus != bookingstore.CollectionStatusUnpaid {
		t.Errorf("an occurrence is a confirmed, unpaid booking; got %s / %s", b.Status, b.CollectionStatus)
	}
	if len(f.notify.confirmed) != 0 {
		t.Error("the job must not notify the client")
	}

	// Idempotent: the date is settled, so later runs add nothing.
	f.runJob(f.at(3, "01:05"))
	f.runJob(f.at(3, "12:00"))
	if len(f.store.inserted) != 1 {
		t.Errorf("two more runs must not duplicate the booking; got %d", len(f.store.inserted))
	}
	// And it is still the only one a week later: next Monday's is for next Monday.
	f.runJob(f.at(6, "12:00"))
	if len(f.store.inserted) != 1 {
		t.Errorf("nothing else is due; got %d", len(f.store.inserted))
	}
}

func TestTheJobBooksAnEarlyMorningOccurrenceTheEveningBefore(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	if w := f.createAt(t, first, "00:30", `,"price":100000`); w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	// 00:30 on day 3 is created during day 2, long enough ahead for the 2-hour
	// reminder (which needs a booking older than two hours).
	f.runJob(f.at(2, "09:00"))
	if len(f.store.inserted) != 1 || !f.store.inserted[0].Date.Equal(first) {
		t.Fatalf("a 00:30 occurrence is due the evening before; got %d bookings", len(f.store.inserted))
	}
}

func TestTheJobRecordsAnExceptionWhenTheSlotGotTaken(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	f.occupy(first, "18:30", 60) // somebody holds the hours on the day itself

	f.runJob(f.at(3, "00:05"))
	f.runJob(f.at(3, "06:00"))
	if len(f.store.inserted) != 0 {
		t.Fatalf("the clashing date must not be booked; got %d", len(f.store.inserted))
	}
	rec := f.ledger(t, first)
	if rec.Outcome != bookingstore.SeriesDateException || rec.Reason == nil || *rec.Reason != ReasonOverlap {
		t.Errorf("the date must become an exception, once; got %+v", rec)
	}
}

func TestTheJobSettlesADateWhoseHoursPassedAsMissed(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")

	f.runJob(f.at(3, "22:00")) // the job was down all day
	if len(f.store.inserted) != 0 {
		t.Fatalf("a finished occurrence is not booked afterwards; got %d", len(f.store.inserted))
	}
	if rec := f.ledger(t, first); rec.Reason == nil || *rec.Reason != ReasonMissed {
		t.Errorf("the date must be settled as missed; got %+v", rec)
	}
}

func TestTheJobRecordsAClosedDayAndEndsExpiredSeries(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, `,"weeks":1`)
	for _, sc := range f.complexes.schedules {
		if sc.Day == slots.DayName(first.Weekday()) {
			sc.IsClosed = true // closed since the series was made
		}
	}
	f.runJob(f.at(3, "00:05"))
	if rec := f.ledger(t, first); rec.Reason == nil || *rec.Reason != ReasonClosedDay {
		t.Errorf("a day the venue closed must become a closed_day exception; got %+v", rec)
	}
	if got := f.onlySeries(t).Status; got != bookingstore.SeriesStatusActive {
		t.Errorf("the series lives until its last day has passed; status %s", got)
	}
	f.runJob(f.at(4, "00:05"))
	if got := f.onlySeries(t).Status; got != bookingstore.SeriesStatusEnded {
		t.Errorf("status = %s, want ended once the last day passed", got)
	}
}

func TestACancelledOccurrenceIsNotRecreatedByTheJob(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(0)
	f.create(t, first, "")
	if len(f.store.inserted) != 1 {
		t.Fatalf("setup: today's occurrence must exist, got %d", len(f.store.inserted))
	}
	f.store.inserted[0].Status = bookingstore.BookingStatusCancelled

	f.runJob(f.at(0, "10:30"))
	f.runJob(f.at(0, "12:00"))
	if len(f.store.inserted) != 1 {
		t.Errorf("the cancelled date was booked again; got %d bookings", len(f.store.inserted))
	}
	if held := f.series.occurrences(nil, nil, first, first); len(held) != 0 {
		t.Error("a cancelled occurrence is settled: it must not hold its hours as a rule either")
	}
}

// ─── Skipping one date ──────────────────────────────────────────────────────

func (f *seriesFx) skip(t *testing.T, complexID, seriesID uuid.UUID, date time.Time) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.SkipSeriesDate(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String(), "seriesID": seriesID.String()},
		fmt.Sprintf(`{"date":%q}`, date.Format(time.DateOnly))))
	return w
}

func TestSkippingADateRecordsAnExceptionAndFreesTheHours(t *testing.T) {
	f := newSeriesFixture(t)
	first := f.day(3)
	f.create(t, first, "")
	ser := f.onlySeries(t)
	target := first.AddDate(0, 0, 14)

	w := f.skip(t, f.complexID, ser.ID, target)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	got := decode(t, w)["skipped"].(map[string]any)
	if got["date"] != target.Format(time.DateOnly) || got["reason"] != ReasonCancelledByOwner {
		t.Errorf("skipped = %v", got)
	}
	if rec := f.ledger(t, target); rec.Outcome != bookingstore.SeriesDateException || *rec.Reason != ReasonCancelledByOwner {
		t.Errorf("ledger = %+v", rec)
	}
	for _, o := range f.series.occurrences(nil, nil, target, target) {
		t.Errorf("the skipped date still holds its hours: %+v", o)
	}
	if held := f.series.occurrences(nil, nil, target.AddDate(0, 0, 7), target.AddDate(0, 0, 7)); len(held) != 1 {
		t.Error("the other dates of the series keep holding theirs")
	}
}

func TestSkippingIsRefusedForPastNonSeriesBookedAndAlreadySkippedDates(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(0), "") // today's occurrence is booked at once
	ser := f.onlySeries(t)
	next := f.day(7)

	cases := map[string]time.Time{
		"past":            f.day(-7),
		"not a rule date": next.AddDate(0, 0, 1),
		"already booked":  f.day(0),
	}
	for name, d := range cases {
		w := f.skip(t, f.complexID, ser.ID, d)
		if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "date") {
			t.Errorf("%s: want 422 naming date, got %d (%s)", name, w.Code, w.Body.String())
		}
	}
	if w := f.skip(t, f.complexID, ser.ID, next); w.Code != http.StatusOK {
		t.Fatalf("a pending date skips; got %d (%s)", w.Code, w.Body.String())
	}
	if w := f.skip(t, f.complexID, ser.ID, next); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("already skipped: want 422, got %d (%s)", w.Code, w.Body.String())
	}
	if rec := f.ledger(t, f.day(0)); rec.Outcome != bookingstore.SeriesDateBooked {
		t.Errorf("a refused skip must not touch a booked date; got %+v", rec)
	}
}

// ─── Ending a series ────────────────────────────────────────────────────────

func (f *seriesFx) end(t *testing.T, complexID, seriesID uuid.UUID, from time.Time) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.EndSeries(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String(), "seriesID": seriesID.String()},
		fmt.Sprintf(`{"from_date":%q}`, from.Format(time.DateOnly))))
	return w
}

func TestEndingFromADateStopsHoldingAndCancelsOnlyUnpaidCreatedBookings(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(0), "") // booked today
	ser := f.onlySeries(t)
	f.runJob(f.at(7, "00:05")) // next Monday's booking
	f.runJob(f.at(14, "00:05"))
	if len(f.store.inserted) != 3 {
		t.Fatalf("setup: want 3 bookings, got %d", len(f.store.inserted))
	}
	paid := f.store.inserted[2]
	paid.CollectionStatus = bookingstore.CollectionStatusFullyPaid

	from := f.day(7)
	w := f.end(t, f.complexID, ser.ID, from)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if n := len(body["cancelled"].([]any)); n != 1 {
		t.Errorf("want the unpaid one cancelled, got %d", n)
	}
	if n := len(body["kept"].([]any)); n != 1 {
		t.Errorf("want the paid one kept, got %d", n)
	}
	if f.store.inserted[0].Status == bookingstore.BookingStatusCancelled {
		t.Error("a booking before from_date must be untouched")
	}
	if f.store.inserted[1].Status != bookingstore.BookingStatusCancelled || paid.Status == bookingstore.BookingStatusCancelled {
		t.Errorf("statuses: %s / %s", f.store.inserted[1].Status, paid.Status)
	}
	if ser.EndsOn == nil || !ser.EndsOn.Equal(from.AddDate(0, 0, -1)) || ser.Status != bookingstore.SeriesStatusEnded {
		t.Errorf("series = %+v, want ended the day before from_date", ser)
	}
	if len(f.notify.cancelled) != 0 {
		t.Error("ending a series must not message the client per booking")
	}
	// Nothing is held from then on, and the job does no more.
	if held := f.series.occurrences(nil, nil, from, from.AddDate(1, 0, 0)); len(held) != 0 {
		t.Errorf("an ended series must hold nothing; got %d dates", len(held))
	}
	f.runJob(f.at(21, "00:05"))
	if len(f.store.inserted) != 3 {
		t.Error("an ended series must not be materialized")
	}
}

func TestEndingASeriesBeforeItsFirstDateLeavesItEmptyAndEnded(t *testing.T) {
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
	if w := f.end(t, f.complexID, ser.ID, f.day(-1)); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("a past from_date must be refused; got %d", w.Code)
	}
}

func TestAnotherOwnerCannotReadSkipOrEndASeries(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(0), "")
	ser := f.onlySeries(t)
	other := uuid.New()

	w := httptest.NewRecorder()
	f.handler.GetSeries(w, ownerRequest(t, http.MethodGet, "/", other,
		map[string]string{"id": other.String(), "seriesID": ser.ID.String()}, ""))
	if w.Code != http.StatusNotFound {
		t.Errorf("reading another complex's series: want 404, got %d", w.Code)
	}
	if w := f.skip(t, other, ser.ID, f.day(7)); w.Code != http.StatusNotFound {
		t.Errorf("skipping another complex's series: want 404, got %d", w.Code)
	}
	if w := f.end(t, other, ser.ID, f.day(5)); w.Code != http.StatusNotFound {
		t.Errorf("ending another complex's series: want 404, got %d", w.Code)
	}
	for _, b := range f.store.inserted {
		if b.Status == bookingstore.BookingStatusCancelled {
			t.Error("a foreign end request cancelled bookings")
		}
	}
	if ser.Status != bookingstore.SeriesStatusActive || ser.EndsOn != nil || len(f.series.dates) != 1 {
		t.Error("a foreign request changed the series")
	}
}

// ─── Reading a series ───────────────────────────────────────────────────────

func TestGetSeriesComputesTheNextDatesFromTheRuleWithTheirState(t *testing.T) {
	f := newSeriesFixture(t)
	f.create(t, f.day(0), "") // today: booked
	ser := f.onlySeries(t)
	if w := f.skip(t, f.complexID, ser.ID, f.day(14)); w.Code != http.StatusOK {
		t.Fatalf("skip: %d", w.Code)
	}

	w := httptest.NewRecorder()
	f.handler.GetSeries(w, ownerRequest(t, http.MethodGet, "/", f.complexID,
		map[string]string{"id": f.complexID.String(), "seriesID": ser.ID.String()}, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	up := body["upcoming"].([]any)
	if len(up) != seriesUpcomingCount {
		t.Fatalf("want %d upcoming dates, got %d", seriesUpcomingCount, len(up))
	}
	state := func(i int) map[string]any { return up[i].(map[string]any) }
	if state(0)["state"] != SeriesStateBooked || state(0)["booking_id"] != f.store.inserted[0].ID.String() {
		t.Errorf("today is booked, naming its booking; got %v", state(0))
	}
	if state(1)["state"] != SeriesStatePending || state(1)["date"] != f.day(7).Format(time.DateOnly) {
		t.Errorf("next week is pending; got %v", state(1))
	}
	if state(2)["state"] != SeriesStateException || state(2)["reason"] != ReasonCancelledByOwner {
		t.Errorf("the skipped date is an exception with its reason; got %v", state(2))
	}
	if body["series"].(map[string]any)["weekday"] != float64(f.day(0).Weekday()) {
		t.Errorf("series = %v", body["series"])
	}
	if _, stored := body["skipped"]; stored {
		t.Error("the detail has no skipped list any more")
	}
}
