package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// incomeCategory is the cash category a court fee lands in: the owner's
// tournament income, as the cash screen and the monthly report already name it.
const incomeCategory = "tournaments"

// SetCourtFee sets what each pair pays per match in the category, in centavos
// (0 is none). It can change at any time while the tournament is active; the
// charges already recorded keep the amount they were made at.
func (m *Store) SetCourtFee(ctx context.Context, categoryID uuid.UUID, fee int) error {
	return m.inCategory(ctx, categoryID, func(e *engine, cat db.TournamentCategory) error {
		//nolint:gosec // G115: validated by the service (0..MaxPrice).
		if err := e.q.SetCategoryCourtFee(e.ctx, db.SetCategoryCourtFeeParams{ID: cat.ID, CourtFee: int32(fee)}); err != nil {
			return fmt.Errorf("tournaments: set court fee: %w", err)
		}
		return nil
	})
}

// ChargeInput is an owner collecting a pair's court fee for a match at the
// complex. Side is 1 or 2, the match's pair1 or pair2; Method is cash or
// transfer.
type ChargeInput struct {
	MatchID uuid.UUID
	Side    int
	Method  string
	By      uuid.UUID
}

// ChargeCourtFee records that a pair paid its court fee for a match, at the
// category's current fee. Like a counter sale it writes one income movement of
// the complex's open cash session and a row linking to it, in one transaction,
// so the money is in the cash session's totals and the monthly report.
//
// It answers ErrNoCourtFee when the category has no fee, ErrNotChargeable for a
// bye, a walkover or a side whose pair is not known yet, ErrFeeCharged when the
// pair was already charged for the match and ErrNoOpenCashSession when there is
// nowhere for the money to land. Nothing is written on a refusal.
func (m *Store) ChargeCourtFee(ctx context.Context, in ChargeInput) (*MatchFee, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var out *MatchFee
	err := m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		match, err := q.GetTournamentMatch(ctx, data.UUIDToPg(in.MatchID))
		if err != nil {
			return matchReadError(err)
		}
		return m.lockedCategory(ctx, q, match.CategoryID, func(e *engine, cat db.TournamentCategory) error {
			out, err = e.charge(cat, in)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (e *engine) charge(cat db.TournamentCategory, in ChargeInput) (*MatchFee, error) {
	// Read again under the lock: a result may have landed meanwhile.
	match, err := e.q.GetTournamentMatch(e.ctx, data.UUIDToPg(in.MatchID))
	if err != nil {
		return nil, matchReadError(err)
	}
	if cat.CourtFee <= 0 {
		return nil, ErrNoCourtFee
	}
	pair := match.Pair1ID
	if in.Side == 2 {
		pair = match.Pair2ID
	}
	if (in.Side != 1 && in.Side != 2) || match.Bye || match.WalkoverLoser.Valid || !pair.Valid {
		return nil, ErrNotChargeable
	}
	if err := e.requireNoLiveFee(match.ID, in.Side); err != nil {
		return nil, err
	}
	session, err := e.openSession(cat.ComplexID)
	if err != nil {
		return nil, err
	}
	t, err := e.q.GetTournament(e.ctx, db.GetTournamentParams{ID: cat.TournamentID})
	if err != nil {
		return nil, fmt.Errorf("tournaments: read tournament: %w", err)
	}
	movement, err := e.q.InsertCashMovement(e.ctx, db.InsertCashMovementParams{
		ComplexID: cat.ComplexID, SessionID: session.ID, Kind: "income", Category: incomeCategory,
		Method: db.PaymentMethod(in.Method), Amount: cat.CourtFee,
		Note:      data.TextToPg(ptr("Cancha de torneo: " + t.Name + " · " + cat.Name)),
		CreatedBy: data.UUIDToPg(in.By),
	})
	if err != nil {
		return nil, fmt.Errorf("tournaments: court fee movement: %w", err)
	}
	row, err := e.q.UpsertMatchFee(e.ctx, db.UpsertMatchFeeParams{
		ComplexID: cat.ComplexID, MatchID: match.ID, Side: int32(in.Side), PairID: pair, //nolint:gosec // G115: 1 or 2
		Amount: cat.CourtFee, Method: db.PaymentMethod(in.Method), CashMovementID: movement.ID, ChargedBy: data.UUIDToPg(in.By),
	})
	if err != nil {
		return nil, fmt.Errorf("tournaments: record court fee: %w", err)
	}
	return matchFeeFromDB(row), nil
}

// requireNoLiveFee answers ErrFeeCharged when the (match, side) has a charge
// that stands; a voided one may be charged again.
func (e *engine) requireNoLiveFee(matchID pgtype.UUID, side int) error {
	fee, err := e.q.GetMatchFeeForUpdate(e.ctx, db.GetMatchFeeForUpdateParams{MatchID: matchID, Side: int32(side)}) //nolint:gosec // G115: 1 or 2
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("tournaments: read court fee: %w", err)
	case !fee.Voided:
		return ErrFeeCharged
	}
	return nil
}

// openSession locks the complex's open cash session, or answers ErrNoOpenCashSession.
func (e *engine) openSession(complexID pgtype.UUID) (db.CashSession, error) {
	session, err := e.q.GetOpenCashSessionForUpdate(e.ctx, complexID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.CashSession{}, ErrNoOpenCashSession
	}
	if err != nil {
		return db.CashSession{}, fmt.Errorf("tournaments: lock open cash session: %w", err)
	}
	return session, nil
}

// UndoCourtFee takes back a charge recorded by mistake: it writes the void of the
// charge's income movement into the open cash session, the way voiding a sale
// does, so the pair can be charged again. It answers ErrFeeNotCharged when no
// charge stands and ErrNoOpenCashSession when there is no session to hold the
// void. It works on a cancelled tournament too: the money still has to be
// returned.
func (m *Store) UndoCourtFee(ctx context.Context, matchID uuid.UUID, side int, by uuid.UUID) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	return m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		match, err := q.GetTournamentMatch(ctx, data.UUIDToPg(matchID))
		if err != nil {
			return matchReadError(err)
		}
		cat, err := q.LockTournamentCategory(ctx, match.CategoryID)
		if err != nil {
			return fmt.Errorf("tournaments: lock category: %w", err)
		}
		if err := data.AssertTenant(ctx, data.PgToUUID(cat.ComplexID)); err != nil {
			return err
		}
		e := &engine{ctx: ctx, q: q}
		fee, err := q.GetMatchFeeForUpdate(ctx, db.GetMatchFeeForUpdateParams{MatchID: match.ID, Side: int32(side)}) //nolint:gosec // G115: 1 or 2
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && fee.Voided) {
			return ErrFeeNotCharged
		}
		if err != nil {
			return fmt.Errorf("tournaments: read court fee: %w", err)
		}
		session, err := e.openSession(cat.ComplexID)
		if err != nil {
			return err
		}
		income, err := q.GetCashMovementByID(ctx, db.GetCashMovementByIDParams{ID: fee.CashMovementID, ComplexID: cat.ComplexID})
		if err != nil {
			return fmt.Errorf("tournaments: read the charge's income movement: %w", err)
		}
		if _, err := q.InsertCashMovement(ctx, db.InsertCashMovementParams{
			ComplexID: cat.ComplexID, SessionID: session.ID, Kind: "expense", Category: income.Category, Method: income.Method,
			Amount: income.Amount, VoidsMovementID: income.ID, CreatedBy: data.UUIDToPg(by),
		}); err != nil {
			return fmt.Errorf("tournaments: void the court fee movement: %w", err)
		}
		return nil
	})
}

func matchReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return data.ErrRecordNotFound
	}
	return fmt.Errorf("tournaments: read match: %w", err)
}

func ptr[T any](v T) *T { return &v }
