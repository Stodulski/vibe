package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// mercadoPagoProvider names the provider column of the events this handler records.
const mercadoPagoProvider = "mercadopago"

// webhookWorkTimeout bounds one attempt at an event. It has to be short enough
// that an attempt which dies is reclaimed promptly (see staleWebhookProcessing in
// internal/data/webhook_events.go) and long enough for a MercadoPago fetch plus
// the writes that follow it.
const webhookWorkTimeout = 30 * time.Second

// webhookBodyLimit caps the delivered body so a hostile caller cannot exhaust
// memory on an unauthenticated endpoint. MercadoPago's own notifications are a
// few hundred bytes, so nothing it sends comes near this.
const webhookBodyLimit = 1_048_576

// MercadoPagoWebhook handles POST /api/v1/webhooks/mercadopago.
//
// The order below is the whole point of this handler:
//
//  1. verify the signature, before a single byte is written anywhere;
//  2. record the event and commit it;
//  3. only then answer 200;
//  4. work the committed row.
//
// One thing precedes step 1, and it decides nothing: the body is read. A body
// that cannot be read leaves no request to verify a signature over, so it is
// refused with a 5xx rather than acknowledged — see the note at that read.
//
// It used to answer 200 first and process in a detached goroutine where every
// failure was a bare log line. MercadoPago never redelivers what it has already
// been told is fine, so one transient MercadoPago or Postgres blip meant the
// client's money was captured, the booking stayed pending until the expiry cron
// cancelled it, and no refund was ever issued. A 200 from here now means the
// event is durably ours; if it could not be recorded we answer 5xx and let
// MercadoPago deliver it again.
//
// It is one linear webhook lifecycle — verify, record, acknowledge, dispatch —
// and splitting it per branch would add indirection without reducing complexity.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) MercadoPagoWebhook(w http.ResponseWriter, r *http.Request) {
	// A delivery whose body could not be read is not one we can decide anything
	// about, and answering 200 to it is the same defect this handler was rewritten
	// to remove, one step earlier: MercadoPago never resends what it has been told
	// is fine, and nothing was recorded to retry from, so the client's money stays
	// captured against a booking that is never confirmed.
	//
	// A body over the cap is the single read failure a redelivery cannot fix.
	// MercadoPago's notifications are a few hundred bytes, so an oversized body is
	// not one of them and asking for it again would return the same bytes. That
	// case is dropped deliberately, with nothing written. Every other read failure
	// is the transport giving out mid-body — a reset connection, a truncated
	// stream — which is exactly what a redelivery repairs.
	r.Body = http.MaxBytesReader(w, r.Body, webhookBodyLimit)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.logger.Error("mp webhook: body over the size limit, dropping the delivery",
				"error", err, "limit_bytes", webhookBodyLimit, "remote_addr", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			return
		}
		h.logger.Error("mp webhook: failed to read the body, refusing delivery so MercadoPago retries", "error", err)
		h.respond.ServerError(w, r, err)
		return
	}

	// Parse the webhook payload.
	// MP sends data.id as number or string depending on event type,
	// so we use json.RawMessage to handle both.
	var webhook struct {
		Type   string `json:"type"`
		Action string `json:"action"`
		Data   struct {
			ID json.RawMessage `json:"id"`
		} `json:"data"`
	}

	// The body is parsed here only because data.id may have to come out of it, and
	// a failure to parse is deliberately not answered yet. Returning on it — which
	// is what used to happen — put a decision, and a log line quoting the caller's
	// own bytes back, in front of the signature check, so a malformed request from
	// anyone on the internet was handled without ever reaching the trust boundary.
	// Whatever this yields is unverified data until the check below passes.
	parseErr := json.Unmarshal(body, &webhook)

	// Extract data.id: prefer query param (MP uses this for signature), fall back to body.
	dataID := r.URL.Query().Get("data.id")
	if dataID == "" && parseErr == nil {
		dataID = strings.Trim(string(webhook.Data.ID), `"`)
	}

	// Verify the signature BEFORE anything is logged, recorded or processed.
	//
	// This ordering is security-critical and must stay first: the endpoint is
	// unauthenticated and reachable by anyone, so recording before verifying would
	// let any caller on the internet flood webhook_events with rows we then keep,
	// index, sweep and retry. Nothing below this point runs for a request
	// MercadoPago did not sign, and every request reaches this line — there is no
	// earlier exit that decides an unverified request on its content.
	if err := h.provider.VerifyWebhookSignature(r, dataID); err != nil {
		h.logger.Error("mp webhook: signature verification failed",
			"error", err,
			"remote_addr", r.RemoteAddr,
			"data_id", dataID,
		)
		// Respond 200 to prevent MP from retrying invalid requests. Nothing was
		// written, so this 200 acknowledges a request we are deliberately dropping
		// rather than one we have taken responsibility for.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Past the trust boundary. From here the request is MercadoPago's, so it can
	// be spoken about and acted on.
	if parseErr != nil {
		h.logger.Error("mp webhook: signed delivery whose body will not parse",
			"error", parseErr,
			"data_id", dataID,
		)
		// Redelivering the same unparseable bytes cannot produce a different
		// answer, so this is a deliberate drop rather than a refusal.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Log only after signature is verified (trusted data).
	h.logger.Info("mp webhook: received",
		"type", webhook.Type,
		"action", webhook.Action,
		"data_id", dataID,
	)

	if dataID == "" {
		h.logger.Error("mp webhook: missing data.id")
		w.WriteHeader(http.StatusOK)
		return
	}

	eventType := webhook.Type
	if eventType == "" {
		eventType = "unknown"
	}

	event := &paymentstore.WebhookEvent{
		Provider:   mercadoPagoProvider,
		ExternalID: dataID,
		EventType:  eventType,
		Payload:    json.RawMessage(body),
	}

	// The event is recorded and committed while MercadoPago is still waiting for
	// its answer. If this fails we must not claim to have it.
	if err := h.svc.RecordWebhookEvent(r.Context(), event); err != nil {
		h.logger.Error("mp webhook: failed to record the event, refusing delivery so MercadoPago retries",
			"error", err,
			"type", webhook.Type,
			"data_id", dataID,
		)
		sentry.CaptureMessage(fmt.Sprintf("MP WEBHOOK NOT RECORDED (answered 5xx, awaiting redelivery): type=%s data_id=%s error=%v", webhook.Type, dataID, err))
		h.respond.ServerError(w, r, err)
		return
	}

	// Recorded. From here a 200 is honest: whatever happens next is retryable
	// from the row above rather than lost.
	w.WriteHeader(http.StatusOK)

	//nolint:contextcheck // intentionally detached. The response is already
	// written, so r.Context() is about to be cancelled; the service's dispatch
	// owns its own bounded webhookWorkTimeout context and must outlive the
	// request.
	h.svc.DispatchRecordedEvent(event)
}

// RecordWebhookEvent commits one delivery to the durable inbox.
//
// It is deliberately nothing more than the write: the acknowledgement the
// handler sends next means "this row is committed", so anything else happening
// here would either widen what the 200 promises or delay it.
func (s *Service) RecordWebhookEvent(ctx context.Context, event *paymentstore.WebhookEvent) error {
	return s.webhookEvents.Insert(ctx, event)
}

// DispatchRecordedEvent works a committed event on the application's tracked
// goroutines, after its delivery has already been acknowledged.
func (s *Service) DispatchRecordedEvent(event *paymentstore.WebhookEvent) {
	// The request's context is about to be cancelled — the response is already
	// written — so the work below owns its own bounded webhookWorkTimeout
	// context and must outlive the request. Nothing is at stake in this
	// goroutine: it works a committed row, and the cron sweeper finishes the
	// job if this process dies first.
	s.run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), webhookWorkTimeout)
		defer cancel()
		// Detached means it also carries none of the request's tenant scope,
		// and this work reads payments and bookings — both under row-level
		// security since the tenant policies. MercadoPago arrives with a payment id
		// and nothing else, so the tenant is something this goroutine has to
		// discover; that is what the bypass is for, and it is the same posture
		// the route itself declares in middleware.CrossTenantRoutes.
		//
		// Without it nothing is lost, which is why this is one line rather
		// than a redesign: the row is already committed, the queries would
		// find nothing, the event would stay 'pending', and the
		// sweep_webhook_events cron would finish the job two minutes later
		// with the bypass its own wrapper sets. A two-minute delay on every
		// payment confirmation is not a defect worth having.
		ctx = data.ContextWithTenantBypass(ctx)
		s.workWebhookEvent(ctx, event)
	})
}

