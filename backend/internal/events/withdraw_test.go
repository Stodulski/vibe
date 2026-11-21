package events

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// paidParticipant signs a paid person up for the fixture's event, which starts in
// startsIn, long enough ago that the complex's refund window is long closed for
// them: the owner's withdrawal must refund regardless.
func paidParticipant(t *testing.T, f *fixture, startsIn time.Duration) *eventstore.Registration {
	t.Helper()
	tok := signedUpAndPaid(t, f, 30*24*time.Hour, startsIn)
	reg := f.store.byToken[tok]
	email := "ana@example.com"
	reg.Email = &email
	return reg
}

// S17/A9: the owner withdraws one participant: the spot ends as the owner's,
// the whole payment is refunded even outside the cancellation window, the person
// is told by the owner's notice quoting the refund, and the owner is not told of
// their own action.
func TestTheOwnerWithdrawingAParticipantRefundsInFullAndTellsThem(t *testing.T) {
	f := newFixture(t)
	// Two hours out: far inside the complex's 24-hour window, so a person leaving
	// by themselves would keep no refund.
	reg := paidParticipant(t, f, 2*time.Hour)
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}

	got, err := f.svc.Withdraw(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Status != eventstore.RegistrationCancelled || reg.CancelledBy == nil || *reg.CancelledBy != eventstore.CancelledByOwner {
		t.Errorf("registration = %s by %v; want cancelled by the owner", reg.Status, reg.CancelledBy)
	}
	if len(f.store.owed) != 1 || !f.store.owed[0] {
		t.Errorf("the refund must be marked owed with the cancellation; owed = %v", f.store.owed)
	}
	if len(f.payments.refunded) != 1 || f.payments.refunded[0] != reg.ID {
		t.Errorf("refunds = %v; want the participant's money sent back", f.payments.refunded)
	}
	if got.RegistrationID != reg.ID || got.Outcome.Result != paymentstore.RefundIssued {
		t.Errorf("result = %+v", got)
	}
	if len(f.notify.cancelled) != 1 {
		t.Fatalf("the person gets one notice; got %d", len(f.notify.cancelled))
	}
	n := f.notify.cancelled[0]
	if !n.ByOwner || n.Email != "ana@example.com" || n.RefundAmount == "" || n.RegistrationID != reg.ID.String() {
		t.Errorf("notice = %+v; want the owner's cancellation quoting the refund", n)
	}
	if len(f.notify.ownerCancelled) != 0 {
		t.Errorf("the owner is not told of their own action: %+v", f.notify.ownerCancelled)
	}
}

// S17: a participant already gone cannot be withdrawn again, and no second
// refund or notice goes out.
func TestASecondWithdrawalIsRefusedWithoutASecondRefund(t *testing.T) {
	f := newFixture(t)
	reg := paidParticipant(t, f, 72*time.Hour)
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}
	if _, err := f.svc.Withdraw(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID); err != nil {
		t.Fatal(err)
	}

	_, err := f.svc.Withdraw(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID)
	var state *StateError
	if !errors.As(err, &state) {
		t.Fatalf("a second withdrawal is a state error; got %v", err)
	}
	if len(f.payments.refunded) != 1 || len(f.notify.cancelled) != 1 {
		t.Errorf("refunds = %v notices = %d; want one of each", f.payments.refunded, len(f.notify.cancelled))
	}
}

// S17: the owner reaches only their own event's participants: another
// complex's event, or a registration of another event, is not found.
func TestTheOwnerCannotWithdrawSomeoneOutsideTheirEvent(t *testing.T) {
	f := newFixture(t)
	reg := paidParticipant(t, f, 72*time.Hour)

	other := complexstore.NewComplexForTest(uuid.New(), nil, nil)
	if _, err := f.svc.Withdraw(t.Context(), other, f.ownerActor(), f.store.event.ID, reg.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("another complex's owner: want not found; got %v", err)
	}
	reg.EventID = uuid.New()
	if _, err := f.svc.Withdraw(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a registration of another event: want not found; got %v", err)
	}
	if len(f.store.released) != 0 || len(f.payments.refunded) != 0 {
		t.Errorf("nothing may be cancelled or refunded: released %v refunded %v", f.store.released, f.payments.refunded)
	}
}

