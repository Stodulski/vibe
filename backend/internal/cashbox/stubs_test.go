package cashbox

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// stubStore is a hand-written double for Store (SessionStore + MovementStore),
// following this codebase's existing fake/stub convention (see
// internal/courts/stubs_test.go): plain fields the test sets up front, a
// handful of "last call" captures, and no mocking framework.
type stubStore struct {
	// Sessions
	openSession     *cashboxstore.CashSession
	getOpenErr      error
	byID            *cashboxstore.CashSession
	getByIDErr      error
	listSessions    []*cashboxstore.CashSession
	listMetadata    data.Metadata
	listErr         error
	closedSession   *cashboxstore.CashSession
	closeErr        error
	openSessionErr  error
	insertedSession *cashboxstore.CashSession

	// Movements
	movements         []*cashboxstore.CashMovement
	movementByID      *cashboxstore.CashMovement
	getMovementErr    error
	sumTotals         []cashboxstore.MovementTotal
	sumErr            error
	insertMovementErr error
	insertedMovements []*cashboxstore.CashMovement

	// closeArgs captures the exact arguments the service passed to Close, so a
	// test can assert what it computed without re-deriving it.
	closeArgs *closeCall
}

type closeCall struct {
	complexID, sessionID, closedBy   uuid.UUID
	countedCash, cashBookingPayments int
	closingNote                      *string
}

func (s *stubStore) OpenSession(_ context.Context, session *cashboxstore.CashSession) error {
	if s.openSessionErr != nil {
		return s.openSessionErr
	}
	session.ID = uuid.New()
	session.OpenedAt = time.Now()
	s.insertedSession = session
	return nil
}

func (s *stubStore) GetOpenByComplex(_ context.Context, _ uuid.UUID) (*cashboxstore.CashSession, error) {
	if s.getOpenErr != nil {
		return nil, s.getOpenErr
	}
	return s.openSession, nil
}

func (s *stubStore) GetByID(_ context.Context, _, _ uuid.UUID) (*cashboxstore.CashSession, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	return s.byID, nil
}

func (s *stubStore) ListByComplex(_ context.Context, _ uuid.UUID, _ data.Filters) ([]*cashboxstore.CashSession, data.Metadata, error) {
	if s.listErr != nil {
		return nil, data.Metadata{}, s.listErr
	}
	return s.listSessions, s.listMetadata, nil
}

func (s *stubStore) Close(_ context.Context, complexID, sessionID, closedBy uuid.UUID, countedCash, cashBookingPaymentsInWindow int, closingNote *string) (*cashboxstore.CashSession, error) {
	s.closeArgs = &closeCall{
		complexID: complexID, sessionID: sessionID, closedBy: closedBy,
		countedCash: countedCash, cashBookingPayments: cashBookingPaymentsInWindow, closingNote: closingNote,
	}
	if s.closeErr != nil {
		return nil, s.closeErr
	}
	return s.closedSession, nil
}

func (s *stubStore) InsertMovement(_ context.Context, m *cashboxstore.CashMovement) error {
	if s.insertMovementErr != nil {
		return s.insertMovementErr
	}
	m.ID = uuid.New()
	m.CreatedAt = time.Now()
	s.insertedMovements = append(s.insertedMovements, m)
	return nil
}

func (s *stubStore) GetMovementByID(_ context.Context, _, _ uuid.UUID) (*cashboxstore.CashMovement, error) {
	if s.getMovementErr != nil {
		return nil, s.getMovementErr
	}
	return s.movementByID, nil
}

func (s *stubStore) ListMovementsBySession(_ context.Context, _, _ uuid.UUID) ([]*cashboxstore.CashMovement, error) {
	return s.movements, nil
}

func (s *stubStore) SumBySession(_ context.Context, _, _ uuid.UUID) ([]cashboxstore.MovementTotal, error) {
	if s.sumErr != nil {
		return nil, s.sumErr
	}
	return s.sumTotals, nil
}

// stubPayments is a double for PaymentWindowReader.
type stubPayments struct {
	summaries []reportstore.PaymentMethodSummary
	err       error
	// lastFrom/lastTo capture the window Close/buildSummary asked for.
	lastFrom, lastTo time.Time
}

func (p *stubPayments) PaymentSummaryByMethodWindow(_ context.Context, _ uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error) {
	p.lastFrom, p.lastTo = from, to
	if p.err != nil {
		return nil, p.err
	}
	return p.summaries, nil
}

// stubRecorder is a double for Recorder.
type stubRecorder struct{ entries []audit.Entry }

func (r *stubRecorder) Record(e audit.Entry) { r.entries = append(r.entries, e) }

func newTestService(store *stubStore, payments *stubPayments, recorder *stubRecorder) *Service {
	return NewService(store, payments, recorder)
}

// newTestHandler wires a real Handler over stub dependencies, following
// courts' own newTestHandler (internal/courts/stubs_test.go).
func newTestHandler(store *stubStore, payments *stubPayments) (*Handler, *stubRecorder) {
	rec := &stubRecorder{}
	responder := httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return NewHandler(NewService(store, payments, rec), responder, false), rec
}