// ProcessPendingWebhookEvents works every recorded event that is due: the ones
// still pending because the process that recorded them died before dispatching,
// the ones a previous attempt failed and requeued, and the ones abandoned
// mid-flight in 'processing'.
//
// This is the half of the fix the inline path cannot provide. The inline dispatch
// runs in this process; a deploy, an OOM kill or a panic in between leaves the row
// behind, and without a sweeper "durably recorded" would only mean "durably
// recorded and then forgotten".
func (s *Service) ProcessPendingWebhookEvents(ctx context.Context) {
	due, err := s.webhookEvents.GetPendingDue(ctx)
	if err != nil {
		s.logger.Error("webhook-sweep: failed to fetch due events", "error", err)
		return
	}
	if len(due) == 0 {
		return
	}

	for _, event := range due {
		s.workWebhookEvent(ctx, event)
	}

	s.logger.Info("webhook-sweep: completed", "events", len(due))
}

// workWebhookEvent drives one recorded event to a terminal state.
//
// Every exit leaves the row in a state something will come back to: 'processed'
// when the work is done, 'pending' with a backoff when it failed, 'exhausted'
// when the budget is spent and a human is needed, and 'processing' only for as
// long as this attempt lives — the sweeper reclaims that after the stale window.
func (s *Service) workWebhookEvent(ctx context.Context, event *paymentstore.WebhookEvent) {
	claimed, err := s.webhookEvents.Claim(ctx, event.ID)
	if err != nil {
		// The row is untouched, so it is still due and the next sweep retries it.
		s.logger.Error("mp webhook: failed to claim the recorded event",
			"error", err, "event_id", event.ID, "data_id", event.ExternalID)
		return
	}
	if !claimed {
		s.logger.Info("mp webhook: event is already being worked elsewhere, skipping",
			"event_id", event.ID, "data_id", event.ExternalID)
		return
	}

	if err := s.dispatchWebhookEvent(ctx, event); err != nil {
		s.requeueWebhookEvent(ctx, event, err)
		return
	}

	if err := s.webhookEvents.MarkProcessed(ctx, event.ID); err != nil {
		// The work itself succeeded; only the bookkeeping failed. The row stays in
		// 'processing' and the sweeper reclaims it once the attempt goes stale,
		// which replays a dispatch that is already idempotent — the advisory lock
		// and the GetByMPPaymentID check see to that.
		s.logger.Error("mp webhook: processed the event but failed to record that",
			"error", err, "event_id", event.ID, "data_id", event.ExternalID)
	}
}

