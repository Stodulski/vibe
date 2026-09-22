// Package store implements the products domain's persistence: the catalog
// (products) and its append-only stock ledger (stock_movements), against
// PostgreSQL.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Domain sentinels this package raises, alongside the shared ones in
// internal/data (ErrRecordNotFound in particular: a product looked up by id
// and not found, or not this tenant's, answers that one).
var (
	// ErrDuplicateProductName reports a name collision with another ACTIVE
	// product of the same complex — idx_products_active_name_unique
	// (db/migrations/004_pos_catalog_stock.sql) is what actually enforces it.
	ErrDuplicateProductName = errors.New("product: a product with this name already exists")
	// ErrNoOpenCashSession reports that a restock was attempted with no cash
	// session open — a restock writes a cash expense movement, and there is
	// nowhere for it to land.
	ErrNoOpenCashSession = errors.New("product: no cash session is open")
	// ErrProductNotTrackingStock reports that a restock or an adjustment was
	// attempted against a product whose tracks_stock is false.
	ErrProductNotTrackingStock = errors.New("product: this product does not track stock")
	// ErrProductInactive reports that a restock was attempted against a
	// deactivated product.
	ErrProductInactive = errors.New("product: this product is not active")
)

// productsActiveNameUnique is the constraint name from
// db/migrations/004_pos_catalog_stock.sql, matched by name for the same
// reason courts/store's translateCourtWrite is: a name collision is the only
// write refusal this table can produce that the caller can fix.
const productsActiveNameUnique = "idx_products_active_name_unique"

// translateProductWrite maps the constraint refusals a products write can
// produce onto this package's sentinels. Anything else is returned
// unchanged — an unrecognised constraint failure is a genuine fault.
func translateProductWrite(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.ConstraintName == productsActiveNameUnique {
		return ErrDuplicateProductName
	}
	return err
}

// Product is one catalog entry a complex sells at the counter.
type Product struct {
	ID                uuid.UUID
	ComplexID         uuid.UUID
	Name              string
	Category          *string
	Price             int
	TracksStock       bool
	StockOnHand       int
	LowStockThreshold *int
	Active            bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	// Version is the optimistic-concurrency counter bumped by
	// trigger_bump_version (db/migrations/001_init.sql, reused rather than
	// redefined) on every UPDATE — the same convention courts.Court.Version
	// follows.
	Version int
}

// LowStock reports whether this product is at or below its own threshold —
// meaningless (and always false) for a product that does not track stock or
// carries no threshold.
func (p *Product) LowStock() bool {
	return p.TracksStock && p.LowStockThreshold != nil && p.StockOnHand <= *p.LowStockThreshold
}

// NeedsStockReview reports whether this product oversold past zero — allowed
// in this MVP (a sale never refuses for lack of stock, T4b), but flagged so
// an owner notices and counts the drawer.
func (p *Product) NeedsStockReview() bool {
	return p.StockOnHand < 0
}

// StockMovement is one append-only entry against a product's stock ledger.
type StockMovement struct {
	ID             uuid.UUID
	ComplexID      uuid.UUID
	ProductID      uuid.UUID
	Kind           string
	Quantity       int
	Reason         *string
	Note           *string
	CashMovementID *uuid.UUID
	SaleID         *uuid.UUID
	CreatedAt      time.Time
	CreatedBy      uuid.UUID
}

// CursorKey implements data.CursorKeyer for the stock-movement history's
// pagination.
func (m *StockMovement) CursorKey() (time.Time, uuid.UUID) { return m.CreatedAt, m.ID }

// Store implements the products domain's persistence against PostgreSQL.
type Store struct {
	DB *data.DB
	Q  *db.Queries
}

