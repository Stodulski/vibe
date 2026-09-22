package products

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
)

// AdjustmentReasons lists the reason values a stock adjustment may carry —
// mirrors stock_movements_reason_check (db/migrations/004_pos_catalog_stock.sql).
var AdjustmentReasons = []string{"breakage", "expired", "own_consumption", "count_correction", "other"}

// ErrEditConflict reports that a product moved out from under a read: the row
// carries a different version now, or is gone. Wraps data.ErrEditConflict —
// the same shape courts.ErrEditConflict follows — so both answer 409.
var ErrEditConflict = fmt.Errorf("product changed before the update: %w", data.ErrEditConflict)

// ErrProductHasStock reports an attempt to stop tracking stock on a product
// that still carries a non-zero stock_on_hand. See Service.Update's own
// comment for why this is the chosen rule.
var ErrProductHasStock = errors.New("product: cannot stop tracking stock while stock is not zero")

// Service holds this module's rules: what a product may be, what a restock
// or an adjustment may do, and how they are audited. Every store call and
// every audit entry of the products domain goes through it.
type Service struct {
	catalog Catalog
	ledger  StockLedger
	audit   Recorder
}

// NewService returns a Service backed by the given store and recorder.
func NewService(store Store, recorder Recorder) *Service {
	return &Service{catalog: store, ledger: store, audit: recorder}
}

// record writes an audit entry for a products change. Every write in this
// domain audits against the product it changed (entity_type "product", even
// a restock or an adjustment) with no old value recorded — unlike cashbox's
// Close, nothing here reads the previous state back into the trail — so
// both are fixed rather than parameters.
func (s *Service) record(complexID uuid.UUID, actor Actor, action string, entityID *uuid.UUID, newVal any) {
	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: "product",
		EntityID:   entityID,
		NewValue:   newVal,
		IPAddress:  actor.IP,
	})
}

// ownedProduct loads a product this complex actually owns, or
// data.ErrRecordNotFound — the same "read before act" every mutation below
// starts from, mirroring courts.Service.ownedCourt.
func (s *Service) ownedProduct(ctx context.Context, complexID, productID uuid.UUID) (*productstore.Product, error) {
	return s.catalog.GetByID(ctx, complexID, productID)
}

// CreateInput is a validated request to add a product to a complex's catalog.
type CreateInput struct {
	Name              string
	Category          *string
	Price             int
	TracksStock       bool
	LowStockThreshold *int
}

// Create adds a product and records it. productstore.ErrDuplicateProductName
// answers a name collision with another active product.
func (s *Service) Create(ctx context.Context, complexID uuid.UUID, actor Actor, in CreateInput) (*productstore.Product, error) {
	product := &productstore.Product{
		ComplexID:         complexID,
		Name:              in.Name,
		Category:          in.Category,
		Price:             in.Price,
		TracksStock:       in.TracksStock,
		LowStockThreshold: in.LowStockThreshold,
		Active:            true,
	}
	if err := s.catalog.Insert(ctx, product); err != nil {
		return nil, err
	}

	s.record(complexID, actor, "create", &product.ID, product)
	return product, nil
}

// Get returns one product of this complex, or data.ErrRecordNotFound.
func (s *Service) Get(ctx context.Context, complexID, productID uuid.UUID) (*productstore.Product, error) {
	return s.ownedProduct(ctx, complexID, productID)
}

// List returns every product of a complex, optionally filtered by active
// state — unpaginated, see productstore.Store.ListByComplex's own comment.
func (s *Service) List(ctx context.Context, complexID uuid.UUID, activeFilter *bool) ([]*productstore.Product, error) {
	return s.catalog.ListByComplex(ctx, complexID, activeFilter)
}

// UpdateInput is a validated partial update. A nil field keeps its current
// value — the same convention courts.UpdateInput follows.
type UpdateInput struct {
	Name *string
	// Category follows courts.UpdateInput.Description's own convention: an
	// empty string is how a client clears it, so it lands as NULL rather than
	// a row holding "". Nil keeps the current value.
	Category          *string
	Price             *int
	LowStockThreshold *int
	Active            *bool
	TracksStock       *bool
	// ExpectedVersion is the version the client read before it filled in the
	// form, from If-Match or the body. Nil means it sent none, and the write
	// stays last-write-wins (API-08) — same as courts.UpdateInput.
	ExpectedVersion *int
}

