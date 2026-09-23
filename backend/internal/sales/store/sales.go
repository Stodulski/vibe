// Package store implements the sales domain's persistence: a sale, its line
// items, the cash income it created and the stock movements it drove — all
// in one transaction — against PostgreSQL.
package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// maxSaleTotal caps a sale's total, which becomes cash_movements.amount (an
// INTEGER column) through the sale's own linked income movement — the same
// bound internal/cashbox's own maxMovementAmount enforces there, and
// sales_total_check (db/migrations/005_pos_sales.sql) enforces at the
// database. Every individual line's own line_total is guaranteed to fit the
// same bound: unit_price and quantity are both non-negative, so no line_total
// can exceed a total that is itself checked against this cap before any row
// is written — see Create's own comment.
const maxSaleTotal = 2_000_000_000

// Domain sentinels this package raises, alongside the shared ones in
// internal/data (ErrRecordNotFound in particular: a sale looked up by id and
// not found, or not this tenant's, answers that one).
var (
	// ErrNoOpenCashSession reports that a sale (or a void) was attempted with
	// no cash session open — a sale writes a cash income movement, and a void
	// writes its own cash movement, and there is nowhere for either to land.
	ErrNoOpenCashSession = errors.New("sale: no cash session is open")
	// ErrZeroTotal reports a sale whose computed total is zero — every priced
	// item was free, so there is no income to record. Refused rather than
	// silently accepted: cash_movements.amount must be > 0, and a sale that
	// gives products away is a gift, not a sale (feature document decision).
	ErrZeroTotal = errors.New("sale: total is zero")
	// ErrTotalExceedsCap reports a sale whose computed total is over
	// maxSaleTotal.
	ErrTotalExceedsCap = errors.New("sale: total exceeds the maximum")
	// ErrAlreadyVoided reports that the sale being voided already carries a
	// voided_at (sales_forbid_update_after_void's Go-side name, checked
	// before that trigger would even fire).
	ErrAlreadyVoided = errors.New("sale: this sale has already been voided")
	// ErrIncomeAlreadyVoided reports that the sale's own income movement was
	// already voided by some other path (cash_movements.voids_movement_id is
	// UNIQUE) — see Void's own comment for why this should not be reachable
	// in ordinary operation, only as defense in depth.
	ErrIncomeAlreadyVoided = errors.New("sale: this sale's income movement has already been voided")
)

// ItemProblem names why one requested product id could not be sold.
type ItemProblem string

const (
	// ItemNotFound means the product id does not exist in this complex.
	ItemNotFound ItemProblem = "not_found"
	// ItemInactive means the product exists but is deactivated.
	ItemInactive ItemProblem = "inactive"
)

// ErrInvalidItems is returned when one or more requested items name a
// product that does not exist in this complex or is not active. Problems
// maps each offending product id to why, so the caller (the handler, which
// still has the original ordered item list) can report a field error naming
// the offending item's index — the store has no notion of "index", only of
// which product ids it could not sell.
type ErrInvalidItems struct {
	Problems map[uuid.UUID]ItemProblem
}

func (e *ErrInvalidItems) Error() string {
	return fmt.Sprintf("sale: %d item(s) reference a product that does not exist in this complex or is not active", len(e.Problems))
}

// Constraint names from db/migrations/005_pos_sales.sql / 003_cashbox.sql,
// matched by name for the same reason courts/store's translateCourtWrite is.
const (
	saleVoidedImmutable = "sales_voided_is_immutable"
	// voidsMovementUnique is cash_movements' own constraint name — see
	// cashboxstore's identical copy. Duplicated here (rather than imported)
	// because internal/sales writes cash_movements directly, through the
	// shared generated queries, the same way internal/products' own
	// Store.Restock does — see this file's own package comment.
	voidsMovementUnique = "cash_movements_voids_movement_id_key"
)

// Sale is one recorded sale: its items are loaded separately (ListItemsBySale
// / ListItemsBySaleIDs) rather than carried on this struct, the same
// separation cashboxstore.CashSession keeps from its own movements.
type Sale struct {
	ID                 uuid.UUID
	ComplexID          uuid.UUID
	SessionID          uuid.UUID
	Method             string
	Total              int
	CashMovementID     uuid.UUID
	VoidedAt           *time.Time
	VoidedBy           *uuid.UUID
	VoidCashMovementID *uuid.UUID
	CreatedAt          time.Time
	CreatedBy          uuid.UUID
}

// IsVoided reports whether this sale has been voided.
func (s *Sale) IsVoided() bool { return s.VoidedAt != nil }

