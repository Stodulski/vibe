package tournaments

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// S6c: a category created with a court fee keeps it; none means no fee, and a
// negative or absurd amount is refused on the category's field.
func TestCreateTakesTheCourtFeeOfEachCategory(t *testing.T) {
	f := newFixture(t)
	in := validCreate()
	in.Categories[1].CourtFee = 150_000
	if _, err := f.svc.Create(t.Context(), f.complex, f.actor(), in); err != nil {
		t.Fatal(err)
	}
	if got := f.store.createdCats; got[0].CourtFee != 0 || got[1].CourtFee != 150_000 {
		t.Errorf("court fees = %d, %d; want 0 and 150000", got[0].CourtFee, got[1].CourtFee)
	}
	for _, bad := range []int{-1, MaxPrice + 1} {
		in := validCreate()
		in.Categories[0].CourtFee = bad
		_, err := f.svc.Create(t.Context(), f.complex, f.actor(), in)
		if fieldErrors(t, err)["categories[0].court_fee"] == "" {
			t.Errorf("a court fee of %d must be refused on categories[0].court_fee", bad)
		}
	}
}

// S6c: the owner can change the fee at any time while the tournament is active.
func TestTheCourtFeeCanBeChangedAndZeroTurnsItOff(t *testing.T) {
	f := newFixture(t)
	tid, cid := f.store.detail.Tournament.ID, f.category().ID
	for _, fee := range []int{200_000, 0} {
		if _, err := f.svc.SetCourtFee(t.Context(), f.complex, f.actor(), tid, cid, fee); err != nil {
			t.Fatalf("fee %d: %v", fee, err)
		}
	}
	if got := f.store.feeSet; len(got) != 2 || got[0] != 200_000 || got[1] != 0 {
		t.Errorf("store saw %v", got)
	}
	if len(f.audit.entries) != 2 || f.audit.entries[0].Action != "set_court_fee" {
		t.Errorf("each change is audited: %+v", f.audit.entries)
	}
	for _, bad := range []int{-5, MaxPrice + 1} {
		_, err := f.svc.SetCourtFee(t.Context(), f.complex, f.actor(), tid, cid, bad)
		if fieldErrors(t, err)["court_fee"] == "" {
			t.Errorf("%d must be refused", bad)
		}
	}
	if _, err := f.svc.SetCourtFee(t.Context(), f.complex, f.actor(), tid, uuid.New(), 100); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("another category is not found; got %v", err)
	}
}

func TestChargingValidatesTheMethodAndTheSideAndMapsTheRefusals(t *testing.T) {
	f := newFixture(t)
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New()}}
	mid, tid := f.store.detail.Matches[0].ID, f.store.detail.Tournament.ID

	for _, method := range []string{"", "mercadopago", "debit_card", "CASH"} {
		_, err := f.svc.ChargeCourtFee(t.Context(), f.complex, f.actor(), tid, mid, 1, method)
		if fieldErrors(t, err)["method"] == "" {
			t.Errorf("method %q must be refused", method)
		}
	}
	for _, side := range []int{0, 3} {
		_, err := f.svc.ChargeCourtFee(t.Context(), f.complex, f.actor(), tid, mid, side, "cash")
		if fieldErrors(t, err)["side"] == "" {
			t.Errorf("side %d must be refused", side)
		}
	}
	if len(f.store.charges) != 0 {
		t.Fatalf("a refused request reaches no store: %+v", f.store.charges)
	}

	cases := []struct {
		err      error
		conflict bool
	}{
		{tournamentstore.ErrNoCourtFee, false},
		{tournamentstore.ErrNotChargeable, false},
		{tournamentstore.ErrCancelled, false},
		{tournamentstore.ErrFeeCharged, true},
		{tournamentstore.ErrNoOpenCashSession, true},
	}
	for _, tc := range cases {
		f.store.feeErr = tc.err
		_, err := f.svc.ChargeCourtFee(t.Context(), f.complex, f.actor(), tid, mid, 1, "cash")
		var conflict *ConflictError
		var state *StateError
		if tc.conflict && !errors.As(err, &conflict) || !tc.conflict && !errors.As(err, &state) {
			t.Errorf("%v mapped to %T %v", tc.err, err, err)
		}
	}
}

func TestAChargeIsPassedOnAuditedAndTheMatchMustBeTheTournaments(t *testing.T) {
	f := newFixture(t)
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New()}}
	mid, tid := f.store.detail.Matches[0].ID, f.store.detail.Tournament.ID

	if _, err := f.svc.ChargeCourtFee(t.Context(), f.complex, f.actor(), tid, mid, 2, "transfer"); err != nil {
		t.Fatal(err)
	}
	got := f.store.charges
	if len(got) != 1 || got[0].MatchID != mid || got[0].Side != 2 || got[0].Method != "transfer" || got[0].By != f.owner {
		t.Errorf("charge = %+v", got)
	}
	if last := f.audit.entries[len(f.audit.entries)-1]; last.Action != "charge_court_fee" {
		t.Errorf("audit = %+v", last)
	}
	if _, err := f.svc.ChargeCourtFee(t.Context(), f.complex, f.actor(), tid, uuid.New(), 1, "cash"); err == nil {
		t.Error("a match of no tournament of the complex is not found")
	}
	if len(f.store.charges) != 1 {
		t.Error("an unknown match must not reach the store")
	}
}

