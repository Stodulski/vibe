package waitlist

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/courts"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/turnstile"
	"github.com/stodulski/vibe-server/internal/waitlist/store"
)

type eventJoin struct {
	complexID, eventID uuid.UUID
	email              string
}

// fakeSlot is one slot entry the fake store keeps, in created_at order.
type fakeSlot struct {
	store.FreedSlotEntry
	duration int
	notified bool
}

// fakePlaceStore records what the place service asks of it. missing makes the
// "can this be waited for" lookups answer data.ErrRecordNotFound; spotsLeft is
// what they report free; waiting is the count CountWaiting answers.
type fakePlaceStore struct {
	missing       bool
	spotsLeft     int
	waiting       int
	categoryEvent uuid.UUID
	events        []eventJoin
	slots         []store.NewSlotEntry
	freedEvents   []store.FreedEventEntry
	waitingSlots  []*fakeSlot
	gridPages     int
}

func (f *fakePlaceStore) lookup() error {
	if f.missing {
		return data.ErrRecordNotFound
	}
	return nil
}

func (f *fakePlaceStore) WaitlistableEvent(context.Context, uuid.UUID, uuid.UUID, time.Time) (int, error) {
	return f.spotsLeft, f.lookup()
}

func (f *fakePlaceStore) WaitlistableCategoryEvent(context.Context, uuid.UUID, uuid.UUID, time.Time) (uuid.UUID, int, error) {
	return f.categoryEvent, f.spotsLeft, f.lookup()
}

func (f *fakePlaceStore) WaitlistableCourt(context.Context, uuid.UUID, uuid.UUID) error {
	return f.lookup()
}

func (f *fakePlaceStore) CountWaiting(context.Context, uuid.UUID, string) (int, error) {
	return f.waiting, nil
}

func (f *fakePlaceStore) InsertEvent(_ context.Context, complexID, eventID uuid.UUID, email string) error {
	f.events = append(f.events, eventJoin{complexID: complexID, eventID: eventID, email: email})
	return nil
}

func (f *fakePlaceStore) InsertSlot(_ context.Context, e store.NewSlotEntry) error {
	f.slots = append(f.slots, e)
	return nil
}

func (f *fakePlaceStore) ClaimFreedEvents(context.Context, time.Time, int) ([]store.FreedEventEntry, error) {
	return f.freedEvents, nil
}

// gridLess is the key order ListWaitingSlotGrids walks: day, then duration (the
// fake keeps one complex).
func gridLess(a, b store.SlotGrid) bool {
	if !a.Day.Equal(b.Day) {
		return a.Day.Before(b.Day)
	}
	return a.DurationMinutes < b.DurationMinutes
}

// ListWaitingGrids answers like the query: distinct grids of the waiting
// entries, in key order after the cursor, at most limit.
func (f *fakePlaceStore) ListWaitingGrids(_ context.Context, _ time.Time, after *store.SlotGrid, limit int) ([]store.SlotGrid, error) {
	f.gridPages++
	seen := map[store.SlotGrid]bool{}
	var grids []store.SlotGrid
	for _, e := range f.waitingSlots {
		g := store.SlotGrid{ComplexSlug: e.ComplexSlug, Day: e.Day, DurationMinutes: e.duration}
		if e.notified || seen[g] || (after != nil && !gridLess(*after, g)) {
			continue
		}
		seen[g] = true
		grids = append(grids, g)
	}
	for i := 1; i < len(grids); i++ {
		for j := i; j > 0 && gridLess(grids[j], grids[j-1]); j-- {
			grids[j], grids[j-1] = grids[j-1], grids[j]
		}
	}
	if len(grids) > limit {
		grids = grids[:limit]
	}
	return grids, nil
}

// ClaimFreeSlots claims, once, the waiting entries of grid at the positions given.
func (f *fakePlaceStore) ClaimFreeSlots(_ context.Context, g store.SlotGrid, positions []string) ([]store.FreedSlotEntry, error) {
	var out []store.FreedSlotEntry
	for _, e := range f.waitingSlots {
		if e.notified || !e.Day.Equal(g.Day) || e.duration != g.DurationMinutes {
			continue
		}
		for _, pos := range positions {
			if pos == e.CourtID.String()+" "+e.StartTime {
				e.notified = true
				out = append(out, e.FreedSlotEntry)
				break
			}
		}
	}
	return out, nil
}

func (f *fakePlaceStore) DeletePast(context.Context, time.Time) (int64, error) { return 0, nil }

