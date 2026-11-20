//go:build integration

package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// S2: a pair signs up as one registration: the payer, with the partner beside
// it, holding one spot of the category.
func TestAPairHoldsOneSpotAndCapacityClosesTheCategory(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 2, 3, 2)

	reg, err := f.signUp(cat, 1, "203.0.113.7")
	if err != nil || reg.Status != eventstore.RegistrationHeld || reg.Price != cat.Price {
		t.Fatalf("first pair = %+v (%v); want a held spot at the category's price", reg, err)
	}
	if _, err := f.signUp(cat, 2, "203.0.113.8"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.signUp(cat, 3, "203.0.113.9"); !errors.Is(err, eventstore.ErrEventFull) {
		t.Fatalf("a third pair must be refused, the category is full; got %v", err)
	}
	got, err := f.Stores.Tournaments.Category(f.ctx, tr.ID, cat.ID)
	if err != nil || got.HeldCount != 2 || got.SpotsLeft() != 0 {
		t.Fatalf("category = %+v (%v); want 2 held and no spots left", got, err)
	}
	d := f.detail(t, tr)
	if len(d.Pairs) != 2 || d.Pairs[0].Partner.Phone != "+549220000001" {
		t.Fatalf("pairs = %+v", d.Pairs)
	}
}

// S2: neither player may already be in the category, as payer or as partner.
func TestAPlayerCannotBeInTwoPairsOfTheCategory(t *testing.T) {
	f := newFixture(t)
	_, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	if _, err := f.signUp(cat, 1, "203.0.113.7"); err != nil {
		t.Fatal(err)
	}
	signUpWith := func(payer, partner, ip string) error {
		_, _, err := f.Stores.Events.SignUp(f.ctx, eventstore.SignUpInput{
			EventID: cat.EventID, FirstName: "X", LastName: "Y", Phone: payer, IP: ip, MaxHoldsPerIP: 100,
			Hooks: f.Stores.Tournaments.PairHooks(tournamentstore.Player{Phone: payer}, tournamentstore.Player{FirstName: "P", LastName: "Q", Phone: partner}),
		})
		return err
	}
	// Payer 1's partner is +549220000001; payer 1 is +549110000001.
	if err := signUpWith("+549110009999", "+549110000001", "203.0.113.20"); !errors.Is(err, eventstore.ErrAlreadyRegistered) {
		t.Fatalf("a partner that is already a payer: %v", err)
	}
	if err := signUpWith("+549220000001", "+5491100009998", "203.0.113.21"); !errors.Is(err, eventstore.ErrAlreadyRegistered) {
		t.Fatalf("a payer that is already a partner: %v", err)
	}
	if err := signUpWith("+5491100009997", "+549220000001", "203.0.113.22"); !errors.Is(err, eventstore.ErrAlreadyRegistered) {
		t.Fatalf("a partner that is already a partner: %v", err)
	}
	if err := signUpWith("+5491100009996", "+5491100009995", "203.0.113.23"); err != nil {
		t.Fatalf("two new players: %v", err)
	}
}

// S2/S4: sign-up closes with the draw, and a pair that pays afterwards is not in it.
func TestSignUpClosesWithTheDrawAndALatePaymentIsRefused(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 2)
	late, err := f.signUp(cat, 3, "203.0.113.9") // held while the draw is made
	if err != nil {
		t.Fatal(err)
	}
	f.generate(t, cat, nil)

	if _, err := f.signUp(cat, 4, "203.0.113.10"); !errors.Is(err, eventstore.ErrEventClosed) {
		t.Fatalf("sign-up after the draw: %v", err)
	}
	if err := f.pay(late, "mp-late"); !errors.Is(err, paymentstore.ErrRegistrationUnavailable) {
		t.Fatalf("a payment after the draw must not confirm the pair (it is refunded): %v", err)
	}
	if d := f.detail(t, tr); len(matchesOf(d, tournamentstore.StageBracket, 0)) != 1 {
		t.Fatalf("the draw must not change: %d matches", len(matchesOf(d, tournamentstore.StageBracket, 0)))
	}
}

// S20 (A2): a category stops taking pairs once the tournament's start date has
// begun in Argentina, drawn or not, and says so when it is read.
func TestSignUpClosesOnTheTournamentsStartDate(t *testing.T) {
	f := newFixture(t)
	today := timezone.Today()
	tr := &tournamentstore.Tournament{
		ComplexID: f.ComplexID, Name: "Hoy", Sport: "padel",
		StartDate: time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(today.Year(), today.Month(), today.Day()+1, 0, 0, 0, 0, time.UTC), CreatedBy: f.UserID,
	}
	if err := f.Stores.Tournaments.Create(f.ctx, tr, []tournamentstore.NewCategory{{
		Name: "5ta", Format: tournamentstore.FormatElimination, Capacity: 8, Price: 500_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	}}); err != nil {
		t.Fatal(err)
	}
	cats, err := f.Stores.Tournaments.Categories(f.ctx, tr.ID)
	if err != nil || len(cats) != 1 {
		t.Fatalf("categories: %v %v", cats, err)
	}
	if _, err := f.signUp(cats[0], 1, "203.0.113.7"); !errors.Is(err, eventstore.ErrEventClosed) {
		t.Fatalf("a sign-up on the tournament's start date must be refused as closed; got %v", err)
	}
	if !cats[0].SignUpClosedByDate(time.Now()) {
		t.Error("the category must read as closed by date on the start date")
	}

	// A tournament that starts tomorrow still takes pairs.
	_, open := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	if open.SignUpClosedByDate(time.Now()) {
		t.Error("a category whose tournament starts in days must read as open")
	}
}