// S17: an open-play participant can be withdrawn until the event starts, not
// after.
func TestAnOpenPlayParticipantCannotBeWithdrawnOnceTheEventStarted(t *testing.T) {
	tests := []struct {
		name     string
		startsIn time.Duration
		refused  bool
	}{
		{name: "a few minutes before the start", startsIn: 5 * time.Minute},
		{name: "a few minutes after the start", startsIn: -5 * time.Minute, refused: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			reg := paidParticipant(t, f, tt.startsIn)
			_, err := f.svc.Withdraw(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID)
			var state *StateError
			if refused := errors.As(err, &state); refused != tt.refused {
				t.Fatalf("refused = %v (%v); want %v", refused, err, tt.refused)
			}
			if tt.refused && (len(f.store.released) != 0 || len(f.payments.refunded) != 0) {
				t.Errorf("a refused withdrawal cancels and refunds nothing")
			}
		})
	}
}

// S17: the open-play route does not reach a tournament category's sign-ups;
// those are withdrawn through the tournament.
func TestTheOpenPlayRouteDoesNotWithdrawATournamentPair(t *testing.T) {
	f, _, token := tournamentFixture(t, false)
	reg := f.store.byToken[token]
	if _, err := f.svc.Withdraw(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("want not found; got %v", err)
	}
}

// S17/A9: a pair withdrawn by the owner after the draw leaves the bracket the
// way a pair leaving by itself does, but is still refunded in full.
func TestTheOwnerWithdrawingADrawnPairTakesItOutOfTheBracketAndRefunds(t *testing.T) {
	f, gate, token := tournamentFixture(t, true)
	f.store.drawnAtCancel = true
	reg := f.store.byToken[token]
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}

	got, err := f.svc.WithdrawAny(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gate.withdrawn) != 1 || gate.withdrawn[0] != reg.ID {
		t.Errorf("the pair must leave the bracket; withdrawn = %v", gate.withdrawn)
	}
	if got.Outcome.Result != paymentstore.RefundIssued || len(f.payments.refunded) != 1 {
		t.Errorf("outcome = %+v refunds = %v; want the full refund after the draw too", got.Outcome, f.payments.refunded)
	}
}

// S17: before the draw there is no bracket to leave.
func TestTheOwnerWithdrawingAPairBeforeTheDrawLeavesTheBracketAlone(t *testing.T) {
	f, gate, token := tournamentFixture(t, false)
	reg := f.store.byToken[token]
	if _, err := f.svc.WithdrawAny(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, reg.ID); err != nil {
		t.Fatal(err)
	}
	if len(gate.withdrawn) != 0 {
		t.Errorf("no draw, no walkovers: %v", gate.withdrawn)
	}
}

// S17: the endpoint answers the participant's refund, 400 for a second
// withdrawal and 404 for a malformed id.
func TestWithdrawAnswersTheRefund(t *testing.T) {
	f := newFixture(t)
	reg := paidParticipant(t, f, 72*time.Hour)
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}
	h := newHandler(f)
	params := map[string]string{"eventID": f.store.event.ID.String(), "registrationID": reg.ID.String()}

	w := httptest.NewRecorder()
	h.Withdraw(w, f.ownerRequest(t, http.MethodPost, "/", "", params))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	refund, _ := body["refund"].(map[string]any)
	if body["registration_id"] != reg.ID.String() || refund["status"] != "issued" || refund["amount"] != float64(820000) {
		t.Errorf("body = %v", body)
	}

	w = httptest.NewRecorder()
	h.Withdraw(w, f.ownerRequest(t, http.MethodPost, "/", "", params))
	if w.Code != http.StatusBadRequest {
		t.Errorf("a second withdrawal is 400; got %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.Withdraw(w, f.ownerRequest(t, http.MethodPost, "/", "", map[string]string{"eventID": f.store.event.ID.String(), "registrationID": "nope"}))
	if w.Code != http.StatusNotFound {
		t.Errorf("a malformed id is 404; got %d", w.Code)
	}
}
