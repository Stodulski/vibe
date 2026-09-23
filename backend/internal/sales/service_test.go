package sales

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

func testActor() Actor {
	id := uuid.New()
	return Actor{UserID: &id, IP: "127.0.0.1"}
}

// --- Create ------------------------------------------------------------

func TestCreatePropagatesNoOpenCashSession(t *testing.T) {
	store := &stubStore{createErr: salestore.ErrNoOpenCashSession}
	svc := newTestService(store, &stubRecorder{})

	_, _, err := svc.Create(t.Context(), uuid.New(), testActor(), uuid.New(), CreateInput{
		Items:  []salestore.ItemInput{{ProductID: uuid.New(), Quantity: 1}},
		Method: "cash",
	})
	if !errors.Is(err, salestore.ErrNoOpenCashSession) {
		t.Fatalf("want ErrNoOpenCashSession; got %v", err)
	}
}

func TestCreatePropagatesInvalidItems(t *testing.T) {
	badID := uuid.New()
	store := &stubStore{createErr: &salestore.ErrInvalidItems{Problems: map[uuid.UUID]salestore.ItemProblem{badID: salestore.ItemInactive}}}
	svc := newTestService(store, &stubRecorder{})

	_, _, err := svc.Create(t.Context(), uuid.New(), testActor(), uuid.New(), CreateInput{
		Items:  []salestore.ItemInput{{ProductID: badID, Quantity: 1}},
		Method: "cash",
	})
	var invalid *salestore.ErrInvalidItems
	if !errors.As(err, &invalid) {
		t.Fatalf("want *ErrInvalidItems; got %v", err)
	}
	if invalid.Problems[badID] != salestore.ItemInactive {
		t.Errorf("want %s flagged inactive; got %+v", badID, invalid.Problems)
	}
}

func TestCreateRecordsAnAuditEntry(t *testing.T) {
	sale := &salestore.Sale{ID: uuid.New(), Total: 1500}
	warnings := []salestore.StockWarning{{ProductID: uuid.New(), ProductName: "Snack", StockOnHand: -1}}
	store := &stubStore{createSale: sale, createWarnings: warnings}
	rec := &stubRecorder{}
	svc := newTestService(store, rec)

	complexID := uuid.New()
	actor := testActor()
	result, gotWarnings, err := svc.Create(t.Context(), complexID, actor, uuid.New(), CreateInput{
		Items:  []salestore.ItemInput{{ProductID: uuid.New(), Quantity: 1}},
		Method: "cash",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Sale != sale {
		t.Error("want the store's sale returned")
	}
	if len(gotWarnings) != 1 {
		t.Fatalf("want the stock warning propagated; got %+v", gotWarnings)
	}
	if len(rec.entries) != 1 {
		t.Fatalf("want 1 audit entry; got %d", len(rec.entries))
	}
	entry := rec.entries[0]
	if entry.Action != "create" || entry.EntityType != "sale" {
		t.Errorf("want action=create entity_type=sale; got action=%s entity_type=%s", entry.Action, entry.EntityType)
	}
	if entry.EntityID == nil || *entry.EntityID != sale.ID {
		t.Error("audit entry's EntityID does not name the created sale")
	}
}

// --- Get / List ------------------------------------------------------------

func TestGetPropagatesRecordNotFound(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	svc := newTestService(store, &stubRecorder{})

	_, err := svc.Get(t.Context(), uuid.New(), uuid.New())
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound; got %v", err)
	}
}

func TestGetAttachesItems(t *testing.T) {
	sale := &salestore.Sale{ID: uuid.New()}
	items := []*salestore.SaleItem{{ID: uuid.New(), SaleID: sale.ID}}
	store := &stubStore{byID: sale, itemsBySale: items}
	svc := newTestService(store, &stubRecorder{})

	result, err := svc.Get(t.Context(), uuid.New(), sale.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0] != items[0] {
		t.Errorf("want the sale's items attached; got %+v", result.Items)
	}
}

func TestListGroupsItemsBySale(t *testing.T) {
	sale1 := &salestore.Sale{ID: uuid.New()}
	sale2 := &salestore.Sale{ID: uuid.New()}
	item1 := &salestore.SaleItem{ID: uuid.New(), SaleID: sale1.ID}
	item2 := &salestore.SaleItem{ID: uuid.New(), SaleID: sale2.ID}
	store := &stubStore{
		listSales:      []*salestore.Sale{sale1, sale2},
		itemsBySaleIDs: []*salestore.SaleItem{item1, item2},
	}
	svc := newTestService(store, &stubRecorder{})

	result, _, err := svc.List(t.Context(), uuid.New(), nil, data.Filters{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("want 2 sales; got %d", len(result))
	}
	if len(result[0].Items) != 1 || result[0].Items[0] != item1 {
		t.Errorf("want sale1's own item attached; got %+v", result[0].Items)
	}
	if len(result[1].Items) != 1 || result[1].Items[0] != item2 {
		t.Errorf("want sale2's own item attached; got %+v", result[1].Items)
	}
}

func TestListPassesTheSessionFilterThrough(t *testing.T) {
	sessionID := uuid.New()
	store := &stubStore{}
	svc := newTestService(store, &stubRecorder{})

	if _, _, err := svc.List(t.Context(), uuid.New(), &sessionID, data.Filters{Limit: 50}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if store.listArgs == nil || store.listArgs.sessionID == nil || *store.listArgs.sessionID != sessionID {
		t.Errorf("want the session filter passed to the store; got %+v", store.listArgs)
	}
}

// --- Void ------------------------------------------------------------------

func TestVoidPropagatesAlreadyVoided(t *testing.T) {
	sale := &salestore.Sale{ID: uuid.New()}
	store := &stubStore{byID: sale, voidErr: salestore.ErrAlreadyVoided}
	svc := newTestService(store, &stubRecorder{})

	_, err := svc.Void(t.Context(), uuid.New(), sale.ID, testActor(), uuid.New(), nil)
	if !errors.Is(err, salestore.ErrAlreadyVoided) {
		t.Fatalf("want ErrAlreadyVoided; got %v", err)
	}
}

func TestVoidPropagatesNoOpenCashSession(t *testing.T) {
	sale := &salestore.Sale{ID: uuid.New()}
	store := &stubStore{byID: sale, voidErr: salestore.ErrNoOpenCashSession}
	svc := newTestService(store, &stubRecorder{})

	_, err := svc.Void(t.Context(), uuid.New(), sale.ID, testActor(), uuid.New(), nil)
	if !errors.Is(err, salestore.ErrNoOpenCashSession) {
		t.Fatalf("want ErrNoOpenCashSession; got %v", err)
	}
}

func TestVoidRecordsAnAuditEntry(t *testing.T) {
	before := &salestore.Sale{ID: uuid.New()}
	after := &salestore.Sale{ID: before.ID}
	store := &stubStore{byID: before, voidSale: after}
	rec := &stubRecorder{}
	svc := newTestService(store, rec)

	_, err := svc.Void(t.Context(), uuid.New(), before.ID, testActor(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("Void: %v", err)
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "void" {
		t.Fatalf("want 1 audit entry action=void; got %+v", rec.entries)
	}
}

func TestVoidRefusesAnUnknownSale(t *testing.T) {
	store := &stubStore{getByIDErr: data.ErrRecordNotFound}
	svc := newTestService(store, &stubRecorder{})

	_, err := svc.Void(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), nil)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound; got %v", err)
	}
	if store.voidCalls != 0 {
		t.Error("a void on an unknown sale must not reach the store's own Void")
	}
}
