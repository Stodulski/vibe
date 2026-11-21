package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// finishQueuedRefund hands the attempt the immediate path queued to the retry
// job, as GetPendingDue would read it back with the cause the claim stored, and
// runs the job with MercadoPago answering.
func finishQueuedRefund(t *testing.T, f *fixture) {
	t.Helper()
	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("setup: want one queued attempt; got %d", len(f.payments.recordedFailure))
	}
	claim := f.payments.recordedFailure[0].claim
	f.failedRefunds.pending = []*paymentstore.FailedRefund{{
		ID: claim.AttemptID, PaymentID: claim.PaymentID, BookingID: claim.BookingID,
		RegistrationID: claim.RegistrationID, ComplexID: claim.ComplexID,
		MPPaymentID: claim.MPPaymentID, Amount: claim.RefundCentavos, Cause: claim.Cause,
	}}
	f.provider.refundErr = nil
	f.service.RetryFailedRefunds(t.Context())
}

// duplicateOnLiveBooking seeds a confirmed booking already paid by mp-OLD, so
// mp-123 arriving for it is a second payment.
func duplicateOnLiveBooking(t *testing.T) (*fixture, *bookingstore.Booking) {
	t.Helper()
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
	readableNotice(f, booking)
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}
	return f, booking
}

// S18: a late payment whose refund MercadoPago refused at first is finished by
// the retry job, and the client is told the reason the immediate path gives.
func TestARetriedLatePaymentRefundTellsTheClientWhy(t *testing.T) {
	withCapturedSentryEvents(t)
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	readableNotice(f, booking)
	f.provider.refundErr = errProvider

	_ = f.service.processApprovedPayment(t.Context(), booking,
		&mp.Payment{ID: 123, TransactionAmount: 1500, CollectorID: 111111111}, "mp-123")
	if len(f.notify.refunds) != 0 {
		t.Fatalf("setup: nothing came back yet; got %+v", f.notify.refunds)
	}
	finishQueuedRefund(t, f)

	if len(f.notify.refunds) != 1 {
		t.Fatalf("want one refund notice from the retry job; got %d", len(f.notify.refunds))
	}
	if r := f.notify.refunds[0]; r.Reason != latePaymentReason || r.Email != "ana@example.com" {
		t.Errorf("the retried refund must carry the late-payment reason; got %+v", r)
	}
	if len(f.notify.ownerDuplicates) != 0 {
		t.Error("a cancelled booking is not live: the owner must not be emailed")
	}
}

// S18: a duplicate payment on a live booking whose refund is finished by the
// retry job tells the client why and tells the owner, with the same keys the
// immediate path uses for the same refund.
func TestARetriedDuplicatePaymentRefundTellsTheClientAndTheOwnerWithTheImmediateKeys(t *testing.T) {
	withCapturedSentryEvents(t)
	immediate, _ := duplicateOnLiveBooking(t)
	if err := immediate.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(immediate.notify.refunds) != 1 || len(immediate.notify.ownerDuplicates) != 1 {
		t.Fatalf("setup: the immediate path tells both; got %d client and %d owner notices",
			len(immediate.notify.refunds), len(immediate.notify.ownerDuplicates))
	}

	retried, booking := duplicateOnLiveBooking(t)
	retried.provider.refundErr = errProvider
	_ = retried.service.processPaymentWebhook(t.Context(), "mp-123")
	finishQueuedRefund(t, retried)

	if len(retried.notify.refunds) != 1 {
		t.Fatalf("want one client notice from the retry job; got %d", len(retried.notify.refunds))
	}
	r := retried.notify.refunds[0]
	if r.Reason != duplicatePaymentReason || r.Cancelled {
		t.Errorf("the client must be told why, and not that the booking is off; got %+v", r)
	}
	if len(retried.notify.ownerDuplicates) != 1 {
		t.Fatalf("the owner of a live booking must be told from the retry path too; got %d", len(retried.notify.ownerDuplicates))
	}
	o := retried.notify.ownerDuplicates[0]
	if o.ClientName != "Ana Diaz" || o.BookingID != booking.ID.String() {
		t.Errorf("the owner's notice must name the client and the booking; got %+v", o)
	}
	// One refund, one key: whichever path records it builds the same one.
	want := immediate.notify.refunds[0].RefundID
	if r.RefundID != want || o.RefundID != immediate.notify.ownerDuplicates[0].RefundID {
		t.Errorf("retry keys client %q owner %q, immediate keys %q and %q",
			r.RefundID, o.RefundID, want, immediate.notify.ownerDuplicates[0].RefundID)
	}
}

// S18: an attempt with no stored cause, a cancellation's refund or a row
// written before the column, is told as before: the plain notice, no owner
// email, even on a live booking.
func TestARetriedRefundWithNoCauseSendsThePlainNotice(t *testing.T) {
	f, booking := duplicateOnLiveBooking(t)
	f.failedRefunds.pending = []*paymentstore.FailedRefund{{
		ID: uuid.New(), PaymentID: f.payments.byBooking.ID, BookingID: booking.ID,
		ComplexID: booking.ComplexID, MPPaymentID: "mp-OLD", Amount: 150_000,
	}}
	f.service.RetryFailedRefunds(t.Context())

	if len(f.notify.refunds) != 1 {
		t.Fatalf("want one refund notice; got %d", len(f.notify.refunds))
	}
	if r := f.notify.refunds[0]; r.Reason != "" || r.RefundID != refundKey("mp-OLD", 150_000) {
		t.Errorf("a refund with no cause gets the plain notice; got %+v", r)
	}
	if len(f.notify.ownerDuplicates) != 0 {
		t.Error("no cause, no owner email")
	}
}

// extraRegistrationPayment seeds a confirmed sign-up that a second payment,
// mp-123, lands on.
func extraRegistrationPayment(t *testing.T) (*fixture, *mp.Payment, *stubRegistrationNotifier, func() error) {
	t.Helper()
	f, reg, paid, notifier := registrationFixture(t)
	confirmSpot(reg)
	f.payments.byRegistration = []*paymentstore.Payment{checkoutRow(reg)}
	f.payments.regConfirmErr = paymentstore.ErrRegistrationAlreadyPaid
	run := func() error { return f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-123") }
	return f, paid, notifier, run
}

// A second payment on a sign-up still confirmed, whose refund the retry job
// finishes, is told as the payment that came back, with the notice the
// immediate path sends, never as a cancellation of the spot.
func TestARetriedExtraRegistrationPaymentRefundKeepsTheSpot(t *testing.T) {
	withCapturedSentryEvents(t)
	_, _, told, runImmediate := extraRegistrationPayment(t)
	if err := runImmediate(); err != nil {
		t.Fatal(err)
	}
	if len(told.refunded) != 1 {
		t.Fatalf("setup: the immediate path tells the refund; got %+v", told.refunded)
	}

	retried, _, notifier, runQueued := extraRegistrationPayment(t)
	retried.provider.refundErr = errProvider
	_ = runQueued()
	finishQueuedRefund(t, retried)

	if len(notifier.cancelled) != 0 {
		t.Errorf("the spot is kept: no cancellation notice; got %+v", notifier.cancelled)
	}
	if len(notifier.refunded) != 1 {
		t.Fatalf("want the payment-came-back notice from the retry job; got %+v", notifier.refunded)
	}
	got, want := notifier.refunded[0], told.refunded[0]
	if got.Amount != want.Amount || got.RefundedTotal != want.RefundedTotal || got.Email != want.Email {
		t.Errorf("retry notice %+v must match the immediate one %+v", got, want)
	}
}
