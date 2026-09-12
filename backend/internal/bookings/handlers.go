package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// List handles GET /api/v1/complexes/:id/bookings for one day. The date is
// required rather than defaulted, so the dashboard always gets the day it
// asked for.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	qs := r.URL.Query()
	dateStr := httpx.ReadString(qs, "date", "")
	if dateStr == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("date query parameter is required"))
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		h.respond.BadRequest(w, r, fmt.Errorf("date must be in YYYY-MM-DD format"))
		return
	}

	filters := data.Filters{
		Cursor: httpx.ReadString(qs, "cursor", ""),
		Limit:  httpx.ReadInt(qs, "limit", 50),
	}

	v := validator.New()
	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	bookings, metadata, err := h.svc.List(r.Context(), complex.ID, ListInput{
		Date:    date,
		Status:  httpx.ReadString(qs, "status", ""),
		Search:  httpx.ReadString(qs, "search", ""),
		Filters: filters,
	})
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidCursor):
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"bookings": bookings,
		"metadata": metadata,
	})
}

// Get handles GET /api/v1/complexes/:id/bookings/:bookingID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	bookingID, err := httpx.ReadUUIDParam(r, "bookingID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	detail, err := h.svc.Get(r.Context(), complex.ID, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// "payment" is the single MercadoPago-preferred row, kept as-is for
	// backward compatibility; "payments" is the whole ledger.
	response := httpx.Envelope{"booking": detail.Booking, "payments": detail.Payments}
	if detail.Client != nil {
		response["client"] = detail.Client
	}
	if detail.Payment != nil {
		response["payment"] = detail.Payment
	}

	h.respond.JSON(w, r, http.StatusOK, response)
}

// Update handles PUT /api/v1/complexes/:id/bookings/:bookingID. Every field is
// optional; an omitted one keeps its current value.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	bookingID, err := httpx.ReadUUIDParam(r, "bookingID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		Status           *string `json:"status"`
		CollectionStatus *string `json:"collection_status"`
		RefundStatus     *string `json:"refund_status"`
		Notes            *string `json:"notes"`
	}

	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	booking, err := h.svc.Update(r.Context(), complex.ID, h.actor(r), bookingID, UpdateInput{
		Status:           input.Status,
		CollectionStatus: input.CollectionStatus,
		RefundStatus:     input.RefundStatus,
		Notes:            input.Notes,
	})
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"booking": booking})
}

// refuse maps a domain error onto the response it has always produced.
//
// It is one switch rather than one per handler because every use case in this
// package refuses in the same vocabulary: a field the request got wrong (422),
// a state the booking is in (400), a collision with somebody else's work (409),
// a booking that is not this caller's (404), and everything else (500).
func (h *Handler) refuse(w http.ResponseWriter, r *http.Request, err error) {
	var fieldErr *FieldError
	var validationErr *ValidationError
	var conflictErr *ConflictError
	var stateErr *StateError

	switch {
	case errors.As(err, &validationErr):
		h.respond.FailedValidation(w, r, validationErr.Errors)
	case errors.As(err, &fieldErr):
		h.respond.FailedValidation(w, r, map[string]string{fieldErr.Field: fieldErr.Message})
	case errors.As(err, &stateErr):
		h.respond.BadRequest(w, r, stateErr)
	case errors.As(err, &conflictErr):
		h.respond.Error(w, r, http.StatusConflict, conflictErr.Message)
	case errors.Is(err, ErrEditConflict),
		errors.Is(err, bookingstore.ErrDuplicateBooking),
		errors.Is(err, bookingstore.ErrSlotUnavailable):
		// The row moved between the read and the write: either it was deleted,
		// or somebody else now holds those hours. Both are a business answer,
		// not a fault.
		h.respond.EditConflict(w, r)
	case errors.Is(err, ErrNoActor):
		h.respond.InvalidAuthenticationToken(w, r)
	case errors.Is(err, ErrSlotTaken):
		h.respond.Error(w, r, http.StatusConflict, slotTakenMessage)
	case errors.Is(err, ErrClientBlocked):
		h.respond.Error(w, r, http.StatusForbidden, "your account is blocked, contact the complex for more information")
	case errors.Is(err, ErrMercadoPagoNotConnected):
		h.respond.Error(w, r, http.StatusBadRequest, "the complex does not have MercadoPago connected, contact the complex")
	case errors.Is(err, ErrCheckoutUnavailable):
		h.respond.Error(w, r, http.StatusServiceUnavailable, "no se pudo crear el enlace de pago, intente nuevamente")
	case errors.Is(err, ErrVenueGone):
		h.respond.Error(w, r, http.StatusGone, venueGoneMessage)
	case errors.Is(err, ErrLinkExpired):
		h.respond.Error(w, r, http.StatusGone, linkExpiredMessage)
	case errors.Is(err, data.ErrRecordNotFound):
		h.respond.NotFound(w, r)
	default:
		h.respond.ServerError(w, r, err)
	}
}

// slotTakenMessage is what both booking write paths tell a client whose chosen
// hours somebody else now holds.
const slotTakenMessage = "the selected time slot is no longer available, please choose another"
