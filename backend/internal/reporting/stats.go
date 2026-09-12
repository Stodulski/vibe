package reporting

import (
	"fmt"
	"net/http"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// allowedRevenuePeriods are the values GetRevenueChart accepts for "period".
var allowedRevenuePeriods = []string{"week", "month"}

// GetDashboardStats handles GET /api/v1/complexes/{id}/stats, returning the
// headline figures for today plus the next bookings due.
//
// It is one cohesive request lifecycle — parse the input, aggregate the day's
// figures, respond — and splitting it would relocate sequential steps into
// helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	dash, err := h.svc.DashboardStats(r.Context(), complex.ID, time.Now())
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"stats": map[string]any{
			"today_bookings":     dash.Stats.TodayBookings,
			"today_revenue":      dash.Stats.TodayRevenue,
			"yesterday_bookings": dash.Stats.YesterdayBookings,
			"yesterday_revenue":  dash.Stats.YesterdayRevenue,
			"weekly_revenue":     dash.Stats.WeeklyRevenue,
			"monthly_revenue":    dash.Stats.MonthlyRevenue,
			"occupancy_rate":     dash.OccupancyRate,
			"pending_bookings":   dash.Stats.PendingBookings,
			"total_clients":      dash.TotalClients,
			"upcoming_bookings":  dash.Upcoming,
			"payment_summary":    dash.PaymentSummary,
		},
	})
}

// GetRevenueChart handles GET /api/v1/complexes/{id}/stats/revenue, returning
// daily revenue over the requested period.
func (h *Handler) GetRevenueChart(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	qs := r.URL.Query()
	period := httpx.ReadString(qs, "period", "week")

	v := validator.New()
	v.Check(validator.PermittedValue(period, allowedRevenuePeriods...), "period", "must be one of: week, month")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	revenue, err := h.svc.RevenueChart(r.Context(), complex.ID, time.Now(), period)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"revenue": revenue})
}

// GetOccupancyChart handles GET /api/v1/complexes/{id}/stats/occupancy, returning
// the share of open court-hours booked, by hour and weekday.
func (h *Handler) GetOccupancyChart(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	qs := r.URL.Query()
	weeks := max(httpx.ReadInt(qs, "weeks", 4), 1)
	if weeks > 12 {
		weeks = 12
	}

	result, err := h.svc.OccupancyChart(r.Context(), complex.ID, time.Now(), weeks)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"occupancy": result})
}

// GetClientInsights handles GET /api/v1/complexes/{id}/stats/clients, returning
// the retention and frequency panel.
func (h *Handler) GetClientInsights(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	insights, err := h.svc.ClientInsights(r.Context(), complex.ID, time.Now())
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"clients": insights})
}
