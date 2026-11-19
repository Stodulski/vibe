//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist/store"
)

func newPlaceStore(f *datatest.Fixture) *store.PlaceStore {
	return &store.PlaceStore{Q: db.New(f.DB)}
}

// sweepCtx is the cron wrapper's cross-tenant context.
func sweepCtx() context.Context { return data.ContextWithTenantBypass(context.Background()) }

func exec(t *testing.T, f *datatest.Fixture, sql string, args ...any) {
	t.Helper()
	if _, err := f.DB.Exec(f.Scoped(context.Background()), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newEvent inserts an open-play event of the fixture's complex on day at start.
func newEvent(t *testing.T, f *datatest.Fixture, day time.Time, start string, capacity int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.DB.QueryRow(f.Scoped(context.Background()), `
		INSERT INTO open_play_events (complex_id, name, date, start_time, duration_minutes, capacity, price, created_by)
		VALUES ($1, 'Cancha abierta de pádel', $2, $3::time, 90, $4, 1000000, $5) RETURNING id`,
		f.ComplexID, day, start, capacity, f.UserID).Scan(&id); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	return id
}

// register inserts a registration of eventID in status, with its hold running
// until heldFor from now (negative: already over).
func register(t *testing.T, f *datatest.Fixture, eventID uuid.UUID, status string, heldFor time.Duration) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.DB.QueryRow(f.Scoped(context.Background()), `
		INSERT INTO open_play_registrations (event_id, complex_id, first_name, last_name, phone, status, price,
		    held_until, access_token_hash)
		VALUES ($1, $2, 'Ana', 'Diaz', $3, $4, 1000000, NOW() + make_interval(secs => $5),
		    decode(md5(random()::text), 'hex')) RETURNING id`,
		eventID, f.ComplexID, "+54911"+uuid.NewString()[:8], status, heldFor.Seconds()).Scan(&id); err != nil {
		t.Fatalf("insert registration: %v", err)
	}
	return id
}

// newCategory inserts a tournament with one category of capacity pairs and
// returns the category, its sign-up event and the tournament.
func newCategory(t *testing.T, f *datatest.Fixture, day time.Time, capacity int) (categoryID, eventID, tournamentID uuid.UUID) {
	t.Helper()
	ctx := f.Scoped(context.Background())
	if err := f.DB.QueryRow(ctx, `
		INSERT INTO tournaments (complex_id, name, start_date, end_date, created_by)
		VALUES ($1, 'Torneo Vibe', $2, $2, $3) RETURNING id`, f.ComplexID, day, f.UserID).Scan(&tournamentID); err != nil {
		t.Fatalf("insert tournament: %v", err)
	}
	if err := f.DB.QueryRow(ctx, `
		INSERT INTO open_play_events (complex_id, name, date, start_time, duration_minutes, capacity, price, created_by, tournament_id)
		VALUES ($1, 'Torneo Vibe · 6ta', $2, '00:00', 30, $3, 1000000, $4, $5) RETURNING id`,
		f.ComplexID, day, capacity, f.UserID, tournamentID).Scan(&eventID); err != nil {
		t.Fatalf("insert category event: %v", err)
	}
	if err := f.DB.QueryRow(ctx, `
		INSERT INTO tournament_categories (tournament_id, complex_id, event_id, name, format)
		VALUES ($1, $2, $3, '6ta', 'elimination') RETURNING id`, tournamentID, f.ComplexID, eventID).Scan(&categoryID); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	return categoryID, eventID, tournamentID
}

func countPlaceEntries(t *testing.T, f *datatest.Fixture) int {
	t.Helper()
	var n int
	if err := f.DB.QueryRow(f.Scoped(context.Background()),
		`SELECT count(*) FROM place_waitlist_entries WHERE complex_id = $1`, f.ComplexID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// S5: a second join of the same address for the same place is a no-op, for an
// event and for a court slot, so the endpoint answers 204 both times.
func TestPlaceInsertIsIdempotentPerAddressAndPlace(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	eventID := newEvent(t, f, timezone.Today().AddDate(0, 0, 5), "20:00", 4)
	slot := store.NewSlotEntry{ComplexID: f.ComplexID, Email: "ana@example.com", CourtID: f.CourtID,
		Day: timezone.Today().AddDate(0, 0, 5), StartTime: "19:00", DurationMinutes: 90}

	for i := 0; i < 2; i++ {
		if err := st.InsertEvent(ctx, f.ComplexID, eventID, "ana@example.com"); err != nil {
			t.Fatalf("event join %d: %v", i+1, err)
		}
		if err := st.InsertSlot(ctx, slot); err != nil {
			t.Fatalf("slot join %d: %v", i+1, err)
		}
	}
	if got := countPlaceEntries(t, f); got != 2 {
		t.Fatalf("want one row per place; got %d", got)
	}
	if err := st.InsertEvent(ctx, f.ComplexID, eventID, "Ana@Example.com"); err == nil {
		t.Error("the schema must refuse an address that was not lowercased first")
	}
}

// S5: only a live, not-yet-started event of the complex, an open category and an
// active court of the complex can be waited for.
func TestOnlyAWaitlistablePlaceIsFound(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	now := timezone.Now()
	future := newEvent(t, f, timezone.Today().AddDate(0, 0, 5), "20:00", 4)
	past := newEvent(t, f, timezone.Today().AddDate(0, 0, -1), "20:00", 4)

	if spots, err := st.WaitlistableEvent(ctx, f.ComplexID, future, now); err != nil || spots != 4 {
		t.Errorf("a future empty event: want 4 spots left; got %d (%v)", spots, err)
	}
	register(t, f, future, "confirmed", time.Hour)
	register(t, f, future, "held", time.Hour)
	register(t, f, future, "held", -time.Minute)
	if spots, _ := st.WaitlistableEvent(ctx, f.ComplexID, future, now); spots != 2 {
		t.Errorf("confirmed and live holds count, an expired hold does not: want 2 left; got %d", spots)
	}
	for name, id := range map[string]uuid.UUID{"started": past, "unknown": uuid.New()} {
		if _, err := st.WaitlistableEvent(ctx, f.ComplexID, id, now); !errors.Is(err, data.ErrRecordNotFound) {
			t.Errorf("%s event: want ErrRecordNotFound; got %v", name, err)
		}
	}
	if _, err := st.WaitlistableEvent(ctx, uuid.New(), future, now); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("another complex's event: want ErrRecordNotFound; got %v", err)
	}

	categoryID, eventID, _ := newCategory(t, f, timezone.Today().AddDate(0, 0, 5), 2)
	got, spots, err := st.WaitlistableCategoryEvent(ctx, f.ComplexID, categoryID, now)
	if err != nil || got != eventID || spots != 2 {
		t.Errorf("an open category resolves to its event with its spots: got %v, %d (%v), want %v, 2", got, spots, err, eventID)
	}
	if _, err := st.WaitlistableEvent(ctx, f.ComplexID, eventID, now); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a category's container is not an open-play event to wait for; got %v", err)
	}
	exec(t, f, `UPDATE tournament_categories SET draw_generated_at = NOW() WHERE id = $1`, categoryID)
	if _, _, err := st.WaitlistableCategoryEvent(ctx, f.ComplexID, categoryID, now); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a category after its draw: want ErrRecordNotFound; got %v", err)
	}

	if err := st.WaitlistableCourt(ctx, f.ComplexID, f.CourtID); err != nil {
		t.Errorf("the complex's court: %v", err)
	}
	if err := st.WaitlistableCourt(ctx, f.ComplexID, uuid.New()); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("an unknown court: want ErrRecordNotFound; got %v", err)
	}
}

