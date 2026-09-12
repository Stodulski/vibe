package bookings

import (
	"errors"
	"fmt"
	"net/http"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Cancel handles POST /api/v1/complexes/:id/bookings/:bookingID/cancel.
//
// Cancelling an already-cancelled booking answers 200 without doing anything,
// so a retried request cannot refund twice. Completed and no-show bookings
// are terminal and refused.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
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
		Reason string `json:"reason"`
	}

	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	result, err := h.svc.Cancel(r.Context(), complex, h.actor(r), bookingID, input.Reason)
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	// A retried cancellation answers from the row alone: this request produced
	// no refund outcome, so it names none.
	if result.AlreadyCancelled {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"booking": result.Booking})
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": result.Booking,
		"refund":  refundEnvelope(result.Outcome),
	})
}

// confirmPaymentRaceMessage is what the owner is told when the booking's status
// changed underneath a cash payment they were confirming at the counter.
//
// Both sentinels answer with it, unchanged in behaviour: an owner taking cash
// gets the same 409 whether the booking was cancelled or completed underneath
// them. The split between the two exists for the webhook, which has money to
// send back (H-23), not for this path, which has cash in a till.
const confirmPaymentRaceMessage = "this booking's status changed before the payment could be confirmed, refresh and check it"

// ConfirmPayment handles POST /api/v1/complexes/:id/bookings/:bookingID/confirm-payment,
// recording a payment the owner took in cash or by transfer.
func (h *Handler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
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
		Method string `json:"method"`
		Amount int    `json:"amount"`
	}

	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Method == "cash" || input.Method == "transfer", "method", "must be cash or transfer")
	v.Check(input.Amount > 0, "amount", "must be greater than 0")
	v.Check(input.Amount <= 99_999_999, "amount", "amount too large")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	booking, payment, err := h.svc.ConfirmPayment(r.Context(), complex.ID, h.actor(r), bookingID, ConfirmPaymentInput{
		Method: input.Method,
		Amount: input.Amount,
	})
	if err != nil {
		// H-15 / R4: the status check inside the service runs against a read
		// taken before the store's own transaction opened, so a client
		// cancellation landing in that gap used to fall through every named
		// case and answer 500 — after the owner had already taken the client's
		// cash at the counter, with no way to tell whether it was recorded.
		// These two sentinels are the re-read inside InsertAndConfirmBooking's
		// own transaction catching exactly that race; naming them turns it into
		// a 409 the owner can act on.
		if errors.Is(err, bookingstore.ErrBookingNotConfirmable) || errors.Is(err, bookingstore.ErrBookingCancelled) {
			h.respond.Error(w, r, http.StatusConflict, confirmPaymentRaceMessage)
			return
		}
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": booking,
		"payment": payment,
	})
}

// ManualRefund handles POST
// /api/v1/complexes/:id/bookings/:bookingID/manual-refund, closing out a
// partially refunded booking once the owner confirms they returned the
// cash/transfer balance to the client by hand.
func (h *Handler) ManualRefund(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.svc.ManualRefund(r.Context(), complex.ID, h.actor(r), bookingID)
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking":         result.Booking,
		"payments":        result.Payments,
		"returned_amount": result.Returned,
	})
}
