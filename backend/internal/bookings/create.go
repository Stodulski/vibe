package bookings

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Create handles POST /api/v1/complexes/:id/bookings, the owner booking from
// the dashboard. Unlike the public flow there is no payment step: the owner
// is trusted, so the booking is confirmed immediately.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input struct {
		CourtID         string `json:"court_id"`
		Date            string `json:"date"`
		StartTime       string `json:"start_time"`
		ClientPhone     string `json:"client_phone"`
		ClientEmail     string `json:"client_email"`
		ClientFirstName string `json:"client_first_name"`
		ClientLastName  string `json:"client_last_name"`
		PaymentMethod   string `json:"payment_method"`
		Notes           string `json:"notes"`
		DurationMinutes int    `json:"duration_minutes"`
		PaymentOption   string `json:"payment_option"`
		DepositAmount   *int   `json:"deposit_amount"`
		Price           *int   `json:"price"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.CourtID != "", "court_id", "must be provided")
	v.Check(input.Date != "", "date", "must be provided")
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
	v.Check(input.PaymentMethod == "" || input.PaymentMethod == "cash" || input.PaymentMethod == "transfer",
		"payment_method", "must be cash or transfer")
	v.Check(input.PaymentOption == "" || input.PaymentOption == "unpaid" || input.PaymentOption == "deposit" || input.PaymentOption == "full",
		"payment_option", "must be unpaid, deposit, or full")
	v.Check(validator.PermittedValue(input.DurationMinutes, slots.PermittedDurations()...), "duration_minutes", durationMessage)
	if input.DepositAmount != nil {
		v.Check(*input.DepositAmount > 0, "deposit_amount", "must be greater than 0")
	}
	if input.Price != nil {
		v.Check(*input.Price >= 0, "price", "must not be negative")
	}
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
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		v.AddError("date", "must be in YYYY-MM-DD format")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	booking, err := h.svc.Create(r.Context(), complex, h.actor(r), CreateInput{
		CourtID:         courtID,
		Date:            date,
		StartTime:       input.StartTime,
		DurationMinutes: input.DurationMinutes,
		ClientFirstName: input.ClientFirstName,
		ClientLastName:  input.ClientLastName,
		ClientPhone:     input.ClientPhone,
		ClientEmail:     input.ClientEmail,
		Notes:           input.Notes,
		PaymentMethod:   input.PaymentMethod,
		PaymentOption:   input.PaymentOption,
		DepositAmount:   input.DepositAmount,
		Price:           input.Price,
	})
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"booking": booking})
}
