package payments

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A second approved payment on a booking that is already settled is money the
// complex collected for a slot it has already sold, so it goes back to the client
// in full. Only that payment is recorded and refunded. The booking and the payment
// that settled it stay exactly as they were, and nothing says the booking was
// cancelled.
func TestASecondApprovedPaymentOnASettledBookingIsRefundedAndTheBookingStaysPut(t *testing.T) {
	for _, status := range []bookingstore.BookingStatus{
		bookingstore.BookingStatusConfirmed,
		bookingstore.BookingStatusCompleted,
		bookingstore.BookingStatusNoShow,
	} {
		t.Run(status.String(), func(t *testing.T) {
			sentryEvents := withCapturedSentryEvents(t)
			f := newFixture(t)
			complexID := uuid.New()
			booking, deposit := paidBooking(complexID)
			booking.Status = status

			// The deposit already recorded is a different MercadoPago payment from the
			// one arriving now. It is the whole ledger, so the second payment is read
			// alongside it, as the store would read it.
			recorded := "mp-OLD"
			deposit.MPPaymentID = &recorded
			f.payments.byBooking = deposit
			f.payments.byBookingAll = []*paymentstore.Payment{deposit}
			f.complexes.complex = linkedComplex(complexID, "111111111")
			f.bookings.booking = booking
			f.provider.payment = &mp.Payment{
				ID: 123, Status: "approved", TransactionAmount: 1500,
				CollectorID: 111111111, ExternalReference: booking.ID.String(),
			}

			// MercadoPago delivers the same event twice. The first delivery refunds the
			// payment; the second finds it already recorded and does nothing.
			for delivery := 1; delivery <= 2; delivery++ {
				if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
					t.Fatalf("delivery %d: the money is captured, so the event must not fail; got %v", delivery, err)
				}
			}

			if len(f.payments.insertedRows) != 1 {
				t.Fatalf("the second payment must be recorded exactly once; got %d rows", len(f.payments.insertedRows))
			}
			row := f.payments.insertedRows[0]
			if row.MPPaymentID == nil || *row.MPPaymentID != "mp-123" {
				t.Errorf("the recorded row must be the second payment; got %v", row.MPPaymentID)
			}
			if row.Status != "refunded" {
				t.Errorf("the second payment must end refunded; got %q", row.Status)
			}
			if len(f.provider.refunds) != 1 {
				t.Fatalf("one MercadoPago refund across both deliveries; got %d", len(f.provider.refunds))
			}
			if want := float64(row.Amount+row.ServiceFee) / 100; f.provider.refunds[0] != want {
				t.Errorf("the refund must be for that payment's amount, %v pesos; got %v", want, f.provider.refunds[0])
			}

			if booking.Status != status {
				t.Errorf("the booking status must not change; got %q, want %q", booking.Status, status)
			}
			if booking.RefundStatus != bookingstore.RefundStatusNone || booking.RefundIntentAt != nil {
				t.Errorf("the booking must not gain a refund or a refund-intent marker; got refund_status=%q intent=%v",
					booking.RefundStatus, booking.RefundIntentAt)
			}
			if deposit.MPPaymentID == nil || *deposit.MPPaymentID != recorded {
				t.Error("the payment already recorded must not be rewritten to point at the second payment")
			}
			if f.bookings.updated != nil || f.payments.confirmed != nil || f.payments.confirmedBooking != nil || f.payments.insertedBooking != nil {
				t.Error("the booking must not be written for the second payment")
			}
			if len(f.refundIntents.claimed) != 0 || len(f.refundIntents.cleared) != 0 {
				t.Error("the refund-intent marker belongs to the cancelled path; this path must not touch it")
			}
			if len(f.notify.ownerRefunds) != 0 || len(f.waitlist.freed) != 0 {
				t.Error("nothing cancelled the booking, so the owner is not told it was refunded and no slot goes to the waitlist")
			}
			if len(sentryEvents.messages()) != 0 {
				t.Errorf("a refund that went through needs no alert; captured %q", sentryEvents.messages())
			}
		})
	}
}

