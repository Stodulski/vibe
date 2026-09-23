// Package store implements the cashbox domain's persistence: cash sessions
// (shifts) and their append-only cash movements, against PostgreSQL.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Domain sentinels this package raises, alongside the shared ones in
// internal/data (ErrRecordNotFound in particular: a session or movement
// looked up by id and not found, or not this tenant's, answers that one).
var (
	// ErrSessionAlreadyOpen reports that the complex already has an open
	// session — either idx_cash_sessions_one_open refused a race, or Open's
	// own pre-check found one.
	ErrSessionAlreadyOpen = errors.New("cash session: this complex already has an open session")
	// ErrSessionNotOpen reports that a write needed an open session and did
	// not find one — the session named is closed, or was closed by a
	// concurrent request while this one waited on the row lock.
	ErrSessionNotOpen = errors.New("cash session: no session is open, or it was closed just now")
	// ErrAlreadyVoided reports that the movement being voided already has a
	// void (cash_movements.voids_movement_id is UNIQUE).
	ErrAlreadyVoided = errors.New("cash movement: this movement has already been voided")
	// ErrVoidOfVoid reports that the movement being voided is itself a void —
	// cash_movements_check_void (003_cashbox.sql) is the actual enforcement;
	// this is its Go-side name.
	ErrVoidOfVoid = errors.New("cash movement: cannot void a movement that is itself a void")
	// ErrCannotVoidSaleManually reports an attempt to void a 'sale' category
	// income movement through the ordinary cash-movement void endpoint.
	// internal/sales.Service.Void is the only path allowed to void one — see
	// cashbox.Service.VoidMovement's own comment for why.
	ErrCannotVoidSaleManually = errors.New("cash movement: a sale's income can only be voided by voiding the sale")
)

// Constraint names from db/migrations/003_cashbox.sql, matched by name for
// the same reason courts/store/courts.go's translateCourtWrite is: several
// constraints on these two tables can raise the same SQLSTATE.
const (
	oneOpenSessionIndex    = "idx_cash_sessions_one_open"
	sessionClosedImmutable = "cash_sessions_closed_is_immutable"
	// voidsMovementUnique is Postgres' default name for the inline
	// `voids_movement_id UUID UNIQUE` column constraint: <table>_<column>_key.
	voidsMovementUnique  = "cash_movements_voids_movement_id_key"
	voidOfVoidConstraint = "cash_movements_no_void_of_void"
)

// translateSessionWrite maps the constraint refusals a cash_sessions write
// can produce onto this package's sentinels. Anything else is returned
// unchanged — an unrecognised constraint failure is a genuine fault.
func translateSessionWrite(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case oneOpenSessionIndex:
		return ErrSessionAlreadyOpen
	case sessionClosedImmutable:
		return ErrSessionNotOpen
	default:
		return err
	}
}

// translateMovementWrite is translateSessionWrite's counterpart for
// cash_movements.
func translateMovementWrite(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case voidsMovementUnique:
		return ErrAlreadyVoided
	case voidOfVoidConstraint:
		return ErrVoidOfVoid
	default:
		return err
	}
}

