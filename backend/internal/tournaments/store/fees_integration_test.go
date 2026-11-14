//go:build integration

package store_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// courtFeeFixture is a drawn four-pair bracket whose category charges a court
// fee, with a cash session open, and its first-round matches.
type courtFeeFixture struct {
	*fixture
	tr      *tournamentstore.Tournament
	cat     *tournamentstore.Category
	session *cashboxstore.CashSession
	first   []*tournamentstore.Match
}

func newCourtFeeFixture(t *testing.T, fee int) *courtFeeFixture {
	t.Helper()
	return newCourtFeeFixtureOf(t, fee, 4)
}

func newCourtFeeFixtureOf(t *testing.T, fee, pairs int) *courtFeeFixture {
	t.Helper()
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, pairs)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	if fee > 0 {
		if err := f.Stores.Tournaments.SetCourtFee(f.ctx, cat.ID, fee); err != nil {
			t.Fatalf("SetCourtFee: %v", err)
		}
	}
	session := &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: 0}
	if err := f.Stores.Cashbox.OpenSession(f.ctx, session); err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	return &courtFeeFixture{
		fixture: f, tr: tr, cat: cat, session: session,
		first: matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1),
	}
}

func (f *courtFeeFixture) charge(match *tournamentstore.Match, side int, method string) (*tournamentstore.MatchFee, error) {
	return f.Stores.Tournaments.ChargeCourtFee(f.ctx, tournamentstore.ChargeInput{MatchID: match.ID, Side: side, Method: method, By: f.UserID})
}

func (f *courtFeeFixture) undo(match *tournamentstore.Match, side int) error {
	return f.Stores.Tournaments.UndoCourtFee(f.ctx, match.ID, side, f.UserID)
}