// fakeGrid answers Availability with one court. free and taken are its starts
// on every grid, unless freeOn names the free starts per "date/duration".
type fakeGrid struct {
	courtID string
	free    []string
	taken   []string
	freeOn  map[string][]string
	reads   int
}

func (g *fakeGrid) Availability(_ context.Context, _, dateStr string, _ time.Time, duration int) (*courts.AvailabilityResponse, error) {
	g.reads++
	free, taken := g.free, g.taken
	if g.freeOn != nil {
		free, taken = g.freeOn[fmt.Sprintf("%s/%d", dateStr, duration)], nil
	}
	c := courts.CourtAvailability{CourtID: g.courtID}
	for _, s := range free {
		c.Slots = append(c.Slots, courts.AvailabilitySlot{StartTime: s, StartMin: slots.ToMinutes(s), Available: true})
	}
	for _, s := range taken {
		c.Slots = append(c.Slots, courts.AvailabilitySlot{StartTime: s, StartMin: slots.ToMinutes(s), Available: false})
	}
	return &courts.AvailabilityResponse{IsOpen: true, Courts: []courts.CourtAvailability{c}}, nil
}

type fakePlaceNotifier struct {
	sent []notifications.WaitlistPlaceFreedEmail
}

func (f *fakePlaceNotifier) WaitlistPlaceFreed(e notifications.WaitlistPlaceFreedEmail) {
	f.sent = append(f.sent, e)
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newPlaceService(st *fakePlaceStore, cx *fakeComplexes, g *fakeGrid, n *fakePlaceNotifier) *PlaceService {
	return NewPlaceService(st, cx, g, n, "https://vibe.test", discard())
}

func newPlaceHandler(st *fakePlaceStore, cx *fakeComplexes, gate *turnstile.Gate) *PlaceHandler {
	return newPlaceHandlerOn(st, cx, &fakeGrid{}, gate)
}

func newPlaceHandlerOn(st *fakePlaceStore, cx *fakeComplexes, g *fakeGrid, gate *turnstile.Gate) *PlaceHandler {
	respond := httpx.NewResponder(discard())
	return NewPlaceHandler(newPlaceService(st, cx, g, &fakePlaceNotifier{}), respond, gate)
}

func postPlace(t *testing.T, h *PlaceHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/public/complexes/vibe-padel/waitlist/places", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("slug", "vibe-padel")
	w := httptest.NewRecorder()
	h.Join(w, r)
	return w
}

func inDays(n int) string { return timezone.Today().AddDate(0, 0, n).Format(time.DateOnly) }

func slotBody(courtID uuid.UUID, date, start string, minutes string) string {
	return `{"email":"ana@example.com","kind":"slot","court_id":"` + courtID.String() + `","date":"` + date +
		`","start_time":"` + start + `","duration_minutes":` + minutes + `}`
}

// S5: each kind answers 204 and stores the trimmed, lowercased address against
// its place; a repeated join answers the same 204.
func TestJoinPlaceAnswers204ForEveryKindAndOnARepeat(t *testing.T) {
	complexID, eventID, categoryEvent, courtID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	st := &fakePlaceStore{categoryEvent: categoryEvent}
	h := newPlaceHandlerOn(st, activeComplex(complexID), &fakeGrid{courtID: courtID.String(), taken: []string{"19:00"}}, nil)

	bodies := []string{
		`{"email":"  Ana@Example.COM ","kind":"open_play","event_id":"` + eventID.String() + `"}`,
		`{"email":"ana@example.com","kind":"tournament_category","category_id":"` + uuid.NewString() + `"}`,
		slotBody(courtID, inDays(2), "19:00", "90"),
	}
	for _, body := range bodies {
		for i := 0; i < 2; i++ {
			if w := postPlace(t, h, body); w.Code != http.StatusNoContent || w.Body.Len() != 0 {
				t.Fatalf("join %d of %s: want an empty 204; got %d (%s)", i+1, body, w.Code, w.Body.String())
			}
		}
	}

	if len(st.events) != 4 || st.events[0] != (eventJoin{complexID, eventID, "ana@example.com"}) {
		t.Fatalf("want the open-play join stored trimmed and lowercased; got %+v", st.events)
	}
	if st.events[2].eventID != categoryEvent {
		t.Errorf("a tournament category is stored by its sign-up event; got %v, want %v", st.events[2].eventID, categoryEvent)
	}
	want := store.NewSlotEntry{ComplexID: complexID, Email: "ana@example.com", CourtID: courtID,
		Day: timezone.Today().AddDate(0, 0, 2), StartTime: "19:00", DurationMinutes: 90}
	if len(st.slots) != 2 || st.slots[0].CourtID != want.CourtID || st.slots[0].StartTime != want.StartTime ||
		st.slots[0].DurationMinutes != want.DurationMinutes || !st.slots[0].Day.Equal(want.Day) {
		t.Fatalf("slot entry = %+v; want %+v", st.slots, want)
	}
}

func TestJoinPlaceRejectsBadInputWith422NamingTheField(t *testing.T) {
	courtID := uuid.New()
	cases := map[string]struct{ body, field string }{
		"kind":          {`{"email":"ana@example.com","kind":"padel"}`, "kind"},
		"email":         {`{"email":"nope","kind":"open_play","event_id":"` + uuid.NewString() + `"}`, "email"},
		"event_id":      {`{"email":"ana@example.com","kind":"open_play","event_id":"x"}`, "event_id"},
		"category_id":   {`{"email":"ana@example.com","kind":"tournament_category"}`, "category_id"},
		"court_id":      {`{"email":"ana@example.com","kind":"slot","date":"` + inDays(1) + `","start_time":"19:00","duration_minutes":60}`, "court_id"},
		"past date":     {slotBody(courtID, inDays(-1), "19:00", "60"), "date"},
		"beyond":        {slotBody(courtID, inDays(366), "19:00", "60"), "date"},
		"not a date":    {slotBody(courtID, "18-11-2026", "19:00", "60"), "date"},
		"start_time":    {slotBody(courtID, inDays(1), "7pm", "60"), "start_time"},
		"zero duration": {slotBody(courtID, inDays(1), "19:00", "0"), "duration_minutes"},
	}
	for name, c := range cases {
		st := &fakePlaceStore{}
		w := postPlace(t, newPlaceHandler(st, activeComplex(uuid.New()), nil), c.body)
		if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"`+c.field+`"`) {
			t.Errorf("%s: want 422 naming %s; got %d (%s)", name, c.field, w.Code, w.Body.String())
		}
		if len(st.events)+len(st.slots) != 0 {
			t.Errorf("%s: an invalid join must store nothing", name)
		}
	}
}

func TestJoinPlaceRejectsAMalformedBody(t *testing.T) {
	w := postPlace(t, newPlaceHandler(&fakePlaceStore{}, activeComplex(uuid.New()), nil), `{"email":`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400; got %d", w.Code)
	}
}

// S5: an event, category or court that is not the complex's (or can no longer
// be waited for), and an unknown complex, answer 404 and store nothing.
func TestJoinPlaceAnswers404WhenThePlaceIsNotTheComplexs(t *testing.T) {
	bodies := []string{
		`{"email":"ana@example.com","kind":"open_play","event_id":"` + uuid.NewString() + `"}`,
		`{"email":"ana@example.com","kind":"tournament_category","category_id":"` + uuid.NewString() + `"}`,
		slotBody(uuid.New(), inDays(1), "19:00", "60"),
	}
	for _, body := range bodies {
		st := &fakePlaceStore{missing: true}
		w := postPlace(t, newPlaceHandler(st, activeComplex(uuid.New()), nil), body)
		if w.Code != http.StatusNotFound || len(st.events)+len(st.slots) != 0 {
			t.Errorf("%s: want 404 and nothing stored; got %d", body, w.Code)
		}
	}
	w := postPlace(t, newPlaceHandler(&fakePlaceStore{}, &fakeComplexes{err: data.ErrRecordNotFound}, nil), bodies[0])
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown complex: want 404; got %d", w.Code)
	}
}

type fakeVerifier struct{}

func (fakeVerifier) Enabled() bool                                { return true }
func (fakeVerifier) Verify(context.Context, string, string) error { return turnstile.ErrInvalidToken }

// S5: the join carries the same anti-bot check as the sign-ups.
func TestJoinPlaceAsksTheTurnstileGate(t *testing.T) {
	gate := turnstile.NewGate(fakeVerifier{}, true, httpx.NewResponder(discard()), discard(), httpx.TrustedProxies{})
	body := `{"email":"ana@example.com","kind":"open_play","event_id":"` + uuid.NewString() + `"}`
	for _, c := range []struct{ body, want string }{
		{body, "required"},
		{strings.Replace(body, `{`, `{"turnstile_token":"bad",`, 1), "invalid"},
	} {
		st := &fakePlaceStore{}
		w := postPlace(t, newPlaceHandler(st, activeComplex(uuid.New()), gate), c.body)
		if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), c.want) ||
			!strings.Contains(w.Body.String(), "turnstile_token") || len(st.events) != 0 {
			t.Errorf("want 422 turnstile_token %s and nothing stored; got %d (%s)", c.want, w.Code, w.Body.String())
		}
	}
}

// S4: every claimed event entry is emailed once, keyed on its row, with what
// freed and the direct link: the event page for open play, the tournament page
// for a category.
func TestSweepEmailsEveryClaimedEventEntry(t *testing.T) {
	tournamentID, eventID, entryA, entryB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	wednesday := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	st := &fakePlaceStore{freedEvents: []store.FreedEventEntry{
		{ID: entryA, Email: "ana@example.com", EventID: eventID, EventName: "Cancha abierta de pádel",
			EventDay: wednesday, EventStart: "20:00", ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel"},
		{ID: entryB, Email: "bruno@example.com", EventID: uuid.New(), EventName: "Torneo Vibe · 6ta",
			EventDay: wednesday, EventStart: "00:00", TournamentID: &tournamentID, CategoryName: "6ta",
			ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel"},
	}}
	n := &fakePlaceNotifier{}
	newPlaceService(st, activeComplex(uuid.New()), &fakeGrid{}, n).Sweep(t.Context())

	if len(n.sent) != 2 {
		t.Fatalf("want one email per claimed entry; got %d", len(n.sent))
	}
	open, cat := n.sent[0], n.sent[1]
	if open.EntryID != entryA.String() || open.Email != "ana@example.com" ||
		open.PlaceLabel != "Cancha abierta de pádel, miércoles 18/11 20:00" ||
		open.BookURL != "https://vibe.test/c/vibe-padel/events/"+eventID.String() {
		t.Errorf("open play email = %+v", open)
	}
	if cat.EntryID != entryB.String() || cat.PlaceLabel != "Torneo Vibe · 6ta" ||
		cat.BookURL != "https://vibe.test/c/vibe-padel/tournaments/"+tournamentID.String() {
		t.Errorf("tournament email = %+v", cat)
	}
}

func slotEntry(courtID uuid.UUID, day time.Time, start string, duration int) *fakeSlot {
	return &fakeSlot{FreedSlotEntry: store.FreedSlotEntry{ID: uuid.New(), Email: "ana@example.com", CourtID: courtID,
		CourtName: "1", Day: day, StartTime: start, ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel"}, duration: duration}
}

// S4: a slot entry is claimed and emailed only when the courts grid says its
// slot is on sale now; one grid read serves every entry of the same day.
func TestSweepEmailsOnlyTheSlotsTheGridSaysAreFree(t *testing.T) {
	courtID := uuid.New()
	wednesday := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	free, taken := slotEntry(courtID, wednesday, "19:00", 90), slotEntry(courtID, wednesday, "20:30", 90)
	st := &fakePlaceStore{waitingSlots: []*fakeSlot{free, taken}}
	g := &fakeGrid{courtID: courtID.String(), free: []string{"19:00"}, taken: []string{"20:30"}}
	n := &fakePlaceNotifier{}
	newPlaceService(st, activeComplex(uuid.New()), g, n).Sweep(t.Context())

	if g.reads != 1 {
		t.Errorf("want one grid read for one day; got %d", g.reads)
	}
	if !free.notified || taken.notified {
		t.Fatalf("want only the free slot's entry claimed; free=%v taken=%v", free.notified, taken.notified)
	}
	if len(n.sent) != 1 {
		t.Fatalf("want one email; got %d", len(n.sent))
	}
	got := n.sent[0]
	if got.EntryID != free.ID.String() || got.PlaceLabel != "Cancha 1 el miércoles 18/11 a las 19:00" ||
		got.BookURL != "https://vibe.test/c/vibe-padel?date=2026-11-18" {
		t.Errorf("slot email = %+v", got)
	}
}

// S4: a claimed entry is not emailed again by the next run.
func TestSweepEmailsASlotEntryOnce(t *testing.T) {
	courtID := uuid.New()
	st := &fakePlaceStore{waitingSlots: []*fakeSlot{slotEntry(courtID, time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC), "19:00", 60)}}
	n := &fakePlaceNotifier{}
	svc := newPlaceService(st, activeComplex(uuid.New()), &fakeGrid{courtID: courtID.String(), free: []string{"19:00"}}, n)
	svc.Sweep(t.Context())
	svc.Sweep(t.Context())

	if len(n.sent) != 1 {
		t.Fatalf("want one email over two runs; got %d", len(n.sent))
	}
}

// B1 regression: a batch of older entries whose slots stay taken must not
// starve a newer one whose slot is free. The walk over the grids resumes after
// the last one read, so the free entry is reached within a few runs, and is
// emailed exactly once.
func TestSweepReachesASlotEntryBehindABatchOfStillTakenOnes(t *testing.T) {
	courtID := uuid.New()
	base := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	st := &fakePlaceStore{}
	// sweepBatch older entries, each on its own grid, every slot still taken.
	for i := 0; i < sweepBatch; i++ {
		st.waitingSlots = append(st.waitingSlots, slotEntry(courtID, base.AddDate(0, 0, i/10), "19:00", 30+i%10))
	}
	// The newest entry, on the last grid in key order, whose slot is free.
	newest := slotEntry(courtID, base.AddDate(0, 0, sweepBatch), "19:00", 60)
	st.waitingSlots = append(st.waitingSlots, newest)
	g := &fakeGrid{courtID: courtID.String(), freeOn: map[string][]string{
		base.AddDate(0, 0, sweepBatch).Format(time.DateOnly) + "/60": {"19:00"},
	}}
	n := &fakePlaceNotifier{}
	svc := newPlaceService(st, activeComplex(uuid.New()), g, n)

	for run := 0; run < 3; run++ {
		svc.Sweep(t.Context())
	}

	if !newest.notified {
		t.Fatal("the free entry behind a full batch of taken ones was never claimed")
	}
	if len(n.sent) != 1 || n.sent[0].EntryID != newest.ID.String() {
		t.Fatalf("want exactly one email, for the free entry; got %+v", n.sent)
	}
}

// B2 regression: a wait is only for a place that is full now. A place with a
// spot free, a slot the grid does not sell, a slot that has begun and an address
// already waiting for too many places are refused with a 422 naming the field,
// and nothing is stored, so the join cannot make the sweep email anyone at once.
func TestJoinPlaceDoesNotStoreAWaitForAPlaceThatIsFree(t *testing.T) {
	courtID := uuid.New()
	eventBody := `{"email":"ana@example.com","kind":"open_play","event_id":"` + uuid.NewString() + `"}`
	categoryBody := `{"email":"ana@example.com","kind":"tournament_category","category_id":"` + uuid.NewString() + `"}`
	taken := &fakeGrid{courtID: courtID.String(), taken: []string{"19:00"}}
	cases := map[string]struct {
		st          *fakePlaceStore
		grid        *fakeGrid
		body, field string
	}{
		"event with a spot":    {&fakePlaceStore{spotsLeft: 1}, taken, eventBody, "event_id"},
		"category with a spot": {&fakePlaceStore{spotsLeft: 1}, taken, categoryBody, "category_id"},
		"free slot": {&fakePlaceStore{}, &fakeGrid{courtID: courtID.String(), free: []string{"19:00"}},
			slotBody(courtID, inDays(2), "19:00", "90"), "start_time"},
		"slot off the grid": {&fakePlaceStore{}, taken, slotBody(courtID, inDays(2), "03:17", "7"), "start_time"},
		"another court's slot": {&fakePlaceStore{}, taken,
			slotBody(uuid.New(), inDays(2), "19:00", "90"), "start_time"},
		"slot already begun": {&fakePlaceStore{}, &fakeGrid{courtID: courtID.String(), taken: []string{"00:00"}},
			slotBody(courtID, inDays(0), "00:00", "90"), "start_time"},
		"too many waits": {&fakePlaceStore{waiting: MaxWaitingPerEmail}, taken, eventBody, "email"},
	}
	for name, c := range cases {
		w := postPlace(t, newPlaceHandlerOn(c.st, activeComplex(uuid.New()), c.grid, nil), c.body)
		if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"`+c.field+`"`) {
			t.Errorf("%s: want 422 naming %s; got %d (%s)", name, c.field, w.Code, w.Body.String())
		}
		if len(c.st.events)+len(c.st.slots) != 0 {
			t.Errorf("%s: nothing may be stored; got %d events, %d slots", name, len(c.st.events), len(c.st.slots))
		}
	}

	// One under the cap is still accepted.
	st := &fakePlaceStore{waiting: MaxWaitingPerEmail - 1}
	if w := postPlace(t, newPlaceHandlerOn(st, activeComplex(uuid.New()), taken, nil), eventBody); w.Code != http.StatusNoContent {
		t.Errorf("one under the cap: want 204; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCourtLabelDoesNotDoubleTheWord(t *testing.T) {
	if got := courtLabel("Cancha 2"); got != "Cancha 2" {
		t.Errorf("courtLabel(Cancha 2) = %q", got)
	}
	if got := courtLabel("Central"); got != "Cancha Central" {
		t.Errorf("courtLabel(Central) = %q", got)
	}
}