func TestUndoingIsPassedOnAndItsRefusalsMapped(t *testing.T) {
	f := newFixture(t)
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New()}}
	mid, tid := f.store.detail.Matches[0].ID, f.store.detail.Tournament.ID

	if _, err := f.svc.UndoCourtFee(t.Context(), f.complex, f.actor(), tid, mid, 1); err != nil {
		t.Fatal(err)
	}
	if got := f.store.undone; len(got) != 1 || got[0] != 1 {
		t.Errorf("undone = %v", got)
	}
	if _, err := f.svc.UndoCourtFee(t.Context(), f.complex, f.actor(), tid, mid, 5); fieldErrors(t, err)["side"] == "" {
		t.Error("side 5 must be refused")
	}
	for _, e := range []error{tournamentstore.ErrFeeNotCharged, tournamentstore.ErrNoOpenCashSession} {
		f.store.feeErr = e
		var conflict *ConflictError
		if _, err := f.svc.UndoCourtFee(t.Context(), f.complex, f.actor(), tid, mid, 1); !errors.As(err, &conflict) {
			t.Errorf("%v mapped to %T", e, err)
		}
	}
}

func TestADrawChangeRefusedByAChargeIsAConflict(t *testing.T) {
	f := newFixture(t)
	f.store.drawErr = tournamentstore.ErrHasCharges
	_, err := f.svc.GenerateDraw(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, DrawInput{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || !strings.Contains(conflict.Message, "court fee") {
		t.Fatalf("got %T %v", err, err)
	}
}

// S6c wire: what a pair owes shows per side with the current fee while uncharged,
// and the charge keeps its own amount, method and time once made.
func TestTheOwnerWireShowsWhatEachPairOwesForTheCourt(t *testing.T) {
	f := newFixture(t)
	d := richDetail(f)
	cat := d.Categories[1]
	cat.CourtFee = 200_000
	play, walk, bye := d.Matches[0], d.Matches[1], d.Matches[2]
	at, by := time.Now(), uuid.New()
	d.Fees = []*tournamentstore.MatchFee{{ID: uuid.New(), MatchID: play.ID, Side: 1, PairID: *play.Pair1ID, Amount: 150_000, Method: "cash", ChargedAt: at, ChargedBy: by}}

	wire := toWireOwnerMatches(d, nil)
	byID := map[uuid.UUID]wireOwnerMatch{}
	for _, m := range wire {
		byID[m.ID] = m
	}
	sides := byID[play.ID].CourtFee
	if s := sides[0]; !s.Charged || s.Amount != 150_000 || s.Chargeable || s.Method == nil || *s.Method != "cash" || *s.ChargedBy != by {
		t.Errorf("a charged side keeps its snapshot: %+v", s)
	}
	if s := sides[1]; s.Charged || s.Amount != 200_000 || !s.Chargeable || s.Method != nil {
		t.Errorf("an uncharged side owes the current fee: %+v", s)
	}
	for name, m := range map[string]wireOwnerMatch{"walkover": byID[walk.ID], "bye": byID[bye.ID]} {
		for _, s := range m.CourtFee {
			if s.Chargeable || s.Charged {
				t.Errorf("a %s charges nothing: %+v", name, s)
			}
		}
	}

	d.Categories[1].CourtFee = 0
	if s := toWireOwnerMatches(d, nil)[0].CourtFee[1]; s.Chargeable || s.Amount != 0 {
		t.Errorf("a category without a fee charges nothing: %+v", s)
	}
}

func TestTheOwnerWireMatchesTheDocumentAndThePublicOneCarriesNoCourtFee(t *testing.T) {
	f := newFixture(t)
	d := richDetail(f)
	d.Categories[1].CourtFee = 200_000
	play := d.Matches[0]
	d.Fees = []*tournamentstore.MatchFee{{ID: uuid.New(), MatchID: play.ID, Side: 2, PairID: *play.Pair2ID, Amount: 150_000, Method: "transfer", ChargedAt: time.Now(), ChargedBy: uuid.New()}}
	h := f.handler()

	validate(t, "TournamentDetail", envelopeOf(h.ownerDetail(t.Context(), f.complex.ID, d)))
	validate(t, "TournamentSummary", wireOwnerSummary{toWireTournament(&d.Tournament), toWireOwnerCategories(d.Categories)})
	validate(t, "TournamentOwnerMatch", toWireOwnerMatches(d, nil)[0])

	raw, _ := json.Marshal(envelopeOf(h.publicDetail(t.Context(), f.complex.ID, d)))
	if strings.Contains(string(raw), "court_fee") {
		t.Error("the public view must not carry the court fee")
	}
}