func (f *courtFeeFixture) movements(t *testing.T) []*cashboxstore.CashMovement {
	t.Helper()
	got, err := f.Stores.Cashbox.ListMovementsBySession(f.ctx, f.ComplexID, f.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func (f *courtFeeFixture) feeOf(t *testing.T, matchID uuid.UUID, side int) *tournamentstore.MatchFee {
	t.Helper()
	for _, fee := range f.detail(t, f.tr).Fees {
		if fee.MatchID == matchID && fee.Side == side {
			return fee
		}
	}
	return nil
}

// S6c: the amount is the category's at the time of the charge and a charge keeps
// it; every match not charged yet uses the current fee.
func TestACourtFeeChargeKeepsItsAmountWhenTheFeeChanges(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	got, err := f.charge(f.first[0], 1, "cash")
	if err != nil || got.Amount != 150_000 {
		t.Fatalf("charge: %+v %v", got, err)
	}
	if err := f.Stores.Tournaments.SetCourtFee(f.ctx, f.cat.ID, 200_000); err != nil {
		t.Fatal(err)
	}
	if kept := f.feeOf(t, f.first[0].ID, 1); kept == nil || kept.Amount != 150_000 {
		t.Fatalf("a recorded charge must keep its amount: %+v", kept)
	}
	cat, err := f.Stores.Tournaments.Category(f.ctx, f.tr.ID, f.cat.ID)
	if err != nil || cat.CourtFee != 200_000 {
		t.Fatalf("the category carries the new fee: %+v %v", cat, err)
	}
	later, err := f.charge(f.first[1], 2, "transfer")
	if err != nil || later.Amount != 200_000 {
		t.Fatalf("an uncharged match uses the current fee: %+v %v", later, err)
	}
}

// S6c: the money lands in the complex cash exactly like a venue charge: an income
// movement of the open session, counted by the session's totals and the
// monthly report.
func TestACourtFeeChargeLandsInTheCashSessionAndTheReport(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.charge(f.first[0], 2, "transfer"); err != nil {
		t.Fatal(err)
	}
	moves := f.movements(t)
	if len(moves) != 2 {
		t.Fatalf("want 2 movements; got %d", len(moves))
	}
	for _, m := range moves {
		if m.Kind != "income" || m.Category != "tournaments" || m.Amount != 150_000 || m.SessionID != f.session.ID {
			t.Errorf("unexpected movement %+v", m)
		}
	}
	totals, err := f.Stores.Cashbox.SumBySession(f.ctx, f.ComplexID, f.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	byMethod := map[string]int{}
	for _, tot := range totals {
		if tot.Kind == "income" && tot.Category == "tournaments" {
			byMethod[tot.Method] += tot.Total
		}
	}
	if byMethod["cash"] != 150_000 || byMethod["transfer"] != 150_000 {
		t.Errorf("the session totals must carry both charges: %v", byMethod)
	}
	now := time.Now()
	summaries, err := f.Stores.Reports.CashMovementsByCategory(f.ctx, f.ComplexID, now.AddDate(0, 0, -1), now.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	var reported int
	for _, s := range summaries {
		if s.Kind == "income" && s.Category == "tournaments" {
			reported += s.Total
		}
	}
	if reported != 300_000 {
		t.Errorf("the report must count both charges as tournament income; got %d", reported)
	}
}

func TestAPairIsNeverChargedTwiceForTheSameMatch(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatal(err)
	}
	_, err := f.charge(f.first[0], 1, "transfer")
	requireErr(t, err, tournamentstore.ErrFeeCharged)
	if got := len(f.movements(t)); got != 1 {
		t.Fatalf("a refused second charge must not touch the cash; got %d movements", got)
	}
	// The other side and the other match are separate charges.
	if _, err := f.charge(f.first[0], 2, "cash"); err != nil {
		t.Fatalf("the other pair of the match: %v", err)
	}
	if _, err := f.charge(f.first[1], 1, "cash"); err != nil {
		t.Fatalf("the same side of another match: %v", err)
	}
}

func TestByesWalkoversAndMatchesWithoutAFeeChargeNothing(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	// The final has no pairs yet.
	final := matchesOf(f.detail(t, f.tr), tournamentstore.StageBracket, 2)[0]
	_, err := f.charge(final, 1, "cash")
	requireErr(t, err, tournamentstore.ErrNotChargeable)

	if _, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: f.first[0].ID, WalkoverLoser: 2}); err != nil {
		t.Fatal(err)
	}
	_, err = f.charge(f.first[0], 1, "cash")
	requireErr(t, err, tournamentstore.ErrNotChargeable)

	if got := len(f.movements(t)); got != 0 {
		t.Fatalf("nothing may reach the cash; got %d movements", got)
	}
}

func TestAByeChargesNothing(t *testing.T) {
	// Three pairs leave one first-round match without an opponent.
	f := newCourtFeeFixtureOf(t, 150_000, 3)
	var bye *tournamentstore.Match
	for _, m := range f.first {
		if m.Bye {
			bye = m
		}
	}
	if bye == nil {
		t.Fatal("expected a bye in the first round")
	}
	_, err := f.charge(bye, 1, "cash")
	requireErr(t, err, tournamentstore.ErrNotChargeable)
	if got := len(f.movements(t)); got != 0 {
		t.Fatalf("nothing may reach the cash; got %d movements", got)
	}
}

func TestACategoryWithoutACourtFeeChargesNothing(t *testing.T) {
	f := newCourtFeeFixture(t, 0)
	_, err := f.charge(f.first[0], 1, "cash")
	requireErr(t, err, tournamentstore.ErrNoCourtFee)
	// Setting it to zero again turns the fee off for the matches not charged yet.
	if err := f.Stores.Tournaments.SetCourtFee(f.ctx, f.cat.ID, 100_000); err != nil {
		t.Fatal(err)
	}
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatal(err)
	}
	if err := f.Stores.Tournaments.SetCourtFee(f.ctx, f.cat.ID, 0); err != nil {
		t.Fatal(err)
	}
	_, err = f.charge(f.first[1], 1, "cash")
	requireErr(t, err, tournamentstore.ErrNoCourtFee)
	if kept := f.feeOf(t, f.first[0].ID, 1); kept == nil || kept.Amount != 100_000 {
		t.Fatalf("the recorded charge stays: %+v", kept)
	}
}

func TestACourtFeeChargeNeedsAnOpenCashSession(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	if _, err := f.Stores.Cashbox.Close(f.ctx, f.ComplexID, f.session.ID, f.UserID, 0, 0, 0, time.Now(), nil); err != nil {
		t.Fatalf("closing the session: %v", err)
	}
	_, err := f.charge(f.first[0], 1, "cash")
	requireErr(t, err, tournamentstore.ErrNoOpenCashSession)
	if f.feeOf(t, f.first[0].ID, 1) != nil {
		t.Fatal("a refused charge must leave no record")
	}
}

