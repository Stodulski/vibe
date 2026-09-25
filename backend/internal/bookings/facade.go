package bookings

import (
	"context"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// FacadeStore is the booking persistence the cross-domain reads need, and
// nothing else: no client, complex, court, payment or notification port. It is
// what lets the Facade be constructed from the stores alone, before any domain
// service exists.
type FacadeStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*bookingstore.Booking, error)
	Update(ctx context.Context, b *bookingstore.Booking) error
	GetByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters) ([]*bookingstore.Booking, data.Metadata, error)
	GetByClient(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*bookingstore.Booking, error)
	GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]bookingstore.BookedSpan, error)
	HasActiveBookings(ctx context.Context, complexID uuid.UUID) (bool, error)
	HasActiveBookingsByCourt(ctx context.Context, courtID uuid.UUID) (bool, error)
	CancelFutureByComplex(ctx context.Context, complexID uuid.UUID) error
	GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DashboardStats, error)
	GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*bookingstore.Booking, error)
	GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.PaymentSummary, error)
	GetDayMoneyTotals(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DayMoneyTotals, error)
	GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.RevenueDataPoint, error)
	GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.OccupancyDataPoint, error)
	GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*bookingstore.Booking, error)
	ClaimRefundIntent(ctx context.Context, id uuid.UUID, seen time.Time) error
	ClearRefundIntent(ctx context.Context, id uuid.UUID) error
}

// Facade is the cross-domain entry point into the booking domain: the one type
// clients, complexes, courts, auth, reporting and payments hold instead of the
// booking store. *Facade satisfies every port they declare for themselves —
// courts.BookingReader, complexes.BookingStore, clients.BookingReader,
// auth.BookingReader, reporting.BookingReader, payments.BookingStore and
// payments.RefundIntentStore.
//
// It exists because Service cannot be it. Service depends on the clients,
// complexes, courts and payments services, so it is constructed last, after
// every domain that reads bookings has already been built — which is why those
// domains used to receive the store. The Facade closes that edge without a
// setter chain: it is built over the store alone, right after the stores, so
// it is available to every consumer and still keeps this package the only one
// that knows the booking store exists.
//
// A method here only proxies the store today. The point is the seam: a rule
// added later to a cross-domain read — authorization, caching, an event — lands
// in one place instead of in each caller. Service embeds it, so the handler and
// cron paths reach the same methods through the same code.
type Facade struct {
	store FacadeStore
}

// NewFacade returns a Facade backed by the booking store.
func NewFacade(store FacadeStore) *Facade {
	return &Facade{store: store}
}

// GetByID returns one booking. Exported for payments.
func (f *Facade) GetByID(ctx context.Context, id uuid.UUID) (*bookingstore.Booking, error) {
	return f.store.GetByID(ctx, id)
}

// Update writes a booking row. Exported for payments, which confirms and
// cancels bookings from the provider's side.
//
// It is the bare write, not this domain's own editing use case: the owner
// editing a booking through the dashboard goes through Service.Update, which
// applies every transition rule first and then reaches the store itself. The
// two share a name on different types because each is the natural one there —
// payments.BookingStore declares this signature.
func (f *Facade) Update(ctx context.Context, b *bookingstore.Booking) error {
	return f.store.Update(ctx, b)
}

// GetByComplex returns a complex's bookings for a date range. Exported for
// other domains that page over them.
func (f *Facade) GetByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters) ([]*bookingstore.Booking, data.Metadata, error) {
	return f.store.GetByComplex(ctx, complexID, dateFrom, dateTo, filters)
}

// GetByClient returns one client's recent bookings. Exported for clients.
func (f *Facade) GetByClient(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*bookingstore.Booking, error) {
	return f.store.GetByClient(ctx, complexID, clientID, limit)
}

// GetBookedSlotsByCourtIDs returns the spans already sold on a date. Exported
// for courts, whose availability grid subtracts them.
func (f *Facade) GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]bookingstore.BookedSpan, error) {
	return f.store.GetBookedSlotsByCourtIDs(ctx, courtIDs, date)
}

// HasActiveBookings reports whether a complex still has live bookings. Exported
// for complexes and auth, both of which refuse a deletion while it does.
func (f *Facade) HasActiveBookings(ctx context.Context, complexID uuid.UUID) (bool, error) {
	return f.store.HasActiveBookings(ctx, complexID)
}

// HasActiveBookingsByCourt is the same question for one court. Exported for
// courts.
func (f *Facade) HasActiveBookingsByCourt(ctx context.Context, courtID uuid.UUID) (bool, error) {
	return f.store.HasActiveBookingsByCourt(ctx, courtID)
}

// CancelFutureByComplex cancels everything still ahead of a venue being closed.
// Exported for complexes.
func (f *Facade) CancelFutureByComplex(ctx context.Context, complexID uuid.UUID) error {
	return f.store.CancelFutureByComplex(ctx, complexID)
}

// GetDashboardStats returns the dashboard headline figures. Exported for reporting.
func (f *Facade) GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DashboardStats, error) {
	return f.store.GetDashboardStats(ctx, complexID, today)
}

// GetUpcomingToday returns today's remaining bookings. Exported for reporting.
func (f *Facade) GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*bookingstore.Booking, error) {
	return f.store.GetUpcomingToday(ctx, complexID, today, nowTime, limit)
}

// GetPaymentSummary returns today's money figures. Exported for reporting.
func (f *Facade) GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.PaymentSummary, error) {
	return f.store.GetPaymentSummary(ctx, complexID, today)
}

// GetDayMoneyTotals returns today's day-level money totals. Exported for reporting.
func (f *Facade) GetDayMoneyTotals(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DayMoneyTotals, error) {
	return f.store.GetDayMoneyTotals(ctx, complexID, today)
}

// GetRevenueByDay returns the revenue chart's series. Exported for reporting.
func (f *Facade) GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.RevenueDataPoint, error) {
	return f.store.GetRevenueByDay(ctx, complexID, from, to)
}

// GetOccupancyByHourDay returns the occupancy grid's series. Exported for reporting.
func (f *Facade) GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.OccupancyDataPoint, error) {
	return f.store.GetOccupancyByHourDay(ctx, complexID, from, to)
}

// GetRefundIntentOrphans returns cancellations whose refund-intent marker has
// stood past its grace period. Exported for payments' reconciliation sweep.
func (f *Facade) GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*bookingstore.Booking, error) {
	return f.store.GetRefundIntentOrphans(ctx, olderThan, limit)
}

// ClaimRefundIntent takes one orphan for a single sweep run. Exported for payments.
func (f *Facade) ClaimRefundIntent(ctx context.Context, id uuid.UUID, seen time.Time) error {
	return f.store.ClaimRefundIntent(ctx, id, seen)
}

// ClearRefundIntent drops the marker once the refund path is done with the
// booking. Exported for payments.
func (f *Facade) ClearRefundIntent(ctx context.Context, id uuid.UUID) error {
	return f.store.ClearRefundIntent(ctx, id)
}
