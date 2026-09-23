package cashbox

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// IncomeCategories and ExpenseCategories are the category values valid for a
// non-void movement of each kind — mirrors
// cash_movements_category_kind_consistent (db/migrations/003_cashbox.sql). A
// void reuses the ORIGINAL's category under the opposite kind (see
// VoidMovement), so these lists apply only to an ordinary write; the handler
// never asks a void request for a category.
var (
	IncomeCategories  = []string{"other_income"}
	ExpenseCategories = []string{"supplies", "salaries", "services", "maintenance", "cleaning", "withdrawal", "other_expense"}
)

// Service holds this module's rules: what a session may be, what a movement
// may be, and how expected cash is computed. Every store call and every
// audit entry of the cashbox domain goes through it.
type Service struct {
	sessions  SessionStore
	movements MovementStore
	payments  PaymentWindowReader
	audit     Recorder
}

// NewService returns a Service backed by the given store and dependencies.
func NewService(store Store, payments PaymentWindowReader, recorder Recorder) *Service {
	return &Service{
		sessions:  store,
		movements: store,
		payments:  payments,
		audit:     recorder,
	}
}

// record writes an audit entry for a cashbox change.
func (s *Service) record(complexID uuid.UUID, actor Actor, action, entityType string, entityID *uuid.UUID, oldVal, newVal any) {
	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   oldVal,
		NewValue:   newVal,
		IPAddress:  actor.IP,
	})
}

// OpenInput is a validated request to start a shift.
type OpenInput struct {
	OpeningCash int64
	OpeningNote *string
}

// Open starts a new cash session for the complex and records it.
// cashboxstore.ErrSessionAlreadyOpen answers a second concurrent open.
func (s *Service) Open(ctx context.Context, complexID uuid.UUID, actor Actor, openedBy uuid.UUID, in OpenInput) (*cashboxstore.CashSession, error) {
	session := &cashboxstore.CashSession{
		ComplexID:   complexID,
		OpenedBy:    openedBy,
		OpeningCash: in.OpeningCash,
		OpeningNote: in.OpeningNote,
	}
	if err := s.sessions.OpenSession(ctx, session); err != nil {
		return nil, err
	}

	s.record(complexID, actor, "open", "cash_session", &session.ID, nil, session)
	return session, nil
}

// Summary is a session's reconciliation view: opening, expected cash, the
// movement breakdown, the booking-payment breakdown, and the manual-refund
// breakdown, all over the session's own window. CountedCash and Difference
// are nil until the session closes.
//
// OpeningCash, ExpectedCash, CashManualRefunds, CountedCash and Difference
// are int64, matching cashboxstore.CashSession's own fields — see that
// type's comment for why (BIGINT session-level aggregates,
// db/migrations/003_cashbox.sql).
type Summary struct {
	OpeningCash  int64  `json:"opening_cash"`
	ExpectedCash int64  `json:"expected_cash"`
	CountedCash  *int64 `json:"counted_cash,omitempty"`
	Difference   *int64 `json:"difference,omitempty"`
	// CashManualRefunds is, for an OPEN session, exactly what buildSummary
	// subtracted from the live ExpectedCash projection below it — informational,
	// so the Caja screen can show the "Devoluciones en efectivo" line without
	// recomputing it from ManualRefunds itself.
	//
	// For a CLOSED session it is NOT that guarantee: ExpectedCash for a closed
	// session is the immutable snapshot Close wrote (see buildSummary and
	// Close's own comment), but CashManualRefunds/ManualRefunds are always
	// recomputed fresh, over [opened_at, closed_at), every time the summary is
	// rebuilt. manual_refunded_at is written as the manual refund's own
	// transaction's NOW() — its start time, not its commit time
	// (db/queries/payments.sql, MarkPaymentManuallyRefunded) — so a manual
	// refund whose transaction started before closed_at but only committed
	// after Close already read the window can be absent from the stored
	// ExpectedCash snapshot yet still show up here, in a later rebuild, because
	// its manual_refunded_at still falls inside the window. This mirrors the
	// closed_at/app-clock note on Service.Close: the two numbers are not
	// guaranteed to agree once a session is closed, and this field never tries
	// to make them.
	CashManualRefunds int64                                   `json:"cash_manual_refunds"`
	MovementTotals    []cashboxstore.MovementTotal            `json:"movement_totals"`
	BookingPayments   []reportstore.PaymentMethodSummary      `json:"booking_payments"`
	ManualRefunds     []reportstore.ManualRefundMethodSummary `json:"manual_refunds"`
}

// SessionWithSummary is what GET /current answers with.
type SessionWithSummary struct {
	*cashboxstore.CashSession
	Summary Summary
}

// SessionDetail is what GET /{sessionId} answers with: the session, its
// summary, and its full movement ledger.
type SessionDetail struct {
	*cashboxstore.CashSession
	Summary   Summary
	Movements []*cashboxstore.CashMovement
}

