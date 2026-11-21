package tournaments

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/mailer"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// pairOf adds a paid pair to the fixture's category: the payer and the partner,
// each with an email when given one.
func pairOf(f *fixture, payer, payerEmail, partner, partnerEmail string) *tournamentstore.PairRow {
	opt := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	p := &tournamentstore.PairRow{
		Pair: tournamentstore.Pair{ID: uuid.New(), CategoryID: f.category().ID, RegistrationID: uuid.New(),
			Status: tournamentstore.PairActive, Partner: tournamentstore.Player{FirstName: partner, LastName: "Socio", Email: opt(partnerEmail)}},
		Payer:              tournamentstore.Player{FirstName: payer, LastName: "Diaz", Email: opt(payerEmail)},
		RegistrationStatus: eventsConfirmed,
	}
	f.store.detail.Pairs = append(f.store.detail.Pairs, p)
	return p
}

func noticesTo(ns []notifications.TournamentNotice) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.Email)
	}
	return out
}

// S13: the draw coming out is told to every player of the category's paid pairs
// who left an email; a pair that left, a spot never paid, and other categories
// are not told.
func TestTheDrawIsToldToEveryPlayerOfThePaidPairs(t *testing.T) {
	f := newFixture(t)
	pairOf(f, "Ana", "ana@example.com", "Beto", "beto@example.com")
	pairOf(f, "Carla", "carla@example.com", "Dani", "")
	gone := pairOf(f, "Eva", "eva@example.com", "Fede", "fede@example.com")
	gone.Status = tournamentstore.PairWithdrawn
	unpaid := pairOf(f, "Gabi", "gabi@example.com", "Hugo", "hugo@example.com")
	unpaid.RegistrationStatus = "held"
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New(), CategoryID: f.category().ID}}

	if _, err := f.svc.GenerateDraw(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, DrawInput{}); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(noticesTo(f.notify.notices), ",")
	if got != "ana@example.com,beto@example.com,carla@example.com" {
		t.Fatalf("draw notices went to %q", got)
	}
	for _, n := range f.notify.notices {
		if n.Kind != mailer.TournamentDrawPublished || n.Subject == "" || n.CategoryName != "6ta" ||
			!strings.Contains(n.PageURL, f.store.detail.Tournament.ID.String()) {
			t.Errorf("notice = %+v", n)
		}
	}
	if f.notify.notices[0].FirstName != "Ana" || f.notify.notices[1].FirstName != "Beto" {
		t.Error("each player is greeted by their own name")
	}
}

// S13: remaking the draw is a notice of its own, read as a change; the same draw
// told twice carries the same key, so a retry sends nothing new.
func TestARemadeDrawIsANewNotice(t *testing.T) {
	f := newFixture(t)
	pairOf(f, "Ana", "ana@example.com", "Beto", "")
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New(), CategoryID: f.category().ID}}
	draw := func() {
		t.Helper()
		if _, err := f.svc.GenerateDraw(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, DrawInput{}); err != nil {
			t.Fatal(err)
		}
	}
	draw()
	first := f.notify.notices[0]

	now := time.Now()
	f.category().DrawGeneratedAt = &now
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New(), CategoryID: f.category().ID}}
	draw()
	remade := f.notify.notices[1]
	if remade.Kind != mailer.TournamentDrawRemade || remade.Subject == first.Subject {
		t.Errorf("remade = %+v; want a change notice with its own key (first %q)", remade, first.Subject)
	}

	draw()
	if f.notify.notices[2].Subject != remade.Subject {
		t.Error("the same draw told again must carry the same key, so the queue drops it")
	}
}

