package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/validator"
)

// CreateSeries handles POST /api/v1/complexes/{id}/booking-series, the owner's
// fixed weekly booking.
//
//nolint:funlen // one request lifecycle: decode, validate, call, answer
func (h *Handler) CreateSeries(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input struct {
		CourtID         string  `json:"court_id"`
		FirstDate       string  `json:"first_date"`
		StartTime       string  `json:"start_time"`
		DurationMinutes int     `json:"duration_minutes"`
		ClientPhone     string  `json:"client_phone"`
		ClientEmail     string  `json:"client_email"`
		ClientFirstName string  `json:"client_first_name"`
		ClientLastName  string  `json:"client_last_name"`
		Notes           string  `json:"notes"`
		Price           *int    `json:"price"`
		EndsOn          *string `json:"ends_on"`
		Weeks           int     `json:"weeks"`
		SkipConflicts   bool    `json:"skip_conflicts"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.CourtID != "", "court_id", "must be provided")
	v.Check(input.FirstDate != "", "first_date", "must be provided")
	v.Check(input.StartTime != "", "start_time", "must be provided")
	v.Check(input.ClientPhone != "", "client_phone", "must be provided")
	if input.ClientPhone != "" {
		normalized, nerr := validator.NormalizePhone(input.ClientPhone)
		if nerr != nil {
			v.AddError("client_phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			input.ClientPhone = normalized
		}
	}
	v.Check(input.ClientFirstName != "", "client_first_name", "must be provided")
	v.Check(input.ClientLastName != "", "client_last_name", "must be provided")
	if input.ClientEmail != "" {
		v.Check(validator.Matches(input.ClientEmail, validator.EmailRX), "client_email", "must be a valid email address")
	}
	v.Check(validator.PermittedValue(input.DurationMinutes, slots.PermittedDurations()...), "duration_minutes", durationMessage)
	if input.Price != nil {
		v.Check(*input.Price >= 0, "price", "must not be negative")
	}
	v.Check(input.Weeks >= 0 && input.Weeks <= MaxSeriesWeeks, "weeks", fmt.Sprintf("must be between 1 and %d", MaxSeriesWeeks))
	if input.StartTime != "" {
		v.Check(slots.ValidFormat(input.StartTime), "start_time", "must be in HH:MM format (00:00-23:59)")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	courtID, err := uuid.Parse(input.CourtID)
	if err != nil {
		v.AddError("court_id", "must be a valid UUID")
	}
	firstDate, err := time.Parse(time.DateOnly, input.FirstDate)
	if err != nil {
		v.AddError("first_date", "must be in YYYY-MM-DD format")
	}
	var endsOn *time.Time
	if input.EndsOn != nil && *input.EndsOn != "" {
		end, err := time.Parse(time.DateOnly, *input.EndsOn)
		if err != nil {
			v.AddError("ends_on", "must be in YYYY-MM-DD format")
		}
		endsOn = &end
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	result, err := h.svc.CreateSeries(r.Context(), complex, h.actor(r), SeriesInput{
		CourtID:         courtID,
		FirstDate:       firstDate,
		StartTime:       input.StartTime,
		DurationMinutes: input.DurationMinutes,
		ClientFirstName: input.ClientFirstName,
		ClientLastName:  input.ClientLastName,
		ClientPhone:     input.ClientPhone,
		ClientEmail:     input.ClientEmail,
		Notes:           input.Notes,
		Price:           input.Price,
		EndsOn:          endsOn,
		Weeks:           input.Weeks,
		SkipConflicts:   input.SkipConflicts,
	})
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"series":   toGenSeries(result.Series),
		"bookings": nonNil(toGenBookings(result.Bookings)),
		"skipped":  toGenSeriesConflicts(result.Skipped),
	})
}

// GetSeries handles GET /api/v1/complexes/{id}/booking-series/{seriesID}.
func (h *Handler) GetSeries(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	seriesID, err := httpx.ReadUUIDParam(r, "seriesID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	detail, err := h.svc.GetSeries(r.Context(), complex.ID, seriesID)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"series":   toGenSeries(detail.Series),
		"upcoming": nonNil(toGenBookings(detail.Upcoming)),
		"skipped":  toGenSeriesConflicts(detail.Skipped),
	})
}

// EndSeries handles POST /api/v1/complexes/{id}/booking-series/{seriesID}/end.
func (h *Handler) EndSeries(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	seriesID, err := httpx.ReadUUIDParam(r, "seriesID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		FromDate string `json:"from_date"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	v := validator.New()
	v.Check(input.FromDate != "", "from_date", "must be provided")
	var from time.Time
	if input.FromDate != "" {
		from, err = time.Parse(time.DateOnly, input.FromDate)
		if err != nil {
			v.AddError("from_date", "must be in YYYY-MM-DD format")
		}
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	result, err := h.svc.EndSeries(r.Context(), complex, h.actor(r), seriesID, from)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"series":    toGenSeries(result.Series),
		"cancelled": nonNil(toGenBookings(result.Cancelled)),
		"kept":      nonNil(toGenBookings(result.Kept)),
	})
}

// refuseSeriesConflicts answers a *SeriesConflictsError with the dates.
func (h *Handler) refuseSeriesConflicts(w http.ResponseWriter, r *http.Request, err error) bool {
	var conflicts *SeriesConflictsError
	if !errors.As(err, &conflicts) {
		return false
	}
	out := make([]httpx.DateConflict, len(conflicts.Conflicts))
	for i, c := range conflicts.Conflicts {
		out[i] = httpx.DateConflict{Date: c.Date.Format(time.DateOnly), Reason: c.Reason}
	}
	h.respond.ConflictDates(w, r, "some dates of the series cannot be booked", out)
	return true
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func toGenSeries(s *bookingstore.Series) gen.BookingSeries {
	out := gen.BookingSeries{
		ClientId:        s.ClientID,
		ComplexId:       s.ComplexID,
		CourtId:         s.CourtID,
		CreatedAt:       s.CreatedAt,
		DurationMinutes: s.DurationMinutes,
		Id:              s.ID,
		Notes:           s.Notes,
		Price:           s.Price,
		StartTime:       s.StartTime,
		StartsOn:        openapi_types.Date{Time: s.StartsOn},
		Status:          gen.BookingSeriesStatus(s.Status),
		Weekday:         s.Weekday,
	}
	if s.EndsOn != nil {
		out.EndsOn = &openapi_types.Date{Time: *s.EndsOn}
	}
	return out
}

func toGenSeriesConflicts(cs []SeriesConflict) []gen.BookingSeriesConflict {
	out := make([]gen.BookingSeriesConflict, len(cs))
	for i, c := range cs {
		out[i] = gen.BookingSeriesConflict{Date: openapi_types.Date{Time: c.Date}, Reason: gen.BookingSeriesConflictReason(c.Reason)}
	}
	return out
}