// requeueWebhookEvent records a failed attempt and alerts when the budget is spent.
func (s *Service) requeueWebhookEvent(ctx context.Context, event *paymentstore.WebhookEvent, cause error) {
	s.logger.Error("mp webhook: processing failed, event queued for retry",
		"error", cause,
		"event_id", event.ID,
		"type", event.EventType,
		"data_id", event.ExternalID,
	)

	exhausted, err := s.webhookEvents.MarkFailed(ctx, event.ID, cause.Error())
	if err != nil {
		// Left in 'processing'; the sweeper reclaims it after the stale window, so
		// only the backoff and the recorded reason are missing.
		s.logger.Error("mp webhook: failed to requeue the event",
			"error", err, "event_id", event.ID, "data_id", event.ExternalID)
		return
	}
	if exhausted {
		s.logger.Error("mp webhook: EXHAUSTED all retries, manual intervention required",
			"event_id", event.ID,
			"type", event.EventType,
			"data_id", event.ExternalID,
			"last_error", cause,
		)
		sentry.CaptureMessage(fmt.Sprintf("MP WEBHOOK EXHAUSTED (payment may be captured with no confirmed booking): type=%s data_id=%s event_id=%s error=%v", event.EventType, event.ExternalID, event.ID, cause))
	}
}

