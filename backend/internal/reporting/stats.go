package reporting

import (
	"fmt"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
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

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"stats": toGenDashboardStats(dash)})
}

// toGenDashboardStats maps the service's dashboard aggregate onto the
// generated wire type, translating the store-level booking counters, the
// upcoming bookings and the payment summary explicitly rather than
// serializing the store types directly (HTTP-08).
func toGenDashboardStats(dash *DashboardStats) gen.DashboardStats {
	var upcoming []gen.Booking
	if dash.Upcoming != nil {
		upcoming = make([]gen.Booking, len(dash.Upcoming))
		for i, b := range dash.Upcoming {
			upcoming[i] = toGenBooking(b)
		}
	}

	return gen.DashboardStats{
		MonthlyRevenue:    dash.Stats.MonthlyRevenue,
		OccupancyRate:     dash.OccupancyRate,
		PaymentSummary:    toGenPaymentSummary(dash.PaymentSummary),
		PendingBookings:   dash.Stats.PendingBookings,
		TodayBookings:     dash.Stats.TodayBookings,
		TodayRevenue:      dash.Stats.TodayRevenue,
		TotalClients:      dash.TotalClients,
		UpcomingBookings:  upcoming,
		WeeklyRevenue:     dash.Stats.WeeklyRevenue,
		YesterdayBookings: dash.Stats.YesterdayBookings,
		YesterdayRevenue:  dash.Stats.YesterdayRevenue,
	}
}

// toGenBooking maps a store booking onto the generated wire type explicitly,
// so an internal-only field (LinkToken, RefundIntentAt) can never leak onto
// the wire by being added to Booking later and serialized by accident
// (HTTP-08).
func toGenBooking(b *bookingstore.Booking) gen.Booking {
	return gen.Booking{
		ClientId:         b.ClientID,
		ClientName:       &b.ClientName,
		ClientPhone:      &b.ClientPhone,
		CollectionStatus: gen.CollectionStatus(b.CollectionStatus),
		ComplexId:        b.ComplexID,
		CourtId:          b.CourtID,
		CourtName:        &b.CourtName,
		CreatedAt:        b.CreatedAt,
		CreatedBy:        b.CreatedBy,
		Date:             b.Date,
		DepositAmount:    b.DepositAmount,
		DurationMinutes:  b.DurationMinutes,
		EndsAt:           b.EndsAt,
		Id:               b.ID,
		Notes:            b.Notes,
		Price:            b.Price,
		RefundStatus:     gen.RefundStatus(b.RefundStatus),
		ReminderSent2h:   b.ReminderSent2h,
		StartTime:        b.StartTime,
		StartsAt:         b.StartsAt,
		Status:           gen.BookingStatus(b.Status),
		UpdatedAt:        b.UpdatedAt,
	}
}

// toGenPaymentSummary maps the store's payment summary onto the generated
// wire type field-for-field; the two already agree on shape, so this only
// keeps the wire body from being the store type itself (HTTP-08).
func toGenPaymentSummary(ps *bookingstore.PaymentSummary) gen.PaymentSummary {
	if ps == nil {
		return gen.PaymentSummary{}
	}

	byStatus := make(map[string]gen.PaymentStatusBreakdown, len(ps.ByStatus))
	for k, v := range ps.ByStatus {
		byStatus[k] = gen.PaymentStatusBreakdown{Count: v.Count, Total: v.Total}
	}

	return gen.PaymentSummary{ByMethod: ps.ByMethod, ByStatus: byStatus}
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

	points, err := toGenRevenueDataPoints(revenue)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"revenue": points})
}

// toGenRevenueDataPoints maps the store's daily revenue points onto the
// generated wire type, parsing the store's "YYYY-MM-DD" string into the
// date-only type the OpenAPI schema declares for RevenueDataPoint.Date.
func toGenRevenueDataPoints(points []bookingstore.RevenueDataPoint) ([]gen.RevenueDataPoint, error) {
	if points == nil {
		return nil, nil
	}

	out := make([]gen.RevenueDataPoint, len(points))
	for i, p := range points {
		d, err := time.Parse("2006-01-02", p.Date)
		if err != nil {
			return nil, fmt.Errorf("parsing revenue date %q: %w", p.Date, err)
		}
		out[i] = gen.RevenueDataPoint{Date: openapi_types.Date{Time: d}, Amount: p.Amount}
	}
	return out, nil
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

	points := make([]gen.OccupancyPoint, len(result))
	for i, s := range result {
		points[i] = toGenOccupancyPoint(s)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"occupancy": points})
}

// toGenOccupancyPoint maps the service's occupancy slot onto the generated
// wire type. The two already agree field-for-field; the point of the mapper
// is that the wire body is always built from the generated type rather than
// whatever fields OccupancySlot happens to carry (HTTP-08).
func toGenOccupancyPoint(s OccupancySlot) gen.OccupancyPoint {
	return gen.OccupancyPoint{DayOfWeek: s.DayOfWeek, Hour: s.Hour, Percentage: s.Percentage}
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

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"clients": toGenClientInsights(insights)})
}

// toGenClientInsights maps the store's client insights onto the generated
// wire type. The two disagree on Go field names (TopClients/Top,
// CompletedCount/ResolvedCount, NewClients/NewClients30d, and so on) even
// though their JSON tags already match, which is exactly the case an
// explicit mapper exists to make safe (HTTP-08).
func toGenClientInsights(ci *clientstore.ClientInsights) gen.ClientInsights {
	var top []gen.TopClient
	if ci.TopClients != nil {
		top = make([]gen.TopClient, len(ci.TopClients))
		for i, c := range ci.TopClients {
			top[i] = toGenTopClient(c)
		}
	}

	return gen.ClientInsights{
		NewClients30d:  ci.NewClients,
		NoShowCount:    ci.NoShowCount,
		NoShowRate:     ci.NoShowRate,
		Recurring30d:   ci.Recurring,
		ResolvedCount:  ci.CompletedCount,
		Top:            top,
		TotalActive30d: ci.TotalActive,
	}
}

// toGenTopClient maps one store top-client row onto the generated wire type.
func toGenTopClient(c clientstore.TopClient) gen.TopClient {
	return gen.TopClient{
		BookingCount: c.BookingCount,
		Id:           c.ID,
		Name:         c.Name,
		Phone:        c.Phone,
		TotalSpent:   c.TotalSpent,
	}
}
