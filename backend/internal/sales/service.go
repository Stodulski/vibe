package sales

import (
	"context"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// Service holds this module's rules: what a sale may be, and how it and its
// void are audited. Every store call and every audit entry of the sales
// domain goes through it.
type Service struct {
	store Store
	audit Recorder
}

// NewService returns a Service backed by the given store and recorder.
func NewService(store Store, recorder Recorder) *Service {
	return &Service{store: store, audit: recorder}
}

// record writes an audit entry for a sale change.
func (s *Service) record(complexID uuid.UUID, actor Actor, action string, entityID *uuid.UUID, newVal any) {
	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: "sale",
		EntityID:   entityID,
		NewValue:   newVal,
		IPAddress:  actor.IP,
	})
}

// CreateInput is a validated request to sell a set of products.
type CreateInput struct {
	Items  []salestore.ItemInput
	Method string
	Note   *string
}

// Create records a sale and audits it. salestore.ErrNoOpenCashSession answers
// a complex with nothing open; *salestore.ErrInvalidItems answers a request
// naming a product that does not exist in this complex or is not active;
// salestore.ErrZeroTotal / ErrTotalExceedsCap answer a computed total outside
// the accepted range — see salestore.Store.Create for the whole transaction.
func (s *Service) Create(ctx context.Context, complexID uuid.UUID, actor Actor, actorID uuid.UUID, in CreateInput) (*SaleWithItems, []salestore.StockWarning, error) {
	sale, items, warnings, err := s.store.Create(ctx, complexID, actorID, in.Items, in.Method, in.Note)
	if err != nil {
		return nil, nil, err
	}

	result := &SaleWithItems{Sale: sale, Items: items}
	s.record(complexID, actor, "create", &sale.ID, map[string]any{"sale": sale, "items": items, "stock_warnings": warnings})
	return result, warnings, nil
}

// Get returns one sale of this complex with its items, or
// data.ErrRecordNotFound.
func (s *Service) Get(ctx context.Context, complexID, saleID uuid.UUID) (*SaleWithItems, error) {
	sale, err := s.store.GetByID(ctx, complexID, saleID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListItemsBySale(ctx, complexID, saleID)
	if err != nil {
		return nil, err
	}
	return &SaleWithItems{Sale: sale, Items: items}, nil
}

// List returns a page of this complex's sales with their items, newest
// first, optionally restricted to one session.
func (s *Service) List(ctx context.Context, complexID uuid.UUID, sessionID *uuid.UUID, filters data.Filters) ([]*SaleWithItems, data.Metadata, error) {
	page, meta, err := s.store.ListByComplex(ctx, complexID, sessionID, filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	ids := make([]uuid.UUID, len(page))
	for i, sale := range page {
		ids[i] = sale.ID
	}
	items, err := s.store.ListItemsBySaleIDs(ctx, complexID, ids)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	itemsBySale := make(map[uuid.UUID][]*salestore.SaleItem, len(page))
	for _, item := range items {
		itemsBySale[item.SaleID] = append(itemsBySale[item.SaleID], item)
	}

	result := make([]*SaleWithItems, len(page))
	for i, sale := range page {
		result[i] = &SaleWithItems{Sale: sale, Items: itemsBySale[sale.ID]}
	}
	return result, meta, nil
}

// Void corrects a sale and audits it — see salestore.Store.Void for the whole
// transaction. salestore.ErrAlreadyVoided answers a sale voided already;
// salestore.ErrNoOpenCashSession answers a complex with nothing open right
// now (the void's own cash movement needs somewhere to land);
// salestore.ErrIncomeAlreadyVoided is defense in depth only — see that
// sentinel's own comment.
func (s *Service) Void(ctx context.Context, complexID, saleID uuid.UUID, actor Actor, actorID uuid.UUID, note *string) (*SaleWithItems, error) {
	before, err := s.store.GetByID(ctx, complexID, saleID)
	if err != nil {
		return nil, err
	}

	voided, err := s.store.Void(ctx, complexID, saleID, actorID, note)
	if err != nil {
		return nil, err
	}

	// Recorded before the read below: the void has already committed, and a
	// failed read must not leave it without an audit entry that no retry
	// could write (a retry answers ErrAlreadyVoided).
	s.record(complexID, actor, "void", &voided.ID, map[string]any{"before": before, "after": voided})

	items, err := s.store.ListItemsBySale(ctx, complexID, saleID)
	if err != nil {
		return nil, err
	}
	return &SaleWithItems{Sale: voided, Items: items}, nil
}
