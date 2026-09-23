package sales

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
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// stubStore is a hand-written double for Store, following this codebase's
// existing fake/stub convention (see internal/products/stubs_test.go): plain
// fields the test sets up front, a handful of "last call" captures, no
// mocking framework.
type stubStore struct {
	createSale     *salestore.Sale
	createItems    []*salestore.SaleItem
	createWarnings []salestore.StockWarning
	createErr      error
	createArgs     *createCall
	createCalls    int

	byID       *salestore.Sale
	getByIDErr error

	itemsBySale    []*salestore.SaleItem
	itemsBySaleErr error

	itemsBySaleIDs    []*salestore.SaleItem
	itemsBySaleIDsErr error

	listSales []*salestore.Sale
	listMD    data.Metadata
	listErr   error
	listArgs  *listCall

	voidSale  *salestore.Sale
	voidErr   error
	voidCalls int
}

type createCall struct {
	complexID, actorID uuid.UUID
	items              []salestore.ItemInput
	method             string
	note               *string
}

type listCall struct {
	complexID uuid.UUID
	sessionID *uuid.UUID
	filters   data.Filters
}

func (s *stubStore) Create(_ context.Context, complexID, actorID uuid.UUID, items []salestore.ItemInput, method string, note *string) (*salestore.Sale, []*salestore.SaleItem, []salestore.StockWarning, error) {
	s.createArgs = &createCall{complexID: complexID, actorID: actorID, items: items, method: method, note: note}
	s.createCalls++
	if s.createErr != nil {
		return nil, nil, nil, s.createErr
	}
	sale := s.createSale
	if sale == nil {
		sale = &salestore.Sale{ID: uuid.New(), ComplexID: complexID}
	}
	return sale, s.createItems, s.createWarnings, nil
}

func (s *stubStore) GetByID(_ context.Context, _, _ uuid.UUID) (*salestore.Sale, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	return s.byID, nil
}

func (s *stubStore) ListItemsBySale(_ context.Context, _, _ uuid.UUID) ([]*salestore.SaleItem, error) {
	if s.itemsBySaleErr != nil {
		return nil, s.itemsBySaleErr
	}
	return s.itemsBySale, nil
}

func (s *stubStore) ListItemsBySaleIDs(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]*salestore.SaleItem, error) {
	if s.itemsBySaleIDsErr != nil {
		return nil, s.itemsBySaleIDsErr
	}
	return s.itemsBySaleIDs, nil
}

func (s *stubStore) ListByComplex(_ context.Context, complexID uuid.UUID, sessionID *uuid.UUID, filters data.Filters) ([]*salestore.Sale, data.Metadata, error) {
	s.listArgs = &listCall{complexID: complexID, sessionID: sessionID, filters: filters}
	if s.listErr != nil {
		return nil, data.Metadata{}, s.listErr
	}
	return s.listSales, s.listMD, nil
}

func (s *stubStore) Void(_ context.Context, _, _, _ uuid.UUID, _ *string) (*salestore.Sale, error) {
	s.voidCalls++
	if s.voidErr != nil {
		return nil, s.voidErr
	}
	return s.voidSale, nil
}

// stubRecorder is a double for Recorder.
type stubRecorder struct{ entries []audit.Entry }

func (r *stubRecorder) Record(e audit.Entry) { r.entries = append(r.entries, e) }

func newTestService(store *stubStore, recorder *stubRecorder) *Service {
	return NewService(store, recorder)
}

// newTestHandler wires a real Handler over a stub store, following
// products'/cashbox's own newTestHandler.
func newTestHandler(store *stubStore) (*Handler, *stubRecorder) {
	rec := &stubRecorder{}
	responder := httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return NewHandler(NewService(store, rec), responder, false), rec
}

// ownerRequest builds a request from the complex's authenticated owner, with
// the given path parameters bound — same shape as products'/cashbox's own
// ownerRequest.
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

// decode parses a recorded JSON response body, following products'/cashbox's
// own decode.
func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return body
}

// fieldError looks up one field's validation message from a decoded Problem
// body's "errors" array, following products'/cashbox's own fieldError.
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
