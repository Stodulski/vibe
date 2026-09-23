//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// insertProduct and openCashSession are the same one-line setup steps
// internal/sales/store's own integration suite uses; duplicated rather than
// shared because the two packages' _test packages have no common test-helper
// import between them.
func insertProduct(t *testing.T, f *datatest.Fixture, name string, price int, tracksStock bool) *productstore.Product {
	t.Helper()
	p := &productstore.Product{ComplexID: f.ComplexID, Name: name, Price: price, TracksStock: tracksStock}
	if err := f.Stores.Products.Insert(f.Scoped(context.Background()), p); err != nil {
		t.Fatalf("inserting product: %v", err)
	}
	return p
}

func openCashSession(t *testing.T, f *datatest.Fixture) *cashboxstore.CashSession {
	t.Helper()
	session := &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: 0}
	if err := f.Stores.Cashbox.OpenSession(f.Scoped(context.Background()), session); err != nil {
		t.Fatalf("opening cash session: %v", err)
	}
	return session
}

// backdate moves a row's created_at directly, the same raw-SQL shape
// reports_window_integration_test.go's own backdate helper uses, since none
// of the domain stores expose a write path for it (every created_at here is
// DB-default NOW()).
func backdate(t *testing.T, f *datatest.Fixture, table, id string, at time.Time) {
	t.Helper()
	if _, err := f.DB.Exec(context.Background(), `UPDATE `+table+` SET created_at = $2 WHERE id = $1`, id, at); err != nil {
		t.Fatalf("backdating %s %s: %v", table, id, err)
	}
}

// TestIntegration_CashSalesByMethodExcludesVoidedSales pins the "Caja"
// section's sales block: a voided sale must never appear as income, in any
// method's total or count.
func TestIntegration_CashSalesByMethodExcludesVoidedSales(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Coca Cola 500ml", 1_000, false)
	openCashSession(t, f)

	kept, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 2}}, "cash", nil)
	if err != nil {
		t.Fatalf("creating the kept sale: %v", err)
	}
	voided, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 3}}, "debit_card", nil)
	if err != nil {
		t.Fatalf("creating the sale to void: %v", err)
	}
	if _, err := f.Stores.Sales.Void(ctx, f.ComplexID, voided.ID, f.UserID, nil); err != nil {
		t.Fatalf("voiding the sale: %v", err)
	}

	now := time.Now()
	summaries, err := f.Stores.Reports.CashSalesByMethod(ctx, f.ComplexID, now.AddDate(0, 0, -1), now.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("CashSalesByMethod: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("want exactly one method summary (the kept cash sale, not the voided debit_card one); got %d: %+v",
			len(summaries), summaries)
	}
	got := summaries[0]
	if got.Method != "cash" || got.Count != 1 || got.Total != kept.Total {
		t.Errorf("want one cash sale totalling %d; got %+v", kept.Total, got)
	}
}

