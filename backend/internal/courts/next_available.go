package courts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// nextAvailableWindowDays bounds how far past `from` the search looks. A
// storefront asks for the next day with room once the day on screen is full;
// two weeks is the window a player plans in, and it caps the work an
// unauthenticated endpoint can cause at fourteen grid reads.
const nextAvailableWindowDays = 14

// NextAvailableDate returns the first day after from, within the search window
// and the booking horizon, on which at least one active court has a bookable
// slot of the given duration. found is false when no day in that range has one.
//
// It reuses Availability for every day, so a day counts as available exactly
// when the grid the storefront would draw for it shows a free slot. A from in
// the past starts the search today, since no earlier day can be booked.
func (s *Service) NextAvailableDate(ctx context.Context, slug string, from time.Time, duration int) (time.Time, bool, error) {
	today := timezone.Today()
	start := from.AddDate(0, 0, 1)
	if start.Before(today) {
		start = today
	}
	last := from.AddDate(0, 0, nextAvailableWindowDays)
	if horizon := today.AddDate(0, 0, bookingstore.MaxBookingHorizonDays); last.After(horizon) {
		last = horizon
	}

	for candidate := start; !candidate.After(last); candidate = candidate.AddDate(0, 0, 1) {
		grid, err := s.Availability(ctx, slug, candidate.Format("2006-01-02"), candidate, duration)
		if err != nil {
			return time.Time{}, false, err
		}
		if hasBookableSlot(grid) {
			return candidate, true, nil
		}
	}
	return time.Time{}, false, nil
}

// hasBookableSlot reports whether any court on the grid has a free slot.
func hasBookableSlot(grid *AvailabilityResponse) bool {
	for _, court := range grid.Courts {
		for _, slot := range court.Slots {
			if slot.Available {
				return true
			}
		}
	}
	return false
}

// NextAvailable handles GET /api/v1/public/complexes/{slug}/next-available. It
// answers with the first day after `from` that has a bookable slot, or a null
// date when none exists in the window.
func (h *Handler) NextAvailable(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	qs := r.URL.Query()
	fromStr := httpx.ReadString(qs, "from", "")
	if fromStr == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("from query parameter is required"))
		return
	}

	from, err := timezone.ParseDay(fromStr)
	if err != nil {
		h.respond.BadRequest(w, r, fmt.Errorf("from must be in YYYY-MM-DD format"))
		return
	}

	// duration is read the same way the availability grid reads it, so the
	// answer is about a length the storefront can actually book.
	duration := defaultAvailabilityDuration
	if durationStr := httpx.ReadString(qs, "duration", ""); durationStr != "" {
		parsed, convErr := strconv.Atoi(durationStr)
		if convErr != nil || !validator.PermittedValue(parsed, slots.PermittedDurations()...) {
			v := validator.New()
			v.AddError("duration", durationParamMessage)
			h.respond.FailedValidation(w, r, v.Errors)
			return
		}
		duration = parsed
	}

	day, found, err := h.svc.NextAvailableDate(r.Context(), slug, from, duration)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	var date *string
	if found {
		formatted := day.Format("2006-01-02")
		date = &formatted
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"date": date})
}