// S4: an event entry is claimed only once a spot is free (a cancelled
// registration or an expired hold), every waiting address at once, and only
// once each.
func TestClaimFreedEventsWaitsForAFreeSpotAndIsOneShot(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	now := timezone.Now()
	eventID := newEvent(t, f, timezone.Today().AddDate(0, 0, 5), "20:00", 2)
	paid := register(t, f, eventID, "confirmed", time.Hour)
	held := register(t, f, eventID, "held", time.Hour)
	for _, email := range []string{"ana@example.com", "bruno@example.com"} {
		if err := st.InsertEvent(ctx, f.ComplexID, eventID, email); err != nil {
			t.Fatal(err)
		}
	}

	if got, err := st.ClaimFreedEvents(sweepCtx(), now, 100); err != nil || len(got) != 0 {
		t.Fatalf("a full event frees nothing; got %d (%v)", len(got), err)
	}

	// The hold runs out: its spot is free though nobody swept it to 'expired'.
	exec(t, f, `UPDATE open_play_registrations SET held_until = NOW() - interval '1 minute' WHERE id = $1`, held)
	got, err := st.ClaimFreedEvents(sweepCtx(), now, 100)
	if err != nil || len(got) != 2 {
		t.Fatalf("want both waiting addresses claimed at once; got %d (%v)", len(got), err)
	}
	e := got[0]
	if e.EventID != eventID || e.EventName != "Cancha abierta de pádel" || e.EventStart != "20:00" ||
		e.TournamentID != nil || e.ComplexSlug == "" || e.ComplexName == "" {
		t.Errorf("claimed entry = %+v", e)
	}

	exec(t, f, `UPDATE open_play_registrations SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, paid)
	if again, err := st.ClaimFreedEvents(sweepCtx(), now, 100); err != nil || len(again) != 0 {
		t.Fatalf("a claimed entry is never claimed again; got %d (%v)", len(again), err)
	}
}

// S4: nothing is claimed once the event has started, or for a cancelled event.
func TestClaimFreedEventsSkipsStartedAndCancelledEvents(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	eventID := newEvent(t, f, timezone.Today().AddDate(0, 0, 5), "20:00", 2)
	if err := st.InsertEvent(ctx, f.ComplexID, eventID, "ana@example.com"); err != nil {
		t.Fatal(err)
	}

	afterStart := time.Date(2099, time.January, 1, 0, 0, 0, 0, timezone.Argentina)
	if got, err := st.ClaimFreedEvents(sweepCtx(), afterStart, 100); err != nil || len(got) != 0 {
		t.Fatalf("an event that has started frees nothing; got %d (%v)", len(got), err)
	}
	exec(t, f, `UPDATE open_play_events SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, eventID)
	if got, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100); err != nil || len(got) != 0 {
		t.Fatalf("a cancelled event frees nothing; got %d (%v)", len(got), err)
	}
}

