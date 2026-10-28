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

// A second approved payment on a booking that is already settled used to be
// dropped without a trace. It is money the complex collected, so it has to be
// reported; the booking and the payment already recorded must not move.
func TestASecondApprovedPaymentOnASettledBookingIsReportedNotDropped(t *testing.T) {
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

			// The deposit already recorded is a different MercadoPago payment from
			// the one arriving now, which is the case that used to vanish.
			recorded := "mp-OLD"
			deposit.MPPaymentID = &recorded
			f.payments.byBooking = deposit
			f.complexes.complex = linkedComplex(complexID, "111111111")

			mpPayment := &mp.Payment{
				ID: 123, Status: "approved", TransactionAmount: 1500,
				CollectorID: 111111111, ExternalReference: booking.ID.String(),
			}
			if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
				t.Fatalf("the money is already captured, so the event must not be requeued; got %v", err)
			}

			if booking.Status != status {
				t.Errorf("the booking status must not change; got %q, want %q", booking.Status, status)
			}
			if deposit.MPPaymentID == nil || *deposit.MPPaymentID != recorded {
				t.Error("the payment already recorded must not be rewritten to point at the second payment")
			}
			if f.payments.inserted != nil || f.payments.confirmed != nil || f.bookings.updated != nil {
				t.Error("nothing may be written for the second payment")
			}
			if _, found := findMessage(sentryEvents.messages(), "SETTLED BOOKING PAID AGAIN"); !found {
				t.Errorf("the second payment must be reported; captured %q", sentryEvents.messages())
			}
		})
	}
}

// The report is only for money the complex actually collected. A payment from
// another seller is not this club's to chase, so it stays a fraud-alert log
// line and nothing more.
func TestASecondPaymentOnASettledBookingFromAForeignCollectorIsNotReported(t *testing.T) {
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

	if _, found := findMessage(sentryEvents.messages(), "SETTLED BOOKING PAID AGAIN"); found {
		t.Error("a payment collected by another seller must not be reported as the complex's money")
	}
	if _, found := findMessage(sentryEvents.messages(), "FRAUD ALERT: collector mismatch"); !found {
		t.Errorf("the collector mismatch must still be reported; captured %q", sentryEvents.messages())
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