// buildSummary computes a session's reconciliation view. For an open session,
// expected cash is a LIVE projection through now(); for a closed one, it is
// exactly the snapshot Close wrote (never recomputed from current data, which
// could have moved on — see Close's own comment on why cash_sessions are
// immutable once closed).
func (s *Service) buildSummary(ctx context.Context, session *cashboxstore.CashSession) (Summary, error) {
	windowEnd := time.Now()
	if session.ClosedAt != nil {
		windowEnd = *session.ClosedAt
	}

	totals, err := s.movements.SumBySession(ctx, session.ComplexID, session.ID)
	if err != nil {
		return Summary{}, err
	}

	bookingPayments, err := s.payments.PaymentSummaryByMethodWindow(ctx, session.ComplexID, session.OpenedAt, windowEnd)
	if err != nil {
		return Summary{}, err
	}

	manualRefunds, err := s.payments.ManualRefundSummaryByMethodWindow(ctx, session.ComplexID, session.OpenedAt, windowEnd)
	if err != nil {
		return Summary{}, err
	}
	cashManualRefunds := int64(sumCashManualRefunds(manualRefunds))

	summary := Summary{
		OpeningCash:       session.OpeningCash,
		MovementTotals:    totals,
		BookingPayments:   bookingPayments,
		ManualRefunds:     manualRefunds,
		CashManualRefunds: cashManualRefunds,
		CountedCash:       session.CountedCash,
		Difference:        session.Difference,
	}

	if session.ClosedAt != nil && session.ExpectedCash != nil {
		summary.ExpectedCash = *session.ExpectedCash
	} else {
		cashIncome, cashExpense := cashboxstore.CashIncomeAndExpense(totals)
		summary.ExpectedCash = session.OpeningCash + int64(cashIncome) - int64(cashExpense) +
			int64(sumCashBookingPayments(bookingPayments)) - cashManualRefunds
	}

	return summary, nil
}

// sumCashBookingPayments totals the cash-method row(s) of a
// PaymentSummaryByMethodWindow result — amount plus service fee, the same
// "what the client actually paid" total payments.go's own refund bound uses.
// Every other method's row is informational only for the summary; only cash
// feeds expected cash ("cash reconciliation counts only cash", decision,
// pos-cashbox).
func sumCashBookingPayments(payments []reportstore.PaymentMethodSummary) int {
	total := 0
	for _, p := range payments {
		if p.Method == "cash" {
			total += p.Amount + p.ServiceFee
		}
	}
	return total
}

// sumCashManualRefunds totals the cash-method row(s) of a
// ManualRefundSummaryByMethodWindow result — the amount actually handed back,
// which subtracts from expected cash the same way sumCashBookingPayments'
// total adds to it. Every other method's row is informational only.
func sumCashManualRefunds(refunds []reportstore.ManualRefundMethodSummary) int {
	total := 0
	for _, r := range refunds {
		if r.Method == "cash" {
			total += r.Amount
		}
	}
	return total
}

// Current returns the complex's open session with its live summary, or
// data.ErrRecordNotFound if none is open.
func (s *Service) Current(ctx context.Context, complexID uuid.UUID) (*SessionWithSummary, error) {
	session, err := s.sessions.GetOpenByComplex(ctx, complexID)
	if err != nil {
		return nil, err
	}
	summary, err := s.buildSummary(ctx, session)
	if err != nil {
		return nil, err
	}
	return &SessionWithSummary{CashSession: session, Summary: summary}, nil
}

// List returns a page of the complex's session history, newest first.
func (s *Service) List(ctx context.Context, complexID uuid.UUID, filters data.Filters) ([]*cashboxstore.CashSession, data.Metadata, error) {
	return s.sessions.ListByComplex(ctx, complexID, filters)
}

// Get returns one session of this complex with its summary and full movement
// ledger, open or closed.
func (s *Service) Get(ctx context.Context, complexID, sessionID uuid.UUID) (*SessionDetail, error) {
	session, err := s.sessions.GetByID(ctx, complexID, sessionID)
	if err != nil {
		return nil, err
	}
	summary, err := s.buildSummary(ctx, session)
	if err != nil {
		return nil, err
	}
	movements, err := s.movements.ListMovementsBySession(ctx, complexID, sessionID)
	if err != nil {
		return nil, err
	}
	return &SessionDetail{CashSession: session, Summary: summary, Movements: movements}, nil
}

// CloseInput is a validated request to close a session.
type CloseInput struct {
	CountedCash int64
	// ClosingNote is its own column (db/migrations/003_cashbox.sql): it never
	// touches OpeningNote, so the closer can never erase what the opener
	// recorded.
	ClosingNote *string
}

