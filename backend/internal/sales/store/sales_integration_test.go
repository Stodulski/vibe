//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

const checkViolation = "23514"

// wantPgError fails the test unless err is a *pgconn.PgError refused by
// exactly sqlState/constraint — see internal/cashbox/store's own copy for why
// the constraint NAME is asserted, not just the SQLSTATE.
func wantPgError(t *testing.T, err error, sqlState, constraint string) {
	t.Helper()

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want a *pgconn.PgError refused by %s; got %T: %v", constraint, err, err)
	}
	if pgErr.Code != sqlState || pgErr.ConstraintName != constraint {
		t.Fatalf("want %s (%s); got %q (%s): %s", constraint, sqlState, pgErr.ConstraintName, pgErr.Code, pgErr.Message)
	}
}

func insertProduct(t *testing.T, f *datatest.Fixture, name string, price int, tracksStock bool) *productstore.Product {
	t.Helper()
	p := &productstore.Product{ComplexID: f.ComplexID, Name: name, Price: price, TracksStock: tracksStock}
	if err := f.Stores.Products.Insert(f.Scoped(context.Background()), p); err != nil {
		t.Fatalf("inserting product: %v", err)
	}
	return p
}

func openSession(t *testing.T, f *datatest.Fixture, openingCash int64) *cashboxstore.CashSession {
	t.Helper()
	session := &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: openingCash}
	if err := f.Stores.Cashbox.OpenSession(f.Scoped(context.Background()), session); err != nil {
		t.Fatalf("opening cash session: %v", err)
	}
	return session
}

// TestIntegration_CreateWritesSaleItemsIncomeAndStockAtomically pins the
// whole point of Store.Create: one transaction writes the sale, its items,
// the cash income, and one stock movement per tracked item together, and the
// cashbox's own reconciliation sees the income for a cash sale.
func TestIntegration_CreateWritesSaleItemsIncomeAndStockAtomically(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	tracked := insertProduct(t, f, "Coca Cola 500ml", 800, true)
	untracked := insertProduct(t, f, "Court Rental Hour", 500000, false)
	session := openSession(t, f, 10000)

	// Give the tracked product enough stock that selling 2 of it stays
	// positive — this test's point is that a sale within stock draws no
	// warning; TestIntegration_SaleCanOversellAndReportsTheWarning covers
	// the opposite case on its own.
	if _, _, err := f.Stores.Products.Restock(ctx, f.ComplexID, tracked.ID, f.UserID, 10, 5000, "cash", nil); err != nil {
		t.Fatalf("restocking before the sale: %v", err)
	}

	sale, items, warnings, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{
			{ProductID: tracked.ID, Quantity: 2},
			{ProductID: untracked.ID, Quantity: 1},
		}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	wantTotal := 800*2 + 500000
	if sale.Total != wantTotal {
		t.Errorf("want total=%d; got %d", wantTotal, sale.Total)
	}
	if sale.SessionID != session.ID {
		t.Errorf("want session_id=%s; got %s", session.ID, sale.SessionID)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 sale items; got %d", len(items))
	}
	if len(warnings) != 0 {
		t.Errorf("want no stock warnings (stock stayed positive); got %+v", warnings)
	}

	cashMovement, err := f.Stores.Cashbox.GetMovementByID(ctx, f.ComplexID, sale.CashMovementID)
	if err != nil {
		t.Fatalf("reading the linked cash movement: %v", err)
	}
	if cashMovement.Kind != "income" || cashMovement.Category != "sale" || cashMovement.Amount != wantTotal || cashMovement.Method != "cash" {
		t.Errorf("want a %d cash sale income; got %+v", wantTotal, cashMovement)
	}

	totals, err := f.Stores.Cashbox.SumBySession(ctx, f.ComplexID, session.ID)
	if err != nil {
		t.Fatalf("summing session movements: %v", err)
	}
	cashIncome, _ := cashboxstore.CashIncomeAndExpense(totals)
	if cashIncome != wantTotal {
		t.Errorf("want cash income %d from the sale; got %d", wantTotal, cashIncome)
	}

	trackedProduct, err := f.Stores.Products.GetByID(ctx, f.ComplexID, tracked.ID)
	if err != nil {
		t.Fatalf("reading tracked product: %v", err)
	}
	if trackedProduct.StockOnHand != 8 {
		t.Errorf("want stock_on_hand=8 after restocking 10 and selling 2; got %d", trackedProduct.StockOnHand)
	}

	untrackedProduct, err := f.Stores.Products.GetByID(ctx, f.ComplexID, untracked.ID)
	if err != nil {
		t.Fatalf("reading untracked product: %v", err)
	}
	if untrackedProduct.StockOnHand != 0 {
		t.Errorf("want an untracked product's stock_on_hand untouched (0); got %d", untrackedProduct.StockOnHand)
	}
}

