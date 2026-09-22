package products

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
)

// --- Create ------------------------------------------------------------

// TestCreateWithoutPriceIs422 pins the required-money-field fix: a body that
// omits price must not decode to a valid-looking 0 — the OpenAPI request
// validator that would catch a missing required field never runs in
// production, so the handler has to refuse it itself.
func TestCreateWithoutPriceIs422(t *testing.T) {
	store := &stubStore{}
	h, rec := newTestHandler(store)

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, nil, `{"name":"Coca Cola"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if msg, ok := fieldError(decode(t, w), "price"); !ok || msg != "must be provided" {
		t.Errorf("price error = %q (present %v), want %q", msg, ok, "must be provided")
	}
	if store.insertedProd != nil {
		t.Error("a rejected create must not be persisted")
	}
	if len(rec.entries) != 0 {
		t.Error("a rejected create must not be audited")
	}
}

func TestCreateWithoutNameIs422(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store)

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, nil, `{"price":500}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if msg, ok := fieldError(decode(t, w), "name"); !ok || msg != "must be provided" {
		t.Errorf("name error = %q (present %v), want %q", msg, ok, "must be provided")
	}
}

func TestCreateDefaultsTracksStockToTrue(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store)

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, nil, `{"name":"Coca Cola","price":500}`))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if store.insertedProd == nil || !store.insertedProd.TracksStock {
		t.Errorf("want tracks_stock defaulted to true; got %+v", store.insertedProd)
	}
}

func TestCreateReportsADuplicateNameAsAFieldError(t *testing.T) {
	store := &stubStore{insertErr: productstore.ErrDuplicateProductName}
	h, _ := newTestHandler(store)

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, nil, `{"name":"Coca Cola","price":500}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if msg, ok := fieldError(decode(t, w), "name"); !ok || msg != "product_name_taken" {
		t.Errorf("name error = %q (present %v), want %q", msg, ok, "product_name_taken")
	}
}

// --- Get -------------------------------------------------------------

// TestGetWithAMalformedProductIDIs404 pins ReadUUIDParam's failure mode: an
// unparseable path segment reads as "not found", the same convention every
// other handler in this codebase follows.
func TestGetWithAMalformedProductIDIs404(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store)

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.Get(w, ownerRequest(t, http.MethodGet, "/", complexID, map[string]string{"productID": "not-a-uuid"}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetWithAnUnknownProductIs404(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.Get(w, ownerRequest(t, http.MethodGet, "/", complexID, map[string]string{"productID": productID.String()}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- Update --------------------------------------------------------------

func TestUpdateOnAnUnknownProductIs404(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.Update(w, ownerRequest(t, http.MethodPatch, "/", complexID, map[string]string{"productID": productID.String()}, `{"price":600}`))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestUpdateWithAStaleVersionIs409(t *testing.T) {
	productID := uuid.New()
	store := &stubStore{
		byID:      &productstore.Product{ID: productID, ComplexID: uuid.New(), Name: "Coca", TracksStock: true},
		updateErr: data.ErrRecordNotFound,
	}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), map[string]string{"productID": productID.String()}, `{"price":600}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestUpdateRefusesTurningOffTracksStockWithStockIs409(t *testing.T) {
	productID := uuid.New()
	store := &stubStore{byID: &productstore.Product{ID: productID, ComplexID: uuid.New(), Name: "Snack", TracksStock: true, StockOnHand: 5}}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), map[string]string{"productID": productID.String()}, `{"tracks_stock":false}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- Restock ---------------------------------------------------------------

func TestRestockRejectsInvalidInput(t *testing.T) {
	tooLongNote := strings.Repeat("a", noteMaxLen+1)

	tests := []struct {
		name string
		body string
	}{
		{"quantity missing", `{"total_cost":5000,"method":"cash"}`},
		{"total_cost missing", `{"quantity":10,"method":"cash"}`},
		{"quantity zero", `{"quantity":0,"total_cost":5000,"method":"cash"}`},
		{"quantity exceeds the cap", `{"quantity":100001,"total_cost":5000,"method":"cash"}`},
		{"total_cost zero (a free restock is an adjustment)", `{"quantity":10,"total_cost":0,"method":"cash"}`},
		{"total_cost exceeds the cap", `{"quantity":10,"total_cost":2000000001,"method":"cash"}`},
		{"mercadopago is not a counter method", `{"quantity":10,"total_cost":5000,"method":"mercadopago"}`},
		{"note too long", `{"quantity":10,"total_cost":5000,"method":"cash","note":"` + tooLongNote + `"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{}
			h, rec := newTestHandler(store)

			complexID, productID := uuid.New(), uuid.New()
			w := httptest.NewRecorder()
			h.Restock(w, ownerRequest(t, http.MethodPost, "/", complexID,
				map[string]string{"productID": productID.String()}, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if store.restockArgs != nil {
				t.Error("an invalid restock must not reach the store")
			}
			if len(rec.entries) != 0 {
				t.Error("a rejected restock must not be audited")
			}
		})
	}
}

