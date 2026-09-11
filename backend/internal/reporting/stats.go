package reporting

import (
	"fmt"
	"net/http"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// allowedRevenuePeriods are the values GetRevenueChart accepts for "period".
var allowedRevenuePeriods = []string{"week", "month"}

// GetDashboardStats handles GET /api/v1/complexes/:id/stats, returning the
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
	now := time.Now().In(timezone.Argentina)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone.Argentina)
	nowTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	stats, err := h.bookings.GetDashboardStats(r.Context(), complex.ID, today)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	totalClients, err := h.clients.CountByComplex(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// Calculate occupancy rate: (booked hours today) / (total available hours today) × 100.
	courts, err := h.courts.GetByComplex(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	activeCourts := 0
	for _, c := range courts {
		if c.IsActive {
			activeCourts++
		}
	}

	schedules, err := h.complexes.GetSchedules(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	dayName := slots.DayName(today.Weekday())
	var openHours float64
	for _, s := range schedules {
		if s.Day == dayName && !s.IsClosed {
			openHours = slots.ToHours(s.CloseTime) - slots.ToHours(s.OpenTime)
			break
		}
	}

	var occupancyRate int
	totalSlotHours := float64(activeCourts) * openHours
	if totalSlotHours > 0 {
		bookedHours := float64(stats.TodayBookedMinutes) / 60.0
		occupancyRate = min(int((bookedHours/totalSlotHours)*100), 100)
	}

	upcoming, err := h.bookings.GetUpcomingToday(r.Context(), complex.ID, today, nowTime, 10)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// Payment summary for today.
	paymentSummary, err := h.bookings.GetPaymentSummary(r.Context(), complex.ID, today)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"stats": map[string]any{
			"today_bookings":     stats.TodayBookings,
			"today_revenue":      stats.TodayRevenue,
			"yesterday_bookings": stats.YesterdayBookings,
			"yesterday_revenue":  stats.YesterdayRevenue,
			"weekly_revenue":     stats.WeeklyRevenue,
			"monthly_revenue":    stats.MonthlyRevenue,
			"occupancy_rate":     occupancyRate,
			"pending_bookings":   stats.PendingBookings,
			"total_clients":      totalClients,
			"upcoming_bookings":  upcoming,
			"payment_summary":    paymentSummary,
		},
	})
}

// GetRevenueChart handles GET /api/v1/complexes/:id/stats/revenue, returning
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

	now := time.Now().In(timezone.Argentina)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone.Argentina)

	var from time.Time
	switch period {
	case "month":
		from = today.AddDate(0, 0, -29)
	default:
		from = today.AddDate(0, 0, -6)
	}

	revenue, err := h.bookings.GetRevenueByDay(r.Context(), complex.ID, from, today)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"revenue": revenue})
}

// GetOccupancyChart handles GET /api/v1/complexes/:id/stats/occupancy, returning
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

	now := time.Now().In(timezone.Argentina)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone.Argentina)
	from := today.AddDate(0, 0, -7*weeks)

	rawData, err := h.bookings.GetOccupancyByHourDay(r.Context(), complex.ID, from, today)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	courts, err := h.courts.GetByComplex(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	activeCourts := 0
	for _, c := range courts {
		if c.IsActive {
			activeCourts++
		}
	}

	// Max possible bookings per (day_of_week, hour) slot = activeCourts × weeks.
	maxPerSlot := activeCourts * weeks
	type occupancyResult struct {
		DayOfWeek  int `json:"day_of_week"`
		Hour       int `json:"hour"`
		Percentage int `json:"percentage"`
	}

	result := make([]occupancyResult, len(rawData))
	for i, dp := range rawData {
		pct := 0
		if maxPerSlot > 0 {
			pct = min((dp.BookingCount*100)/maxPerSlot, 100)
		}
		result[i] = occupancyResult{
			DayOfWeek:  dp.DayOfWeek,
			Hour:       dp.Hour,
			Percentage: pct,
		}
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"occupancy": result})
}

// GetClientInsights handles GET /api/v1/complexes/:id/stats/clients, returning
// the retention and frequency panel.
func (h *Handler) GetClientInsights(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	now := time.Now().In(timezone.Argentina)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone.Argentina)

	insights, err := h.clients.GetInsights(r.Context(), complex.ID, today)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"clients": insights})
}