// CashSession is one cash shift: an owner's opening float, everything
// recorded against it, and — once closed — the count and the reconciliation
// snapshot.
//
// OpeningCash, CountedCash, ExpectedCash and Difference are int64, not int:
// they are BIGINT columns (db/migrations/003_cashbox.sql), unlike every
// per-entry money field in this codebase (a movement's own Amount below
// stays int/INTEGER), because they are Go-computed SUMs that must never wrap
// the way an INTEGER column silently did before that migration.
type CashSession struct {
	ID          uuid.UUID
	ComplexID   uuid.UUID
	OpenedAt    time.Time
	OpenedBy    uuid.UUID
	OpeningCash int64
	// ClosedAt, ClosedBy, CountedCash and ExpectedCash are all nil while the
	// session is open and all set once Close succeeds — cash_sessions'
	// close-state CHECK enforces the same all-or-nothing rule at the database.
	ClosedAt     *time.Time
	ClosedBy     *uuid.UUID
	CountedCash  *int64
	ExpectedCash *int64
	// Difference is counted minus expected. A generated column
	// (db/migrations/003_cashbox.sql): the database computes it, this field
	// only ever carries what came back on a read.
	Difference *int64
	// OpeningNote is written once, by Open. ClosingNote is written once, by
	// Close. Two columns, not one: the closer must never be able to erase
	// what the opener recorded.
	OpeningNote *string
	ClosingNote *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsOpen reports whether the session has not been closed yet.
func (s *CashSession) IsOpen() bool { return s.ClosedAt == nil }

// CursorKey implements data.CursorKeyer for the history list's pagination.
func (s *CashSession) CursorKey() (time.Time, uuid.UUID) { return s.OpenedAt, s.ID }

// CashMovement is one append-only entry against a session: ordinary income or
// expense, or a void of an earlier movement (VoidsMovementID set).
type CashMovement struct {
	ID              uuid.UUID
	ComplexID       uuid.UUID
	SessionID       uuid.UUID
	Kind            string
	Category        string
	Method          string
	Amount          int
	Note            *string
	VoidsMovementID *uuid.UUID
	CreatedAt       time.Time
	CreatedBy       uuid.UUID
}

// IsVoid reports whether this movement corrects an earlier one.
func (m *CashMovement) IsVoid() bool { return m.VoidsMovementID != nil }

// MovementTotal is one (method, kind, category) bucket's total within a
// session — the raw material for both the summary's full breakdown and (the
// method == "cash" rows only) the cash reconciliation arithmetic.
type MovementTotal struct {
	Method   string
	Kind     string
	Category string
	Total    int
	Count    int
}

// Store implements the cashbox domain's two ports against PostgreSQL.
type Store struct {
	DB *data.DB
	Q  *db.Queries
}

// OpenSession creates a new cash session and populates s with its generated
// id and timestamps.
//
// A friendly pre-check answers the common case — this complex's owner tried
// to open a second shift — without ever reaching the database's own
// constraint; idx_cash_sessions_one_open (003_cashbox.sql) is what actually
// closes the race between two concurrent opens, and its violation is
// translated to the same ErrSessionAlreadyOpen so the caller cannot tell
// which path answered.
func (m *Store) OpenSession(ctx context.Context, s *CashSession) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, s.ComplexID); err != nil {
		return err
	}

	if _, err := m.Q.GetOpenCashSessionByComplex(ctx, data.UUIDToPg(s.ComplexID)); err == nil {
		return ErrSessionAlreadyOpen
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("cash session: checking for an already-open session: %w", err)
	}

	row, err := m.Q.InsertCashSession(ctx, db.InsertCashSessionParams{
		ComplexID:   data.UUIDToPg(s.ComplexID),
		OpenedBy:    data.UUIDToPg(s.OpenedBy),
		OpeningCash: s.OpeningCash,
		OpeningNote: data.TextToPg(s.OpeningNote),
	})
	if err != nil {
		return translateSessionWrite(err)
	}

	*s = *cashSessionFromDB(row)
	return nil
}