// TestRestockWithoutOpenSessionIs409 pins the domain-error mapping for
// productstore.ErrNoOpenCashSession.
func TestRestockWithoutOpenSessionIs409(t *testing.T) {
	store := &stubStore{restockErr: productstore.ErrNoOpenCashSession}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.Restock(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"productID": productID.String()}, `{"quantity":10,"total_cost":5000,"method":"cash"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRestockOnAnUnknownProductIs404(t *testing.T) {
	store := &stubStore{restockErr: data.ErrRecordNotFound}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.Restock(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"productID": productID.String()}, `{"quantity":10,"total_cost":5000,"method":"cash"}`))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRestockOnAnInactiveProductIs409(t *testing.T) {
	store := &stubStore{restockErr: productstore.ErrProductInactive}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.Restock(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"productID": productID.String()}, `{"quantity":10,"total_cost":5000,"method":"cash"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRestockOnAProductThatDoesNotTrackStockIs409(t *testing.T) {
	store := &stubStore{restockErr: productstore.ErrProductNotTrackingStock}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.Restock(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"productID": productID.String()}, `{"quantity":10,"total_cost":5000,"method":"cash"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- Adjust ------------------------------------------------------------

func TestAdjustRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"quantity missing", `{"reason":"breakage"}`},
		{"quantity zero", `{"quantity":0,"reason":"breakage"}`},
		{"quantity exceeds the cap", `{"quantity":100001,"reason":"breakage"}`},
		{"quantity below the negative cap", `{"quantity":-100001,"reason":"breakage"}`},
		{"bad reason", `{"quantity":-1,"reason":"because"}`},
		{"reason missing", `{"quantity":-1,"reason":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{}
			h, rec := newTestHandler(store)

			complexID, productID := uuid.New(), uuid.New()
			w := httptest.NewRecorder()
			h.Adjust(w, ownerRequest(t, http.MethodPost, "/", complexID,
				map[string]string{"productID": productID.String()}, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if store.adjustArgs != nil {
				t.Error("an invalid adjustment must not reach the store")
			}
			if len(rec.entries) != 0 {
				t.Error("a rejected adjustment must not be audited")
			}
		})
	}
}

func TestAdjustOnAProductThatDoesNotTrackStockIs409(t *testing.T) {
	store := &stubStore{adjustErr: productstore.ErrProductNotTrackingStock}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.Adjust(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"productID": productID.String()}, `{"quantity":-1,"reason":"breakage"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- List / ListStockMovements -------------------------------------------

func TestListFiltersByActive(t *testing.T) {
	store := &stubStore{listProducts: []*productstore.Product{{ID: uuid.New(), Name: "Active One", Active: true}}}
	h, _ := newTestHandler(store)

	complexID := uuid.New()
	w := httptest.NewRecorder()
	h.List(w, ownerRequest(t, http.MethodGet, "/?active=true", complexID, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestListStockMovementsOnAnUnknownProductIs404(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.ListStockMovements(w, ownerRequest(t, http.MethodGet, "/", complexID, map[string]string{"productID": productID.String()}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestListStockMovementsWithAMalformedCursorIs400(t *testing.T) {
	store := &stubStore{byID: &productstore.Product{ID: uuid.New()}, listMovementsErr: data.ErrInvalidCursor}
	h, _ := newTestHandler(store)

	complexID, productID := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	req := ownerRequest(t, http.MethodGet, "/?cursor=not-a-valid-cursor!!", complexID, map[string]string{"productID": productID.String()}, "")
	h.ListStockMovements(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400; got %d (%s)", w.Code, w.Body.String())
	}
}