// dispatchWebhookEvent routes a recorded event to whatever handles it.
//
// A returned error means "try this again": the database or MercadoPago was
// unavailable. A nil return means the event reached a decision, including the
// decisions that refuse to act — an unknown event type, or a payment whose
// booking cannot be identified. Retrying those would only burn the budget and end
// in a false alert.
func (s *Service) dispatchWebhookEvent(ctx context.Context, event *paymentstore.WebhookEvent) error {
	switch event.EventType {
	case "payment":
		return s.processPaymentWebhook(ctx, event.ExternalID)
	case "chargebacks":
		// Chargebacks are handled through payment status changes (charged_back).
		// Alert via Sentry so the team is notified immediately.
		s.logger.Error("mp webhook: CHARGEBACK received — review required",
			"action", webhookAction(event.Payload),
			"data_id", event.ExternalID,
		)
		sentry.CaptureMessage(fmt.Sprintf("MercadoPago CHARGEBACK: action=%s data_id=%s", webhookAction(event.Payload), event.ExternalID))
	case "topic_claims_integration_wh":
		// Claims/disputes — alert via Sentry for immediate attention.
		s.logger.Error("mp webhook: CLAIM/DISPUTE received — review required",
			"action", webhookAction(event.Payload),
			"data_id", event.ExternalID,
		)
		sentry.CaptureMessage(fmt.Sprintf("MercadoPago CLAIM/DISPUTE: action=%s data_id=%s", webhookAction(event.Payload), event.ExternalID))
	case "mp-connect":
		// OAuth connection/disconnection events.
		s.logger.Info("mp webhook: mp-connect event received",
			"action", webhookAction(event.Payload),
			"data_id", event.ExternalID,
		)
	default:
		s.logger.Info("mp webhook: ignoring unhandled event type", "type", event.EventType)
	}
	return nil
}

// webhookAction reads the provider's action string back out of a recorded
// payload. It is only ever used for logging, so an unreadable payload yields ""
// rather than an error.
func webhookAction(payload json.RawMessage) string {
	var body struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	return body.Action
}

