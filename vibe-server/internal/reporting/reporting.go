// Package reporting serves the complex owner's numbers: the live dashboard,
// the revenue and occupancy charts, the client insights, and the monthly
// payment report with its spreadsheet export.
//
// Everything here is read-only and scoped to one complex.
package reporting

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// BookingReader is the booking side of the dashboard and charts.
type BookingReader interface {
	GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*data.DashboardStats, error)
	GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*data.Booking, error)
	GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*data.PaymentSummary, error)
	GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]data.RevenueDataPoint, error)
	GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]data.OccupancyDataPoint, error)
}

// ClientReader is the client side: headline counts and the insights panel.
type ClientReader interface {
	CountByComplex(ctx context.Context, complexID uuid.UUID) (int, error)
	GetInsights(ctx context.Context, complexID uuid.UUID, today time.Time) (*data.ClientInsights, error)
}

// CourtReader supplies the courts a complex has, which the dashboard and the
// occupancy grid are dimensioned by.
type CourtReader interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*data.Court, error)
}

// ScheduleReader supplies opening hours, which bound the occupancy grid.
type ScheduleReader interface {
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*data.Schedule, error)
}

// PaymentReportReader supplies the monthly aggregates behind the report and
// its export.
type PaymentReportReader interface {
	PaymentSummaryByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]data.PaymentMethodSummary, error)
	PaymentSummaryByCourt(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]data.PaymentCourtSummary, error)
	PaymentDetails(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]data.PaymentDetail, error)
}

// Handler serves the reporting routes.
type Handler struct {
	bookings  BookingReader
	clients   ClientReader
	courts    CourtReader
	complexes ScheduleReader
	reports   PaymentReportReader
	respond   *httpx.Responder

	// maxExportRows is defaultMaxExportRows, held as a field so a test can
	// exercise the cap boundary — which is an off-by-one in one comparison —
	// without building a fifty-thousand-row workbook to do it. The budget
	// beside it needs no such seam, so it stays a constant.
	maxExportRows int
}

// NewHandler returns a Handler backed by the given readers.
func NewHandler(
	bookings BookingReader,
	clients ClientReader,
	courts CourtReader,
	complexes ScheduleReader,
	reports PaymentReportReader,
	respond *httpx.Responder,
) *Handler {
	return &Handler{
		bookings:      bookings,
		clients:       clients,
		courts:        courts,
		complexes:     complexes,
		reports:       reports,
		respond:       respond,
		maxExportRows: defaultMaxExportRows,
	}
}

// Routes registers this module's endpoints. All of them expose one complex's
// commercial figures, so all of them require its owner.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	owner := func(next http.HandlerFunc) http.HandlerFunc {
		return guards.RequireAuth(guards.RequireComplexOwner(next))
	}

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/stats", owner(h.GetDashboardStats))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/stats/revenue", owner(h.GetRevenueChart))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/stats/occupancy", owner(h.GetOccupancyChart))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/stats/clients", owner(h.GetClientInsights))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/reports/monthly", owner(h.GetMonthlyReport))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/reports/export", owner(h.ExportPaymentsExcel))
}