// S4: a tournament category's freed pair spot is claimed with its tournament,
// and no notice goes out once the draw closed the sign-up.
func TestClaimFreedEventsForATournamentCategoryStopsAtTheDraw(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	categoryID, eventID, tournamentID := newCategory(t, f, timezone.Today().AddDate(0, 0, 5), 1)
	pair := register(t, f, eventID, "confirmed", time.Hour)
	if err := st.InsertEvent(ctx, f.ComplexID, eventID, "ana@example.com"); err != nil {
		t.Fatal(err)
	}
	exec(t, f, `UPDATE open_play_registrations SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, pair)

	exec(t, f, `UPDATE tournament_categories SET draw_generated_at = NOW() WHERE id = $1`, categoryID)
	if got, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100); err != nil || len(got) != 0 {
		t.Fatalf("no notice after the draw; got %d (%v)", len(got), err)
	}

	exec(t, f, `UPDATE tournament_categories SET draw_generated_at = NULL WHERE id = $1`, categoryID)
	got, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100)
	if err != nil || len(got) != 1 {
		t.Fatalf("want the open category's entry claimed; got %d (%v)", len(got), err)
	}
	if got[0].TournamentID == nil || *got[0].TournamentID != tournamentID || got[0].CategoryName != "6ta" {
		t.Errorf("claimed category entry = %+v", got[0])
	}
}

// S4: the grids slot entries wait on are listed once each, in key order after
// the cursor, and the claim takes only the entries at the free positions given,
// once.
func TestSlotGridsAreListedAndFreeSlotsClaimedOnce(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	day := timezone.Today().AddDate(0, 0, 3)
	for _, e := range []struct {
		email, start string
		minutes      int
	}{{"ana@example.com", "19:00", 90}, {"bruno@example.com", "19:00", 90}, {"carla@example.com", "20:30", 90}, {"dario@example.com", "19:00", 60}} {
		if err := st.InsertSlot(ctx, store.NewSlotEntry{ComplexID: f.ComplexID, Email: e.email,
			CourtID: f.CourtID, Day: day, StartTime: e.start, DurationMinutes: e.minutes}); err != nil {
			t.Fatal(err)
		}
	}
	from := timezone.Today().AddDate(0, 0, -1)

	grids, err := st.ListWaitingGrids(sweepCtx(), from, nil, 100)
	if err != nil || len(grids) != 2 {
		t.Fatalf("want two grids (60 and 90 minutes); got %+v (%v)", grids, err)
	}
	if grids[0].DurationMinutes != 60 || grids[1].DurationMinutes != 90 || grids[0].ComplexID != f.ComplexID ||
		grids[0].ComplexSlug == "" || grids[0].Day.Format(time.DateOnly) != day.Format(time.DateOnly) {
		t.Errorf("grids = %+v", grids)
	}
	page, err := st.ListWaitingGrids(sweepCtx(), from, &grids[0], 1)
	if err != nil || len(page) != 1 || page[0].DurationMinutes != 90 {
		t.Fatalf("the page after the first grid is the second; got %+v (%v)", page, err)
	}

	pos := f.CourtID.String() + " 19:00"
	first, err := st.ClaimFreeSlots(sweepCtx(), grids[1], []string{pos})
	if err != nil || len(first) != 2 {
		t.Fatalf("want both 19:00 entries of the 90-minute grid claimed; got %+v (%v)", first, err)
	}
	if first[0].CourtID != f.CourtID || first[0].StartTime != "19:00" || first[0].CourtName == "" || first[0].ComplexName == "" {
		t.Errorf("claimed entry = %+v", first[0])
	}
	if again, err := st.ClaimFreeSlots(sweepCtx(), grids[1], []string{pos}); err != nil || len(again) != 0 {
		t.Fatalf("a claimed entry is never claimed again; got %d (%v)", len(again), err)
	}
	if n, _ := st.CountWaiting(ctx, f.ComplexID, "carla@example.com"); n != 1 {
		t.Errorf("the 20:30 entry is still waiting; got %d", n)
	}
	if n, _ := st.CountWaiting(ctx, f.ComplexID, "dario@example.com"); n != 1 {
		t.Errorf("the 60-minute grid's entry is still waiting; got %d", n)
	}
	if n, _ := st.CountWaiting(ctx, f.ComplexID, "ana@example.com"); n != 0 {
		t.Errorf("a notified entry is no longer waiting; got %d", n)
	}
}

// B3 regression: an address that was told and missed the place can wait for it
// again; the new wait is a new row with its own notice.
func TestRejoinAfterANoticeWaitsAgain(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	eventID := newEvent(t, f, timezone.Today().AddDate(0, 0, 5), "20:00", 1)
	reg := register(t, f, eventID, "confirmed", time.Hour)
	if err := st.InsertEvent(ctx, f.ComplexID, eventID, "ana@example.com"); err != nil {
		t.Fatal(err)
	}
	exec(t, f, `UPDATE open_play_registrations SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, reg)

	first, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100)
	if err != nil || len(first) != 1 {
		t.Fatalf("first notice: got %d (%v)", len(first), err)
	}

	if err := st.InsertEvent(ctx, f.ComplexID, eventID, "ana@example.com"); err != nil {
		t.Fatalf("re-join: %v", err)
	}
	second, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100)
	if err != nil || len(second) != 1 {
		t.Fatalf("the re-join must wait again and be told again; got %d (%v)", len(second), err)
	}
	if second[0].ID == first[0].ID {
		t.Errorf("the second notice must come from a new entry (a new dedup key); both are %v", first[0].ID)
	}
}

