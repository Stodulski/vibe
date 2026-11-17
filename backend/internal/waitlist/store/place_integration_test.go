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

	if err := st.WaitlistableEvent(ctx, f.ComplexID, future, now); err != nil {
		t.Errorf("a future event: %v", err)
	}
	for name, id := range map[string]uuid.UUID{"started": past, "unknown": uuid.New()} {
		if err := st.WaitlistableEvent(ctx, f.ComplexID, id, now); !errors.Is(err, data.ErrRecordNotFound) {
			t.Errorf("%s event: want ErrRecordNotFound; got %v", name, err)
		}
	}
	if err := st.WaitlistableEvent(ctx, uuid.New(), future, now); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("another complex's event: want ErrRecordNotFound; got %v", err)
	}

	categoryID, eventID, _ := newCategory(t, f, timezone.Today().AddDate(0, 0, 5), 2)
	got, err := st.WaitlistableCategoryEvent(ctx, f.ComplexID, categoryID, now)
	if err != nil || got != eventID {
		t.Errorf("an open category resolves to its event: got %v (%v), want %v", got, err, eventID)
	}
	if err := st.WaitlistableEvent(ctx, f.ComplexID, eventID, now); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a category's container is not an open-play event to wait for; got %v", err)
	}
	exec(t, f, `UPDATE tournament_categories SET draw_generated_at = NOW() WHERE id = $1`, categoryID)
	if _, err := st.WaitlistableCategoryEvent(ctx, f.ComplexID, categoryID, now); !errors.Is(err, data.ErrRecordNotFound) {
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

// S4: slot entries are listed while waiting, and the id claim is one-shot.
func TestSlotEntriesAreListedAndClaimedOnce(t *testing.T) {
	f := datatest.Isolated(t)
	st := newPlaceStore(f)
	ctx := f.Scoped(context.Background())
	day := timezone.Today().AddDate(0, 0, 3)
	if err := st.InsertSlot(ctx, store.NewSlotEntry{ComplexID: f.ComplexID, Email: "ana@example.com",
		CourtID: f.CourtID, Day: day, StartTime: "19:00", DurationMinutes: 90}); err != nil {
		t.Fatal(err)
	}

	waiting, err := st.ListWaitingSlots(sweepCtx(), timezone.Today().AddDate(0, 0, -1), 100)
	if err != nil || len(waiting) != 1 {
		t.Fatalf("want the waiting slot listed; got %d (%v)", len(waiting), err)
	}
	w := waiting[0]
	if w.CourtID != f.CourtID || w.StartTime != "19:00" || w.DurationMinutes != 90 ||
		w.Day.Format(time.DateOnly) != day.Format(time.DateOnly) || w.CourtName == "" {
		t.Errorf("listed entry = %+v", w)
	}

	first, err := st.Claim(sweepCtx(), []uuid.UUID{w.ID})
	if err != nil || len(first) != 1 {
		t.Fatalf("first claim = %v (%v)", first, err)
	}
	second, err := st.Claim(sweepCtx(), []uuid.UUID{w.ID})
	if err != nil || len(second) != 0 {
		t.Fatalf("a claimed entry is never claimed again; got %v (%v)", second, err)
	}
	if again, _ := st.ListWaitingSlots(sweepCtx(), timezone.Today().AddDate(0, 0, -1), 100); len(again) != 0 {
		t.Errorf("a notified entry is no longer waiting; got %d", len(again))
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
