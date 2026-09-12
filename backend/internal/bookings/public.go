package bookings

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/validator"
)

// linkExpiredMessage is the 410 body's recourse copy for all three public
// routes (specs/booking-link-credential's "distinguishable, actionable
// response" scenario): non-empty, distinguishable from the 404 an unknown
// token gets, and it names a next step. No self-service reissue path exists
// (proposal's Out of Scope), so the recourse is contacting the venue.
const linkExpiredMessage = "this link has expired; contact the complex directly to check on your booking"

// venueGoneMessage is H-18's fix direction applied: a booking link can be
// perfectly live and still point at a venue (or, more narrowly, a single
// court of one still-open venue) that has since been soft-deleted. That is
// not the same fact as the token itself being unknown or expired, and the
// client should not be told to keep retrying a request that will never
// succeed. It follows linkExpiredMessage's lead — non-empty, distinguishable
// from a bare 404/500, and naming a next step — because that is the only
// hand-written, deliberately actionable body this package already has for
// exactly this class of dead end; a second copy of the same shape would be
// the one that drifts.
const venueGoneMessage = "the venue for this booking is no longer available; contact them directly if you need to follow up"

// PublicBook handles POST /api/v1/book, the flow a client uses with no account.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) PublicBook(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ComplexID       string `json:"complex_id"`
		CourtID         string `json:"court_id"`
		Date            string `json:"date"`
		StartTime       string `json:"start_time"`
		DurationMinutes int    `json:"duration_minutes"`
		ClientFirstName string `json:"client_first_name"`
		ClientLastName  string `json:"client_last_name"`
		ClientPhone     string `json:"client_phone"`
		ClientEmail     string `json:"client_email"`
		ClientNotes     string `json:"client_notes"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.ComplexID != "", "complex_id", "must be provided")
	v.Check(input.CourtID != "", "court_id", "must be provided")
	v.Check(input.Date != "", "date", "must be provided")
	v.Check(input.StartTime != "", "start_time", "must be provided")
	v.Check(input.ClientFirstName != "", "client_first_name", "must be provided")
	// H-16: a 10,000-character client_first_name used to be accepted outright
	// — nothing checked how long it was, only that it was non-empty. The value
	// goes into clients, into the booking, into every notification payload,
	// and out to MercadoPago as the payer name, so an unbounded free-text
	// field here reaches a third-party API and the owner's dashboard from an
	// endpoint that needs no account. 100 matches the bound this codebase
	// already puts on other short name fields (e.g. courts.Create's own
	// "name").
	v.Check(len(input.ClientFirstName) <= 100, "client_first_name", "must not be more than 100 characters")
	v.Check(input.ClientLastName != "", "client_last_name", "must be provided")
	v.Check(len(input.ClientLastName) <= 100, "client_last_name", "must not be more than 100 characters")
	v.Check(input.ClientPhone != "", "client_phone", "must be provided")
	if input.ClientPhone != "" {
		normalized, nerr := validator.NormalizePhone(input.ClientPhone)
		if nerr != nil {
			v.AddError("client_phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			input.ClientPhone = normalized
		}
	}
	// Optional: every notification below already skips an empty address, and
	// the payer email on the MercadoPago preference is sent only when present.
	if input.ClientEmail != "" {
		// 254 is RFC 5321's own practical ceiling on a full email address
		// (local-part@domain, both bounded), and EmailRX's character classes
		// alone do not bound the total length.
		v.Check(len(input.ClientEmail) <= 254, "client_email", "must not be more than 254 characters")
		v.Check(validator.Matches(input.ClientEmail, validator.EmailRX), "client_email", "must be a valid email address")
	}
	if input.StartTime != "" {
		v.Check(slots.ValidFormat(input.StartTime), "start_time", "must be in HH:MM format (00:00-23:59)")
	}

	v.Check(validator.PermittedValue(input.DurationMinutes, slots.PermittedDurations()...), "duration_minutes", durationMessage)
	v.Check(len(input.ClientNotes) <= 2000, "client_notes", "must not exceed 2000 characters")

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	complexID, err := uuid.Parse(input.ComplexID)
	if err != nil {
		v.AddError("complex_id", "must be a valid UUID")
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

	result, err := h.svc.PublicBook(r.Context(), h.actor(r), PublicBookInput{
		ComplexID:       complexID,
		CourtID:         courtID,
		Date:            date,
		RawDate:         input.Date,
		StartTime:       input.StartTime,
		DurationMinutes: input.DurationMinutes,
		ClientFirstName: input.ClientFirstName,
		ClientLastName:  input.ClientLastName,
		ClientPhone:     input.ClientPhone,
		ClientEmail:     input.ClientEmail,
		ClientNotes:     input.ClientNotes,
	})
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	booking, complex, court := result.Booking, result.Complex, result.Court

	// The response never carries the booking's primary key
	// (specs/booking-link-credential): only the fields the frontend needs to
	// render a confirmation, plus the token that now authorizes the three
	// public routes.
	response := httpx.Envelope{
		"booking": httpx.Envelope{
			"status":            booking.Status,
			"collection_status": booking.CollectionStatus,
			"refund_status":     booking.RefundStatus,
			"date":              booking.Date.Format("2006-01-02"),
			"start_time":        booking.StartTime,
			"starts_at":         booking.StartsAt.Format(time.RFC3339),
			"ends_at":           booking.EndsAt.Format(time.RFC3339),
			"court_name":        court.Name,
			"complex_name":      complex.Name,
			"price":             booking.Price,
			"deposit_amount":    booking.DepositAmount,
		},
		"token": booking.LinkToken,
	}

	// MP retired the sandbox environment: whether a payment runs in test
	// or production mode is decided by the credential (test-user
	// APP_USR- tokens), not by the redirect URL. sandbox_init_point is
	// deprecated by MP; always use init_point.
	response["mp_init_point"] = result.Preference.InitPoint
	response["mp_preference_id"] = result.Preference.ID
	response["service_fee"] = result.ServiceFee
	response["total_client_pays"] = result.TotalClientPays

	h.respond.JSON(w, r, http.StatusCreated, response)
}

// PublicStatus handles GET /api/v1/book/status. It is public because the
// client has no account; the access token they were given is the credential
// (specs/booking-link-credential) — the booking's primary key authorizes
// nothing here.
//
// The public success page renders entirely from this response: a client who
// opens the MercadoPago redirect in a different browser, or opens the link
// later, has no cached copy of the booking to fall back on, so the response
// carries the whole picture rather than just the two status fields.
func (h *Handler) PublicStatus(w http.ResponseWriter, r *http.Request) {
	token := httpx.ReadString(r.URL.Query(), booklink.QueryParam, "")
	if token == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("%s query parameter is required", booklink.QueryParam))
		return
	}

	view, err := h.svc.PublicStatus(r.Context(), token)
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, publicStatusResponse(view))
}

// publicStatusResponse builds PublicStatus's payload. The payment is nil when
// the booking carries no payment row yet, in which case service_fee reads 0
// rather than failing the request.
func publicStatusResponse(view StatusView) httpx.Envelope {
	booking, complex, court := view.Booking, view.Complex, view.Court

	serviceFee := 0
	if view.Payment != nil {
		serviceFee = view.Payment.ServiceFee
	}

	remaining := booking.Price - booking.DepositAmount
	if remaining < 0 {
		remaining = 0
	}

	cancellation := httpx.Envelope{
		"can_cancel":         view.Cancellation.CanCancel,
		"can_refund_now":     view.Cancellation.WithinWindow,
		"cancellation_hours": complex.CancellationHours,
	}
	if view.Cancellation.CanCancel {
		cancellation["refund_deadline"] = view.Cancellation.Deadline.Format(time.RFC3339)
	} else {
		cancellation["refund_deadline"] = nil
	}

	bookingEnvelope := httpx.Envelope{
		"status":            booking.Status,
		"collection_status": booking.CollectionStatus,
		"refund_status":     booking.RefundStatus,
		"complex_name":      complex.Name,
		"complex_address":   complex.Address,
		"court_name":        court.Name,
		"sport":             court.Sport,
		"court_type":        court.CourtType,
		"date":              booking.Date.Format("2006-01-02"),
		"start_time":        booking.StartTime,
		"starts_at":         booking.StartsAt.Format(time.RFC3339),
		"ends_at":           booking.EndsAt.Format(time.RFC3339),
		"duration_minutes":  booking.DurationMinutes,
		"price":             booking.Price,
		"deposit_amount":    booking.DepositAmount,
		"service_fee":       serviceFee,
		"remaining_amount":  remaining,
		"cancellation":      cancellation,
	}
	if complex.Phone != "" {
		bookingEnvelope["complex_phone"] = complex.Phone
	}

	return httpx.Envelope{"booking": bookingEnvelope}
}

// PublicCancelInfo handles GET /api/v1/book/cancel-info, telling the client
// whether cancelling now would return their deposit.
func (h *Handler) PublicCancelInfo(w http.ResponseWriter, r *http.Request) {
	token := httpx.ReadString(r.URL.Query(), booklink.QueryParam, "")
	if token == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("%s query parameter is required", booklink.QueryParam))
		return
	}

	view, err := h.svc.PublicCancelInfo(r.Context(), token)
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, publicCancelInfoResponse(view))
}

// publicCancelInfoResponse builds PublicCancelInfo's payload: the same booking
// detail the success page shows, plus the money actually at stake if the client
// cancels right now.
func publicCancelInfoResponse(view CancelInfoView) httpx.Envelope {
	booking, complex, court := view.Booking, view.Complex, view.Court

	bookingEnvelope := httpx.Envelope{
		"status":           booking.Status,
		"date":             booking.Date.Format("2006-01-02"),
		"start_time":       booking.StartTime,
		"starts_at":        booking.StartsAt.Format(time.RFC3339),
		"ends_at":          booking.EndsAt.Format(time.RFC3339),
		"duration_minutes": booking.DurationMinutes,
		"court_name":       court.Name,
		"sport":            court.Sport,
		"court_type":       court.CourtType,
		"complex_name":     complex.Name,
	}
	if complex.Address != "" {
		bookingEnvelope["complex_address"] = complex.Address
	}

	return httpx.Envelope{
		"booking":    bookingEnvelope,
		"can_cancel": view.Cancellation.CanCancel,
		"can_refund": view.CanRefund,
		// How it would come back: "mercadopago" is automatic, "manual" means the
		// complex has to hand it over, "none" means there is nothing to return.
		"refund_method":      view.RefundMethod,
		"cancellation_hours": complex.CancellationHours,
		// refund_amount is what an automatic MercadoPago refund would return if
		// the client cancels right now, computed row by row the same way the
		// automatic refund itself does.
		"refund_amount": view.RefundAmount,
		// paid_amount is what has actually been paid so far, regardless of
		// whether any of it comes back — so the page can say what was paid even
		// when cancelling now returns nothing.
		"paid_amount": view.PaidAmount,
	}
}

// PublicCancel handles POST /api/v1/book/cancel.
//
// Outside the refund window the booking is still cancelled — a client should
// not have to show up — but the deposit is not returned.
func (h *Handler) PublicCancel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}

	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Token != "", "token", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	result, err := h.svc.PublicCancel(r.Context(), h.actor(r), input.Token)
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	// Return minimal info — don't expose internal booking details to public endpoint.
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": httpx.Envelope{
			"status":            result.Booking.Status,
			"collection_status": result.Booking.CollectionStatus,
			"refund_status":     refundStatusAfter(result.Booking, result.Outcome),
		},
		// Kept for the clients already reading it, and now true when the money
		// actually came back rather than never. It stays true even when
		// "refund_status" above reads "partial": the automatic half genuinely
		// came back, and "manual_amount" in "refund" below is what says a
		// cash/transfer balance is still owed.
		"refunded": result.Outcome.MoneyReturned(),
		"refund":   refundEnvelope(result.Outcome),
	})
}
