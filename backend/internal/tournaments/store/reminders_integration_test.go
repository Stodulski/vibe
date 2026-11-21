//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/timezone"
)

func dueMatches(t *testing.T, f *fixture, now time.Time) map[uuid.UUID]bool {
	t.Helper()
	due, err := f.Stores.Tournaments.DueMatchReminders(data.ContextWithTenantBypass(context.Background()), now)
	if err != nil {
		t.Fatal(err)
	}
	out := map[uuid.UUID]bool{}
	for _, d := range due {
		out[d.MatchID] = true
	}
	return out
}

// S12 + A3: a paid pair of a category without its own hour is reminded the day
// before the tournament's first day, from 09:00, with its partner beside it.
func TestATournamentPairIsDueTheDayBeforeTheFirstDay(t *testing.T) {
	f := newFixture(t)
	_, cat := f.tournament(t, "elimination", 8, 3, 2)
	reg := f.pairs(t, cat, 1)[0]
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE tournament_pairs SET partner_email = 'beto@example.com' WHERE registration_id = $1`, reg.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE open_play_registrations SET confirmed_at = NOW() - interval '7 days' WHERE id = $1`, reg.ID); err != nil {
		t.Fatal(err)
	}
	d := start()
	first := time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, timezone.Argentina)
	read := func(now time.Time) (bool, string, bool) {
		due, err := f.Stores.Events.DueReminders(data.ContextWithTenantBypass(context.Background()), now)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range due {
			if r.RegistrationID == reg.ID {
				email := ""
				if r.PartnerEmail != nil {
					email = *r.PartnerEmail
				}
				return true, email, r.HasTime
			}
		}
		return false, "", false
	}
	if due, _, _ := read(first.Add(-25 * time.Hour)); due {
		t.Error("before 09:00 the day before, the pair is not due yet")
	}
	due, partner, hasTime := read(first.Add(-23 * time.Hour))
	if !due || partner != "beto@example.com" || hasTime {
		t.Errorf("the pair is due with its partner and no hour; due=%v partner=%q hasTime=%v", due, partner, hasTime)
	}
}

// S12: a scheduled match is due its reminder inside the two hours before it on
// Argentina's clock, once; scheduling it again (a new time) re-arms it, and an
// unscheduled match is never due.
func TestDueMatchRemindersFollowTheSchedule(t *testing.T) {
	f := newFixture(t)
	_, _, first := drawn(t, f)
	m, other := first[0], first[1]
	if err := f.schedule(m, f.CourtID, "18:00"); err != nil {
		t.Fatal(err)
	}
	d := start()
	at := time.Date(d.Year(), d.Month(), d.Day(), 18, 0, 0, 0, timezone.Argentina)

	if dueMatches(t, f, at.Add(-3*time.Hour))[m.ID] {
		t.Error("three hours out is not due")
	}
	due := dueMatches(t, f, at.Add(-90*time.Minute))
	if !due[m.ID] || due[other.ID] {
		t.Fatalf("90 minutes out the scheduled match is due and the unscheduled one is not; due = %v", due)
	}
	if err := f.Stores.Tournaments.MarkMatchReminded(data.ContextWithTenantBypass(context.Background()), m.ID); err != nil {
		t.Fatal(err)
	}
	if dueMatches(t, f, at.Add(-90*time.Minute))[m.ID] {
		t.Error("a reminded match is not due twice")
	}

	if err := f.schedule(m, f.CourtID, "20:00"); err != nil {
		t.Fatal(err)
	}
	later := at.Add(2 * time.Hour)
	if !dueMatches(t, f, later.Add(-time.Hour))[m.ID] {
		t.Error("moving the match must re-arm its reminder for the new time")
	}
}
