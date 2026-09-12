package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Cancel handles POST /api/v1/complexes/:id/bookings/:bookingID/cancel.
//
// Cancelling an already-cancelled booking answers 200 without doing anything,
// so a retried request cannot refund twice. Completed and no-show bookings
// are terminal and refused.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
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

	// Only pending/confirmed bookings can be cancelled. Terminal states are immutable.
	if booking.Status == "cancelled" || booking.Status == "completed" || booking.Status == "no_show" {
		if booking.Status == "cancelled" {
			// A retried cancellation answers from the row: the booking already
			// carries the payment status the refund path wrote, so nothing here
			// has to guess at an outcome this request did not produce.
			h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"booking": booking})
			return
		}
		h.respond.BadRequest(w, r, fmt.Errorf("cannot cancel a booking with status '%s'", booking.Status))
		return
	}

	var input struct {
		Reason string `json:"reason"`
	}

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	booking.Status = "cancelled"
	if input.Reason != "" {
		notes := input.Reason
		if booking.Notes != nil {
			notes = *booking.Notes + " | Cancelación: " + input.Reason
		}
		booking.Notes = &notes
	}

	// owesRefund feeds both the marker write below and the refund dispatch
	// after Update — one expression, not two, so an edit that marks a
	// cancellation the money must not follow is, by construction, the same
	// edit that stops refunding it (refund-intent-durability spec's
	// load-bearing property). The staff path ignores the complex's refund
	// window entirely (see the comment below), so this reads only the
	// collection axis: money was taken, so money is owed back.
	owesRefund := booking.CollectionStatus != bookingstore.CollectionStatusUnpaid
	if owesRefund {
		now := time.Now()
		booking.RefundIntentAt = &now
	}

	err = h.store.Update(r.Context(), booking)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "cancel", &booking.ID, booking)

	// Handle MP payment: expire the preference if unpaid, auto-refund if paid.
	//
	// Staff cancellations refund regardless of the complex's cancellation window;
	// only the public path applies it. That asymmetry is the product's, not an
	// oversight — the owner cancelling is the venue's own decision, and it should
	// not charge the client a penalty for it.
	var outcome paymentstore.RefundOutcome
	if !owesRefund {
		h.expireCheckoutPreference(r.Context(), booking, complex)
		outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "the booking was never paid"}
	} else {
		outcome = h.refunds.AutoRefundIfPaid(r.Context(), booking)
	}
	if outcome.NeedsAHuman() {
		// The owner is the person who has to hand the money back, and this
		// response is the only moment they are looking.
		h.logger.Error("cancel booking: a refund is owed that must be returned by hand",
			"booking_id", booking.ID, "complex_id", complex.ID,
			"amount", outcome.AmountCentavos, "reason", outcome.Reason)
	}

	// Send cancellation notification (email + optionally WhatsApp).
	court, courterr := h.courts.GetByID(r.Context(), booking.CourtID)
	client, cerr := h.clients.GetByID(r.Context(), booking.ClientID)
	if cerr == nil && courterr == nil {
		clientEmail := ""
		if client.Email != nil {
			clientEmail = *client.Email
		}
		// The refund decision made above travels with the message. It used to
		// stop here: the client was told their booking was off and nothing
		// about their deposit, and found out about the money — or did not —
		// from a separate email that only fires when one is actually sent.
		refundLine, refundAmount := refundNotice(outcome)
		h.notify.BookingCancelled(notifications.Cancellation{
			Email:        clientEmail,
			Phone:        client.Phone,
			ComplexName:  complex.Name,
			CourtName:    court.Name,
			Date:         booking.Date.Format("02/01"),
			StartTime:    timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
			RefundLine:   refundLine,
			RefundAmount: refundAmount,
			BookPath:     booklink.BookPath(complex.Slug),
			BookURL:      booklink.Book(h.cfg.FrontendURL, complex.Slug),
		})
	}

	// The slot is free now; the checkout lock that was holding it is not.
	if courterr == nil {
		h.releaseHeldSlots(r.Context(), booking)
	}

	h.realtime.PublishBookingChanged(complex.ID)

	booking.RefundStatus = refundStatusAfter(booking, outcome)
	// ClaimRefund clears the marker in the database but cannot reach this
	// in-memory *Booking, so it is cleared here unconditionally: the field was
	// either never set, cleared inside a committed ClaimRefund, or cleared by
	// AutoRefundIfPaid's other exits — nil is correct in every case.
	booking.RefundIntentAt = nil
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": booking,
		"refund":  refundEnvelope(outcome),
	})
}

