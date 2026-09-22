//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
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

func insertProduct(t *testing.T, f *datatest.Fixture, name string, tracksStock bool) *productstore.Product {
	t.Helper()
	p := &productstore.Product{ComplexID: f.ComplexID, Name: name, Price: 500, TracksStock: tracksStock}
	if err := f.Stores.Products.Insert(f.Scoped(context.Background()), p); err != nil {
		t.Fatalf("inserting product: %v", err)
	}
	return p
}

// TestIntegration_RestockMovesStockAndCashAtomically pins the whole point of
// Store.Restock: one transaction writes the cash expense, the stock
// movement, and the product's new stock_on_hand together.
func TestIntegration_RestockMovesStockAndCashAtomically(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Coca Cola 500ml", true)
	session := &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: 10000}
	if err := f.Stores.Cashbox.OpenSession(ctx, session); err != nil {
		t.Fatalf("opening cash session: %v", err)
	}

	updated, movement, err := f.Stores.Products.Restock(ctx, f.ComplexID, product.ID, f.UserID, 24, 12000, "cash", nil)
	if err != nil {
		t.Fatalf("Restock: %v", err)
	}
	if updated.StockOnHand != 24 {
		t.Errorf("want stock_on_hand=24; got %d", updated.StockOnHand)
	}
	if movement.Kind != "restock" || movement.Quantity != 24 {
		t.Errorf("want a restock movement of quantity 24; got %+v", movement)
	}
	if movement.CashMovementID == nil {
		t.Fatal("want the stock movement linked to the cash expense it created")
	}

	cashMovement, err := f.Stores.Cashbox.GetMovementByID(ctx, f.ComplexID, *movement.CashMovementID)
	if err != nil {
		t.Fatalf("reading the linked cash movement: %v", err)
	}
	if cashMovement.Kind != "expense" || cashMovement.Category != "restock" || cashMovement.Amount != 12000 || cashMovement.Method != "cash" {
		t.Errorf("want a 12000 cash restock expense; got %+v", cashMovement)
	}

	// The cashbox's own reconciliation must see the expense: expected cash
	// drops by the restock's cost when its method is cash.
	totals, err := f.Stores.Cashbox.SumBySession(ctx, f.ComplexID, session.ID)
	if err != nil {
		t.Fatalf("summing session movements: %v", err)
	}
	_, cashExpense := cashboxstore.CashIncomeAndExpense(totals)
	if cashExpense != 12000 {
		t.Errorf("want cash expense 12000 from the restock; got %d", cashExpense)
	}
}

// TestIntegration_RestockRefusesWithNoOpenSession pins ErrNoOpenCashSession.
func TestIntegration_RestockRefusesWithNoOpenSession(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "No Session Soda", true)

	_, _, err := f.Stores.Products.Restock(ctx, f.ComplexID, product.ID, f.UserID, 10, 5000, "cash", nil)
	if !errors.Is(err, productstore.ErrNoOpenCashSession) {
		t.Fatalf("want ErrNoOpenCashSession; got %v", err)
	}
}

// TestIntegration_RestockRefusesAProductThatDoesNotTrackStock pins
// ErrProductNotTrackingStock under the row lock.
func TestIntegration_RestockRefusesAProductThatDoesNotTrackStock(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Untracked Snack", false)
	if err := f.Stores.Cashbox.OpenSession(ctx, &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: 1000}); err != nil {
		t.Fatalf("opening cash session: %v", err)
	}

	_, _, err := f.Stores.Products.Restock(ctx, f.ComplexID, product.ID, f.UserID, 10, 5000, "cash", nil)
	if !errors.Is(err, productstore.ErrProductNotTrackingStock) {
		t.Fatalf("want ErrProductNotTrackingStock; got %v", err)
	}
}

// TestIntegration_AdjustmentMovesStockWithoutMoney pins Store.Adjust: no cash
// session is required, and no cash_movements row is ever written.
func TestIntegration_AdjustmentMovesStockWithoutMoney(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Breakage Item", true)

	updated, movement, err := f.Stores.Products.Adjust(ctx, f.ComplexID, product.ID, f.UserID, -3, "breakage", nil)
	if err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if updated.StockOnHand != -3 {
		t.Errorf("want stock_on_hand=-3; got %d", updated.StockOnHand)
	}
	if movement.Kind != "adjustment" || movement.CashMovementID != nil {
		t.Errorf("want an adjustment movement with no linked cash movement; got %+v", movement)
	}
	if movement.Reason == nil || *movement.Reason != "breakage" {
		t.Errorf("want reason=breakage; got %+v", movement.Reason)
	}
}

// TestIntegration_StockCanGoNegativeThroughAnAdjustment pins the owner
// decision that selling (and, here, adjusting) past zero is allowed.
func TestIntegration_StockCanGoNegativeThroughAnAdjustment(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Oversold Item", true)

	updated, _, err := f.Stores.Products.Adjust(ctx, f.ComplexID, product.ID, f.UserID, -10, "count_correction", nil)
	if err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if updated.StockOnHand != -10 {
		t.Fatalf("want stock_on_hand=-10 (negative stock allowed); got %d", updated.StockOnHand)
	}
	if !updated.NeedsStockReview() {
		t.Error("want NeedsStockReview() true for negative stock")
	}
}