// The daily cleanup drops slot entries of past days and entries of cancelled or
// past events, and keeps the rest.
func TestDeletePastDropsOnlyPlacesThatHaveGone(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	today := timezone.Today()
	for _, d := range []int{-3, 2} {
		if err := st.InsertSlot(ctx, store.NewSlotEntry{ComplexID: f.ComplexID, Email: "ana@example.com",
			CourtID: f.CourtID, Day: today.AddDate(0, 0, d), StartTime: "19:00", DurationMinutes: 60}); err != nil {
			t.Fatal(err)
		}
	}
	past := newEvent(t, f, today.AddDate(0, 0, -2), "20:00", 2)
	cancelled := newEvent(t, f, today.AddDate(0, 0, 2), "20:00", 2)
	ahead := newEvent(t, f, today.AddDate(0, 0, 2), "21:00", 2)
	for _, id := range []uuid.UUID{past, cancelled, ahead} {
		if err := st.InsertEvent(ctx, f.ComplexID, id, "ana@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	exec(t, f, `UPDATE open_play_events SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, cancelled)

	n, err := st.DeletePast(sweepCtx(), today)
	if err != nil || n != 3 {
		t.Fatalf("want the past slot, the past event and the cancelled event dropped; got %d (%v)", n, err)
	}
	if got := countPlaceEntries(t, f); got != 2 {
		t.Errorf("want the future slot and the future event kept; got %d", got)
	}
}

// fiveWaiting is the five addresses the "everyone at once" tests put on a place.
var fiveWaiting = []string{"ana@example.com", "bruno@example.com", "carla@example.com", "dario@example.com", "eva@example.com"}

// claimedEmails is the set of addresses a claim returned.
func claimedEmails[T any](rows []T, email func(T) string) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		out[email(r)] = true
	}
	return out
}