// The duplicate path is only for money the complex actually collected. A payment
// from another seller is neither recorded nor refunded by this complex, so it stays
// a fraud-alert log line and nothing more.
func TestASecondPaymentOnASettledBookingFromAForeignCollectorIsNotRefunded(t *testing.T) {
	sentryEvents := withCapturedSentryEvents(t)
	f := newFixture(t)
	complexID := uuid.New()
	booking, deposit := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusConfirmed
	recorded := "mp-OLD"
	deposit.MPPaymentID = &recorded
	f.payments.byBooking = deposit
	f.complexes.complex = linkedComplex(complexID, "111111111")

	mpPayment := &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 999999999, ExternalReference: booking.ID.String(),
	}
	_ = f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123")

	if len(f.payments.insertedRows) != 0 || len(f.provider.refunds) != 0 {
		t.Error("a payment collected by another seller must neither be recorded nor refunded by this complex")
	}
	if _, found := findMessage(sentryEvents.messages(), "FRAUD ALERT: collector mismatch"); !found {
		t.Errorf("the collector mismatch must still be reported; captured %q", sentryEvents.messages())
	}
}

// A duplicate whose row cannot be written has nothing to claim, so it is not
// refunded. Nothing was written, so the event fails and is retried cleanly, and the
// money still held is reported.
func TestASecondApprovedPaymentThatCannotBeRecordedIsRetriedAndAlerted(t *testing.T) {
	sentryEvents := withCapturedSentryEvents(t)
	f := newFixture(t)
	complexID := uuid.New()
	booking, deposit := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusConfirmed
	recorded := "mp-OLD"
	deposit.MPPaymentID = &recorded
	f.payments.byBooking = deposit
	f.payments.byBookingAll = []*paymentstore.Payment{deposit}
	f.payments.insertRowErr = errDatabase
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err == nil {
		t.Fatal("a payment that could not be recorded must fail the event so MercadoPago's delivery is retried")
	}
	if len(f.provider.refunds) != 0 {
		t.Error("nothing may be refunded for a payment that was not recorded")
	}
	if _, found := findMessage(sentryEvents.messages(), "SETTLED BOOKING PAID AGAIN NOT RECORDED"); !found {
		t.Errorf("money held without a record must reach Sentry; captured %q", sentryEvents.messages())
	}
}

// A duplicate whose refund MercadoPago refuses is queued at payment level, the same
// way as any refund. The queue does not read the booking's refund-intent marker, and
// this path never sets it.
func TestASecondApprovedPaymentWhoseRefundFailsIsQueuedWithoutTheBookingMarker(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, deposit := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusConfirmed
	recorded := "mp-OLD"
	deposit.MPPaymentID = &recorded
	f.payments.byBooking = deposit
	f.payments.byBookingAll = []*paymentstore.Payment{deposit}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}
	f.provider.refundErr = errProvider

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("a refund MercadoPago refuses is queued, not retried as the webhook; got %v", err)
	}
	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("the refused refund must be queued once at payment level; got %d", len(f.payments.recordedFailure))
	}
	if len(f.refundIntents.cleared) != 0 || len(f.refundIntents.claimed) != 0 {
		t.Error("the queued refund must not depend on, or touch, the booking's refund-intent marker")
	}
	if booking.Status != bookingstore.BookingStatusConfirmed || booking.RefundIntentAt != nil {
		t.Errorf("the booking must stay confirmed and unmarked; got status=%q intent=%v", booking.Status, booking.RefundIntentAt)
	}
}

// A first rejected card used to cancel a pending booking, and the client who
// retried with a second card lost the slot and was refunded. A rejection leaves
// the booking pending until the normal expiry, and its reason is still recorded.
func TestRejectedPaymentLeavesAPendingBookingPending(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.Status = "rejected"
	mpPayment.StatusDetail = "cc_rejected_insufficient_amount"
	mpPayment.CollectorID = 111111111
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.payments.byBooking = &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: booking.DepositAmount, Status: "unpaid",
	}

	if err := f.service.processRejectedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("a rejected attempt is not retryable; got %v", err)
	}

	if f.bookings.updated != nil || booking.Status != "pending" {
		t.Errorf("a rejected attempt must not cancel the pending booking; got status=%q", booking.Status)
	}
	if len(f.realtime.published) != 0 {
		t.Error("nothing changed on the booking, so there is nothing to push to the dashboard")
	}
	if f.payments.updated == nil || f.payments.updated.StatusDetail == nil ||
		*f.payments.updated.StatusDetail != "cc_rejected_insufficient_amount" {
		t.Error("the decline reason must still be recorded on the payment row")
	}
}