// TestIntegration_AdjustmentRefusesAProductThatDoesNotTrackStock pins
// ErrProductNotTrackingStock for Adjust too.
func TestIntegration_AdjustmentRefusesAProductThatDoesNotTrackStock(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Untracked Adjustment", false)

	_, _, err := f.Stores.Products.Adjust(ctx, f.ComplexID, product.ID, f.UserID, 5, "other", nil)
	if !errors.Is(err, productstore.ErrProductNotTrackingStock) {
		t.Fatalf("want ErrProductNotTrackingStock; got %v", err)
	}
}

// TestIntegration_ActiveNameUniqueIndexRefusesADuplicate pins
// idx_products_active_name_unique, case-folded (lower(name)).
func TestIntegration_ActiveNameUniqueIndexRefusesADuplicate(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	insertProduct(t, f, "Agua Mineral", true)

	dup := &productstore.Product{ComplexID: f.ComplexID, Name: "agua mineral", Price: 300, TracksStock: true}
	err := f.Stores.Products.Insert(ctx, dup)
	if !errors.Is(err, productstore.ErrDuplicateProductName) {
		t.Fatalf("want ErrDuplicateProductName for a case-insensitive collision; got %v", err)
	}
}

// TestIntegration_DeactivatedProductFreesItsName pins the partial index's
// WHERE active clause: a deactivated product's name becomes free again.
func TestIntegration_DeactivatedProductFreesItsName(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	original := insertProduct(t, f, "Toallon", true)
	original.Active = false
	if err := f.Stores.Products.Update(ctx, original, nil); err != nil {
		t.Fatalf("deactivating: %v", err)
	}

	reused := &productstore.Product{ComplexID: f.ComplexID, Name: "Toallon", Price: 400, TracksStock: true}
	if err := f.Stores.Products.Insert(ctx, reused); err != nil {
		t.Fatalf("want reusing a deactivated product's name to succeed; got %v", err)
	}
}

// TestIntegration_QuantitySignMustMatchKind pins
// stock_movements_quantity_sign_check via a raw insert bypassing the store's
// own Go-side kind selection.
func TestIntegration_QuantitySignMustMatchKind(t *testing.T) {
	f := datatest.Isolated(t)
	product := insertProduct(t, f, "Sign Check Item", true)

	_, err := f.DB.Exec(context.Background(),
		`INSERT INTO stock_movements (complex_id, product_id, kind, quantity, created_by) VALUES ($1, $2, 'sale', 5, $3)`,
		f.ComplexID, product.ID, f.UserID)
	wantPgError(t, err, checkViolation, "stock_movements_quantity_sign_check")
}

// TestIntegration_AdjustmentReasonRequired pins
// stock_movements_reason_consistent for an adjustment sent with no reason.
func TestIntegration_AdjustmentReasonRequired(t *testing.T) {
	f := datatest.Isolated(t)
	product := insertProduct(t, f, "Reason Required Item", true)

	_, err := f.DB.Exec(context.Background(),
		`INSERT INTO stock_movements (complex_id, product_id, kind, quantity, created_by) VALUES ($1, $2, 'adjustment', -2, $3)`,
		f.ComplexID, product.ID, f.UserID)
	wantPgError(t, err, checkViolation, "stock_movements_reason_consistent")
}

// TestIntegration_RestockRequiresACashMovement pins
// stock_movements_cash_movement_consistent for a restock inserted with no
// linked cash_movement_id.
func TestIntegration_RestockRequiresACashMovement(t *testing.T) {
	f := datatest.Isolated(t)
	product := insertProduct(t, f, "No Cash Link Item", true)

	_, err := f.DB.Exec(context.Background(),
		`INSERT INTO stock_movements (complex_id, product_id, kind, quantity, created_by) VALUES ($1, $2, 'restock', 5, $3)`,
		f.ComplexID, product.ID, f.UserID)
	wantPgError(t, err, checkViolation, "stock_movements_cash_movement_consistent")
}

// TestIntegration_SystemCashCategoriesAcceptedAndConsistencyHolds pins the
// new 'sale'/'restock' cash_movements categories: a system row using them
// must be accepted, and the kind/category consistency check must still
// refuse a mismatch (e.g. 'restock' on an income row).
func TestIntegration_SystemCashCategoriesAcceptedAndConsistencyHolds(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: 1000}
	if err := f.Stores.Cashbox.OpenSession(ctx, session); err != nil {
		t.Fatalf("opening cash session: %v", err)
	}

	saleIncome := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "sale", Method: "cash", Amount: 1500, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, saleIncome); err != nil {
		t.Fatalf("want a 'sale' income row accepted; got %v", err)
	}

	restockExpense := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense",
		Category: "restock", Method: "cash", Amount: 800, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, restockExpense); err != nil {
		t.Fatalf("want a 'restock' expense row accepted; got %v", err)
	}

	_, err := f.DB.Exec(context.Background(),
		`INSERT INTO cash_movements (complex_id, session_id, kind, category, method, amount, created_by)
		 VALUES ($1, $2, 'income', 'restock', 'cash', 100, $3)`,
		f.ComplexID, session.ID, f.UserID)
	wantPgError(t, err, checkViolation, "cash_movements_category_kind_consistent")
}
