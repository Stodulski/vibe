package tournaments

import (
	"context"
	"errors"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// The court fee is a fixed amount per match each pair pays at the complex on
// match day, not online. The category holds the current amount; charging a pair
// snapshots it and writes the money into the open cash session like a counter
// sale (see the store), so everything the cash and the reports already do with
// income applies to it.

// Methods a court fee can be collected with: the counter's own, never the online
// checkout's.
const (
	MethodCash     = "cash"
	MethodTransfer = "transfer"
)

func validCourtFee(fee int) map[string]string {
	if fee < 0 || fee > MaxPrice {
		return map[string]string{"court_fee": "must be an amount in centavos, 0 for none"}
	}
	return nil
}

// SetCourtFee changes what each pair of a category pays per match (0 is none). It
// can change at any time while the tournament is active: a charge already
// recorded keeps the amount it was made at, every other match uses the new one.
func (s *Service) SetCourtFee(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, categoryID uuid.UUID, fee int) (*tournamentstore.Detail, error) {
	if errs := validCourtFee(fee); errs != nil {
		return nil, &ValidationError{Errors: errs}
	}
	cat, err := s.categoryOf(ctx, complex.ID, tournamentID, categoryID)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetCourtFee(ctx, categoryID, fee); err != nil {
		return nil, feeError(err)
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "set_court_fee", entityTournament, categoryID, map[string]int{"from": cat.CourtFee, "to": fee})
	return s.Get(ctx, complex.ID, tournamentID)
}

// categoryOf is a category of a tournament of the complex.
func (s *Service) categoryOf(ctx context.Context, complexID, tournamentID, categoryID uuid.UUID) (*tournamentstore.Category, error) {
	_, cat, err := s.category(ctx, complexID, tournamentID, categoryID)
	return cat, err
}

// ChargeCourtFee records that the pair on a side (1 or 2) of a match paid its
// court fee at the complex, by cash or transfer, at the category's current fee.
func (s *Service) ChargeCourtFee(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, matchID uuid.UUID, side int, method string) (*tournamentstore.Detail, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	errs := map[string]string{}
	if method != MethodCash && method != MethodTransfer {
		errs["method"] = "must be cash or transfer"
	}
	if side != 1 && side != 2 {
		errs["side"] = "must be 1 or 2"
	}
	if len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	if _, err := s.match(ctx, complex.ID, tournamentID, matchID); err != nil {
		return nil, err
	}
	fee, err := s.store.ChargeCourtFee(ctx, tournamentstore.ChargeInput{MatchID: matchID, Side: side, Method: method, By: *actor.UserID})
	if err != nil {
		return nil, feeError(err)
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "charge_court_fee", entityTournament, matchID, map[string]any{"side": side, "method": method, "amount": fee.Amount})
	return s.Get(ctx, complex.ID, tournamentID)
}

// UndoCourtFee takes back a charge recorded by mistake. It is a void in the
// complex's open cash session, so it needs one.
func (s *Service) UndoCourtFee(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, matchID uuid.UUID, side int) (*tournamentstore.Detail, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	if side != 1 && side != 2 {
		return nil, &ValidationError{Errors: map[string]string{"side": "must be 1 or 2"}}
	}
	if _, err := s.match(ctx, complex.ID, tournamentID, matchID); err != nil {
		return nil, err
	}
	if err := s.store.UndoCourtFee(ctx, matchID, side, *actor.UserID); err != nil {
		return nil, feeError(err)
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "undo_court_fee", entityTournament, matchID, map[string]int{"side": side})
	return s.Get(ctx, complex.ID, tournamentID)
}

// feeError turns what the store refused a court fee change with into the error
// the handler maps.
func feeError(err error) error {
	switch {
	case errors.Is(err, tournamentstore.ErrNoCourtFee):
		return &StateError{Message: "the category has no court fee"}
	case errors.Is(err, tournamentstore.ErrNotChargeable):
		return &StateError{Message: "this pair owes no court fee for the match (a bye, a walkover, or a pair not known yet)"}
	case errors.Is(err, tournamentstore.ErrFeeCharged):
		return &ConflictError{Message: "the court fee was already charged for this pair and match"}
	case errors.Is(err, tournamentstore.ErrFeeNotCharged):
		return &ConflictError{Message: "the court fee is not charged for this pair and match"}
	case errors.Is(err, tournamentstore.ErrNoOpenCashSession):
		return &ConflictError{Message: "no cash session is open"}
	case errors.Is(err, tournamentstore.ErrCancelled):
		return &StateError{Message: "the tournament or category was cancelled"}
	}
	return err
}