// Undoing a charge is a void of its income movement, like a sale's, so it needs
// the open session; afterwards the pair can be charged again at the current fee.
func TestUndoingACourtFeeChargeVoidsItsCashMovement(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatal(err)
	}
	if err := f.undo(f.first[0], 1); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if f.feeOf(t, f.first[0].ID, 1) != nil {
		t.Fatal("the charge must be gone from the detail")
	}
	moves := f.movements(t)
	if len(moves) != 2 || moves[1].VoidsMovementID == nil || *moves[1].VoidsMovementID != moves[0].ID || moves[1].Kind != "expense" {
		t.Fatalf("undo must write the void of the income: %+v", moves)
	}
	requireErr(t, f.undo(f.first[0], 1), tournamentstore.ErrFeeNotCharged)

	if err := f.Stores.Tournaments.SetCourtFee(f.ctx, f.cat.ID, 180_000); err != nil {
		t.Fatal(err)
	}
	again, err := f.charge(f.first[0], 1, "transfer")
	if err != nil || again.Amount != 180_000 || again.Method != "transfer" {
		t.Fatalf("a charge after an undo: %+v %v", again, err)
	}
	if got := f.feeOf(t, f.first[0].ID, 1); got == nil || got.Amount != 180_000 {
		t.Fatalf("the new charge shows: %+v", got)
	}
}

func TestUndoingNeedsAnOpenCashSessionAndACharge(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	requireErr(t, f.undo(f.first[0], 1), tournamentstore.ErrFeeNotCharged)
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Stores.Cashbox.Close(f.ctx, f.ComplexID, f.session.ID, f.UserID, 150_000, 0, 0, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	requireErr(t, f.undo(f.first[0], 1), tournamentstore.ErrNoOpenCashSession)
	if f.feeOf(t, f.first[0].ID, 1) == nil {
		t.Fatal("a refused undo leaves the charge in place")
	}
}

// A charged pair's matches cannot be redrawn or swapped: the charge belongs to
// the pair that was in the match. Undoing it frees the draw.
func TestADrawChangeIsRefusedWhileAChargeStands(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatal(err)
	}
	err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: f.cat.ID})
	requireErr(t, err, tournamentstore.ErrHasCharges)
	d := f.detail(t, f.tr)
	err = f.Stores.Tournaments.MovePair(f.ctx, f.cat.ID, pairIDs(d)[0], pairIDs(d)[3])
	requireErr(t, err, tournamentstore.ErrHasCharges)
	if err := f.undo(f.first[0], 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: f.cat.ID}); err != nil {
		t.Fatalf("with the charge undone the draw can be remade: %v", err)
	}
}

func TestTheCourtFeeCanBeSetAtCreation(t *testing.T) {
	f := newFixture(t)
	_, cat := f.tournamentOf(t, tournamentstore.NewCategory{
		Name: "7ma", Format: tournamentstore.FormatElimination, Capacity: 8, Price: 1_000_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90, CourtFee: 90_000,
	})
	if cat.CourtFee != 90_000 {
		t.Fatalf("CourtFee = %d", cat.CourtFee)
	}
}

// A movement voided on the Caja screen frees the pair the same way an undo does:
// whether a charge stands is read from the cash, never from a flag that can drift.
func TestAChargeVoidedFromTheCashScreenIsNoLongerACharge(t *testing.T) {
	f := newCourtFeeFixture(t, 150_000)
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatal(err)
	}
	original := f.movements(t)[0]
	void := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: f.session.ID, Kind: "expense", Category: original.Category, Method: original.Method,
		Amount: original.Amount, VoidsMovementID: &original.ID, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(f.ctx, void); err != nil {
		t.Fatalf("voiding from the cash screen: %v", err)
	}
	if f.feeOf(t, f.first[0].ID, 1) != nil {
		t.Fatal("a voided charge must not show as charged")
	}
	if _, err := f.charge(f.first[0], 1, "cash"); err != nil {
		t.Fatalf("the pair can be charged again: %v", err)
	}
}