// ConfirmPayment handles POST /api/v1/complexes/:id/bookings/:bookingID/confirm-payment,
// recording a payment the owner took in cash or by transfer.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
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
		Method string `json:"method"`
		Amount int    `json:"amount"`
	}

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
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

	// Prevent confirming payment on cancelled or already-paid bookings.
	if booking.Status == "cancelled" {
		h.respond.BadRequest(w, r, fmt.Errorf("cannot confirm payment for a cancelled booking"))
		return
	}
	if booking.CollectionStatus == bookingstore.CollectionStatusFullyPaid {
		h.respond.BadRequest(w, r, fmt.Errorf("esta reserva ya tiene pago confirmado"))
		return
	}

	// Confirming a pending booking is what makes it hold its slot: until this
	// point the row does not stop anyone else, and afterwards it does. So it is
	// the second place that has to ask whether those hours are still on sale.
	//
	// InsertAndConfirmBooking re-checks the bookings table under the court-day
	// advisory lock, which is why a genuine race reaches here as
	// ErrSlotUnavailable. It does not look at blocked_slots at all, and neither
	// did this handler — so an owner who blocked a court for maintenance after
	// a client started checkout could still be handed a confirmed booking on
	// it, taking the hours out of circulation for the maintenance and selling
	// them at the same time.
	blocked, err := h.slotIsBlocked(r.Context(), booking.CourtID, booking.Date, booking.StartsAt, booking.EndsAt)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if blocked {
		h.respond.Error(w, r, http.StatusConflict, blockedSlotMessage)
		return
	}

	payment := &paymentstore.Payment{
		BookingID: booking.ID,
		ComplexID: booking.ComplexID,
		Amount:    input.Amount,
		Method:    input.Method,
		Status:    "fully_paid",
	}

	// Confirming payment also confirms the booking (prevents cron from
	// expiring a paid booking that was still in "pending" status).
	if booking.Status == "pending" {
		booking.Status = "confirmed"
	}
	totalPaid := input.Amount
	if booking.CollectionStatus == bookingstore.CollectionStatusDepositPaid {
		totalPaid += booking.DepositAmount
	}
	if totalPaid >= booking.Price {
		booking.CollectionStatus = bookingstore.CollectionStatusFullyPaid
	} else {
		booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
		booking.DepositAmount = totalPaid
	}
	err = h.payments.InsertAndConfirmBooking(r.Context(), payment, booking)
	if err != nil {
		// Confirming now re-checks that the slot is still free, so a genuine
		// conflict reaches here as ErrSlotUnavailable. It is a business answer —
		// somebody else holds those hours — not a server fault, and it gets the
		// same 409 the booking-creation paths return for it.
		if errors.Is(err, bookingstore.ErrSlotUnavailable) || errors.Is(err, bookingstore.ErrDuplicateBooking) {
			h.respond.EditConflict(w, r)
			return
		}
		// H-15 / R4: the booking.Status check a few lines above this function's
		// own comment runs against a read taken before this request's
		// transaction opened, so a client cancellation landing in that gap used
		// to fall through every named case here and answer 500 — after the
		// owner had already taken the client's cash at the counter, with no
		// way to tell whether it was recorded. ErrBookingNotConfirmable is the
		// re-read inside InsertAndConfirmBooking's own transaction catching
		// exactly that race; naming it here is what turns it into a 409 the
		// owner can act on (refresh and check the booking's real status)
		// instead of the opaque "server encountered a problem" every other
		// unmapped error still gets.
		// Both sentinels, unchanged in behaviour: an owner taking cash at the
		// counter gets the same 409 whether the booking was cancelled or
		// completed underneath them. The split between the two exists for the
		// webhook, which has money to send back (H-23), not for this path,
		// which has cash in a till.
		if errors.Is(err, bookingstore.ErrBookingNotConfirmable) || errors.Is(err, bookingstore.ErrBookingCancelled) {
			h.respond.Error(w, r, http.StatusConflict,
				"this booking's status changed before the payment could be confirmed, refresh and check it")
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "confirm_payment", &booking.ID, booking)
	h.realtime.PublishBookingChanged(complex.ID)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": booking,
		"payment": payment,
	})
}

// ManualRefund handles POST
// /api/v1/complexes/:id/bookings/:bookingID/manual-refund, closing out a
// partially refunded booking once the owner confirms they returned the
// cash/transfer balance to the client by hand.
//
// A booking reaches refund_status 'partial' when the refund pipeline
// auto-refunds its MercadoPago-backed payment rows but a sibling cash or
// transfer row is still owed — nothing automatic can return that money, and it
// stays owed until this endpoint records that a person did. It is the only path
// allowed to move a booking off 'partial' (see validRefundStatuses in
// handlers.go); a generic PUT may not.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
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

	if booking.Status != "cancelled" || booking.RefundStatus != bookingstore.RefundStatusPartial {
		h.respond.BadRequest(w, r, fmt.Errorf("esta reserva no tiene una devolución manual pendiente"))
		return
	}

	returned, err := h.payments.RecordManualRefund(r.Context(), booking.ID)
	if err != nil {
		switch {
		case errors.Is(err, paymentstore.ErrNoManualRefundOwed):
			// Locked under its own transaction, the booking no longer read
			// refund_status 'partial' — another request closed it out between
			// the read above and the write.
			h.respond.BadRequest(w, r, fmt.Errorf("esta reserva no tiene una devolución manual pendiente"))
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	booking.RefundStatus = bookingstore.RefundStatusFull

	payments, err := h.payments.ListByBookingID(r.Context(), booking.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "manual_refund", &booking.ID, booking)
	h.realtime.PublishBookingChanged(complex.ID)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking":         booking,
		"payments":        payments,
		"returned_amount": returned,
	})
}