// processPaymentWebhook handles the actual payment processing logic for one
// recorded event, and reports whether the event should be tried again.
//
// Errors are reserved for what a retry can fix — the idempotency lock, the
// database, and MercadoPago's API. Everything the payment itself decides returns
// nil: it happened, it was recorded, and doing it again changes nothing.
//
// It is one linear payment-webhook dispatch — fetch the payment, dispatch by
// status, handle idempotency — and per-branch extraction would add indirection
// without reducing complexity, at the cost of disturbing this domain's tested
// control flow.
//
//nolint:funlen // see the cohesion note above
func (s *Service) processPaymentWebhook(ctx context.Context, mpPaymentID string) error {
	// MercadoPago delivers the same webhook more than once. The advisory lock
	// makes handling it idempotent across every instance: whoever takes it
	// processes the payment, and everyone else returns. This is unchanged by the
	// durable inbox — each delivery is its own row, and deduplication still
	// happens here.
	acquired, release, err := s.locks.TryAdvisory(ctx, "mp_webhook:"+mpPaymentID)
	if err != nil {
		return fmt.Errorf("take the idempotency lock for %s: %w", mpPaymentID, err)
	}
	defer release()

	if !acquired {
		// Another worker holds this payment. Its own event row is durable and gets
		// retried if it dies, so this delivery has nothing left to do.
		s.logger.Info("mp webhook: another instance is handling this payment, skipping", "mp_payment_id", mpPaymentID)
		return nil
	}

	// Check if we already have a record of this payment.
	existingPayment, err := s.payments.GetByMPPaymentID(ctx, mpPaymentID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		return fmt.Errorf("check whether payment %s is already recorded: %w", mpPaymentID, err)
	}

	// Fetch the real payment from MercadoPago API (never trust webhook data).
	// As the platform, deliberately: the seller who collected this money is
	// not known until the payment itself says so, and MercadoPago answers the
	// app owner for anything created under its own app_id.
	mpPayment, err := s.provider.GetPayment(ctx, mpPaymentID, mp.AsPlatform())
	if err != nil {
		return fmt.Errorf("fetch payment %s from mercadopago: %w", mpPaymentID, err)
	}

	s.logger.Info("mp webhook: fetched payment from MP",
		"mp_payment_id", mpPaymentID,
		"status", mpPayment.Status,
		"status_detail", mpPayment.StatusDetail,
		"amount", mpPayment.TransactionAmount,
		"net_received", mpPayment.NetReceivedAmount,
		"fee_details", fmt.Sprintf("%+v", mpPayment.FeeDetails),
		"external_reference", mpPayment.ExternalReference,
	)

	// If the payment was already processed, only handle status transitions
	// that require action (refund/chargeback). Skip everything else.
	if existingPayment != nil {
		if mpPayment.Status == "refunded" || mpPayment.Status == "charged_back" {
			return s.processRefundedPayment(ctx, existingPayment, mpPayment, mpPaymentID)
		}
		s.logger.Info("mp webhook: payment already processed, skipping",
			"mp_payment_id", mpPaymentID,
			"mp_status", mpPayment.Status,
		)
		return nil
	}

	// Extract booking_id from external_reference or metadata.
	bookingIDStr := mpPayment.ExternalReference
	if bookingIDStr == "" {
		if bid, ok := mpPayment.Metadata["booking_id"].(string); ok {
			bookingIDStr = bid
		}
	}

	if bookingIDStr == "" {
		// Not retryable: MercadoPago's own copy of the payment carries no booking,
		// and it will not grow one. The event stands as the record that it arrived.
		s.logger.Error("mp webhook: no booking_id found in payment", "mp_payment_id", mpPaymentID)
		return nil
	}

	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		s.logger.Error("mp webhook: invalid booking_id", "booking_id", bookingIDStr, "error", err)
		return nil
	}

	// Fetch the booking.
	booking, err := s.bookings.GetByID(ctx, bookingID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			s.logger.Error("mp webhook: payment names a booking that does not exist",
				"booking_id", bookingID, "mp_payment_id", mpPaymentID)
			return nil
		}
		return fmt.Errorf("fetch booking %s: %w", bookingID, err)
	}

	switch mpPayment.Status {
	case "approved":
		return s.processApprovedPayment(ctx, booking, mpPayment, mpPaymentID)
	case "rejected", "cancelled":
		return s.processRejectedPayment(ctx, booking, mpPayment, mpPaymentID)
	case "refunded", "charged_back":
		// Refund/chargeback on a payment we haven't recorded yet — cancel the booking.
		return s.processRefundedPaymentFromBooking(ctx, booking, mpPayment, mpPaymentID)
	case "in_process", "pending":
		// Payment is being reviewed or waiting for offline payment.
		// MP will send another webhook when the status changes.
		s.logger.Info("mp webhook: payment pending/in_process, awaiting resolution",
			"status", mpPayment.Status,
			"status_detail", mpPayment.StatusDetail,
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
		)
	default:
		s.logger.Info("mp webhook: unhandled payment status",
			"status", mpPayment.Status,
			"mp_payment_id", mpPaymentID,
		)
	}
	return nil
}