// S22: a payment that arrives while the draw is being made waits for it on the
// category's lock, and then sees the draw: the pair is not in the bracket, so
// the payment is refused (and refunded) rather than confirming a pair the
// bracket never got.
func TestAPaymentDuringTheDrawWaitsForItAndIsRefused(t *testing.T) {
	f := datatest.Shared(t)
	ctx := f.Scoped(context.Background())
	tr := &tournamentstore.Tournament{
		ComplexID: f.ComplexID, Name: "Sorteo", Sport: "padel", StartDate: start(), EndDate: start(), CreatedBy: f.UserID,
	}
	if err := f.Stores.Tournaments.Create(ctx, tr, []tournamentstore.NewCategory{{
		Name: "8va", Format: tournamentstore.FormatElimination, Capacity: 8, Price: 500_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	}}); err != nil {
		t.Fatal(err)
	}
	cats, err := f.Stores.Tournaments.Categories(ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	payer := "+549110007777"
	reg, _, err := f.Stores.Events.SignUp(ctx, eventstore.SignUpInput{
		EventID: cats[0].EventID, FirstName: "A", LastName: "B", Phone: payer, ServiceFee: 1000, IP: "198.51.100.77", MaxHoldsPerIP: 100,
		Hooks: f.Stores.Tournaments.PairHooks(tournamentstore.Player{Phone: payer}, tournamentstore.Player{FirstName: "C", LastName: "D", Phone: "+549220007777"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	checkout := &paymentstore.Payment{
		RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee, Method: "mercadopago", Status: "unpaid",
	}
	if err := f.Stores.Payments.Insert(ctx, checkout); err != nil {
		t.Fatal(err)
	}

	// The draw in progress: it holds the category's lock and has written the
	// draw, not yet committed.
	draw, err := f.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = draw.Rollback(context.Background()) }()
	if _, err := draw.Exec(context.Background(),
		`UPDATE tournament_categories SET draw_generated_at = NOW() WHERE id = $1`, cats[0].ID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		mpID := "mp-during-draw-" + uuid.NewString()
		checkout.Status, checkout.MPPaymentID = "deposit_paid", &mpID
		done <- f.Stores.Payments.ConfirmRegistrationPayment(ctx, checkout, reg.ID)
	}()
	select {
	case err := <-done:
		t.Fatalf("the payment must wait for the draw holding the category; it returned %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := draw.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, paymentstore.ErrRegistrationUnavailable) {
		t.Fatalf("after the draw committed the payment must be refused; got %v", err)
	}
}

// S22: a pair cancelling while the draw is being made waits for it on the
// category's lock, then reports the draw: the pair is in the bracket, so no
// refund is marked owed (it is a walkover, decided by the caller).
func TestACancellationDuringTheDrawWaitsForItAndReportsIt(t *testing.T) {
	f := datatest.Shared(t)
	ctx := f.Scoped(context.Background())
	tr := &tournamentstore.Tournament{
		ComplexID: f.ComplexID, Name: "Baja", Sport: "padel", StartDate: start(), EndDate: start(), CreatedBy: f.UserID,
	}
	if err := f.Stores.Tournaments.Create(ctx, tr, []tournamentstore.NewCategory{{
		Name: "9na", Format: tournamentstore.FormatElimination, Capacity: 8, Price: 500_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	}}); err != nil {
		t.Fatal(err)
	}
	cats, err := f.Stores.Tournaments.Categories(ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	payer := "+549110008888"
	reg, _, err := f.Stores.Events.SignUp(ctx, eventstore.SignUpInput{
		EventID: cats[0].EventID, FirstName: "A", LastName: "B", Phone: payer, ServiceFee: 1000, IP: "198.51.100.88", MaxHoldsPerIP: 100,
		Hooks: f.Stores.Tournaments.PairHooks(tournamentstore.Player{Phone: payer}, tournamentstore.Player{FirstName: "C", LastName: "D", Phone: "+549220008888"}),
	})
	if err != nil {
		t.Fatal(err)
	}

	draw, err := f.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = draw.Rollback(context.Background()) }()
	if _, err := draw.Exec(context.Background(),
		`UPDATE tournament_categories SET draw_generated_at = NOW() WHERE id = $1`, cats[0].ID); err != nil {
		t.Fatal(err)
	}

	type result struct {
		res *eventstore.ParticipantCancel
		err error
	}
	done := make(chan result, 1)
	go func() {
		res, err := f.Stores.Events.CancelByParticipant(ctx, reg.ID, true)
		done <- result{res, err}
	}()
	select {
	case r := <-done:
		t.Fatalf("the cancellation must wait for the draw holding the category; it returned %+v %v", r.res, r.err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := draw.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.err != nil || !r.res.Drawn || r.res.Registration.Status != eventstore.RegistrationCancelled {
		t.Fatalf("after the draw committed the cancellation must report it; got %+v %v", r.res, r.err)
	}
	var marked bool
	if err := f.DB.QueryRow(context.Background(),
		`SELECT refund_intent_at IS NOT NULL FROM open_play_registrations WHERE id = $1`, reg.ID).Scan(&marked); err != nil || marked {
		t.Fatalf("a walkover owes no refund; refund_intent_at set = %v (%v)", marked, err)
	}
}

// S2/S4: two pairs racing for the last spot cannot both get it.
func TestPairsRacingForTheLastSpotCannotExceedCapacity(t *testing.T) {
	f := datatest.Shared(t)
	ctx := f.Scoped(context.Background())
	tr := &tournamentstore.Tournament{
		ComplexID: f.ComplexID, Name: "Carrera", Sport: "padel", StartDate: start(), EndDate: start(), CreatedBy: f.UserID,
	}
	if err := f.Stores.Tournaments.Create(ctx, tr, []tournamentstore.NewCategory{{
		Name: "7ma", Format: tournamentstore.FormatElimination, Capacity: 3, Price: 500_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	}}); err != nil {
		t.Fatal(err)
	}
	cats, err := f.Stores.Tournaments.Categories(ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}

	const pairs = 10
	var wg sync.WaitGroup
	results := make([]error, pairs)
	for i := range pairs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payer := fmt.Sprintf("+54911000%04d", i)
			_, _, results[i] = f.Stores.Events.SignUp(ctx, eventstore.SignUpInput{
				EventID: cats[0].EventID, FirstName: "A", LastName: "B", Phone: payer, ServiceFee: 1000,
				IP: fmt.Sprintf("198.51.100.%d", i+1), MaxHoldsPerIP: 100,
				Hooks: f.Stores.Tournaments.PairHooks(tournamentstore.Player{Phone: payer}, tournamentstore.Player{FirstName: "C", LastName: "D", Phone: fmt.Sprintf("+54922000%04d", i)}),
			})
		}()
	}
	wg.Wait()
	won, full := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, eventstore.ErrEventFull):
			full++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if won != 3 || full != pairs-3 {
		t.Fatalf("%d pairs got in and %d were refused; want 3 and %d", won, full, pairs-3)
	}
	var n int
	if err := f.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM tournament_pairs WHERE category_id = $1`, cats[0].ID).Scan(&n); err != nil || n != 3 {
		t.Fatalf("%d pair rows (%v), want 3", n, err)
	}
}

// S18: a pair whose hold ran out lets its players sign up again in another pair.
// A late payment for the old pair is refused as unavailable (and so refunded):
// the same players never hold two paid pairs of the category.
func TestALatePaymentCannotGiveAPlayerTwoPaidPairs(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	old, err := f.signUp(cat, 1, "203.0.113.7") // payer +549110000001, partner +549220000001
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE open_play_registrations SET held_until = NOW() - interval '1 minute' WHERE id = $1`, old.ID); err != nil {
		t.Fatal(err)
	}
	// The old partner now pays as the payer of a new pair.
	payer := "+549220000001"
	again, _, err := f.Stores.Events.SignUp(f.ctx, eventstore.SignUpInput{
		EventID: cat.EventID, FirstName: "Beto", LastName: "Socio", Phone: payer, ServiceFee: 20_000, IP: "203.0.113.8", MaxHoldsPerIP: 100,
		Hooks: f.Stores.Tournaments.PairHooks(tournamentstore.Player{Phone: payer},
			tournamentstore.Player{FirstName: "Caro", LastName: "Nueva", Phone: "+549330000001"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.pay(again, "mp-again-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := f.pay(old, "mp-late-"+uuid.NewString()); !errors.Is(err, paymentstore.ErrRegistrationUnavailable) {
		t.Fatalf("the late payment must be refused as unavailable; got %v", err)
	}
	if got, err := f.Stores.Tournaments.Category(f.ctx, tr.ID, cat.ID); err != nil || got.ConfirmedCount != 1 {
		t.Fatalf("category = %+v (%v); want only the new pair paid", got, err)
	}
}

// Tournament data is tenant data.
func TestTournamentsAreScopedToTheirComplex(t *testing.T) {
	f := newFixture(t)
	tr, _ := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	foreign := data.ContextWithTenant(context.Background(), uuid.New())
	if _, err := f.Stores.Tournaments.Get(foreign, tr.ID); err == nil {
		t.Error("another tenant must not read the tournament")
	}
	if _, err := f.Stores.Tournaments.Cancel(foreign, tr.ID, f.UserID, nil); err == nil {
		t.Error("another tenant must not cancel the tournament")
	}
	if got, err := f.Stores.Tournaments.Get(f.ctx, tr.ID); err != nil || got.Status != tournamentstore.StatusActive {
		t.Errorf("the tournament must be untouched: %+v (%v)", got, err)
	}
}
