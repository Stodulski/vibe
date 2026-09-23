package sales

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// --- Create ------------------------------------------------------------

func TestCreateRejectsInvalidInput(t *testing.T) {
	tooLongNote := strings.Repeat("a", noteMaxLen+1)
	productA := uuid.New()

	tests := []struct {
		name string
		body string
	}{
		{"items missing", `{"method":"cash"}`},
		{"items empty", `{"items":[],"method":"cash"}`},
		{"method missing", `{"items":[{"product_id":"` + productA.String() + `","quantity":1}]}`},
		{"mercadopago is not a counter method", `{"items":[{"product_id":"` + productA.String() + `","quantity":1}],"method":"mercadopago"}`},
		{"quantity missing", `{"items":[{"product_id":"` + productA.String() + `"}],"method":"cash"}`},
		{"quantity zero", `{"items":[{"product_id":"` + productA.String() + `","quantity":0}],"method":"cash"}`},
		{"quantity exceeds the cap", `{"items":[{"product_id":"` + productA.String() + `","quantity":1001}],"method":"cash"}`},
		{
			"duplicate product",
			`{"items":[{"product_id":"` + productA.String() + `","quantity":1},{"product_id":"` + productA.String() + `","quantity":2}],"method":"cash"}`,
		},
		{
			"note too long",
			`{"items":[{"product_id":"` + productA.String() + `","quantity":1}],"method":"cash","note":"` + tooLongNote + `"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{}
			h, rec := newTestHandler(store)

			w := httptest.NewRecorder()
			h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if store.createCalls != 0 {
				t.Error("an invalid create must not reach the store")
			}
			if len(rec.entries) != 0 {
				t.Error("a rejected create must not be audited")
			}
		})
	}
}

func TestCreateWithTooManyItemsIs422(t *testing.T) {
	items := make([]string, 0, 51)
	for range 51 {
		items = append(items, `{"product_id":"`+uuid.New().String()+`","quantity":1}`)
	}
	body := `{"items":[` + strings.Join(items, ",") + `],"method":"cash"}`

	store := &stubStore{}
	h, _ := newTestHandler(store)
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, body))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCreateWithoutOpenSessionIs409(t *testing.T) {
	store := &stubStore{createErr: salestore.ErrNoOpenCashSession}
	h, _ := newTestHandler(store)

	body := `{"items":[{"product_id":"` + uuid.New().String() + `","quantity":1}],"method":"cash"}`
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, body))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCreateWithAZeroTotalIs422(t *testing.T) {
	store := &stubStore{createErr: salestore.ErrZeroTotal}
	h, _ := newTestHandler(store)

	body := `{"items":[{"product_id":"` + uuid.New().String() + `","quantity":1}],"method":"cash"}`
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, body))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCreateWithATotalOverTheCapIs422(t *testing.T) {
	store := &stubStore{createErr: salestore.ErrTotalExceedsCap}
	h, _ := newTestHandler(store)

	body := `{"items":[{"product_id":"` + uuid.New().String() + `","quantity":1}],"method":"cash"}`
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, body))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestCreateWithAnInvalidItemNamesItsIndex pins *salestore.ErrInvalidItems'
// field-scoped mapping: the response must name WHICH item, by index, not
// just "some item was bad".
func TestCreateWithAnInvalidItemNamesItsIndex(t *testing.T) {
	good, bad := uuid.New(), uuid.New()
	store := &stubStore{createErr: &salestore.ErrInvalidItems{Problems: map[uuid.UUID]salestore.ItemProblem{bad: salestore.ItemInactive}}}
	h, _ := newTestHandler(store)

	body := `{"items":[{"product_id":"` + good.String() + `","quantity":1},{"product_id":"` + bad.String() + `","quantity":1}],"method":"cash"}`
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, body))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if _, ok := fieldError(decode(t, w), "items[0].product_id"); ok {
		t.Error("want the GOOD item (index 0) to carry no field error")
	}
	if msg, ok := fieldError(decode(t, w), "items[1].product_id"); !ok || msg == "" {
		t.Errorf("want the BAD item (index 1) to carry a field error; got %q (present %v)", msg, ok)
	}
}

func TestCreateSucceedsAndReportsStockWarnings(t *testing.T) {
	sale := &salestore.Sale{ID: uuid.New(), Total: 800}
	warnings := []salestore.StockWarning{{ProductID: uuid.New(), ProductName: "Snack", StockOnHand: -1}}
	store := &stubStore{createSale: sale, createWarnings: warnings}
	h, rec := newTestHandler(store)

	body := `{"items":[{"product_id":"` + uuid.New().String() + `","quantity":1}],"method":"cash"}`
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, body))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	body2 := decode(t, w)
	warningsField, ok := body2["stock_warnings"].([]any)
	if !ok || len(warningsField) != 1 {
		t.Errorf("want 1 stock warning in the response; got %+v", body2["stock_warnings"])
	}
	if len(rec.entries) != 1 {
		t.Errorf("want 1 audit entry; got %d", len(rec.entries))
	}
}

// --- Get -------------------------------------------------------------

func TestGetWithAMalformedSaleIDIs404(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Get(w, ownerRequest(t, http.MethodGet, "/", uuid.New(), map[string]string{"saleID": "not-a-uuid"}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetWithAnUnknownSaleIs404(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Get(w, ownerRequest(t, http.MethodGet, "/", uuid.New(), map[string]string{"saleID": uuid.New().String()}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- List ------------------------------------------------------------------

func TestListWithAMalformedSessionIDIs400(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.List(w, ownerRequest(t, http.MethodGet, "/?session_id=not-a-uuid", uuid.New(), nil, ""))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestListWithAMalformedCursorIs400(t *testing.T) {
	store := &stubStore{listErr: data.ErrInvalidCursor}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.List(w, ownerRequest(t, http.MethodGet, "/?cursor=not-a-valid-cursor!!", uuid.New(), nil, ""))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestListFiltersBySession(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store)

	sessionID := uuid.New()
	w := httptest.NewRecorder()
	h.List(w, ownerRequest(t, http.MethodGet, "/?session_id="+sessionID.String(), uuid.New(), nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if store.listArgs == nil || store.listArgs.sessionID == nil || *store.listArgs.sessionID != sessionID {
		t.Errorf("want the session filter passed through; got %+v", store.listArgs)
	}
}

// --- Void ------------------------------------------------------------------

func TestVoidWithAMalformedSaleIDIs404(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Void(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), map[string]string{"saleID": "not-a-uuid"}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestVoidWithNoOpenSessionIs409(t *testing.T) {
	store := &stubStore{byID: &salestore.Sale{ID: uuid.New()}, voidErr: salestore.ErrNoOpenCashSession}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Void(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), map[string]string{"saleID": uuid.New().String()}, ""))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestVoidASecondTimeIs409 pins ErrAlreadyVoided's mapping — the "double
// void" case the feature document's own test list names.
func TestVoidASecondTimeIs409(t *testing.T) {
	store := &stubStore{byID: &salestore.Sale{ID: uuid.New()}, voidErr: salestore.ErrAlreadyVoided}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Void(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), map[string]string{"saleID": uuid.New().String()}, ""))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestVoidOnAnUnknownSaleIs404(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Void(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), map[string]string{"saleID": uuid.New().String()}, ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestVoidAcceptsAnEmptyBody pins that the void's note is optional — an
// owner voiding a sale usually sends no body at all.
func TestVoidAcceptsAnEmptyBody(t *testing.T) {
	store := &stubStore{byID: &salestore.Sale{ID: uuid.New()}, voidSale: &salestore.Sale{ID: uuid.New(), VoidedBy: nil}}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Void(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), map[string]string{"saleID": uuid.New().String()}, ""))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestVoidRejectsATooLongNote(t *testing.T) {
	tooLongNote := strings.Repeat("a", noteMaxLen+1)
	store := &stubStore{byID: &salestore.Sale{ID: uuid.New()}}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Void(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), map[string]string{"saleID": uuid.New().String()}, `{"note":"`+tooLongNote+`"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if store.voidCalls != 0 {
		t.Error("a rejected void must not reach the store")
	}
}
