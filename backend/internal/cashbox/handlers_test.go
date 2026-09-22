package cashbox

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// --- Open ------------------------------------------------------------------

// TestOpenWithoutOpeningCashIs422 pins the required-money-field fix: a body
// that omits opening_cash must not decode to a valid-looking 0 (0 is itself a
// legitimate opening float) — the OpenAPI request validator that would catch
// a missing required field never runs in production, so the handler has to
// refuse it itself.
func TestOpenWithoutOpeningCashIs422(t *testing.T) {
	store := &stubStore{}
	h, rec := newTestHandler(store, &stubPayments{})

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Open(w, ownerRequest(t, http.MethodPost, "/", complexID, nil, `{}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if msg, ok := fieldError(decode(t, w), "opening_cash"); !ok || msg != "must be provided" {
		t.Errorf("opening_cash error = %q (present %v), want %q", msg, ok, "must be provided")
	}
	if store.insertedSession != nil {
		t.Error("a rejected open must not be persisted")
	}
	if len(rec.entries) != 0 {
		t.Error("a rejected open must not be audited")
	}
}

// --- Close -------------------------------------------------------------

// TestCloseWithoutCountedCashIs422 is TestOpenWithoutOpeningCashIs422's
// counterpart for the close body.
func TestCloseWithoutCountedCashIs422(t *testing.T) {
	sessionID := uuid.New()
	store := &stubStore{byID: &cashboxstore.CashSession{ID: sessionID, ComplexID: uuid.New(), OpenedAt: time.Now()}}
	h, _ := newTestHandler(store, &stubPayments{})

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Close(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"sessionID": sessionID.String()}, `{}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if msg, ok := fieldError(decode(t, w), "counted_cash"); !ok || msg != "must be provided" {
		t.Errorf("counted_cash error = %q (present %v), want %q", msg, ok, "must be provided")
	}
	if store.closeArgs != nil {
		t.Error("a rejected close must not reach the store")
	}
}

// TestCloseOnAnAlreadyClosedSessionIs409 pins the domain-error mapping for
// cashboxstore.ErrSessionNotOpen on the close route.
func TestCloseOnAnAlreadyClosedSessionIs409(t *testing.T) {
	sessionID := uuid.New()
	closedAt := time.Now()
	store := &stubStore{byID: &cashboxstore.CashSession{ID: sessionID, ComplexID: uuid.New(), ClosedAt: &closedAt}}
	h, _ := newTestHandler(store, &stubPayments{})

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Close(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"sessionID": sessionID.String()}, `{"counted_cash":1000}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- List --------------------------------------------------------------

// TestListWithAMalformedCursorIs400 pins the handler's translation of
// data.ErrInvalidCursor to 400. The cursor is actually decoded inside the
// real store's ListByComplex (data.Filters.ParseCursor), which a stub never
// reaches — so this gives the stub the exact error the real store returns for
// a malformed cursor and proves the handler answers it the same way
// TestListWithAMalformedCursorIs400 exercises List reaching the service at
// all: List's own query-string parsing accepts any string, and it is
// ListByComplex that would refuse it in production.
func TestListWithAMalformedCursorIs400(t *testing.T) {
	store := &stubStore{listErr: data.ErrInvalidCursor}
	h, _ := newTestHandler(store, &stubPayments{})

	complexID := uuid.New()
	w := httptest.NewRecorder()
	req := ownerRequest(t, http.MethodGet, "/?cursor=not-a-valid-cursor!!", complexID, nil, "")
	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- Get / VoidMovement path parameters ---------------------------------

// TestGetWithAMalformedSessionIDIs404 pins ReadUUIDParam's failure mode: an
// unparseable path segment reads as "not found", not a 400 or 422 — the same
// convention every other handler in this codebase follows.
func TestGetWithAMalformedSessionIDIs404(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store, &stubPayments{})

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Get(w, ownerRequest(t, http.MethodGet, "/", complexID,
		map[string]string{"sessionID": "not-a-uuid"}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestVoidMovementWithAMalformedMovementIDIs404 is
// TestGetWithAMalformedSessionIDIs404's counterpart for the second path
// parameter the void route binds.
func TestVoidMovementWithAMalformedMovementIDIs404(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store, &stubPayments{})

	complexID, sessionID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.VoidMovement(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"sessionID": sessionID.String(), "movementID": "not-a-uuid"}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- CreateMovement ------------------------------------------------------

func TestCreateMovementRejectsInvalidInput(t *testing.T) {
	tooLongNote := strings.Repeat("a", noteMaxLen+1)

	tests := []struct {
		name string
		body string
	}{
		{"category does not match kind", `{"kind":"income","category":"supplies","method":"cash","amount":1000}`},
		{"mercadopago is not a counter method", `{"kind":"income","category":"other_income","method":"mercadopago","amount":1000}`},
		{"amount is zero", `{"kind":"income","category":"other_income","method":"cash","amount":0}`},
		{"amount is negative", `{"kind":"income","category":"other_income","method":"cash","amount":-500}`},
		{"amount exceeds the cap", `{"kind":"income","category":"other_income","method":"cash","amount":2000000001}`},
		{"note too long", `{"kind":"income","category":"other_income","method":"cash","amount":1000,"note":"` + tooLongNote + `"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{}
			h, rec := newTestHandler(store, &stubPayments{})

			complexID, sessionID := uuid.New(), uuid.New()
			w := httptest.NewRecorder()
			h.CreateMovement(w, ownerRequest(t, http.MethodPost, "/", complexID,
				map[string]string{"sessionID": sessionID.String()}, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if len(store.insertedMovements) != 0 {
				t.Error("an invalid movement must not be persisted")
			}
			if len(rec.entries) != 0 {
				t.Error("a rejected movement must not be audited")
			}
		})
	}
}

// --- VoidMovement --------------------------------------------------------

// TestVoidMovementWithAnEmptyBodyIs201 pins the optional-body fix: the void
// requestBody is optional in the document (internal/openapi/openapi.yaml's
// cashMovementsVoid carries no `required: true`), so a genuinely empty body —
// no bytes, no Content-Type — must be accepted as "no note", not answered
// with the generic invalid-JSON 400 ReadJSON gives an empty body it is asked
// to decode.
func TestVoidMovementWithAnEmptyBodyIs201(t *testing.T) {
	original := &cashboxstore.CashMovement{
		ID: uuid.New(), ComplexID: uuid.New(), SessionID: uuid.New(),
		Kind: "expense", Category: "supplies", Method: "cash", Amount: 1500,
	}
	openSession := &cashboxstore.CashSession{ID: original.SessionID}
	store := &stubStore{movementByID: original, openSession: openSession}
	h, rec := newTestHandler(store, &stubPayments{})

	w := httptest.NewRecorder()
	h.VoidMovement(w, ownerRequest(t, http.MethodPost, "/", original.ComplexID,
		map[string]string{"sessionID": original.SessionID.String(), "movementID": original.ID.String()}, ""))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201 for an empty (but valid, per the document) body; got %d (%s)", w.Code, w.Body.String())
	}
	if len(store.insertedMovements) != 1 {
		t.Fatalf("want exactly one void movement inserted; got %d", len(store.insertedMovements))
	}
	if store.insertedMovements[0].Note != nil {
		t.Errorf("want no note on a void with an empty body; got %+v", store.insertedMovements[0].Note)
	}
	if len(rec.entries) != 1 {
		t.Error("an accepted void must be audited")
	}
}

// TestVoidMovementASecondTimeIs409 pins the domain-error mapping for
// cashboxstore.ErrAlreadyVoided on the void route.
func TestVoidMovementASecondTimeIs409(t *testing.T) {
	original := &cashboxstore.CashMovement{
		ID: uuid.New(), ComplexID: uuid.New(), SessionID: uuid.New(),
		Kind: "expense", Category: "supplies", Method: "cash", Amount: 1500,
	}
	openSession := &cashboxstore.CashSession{ID: original.SessionID}
	store := &stubStore{
		movementByID:      original,
		openSession:       openSession,
		insertMovementErr: cashboxstore.ErrAlreadyVoided,
	}
	h, _ := newTestHandler(store, &stubPayments{})

	w := httptest.NewRecorder()
	h.VoidMovement(w, ownerRequest(t, http.MethodPost, "/", original.ComplexID,
		map[string]string{"sessionID": original.SessionID.String(), "movementID": original.ID.String()}, ""))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}
