package waitlist

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
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
	eventRearms   int
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

// ListWaitingGrids answers like the query: distinct grids of the entries, told
// ones included, in key order after the cursor, at most limit.
func (f *fakePlaceStore) ListWaitingGrids(_ context.Context, _ time.Time, after *store.SlotGrid, limit int) ([]store.SlotGrid, error) {
	f.gridPages++
	seen := map[store.SlotGrid]bool{}
	var grids []store.SlotGrid
	for _, e := range f.waitingSlots {
		g := store.SlotGrid{ComplexSlug: e.ComplexSlug, Day: e.Day, DurationMinutes: e.duration}
		if seen[g] || (after != nil && !gridLess(*after, g)) {
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

// ClaimFreeSlots answers like the query: it claims, once, the waiting entries of
// grid at the positions given, and with them the same address's other waiting
// entries for that hour.
func (f *fakePlaceStore) ClaimFreeSlots(_ context.Context, g store.SlotGrid, positions []string) ([]store.FreedSlotEntry, error) {
	type key struct{ email, start string }
	inGrid := func(e *fakeSlot) bool {
		return !e.notified && e.Day.Equal(g.Day) && e.duration == g.DurationMinutes
	}
	free := func(e *fakeSlot) bool { return slices.Contains(positions, e.CourtID.String()+" "+e.StartTime) }
	hours := map[key]bool{}
	for _, e := range f.waitingSlots {
		if inGrid(e) && free(e) {
			hours[key{e.Email, e.StartTime}] = true
		}
	}
	var out []store.FreedSlotEntry
	for _, e := range f.waitingSlots {
		if inGrid(e) && hours[key{e.Email, e.StartTime}] {
			e.notified = true
			claimed := e.FreedSlotEntry
			claimed.Free = free(e)
			out = append(out, claimed)
		}
	}
	return out, nil
}

// RearmTakenSlots answers like the query: the told entries of g at a taken
// position wait again.
func (f *fakePlaceStore) RearmTakenSlots(_ context.Context, g store.SlotGrid, positions []string) (int64, error) {
	var n int64
	for _, e := range f.waitingSlots {
		if !e.notified || !e.Day.Equal(g.Day) || e.duration != g.DurationMinutes {
			continue
		}
		for _, pos := range positions {
			if pos == e.CourtID.String()+" "+e.StartTime {
				e.notified = false
				n++
				break
			}
		}
	}
	return n, nil
}

func (f *fakePlaceStore) RearmFullEvents(context.Context, time.Time) (int64, error) {
	f.eventRearms++
	return 0, nil
}

func (f *fakePlaceStore) DeletePast(context.Context, time.Time) (int64, error) { return 0, nil }

// fakeGrid answers Availability with one court. free and taken are its starts
// on every grid, unless freeOn names the free starts per "date/duration".
type fakeGrid struct {
	courtID string
	free    []string
	taken   []string
	freeOn  map[string][]string
	// more are further courts on every grid, each with its own taken starts.
	more map[string][]string
	// freeAlso are further courts whose free starts are the main court's.
	freeAlso []string
	reads    int
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
	out := []courts.CourtAvailability{c}
	for id, takenThere := range g.more {
		other := courts.CourtAvailability{CourtID: id}
		for _, s := range takenThere {
			other.Slots = append(other.Slots, courts.AvailabilitySlot{StartTime: s, StartMin: slots.ToMinutes(s), Available: false})
		}
		out = append(out, other)
	}
	for _, id := range g.freeAlso {
		other := courts.CourtAvailability{CourtID: id}
		for _, s := range free {
			other.Slots = append(other.Slots, courts.AvailabilitySlot{StartTime: s, StartMin: slots.ToMinutes(s), Available: true})
		}
		out = append(out, other)
	}
	return &courts.AvailabilityResponse{IsOpen: true, Courts: out}, nil
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
		CourtName: "1", CourtSport: "padel", Day: day, StartTime: start, ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel"}, duration: duration}
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
		got.BookURL != "https://vibe.test/c/vibe-padel?court="+courtID.String()+"&date=2026-11-18&duration=90&sport=padel&time=19%3A00" {
		t.Errorf("slot email = %+v", got)
	}
}

// S6: a person waiting on several courts of the same hour gets one email for
// that hour, whether one of those courts frees or several do, pointing at a free
// court; every one of their entries for the hour is claimed with it, so a court
// that frees later in the round sends nothing more. Someone else waiting on the
// same court still gets their own email.
func TestSweepEmailsAPersonOncePerFreedHourAcrossCourts(t *testing.T) {
	courtA, courtB, courtC := uuid.New(), uuid.New(), uuid.New()
	wednesday := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	onA, onB, onC := slotEntry(courtA, wednesday, "19:00", 90), slotEntry(courtB, wednesday, "19:00", 90), slotEntry(courtC, wednesday, "19:00", 90)
	bruno := slotEntry(courtA, wednesday, "19:00", 90)
	bruno.Email = "bruno@example.com"
	st := &fakePlaceStore{waitingSlots: []*fakeSlot{onC, onA, onB, bruno}}
	// A and B are free at 19:00, C is taken.
	g := &fakeGrid{courtID: courtA.String(), free: []string{"19:00"}, freeAlso: []string{courtB.String()},
		more: map[string][]string{courtC.String(): {"19:00"}}}
	n := &fakePlaceNotifier{}
	newPlaceService(st, activeComplex(uuid.New()), g, n).Sweep(t.Context())

	perEmail := map[string]int{}
	for _, e := range n.sent {
		perEmail[e.Email]++
		if e.Email == "ana@example.com" && !strings.Contains(e.BookURL, "court="+courtA.String()) &&
			!strings.Contains(e.BookURL, "court="+courtB.String()) {
			t.Errorf("Ana's link must open a free court; got %s", e.BookURL)
		}
	}
	if perEmail["ana@example.com"] != 1 || perEmail["bruno@example.com"] != 1 || len(n.sent) != 2 {
		t.Fatalf("want one email for Ana and one for Bruno; got %v", perEmail)
	}
	if !onA.notified || !onB.notified || !onC.notified {
		t.Errorf("every entry of Ana's for 19:00 is claimed with the email; A=%v B=%v C=%v", onA.notified, onB.notified, onC.notified)
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

// When a taken slot frees, everyone waiting for it is emailed in the same run,
// each under their own entry, and each link opens that exact day, sport,
// duration and hour. Nobody waits for a later run, and nobody gets it twice.
func TestSweepEmailsEveryoneWaitingForAFreedSlotAtOnceWithTheReadyLink(t *testing.T) {
	courtID := uuid.New()
	wednesday := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	waiting := make([]*fakeSlot, 5)
	for i := range waiting {
		waiting[i] = slotEntry(courtID, wednesday, "19:00", 90)
		waiting[i].Email = fmt.Sprintf("persona%d@example.com", i+1)
	}
	st := &fakePlaceStore{waitingSlots: waiting}
	n := &fakePlaceNotifier{}
	svc := newPlaceService(st, activeComplex(uuid.New()), &fakeGrid{courtID: courtID.String(), free: []string{"19:00"}}, n)
	svc.Sweep(t.Context())

	if len(n.sent) != 5 {
		t.Fatalf("want the five people emailed in one run; got %d", len(n.sent))
	}
	link := "https://vibe.test/c/vibe-padel?court=" + courtID.String() + "&date=2026-11-18&duration=90&sport=padel&time=19%3A00"
	for i, got := range n.sent {
		if got.Email != waiting[i].Email || got.EntryID != waiting[i].ID.String() || got.BookURL != link {
			t.Errorf("email %d = %+v; want %s, its own entry, and the ready link", i, got, waiting[i].Email)
		}
	}

	svc.Sweep(t.Context())
	if len(n.sent) != 5 {
		t.Errorf("a second run must email nobody again; got %d emails in total", len(n.sent))
	}
}

// When a full event or tournament category frees a spot, everyone waiting for
// it is emailed in the same run, each with the link to that event's page.
func TestSweepEmailsEveryoneWaitingForAFreedEventAtOnceWithItsPage(t *testing.T) {
	eventID := uuid.New()
	wednesday := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	freed := make([]store.FreedEventEntry, 5)
	for i := range freed {
		freed[i] = store.FreedEventEntry{ID: uuid.New(), Email: fmt.Sprintf("persona%d@example.com", i+1),
			EventID: eventID, EventName: "Cancha abierta de pádel", EventDay: wednesday, EventStart: "20:00",
			ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel"}
	}
	n := &fakePlaceNotifier{}
	newPlaceService(&fakePlaceStore{freedEvents: freed}, activeComplex(uuid.New()), &fakeGrid{}, n).Sweep(t.Context())

	if len(n.sent) != 5 {
		t.Fatalf("want the five people emailed in one run; got %d", len(n.sent))
	}
	for i, got := range n.sent {
		if got.Email != freed[i].Email || got.EntryID != freed[i].ID.String() ||
			got.BookURL != "https://vibe.test/c/vibe-padel/events/"+eventID.String() {
			t.Errorf("email %d = %+v", i, got)
		}
	}
}

// The four who were told and did not get the slot are not forgotten: once it is
// taken again they wait again, and when it frees again all five are told again.
func TestSweepTellsEveryoneAgainWhenATakenSlotFreesOnceMore(t *testing.T) {
	courtID := uuid.New()
	wednesday := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	waiting := make([]*fakeSlot, 5)
	for i := range waiting {
		waiting[i] = slotEntry(courtID, wednesday, "19:00", 90)
		waiting[i].Email = fmt.Sprintf("persona%d@example.com", i+1)
	}
	st := &fakePlaceStore{waitingSlots: waiting}
	g := &fakeGrid{courtID: courtID.String(), free: []string{"19:00"}}
	n := &fakePlaceNotifier{}
	svc := newPlaceService(st, activeComplex(uuid.New()), g, n)

	svc.Sweep(t.Context())
	if len(n.sent) != 5 {
		t.Fatalf("want five told when it frees; got %d", len(n.sent))
	}

	// The first of them holds it: everyone told waits again, nobody is emailed.
	g.free, g.taken = nil, []string{"19:00"}
	svc.Sweep(t.Context())
	if len(n.sent) != 5 {
		t.Fatalf("a taken slot emails nobody; got %d emails in total", len(n.sent))
	}
	for i, e := range waiting {
		if e.notified {
			t.Errorf("entry %d must wait again once the slot is taken", i)
		}
	}

	// The hold runs out: the five are told again.
	g.free, g.taken = []string{"19:00"}, nil
	svc.Sweep(t.Context())
	if len(n.sent) != 10 {
		t.Fatalf("want the five told again when it frees once more; got %d emails in total", len(n.sent))
	}
	if st.eventRearms != 3 {
		t.Errorf("every run must also put the told event entries back; got %d", st.eventRearms)
	}
}

func anyCourtBody(date, start string, courtIDs ...uuid.UUID) string {
	ids := make([]string, len(courtIDs))
	for i, id := range courtIDs {
		ids[i] = `"` + id.String() + `"`
	}
	return `{"email":"ana@example.com","kind":"slot","court_ids":[` + strings.Join(ids, ",") + `],"date":"` + date +
		`","start_time":"` + start + `","duration_minutes":90}`
}

// Waiting for an hour on any court: one join puts the address on the list of
// every court taken at that hour, so whichever frees first reaches them.
func TestJoinPlaceWaitsForEveryCourtTakenAtThatHour(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	st := &fakePlaceStore{}
	g := &fakeGrid{courtID: first.String(), taken: []string{"19:00"}, more: map[string][]string{second.String(): {"19:00"}}}
	h := newPlaceHandlerOn(st, activeComplex(uuid.New()), g, nil)

	if w := postPlace(t, h, anyCourtBody(inDays(1), "19:00", first, second)); w.Code != http.StatusNoContent {
		t.Fatalf("want 204; got %d (%s)", w.Code, w.Body.String())
	}
	if len(st.slots) != 2 || st.slots[0].CourtID != first || st.slots[1].CourtID != second {
		t.Fatalf("want one waiting entry per court; got %+v", st.slots)
	}
	for _, e := range st.slots {
		if e.StartTime != "19:00" || e.DurationMinutes != 90 || e.Email != "ana@example.com" {
			t.Errorf("entry = %+v", e)
		}
	}
}

// When one of the courts is free at that hour there is nothing to wait for: the
// hour can be booked, and nothing is stored.
func TestJoinPlaceForAnyCourtRefusesWhenOneIsFree(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	st := &fakePlaceStore{}
	g := &fakeGrid{courtID: first.String(), free: []string{"19:00"}, more: map[string][]string{second.String(): {"19:00"}}}
	h := newPlaceHandlerOn(st, activeComplex(uuid.New()), g, nil)

	w := postPlace(t, h, anyCourtBody(inDays(1), "19:00", first, second))
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "start_time") {
		t.Fatalf("want 422 on start_time; got %d (%s)", w.Code, w.Body.String())
	}
	if len(st.slots) != 0 {
		t.Errorf("nothing must be stored; got %+v", st.slots)
	}
}

// The per-address cap counts every court the join would add.
func TestJoinPlaceForAnyCourtRespectsTheCap(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	st := &fakePlaceStore{waiting: MaxWaitingPerEmail - 1}
	g := &fakeGrid{courtID: first.String(), taken: []string{"19:00"}, more: map[string][]string{second.String(): {"19:00"}}}
	h := newPlaceHandlerOn(st, activeComplex(uuid.New()), g, nil)

	w := postPlace(t, h, anyCourtBody(inDays(1), "19:00", first, second))
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "email") {
		t.Fatalf("want 422 on email; got %d (%s)", w.Code, w.Body.String())
	}
	if len(st.slots) != 0 {
		t.Errorf("nothing must be stored; got %+v", st.slots)
	}
}

// A join names one court or a list, never neither.
func TestJoinPlaceForASlotNeedsACourt(t *testing.T) {
	h := newPlaceHandler(&fakePlaceStore{}, activeComplex(uuid.New()), nil)
	body := `{"email":"ana@example.com","kind":"slot","date":"` + inDays(1) + `","start_time":"19:00","duration_minutes":90}`
	if w := postPlace(t, h, body); w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "court_id") {
		t.Fatalf("want 422 on court_id; got %d (%s)", w.Code, w.Body.String())
	}
}
