//go:build integration

package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// start is a day a few days out, midnight UTC like a parsed request date.
func start() time.Time {
	d := time.Now().AddDate(0, 0, 4)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

type fixture struct {
	*datatest.Fixture
	ctx context.Context
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := datatest.Isolated(t)
	return &fixture{Fixture: f, ctx: f.Scoped(context.Background())}
}

// tournament creates a two-day tournament with one category of the format.
func (f *fixture) tournament(t *testing.T, format string, capacity, groupSize, qualifiers int) (*tournamentstore.Tournament, *tournamentstore.Category) {
	t.Helper()
	return f.tournamentOf(t, tournamentstore.NewCategory{
		Name: "6ta", Format: format, Capacity: capacity, Price: 1_000_000, GroupSize: groupSize, QualifiersPerGroup: qualifiers,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	})
}

// tournamentOf creates a two-day tournament with the one category given.
func (f *fixture) tournamentOf(t *testing.T, nc tournamentstore.NewCategory) (*tournamentstore.Tournament, *tournamentstore.Category) {
	t.Helper()
	tr := &tournamentstore.Tournament{
		ComplexID: f.ComplexID, Name: "Torneo", Sport: "padel", StartDate: start(), EndDate: start().AddDate(0, 0, 1), CreatedBy: f.UserID,
	}
	err := f.Stores.Tournaments.Create(f.ctx, tr, []tournamentstore.NewCategory{nc})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cats, err := f.Stores.Tournaments.Categories(f.ctx, tr.ID)
	if err != nil || len(cats) != 1 {
		t.Fatalf("Categories: %v %v", cats, err)
	}
	return tr, cats[0]
}

// signUp holds a spot for a pair; the payer's phone is +54911000NNNN and the
// partner's +54922000NNNN.
func (f *fixture) signUp(cat *tournamentstore.Category, n int, ip string) (*eventstore.Registration, error) {
	payer := fmt.Sprintf("+54911000%04d", n)
	partner := tournamentstore.Player{FirstName: "Beto", LastName: fmt.Sprintf("Socio%d", n), Phone: fmt.Sprintf("+54922000%04d", n)}
	reg, _, err := f.Stores.Events.SignUp(f.ctx, eventstore.SignUpInput{
		EventID: cat.EventID, FirstName: "Ana", LastName: fmt.Sprintf("Pagadora%d", n), Phone: payer, ServiceFee: 20_000,
		IP: ip, MaxHoldsPerIP: 1000, Hooks: f.Stores.Tournaments.PairHooks(tournamentstore.Player{Phone: payer}, partner),
	})
	return reg, err
}

// pay confirms a held spot the way the webhook does.
func (f *fixture) pay(reg *eventstore.Registration, mpID string) error {
	checkout := &paymentstore.Payment{
		RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee,
		Method: "mercadopago", Status: "unpaid",
	}
	if err := f.Stores.Payments.Insert(f.ctx, checkout); err != nil {
		return err
	}
	checkout.Status, checkout.MPPaymentID = "deposit_paid", &mpID
	return f.Stores.Payments.ConfirmRegistrationPayment(f.ctx, checkout, reg.ID)
}

// pairs signs up and pays n pairs, returning the registrations in sign-up order.
func (f *fixture) pairs(t *testing.T, cat *tournamentstore.Category, n int) []*eventstore.Registration {
	t.Helper()
	regs := make([]*eventstore.Registration, n)
	for i := range n {
		reg, err := f.signUp(cat, i+1, "203.0.113.7")
		if err != nil {
			t.Fatalf("sign-up %d: %v", i+1, err)
		}
		if err := f.pay(reg, fmt.Sprintf("mp-%d-%s", i, uuid.NewString())); err != nil {
			t.Fatalf("pay %d: %v", i+1, err)
		}
		regs[i] = reg
	}
	return regs
}

func (f *fixture) detail(t *testing.T, tr *tournamentstore.Tournament) *tournamentstore.Detail {
	t.Helper()
	d, err := f.Stores.Tournaments.Detail(f.ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// pairIDs are the pair ids in sign-up order.
func pairIDs(d *tournamentstore.Detail) []uuid.UUID {
	out := make([]uuid.UUID, len(d.Pairs))
	for i, p := range d.Pairs {
		out[i] = p.ID
	}
	return out
}

func (f *fixture) generate(t *testing.T, cat *tournamentstore.Category, seeds []uuid.UUID) {
	t.Helper()
	if err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: cat.ID, SeedOrder: seeds}); err != nil {
		t.Fatalf("GenerateDraw: %v", err)
	}
}

func matchesOf(d *tournamentstore.Detail, stage string, round int) []*tournamentstore.Match {
	var out []*tournamentstore.Match
	for _, m := range d.Matches {
		if m.Stage == stage && (round == 0 || m.Round == round) {
			out = append(out, m)
		}
	}
	return out
}

// win loads a straight-sets win for side 1 or 2.
func (f *fixture) win(t *testing.T, m *tournamentstore.Match, side int) *tournamentstore.Match {
	t.Helper()
	sets := []draw.Set{{A: 6, B: 3}, {A: 6, B: 4}}
	if side == 2 {
		sets = []draw.Set{{A: 3, B: 6}, {A: 4, B: 6}}
	}
	got, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: m.ID, Sets: sets})
	if err != nil {
		t.Fatalf("SetResult: %v", err)
	}
	return got
}

func requireErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func requireConflict(t *testing.T, err error, reason string) {
	t.Helper()
	var conflict *eventstore.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != reason {
		t.Fatalf("want a %q conflict; got %v", reason, err)
	}
}