// Update applies a partial change to a product of this complex and records
// it. It reports ErrEditConflict when the row moved out from under the read,
// mirroring courts.Service.Update.
//
// Turning tracks_stock off while stock_on_hand is not zero is refused with
// ErrProductHasStock rather than silently freezing the count: the owner's
// last true count would otherwise sit unreviewed and unreachable (no
// adjustment or restock route accepts a product that does not track stock),
// so the least surprising rule is to make them adjust it to zero first — an
// explicit, audited action — before they can stop tracking it. Turning
// tracks_stock ON needs no such check: a freshly-tracked product starting
// from whatever stock_on_hand already holds (0, if it never tracked before)
// is exactly the state a client resuming tracking expects.
func (s *Service) Update(ctx context.Context, complexID uuid.UUID, actor Actor, productID uuid.UUID, in UpdateInput) (*productstore.Product, error) {
	product, err := s.ownedProduct(ctx, complexID, productID)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		product.Name = *in.Name
	}
	if in.Category != nil {
		product.Category = emptyToNil(in.Category)
	}
	if in.Price != nil {
		product.Price = *in.Price
	}
	if in.LowStockThreshold != nil {
		product.LowStockThreshold = in.LowStockThreshold
	}
	if in.Active != nil {
		product.Active = *in.Active
	}
	if in.TracksStock != nil {
		if !*in.TracksStock && product.TracksStock && product.StockOnHand != 0 {
			return nil, ErrProductHasStock
		}
		product.TracksStock = *in.TracksStock
	}

	if err := s.catalog.Update(ctx, product, in.ExpectedVersion); err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			// Zero rows is either "the product is gone" or "somebody wrote it
			// first" — ownedProduct already proved it existed a moment ago, so
			// this can only be the second case. Same reasoning as
			// courts.Service.Update.
			return nil, ErrEditConflict
		}
		return nil, err
	}

	s.record(complexID, actor, "update", &product.ID, product)
	return product, nil
}

// RestockInput is a validated request to record a stock delivery.
type RestockInput struct {
	Quantity  int
	TotalCost int
	Method    string
	Note      *string
}

// Restock records a stock delivery against an open cash session, in one
// transaction with the cash expense it costs — see
// productstore.Store.Restock. productstore.ErrNoOpenCashSession,
// ErrProductInactive and ErrProductNotTrackingStock answer the product/session
// states this cannot run against.
func (s *Service) Restock(ctx context.Context, complexID, productID uuid.UUID, actor Actor, actorID uuid.UUID, in RestockInput) (*productstore.Product, *productstore.StockMovement, error) {
	product, movement, err := s.ledger.Restock(ctx, complexID, productID, actorID, in.Quantity, in.TotalCost, in.Method, in.Note)
	if err != nil {
		return nil, nil, err
	}

	s.record(complexID, actor, "restock", &product.ID, map[string]any{"product": product, "movement": movement})
	return product, movement, nil
}

// AdjustInput is a validated request to correct a product's stock by hand.
type AdjustInput struct {
	Quantity int
	Reason   string
	Note     *string
}

// Adjust records a stock correction with no money involved — see
// productstore.Store.Adjust. productstore.ErrProductNotTrackingStock answers
// a product whose tracks_stock is false.
func (s *Service) Adjust(ctx context.Context, complexID, productID uuid.UUID, actor Actor, actorID uuid.UUID, in AdjustInput) (*productstore.Product, *productstore.StockMovement, error) {
	product, movement, err := s.ledger.Adjust(ctx, complexID, productID, actorID, in.Quantity, in.Reason, in.Note)
	if err != nil {
		return nil, nil, err
	}

	s.record(complexID, actor, "adjust", &product.ID, map[string]any{"product": product, "movement": movement})
	return product, movement, nil
}

// StockMovements returns a page of a product's stock-movement history,
// newest first. It confirms the product belongs to this complex first, so a
// mismatched (complexID, productID) pair answers data.ErrRecordNotFound
// rather than an empty page.
func (s *Service) StockMovements(ctx context.Context, complexID, productID uuid.UUID, filters data.Filters) ([]*productstore.StockMovement, data.Metadata, error) {
	if _, err := s.ownedProduct(ctx, complexID, productID); err != nil {
		return nil, data.Metadata{}, err
	}
	return s.ledger.ListStockMovements(ctx, complexID, productID, filters)
}

// emptyToNil turns an empty string into nil, the same convention
// courts/handlers.go's own emptyToNil follows for a clearable free-text field.
func emptyToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}