// TestIntegration_CashMovementsByCategoryExcludesVoidsAndDoesNotDoubleCountSales
// pins the rest of the "Caja" section: a voided movement and the void row
// that corrects it are both excluded, a restock lands as an expense, and the
// system 'sale' category never appears (that money is reported by
// CashSalesByMethod, by payment method, not here).
func TestIntegration_CashMovementsByCategoryExcludesVoidsAndDoesNotDoubleCountSales(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Paletas", 5_000, true)
	session := openCashSession(t, f)

	// A sale — its income movement is category 'sale' and must never appear
	// in the categories block.
	if _, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil); err != nil {
		t.Fatalf("creating the sale: %v", err)
	}

	// A restock — category 'restock', an expense.
	if _, _, err := f.Stores.Products.Restock(ctx, f.ComplexID, product.ID, f.UserID, 10, 2_000, "cash", nil); err != nil {
		t.Fatalf("restocking: %v", err)
	}

	// A manual income, kept.
	kept := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income", Category: "other_income",
		Method: "cash", Amount: 1_500, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, kept); err != nil {
		t.Fatalf("inserting the kept movement: %v", err)
	}

	// A manual expense that is voided — original and void must both be
	// excluded from the categories block.
	original := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense", Category: "supplies",
		Method: "cash", Amount: 800, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, original); err != nil {
		t.Fatalf("inserting the movement to void: %v", err)
	}
	void := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income", Category: "supplies",
		Method: "cash", Amount: 800, VoidsMovementID: &original.ID, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, void); err != nil {
		t.Fatalf("inserting the void: %v", err)
	}

	now := time.Now()
	summaries, err := f.Stores.Reports.CashMovementsByCategory(ctx, f.ComplexID, now.AddDate(0, 0, -1), now.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("CashMovementsByCategory: %v", err)
	}

	byCategory := map[string]reportstore.CashCategorySummary{}
	for _, s := range summaries {
		byCategory[s.Category] = s
	}

	if _, ok := byCategory["sale"]; ok {
		t.Errorf("want no 'sale' row (reported by CashSalesByMethod instead); got %+v", summaries)
	}
	if _, ok := byCategory["supplies"]; ok {
		t.Errorf("want the voided 'supplies' movement and its void both excluded; got %+v", summaries)
	}
	restock, ok := byCategory["restock"]
	if !ok || restock.Kind != "expense" || restock.Total != 2_000 {
		t.Errorf("want a 2000 restock expense; got %+v (present=%v)", restock, ok)
	}
	income, ok := byCategory["other_income"]
	if !ok || income.Kind != "income" || income.Total != 1_500 {
		t.Errorf("want a 1500 other_income row; got %+v (present=%v)", income, ok)
	}
}

// TestIntegration_CashReportsUseTheVenueTimeZoneMonthBoundary pins the same
// month-boundary semantics PaymentSummaryByMethod already has: a sale or
// movement just past local midnight in Argentina belongs to the next
// calendar day even when its UTC timestamp still reads as the day before.
func TestIntegration_CashReportsUseTheVenueTimeZoneMonthBoundary(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	product := insertProduct(t, f, "Agua", 500, false)
	session := openCashSession(t, f)

	sale, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, "cash", nil)
	if err != nil {
		t.Fatalf("creating the sale: %v", err)
	}
	movement := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense", Category: "cleaning",
		Method: "cash", Amount: 300, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, movement); err != nil {
		t.Fatalf("inserting the movement: %v", err)
	}

	// 2026-03-01 00:30 in Buenos Aires (UTC-3) is 2026-02-28 03:30 UTC — a
	// naive UTC-date comparison would place both rows in February.
	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatalf("loading the venue time zone: %v", err)
	}
	justAfterMidnight := time.Date(2026, time.March, 1, 0, 30, 0, 0, loc)
	backdate(t, f, "sales", sale.ID.String(), justAfterMidnight)
	backdate(t, f, "cash_movements", movement.ID.String(), justAfterMidnight)

	march := time.Date(2026, time.March, 1, 0, 0, 0, 0, loc)
	marchEnd := march.AddDate(0, 1, -1)
	february := march.AddDate(0, -1, 0)
	februaryEnd := february.AddDate(0, 1, -1)

	salesInMarch, err := f.Stores.Reports.CashSalesByMethod(ctx, f.ComplexID, march, marchEnd)
	if err != nil {
		t.Fatalf("CashSalesByMethod(March): %v", err)
	}
	if len(salesInMarch) != 1 || salesInMarch[0].Count != 1 {
		t.Fatalf("want the sale counted in March (venue-local date); got %+v", salesInMarch)
	}

	salesInFebruary, err := f.Stores.Reports.CashSalesByMethod(ctx, f.ComplexID, february, februaryEnd)
	if err != nil {
		t.Fatalf("CashSalesByMethod(February): %v", err)
	}
	if len(salesInFebruary) != 0 {
		t.Fatalf("want no sale counted in February (its UTC timestamp is a distraction); got %+v", salesInFebruary)
	}

	categoriesInMarch, err := f.Stores.Reports.CashMovementsByCategory(ctx, f.ComplexID, march, marchEnd)
	if err != nil {
		t.Fatalf("CashMovementsByCategory(March): %v", err)
	}
	if len(categoriesInMarch) != 1 || categoriesInMarch[0].Category != "cleaning" {
		t.Fatalf("want the cleaning expense counted in March; got %+v", categoriesInMarch)
	}
}
