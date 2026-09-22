package cashbox

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
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
	countedCash, cashBookingPayments int64
	closedAt                         time.Time
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

func (s *stubStore) Close(_ context.Context, complexID, sessionID, closedBy uuid.UUID, countedCash, cashBookingPaymentsInWindow int64, closedAt time.Time, closingNote *string) (*cashboxstore.CashSession, error) {
	s.closeArgs = &closeCall{
		complexID: complexID, sessionID: sessionID, closedBy: closedBy,
		countedCash: countedCash, cashBookingPayments: cashBookingPaymentsInWindow, closedAt: closedAt, closingNote: closingNote,
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

// ownerRequest builds a request from the complex's authenticated owner, with
// the given path parameters bound. Same shape as courts' own ownerRequest
// (internal/courts/stubs_test.go): a body of "" produces a request with no
// body and no Content-Type header at all, which is what an actually-empty
// request looks like (as opposed to a body of "{}"), for the void-with-no-
// body case.
func ownerRequest(t *testing.T, method, target string, complexID uuid.UUID, params map[string]string, body string) *http.Request {
	t.Helper()

	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}

	r = httpx.ContextSetUser(r, &authstore.User{ID: uuid.New(), Role: "owner"})
	r = httpx.ContextSetComplex(r, &complexstore.Complex{ID: complexID})

	for k, v := range params {
		r.SetPathValue(k, v)
	}
	return r
}

// decode parses a recorded JSON response body, following courts' own decode.
func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return body
}

// fieldError looks up one field's validation message from a decoded Problem
// body's "errors" array (internal/httpx/problem.go's FieldError), following
// internal/auth's own fieldError.
func fieldError(body map[string]any, field string) (string, bool) {
	errs, _ := body["errors"].([]any)
	for _, e := range errs {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if entry["field"] == field {
			msg, _ := entry["message"].(string)
			return msg, true
		}
	}
	return "", false
}
