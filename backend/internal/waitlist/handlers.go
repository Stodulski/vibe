package waitlist

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// maxEmailLength is RFC 5321's limit on an address, the same bound the leads
// endpoint and the booking form apply.
const maxEmailLength = 254

// Handler answers the public waitlist endpoint.
type Handler struct {
	svc     *Service
	respond *httpx.Responder
}

// NewHandler returns a Handler over the given service.
func NewHandler(svc *Service, respond *httpx.Responder) *Handler {
	return &Handler{svc: svc, respond: respond}
}

// Join handles POST /api/v1/public/complexes/{slug}/waitlist. It answers 204
// whether or not the entry was new, so a repeated join looks the same as the
// first.
func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		Date  string `json:"date"`
		Email string `json:"email"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	input.Date = strings.TrimSpace(input.Date)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	v := validator.New()
	day, parseErr := timezone.ParseDay(input.Date)
	v.Check(parseErr == nil, "date", "must be a date in YYYY-MM-DD format")
	if parseErr == nil {
		today := timezone.Today()
		v.Check(!day.Before(today), "date", "must not be in the past")
		v.Check(!day.After(today.AddDate(0, 0, bookingstore.MaxBookingHorizonDays)), "date",
			fmt.Sprintf("must not be more than %d days in the future", bookingstore.MaxBookingHorizonDays))
	}
	v.Check(input.Email != "", "email", "must be provided")
	v.Check(len(input.Email) <= maxEmailLength, "email", "must not be more than 254 characters")
	v.Check(validator.Matches(input.Email, validator.EmailRX), "email", "must be a valid email address")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	err := h.svc.Join(r.Context(), slug, day, input.Email)
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		h.respond.NotFound(w, r)
	case err != nil:
		h.respond.ServerError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