// TestIntegration_SaleCanOversellAndReportsTheWarning pins the owner
// decision that selling past zero stock is allowed, flagged with a warning.
func TestIntegration_SaleCanOversellAndReportsTheWarning(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Oversold Snack", 300, true)
	openSession(t, f, 5000)

	_, _, warnings, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 5}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("want exactly one stock warning; got %+v", warnings)
	}
	if warnings[0].ProductID != product.ID || warnings[0].StockOnHand != -5 {
		t.Errorf("want a warning for %s at -5; got %+v", product.ID, warnings[0])
	}
}

// TestIntegration_UntrackedProductGetsNoStockMovement pins that a product
// with tracks_stock = false never gets a 'sale' stock movement.
func TestIntegration_UntrackedProductGetsNoStockMovement(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Court Time", 100000, false)
	openSession(t, f, 5000)

	sale, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	movements, _, err := f.Stores.Products.ListStockMovements(ctx, f.ComplexID, product.ID, data.Filters{Limit: 50})
	if err != nil {
		t.Fatalf("listing stock movements: %v", err)
	}
	if len(movements) != 0 {
		t.Errorf("want no stock movement for an untracked product's sale %s; got %+v", sale.ID, movements)
	}
}

// TestIntegration_PriceSnapshotSurvivesALaterPriceChange pins that
// sale_items.unit_price is a snapshot: changing the product's price later
// must never change what a past sale reads as having charged.
func TestIntegration_PriceSnapshotSurvivesALaterPriceChange(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Agua Mineral", 500, true)
	openSession(t, f, 5000)

	sale, items, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 3}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if items[0].UnitPrice != 500 || items[0].ProductName != "Agua Mineral" {
		t.Fatalf("want a 500 snapshot named Agua Mineral; got %+v", items[0])
	}

	product.Price = 900
	product.Name = "Agua Mineral 600ml"
	if err := f.Stores.Products.Update(ctx, product, nil); err != nil {
		t.Fatalf("updating price: %v", err)
	}

	reread, err := f.Stores.Sales.ListItemsBySale(ctx, f.ComplexID, sale.ID)
	if err != nil {
		t.Fatalf("re-reading sale items: %v", err)
	}
	if reread[0].UnitPrice != 500 || reread[0].ProductName != "Agua Mineral" {
		t.Errorf("want the ORIGINAL snapshot preserved; got %+v", reread[0])
	}
}

// TestIntegration_CreateRefusesWithNoOpenSession pins ErrNoOpenCashSession.
func TestIntegration_CreateRefusesWithNoOpenSession(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "No Session Item", 500, true)

	_, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil)
	if !errors.Is(err, salestore.ErrNoOpenCashSession) {
		t.Fatalf("want ErrNoOpenCashSession; got %v", err)
	}
}

// TestIntegration_CreateRefusesAnUnknownOrInactiveProduct pins
// *ErrInvalidItems for both cases in the same call.
func TestIntegration_CreateRefusesAnUnknownOrInactiveProduct(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	inactive := insertProduct(t, f, "Discontinued", 500, true)
	inactive.Active = false
	if err := f.Stores.Products.Update(ctx, inactive, nil); err != nil {
		t.Fatalf("deactivating: %v", err)
	}
	unknown := uuid.New()
	openSession(t, f, 5000)

	_, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{
			{ProductID: inactive.ID, Quantity: 1},
			{ProductID: unknown, Quantity: 1},
		}, "cash", nil)

	var invalid *salestore.ErrInvalidItems
	if !errors.As(err, &invalid) {
		t.Fatalf("want *ErrInvalidItems; got %T: %v", err, err)
	}
	if invalid.Problems[inactive.ID] != salestore.ItemInactive {
		t.Errorf("want %s flagged inactive; got %+v", inactive.ID, invalid.Problems)
	}
	if invalid.Problems[unknown] != salestore.ItemNotFound {
		t.Errorf("want %s flagged not_found; got %+v", unknown, invalid.Problems)
	}
}