// CursorKey implements data.CursorKeyer for the list endpoint's pagination.
func (s *Sale) CursorKey() (time.Time, uuid.UUID) { return s.CreatedAt, s.ID }

// SaleItem is one line of a sale: the product it named, and a snapshot of its
// name and price at the moment of the sale.
type SaleItem struct {
	ID          uuid.UUID
	ComplexID   uuid.UUID
	SaleID      uuid.UUID
	ProductID   uuid.UUID
	ProductName string
	UnitPrice   int
	Quantity    int
	LineTotal   int
	CreatedAt   time.Time
}

// ItemInput is one requested line: a product id and a quantity, exactly what
// a client sends — prices and totals always come from the server.
type ItemInput struct {
	ProductID uuid.UUID
	Quantity  int
}

// StockWarning names a tracked product whose stock_on_hand ended at zero or
// below after a sale — never a reason to refuse the sale (selling past zero
// stock is allowed, feature document decision), only a flag for the owner to
// notice and count the drawer.
type StockWarning struct {
	ProductID   uuid.UUID
	ProductName string
	StockOnHand int
}

// Store implements the sales domain's persistence against PostgreSQL.
type Store struct {
	DB *data.DB
	Q  *db.Queries
}

// Create records a sale in ONE transaction: it locks the complex's open cash
// session, locks every named product (in ascending id order, one
// single-row lock at a time — see below for why not a single batched
// query), validates that every product exists and is active, computes the
// total from the LOCKED products' own prices (never the caller's), writes
// the 'sale' cash income, the sale row, one sale_items row per requested
// product, and — for every product that tracks stock — one 'sale' stock
// movement and the stock_on_hand delta it drives.
//
// Locking is not restricted to stock-tracked products, unlike the feature
// document's own wording ("lock ... every stock-tracked product"): every
// requested product is locked here, tracked or not. An untracked product's
// price still has to be read for the total, and locking it too closes the
// same "price changed mid-sale" race for free, at a cost — one more row
// lock, released at commit, on a sale bounded to 50 lines — this codebase
// already treats as negligible everywhere else (Restock locks the one
// product it touches the same way). This is a superset of the minimum the
// document asks for, not a smaller guarantee.
//
// Products are locked ONE AT A TIME, via the existing single-row
// GetProductByIDForUpdate (products.sql), sorted by id first, rather than
// through one batched `WHERE id = ANY(...) FOR UPDATE` call: PostgreSQL does
// not guarantee that a single statement's row-level locks are acquired in
// its ORDER BY's output order — the sort can run above the locking step in
// the plan — so a batched call cannot actually promise the stable lock order
// two concurrent sales sharing a product need to avoid deadlocking each
// other. A sequence of single-row lock statements, issued in a fixed id
// order chosen in Go before any of them runs, gives that guarantee exactly:
// each statement is issued only after the previous one's lock is held.
//
// ErrNoOpenCashSession answers a complex with nothing open.
// *ErrInvalidItems answers a request naming a product that does not exist in
// this complex or is not active — see that type's own comment.
// ErrZeroTotal / ErrTotalExceedsCap answer a computed total outside (0, cap].
//
//nolint:funlen // one cohesive transaction (lock session, lock products, validate, price, write the cash income, the sale, its items, and every tracked item's stock movement) — splitting it would relocate sequential steps of the same atomic write into helpers without reducing what a reader holds at once, the same cohesion argument products.Store.Restock's identical comment makes.
func (m *Store) Create(ctx context.Context, complexID, actorID uuid.UUID, items []ItemInput, method string, note *string) (*Sale, []*SaleItem, []StockWarning, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, complexID); err != nil {
		return nil, nil, nil, err
	}

	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ProductID
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })

	var sale *Sale
	var saleItems []*SaleItem
	var warnings []StockWarning
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		session, err := qtx.GetOpenCashSessionForUpdate(ctx, data.UUIDToPg(complexID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNoOpenCashSession
			}
			return fmt.Errorf("sale: lock open cash session: %w", err)
		}

		locked := make(map[uuid.UUID]db.Product, len(ids))
		problems := map[uuid.UUID]ItemProblem{}
		for _, id := range ids {
			row, err := qtx.GetProductByIDForUpdate(ctx, db.GetProductByIDForUpdateParams{
				ID:        data.UUIDToPg(id),
				ComplexID: data.UUIDToPg(complexID),
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					problems[id] = ItemNotFound
					continue
				}
				return fmt.Errorf("sale: lock product %s: %w", id, err)
			}
			if !row.Active {
				problems[id] = ItemInactive
				continue
			}
			locked[id] = row
		}
		if len(problems) > 0 {
			return &ErrInvalidItems{Problems: problems}
		}

		total := 0
		for _, it := range items {
			total += int(locked[it.ProductID].Price) * it.Quantity
		}
		if total <= 0 {
			return ErrZeroTotal
		}
		if total > maxSaleTotal {
			return ErrTotalExceedsCap
		}

		description := fmt.Sprintf("Sale: %d item(s)", len(items))
		if note != nil && *note != "" {
			description = description + " — " + *note
		}

		//nolint:gosec // G115: total is checked positive and <= maxSaleTotal above, well within int32 range.
		cashRow, err := qtx.InsertCashMovement(ctx, db.InsertCashMovementParams{
			ComplexID: data.UUIDToPg(complexID),
			SessionID: session.ID,
			Kind:      "income",
			Category:  "sale",
			Method:    db.PaymentMethod(method),
			Amount:    int32(total),
			Note:      data.TextToPg(&description),
			CreatedBy: data.UUIDToPg(actorID),
		})
		if err != nil {
			return fmt.Errorf("sale: insert cash movement: %w", err)
		}

		saleRow, err := qtx.InsertSale(ctx, db.InsertSaleParams{
			ComplexID:      data.UUIDToPg(complexID),
			SessionID:      session.ID,
			Method:         db.PaymentMethod(method),
			Total:          int32(total), //nolint:gosec // G115: same bound as Amount above.
			CashMovementID: cashRow.ID,
			CreatedBy:      data.UUIDToPg(actorID),
		})
		if err != nil {
			return fmt.Errorf("sale: insert sale: %w", err)
		}
		sale = saleFromDB(saleRow)

		for _, it := range items {
			product := locked[it.ProductID]
			lineTotal := int(product.Price) * it.Quantity

			itemRow, err := qtx.InsertSaleItem(ctx, db.InsertSaleItemParams{
				ComplexID:   data.UUIDToPg(complexID),
				SaleID:      saleRow.ID,
				ProductID:   product.ID,
				ProductName: product.Name,
				UnitPrice:   product.Price,
				//nolint:gosec // G115: quantity is validated 1..1000 by the handler.
				Quantity: int32(it.Quantity),
				//nolint:gosec // G115: bounded by unit_price and quantity above, both well within range, and by the sale total cap.
				LineTotal: int32(lineTotal),
			})
			if err != nil {
				return fmt.Errorf("sale: insert sale item for product %s: %w", data.PgToUUID(product.ID), err)
			}
			saleItems = append(saleItems, saleItemFromDB(itemRow))

			if !product.TracksStock {
				continue
			}

			movementRow, err := qtx.InsertStockMovement(ctx, db.InsertStockMovementParams{
				ComplexID: data.UUIDToPg(complexID),
				ProductID: product.ID,
				Kind:      "sale",
				//nolint:gosec // G115: quantity is validated 1..1000 by the handler, negated here, still far below int32 range.
				Quantity:  int32(-it.Quantity),
				SaleID:    saleRow.ID,
				CreatedBy: data.UUIDToPg(actorID),
			})
			if err != nil {
				return fmt.Errorf("sale: insert stock movement for product %s: %w", data.PgToUUID(product.ID), err)
			}
			_ = movementRow

			updatedProduct, err := qtx.UpdateProductStock(ctx, db.UpdateProductStockParams{
				ID:        product.ID,
				ComplexID: data.UUIDToPg(complexID),
				//nolint:gosec // G115: same bound as Quantity above.
				Delta: int32(-it.Quantity),
			})
			if err != nil {
				return fmt.Errorf("sale: apply stock delta for product %s: %w", data.PgToUUID(product.ID), err)
			}
			if updatedProduct.StockOnHand <= 0 {
				warnings = append(warnings, StockWarning{
					ProductID:   data.PgToUUID(product.ID),
					ProductName: product.Name,
					StockOnHand: int(updatedProduct.StockOnHand),
				})
			}
		}
		return nil
	})
	if err != nil {
		var invalid *ErrInvalidItems
		if errors.As(err, &invalid) {
			return nil, nil, nil, invalid
		}
		return nil, nil, nil, err
	}
	return sale, saleItems, warnings, nil
}