// Five people wait for one taken court slot. When it frees, a single claim
// returns all five, with the court's sport the email link opens, and none of
// them is claimed again.
func TestFiveWaitingForASlotAreAllClaimedInOneSweep(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	day := timezone.Today().AddDate(0, 0, 2)
	for _, email := range fiveWaiting {
		if err := st.InsertSlot(ctx, store.NewSlotEntry{ComplexID: f.ComplexID, Email: email,
			CourtID: f.CourtID, Day: day, StartTime: "19:00", DurationMinutes: 90}); err != nil {
			t.Fatal(err)
		}
	}
	grids, err := st.ListWaitingGrids(sweepCtx(), timezone.Today().AddDate(0, 0, -1), nil, 100)
	if err != nil || len(grids) != 1 {
		t.Fatalf("want one grid; got %+v (%v)", grids, err)
	}

	claimed, err := st.ClaimFreeSlots(sweepCtx(), grids[0], []string{f.CourtID.String() + " 19:00"})
	if err != nil || len(claimed) != 5 {
		t.Fatalf("want the five entries claimed in one sweep; got %d (%v)", len(claimed), err)
	}
	got := claimedEmails(claimed, func(e store.FreedSlotEntry) string { return e.Email })
	for _, email := range fiveWaiting {
		if !got[email] {
			t.Errorf("%s was not claimed", email)
		}
	}
	for _, e := range claimed {
		if e.CourtSport != "padel" || e.StartTime != "19:00" || e.ComplexSlug == "" || e.NotifiedAt.IsZero() {
			t.Errorf("claimed entry = %+v; want the court's sport, the hour and the slug for the link", e)
		}
	}
	if again, err := st.ClaimFreeSlots(sweepCtx(), grids[0], []string{f.CourtID.String() + " 19:00"}); err != nil || len(again) != 0 {
		t.Fatalf("nobody is claimed twice; got %d (%v)", len(again), err)
	}
}

// Five people wait for a full open-play event. When one registration is
// cancelled, a single claim returns all five, not only as many as spots freed.
func TestFiveWaitingForAFullEventAreAllClaimedWhenOneSpotFrees(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	eventID := newEvent(t, f, timezone.Today().AddDate(0, 0, 5), "20:00", 1)
	paid := register(t, f, eventID, "confirmed", time.Hour)
	for _, email := range fiveWaiting {
		if err := st.InsertEvent(ctx, f.ComplexID, eventID, email); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100); err != nil || len(got) != 0 {
		t.Fatalf("a full event frees nothing; got %d (%v)", len(got), err)
	}

	exec(t, f, `UPDATE open_play_registrations SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, paid)
	claimed, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100)
	if err != nil || len(claimed) != 5 {
		t.Fatalf("want the five entries claimed in one sweep; got %d (%v)", len(claimed), err)
	}
	got := claimedEmails(claimed, func(e store.FreedEventEntry) string { return e.Email })
	for _, email := range fiveWaiting {
		if !got[email] {
			t.Errorf("%s was not claimed", email)
		}
	}
	for _, e := range claimed {
		if e.EventID != eventID || e.ComplexSlug == "" || e.NotifiedAt.IsZero() {
			t.Errorf("claimed entry = %+v; want the event, the slug for the link and the notice instant", e)
		}
	}
	if again, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100); err != nil || len(again) != 0 {
		t.Fatalf("nobody is claimed twice; got %d (%v)", len(again), err)
	}
}

// Five people wait for a full tournament category. When a pair drops out, a
// single claim returns all five, each with the tournament the link opens.
func TestFiveWaitingForAFullCategoryAreAllClaimedWhenAPairDropsOut(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	_, eventID, tournamentID := newCategory(t, f, timezone.Today().AddDate(0, 0, 5), 1)
	pair := register(t, f, eventID, "confirmed", time.Hour)
	for _, email := range fiveWaiting {
		if err := st.InsertEvent(ctx, f.ComplexID, eventID, email); err != nil {
			t.Fatal(err)
		}
	}

	exec(t, f, `UPDATE open_play_registrations SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, pair)
	claimed, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100)
	if err != nil || len(claimed) != 5 {
		t.Fatalf("want the five entries claimed in one sweep; got %d (%v)", len(claimed), err)
	}
	for _, e := range claimed {
		if e.TournamentID == nil || *e.TournamentID != tournamentID {
			t.Errorf("claimed entry = %+v; want the tournament for the link", e)
		}
	}
}

