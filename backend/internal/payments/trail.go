package payments

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Actors this module records. Nothing here runs on an authenticated session,
// so audit_log.user_id is always NULL and the actor has to be said in words —
// and the distinction that matters on the money path is which side moved it.
//
// actorSystem is this system doing what a cancellation implied: the refund it
// issued, the retry that finished one, the sweep that recovered one. Who asked
// for the cancellation is recorded by internal/bookings on the same booking id,
// next to this entry.
//
// actorProvider is MercadoPago moving money without being asked — a chargeback,
// or a refund a buyer took out directly through the provider. That is the one
// event on this path nobody here decided, and the one an owner is most likely
// to come looking for.
const (
	actorSystem   = "system"
	actorProvider = "mercadopago"
)

// moneyEvent is the audit value for one thing that happened to a client's
// money.
//
// The fields are named explicitly rather than handing over a paymentstore.Payment: a
// payment row is not a credential-free struct by construction the way
// bookingstore.Booking is, and this value is written from paths that sometimes hold a
// claim and no row at all. Everything here is an amount, an internal id, or a
// MercadoPago payment reference — never a seller token, which is what
// complexstore.Complex's `json:"-"` tags exist to keep out of exactly this table.
type moneyEvent struct {
	Actor string `json:"actor"`
	// PaymentID is our own payments row, absent on the exits that refuse
	// before one is in hand.
	PaymentID *uuid.UUID `json:"payment_id,omitempty"`
	// MPPaymentID is MercadoPago's reference for the same money. It is an
	// external record locator, not a secret — it is already in the logs and in
	// every Sentry alert this module raises.
	MPPaymentID string `json:"mp_payment_id,omitempty"`
	// AmountCentavos is the sum the Result is about: confirmed, refunded,
	// queued or owed.
	AmountCentavos int `json:"amount_centavos"`
	// Result is paymentstore.RefundResult on the refund paths, and a short verb on the
	// others. Reason is the operator-facing note the outcome already carries.
	Result string `json:"result,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// refundEvent builds the value for a refund whose outcome is already decided.
func refundEvent(actor string, outcome paymentstore.RefundOutcome) moneyEvent {
	return moneyEvent{
		Actor:          actor,
		AmountCentavos: outcome.AmountCentavos,
		Result:         string(outcome.Result),
		Reason:         outcome.Reason,
	}
}

// claimEvent builds the value for a refund attempt a claim already reserved.
//
// The claim carries the payment row, MercadoPago's reference and the sum, so
// no call site restates them — which is how the three refund paths came to
// disagree about which figure they were recording in the first place.
func claimEvent(claim paymentstore.RefundClaim, amount int, result paymentstore.RefundResult, reason string) moneyEvent {
	paymentID := claim.PaymentID
	return moneyEvent{
		Actor:          actorSystem,
		PaymentID:      &paymentID,
		MPPaymentID:    claim.MPPaymentID,
		AmountCentavos: amount,
		Result:         string(result),
		Reason:         reason,
	}
}

// shortfallReason is the one sentence for money MercadoPago did not move on a
// refund it accepted. AutoRefundIfPaid's own shortfall exit says the same
// thing through owedManually.
const shortfallReason = "mercadopago refunded less than was claimed, and the remainder has no queued attempt behind it"

// reportShortfall logs, alerts and records whatever MercadoPago did not move
// on a refund it accepted.
//
// RecordRefundSuccess has already resolved the attempt row for the amount that
// did move, so the remainder is owed with nothing automatic behind it: it is a
// person's job, and the only two places it can be seen are the alert and this
// entry. Both webhook and retry paths report it identically, which is why they
// share this rather than each keeping their own copy.
func (s *Service) reportShortfall(source, action string, complexID, bookingID uuid.UUID, claimed, settled paymentstore.RefundClaim) {
	short := refundShortfall(claimed, settled)
	if short <= 0 {
		return
	}

	s.logger.Error(source+": mercadopago refunded less than was claimed, the remainder is owed by hand",
		"booking_id", bookingID,
		"mp_payment_id", claimed.MPPaymentID,
		"attempt_id", claimed.AttemptID,
		"claimed", claimed.RefundCentavos,
		"moved", settled.RefundCentavos,
		"shortfall", short,
	)
	sentry.CaptureMessage(fmt.Sprintf("MANUAL REFUND OWED: booking_id=%s mp_payment_id=%s amount=%d — %s",
		bookingID, claimed.MPPaymentID, short, shortfallReason))
	s.record(complexID, bookingID, action, claimEvent(claimed, short, paymentstore.RefundManual, shortfallReason))
}

// record writes one entry to the audit trail.
//
// It takes ids rather than an *http.Request because nothing in this module
// runs on one. The webhook body is committed to the durable inbox and worked
// afterwards, on a background goroutine; the refund queue and the refund-intent
// sweep are cron jobs. So there is no authenticated user (UserID nil), and no
// client address to attribute this to (IPAddress empty, which InsertAuditLog
// stores as SQL NULL rather than inventing one).
//
// Every entry is keyed on the booking, not on the payment row. The trail is
// read by the complex that owns the booking, scoped to entity_type
// (internal/audit/handler.go), and that reader navigates by booking: keying
// these on "booking" puts a venue's create, cancel and refund entries in one
// list in the order they happened. The payment row's own id travels in the
// value.
func (s *Service) record(complexID, bookingID uuid.UUID, action string, value moneyEvent) {
	s.audit.Record(audit.Entry{
		ComplexID:  &complexID,
		Action:     action,
		EntityType: "booking",
		EntityID:   &bookingID,
		NewValue:   value,
	})
}

// providerRefundAction names a refund MercadoPago performed without being
// asked, separating a chargeback — money taken back by the buyer's bank or by
// a dispute — from an ordinary refund the buyer took out through MercadoPago
// itself. The two have very different consequences for a venue, and the trail
// is where an owner reconstructs which one hit them.
func providerRefundAction(mpStatus string) string {
	if mpStatus == "charged_back" {
		return "chargeback"
	}
	return "refund_external"
}