// GetByID returns one sale of this complex, or ErrRecordNotFound.
func (m *Store) GetByID(ctx context.Context, complexID, saleID uuid.UUID) (*Sale, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetSaleByID(ctx, db.GetSaleByIDParams{
		ID:        data.UUIDToPg(saleID),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return saleFromDB(row), nil
}

// ListItemsBySale returns one sale's own line items, in insertion order.
func (m *Store) ListItemsBySale(ctx context.Context, complexID, saleID uuid.UUID) ([]*SaleItem, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListSaleItemsBySale(ctx, db.ListSaleItemsBySaleParams{
		SaleID:    data.UUIDToPg(saleID),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		return nil, fmt.Errorf("sale item: list by sale: %w", err)
	}

	items := make([]*SaleItem, len(rows))
	for i, r := range rows {
		items[i] = saleItemFromDB(r)
	}
	return items, nil
}

// ListItemsBySaleIDs is ListItemsBySale's batched counterpart, for a page of
// sales at once.
func (m *Store) ListItemsBySaleIDs(ctx context.Context, complexID uuid.UUID, saleIDs []uuid.UUID) ([]*SaleItem, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if len(saleIDs) == 0 {
		return nil, nil
	}

	rows, err := m.Q.ListSaleItemsBySaleIDs(ctx, db.ListSaleItemsBySaleIDsParams{
		SaleIds:   data.UUIDSliceToPg(saleIDs),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		return nil, fmt.Errorf("sale item: list by sale ids: %w", err)
	}

	items := make([]*SaleItem, len(rows))
	for i, r := range rows {
		items[i] = saleItemFromDB(r)
	}
	return items, nil
}

// ListByComplex returns a page of this complex's sales, newest first,
// optionally restricted to one session.
func (m *Store) ListByComplex(ctx context.Context, complexID uuid.UUID, sessionID *uuid.UUID, filters data.Filters) ([]*Sale, data.Metadata, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, data.Metadata{}, err
	}
	hasCursor := !cursorTime.IsZero()

	//nolint:gosec // G115: filters.Limit is validated by data.ValidateFilters (1-200) before this is ever called.
	fetchLimit := int32(filters.Limit + 1)

	rows, err := m.Q.ListSalesByComplex(ctx, db.ListSalesByComplexParams{
		ComplexID:        data.UUIDToPg(complexID),
		HasSessionFilter: sessionID != nil,
		SessionFilter:    data.UUIDPtrToPg(sessionID),
		HasCursor:        hasCursor,
		CursorCreatedAt:  data.TimeToPg(cursorTime),
		CursorID:         data.UUIDToPg(cursorID),
		PageLimit:        fetchLimit,
	})
	if err != nil {
		return nil, data.Metadata{}, fmt.Errorf("sale: list by complex: %w", err)
	}

	sales := make([]*Sale, len(rows))
	for i, r := range rows {
		sales[i] = saleFromDB(r)
	}

	sales, meta := data.TrimPage(sales, filters.Limit, data.BuildTimestampCursor)
	return sales, meta, nil
}

// Void corrects a sale in ONE transaction: it locks the sale row (refusing
// ErrAlreadyVoided if it is already voided) and the CURRENTLY open cash
// session (ErrNoOpenCashSession if none — the void's own cash movement, like
// cashbox's own VoidMovement, always lands in whichever session is open
// right now, which may not be the sale's own session), reads exactly the
// 'sale' stock movements this sale originally wrote (ListSaleStockMovements
// — never re-derived from the products' CURRENT tracks_stock/active state,
// so a product later deactivated or untracked is still restored exactly as
// much as it was sold), locks each of those products in ascending id order
// (same reasoning as Create) and reverses each with a 'sale_void' movement
// and a matching stock_on_hand delta, inserts the cash movement that voids
// the sale's own income (opposite kind, same amount/method/category,
// voids_movement_id set — the same shape cashbox.Service.VoidMovement
// builds), and marks the sale voided.
//
// ErrIncomeAlreadyVoided answers the income movement already carrying a
// void — defense in depth only: internal/cashbox.Service.VoidMovement
// refuses to manually void a 'sale' category movement precisely so this
// state is normally unreachable, but a movement voided before that
// restriction existed, or reached some other way, must still fail cleanly
// here rather than write a second, constraint-violating void.
//
//nolint:funlen // one cohesive transaction, the same shape and justification as Create above.
func (m *Store) Void(ctx context.Context, complexID, saleID, actorID uuid.UUID, note *string) (*Sale, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, complexID); err != nil {
		return nil, err
	}

	var voided *Sale
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		lockedSale, err := qtx.GetSaleByIDForUpdate(ctx, db.GetSaleByIDForUpdateParams{
			ID:        data.UUIDToPg(saleID),
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("sale: lock for void: %w", err)
		}
		if lockedSale.VoidedAt.Valid {
			return ErrAlreadyVoided
		}

		session, err := qtx.GetOpenCashSessionForUpdate(ctx, data.UUIDToPg(complexID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNoOpenCashSession
			}
			return fmt.Errorf("sale void: lock open cash session: %w", err)
		}

		debits, err := qtx.ListSaleStockMovements(ctx, db.ListSaleStockMovementsParams{
			SaleID:    lockedSale.ID,
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			return fmt.Errorf("sale void: list original stock movements: %w", err)
		}
		sort.Slice(debits, func(i, j int) bool {
			return bytes.Compare(debits[i].ProductID.Bytes[:], debits[j].ProductID.Bytes[:]) < 0
		})

		for _, debit := range debits {
			lockedProduct, err := qtx.GetProductByIDForUpdate(ctx, db.GetProductByIDForUpdateParams{
				ID:        debit.ProductID,
				ComplexID: data.UUIDToPg(complexID),
			})
			if err != nil {
				return fmt.Errorf("sale void: lock product %s: %w", data.PgToUUID(debit.ProductID), err)
			}

			restore := -debit.Quantity // debit.Quantity is negative (a 'sale' row); restore is positive.
			if _, err := qtx.InsertStockMovement(ctx, db.InsertStockMovementParams{
				ComplexID: data.UUIDToPg(complexID),
				ProductID: lockedProduct.ID,
				Kind:      "sale_void",
				Quantity:  restore,
				SaleID:    lockedSale.ID,
				CreatedBy: data.UUIDToPg(actorID),
			}); err != nil {
				return fmt.Errorf("sale void: insert sale_void movement for product %s: %w", data.PgToUUID(lockedProduct.ID), err)
			}

			if _, err := qtx.UpdateProductStock(ctx, db.UpdateProductStockParams{
				ID:        lockedProduct.ID,
				ComplexID: data.UUIDToPg(complexID),
				Delta:     restore,
			}); err != nil {
				return fmt.Errorf("sale void: restore stock for product %s: %w", data.PgToUUID(lockedProduct.ID), err)
			}
		}

		originalIncome, err := qtx.GetCashMovementByID(ctx, db.GetCashMovementByIDParams{
			ID:        lockedSale.CashMovementID,
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			return fmt.Errorf("sale void: read the sale's own income movement: %w", err)
		}

		voidMovement, err := qtx.InsertCashMovement(ctx, db.InsertCashMovementParams{
			ComplexID:       data.UUIDToPg(complexID),
			SessionID:       session.ID,
			Kind:            "expense",
			Category:        originalIncome.Category,
			Method:          originalIncome.Method,
			Amount:          originalIncome.Amount,
			Note:            data.TextToPg(note),
			VoidsMovementID: originalIncome.ID,
			CreatedBy:       data.UUIDToPg(actorID),
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.ConstraintName == voidsMovementUnique {
				return ErrIncomeAlreadyVoided
			}
			return fmt.Errorf("sale void: void the income movement: %w", err)
		}

		updatedRow, err := qtx.VoidSale(ctx, db.VoidSaleParams{
			VoidedAt:           data.TimeToPg(time.Now()),
			VoidedBy:           data.UUIDToPg(actorID),
			VoidCashMovementID: voidMovement.ID,
			ID:                 lockedSale.ID,
			ComplexID:          data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAlreadyVoided
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.ConstraintName == saleVoidedImmutable {
				return ErrAlreadyVoided
			}
			return fmt.Errorf("sale void: mark voided: %w", err)
		}

		voided = saleFromDB(updatedRow)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return voided, nil
}

func saleFromDB(s db.Sale) *Sale {
	return &Sale{
		ID:                 data.PgToUUID(s.ID),
		ComplexID:          data.PgToUUID(s.ComplexID),
		SessionID:          data.PgToUUID(s.SessionID),
		Method:             string(s.Method),
		Total:              int(s.Total),
		CashMovementID:     data.PgToUUID(s.CashMovementID),
		VoidedAt:           data.PgToTimePtr(s.VoidedAt),
		VoidedBy:           data.PgToUUIDPtr(s.VoidedBy),
		VoidCashMovementID: data.PgToUUIDPtr(s.VoidCashMovementID),
		CreatedAt:          data.PgToTime(s.CreatedAt),
		CreatedBy:          data.PgToUUID(s.CreatedBy),
	}
}

func saleItemFromDB(i db.SaleItem) *SaleItem {
	return &SaleItem{
		ID:          data.PgToUUID(i.ID),
		ComplexID:   data.PgToUUID(i.ComplexID),
		SaleID:      data.PgToUUID(i.SaleID),
		ProductID:   data.PgToUUID(i.ProductID),
		ProductName: i.ProductName,
		UnitPrice:   int(i.UnitPrice),
		Quantity:    int(i.Quantity),
		LineTotal:   int(i.LineTotal),
		CreatedAt:   data.PgToTime(i.CreatedAt),
	}
}