// The people told about a freed slot who did not get it wait again once the
// slot is taken: a later free spot reaches them with a new notice. The grid
// stays listed after everyone was told, so the sweep can see it taken again;
// an address that already joined again keeps one waiting entry, not two.
func TestToldEntriesWaitAgainWhenTheirSlotIsTakenAgain(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	day := timezone.Today().AddDate(0, 0, 2)
	pos := []string{f.CourtID.String() + " 19:00"}
	for _, email := range fiveWaiting {
		if err := st.InsertSlot(ctx, store.NewSlotEntry{ComplexID: f.ComplexID, Email: email,
			CourtID: f.CourtID, Day: day, StartTime: "19:00", DurationMinutes: 90}); err != nil {
			t.Fatal(err)
		}
	}
	from := timezone.Today().AddDate(0, 0, -1)
	grids, err := st.ListWaitingGrids(sweepCtx(), from, nil, 100)
	if err != nil || len(grids) != 1 {
		t.Fatalf("want one grid; got %+v (%v)", grids, err)
	}
	if got, err := st.ClaimFreeSlots(sweepCtx(), grids[0], pos); err != nil || len(got) != 5 {
		t.Fatalf("want the five told at once; got %d (%v)", len(got), err)
	}

	// Everyone was told: the grid is still watched, so its slot can be seen taken.
	if again, err := st.ListWaitingGrids(sweepCtx(), from, nil, 100); err != nil || len(again) != 1 {
		t.Fatalf("a grid whose entries were all told must stay watched; got %+v (%v)", again, err)
	}
	// One of them joins again by hand before the sweep notices.
	if err := st.InsertSlot(ctx, store.NewSlotEntry{ComplexID: f.ComplexID, Email: "ana@example.com",
		CourtID: f.CourtID, Day: day, StartTime: "19:00", DurationMinutes: 90}); err != nil {
		t.Fatal(err)
	}

	// Somebody holds the slot: the four still told are put back on the list.
	rearmed, err := st.RearmTakenSlots(sweepCtx(), grids[0], pos)
	if err != nil || rearmed != 4 {
		t.Fatalf("want the four told addresses waiting again; got %d (%v)", rearmed, err)
	}
	for _, email := range fiveWaiting {
		if n, _ := st.CountWaiting(ctx, f.ComplexID, email); n != 1 {
			t.Errorf("%s must have exactly one waiting entry; got %d", email, n)
		}
	}
	if again, err := st.RearmTakenSlots(sweepCtx(), grids[0], pos); err != nil || again != 0 {
		t.Fatalf("a second pass re-arms nothing; got %d (%v)", again, err)
	}

	// The hold runs out: all five are told again, in one claim.
	if got, err := st.ClaimFreeSlots(sweepCtx(), grids[0], pos); err != nil || len(got) != 5 {
		t.Fatalf("want the five told again when it frees; got %d (%v)", len(got), err)
	}
}

