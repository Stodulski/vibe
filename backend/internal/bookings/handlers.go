package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/validator"
)

// List handles GET /api/v1/complexes/:id/bookings for one day. The date is
// required rather than defaulted, so the dashboard always gets the day it
// asked for.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
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

	statusFilter := httpx.ReadString(qs, "status", "")
	searchFilter := httpx.ReadString(qs, "search", "")

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

	bookings, metadata, err := h.store.GetByComplex(r.Context(), complex.ID, date, date, filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidCursor):
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// Filter by status and/or client name/phone in handler (volumes per day are small).
	if statusFilter != "" || searchFilter != "" {
		searchLower := strings.ToLower(searchFilter)
		filtered := make([]*data.Booking, 0, len(bookings))
		for _, b := range bookings {
			if statusFilter != "" && b.Status != statusFilter {
				continue
			}
			if searchFilter != "" &&
				!strings.Contains(strings.ToLower(b.ClientName), searchLower) &&
				!strings.Contains(b.ClientPhone, searchFilter) {
				continue
			}
			filtered = append(filtered, b)
		}
		bookings = filtered
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

	booking, err := h.store.GetByID(r.Context(), bookingID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	if booking.ComplexID != complex.ID {
		h.respond.NotFound(w, r)
		return
	}

	client, err := h.clients.GetByID(r.Context(), booking.ClientID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		h.respond.ServerError(w, r, err)
		return
	}

	payment, err := h.payments.GetByBookingID(r.Context(), booking.ID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		h.respond.ServerError(w, r, err)
		return
	}

	// The whole ledger, not just the single (MercadoPago-preferred) row above:
	// a booking can carry more than one payment row (a deposit paid online plus
	// a balance confirmed in cash), and the detail view has to show every one of
	// them. "payment" stays as-is for backward compatibility.
	payments, err := h.payments.ListByBookingID(r.Context(), booking.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if payments == nil {
		payments = []*paymentstore.Payment{}
	}

	response := httpx.Envelope{"booking": booking, "payments": payments}
	if client != nil {
		response["client"] = client
	}
	if payment != nil {
		response["payment"] = payment
	}

	h.respond.JSON(w, r, http.StatusOK, response)
}

// Update handles PUT /api/v1/complexes/:id/bookings/:bookingID. Every field is
// optional; an omitted one keeps its current value.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
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

	booking, err := h.store.GetByID(r.Context(), bookingID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	if booking.ComplexID != complex.ID {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		Status           *string `json:"status"`
		CollectionStatus *string `json:"collection_status"`
		RefundStatus     *string `json:"refund_status"`
		Notes            *string `json:"notes"`
	}

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	// No version field: the request carries no optimistic-concurrency token
	// because the column behind it was removed with the version counter. What refuses a
	// write that races a bulk cancel or a payment confirmation is the
	// bookings_forbid_status_reversal trigger, in the database, for every
	// writer — the version check provably never fired on that path.
	v := validator.New()

	if input.Status != nil {
		validStatuses := map[string]bool{"pending": true, "confirmed": true, "cancelled": true, "completed": true, "no_show": true}
		v.Check(validStatuses[*input.Status], "status", "must be pending, confirmed, cancelled, completed, or no_show")

		// Validate state transitions.
		allowedTransitions := map[string]map[string]bool{
			"pending":   {"confirmed": true, "cancelled": true},
			"confirmed": {"cancelled": true, "completed": true, "no_show": true},
			"cancelled": {},
			"completed": {"no_show": true},
			"no_show":   {},
		}
		statusLabels := map[string]string{
			"pending":   "pendiente",
			"confirmed": "confirmada",
			"cancelled": "cancelada",
			"completed": "completada",
			"no_show":   "ausente",
		}
		if allowed, ok := allowedTransitions[booking.Status]; ok {
			if !allowed[*input.Status] && *input.Status != booking.Status {
				fromLabel := statusLabels[booking.Status]
				toLabel := statusLabels[*input.Status]
				v.AddError("status", fmt.Sprintf("cannot change from '%s' to '%s'", fromLabel, toLabel))
			}
		}

		// Prevent cancelling a paid booking via generic update — use the cancel or refund endpoint instead.
		//
		// Money was collected AND nothing is being given back yet, which is the
		// pair the single payment_status enum spelled 'deposit_paid' or
		// 'fully_paid'. Both halves are load-bearing: a booking already mid- or
		// post-refund was cancellable here before the payment_status split and still is,
		// because the refund pipeline has already decided what happens to that
		// money.
		if *input.Status == "cancelled" &&
			booking.CollectionStatus != data.CollectionStatusUnpaid &&
			booking.RefundStatus == data.RefundStatusNone {
			h.respond.BadRequest(w, r, fmt.Errorf("cannot cancel a paid booking via update, use the /cancel endpoint"))
			return
		}

		booking.Status = *input.Status
	}
	if input.CollectionStatus != nil {
		validCollectionStatuses := map[string]bool{
			data.CollectionStatusUnpaid:      true,
			data.CollectionStatusDepositPaid: true,
			data.CollectionStatusFullyPaid:   true,
		}
		v.Check(validCollectionStatuses[*input.CollectionStatus], "collection_status",
			"must be unpaid, deposit_paid or fully_paid")

		// Collection only moves upward. This is the half of the old
		// validPaymentTransitions matrix that lived on the money-in axis
		// (unpaid -> deposit_paid/fully_paid, deposit_paid -> fully_paid); the
		// refund half is the matrix below. The database refuses the return to
		// 'unpaid' as well, in bookings_forbid_status_reversal — this is the
		// policy above that floor, and it also refuses fully_paid ->
		// deposit_paid, which the floor deliberately does not (see the bookings
		// section of db/migrations/001_init.sql for why the floor stops short).
		validCollectionTransitions := map[string]map[string]bool{
			data.CollectionStatusUnpaid:      {data.CollectionStatusDepositPaid: true, data.CollectionStatusFullyPaid: true},
			data.CollectionStatusDepositPaid: {data.CollectionStatusFullyPaid: true},
			data.CollectionStatusFullyPaid:   {},
		}
		collectionLabels := map[string]string{
			data.CollectionStatusUnpaid:      "sin pago",
			data.CollectionStatusDepositPaid: "seña pagada",
			data.CollectionStatusFullyPaid:   "pago completo",
		}
		if allowed, ok := validCollectionTransitions[booking.CollectionStatus]; ok {
			if !allowed[*input.CollectionStatus] && *input.CollectionStatus != booking.CollectionStatus {
				h.respond.Error(w, r, http.StatusConflict, fmt.Sprintf(
					"cannot change payment status from '%s' to '%s'",
					collectionLabels[booking.CollectionStatus], collectionLabels[*input.CollectionStatus]))
				return
			}
		}

		booking.CollectionStatus = *input.CollectionStatus
	}
	if input.RefundStatus != nil {
		// 'partial' is deliberately absent from validRefundStatuses: it names a
		// booking whose MercadoPago-backed rows were refunded automatically and
		// whose cash/transfer rows are still owed by hand, and only the refund
		// pipeline that computed that split may set it
		// (internal/payments/refund.go). It IS a key of the transition matrix
		// below, though, and that is not an oversight — before the equivalent
		// entry existed on the merged enum, a booking sitting in that state
		// matched no key at all, `allowed, ok := ...` left ok false, the whole
		// guard fell through, and a generic edit could move it anywhere,
		// including back to fully collected with the manual balance still
		// unpaid. The only transition a staff edit may make from 'partial' is
		// the one the manual-refund endpoint makes, to 'full', once the owner
		// confirms the cash portion was actually returned.
		validRefundStatuses := map[string]bool{
			data.RefundStatusNone:    true,
			data.RefundStatusPending: true,
			data.RefundStatusFull:    true,
		}
		v.Check(validRefundStatuses[*input.RefundStatus], "refund_status",
			"must be none, pending or full")

		validRefundTransitions := map[string]map[string]bool{
			data.RefundStatusNone:    {data.RefundStatusPending: true},
			data.RefundStatusPending: {data.RefundStatusFull: true},
			data.RefundStatusPartial: {data.RefundStatusFull: true},
			data.RefundStatusFull:    {},
		}
		refundLabels := map[string]string{
			data.RefundStatusNone:    "sin reembolso",
			data.RefundStatusPending: "reembolso pendiente",
			data.RefundStatusPartial: "reembolso parcial",
			data.RefundStatusFull:    "reembolsado",
		}
		if allowed, ok := validRefundTransitions[booking.RefundStatus]; ok {
			if !allowed[*input.RefundStatus] && *input.RefundStatus != booking.RefundStatus {
				h.respond.Error(w, r, http.StatusConflict, fmt.Sprintf(
					"cannot change payment status from '%s' to '%s'",
					refundLabels[booking.RefundStatus], refundLabels[*input.RefundStatus]))
				return
			}
		}

		booking.RefundStatus = *input.RefundStatus
	}
	if input.Notes != nil {
		booking.Notes = input.Notes
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	err = h.store.Update(r.Context(), booking)
	if err != nil {
		switch {
		// The row was deleted between the read above and this write. Same 409
		// the other Update handlers give for it.
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.EditConflict(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.record(r, complex.ID, "update", &booking.ID, booking)

	// Increment no_shows counter when owner marks a booking as no_show.
	if input.Status != nil && *input.Status == "no_show" {
		if err := h.clients.IncrementNoShows(r.Context(), booking.ClientID); err != nil {
			h.logger.Error("update booking: failed to increment no_shows", "error", err, "client_id", booking.ClientID)
		}
	}

	h.realtime.PublishBookingChanged(complex.ID)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"booking": booking})
}

// unpricedSlotMessage is what both write paths tell a client who asked for a
// slot no price rule covers. It is a business answer, not a fault: the court is
// configured and the complex is open, and this particular position is simply
// not for sale.
const unpricedSlotMessage = "the selected time has no price configured and cannot be booked"

// findPrice answers what one slot costs, and refuses when nothing prices it.
//
// It used to answer that question with a fallback ladder of its own — the band
// covering the slot, then any band for that weekday, then whichever price row
// the query happened to return first — while the availability grid answered the
// same question with pricing.SlotPrice, which has no fallback at all. Two
// answers to one question, and they disagreed exactly where it costs money: a
// slot no band covers is omitted from the storefront and was sold here at the
// first band for that weekday, which under GetCourtPrices' `ORDER BY day_type,
// time_from` is the earliest — the base band. A court whose peak window stops
// short of closing time therefore charged off-peak for peak hours, and a court
// with no bands for that weekday at all charged another weekday's price
// outright. The number the client was shown was never the number they paid.
//
// The overlap half of that defect is already closed in the schema: the
// court_prices_no_overlapping_rule exclusion constraint makes at most one
// band cover any given minute, which is what makes a first-match loop correct
// rather than merely repeatable. What was left was the fallback, and the fix is
// to stop having a second implementation to fall back in.
//
// pricing.ErrNoPriceRule travels out unwrapped so both callers can answer 409
// rather than 500.
//
// It prices the whole booking span, not one slot: court_prices.price is now an
// hourly rate, and a booking's chosen duration can cross a band boundary the
// same way a run of consecutive slots used to.
func (h *Handler) findPrice(r *http.Request, complexID, courtID uuid.UUID, date time.Time, startTime string, durationMinutes int) (int, error) {
	prices, err := h.courts.GetPrices(r.Context(), courtID)
	if err != nil {
		return 0, err
	}

	// Which window sold this hour decides which rate card prices it. For a
	// venue trading past midnight that is not the calendar day: an hour at
	// 00:30 came out of the previous night's window while that window was
	// still open, and pays that night's rate. See priceWindow.
	schedules, err := h.complexes.GetSchedules(r.Context(), complexID)
	if err != nil {
		return 0, err
	}
	window := priceWindow(schedules, date, startTime)
	return pricing.BookingPrice(prices, window.Day, window.StartMin, durationMinutes)
}