// TestIntegration_CreateRefusesAZeroTotal pins the "a free sale is a gift"
// decision: every priced item at 0 refuses with ErrZeroTotal.
func TestIntegration_CreateRefusesAZeroTotal(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	free := insertProduct(t, f, "Free Sample", 0, true)
	openSession(t, f, 5000)

	_, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: free.ID, Quantity: 3}}, "cash", nil)
	if !errors.Is(err, salestore.ErrZeroTotal) {
		t.Fatalf("want ErrZeroTotal; got %v", err)
	}
}

// TestIntegration_CreateRefusesATotalOverTheCap pins ErrTotalExceedsCap.
func TestIntegration_CreateRefusesATotalOverTheCap(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	expensive := insertProduct(t, f, "Very Expensive Item", 2_000_000_000, true)
	openSession(t, f, 5000)

	_, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: expensive.ID, Quantity: 2}}, "cash", nil)
	if !errors.Is(err, salestore.ErrTotalExceedsCap) {
		t.Fatalf("want ErrTotalExceedsCap; got %v", err)
	}
}

// TestIntegration_VoidRestoresStockAndVoidsTheIncome pins Store.Void's
// atomicity: stock is restored, and a new cash movement voids the sale's
// income in the CURRENTLY open session (which may differ from the sale's
// own, when correcting a sale from an earlier shift).
func TestIntegration_VoidRestoresStockAndVoidsTheIncome(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Voidable Item", 700, true)
	firstSession := openSession(t, f, 5000)

	sale, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 4}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Close the original session and open a new one — the void must land in
	// the NEW one, not the sale's own.
	if _, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, firstSession.ID, f.UserID, 5000, 0, time.Now(), nil); err != nil {
		t.Fatalf("closing the original session: %v", err)
	}
	newSession := openSession(t, f, 1000)

	note := "customer changed their mind"
	voided, err := f.Stores.Sales.Void(ctx, f.ComplexID, sale.ID, f.UserID, &note)
	if err != nil {
		t.Fatalf("Void: %v", err)
	}
	if !voided.IsVoided() {
		t.Fatal("want the sale marked voided")
	}
	if voided.VoidCashMovementID == nil {
		t.Fatal("want a linked void cash movement")
	}

	restored, err := f.Stores.Products.GetByID(ctx, f.ComplexID, product.ID)
	if err != nil {
		t.Fatalf("reading product: %v", err)
	}
	if restored.StockOnHand != 0 {
		t.Errorf("want stock restored to 0 (started at -4, +4 back); got %d", restored.StockOnHand)
	}

	voidMovement, err := f.Stores.Cashbox.GetMovementByID(ctx, f.ComplexID, *voided.VoidCashMovementID)
	if err != nil {
		t.Fatalf("reading the void cash movement: %v", err)
	}
	if voidMovement.SessionID != newSession.ID {
		t.Errorf("want the void to land in the currently open session %s; got %s", newSession.ID, voidMovement.SessionID)
	}
	if voidMovement.Kind != "expense" || voidMovement.Category != "sale" || voidMovement.Amount != sale.Total {
		t.Errorf("want an expense voiding the %d sale income; got %+v", sale.Total, voidMovement)
	}
	if voidMovement.VoidsMovementID == nil || *voidMovement.VoidsMovementID != sale.CashMovementID {
		t.Errorf("want the void to point at the sale's own income movement; got %+v", voidMovement.VoidsMovementID)
	}
}

// TestIntegration_SecondVoidIsRefused pins ErrAlreadyVoided.
func TestIntegration_SecondVoidIsRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Double Void Item", 500, true)
	openSession(t, f, 5000)

	sale, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := f.Stores.Sales.Void(ctx, f.ComplexID, sale.ID, f.UserID, nil); err != nil {
		t.Fatalf("first Void: %v", err)
	}

	_, err = f.Stores.Sales.Void(ctx, f.ComplexID, sale.ID, f.UserID, nil)
	if !errors.Is(err, salestore.ErrAlreadyVoided) {
		t.Fatalf("want ErrAlreadyVoided on the second void; got %v", err)
	}
}

