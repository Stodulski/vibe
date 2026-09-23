// Package reporting serves the complex owner's numbers: the live dashboard,
// the revenue and occupancy charts, the client insights, and the monthly
// payment report with its spreadsheet export.
//
// Everything here is read-only and scoped to one complex.
package reporting

import (
	"context"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// BookingReader is the booking side of the dashboard and charts.
type BookingReader interface {
	GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DashboardStats, error)
	GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*bookingstore.Booking, error)
	GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.PaymentSummary, error)
	GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.RevenueDataPoint, error)
	GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.OccupancyDataPoint, error)
}

// ClientReader is the client side: headline counts and the insights panel.
type ClientReader interface {
	CountByComplex(ctx context.Context, complexID uuid.UUID) (int, error)
	GetInsights(ctx context.Context, complexID uuid.UUID, today time.Time) (*clientstore.ClientInsights, error)
}

// CourtReader supplies the courts a complex has, which the dashboard and the
// occupancy grid are dimensioned by.
type CourtReader interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error)
}

// ScheduleReader supplies opening hours, which bound the occupancy grid.
type ScheduleReader interface {
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
}

// PaymentReportReader supplies the monthly aggregates behind the report and
// its export.
type PaymentReportReader interface {
	PaymentSummaryByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error)
	PaymentSummaryByCourt(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentCourtSummary, error)
	PaymentDetails(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentDetail, error)
	// CashSalesByMethod and CashMovementsByCategory back the monthly export's
	// "Caja" section — see reportstore.Store's own comments.
	CashSalesByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashSalesSummary, error)
	CashMovementsByCategory(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashCategorySummary, error)
}

// Handler serves the reporting routes. It decodes, validates, and maps the
// service's domain errors onto HTTP; every store call and every calculation
// lives in the Service.
type Handler struct {
	svc     *Service
	respond *httpx.Responder
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder) *Handler {
	return &Handler{svc: svc, respond: respond}
}