// Close closes a session: picks ONE instant (now) to be both the end of the
// booking-payments window it reads (a plain, unlocked read — nothing races a
// booking payment against this specific close, see cashboxstore.Store.Close's
// own comment) and the closed_at this session will carry, then hands the
// count and that figure to the store, which locks the session row, computes
// the cash-movements side of expected cash under that same lock, and writes
// closed_at as EXACTLY the instant given here rather than its own NOW().
//
// Those two have to be the same instant: a session's summary is always
// rebuilt over [opened_at, closed_at) (buildSummary above), so a booking
// payment landing between this read and a later, different closed_at would
// be missing from the snapshot Close wrote here yet appear the next time
// anyone opens this session's detail — the count would look wrong forever
// for a reason nobody could see.
//
// cashboxstore.ErrSessionNotOpen answers a session that is already closed —
// including the race where a concurrent close won between this method's own
// read and the store's lock.
func (s *Service) Close(ctx context.Context, complexID, sessionID uuid.UUID, actor Actor, closedBy uuid.UUID, in CloseInput) (*cashboxstore.CashSession, error) {
	session, err := s.sessions.GetByID(ctx, complexID, sessionID)
	if err != nil {
		return nil, err
	}
	if !session.IsOpen() {
		return nil, cashboxstore.ErrSessionNotOpen
	}

	now := time.Now()
	bookingPayments, err := s.payments.PaymentSummaryByMethodWindow(ctx, complexID, session.OpenedAt, now)
	if err != nil {
		return nil, err
	}
	cashBookingPayments := int64(sumCashBookingPayments(bookingPayments))

	manualRefunds, err := s.payments.ManualRefundSummaryByMethodWindow(ctx, complexID, session.OpenedAt, now)
	if err != nil {
		return nil, err
	}
	cashManualRefunds := int64(sumCashManualRefunds(manualRefunds))

	closed, err := s.sessions.Close(ctx, complexID, sessionID, closedBy, in.CountedCash, cashBookingPayments, cashManualRefunds, now, in.ClosingNote)
	if err != nil {
		return nil, err
	}

	s.record(complexID, actor, "close", "cash_session", &closed.ID, session, closed)
	return closed, nil
}

// MovementInput is a validated request to record a movement.
type MovementInput struct {
	Kind     string
	Category string
	Method   string
	Amount   int
	Note     *string
}

// CreateMovement records an income or expense against sessionID, which must
// be open — cashboxstore.ErrSessionNotOpen answers one that is not.
func (s *Service) CreateMovement(ctx context.Context, complexID, sessionID uuid.UUID, actor Actor, createdBy uuid.UUID, in MovementInput) (*cashboxstore.CashMovement, error) {
	movement := &cashboxstore.CashMovement{
		ComplexID: complexID,
		SessionID: sessionID,
		Kind:      in.Kind,
		Category:  in.Category,
		Method:    in.Method,
		Amount:    in.Amount,
		Note:      in.Note,
		CreatedBy: createdBy,
	}
	if err := s.movements.InsertMovement(ctx, movement); err != nil {
		return nil, err
	}

	s.record(complexID, actor, "movement", "cash_movement", &movement.ID, nil, movement)
	return movement, nil
}

// VoidMovement corrects an earlier movement with a new one of the opposite
// kind, same amount/method/category — always in the CURRENTLY open session,
// never in sessionID from the request path.
//
// This is the "cleanest REST shape" call the feature document asks for: the
// path's {sessionId}/{movementId} pair addresses which historical movement is
// being corrected (and refuses with data.ErrRecordNotFound if movementID does
// not actually belong to sessionID — the ordinary nested-resource check), but
// the correction itself is a live cash-drawer event and can only ever land
// where the drawer actually is: today's open session, which may already be a
// different one than the movement's own. Voiding a movement from a session
// closed yesterday is exactly the "correcting a past mistake" case the
// decisions section describes.
func (s *Service) VoidMovement(ctx context.Context, complexID, sessionID, movementID uuid.UUID, actor Actor, createdBy uuid.UUID, note *string) (*cashboxstore.CashMovement, error) {
	original, err := s.movements.GetMovementByID(ctx, complexID, movementID)
	if err != nil {
		return nil, err
	}
	if original.SessionID != sessionID {
		return nil, data.ErrRecordNotFound
	}
	// A 'sale' category income is system-written (internal/sales.Store.Create,
	// the same way 'restock' is products.Store.Restock's own), and its void is
	// system-written too: internal/sales.Store.Void restores the sale's stock
	// AND voids its income in the same transaction, something this endpoint
	// has no way to do. Refusing it here — rather than letting it through and
	// leaving stock silently unrestored — means a sale's income can only ever
	// be voided by voiding the sale, so the two can never drift apart.
	if original.Category == "sale" {
		return nil, cashboxstore.ErrCannotVoidSaleManually
	}

	current, err := s.sessions.GetOpenByComplex(ctx, complexID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, cashboxstore.ErrSessionNotOpen
		}
		return nil, err
	}

	opposite := "expense"
	if original.Kind == "expense" {
		opposite = "income"
	}

	void := &cashboxstore.CashMovement{
		ComplexID:       complexID,
		SessionID:       current.ID,
		Kind:            opposite,
		Category:        original.Category,
		Method:          original.Method,
		Amount:          original.Amount,
		Note:            note,
		VoidsMovementID: &original.ID,
		CreatedBy:       createdBy,
	}
	if err := s.movements.InsertMovement(ctx, void); err != nil {
		return nil, err
	}

	s.record(complexID, actor, "void", "cash_movement", &void.ID, original, void)
	return void, nil
}
