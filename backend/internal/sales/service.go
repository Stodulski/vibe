package sales

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Service holds this module's rules: what a sale may be, and how it and its
// void are audited. Every store call and every audit entry of the sales
// domain goes through it.
type Service struct {
	store    Store
	audit    Recorder
	realtime BookingChangedPublisher
	// now is the clock the sell page's "today" is read from. Tests replace it.
	now func() time.Time
}

// NewService returns a Service backed by the given store and recorder.
func NewService(store Store, recorder Recorder) *Service {
	return &Service{store: store, audit: recorder, now: time.Now}
}

// WithBookingPublisher makes a checkout that collects a booking balance refresh
// open booking pages, the way the booking panel's own payment does.
func (s *Service) WithBookingPublisher(p BookingChangedPublisher) *Service {
	s.realtime = p
	return s
}

// artDay is the Argentina calendar day of an instant, as bookings.date stores
// a day: midnight UTC of the local date, so a DATE round-trips unchanged.
func artDay(at time.Time) time.Time {
	local := at.In(timezone.Argentina)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
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

// CheckoutInput is a validated counter checkout: product lines, booking
// balances, or both.
type CheckoutInput struct {
	Items           []salestore.ItemInput
	BookingPayments []salestore.BookingPaymentInput
	Method          string
	Note            *string
}

// Checkout records one counter checkout and audits what it did. The store
// refusals (salestore.Checkout's list) are returned unchanged, and a refused
// checkout records nothing, so there is nothing to audit on that path.
func (s *Service) Checkout(ctx context.Context, complexID uuid.UUID, actor Actor, actorID uuid.UUID, in CheckoutInput) (*salestore.CheckoutResult, error) {
	res, err := s.store.Checkout(ctx, complexID, actorID, in.Items, in.BookingPayments, in.Method, in.Note)
	if err != nil {
		return nil, err
	}

	if res.Sale != nil {
		s.record(complexID, actor, "create", &res.Sale.ID, map[string]any{"sale": res.Sale, "items": res.Items, "stock_warnings": res.Warnings, "booking_payments": res.BookingLines})
	}
	for _, line := range res.BookingLines {
		bookingID := line.BookingID
		s.audit.Record(audit.Entry{
			UserID:     actor.UserID,
			ComplexID:  &complexID,
			Action:     "confirm_payment",
			EntityType: "booking",
			EntityID:   &bookingID,
			NewValue:   map[string]any{"payment_id": line.PaymentID, "amount": line.Amount, "method": in.Method, "sale_id": saleIDOf(res)},
			IPAddress:  actor.IP,
		})
	}
	if len(res.BookingLines) > 0 && s.realtime != nil {
		s.realtime.PublishBookingChanged(complexID)
	}
	return res, nil
}

// saleIDOf names the sale a checkout recorded, or nil for a booking-only one.
func saleIDOf(res *salestore.CheckoutResult) *uuid.UUID {
	if res.Sale == nil {
		return nil
	}
	return &res.Sale.ID
}

// BookingsDue lists the complex's bookings of today (Argentina) that still owe
// money, and returns the day they were read for.
func (s *Service) BookingsDue(ctx context.Context, complexID uuid.UUID) (time.Time, []*salestore.BalanceDue, error) {
	day := artDay(s.now())
	rows, err := s.store.ListBalancesDue(ctx, complexID, day)
	if err != nil {
		return time.Time{}, nil, err
	}
	return day, rows, nil
}

// Create records a product sale and audits it. salestore.ErrNoOpenCashSession answers
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
	lines, err := s.store.ListBookingPaymentsBySaleIDs(ctx, complexID, []uuid.UUID{saleID})
	if err != nil {
		return nil, err
	}
	return &SaleWithItems{Sale: sale, Items: items, BookingPayments: lines[saleID]}, nil
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

	linesBySale, err := s.store.ListBookingPaymentsBySaleIDs(ctx, complexID, ids)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	result := make([]*SaleWithItems, len(page))
	for i, sale := range page {
		result[i] = &SaleWithItems{Sale: sale, Items: itemsBySale[sale.ID], BookingPayments: linesBySale[sale.ID]}
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
