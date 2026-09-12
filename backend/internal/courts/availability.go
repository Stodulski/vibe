package courts

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// defaultAvailabilityDuration is the booking length the availability grid
// generates slots for when the caller does not name one.
//
// It matches slots.DefaultDuration, kept as its own constant here because this
// is a query-parameter default rather than the grid arithmetic itself.
const defaultAvailabilityDuration = slots.DefaultDuration

// durationParamMessage is what a caller reading a bad `duration` query param
// is told, derived from slots.PermittedDurations the same way the courts and
// bookings handlers derive their own duration refusals.
var durationParamMessage = func() string {
	d := slots.PermittedDurations()
	parts := make([]string, len(d))
	for i, m := range d {
		parts[i] = strconv.Itoa(m)
	}
	return "duration must be " + strings.Join(parts[:len(parts)-1], ", ") + ", or " + parts[len(parts)-1]
}()

// Availability handles GET /api/v1/public/complexes/{slug}/availability,
// returning the bookable grid for one day.
func (h *Handler) Availability(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	qs := r.URL.Query()
	dateStr := httpx.ReadString(qs, "date", "")
	if dateStr == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("date query parameter is required"))
		return
	}

	date, err := timezone.ParseDay(dateStr)
	if err != nil {
		h.respond.BadRequest(w, r, fmt.Errorf("date must be in YYYY-MM-DD format"))
		return
	}

	// A day that has already ended has nothing left to book. This mirrors the
	// past-date refusal POST /api/v1/book gives, anchored to the same
	// product wall-clock, so the storefront never advertises a grid for a
	// date the booking write would immediately reject.
	if date.Before(timezone.Today()) {
		v := validator.New()
		v.AddError("date", "date must not be in the past")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// duration is the booking length the grid is rendered for. It defaults to
	// slots.DefaultDuration, the same default a booking gets when nothing
	// chooses a length for it, and is otherwise restricted to the same set the
	// write paths accept — so the storefront can never publish a length
	// neither booking handler would take back.
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

	availability, err := h.svc.Availability(r.Context(), slug, dateStr, date, duration)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"availability": availability})
}
