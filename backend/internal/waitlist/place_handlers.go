package waitlist

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/turnstile"
	"github.com/stodulski/vibe-server/internal/validator"
)

// maxSlotMinutes is the longest slot a person can wait for: the longest an
// open-play event may last, far beyond any booking the grid sells.
const maxSlotMinutes = 720

// clockRX is a time of day as the storefront grid writes it, "HH:MM".
var clockRX = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// PlaceHandler answers the public place-waitlist endpoint.
type PlaceHandler struct {
	svc     *PlaceService
	respond *httpx.Responder
	gate    *turnstile.Gate
}

// NewPlaceHandler returns a PlaceHandler over the given service. gate is the
// storefront's bot check, the same one the open-play and tournament sign-ups
// ask; a nil gate lets every request through.
func NewPlaceHandler(svc *PlaceService, respond *httpx.Responder, gate *turnstile.Gate) *PlaceHandler {
	return &PlaceHandler{svc: svc, respond: respond, gate: gate}
}

// Join handles POST /api/v1/public/complexes/{slug}/waitlist/places. It answers
// 204 whether or not the entry was new, so a repeated join looks the same as
// the first.
//
//nolint:funlen // one flat validation per kind; splitting it hides which field each rule names
func (h *PlaceHandler) Join(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		Email           string  `json:"email"`
		TurnstileToken  *string `json:"turnstile_token"`
		Kind            string  `json:"kind"`
		EventID         string  `json:"event_id"`
		CategoryID      string  `json:"category_id"`
		CourtID         string  `json:"court_id"`
		Date            string  `json:"date"`
		StartTime       string  `json:"start_time"`
		DurationMinutes int     `json:"duration_minutes"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	target := PlaceTarget{Kind: strings.TrimSpace(input.Kind)}

	v := validator.New()
	v.Check(email != "", "email", "must be provided")
	v.Check(len(email) <= maxEmailLength, "email", "must not be more than 254 characters")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")

	uuidField := func(field, value string) uuid.UUID {
		id, err := uuid.Parse(strings.TrimSpace(value))
		v.Check(err == nil, field, "must be a valid UUID")
		return id
	}
	switch target.Kind {
	case KindOpenPlay:
		target.EventID = uuidField("event_id", input.EventID)
	case KindTournamentCategory:
		target.CategoryID = uuidField("category_id", input.CategoryID)
	case KindSlot:
		target.CourtID = uuidField("court_id", input.CourtID)
		day, err := timezone.ParseDay(strings.TrimSpace(input.Date))
		v.Check(err == nil, "date", "must be a date in YYYY-MM-DD format")
		if err == nil {
			today := timezone.Today()
			v.Check(!day.Before(today), "date", "must not be in the past")
			v.Check(!day.After(today.AddDate(0, 0, bookingstore.MaxBookingHorizonDays)), "date",
				fmt.Sprintf("must not be more than %d days in the future", bookingstore.MaxBookingHorizonDays))
		}
		target.Day = day
		target.StartTime = strings.TrimSpace(input.StartTime)
		v.Check(validator.Matches(target.StartTime, clockRX), "start_time", "must be a time in HH:MM format")
		target.DurationMinutes = input.DurationMinutes
		v.Check(input.DurationMinutes > 0, "duration_minutes", "must be greater than zero")
		v.Check(input.DurationMinutes <= maxSlotMinutes, "duration_minutes",
			fmt.Sprintf("must not be more than %d", maxSlotMinutes))
	default:
		v.AddError("kind", "must be one of open_play, tournament_category, slot")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}
	// The bot check runs after the cheap validation, before anything is stored.
	token := ""
	if input.TurnstileToken != nil {
		token = *input.TurnstileToken
	}
	if !h.gate.Check(w, r, token) {
		return
	}

	err := h.svc.Join(r.Context(), slug, target, email)
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		h.respond.NotFound(w, r)
	case errors.Is(err, ErrPlaceFree):
		h.refuse(w, r, placeField(target.Kind), "is free now: book it instead of waiting")
	case errors.Is(err, ErrNotOnGrid):
		h.refuse(w, r, "start_time", "is not a slot the court sells on that day for that duration")
	case errors.Is(err, ErrPlaceStarted):
		h.refuse(w, r, "start_time", "must be in the future")
	case errors.Is(err, ErrTooManyWaits):
		h.refuse(w, r, "email", fmt.Sprintf("is already waiting for %d places at this complex", MaxWaitingPerEmail))
	case err != nil:
		h.respond.ServerError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// refuse answers a join the service turned down as a 422 naming field.
func (h *PlaceHandler) refuse(w http.ResponseWriter, r *http.Request, field, message string) {
	v := validator.New()
	v.AddError(field, message)
	h.respond.FailedValidation(w, r, v.Errors)
}

// placeField is the request field that names the place of a kind.
func placeField(kind string) string {
	switch kind {
	case KindOpenPlay:
		return "event_id"
	case KindTournamentCategory:
		return "category_id"
	default:
		return "start_time"
	}
}
