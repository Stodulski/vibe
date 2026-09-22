package bookings

import (
	"fmt"
	"net/http"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Cancel handles POST /api/v1/complexes/{id}/bookings/{bookingID}/cancel.
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

	var input gen.BookingsCancelJSONBody

	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	var reason string
	if input.Reason != nil {
		reason = *input.Reason
	}

	result, err := h.svc.Cancel(r.Context(), complex, h.actor(r), bookingID, reason)
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	// A retried cancellation answers from the row alone: this request produced
	// no refund outcome, so it names none.
	if result.AlreadyCancelled {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"booking": toGenBooking(result.Booking)})
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": toGenBooking(result.Booking),
		"refund":  toGenRefundOutcome(result.Outcome),
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

// ConfirmPayment handles POST /api/v1/complexes/{id}/bookings/{bookingID}/confirm-payment,
// recording a payment the owner took at the counter, by any method but
// MercadoPago.
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

	var input gen.BookingsConfirmPaymentJSONBody

	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(isCounterPaymentMethod(string(input.Method)), "method", counterPaymentMethodsMessage)
	v.Check(input.Amount > 0, "amount", "must be greater than 0")
	v.Check(input.Amount <= 99_999_999, "amount", "amount too large")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	booking, payment, err := h.svc.ConfirmPayment(r.Context(), complex.ID, h.actor(r), bookingID, ConfirmPaymentInput{
		Method: string(input.Method),
		Amount: input.Amount,
	})
	if err != nil {
		// The two sentinels this path is most easily got wrong on — the
		// confirm-payment race of H-15 / R4 — are named in the module's
		// refusals table, so refuse answers them 409 rather than 500.
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": toGenBooking(booking),
		"payment": toGenPayment(payment),
	})
}

// ManualRefund handles POST
// /api/v1/complexes/{id}/bookings/{bookingID}/manual-refund, closing out a
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
		"booking":         toGenBooking(result.Booking),
		"payments":        toGenPayments(result.Payments),
		"returned_amount": result.Returned,
	})
}

// toGenRefundOutcome renders a refund outcome as the generated wire type,
// reproducing cancel.go's refundEnvelope field-for-field (HTTP-08): "amount"
// present only when positive, "manual_amount"/"manual_message" present only
// when a manual balance remains. refundMessage is cancel.go's own renderer —
// reused here rather than duplicated, so the API response and the
// notification text this same outcome drives can never drift into two
// different accounts of the same money.
func toGenRefundOutcome(outcome paymentstore.RefundOutcome) gen.RefundOutcome {
	out := gen.RefundOutcome{
		Message: refundMessage(outcome.Result, outcome.AmountCentavos),
		Status:  gen.RefundOutcomeStatus(outcome.Result),
	}
	if outcome.AmountCentavos > 0 {
		amount := outcome.AmountCentavos
		out.Amount = &amount
	}
	if outcome.ManualAmountCentavos > 0 {
		manualAmount := outcome.ManualAmountCentavos
		out.ManualAmount = &manualAmount
		manualMessage := refundMessage(paymentstore.RefundManual, outcome.ManualAmountCentavos)
		out.ManualMessage = &manualMessage
	}
	return out
}
