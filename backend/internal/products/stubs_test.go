package products

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
)

// stubStore is a hand-written double for Store (Catalog + StockLedger),
// following this codebase's existing fake/stub convention (see
// internal/cashbox/stubs_test.go): plain fields the test sets up front, a
// handful of "last call" captures, no mocking framework.
type stubStore struct {
	listCalled   bool
	listActive   *bool
	insertErr    error
	insertedProd *productstore.Product

	byID       *productstore.Product
	getByIDErr error

	listProducts []*productstore.Product
	listErr      error

	updateErr error
	// updateArgs captures the exact product the service asked to persist, so
	// a test can assert what changed without re-deriving it.
	updateArgs *productstore.Product

	restockProduct   *productstore.Product
	restockMovement  *productstore.StockMovement
	restockErr       error
	restockArgs      *restockCall
	restockCallCount int

	adjustProduct   *productstore.Product
	adjustMovement  *productstore.StockMovement
	adjustErr       error
	adjustArgs      *adjustCall
	adjustCallCount int

	listMovements    []*productstore.StockMovement
	listMovementsMD  data.Metadata
	listMovementsErr error
}

type restockCall struct {
	complexID, productID, actorID uuid.UUID
	quantity, totalCost           int
	method                        string
	note                          *string
}

type adjustCall struct {
	complexID, productID, actorID uuid.UUID
	quantity                      int
	reason                        string
	note                          *string
}

func (s *stubStore) Insert(_ context.Context, p *productstore.Product) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	p.ID = uuid.New()
	s.insertedProd = p
	return nil
}

func (s *stubStore) GetByID(_ context.Context, _, _ uuid.UUID) (*productstore.Product, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	return s.byID, nil
}

func (s *stubStore) ListByComplex(_ context.Context, _ uuid.UUID, active *bool) ([]*productstore.Product, error) {
	s.listCalled = true
	s.listActive = active
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listProducts, nil
}

func (s *stubStore) Update(_ context.Context, p *productstore.Product, _ *int) error {
	s.updateArgs = p
	if s.updateErr != nil {
		return s.updateErr
	}
	return nil
}

func (s *stubStore) Restock(_ context.Context, complexID, productID, actorID uuid.UUID, quantity, totalCost int, method string, note *string) (*productstore.Product, *productstore.StockMovement, error) {
	s.restockArgs = &restockCall{complexID: complexID, productID: productID, actorID: actorID, quantity: quantity, totalCost: totalCost, method: method, note: note}
	s.restockCallCount++
	if s.restockErr != nil {
		return nil, nil, s.restockErr
	}
	return s.restockProduct, s.restockMovement, nil
}

func (s *stubStore) Adjust(_ context.Context, complexID, productID, actorID uuid.UUID, quantity int, reason string, note *string) (*productstore.Product, *productstore.StockMovement, error) {
	s.adjustArgs = &adjustCall{complexID: complexID, productID: productID, actorID: actorID, quantity: quantity, reason: reason, note: note}
	s.adjustCallCount++
	if s.adjustErr != nil {
		return nil, nil, s.adjustErr
	}
	return s.adjustProduct, s.adjustMovement, nil
}

func (s *stubStore) ListStockMovements(_ context.Context, _, _ uuid.UUID, _ data.Filters) ([]*productstore.StockMovement, data.Metadata, error) {
	if s.listMovementsErr != nil {
		return nil, data.Metadata{}, s.listMovementsErr
	}
	return s.listMovements, s.listMovementsMD, nil
}

// stubRecorder is a double for Recorder.
type stubRecorder struct{ entries []audit.Entry }

func (r *stubRecorder) Record(e audit.Entry) { r.entries = append(r.entries, e) }

func newTestService(store *stubStore, recorder *stubRecorder) *Service {
	return NewService(store, recorder)
}

// newTestHandler wires a real Handler over a stub store, following cashbox's
// own newTestHandler (internal/cashbox/stubs_test.go).
func newTestHandler(store *stubStore) (*Handler, *stubRecorder) {
	rec := &stubRecorder{}
	responder := httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return NewHandler(NewService(store, rec), responder, false), rec
}

// ownerRequest builds a request from the complex's authenticated owner, with
// the given path parameters bound — same shape as cashbox's own ownerRequest.
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

// decode parses a recorded JSON response body, following cashbox's own decode.
func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return body
}

// fieldError looks up one field's validation message from a decoded Problem
// body's "errors" array, following cashbox's own fieldError.
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