// TestIntegration_VoidRefusesWithNoOpenSession pins that a void, like a sale,
// needs an open session to land its own cash movement in.
func TestIntegration_VoidRefusesWithNoOpenSession(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "No Session Void Item", 500, true)
	session := openSession(t, f, 5000)

	sale, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 5000, 0, time.Now(), nil); err != nil {
		t.Fatalf("closing session: %v", err)
	}

	_, err = f.Stores.Sales.Void(ctx, f.ComplexID, sale.ID, f.UserID, nil)
	if !errors.Is(err, salestore.ErrNoOpenCashSession) {
		t.Fatalf("want ErrNoOpenCashSession; got %v", err)
	}
}

// TestIntegration_SaleTotalCheckRefusesAZeroOrNegativeRow pins
// sales_total_check via a raw insert bypassing the store's own validation.
func TestIntegration_SaleTotalCheckRefusesAZeroOrNegativeRow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 5000)
	income := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "sale", Method: "cash", Amount: 100, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, income); err != nil {
		t.Fatalf("inserting income movement: %v", err)
	}

	_, err := f.DB.Exec(context.Background(),
		`INSERT INTO sales (complex_id, session_id, method, total, cash_movement_id, created_by)
		 VALUES ($1, $2, 'cash', 0, $3, $4)`,
		f.ComplexID, session.ID, income.ID, f.UserID)
	wantPgError(t, err, checkViolation, "sales_total_check")
}

// TestIntegration_SaleItemLineTotalCheckRefusesAMismatch pins
// sale_items_line_total_check via a raw insert.
func TestIntegration_SaleItemLineTotalCheckRefusesAMismatch(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Line Total Check Item", 500, true)
	session := openSession(t, f, 5000)
	income := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "sale", Method: "cash", Amount: 1000, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, income); err != nil {
		t.Fatalf("inserting income movement: %v", err)
	}

	var saleID uuid.UUID
	if err := f.DB.QueryRow(context.Background(),
		`INSERT INTO sales (complex_id, session_id, method, total, cash_movement_id, created_by)
		 VALUES ($1, $2, 'cash', 1000, $3, $4) RETURNING id`,
		f.ComplexID, session.ID, income.ID, f.UserID).Scan(&saleID); err != nil {
		t.Fatalf("inserting sale: %v", err)
	}

	_, err := f.DB.Exec(context.Background(),
		`INSERT INTO sale_items (complex_id, sale_id, product_id, product_name, unit_price, quantity, line_total)
		 VALUES ($1, $2, $3, 'Line Total Check Item', 500, 2, 999)`,
		f.ComplexID, saleID, product.ID)
	wantPgError(t, err, checkViolation, "sale_items_line_total_check")
}

// TestIntegration_SaleItemDuplicateProductRefused pins
// sale_items_sale_id_product_id_key.
func TestIntegration_SaleItemDuplicateProductRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Duplicate Line Item", 500, true)
	openSession(t, f, 5000)

	sale, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = f.DB.Exec(context.Background(),
		`INSERT INTO sale_items (complex_id, sale_id, product_id, product_name, unit_price, quantity, line_total)
		 VALUES ($1, $2, $3, 'Duplicate Line Item', 500, 1, 500)`,
		f.ComplexID, sale.ID, product.ID)
	wantPgError(t, err, "23505", "sale_items_sale_id_product_id_key")
}

// TestIntegration_VoidedSaleIsImmutable pins sales_forbid_update_after_void:
// even a raw UPDATE against an already-voided sale is refused.
func TestIntegration_VoidedSaleIsImmutable(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Immutable After Void", 500, true)
	openSession(t, f, 5000)

	sale, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := f.Stores.Sales.Void(ctx, f.ComplexID, sale.ID, f.UserID, nil); err != nil {
		t.Fatalf("Void: %v", err)
	}

	_, err = f.DB.Exec(context.Background(), `UPDATE sales SET total = 999 WHERE id = $1`, sale.ID)
	wantPgError(t, err, checkViolation, "sales_voided_is_immutable")
}