// A rejection another seller collected must not be written onto this complex's
// payment row, the same as an approval would not be.
func TestRejectedPaymentFromAForeignCollectorWritesNothing(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.Status = "rejected"
	mpPayment.StatusDetail = "cc_rejected_insufficient_amount"
	mpPayment.CollectorID = 999999999
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.payments.byBooking = &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: booking.DepositAmount, Status: "unpaid",
	}

	if err := f.service.processRejectedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("an unverified rejection is dropped, not retried; got %v", err)
	}
	if f.payments.updated != nil {
		t.Error("a rejection another seller collected must not be written onto this complex's payment row")
	}
	if f.bookings.updated != nil {
		t.Error("a rejection another seller collected must not touch the booking")
	}
}

// A refund for a payment this system never recorded cancels the booking it names.
// That is only safe when the money was the complex's own, so the collector is
// checked first, as it is for an approval.
func TestRefundOfAPaymentWeNeverRecordedIsCheckedAgainstTheCollector(t *testing.T) {
	tests := []struct {
		name      string
		collector int64
		cancels   bool
	}{
		{name: "the complex collected it", collector: 111111111, cancels: true},
		{name: "another seller collected it", collector: 999999999, cancels: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, mpPayment := pendingBooking(complexID)
			booking.Status = bookingstore.BookingStatusConfirmed
			mpPayment.Status = "refunded"
			mpPayment.CollectorID = tt.collector
			mpPayment.ExternalReference = booking.ID.String()
			mpPayment.TransactionAmountRefunded = mpPayment.TransactionAmount
			f.provider.payment = mpPayment
			f.bookings.booking = booking
			f.complexes.complex = linkedComplex(complexID, "111111111")

			_ = f.service.processPaymentWebhook(t.Context(), "mp-123")

			if cancelled := f.bookings.updated != nil; cancelled != tt.cancels {
				t.Errorf("booking cancelled = %v, want %v", cancelled, tt.cancels)
			}
		})
	}
}

// A client whose record cannot be read used to get no confirmation and no log
// line at all, and the webhook is never retried once the payment is committed.
func TestAFailedClientLookupIsReportedNotSilentlySkipped(t *testing.T) {
	sentryEvents := withCapturedSentryEvents(t)
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID, Name: "Vibe", Slug: "vibe"}
	f.bookings.booking = booking
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.clients.err = errDatabase

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("the payment is committed, so a notification lookup must not fail the webhook; got %v", err)
	}
	if f.payments.inserted == nil {
		t.Fatal("the payment must still be recorded when the notification lookup fails")
	}
	if len(f.notify.confirmations) != 0 {
		t.Error("with no client there is no confirmation to send")
	}
	if _, found := findMessage(sentryEvents.messages(), "CONFIRMATION NOT SENT"); !found {
		t.Errorf("the skipped confirmation must reach Sentry; captured %q", sentryEvents.messages())
	}
	logs := f.logs.String()
	if !strings.Contains(logs, "level=ERROR") || !strings.Contains(logs, booking.ID.String()) {
		t.Errorf("the skipped confirmation must be logged at error level with the booking id; got %q", logs)
	}
}

// Every rejected signature used to be a single log line. A wrong webhook secret
// silently drops every payment notification, so it has to reach Sentry too.
func TestAWebhookWithABadSignatureIsReportedToSentry(t *testing.T) {
	sentryEvents := withCapturedSentryEvents(t)
	f := newFixture(t)
	f.provider.signatureErr = errProvider

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if w.Code != http.StatusOK {
		t.Errorf("want 200 so the provider stops retrying; got %d", w.Code)
	}
	if _, found := findMessage(sentryEvents.messages(), "MP WEBHOOK SIGNATURE REJECTED"); !found {
		t.Errorf("a dropped payment notification must reach Sentry; captured %q", sentryEvents.messages())
	}
}
