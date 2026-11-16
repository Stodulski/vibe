package tournaments

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/events"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

func pairInput() PairSignUpInput {
	return PairSignUpInput{
		Payer:   events.SignUpInput{FirstName: "Ana", LastName: "Diaz", Phone: "+5491100000001"},
		Partner: events.SignUpInput{FirstName: "Beto", LastName: "Socio", Phone: "+5491100000002", Email: "beto@example.com"},
	}
}

// S2: one person signs up both players: the sign-up is the events domain's, with
// the pair's hooks carrying the partner, run in the same transaction.
func TestSignUpHoldsOneSpotForThePairThroughTheEventsDomain(t *testing.T) {
	f := newFixture(t)
	res, err := f.svc.SignUp(t.Context(), events.Actor{IP: "198.51.100.9"}, "vibe", f.store.detail.Tournament.ID, f.category().ID, pairInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.events.signUps) != 1 || f.events.signUps[0].Phone != "+5491100000001" || f.events.signUps[0].FirstName != "Ana" {
		t.Errorf("the payer is the one who signs up: %+v", f.events.signUps)
	}
	if len(f.store.hooksFor) != 1 || f.store.hooksFor[0] != "+5491100000001++5491100000002" || f.events.hooks[0] == nil {
		t.Errorf("the pair's hooks must name the payer and the partner: %v", f.store.hooksFor)
	}
	if res.Category.ID != f.category().ID || res.Tournament.ID != f.store.detail.Tournament.ID {
		t.Errorf("result = %+v", res)
	}
}

func TestSignUpRefusesTheSamePhoneForBothPlayers(t *testing.T) {
	f := newFixture(t)
	in := pairInput()
	in.Partner.Phone = in.Payer.Phone
	_, err := f.svc.SignUp(t.Context(), events.Actor{}, "vibe", f.store.detail.Tournament.ID, f.category().ID, in)
	if fieldErrors(t, err)["partner.phone"] == "" {
		t.Errorf("err = %v", err)
	}
	if len(f.events.signUps) != 0 {
		t.Error("no spot may be held")
	}
}

// S2/S4: sign-up is closed for a cancelled tournament or category and once the draw exists.
func TestSignUpIsClosedAfterTheDrawAndForCancelledThings(t *testing.T) {
	cases := map[string]func(*fixture){
		"draw generated":       func(f *fixture) { now := f.store.detail.Tournament.StartDate; f.category().DrawGeneratedAt = &now },
		"category cancelled":   func(f *fixture) { f.category().Cancelled = true },
		"tournament cancelled": func(f *fixture) { f.store.detail.Tournament.Status = tournamentstore.StatusCancelled },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			change(f)
			_, err := f.svc.SignUp(t.Context(), events.Actor{}, "vibe", f.store.detail.Tournament.ID, f.category().ID, pairInput())
			if !errors.Is(err, events.ErrEventClosed) {
				t.Errorf("err = %v, want ErrEventClosed", err)
			}
			if len(f.events.signUps) != 0 {
				t.Error("no spot may be held")
			}
		})
	}
	f := newFixture(t)
	f.events.signUpErr = events.ErrEventFull
	if _, err := f.svc.SignUp(t.Context(), events.Actor{}, "vibe", f.store.detail.Tournament.ID, f.category().ID, pairInput()); !errors.Is(err, events.ErrEventFull) {
		t.Errorf("capacity refusals pass through: %v", err)
	}
	if _, err := f.svc.SignUp(t.Context(), events.Actor{}, "vibe", f.store.detail.Tournament.ID, uuid.New(), pairInput()); err == nil {
		t.Error("a category of another tournament")
	}
}

// The tournaments domain answers the events domain's questions about a pair.
func TestTheGateAnswersForTheEventsDomain(t *testing.T) {
	f := newFixture(t)
	var _ events.TournamentGate = f.svc
	f.store.drawn = true
	if drawn, err := f.svc.DrawGenerated(t.Context(), f.category().EventID); err != nil || !drawn {
		t.Errorf("DrawGenerated = %v, %v", drawn, err)
	}
	if err := f.svc.PairWithdrawn(t.Context(), uuid.New()); err != nil {
		t.Error(err)
	}
}