// S13: a match getting its court and time is told to the four players, each
// naming the pair across the net; moving it is a notice of its own, and
// scheduling it again where it already was sends nothing.
func TestSchedulingAMatchIsToldToItsPlayers(t *testing.T) {
	f := newFixture(t)
	a := pairOf(f, "Ana", "ana@example.com", "Beto", "beto@example.com")
	b := pairOf(f, "Carla", "carla@example.com", "Dani", "dani@example.com")
	matchID := uuid.New()
	f.store.detail.Matches = []*tournamentstore.Match{{ID: matchID, CategoryID: f.category().ID, Pair1ID: &a.ID, Pair2ID: &b.ID}}
	tid := f.store.detail.Tournament.ID
	in := ScheduleInput{CourtID: f.court.ID, Date: day(3), StartTime: "18:00"}

	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in); err != nil {
		t.Fatal(err)
	}
	if len(f.notify.notices) != 4 {
		t.Fatalf("notices = %v, want the four players", noticesTo(f.notify.notices))
	}
	for _, n := range f.notify.notices {
		mine := n.Email == "ana@example.com" || n.Email == "beto@example.com"
		wantOpponent := "Carla Diaz / Dani Socio"
		if !mine {
			wantOpponent = "Ana Diaz / Beto Socio"
		}
		if n.Kind != mailer.TournamentMatchScheduled || n.Opponent != wantOpponent || n.Court != "Cancha 1" ||
			!strings.Contains(n.When, "18:00") || n.Subject == "" {
			t.Errorf("notice = %+v", n)
		}
	}

	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in); err != nil {
		t.Fatal(err)
	}
	if len(f.notify.notices) != 4 {
		t.Fatalf("scheduling it where it already was must tell nobody; notices = %d", len(f.notify.notices))
	}

	in.StartTime = "20:00"
	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in); err != nil {
		t.Fatal(err)
	}
	if len(f.notify.notices) != 8 {
		t.Fatalf("moving it must tell the four players again; notices = %d", len(f.notify.notices))
	}
	moved := f.notify.notices[4]
	if moved.Kind != mailer.TournamentMatchMoved || !strings.Contains(moved.When, "20:00") || moved.Subject == f.notify.notices[0].Subject {
		t.Errorf("moved = %+v", moved)
	}
}

// S13: a bracket match whose other side is not known yet tells the pair that is,
// with the opponent to be defined.
func TestSchedulingAMatchWithOneSideKnownTellsThatPair(t *testing.T) {
	f := newFixture(t)
	a := pairOf(f, "Ana", "ana@example.com", "Beto", "")
	matchID := uuid.New()
	f.store.detail.Matches = []*tournamentstore.Match{{ID: matchID, CategoryID: f.category().ID, Pair1ID: &a.ID}}
	in := ScheduleInput{CourtID: f.court.ID, Date: day(3), StartTime: "18:00"}
	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, matchID, in); err != nil {
		t.Fatal(err)
	}
	if len(f.notify.notices) != 1 || f.notify.notices[0].Opponent != mailer.TournamentOpponentUnknown {
		t.Errorf("notices = %+v", f.notify.notices)
	}
}

// S12: a scheduled match two hours out is reminded to its players once, and
// marked; a refused reminder stays unmarked for the next tick.
func TestTheMatchReminderSweepRemindsEachMatchOnce(t *testing.T) {
	f := newFixture(t)
	a := pairOf(f, "Ana", "ana@example.com", "Beto", "")
	b := pairOf(f, "Carla", "carla@example.com", "Dani", "")
	matchID := uuid.New()
	// The match's day and clock are read on Argentina's wall clock, as the store
	// writes them, whatever zone the test runs in.
	start := time.Now().Add(90 * time.Minute).In(timezone.Argentina)
	on := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	clock, minutes := start.Format("15:04"), 90
	f.store.detail.Matches = []*tournamentstore.Match{{ID: matchID, CategoryID: f.category().ID, Pair1ID: &a.ID, Pair2ID: &b.ID,
		CourtID: &f.court.ID, ScheduledOn: &on, StartTime: &clock, Duration: &minutes}}
	f.store.due = []tournamentstore.DueMatch{{MatchID: matchID, TournamentID: f.store.detail.Tournament.ID, ComplexID: f.complex.ID}}

	f.notify.refuse = true
	f.svc.RemindMatches(t.Context())
	if len(f.store.reminded) != 0 {
		t.Fatal("a refused reminder must stay unmarked")
	}
	f.notify.refuse = false
	f.svc.RemindMatches(t.Context())
	f.svc.RemindMatches(t.Context())
	if len(f.notify.notices) != 2 || len(f.store.reminded) != 1 {
		t.Fatalf("notices = %v marked = %v; want both payers once", noticesTo(f.notify.notices), f.store.reminded)
	}
	for _, n := range f.notify.notices {
		if n.Kind != mailer.TournamentMatchReminder || !strings.Contains(n.When, clock) || n.Court != "Cancha 1" {
			t.Errorf("reminder = %+v", n)
		}
	}
}
