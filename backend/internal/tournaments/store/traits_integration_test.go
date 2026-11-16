//go:build integration

package store_test

import (
	"errors"
	"testing"

	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

func ptrOf[T any](v T) *T { return &v }

// A category's traits and a pair's declared players are written and read back
// unchanged, and a category without traits reads back with none.
func TestTraitsAndDeclaredPlayersRoundTripThroughTheStore(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournamentOf(t, tournamentstore.NewCategory{
		Name: "Caballeros 6ta", Modality: "men", Level: 6, Prizes: ptrOf("Trofeo"),
		StartsOn: start(), StartsAt: "19:30", Format: tournamentstore.FormatElimination, Capacity: 4, Price: 1_000_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	})
	if cat.Modality == nil || *cat.Modality != "men" || cat.Level == nil || *cat.Level != 6 || cat.Suma != nil {
		t.Fatalf("traits = %v / %v / %v; want men, 6 and no suma", cat.Modality, cat.Level, cat.Suma)
	}
	if cat.Prizes == nil || *cat.Prizes != "Trofeo" || cat.StartsAt == nil || *cat.StartsAt != "19:30" || cat.StartsOn == nil {
		t.Fatalf("prizes = %v; starts = %v %v", cat.Prizes, cat.StartsOn, cat.StartsAt)
	}

	payer, partner := "+549110000001", "+549220000001"
	payerGender, payerLevel := "men", 8
	reg, _, err := f.Stores.Events.SignUp(f.ctx, eventstore.SignUpInput{
		EventID: cat.EventID, FirstName: "Ana", LastName: "Pagadora", Phone: payer, ServiceFee: 20_000,
		IP: "203.0.113.9", MaxHoldsPerIP: 1000, Gender: &payerGender, Level: &payerLevel,
		Hooks: f.Stores.Tournaments.PairHooks(tournamentstore.Player{Phone: payer}, tournamentstore.Player{
			FirstName: "Beto", LastName: "Socio", Phone: partner, Gender: ptrOf("men"), Level: ptrOf(7),
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if reg.Gender == nil || *reg.Gender != "men" || reg.Level == nil || *reg.Level != 8 {
		t.Errorf("registration declared = %v / %v; want men and 8", reg.Gender, reg.Level)
	}
	d := f.detail(t, tr)
	if len(d.Pairs) != 1 {
		t.Fatalf("pairs = %d; want the one pair", len(d.Pairs))
	}
	pr := d.Pairs[0]
	if pr.Payer.Gender == nil || *pr.Payer.Gender != "men" || pr.Payer.Level == nil || *pr.Payer.Level != 8 {
		t.Errorf("payer declared = %v / %v; want men and 8", pr.Payer.Gender, pr.Payer.Level)
	}
	if pr.Partner.Gender == nil || *pr.Partner.Gender != "men" || pr.Partner.Level == nil || *pr.Partner.Level != 7 {
		t.Errorf("partner declared = %v / %v; want men and 7", pr.Partner.Gender, pr.Partner.Level)
	}
}

// Two categories with the same composed name in one tournament are refused with
// the sentinel the handler maps to the stable code.
func TestARepeatedCategoryNameIsTheTaken(t *testing.T) {
	f := newFixture(t)
	nc := tournamentstore.NewCategory{
		Name: "Mixto 6ta", Modality: "mixed", Level: 6, StartsOn: start(), StartsAt: "19:30",
		Format: tournamentstore.FormatElimination, Capacity: 4, Price: 1_000_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	}
	tr := &tournamentstore.Tournament{
		ComplexID: f.ComplexID, Name: "Torneo", Sport: "padel", StartDate: start(), EndDate: start().AddDate(0, 0, 1), CreatedBy: f.UserID,
	}
	err := f.Stores.Tournaments.Create(f.ctx, tr, []tournamentstore.NewCategory{nc, nc})
	if !errors.Is(err, tournamentstore.ErrCategoryTaken) {
		t.Fatalf("err = %v; want ErrCategoryTaken", err)
	}
}