// Insert creates a new product and populates p with its generated id and
// timestamps.
func (m *Store) Insert(ctx context.Context, p *Product) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return err
	}

	row, err := m.Q.InsertProduct(ctx, db.InsertProductParams{
		ComplexID: data.UUIDToPg(p.ComplexID),
		Name:      p.Name,
		Category:  data.TextToPg(p.Category),
		//nolint:gosec // G115: price is a validated currency amount capped by the handler, far below int32 range.
		Price:             int32(p.Price),
		TracksStock:       p.TracksStock,
		LowStockThreshold: data.Int4PtrToPg(p.LowStockThreshold),
	})
	if err != nil {
		return translateProductWrite(err)
	}

	*p = *productFromDB(row)
	return nil
}

// GetByID returns one product of this complex, or ErrRecordNotFound.
func (m *Store) GetByID(ctx context.Context, complexID, productID uuid.UUID) (*Product, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetProductByID(ctx, db.GetProductByIDParams{
		ID:        data.UUIDToPg(productID),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return productFromDB(row), nil
}

// ListByComplex returns every product of a complex, alphabetically by name —
// unpaginated, the same convention courtstore.Store.GetByComplex follows for
// a catalog bounded by what one shop actually stocks. activeFilter nil
// returns every product regardless of state; non-nil restricts to that
// active value.
func (m *Store) ListByComplex(ctx context.Context, complexID uuid.UUID, activeFilter *bool) ([]*Product, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListProductsByComplex(ctx, db.ListProductsByComplexParams{
		ComplexID:       data.UUIDToPg(complexID),
		HasActiveFilter: activeFilter != nil,
		ActiveFilter:    activeFilter != nil && *activeFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("product: list by complex: %w", err)
	}

	products := make([]*Product, len(rows))
	for i, r := range rows {
		products[i] = productFromDB(r)
	}
	return products, nil
}

// Update persists a partial change to an existing product, returning
// data.ErrRecordNotFound if it no longer exists or the expected version no
// longer matches — the caller (products.Service.Update, which already read
// the product before assembling p) is what tells those two apart, the same
// way courts.Service.Update does for courtstore.Store.Update.
func (m *Store) Update(ctx context.Context, p *Product, expectedVersion *int) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return err
	}

	row, err := m.Q.UpdateProduct(ctx, db.UpdateProductParams{
		Name:     p.Name,
		Category: data.TextToPg(p.Category),
		//nolint:gosec // G115: price is a validated currency amount capped by the handler, far below int32 range.
		Price:             int32(p.Price),
		LowStockThreshold: data.Int4PtrToPg(p.LowStockThreshold),
		Active:            p.Active,
		TracksStock:       p.TracksStock,
		ID:                data.UUIDToPg(p.ID),
		ComplexID:         data.UUIDToPg(p.ComplexID),
		ExpectedVersion:   data.Int4PtrToPg(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return translateProductWrite(err)
	}

	*p = *productFromDB(row)
	return nil
}

// Restock records a stock delivery: in ONE transaction, it locks the
// complex's open cash session and the product row, writes the cash expense
// movement (amount = totalCost, method, a note naming the product and
// quantity), writes the linked stock movement, and applies quantity to the
// product's stock_on_hand.
//
// This is the one store method that writes to cash_movements — a table
// internal/cashbox's own store owns — rather than staying inside its own
// domain's tables. It calls the sqlc queries db/queries/cashbox.sql already
// generates (GetOpenCashSessionForUpdate, InsertCashMovement) directly
// through the shared *db.Queries, the same way internal/db is shared
// infrastructure for every other domain: the restock and its cash entry must
// commit or fail together, in one transaction, and neither
// internal/cashbox's nor internal/products' own store can see the other's
// transaction from the outside. Domain ownership stays at the Go package
// level (internal/products never imports internal/cashbox), not at the
// generated-query level, which has no domain boundaries of its own.
//
// ErrNoOpenCashSession answers a complex with nothing open.
// ErrProductInactive / ErrProductNotTrackingStock answer a product this
// cannot be recorded against, checked under the row lock so a concurrent
// deactivation cannot race it.
//
//nolint:funlen // one cohesive transaction (lock session, lock product, write the cash expense, write the stock movement, apply the delta) — splitting it would relocate sequential steps of the same atomic write into helpers without reducing what a reader holds at once, the same cohesion argument courts.Handler.Update and products.Handler.Update make.
func (m *Store) Restock(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity, totalCost int, method string, note *string) (*Product, *StockMovement, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, complexID); err != nil {
		return nil, nil, err
	}

	var product *Product
	var movement *StockMovement
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		session, err := qtx.GetOpenCashSessionForUpdate(ctx, data.UUIDToPg(complexID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNoOpenCashSession
			}
			return fmt.Errorf("product restock: lock open cash session: %w", err)
		}

		lockedProduct, err := qtx.GetProductByIDForUpdate(ctx, db.GetProductByIDForUpdateParams{
			ID:        data.UUIDToPg(productID),
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("product restock: lock product: %w", err)
		}
		if !lockedProduct.Active {
			return ErrProductInactive
		}
		if !lockedProduct.TracksStock {
			return ErrProductNotTrackingStock
		}

		// The cash expense's own note always names the product and quantity —
		// the feature document's own requirement — with the caller's optional
		// note appended, rather than replaced by it: a cashbox reconciliation
		// reading this movement in isolation (it has no product_id of its
		// own; stock_movements.cash_movement_id is the only link, and it
		// points the other way) still has to be able to tell what was bought.
		description := fmt.Sprintf("Restock: %s x%d", lockedProduct.Name, quantity)
		if note != nil && *note != "" {
			description = description + " — " + *note
		}

		cashRow, err := qtx.InsertCashMovement(ctx, db.InsertCashMovementParams{
			ComplexID: data.UUIDToPg(complexID),
			SessionID: session.ID,
			Kind:      "expense",
			Category:  "restock",
			Method:    db.PaymentMethod(method),
			//nolint:gosec // G115: totalCost is a validated currency amount checked positive and capped by the handler, far below int32 range.
			Amount:    int32(totalCost),
			Note:      data.TextToPg(&description),
			CreatedBy: data.UUIDToPg(actorID),
		})
		if err != nil {
			return fmt.Errorf("product restock: insert cash movement: %w", err)
		}

		movementRow, err := qtx.InsertStockMovement(ctx, db.InsertStockMovementParams{
			ComplexID: data.UUIDToPg(complexID),
			ProductID: data.UUIDToPg(productID),
			Kind:      "restock",
			//nolint:gosec // G115: quantity is validated positive and capped by the handler, far below int32 range.
			Quantity:       int32(quantity),
			CashMovementID: cashRow.ID,
			CreatedBy:      data.UUIDToPg(actorID),
		})
		if err != nil {
			return fmt.Errorf("product restock: insert stock movement: %w", err)
		}

		updatedProduct, err := qtx.UpdateProductStock(ctx, db.UpdateProductStockParams{
			ID:        data.UUIDToPg(productID),
			ComplexID: data.UUIDToPg(complexID),
			//nolint:gosec // G115: same bound as Quantity above.
			Delta: int32(quantity),
		})
		if err != nil {
			return fmt.Errorf("product restock: apply stock delta: %w", err)
		}

		product = productFromDB(updatedProduct)
		movement = stockMovementFromDB(movementRow)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return product, movement, nil
}

// Adjust records a stock correction: in ONE transaction, it locks the
// product row, writes the stock movement (reason required, no money), and
// applies quantity (signed, may be negative) to stock_on_hand.
//
// ErrProductNotTrackingStock answers a product whose tracks_stock is false,
// checked under the row lock for the same reason Restock checks it there.
// Unlike Restock, an inactive product is not refused: an owner correcting the
// count of a product they are about to deactivate (or have just deactivated)
// is exactly the "count correction" this exists for.
func (m *Store) Adjust(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity int, reason string, note *string) (*Product, *StockMovement, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, complexID); err != nil {
		return nil, nil, err
	}

	var product *Product
	var movement *StockMovement
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		lockedProduct, err := qtx.GetProductByIDForUpdate(ctx, db.GetProductByIDForUpdateParams{
			ID:        data.UUIDToPg(productID),
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("product adjustment: lock product: %w", err)
		}
		if !lockedProduct.TracksStock {
			return ErrProductNotTrackingStock
		}

		reasonCopy := reason
		movementRow, err := qtx.InsertStockMovement(ctx, db.InsertStockMovementParams{
			ComplexID: data.UUIDToPg(complexID),
			ProductID: data.UUIDToPg(productID),
			Kind:      "adjustment",
			//nolint:gosec // G115: quantity is validated non-zero and bounded by the handler, far below int32 range.
			Quantity:  int32(quantity),
			Reason:    data.TextToPg(&reasonCopy),
			Note:      data.TextToPg(note),
			CreatedBy: data.UUIDToPg(actorID),
		})
		if err != nil {
			return fmt.Errorf("product adjustment: insert stock movement: %w", err)
		}

		updatedProduct, err := qtx.UpdateProductStock(ctx, db.UpdateProductStockParams{
			ID:        data.UUIDToPg(productID),
			ComplexID: data.UUIDToPg(complexID),
			//nolint:gosec // G115: same bound as Quantity above.
			Delta: int32(quantity),
		})
		if err != nil {
			return fmt.Errorf("product adjustment: apply stock delta: %w", err)
		}

		product = productFromDB(updatedProduct)
		movement = stockMovementFromDB(movementRow)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return product, movement, nil
}

// ListStockMovements returns a page of a product's stock-movement history,
// newest first.
func (m *Store) ListStockMovements(ctx context.Context, complexID, productID uuid.UUID, filters data.Filters) ([]*StockMovement, data.Metadata, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, data.Metadata{}, err
	}
	hasCursor := !cursorTime.IsZero()

	//nolint:gosec // G115: filters.Limit is validated by data.ValidateFilters (1-200) before this is ever called.
	fetchLimit := int32(filters.Limit + 1)

	rows, err := m.Q.ListStockMovementsByProduct(ctx, db.ListStockMovementsByProductParams{
		ProductID:       data.UUIDToPg(productID),
		ComplexID:       data.UUIDToPg(complexID),
		HasCursor:       hasCursor,
		CursorCreatedAt: data.TimeToPg(cursorTime),
		CursorID:        data.UUIDToPg(cursorID),
		PageLimit:       fetchLimit,
	})
	if err != nil {
		return nil, data.Metadata{}, fmt.Errorf("stock movement: list by product: %w", err)
	}

	movements := make([]*StockMovement, len(rows))
	for i, r := range rows {
		movements[i] = stockMovementFromDB(r)
	}

	movements, meta := data.TrimPage(movements, filters.Limit, data.BuildTimestampCursor)
	return movements, meta, nil
}

func productFromDB(p db.Product) *Product {
	return &Product{
		ID:                data.PgToUUID(p.ID),
		ComplexID:         data.PgToUUID(p.ComplexID),
		Name:              p.Name,
		Category:          data.PgToTextPtr(p.Category),
		Price:             int(p.Price),
		TracksStock:       p.TracksStock,
		StockOnHand:       int(p.StockOnHand),
		LowStockThreshold: data.PgToInt4Ptr(p.LowStockThreshold),
		Active:            p.Active,
		CreatedAt:         data.PgToTime(p.CreatedAt),
		UpdatedAt:         data.PgToTime(p.UpdatedAt),
		Version:           int(p.Version),
	}
}

func stockMovementFromDB(m db.StockMovement) *StockMovement {
	return &StockMovement{
		ID:             data.PgToUUID(m.ID),
		ComplexID:      data.PgToUUID(m.ComplexID),
		ProductID:      data.PgToUUID(m.ProductID),
		Kind:           m.Kind,
		Quantity:       int(m.Quantity),
		Reason:         data.PgToTextPtr(m.Reason),
		Note:           data.PgToTextPtr(m.Note),
		CashMovementID: data.PgToUUIDPtr(m.CashMovementID),
		SaleID:         data.PgToUUIDPtr(m.SaleID),
		CreatedAt:      data.PgToTime(m.CreatedAt),
		CreatedBy:      data.PgToUUID(m.CreatedBy),
	}
}