// GetOpenByComplex returns the complex's open session, or ErrRecordNotFound
// if none is open. Read-only — GET /current never locks the row.
func (m *Store) GetOpenByComplex(ctx context.Context, complexID uuid.UUID) (*CashSession, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetOpenCashSessionByComplex(ctx, data.UUIDToPg(complexID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return cashSessionFromDB(row), nil
}

// GetByID returns one session of this complex, open or closed, or
// ErrRecordNotFound.
func (m *Store) GetByID(ctx context.Context, complexID, sessionID uuid.UUID) (*CashSession, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetCashSessionByID(ctx, db.GetCashSessionByIDParams{
		ID:        data.UUIDToPg(sessionID),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return cashSessionFromDB(row), nil
}

// ListByComplex returns a page of this complex's sessions, newest (most
// recently opened) first.
func (m *Store) ListByComplex(ctx context.Context, complexID uuid.UUID, filters data.Filters) ([]*CashSession, data.Metadata, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, data.Metadata{}, err
	}
	hasCursor := !cursorTime.IsZero()

	//nolint:gosec // G115: filters.Limit is validated by data.ValidateFilters (1-200) before this is ever called.
	fetchLimit := int32(filters.Limit + 1)

	rows, err := m.Q.ListCashSessionsByComplex(ctx, db.ListCashSessionsByComplexParams{
		ComplexID:      data.UUIDToPg(complexID),
		HasCursor:      hasCursor,
		CursorOpenedAt: data.TimeToPg(cursorTime),
		CursorID:       data.UUIDToPg(cursorID),
		PageLimit:      fetchLimit,
	})
	if err != nil {
		return nil, data.Metadata{}, fmt.Errorf("cash session: list by complex: %w", err)
	}

	sessions := make([]*CashSession, len(rows))
	for i, r := range rows {
		sessions[i] = cashSessionFromDB(r)
	}

	sessions, meta := data.TrimPage(sessions, filters.Limit, data.BuildTimestampCursor)
	return sessions, meta, nil
}

// Close closes an open session in one transaction: it locks the session row,
// refuses with ErrRecordNotFound if it does not belong to this complex,
// ErrSessionNotOpen if it is already closed (a concurrent close won the
// race), sums this session's own cash movements UNDER THAT SAME LOCK (so a
// movement racing the close is either fully counted or fully rejected by
// InsertMovement's own lock on this row — never half-landed), and writes the
// count and the expected-cash snapshot.
//
// cashBookingPaymentsInWindow and cashManualRefundsInWindow are the two parts
// of expected cash this store cannot compute itself — both booking payments
// and manual refunds live in another domain's table (payments) — so the
// service reads them (a plain, unlocked read: nothing else races a booking
// payment or a manual refund against this specific close) and passes them
// in. Everything this store CAN compute from its own tables (opening_cash,
// this session's cash movements), it does, inside the lock, rather than
// trusting a value the caller read earlier and might now be stale.
//
// closedAt is likewise the service's, not this store's: it is the exact
// instant Service.Close used as the end of the booking-payments window it
// read cashBookingPaymentsInWindow over, and it must be the exact value
// written to the closed_at column — not a fresh NOW() taken here — or a
// booking payment landing between that read and this write would be missing
// from the stored expected_cash snapshot while still falling inside
// [opened_at, closed_at) the next time a summary is rebuilt from it.
func (m *Store) Close(ctx context.Context, complexID, sessionID, closedBy uuid.UUID, countedCash, cashBookingPaymentsInWindow, cashManualRefundsInWindow int64, closedAt time.Time, closingNote *string) (*CashSession, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, complexID); err != nil {
		return nil, err
	}

	var closed *CashSession
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		locked, err := qtx.GetCashSessionByIDForUpdate(ctx, db.GetCashSessionByIDForUpdateParams{
			ID:        data.UUIDToPg(sessionID),
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("cash session: lock for close: %w", err)
		}
		if locked.ClosedAt.Valid {
			return ErrSessionNotOpen
		}

		totals, err := sumMovements(ctx, qtx, sessionID, complexID)
		if err != nil {
			return err
		}
		cashIncome, cashExpense := CashIncomeAndExpense(totals)
		expectedCash := locked.OpeningCash + int64(cashIncome) - int64(cashExpense) +
			cashBookingPaymentsInWindow - cashManualRefundsInWindow

		row, err := qtx.CloseCashSession(ctx, db.CloseCashSessionParams{
			ClosedAt:     data.TimeToPg(closedAt),
			ClosedBy:     data.UUIDToPg(closedBy),
			CountedCash:  data.Int8ToPg(countedCash),
			ExpectedCash: data.Int8ToPg(expectedCash),
			ClosingNote:  data.TextToPg(closingNote),
			ID:           data.UUIDToPg(sessionID),
			ComplexID:    data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return translateSessionWrite(err)
		}
		closed = cashSessionFromDB(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return closed, nil
}

// InsertMovement records a movement — ordinary or a void — against an open
// session, in one transaction: it locks the session row first and refuses
// with ErrSessionNotOpen if it is not open, so a movement cannot land in a
// session that a concurrent request is closing.
//
// For a void, m.VoidsMovementID is already set and every other field already
// copied from the original by the service; cash_movements_check_void
// (003_cashbox.sql) is what actually verifies the copy is faithful and that
// the original is voidable, translated by translateMovementWrite into
// ErrAlreadyVoided / ErrVoidOfVoid.
func (m *Store) InsertMovement(ctx context.Context, movement *CashMovement) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, movement.ComplexID); err != nil {
		return err
	}

	return m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		locked, err := qtx.GetCashSessionByIDForUpdate(ctx, db.GetCashSessionByIDForUpdateParams{
			ID:        data.UUIDToPg(movement.SessionID),
			ComplexID: data.UUIDToPg(movement.ComplexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("cash movement: lock session: %w", err)
		}
		if locked.ClosedAt.Valid {
			return ErrSessionNotOpen
		}

		row, err := qtx.InsertCashMovement(ctx, db.InsertCashMovementParams{
			ComplexID: data.UUIDToPg(movement.ComplexID),
			SessionID: data.UUIDToPg(movement.SessionID),
			Kind:      movement.Kind,
			Category:  movement.Category,
			Method:    db.PaymentMethod(movement.Method),
			//nolint:gosec // G115: amount is a validated currency amount (centavos) checked positive before this, far below int32 range.
			Amount:          int32(movement.Amount),
			Note:            data.TextToPg(movement.Note),
			VoidsMovementID: data.UUIDPtrToPg(movement.VoidsMovementID),
			CreatedBy:       data.UUIDToPg(movement.CreatedBy),
		})
		if err != nil {
			return translateMovementWrite(err)
		}

		*movement = *cashMovementFromDB(row)
		return nil
	})
}

// GetMovementByID returns one movement of this complex, or
// ErrRecordNotFound. Used to load the original a void points at.
func (m *Store) GetMovementByID(ctx context.Context, complexID, movementID uuid.UUID) (*CashMovement, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetCashMovementByID(ctx, db.GetCashMovementByIDParams{
		ID:        data.UUIDToPg(movementID),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return cashMovementFromDB(row), nil
}

// ListMovementsBySession returns every movement of a session, oldest first.
// Not paginated — see db/queries/cashbox.sql's ListCashMovementsBySession.
func (m *Store) ListMovementsBySession(ctx context.Context, complexID, sessionID uuid.UUID) ([]*CashMovement, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListCashMovementsBySession(ctx, db.ListCashMovementsBySessionParams{
		SessionID: data.UUIDToPg(sessionID),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		return nil, fmt.Errorf("cash movement: list by session: %w", err)
	}

	movements := make([]*CashMovement, len(rows))
	for i, r := range rows {
		movements[i] = cashMovementFromDB(r)
	}
	return movements, nil
}

// SumBySession returns one row per (method, kind, category) combination
// actually used in the session — see db/queries/cashbox.sql's
// SumCashMovementsBySession for what the service derives from each half of
// this. Used by the read paths (current, detail); Close computes its own copy
// under the session lock instead of calling this — see sumMovements.
func (m *Store) SumBySession(ctx context.Context, complexID, sessionID uuid.UUID) ([]MovementTotal, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	return sumMovements(ctx, m.Q, sessionID, complexID)
}

// sumMovements is SumBySession's query, factored out so Close can run the
// exact same aggregate through the transaction's own *db.Queries (qtx) —
// under the session row lock — instead of a fresh pool query that could race
// a concurrent movement insert.
func sumMovements(ctx context.Context, q *db.Queries, sessionID, complexID uuid.UUID) ([]MovementTotal, error) {
	rows, err := q.SumCashMovementsBySession(ctx, db.SumCashMovementsBySessionParams{
		SessionID: data.UUIDToPg(sessionID),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		return nil, fmt.Errorf("cash movement: sum by session: %w", err)
	}

	totals := make([]MovementTotal, len(rows))
	for i, r := range rows {
		totals[i] = MovementTotal{
			Method:   r.Method,
			Kind:     r.Kind,
			Category: r.Category,
			Total:    int(r.Total),
			Count:    int(r.Count),
		}
	}
	return totals, nil
}

// CashIncomeAndExpense sums the cash-method rows of a movement-totals result
// by kind — the two components of expected cash this store owns outright
// (see Close and the feature document's expected-cash formula). Every other
// method's rows are display-only for the summary and play no part here,
// because "cash reconciliation counts only cash" (decision, pos-cashbox).
func CashIncomeAndExpense(totals []MovementTotal) (income, expense int) {
	for _, t := range totals {
		if t.Method != "cash" {
			continue
		}
		switch t.Kind {
		case "income":
			income += t.Total
		case "expense":
			expense += t.Total
		}
	}
	return income, expense
}

func cashSessionFromDB(s db.CashSession) *CashSession {
	return &CashSession{
		ID:           data.PgToUUID(s.ID),
		ComplexID:    data.PgToUUID(s.ComplexID),
		OpenedAt:     data.PgToTime(s.OpenedAt),
		OpenedBy:     data.PgToUUID(s.OpenedBy),
		OpeningCash:  s.OpeningCash,
		ClosedAt:     data.PgToTimePtr(s.ClosedAt),
		ClosedBy:     data.PgToUUIDPtr(s.ClosedBy),
		CountedCash:  data.PgToInt8Ptr(s.CountedCash),
		ExpectedCash: data.PgToInt8Ptr(s.ExpectedCash),
		Difference:   data.PgToInt8Ptr(s.Difference),
		OpeningNote:  data.PgToTextPtr(s.OpeningNote),
		ClosingNote:  data.PgToTextPtr(s.ClosingNote),
		CreatedAt:    data.PgToTime(s.CreatedAt),
		UpdatedAt:    data.PgToTime(s.UpdatedAt),
	}
}

func cashMovementFromDB(m db.CashMovement) *CashMovement {
	return &CashMovement{
		ID:              data.PgToUUID(m.ID),
		ComplexID:       data.PgToUUID(m.ComplexID),
		SessionID:       data.PgToUUID(m.SessionID),
		Kind:            m.Kind,
		Category:        m.Category,
		Method:          string(m.Method),
		Amount:          int(m.Amount),
		Note:            data.PgToTextPtr(m.Note),
		VoidsMovementID: data.PgToUUIDPtr(m.VoidsMovementID),
		CreatedAt:       data.PgToTime(m.CreatedAt),
		CreatedBy:       data.PgToUUID(m.CreatedBy),
	}
}