// The people told about a freed event spot wait again once the event is full
// again, so a later free spot reaches them with a new notice.
func TestToldEntriesWaitAgainWhenTheirEventIsFullAgain(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	eventID := newEvent(t, f, timezone.Today().AddDate(0, 0, 5), "20:00", 1)
	paid := register(t, f, eventID, "confirmed", time.Hour)
	for _, email := range fiveWaiting {
		if err := st.InsertEvent(ctx, f.ComplexID, eventID, email); err != nil {
			t.Fatal(err)
		}
	}
	exec(t, f, `UPDATE open_play_registrations SET status = 'cancelled', cancelled_at = NOW() WHERE id = $1`, paid)
	if got, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100); err != nil || len(got) != 5 {
		t.Fatalf("want the five told at once; got %d (%v)", len(got), err)
	}
	if n, err := st.RearmFullEvents(sweepCtx(), timezone.Now()); err != nil || n != 0 {
		t.Fatalf("while the spot is still free nobody is re-armed; got %d (%v)", n, err)
	}

	register(t, f, eventID, "held", time.Hour)
	if n, err := st.RearmFullEvents(sweepCtx(), timezone.Now()); err != nil || n != 5 {
		t.Fatalf("want the five waiting again once the event is full; got %d (%v)", n, err)
	}
	if got, err := st.ClaimFreedEvents(sweepCtx(), timezone.Now(), 100); err != nil || len(got) != 0 {
		t.Fatalf("a full event tells nobody; got %d (%v)", len(got), err)
	}
}

// S6: one address waiting on two courts at 19:00, only one of them free, is
// claimed on both in one statement: the free court's entry (Free) and the
// other court's (not Free), so the person gets one email for the hour and the
// second court freeing later in the round finds nothing of theirs waiting.
// Another address's 19:00 on the taken court, and the same address's 20:30,
// are left waiting.
func TestClaimTakesAnAddressSiblingEntriesForTheSameHour(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	var courtB uuid.UUID
	if err := f.DB.QueryRow(ctx, `INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 2') RETURNING id`,
		f.ComplexID).Scan(&courtB); err != nil {
		t.Fatal(err)
	}
	day := timezone.Today().AddDate(0, 0, 3)
	for _, e := range []struct {
		email, start string
		court        uuid.UUID
	}{
		{"ana@example.com", "19:00", f.CourtID}, {"ana@example.com", "19:00", courtB},
		{"ana@example.com", "20:30", courtB}, {"bruno@example.com", "19:00", courtB},
	} {
		if err := st.InsertSlot(ctx, store.NewSlotEntry{ComplexID: f.ComplexID, Email: e.email,
			CourtID: e.court, Day: day, StartTime: e.start, DurationMinutes: 90}); err != nil {
			t.Fatal(err)
		}
	}
	grids, err := st.ListWaitingGrids(sweepCtx(), timezone.Today().AddDate(0, 0, -1), nil, 100)
	if err != nil || len(grids) != 1 {
		t.Fatalf("want one grid; got %+v (%v)", grids, err)
	}

	claimed, err := st.ClaimFreeSlots(sweepCtx(), grids[0], []string{f.CourtID.String() + " 19:00"})
	if err != nil || len(claimed) != 2 {
		t.Fatalf("want Ana's two 19:00 entries claimed; got %+v (%v)", claimed, err)
	}
	free := 0
	for _, e := range claimed {
		if e.Email != "ana@example.com" || e.StartTime != "19:00" {
			t.Errorf("claimed %+v; only Ana's 19:00 entries belong to this claim", e)
		}
		if e.Free != (e.CourtID == f.CourtID) {
			t.Errorf("entry on court %s: Free = %v", e.CourtID, e.Free)
		}
		if e.Free {
			free++
		}
	}
	if free != 1 {
		t.Errorf("want exactly the free court's entry marked Free; got %d", free)
	}
	if n, _ := st.CountWaiting(ctx, f.ComplexID, "ana@example.com"); n != 1 {
		t.Errorf("Ana's 20:30 entry still waits; got %d waiting", n)
	}
	if n, _ := st.CountWaiting(ctx, f.ComplexID, "bruno@example.com"); n != 1 {
		t.Errorf("Bruno's 19:00 on the taken court still waits; got %d waiting", n)
	}
	if again, err := st.ClaimFreeSlots(sweepCtx(), grids[0], []string{courtB.String() + " 19:00"}); err != nil ||
		len(again) != 1 || again[0].Email != "bruno@example.com" {
		t.Fatalf("court 2 freeing later reaches Bruno alone; got %+v (%v)", again, err)
	}
}
