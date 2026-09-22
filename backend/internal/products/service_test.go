package products

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
)

func testActor() Actor {
	id := uuid.New()
	return Actor{UserID: &id, IP: "127.0.0.1"}
}

// --- Create ------------------------------------------------------------

func TestCreatePropagatesADuplicateName(t *testing.T) {
	store := &stubStore{insertErr: productstore.ErrDuplicateProductName}
	svc := newTestService(store, &stubRecorder{})

	_, err := svc.Create(t.Context(), uuid.New(), testActor(), CreateInput{Name: "Coca Cola", TracksStock: true})
	if !errors.Is(err, productstore.ErrDuplicateProductName) {
		t.Fatalf("want ErrDuplicateProductName; got %v", err)
	}
}

func TestCreateRecordsAnAuditEntry(t *testing.T) {
	store := &stubStore{}
	rec := &stubRecorder{}
	svc := newTestService(store, rec)

	complexID := uuid.New()
	actor := testActor()
	product, err := svc.Create(t.Context(), complexID, actor, CreateInput{Name: "Agua", Price: 500, TracksStock: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(rec.entries) != 1 {
		t.Fatalf("want 1 audit entry; got %d", len(rec.entries))
	}
	entry := rec.entries[0]
	if entry.Action != "create" || entry.EntityType != "product" {
		t.Errorf("want action=create entity_type=product; got action=%s entity_type=%s", entry.Action, entry.EntityType)
	}
	if entry.EntityID == nil || *entry.EntityID != product.ID {
		t.Error("audit entry's EntityID does not name the created product")
	}
	if !product.Active {
		t.Error("a newly created product must be active")
	}
}

// --- Update --------------------------------------------------------------

func TestUpdateReportsAnEditConflictOnAStaleVersion(t *testing.T) {
	store := &stubStore{
		byID:      &productstore.Product{ID: uuid.New(), ComplexID: uuid.New(), Name: "Old", TracksStock: true},
		updateErr: data.ErrRecordNotFound,
	}
	svc := newTestService(store, &stubRecorder{})

	newName := "New"
	_, err := svc.Update(t.Context(), uuid.New(), testActor(), uuid.New(), UpdateInput{Name: &newName})
	if !errors.Is(err, ErrEditConflict) {
		t.Fatalf("want ErrEditConflict; got %v", err)
	}
}

func TestUpdateWithAnEmptyCategoryClearsIt(t *testing.T) {
	category := "Bebidas"
	store := &stubStore{byID: &productstore.Product{ID: uuid.New(), ComplexID: uuid.New(), Name: "Coca", Category: &category, TracksStock: true}}
	svc := newTestService(store, &stubRecorder{})

	empty := ""
	_, err := svc.Update(t.Context(), uuid.New(), testActor(), uuid.New(), UpdateInput{Category: &empty})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if store.updateArgs.Category != nil {
		t.Errorf("want category cleared to nil; got %+v", store.updateArgs.Category)
	}
}

func TestUpdateLeavesCategoryUntouchedWhenNil(t *testing.T) {
	category := "Bebidas"
	store := &stubStore{byID: &productstore.Product{ID: uuid.New(), ComplexID: uuid.New(), Name: "Coca", Category: &category, TracksStock: true}}
	svc := newTestService(store, &stubRecorder{})

	newPrice := 999
	_, err := svc.Update(t.Context(), uuid.New(), testActor(), uuid.New(), UpdateInput{Price: &newPrice})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if store.updateArgs.Category == nil || *store.updateArgs.Category != category {
		t.Errorf("want category left untouched (%q); got %+v", category, store.updateArgs.Category)
	}
	if store.updateArgs.Price != newPrice {
		t.Errorf("want price updated to %d; got %d", newPrice, store.updateArgs.Price)
	}
}

// TestUpdateRefusesTurningOffTracksStockWithNonZeroStock pins the
// least-surprising rule Service.Update documents: a product must be adjusted
// to zero before it can stop tracking stock.
func TestUpdateRefusesTurningOffTracksStockWithNonZeroStock(t *testing.T) {
	store := &stubStore{byID: &productstore.Product{ID: uuid.New(), ComplexID: uuid.New(), Name: "Snack", TracksStock: true, StockOnHand: 5}}
	svc := newTestService(store, &stubRecorder{})

	trackFalse := false
	_, err := svc.Update(t.Context(), uuid.New(), testActor(), uuid.New(), UpdateInput{TracksStock: &trackFalse})
	if !errors.Is(err, ErrProductHasStock) {
		t.Fatalf("want ErrProductHasStock; got %v", err)
	}
	if store.updateArgs != nil {
		t.Error("a refused update must not reach the store")
	}
}

// TestUpdateAllowsTurningOffTracksStockAtZero is the control: a product with
// zero stock may stop tracking it.
func TestUpdateAllowsTurningOffTracksStockAtZero(t *testing.T) {
	store := &stubStore{byID: &productstore.Product{ID: uuid.New(), ComplexID: uuid.New(), Name: "Snack", TracksStock: true, StockOnHand: 0}}
	svc := newTestService(store, &stubRecorder{})

	trackFalse := false
	_, err := svc.Update(t.Context(), uuid.New(), testActor(), uuid.New(), UpdateInput{TracksStock: &trackFalse})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if store.updateArgs == nil || store.updateArgs.TracksStock {
		t.Errorf("want tracks_stock=false persisted; got %+v", store.updateArgs)
	}
}

// TestUpdateAllowsTurningOnTracksStockUnconditionally is
// TestUpdateRefusesTurningOffTracksStockWithNonZeroStock's counterpart:
// turning tracking ON never checks stock_on_hand.
func TestUpdateAllowsTurningOnTracksStockUnconditionally(t *testing.T) {
	store := &stubStore{byID: &productstore.Product{ID: uuid.New(), ComplexID: uuid.New(), Name: "Snack", TracksStock: false, StockOnHand: 0}}
	svc := newTestService(store, &stubRecorder{})

	trackTrue := true
	_, err := svc.Update(t.Context(), uuid.New(), testActor(), uuid.New(), UpdateInput{TracksStock: &trackTrue})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if store.updateArgs == nil || !store.updateArgs.TracksStock {
		t.Errorf("want tracks_stock=true persisted; got %+v", store.updateArgs)
	}
}

// --- Restock / Adjust ----------------------------------------------------

func TestRestockPropagatesNoOpenCashSession(t *testing.T) {
	store := &stubStore{restockErr: productstore.ErrNoOpenCashSession}
	svc := newTestService(store, &stubRecorder{})

	_, _, err := svc.Restock(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), RestockInput{Quantity: 10, TotalCost: 5000, Method: "cash"})
	if !errors.Is(err, productstore.ErrNoOpenCashSession) {
		t.Fatalf("want ErrNoOpenCashSession; got %v", err)
	}
}

func TestRestockRecordsAnAuditEntry(t *testing.T) {
	productID := uuid.New()
	store := &stubStore{
		restockProduct:  &productstore.Product{ID: productID, StockOnHand: 24},
		restockMovement: &productstore.StockMovement{ID: uuid.New(), Kind: "restock", Quantity: 24},
	}
	rec := &stubRecorder{}
	svc := newTestService(store, rec)

	_, _, err := svc.Restock(t.Context(), uuid.New(), productID, testActor(), uuid.New(), RestockInput{Quantity: 24, TotalCost: 12000, Method: "cash"})
	if err != nil {
		t.Fatalf("Restock: %v", err)
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "restock" {
		t.Fatalf("want 1 audit entry action=restock; got %+v", rec.entries)
	}
}

func TestAdjustPropagatesNotTrackingStock(t *testing.T) {
	store := &stubStore{adjustErr: productstore.ErrProductNotTrackingStock}
	svc := newTestService(store, &stubRecorder{})

	_, _, err := svc.Adjust(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), AdjustInput{Quantity: -1, Reason: "breakage"})
	if !errors.Is(err, productstore.ErrProductNotTrackingStock) {
		t.Fatalf("want ErrProductNotTrackingStock; got %v", err)
	}
}

func TestAdjustRecordsAnAuditEntry(t *testing.T) {
	productID := uuid.New()
	store := &stubStore{
		adjustProduct:  &productstore.Product{ID: productID, StockOnHand: -3},
		adjustMovement: &productstore.StockMovement{ID: uuid.New(), Kind: "adjustment", Quantity: -3},
	}
	rec := &stubRecorder{}
	svc := newTestService(store, rec)

	_, _, err := svc.Adjust(t.Context(), uuid.New(), productID, testActor(), uuid.New(), AdjustInput{Quantity: -3, Reason: "breakage"})
	if err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "adjust" {
		t.Fatalf("want 1 audit entry action=adjust; got %+v", rec.entries)
	}
}

// --- StockMovements --------------------------------------------------------

func TestStockMovementsRefusesAMismatchedComplex(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	svc := newTestService(store, &stubRecorder{})

	_, _, err := svc.StockMovements(t.Context(), uuid.New(), uuid.New(), data.Filters{Limit: 50})
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound; got %v", err)
	}
}
