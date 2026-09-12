package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/validator"
)

// List handles GET /api/v1/complexes/{id}/bookings for one day. The date is
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
		"bookings": toGenBookings(bookings),
		"metadata": toGenMetadata(metadata),
	})
}

// Get handles GET /api/v1/complexes/{id}/bookings/{bookingID}.
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
	response := httpx.Envelope{"booking": toGenBooking(detail.Booking), "payments": toGenPayments(detail.Payments)}
	if detail.Client != nil {
		response["client"] = toGenClient(detail.Client)
	}
	if detail.Payment != nil {
		response["payment"] = toGenPayment(detail.Payment)
	}

	h.respond.JSON(w, r, http.StatusOK, response)
}

// Update handles PUT /api/v1/complexes/{id}/bookings/{bookingID}. Every field is
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

	var input gen.BookingsUpdateJSONBody

	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	booking, err := h.svc.Update(r.Context(), complex.ID, h.actor(r), bookingID, UpdateInput{
		Status:           ptrString(input.Status),
		CollectionStatus: ptrString(input.CollectionStatus),
		RefundStatus:     ptrString(input.RefundStatus),
		Notes:            input.Notes,
	})
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"booking": toGenBooking(booking)})
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
		// A ConflictError carries its own sentence, built from the booking it
		// collided with, so it cannot live in a table keyed on identity.
		h.respond.Refuse(w, r, httpx.Conflict(conflictErr.Message))
	case errors.Is(err, bookingstore.ErrDuplicateBooking),
		errors.Is(err, bookingstore.ErrSlotUnavailable):
		// Somebody else now holds those hours. A business answer, not a fault,
		// and the same one the shared edit conflict gets.
		h.respond.EditConflict(w, r)
	case errors.Is(err, ErrNoActor):
		h.respond.InvalidAuthenticationToken(w, r)
	default:
		h.respond.DomainError(w, r, err)
	}
}

// slotTakenMessage is what both booking write paths tell a client whose chosen
// hours somebody else now holds.
const slotTakenMessage = "the selected time slot is no longer available, please choose another"

// toGenBooking maps a store booking onto the generated wire type (HTTP-08): a
// handler must never serialize bookingstore.Booking directly, because that
// would leak RefundIntentAt and LinkToken the moment anyone forgets to keep
// their json:"-" tags in sync by hand. Every field on the wire today is
// reproduced here by name.
//
// CourtName, ClientName and ClientPhone are enrichment fields the store
// always populates (possibly with an empty string when nothing joined) and
// the wire has always carried them unconditionally; pointers are taken here
// rather than left nil so the generated type's omitempty never drops them.
func toGenBooking(b *bookingstore.Booking) gen.Booking {
	courtName := b.CourtName
	clientName := b.ClientName
	clientPhone := b.ClientPhone

	return gen.Booking{
		ClientId:         b.ClientID,
		ClientName:       &clientName,
		ClientPhone:      &clientPhone,
		CollectionStatus: gen.CollectionStatus(b.CollectionStatus),
		ComplexId:        b.ComplexID,
		CourtId:          b.CourtID,
		CourtName:        &courtName,
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

// toGenBookings maps a page of store bookings, preserving nil-vs-empty
// exactly as the store handed it in: a nil slice stays `null` on the wire and
// a non-nil, possibly zero-length one stays `[]`, matching whatever encoding
// the direct-serialization code produced before this mapper existed.
func toGenBookings(bookings []*bookingstore.Booking) []gen.Booking {
	if bookings == nil {
		return nil
	}
	out := make([]gen.Booking, len(bookings))
	for i, b := range bookings {
		out[i] = toGenBooking(b)
	}
	return out
}

// toGenMetadata maps a page's pagination metadata onto the generated wire
// type. data.Metadata already omits an empty cursor and a zero total count
// through its own json tags; the pointers here reproduce that same omission
// on the generated type rather than changing it.
func toGenMetadata(m data.Metadata) gen.Metadata {
	out := gen.Metadata{HasMore: m.HasMore}
	if m.NextCursor != "" {
		cursor := m.NextCursor
		out.NextCursor = &cursor
	}
	if m.TotalCount != 0 {
		total := m.TotalCount
		out.TotalCount = &total
	}
	return out
}

// toGenClient maps a store client onto the generated wire type (HTTP-08).
// Email needs no conversion: the OpenAPI document declares it a plain string
// (format: email was dropped, since it made oapi-codegen emit
// openapi_types.Email, which fails to marshal any stored value that is not a
// valid net/mail address).
func toGenClient(c *clientstore.Client) gen.Client {
	return gen.Client{
		ComplexId:     c.ComplexID,
		CreatedAt:     c.CreatedAt,
		Email:         c.Email,
		FirstName:     c.FirstName,
		Id:            c.ID,
		IsBlocked:     c.IsBlocked,
		LastName:      c.LastName,
		NoShows:       c.NoShows,
		Notes:         c.Notes,
		Phone:         c.Phone,
		TotalBookings: c.TotalBookings,
		UpdatedAt:     c.UpdatedAt,
	}
}

// toGenPayment maps a store payment onto the generated wire type (HTTP-08).
func toGenPayment(p *paymentstore.Payment) gen.Payment {
	return gen.Payment{
		Amount:         p.Amount,
		BookingId:      p.BookingID,
		ComplexId:      p.ComplexID,
		CreatedAt:      p.CreatedAt,
		Id:             p.ID,
		Method:         gen.PaymentMethod(p.Method),
		MpPaymentId:    p.MPPaymentID,
		MpPreferenceId: p.MPPreferenceID,
		RefundAmount:   p.RefundAmount,
		ServiceFee:     p.ServiceFee,
		Status:         gen.PaymentStatus(p.Status),
		StatusDetail:   p.StatusDetail,
		UpdatedAt:      p.UpdatedAt,
	}
}

// toGenPayments maps a booking's whole payment ledger, preserving
// nil-vs-empty the same way toGenBookings does.
func toGenPayments(payments []*paymentstore.Payment) []gen.Payment {
	if payments == nil {
		return nil
	}
	out := make([]gen.Payment, len(payments))
	for i, p := range payments {
		out[i] = toGenPayment(p)
	}
	return out
}

// ptrString reads an optional generated enum/string field (a pointer to a
// named string type) as the plain *string the service layer's input structs
// take. A nil pointer stays nil; nothing here changes what "the field was
// omitted" means on the wire.
func ptrString[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}
