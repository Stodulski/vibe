package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// stubGate is the tournaments domain as the events domain sees it.
type stubGate struct {
	drawn     bool
	withdrawn []uuid.UUID
	err       error
}

func (g *stubGate) DrawGenerated(context.Context, uuid.UUID) (bool, error) { return g.drawn, g.err }

func (g *stubGate) PairWithdrawn(_ context.Context, id uuid.UUID) error {
	g.withdrawn = append(g.withdrawn, id)
	return g.err
}

// tournamentFixture makes the fixture's event the sign-up container of a tournament
// category, three days from now, with a paid pair that signed up long ago.
func tournamentFixture(t *testing.T, drawn bool) (*fixture, *stubGate, string) {
	t.Helper()
	f := newFixture(t)
	tid := uuid.New()
	f.store.event.TournamentID = &tid
	gate := &stubGate{drawn: drawn}
	f.svc.SetTournaments(gate)
	return f, gate, signedUpAndPaid(t, f, 48*time.Hour, 72*time.Hour)
}

// S8: before the draw a pair withdraws under the complex's cancellation policy,
// refunded when it is far enough ahead, and the bracket is untouched.
func TestAPairWithdrawingBeforeTheDrawFollowsThePolicy(t *testing.T) {
	f, gate, token := tournamentFixture(t, false)
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}

	got, err := f.svc.CancelRegistration(t.Context(), visitor(), token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome.Result != paymentstore.RefundIssued || len(f.payments.refunded) != 1 {
		t.Errorf("outcome = %+v refunds = %v; want a refund", got.Outcome, f.payments.refunded)
	}
	if len(gate.withdrawn) != 0 {
		t.Errorf("no draw exists, so no pair leaves a bracket: %v", gate.withdrawn)
	}
}

// S8: after the draw withdrawing is a walkover with no refund, whatever the
// policy window says, and the pair is told to leave its bracket.
func TestAPairWithdrawingAfterTheDrawForfeitsAndKeepsThePayment(t *testing.T) {
	f, gate, token := tournamentFixture(t, true)
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}

	view, err := f.svc.Status(t.Context(), token)
	if err != nil || !view.Cancellation.Walkover || view.Cancellation.WithinWindow || !view.Cancellation.CanCancel {
		t.Fatalf("status = %+v (%v); want cancellable as a walkover, out of the refund window", view.Cancellation, err)
	}
	got, err := f.svc.CancelRegistration(t.Context(), visitor(), token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome.Result != paymentstore.RefundNotEligible || len(f.payments.refunded) != 0 {
		t.Errorf("outcome = %+v refunds = %v; want the payment kept", got.Outcome, f.payments.refunded)
	}
	if len(gate.withdrawn) != 1 || gate.withdrawn[0] != got.View.Registration.ID {
		t.Errorf("the pair must be withdrawn from the bracket: %v", gate.withdrawn)
	}
	if len(f.notify.cancelled) != 1 {
		t.Errorf("the pair is told: %+v", f.notify.cancelled)
	}
}

// A gate that cannot say whether the draw exists stops the withdrawal: guessing the
// refund policy would either keep money that is owed or give back what is not.
func TestAGateThatCannotAnswerStopsTheWithdrawal(t *testing.T) {
	f, gate, token := tournamentFixture(t, true)
	gate.err = errBoom
	if _, err := f.svc.CancelRegistration(t.Context(), visitor(), token); !errors.Is(err, errBoom) {
		t.Fatalf("an unreadable draw must not guess the refund policy: %v", err)
	}
	if len(f.store.released) != 0 {
		t.Errorf("nothing must be cancelled on a failed read: %v", f.store.released)
	}
}

// A tournament category is not an open-play event: the open-play routes neither
// show it nor take a person's sign-up for it.
func TestTournamentEventsAreHiddenFromOpenPlayRoutes(t *testing.T) {
	f, _, _ := tournamentFixture(t, false)
	id := f.store.event.ID
	if _, err := f.svc.PublicGet(t.Context(), "vibe", id); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("public get: %v", err)
	}
	if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", id, signUpInput()); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a single person cannot sign up for a pair category: %v", err)
	}
	if _, err := f.svc.Get(t.Context(), f.complex.ID, id); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("owner get: %v", err)
	}
	if _, err := f.svc.Cancel(t.Context(), f.complex, f.ownerActor(), id, nil); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("owner cancel through the open-play route: %v", err)
	}
	if _, err := f.svc.CancelAny(t.Context(), f.complex, f.ownerActor(), id, nil); err != nil {
		t.Errorf("the tournaments domain cancels its own: %v", err)
	}
}
