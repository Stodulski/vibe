package tournaments

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// S17: the owner withdraws one pair of a category through the events domain's
// withdrawal of the category's sign-up container, which refunds and tells it.
func TestTheOwnerWithdrawsOnePairOfACategory(t *testing.T) {
	f := newFixture(t)
	pair := uuid.New()
	f.events.withdrawOutcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 1_020_000}

	got, err := f.svc.WithdrawPair(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, pair)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.events.withdrawn) != 1 || f.events.withdrawn[0] != [2]uuid.UUID{f.category().EventID, pair} {
		t.Errorf("withdrawn = %v; want the pair taken out of the category's event", f.events.withdrawn)
	}
	if got.RegistrationID != pair || got.Outcome.Result != paymentstore.RefundIssued {
		t.Errorf("result = %+v", got)
	}
}

// S17: a pair can be withdrawn while the tournament is not over (mid-tournament
// injuries are real), and not once its last day has passed in Argentina.
func TestAPairCanBeWithdrawnUntilTheTournamentsLastDayPasses(t *testing.T) {
	today := timezone.Today()
	tests := []struct {
		name    string
		lastDay int // days from today
		refused bool
	}{
		{name: "its last day is today", lastDay: 0},
		{name: "its last day was yesterday", lastDay: -1, refused: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			last := today.AddDate(0, 0, tt.lastDay)
			// Stored as a calendar date, the way the store reads it.
			f.store.detail.Tournament.EndDate = data.PgToDate(data.DateToPg(last))
			f.store.detail.Tournament.StartDate = f.store.detail.Tournament.EndDate.AddDate(0, 0, -2)

			_, err := f.svc.WithdrawPair(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, uuid.New())
			var state *StateError
			if refused := errors.As(err, &state); refused != tt.refused {
				t.Fatalf("refused = %v (%v); want %v", refused, err, tt.refused)
			}
			if tt.refused && len(f.events.withdrawn) != 0 {
				t.Error("a refused withdrawal reaches nobody")
			}
		})
	}
}

// S17: no pair is withdrawn from a cancelled category or another complex's
// tournament.
func TestNoPairIsWithdrawnFromACancelledCategoryOrAnotherComplex(t *testing.T) {
	f := newFixture(t)
	other := complexstore.NewComplexForTest(uuid.New(), nil, nil)
	if _, err := f.svc.WithdrawPair(t.Context(), other, f.actor(), f.store.detail.Tournament.ID, f.category().ID, uuid.New()); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("another complex's tournament: want not found; got %v", err)
	}
	f.category().Cancelled = true
	var state *StateError
	if _, err := f.svc.WithdrawPair(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, uuid.New()); !errors.As(err, &state) {
		t.Errorf("a cancelled category: want a state error; got %v", err)
	}
	if len(f.events.withdrawn) != 0 {
		t.Errorf("nothing may be withdrawn: %v", f.events.withdrawn)
	}
}

// S17: the endpoint answers the pair's refund.
func TestWithdrawPairAnswersTheRefund(t *testing.T) {
	f := newFixture(t)
	pair := uuid.New()
	f.events.withdrawOutcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 1_020_000}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	r.SetPathValue("tournamentID", f.store.detail.Tournament.ID.String())
	r.SetPathValue("categoryID", f.category().ID.String())
	r.SetPathValue("registrationID", pair.String())
	r = httpx.ContextSetUser(r, &authstore.User{ID: f.owner, Role: "owner"})
	r = httpx.ContextSetComplex(r, f.complex)

	w := httptest.NewRecorder()
	f.handler().WithdrawPair(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	validate(t, "RegistrationWithdrawal", body)
	if body["registration_id"] != pair.String() {
		t.Errorf("body = %v", body)
	}
}
