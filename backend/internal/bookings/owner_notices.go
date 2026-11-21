package bookings

import (
	"context"
	"errors"
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// The client notices of the owner's own actions on a booking: a cancellation
// through the general edit, a no-show, a counter payment and a cash refund
// returned by hand. Each runs after its write committed, so a failed read here
// costs the notice and never the action; the skip is logged and alerted, the
// way notifyCancelledAndRelease reports its own.

// manualRefundReason is the sentence a client returned money by hand reads.
const manualRefundReason = "El complejo te lo devolvió directamente."

// manualRefundSubject names the one manual close-out a booking can have (it
// moves the booking off 'partial' for good) in the notice's deduplication key.
const manualRefundSubject = "manual"

// noticeSubject is what every booking notice names: the complex, the court and
// the person to write to, with the booking's own typed address before the
// client row's (bookingstore.ContactOf).
type noticeSubject struct {
	complex *complexstore.Complex
	court   *courtstore.Court
	contact bookingstore.Contact
}

// readNoticeSubject reads what a notice about booking names. ok is false when
// any of it cannot be read; the skip is logged and alerted under op, naming the
// complex only (the booking id stays in the log, as on the public paths).
func (s *Service) readNoticeSubject(ctx context.Context, complexID uuid.UUID, booking *bookingstore.Booking, op string) (noticeSubject, bool) {
	complex, cerr := s.complexes.GetByID(ctx, complexID)
	court, courterr := s.courts.GetByID(ctx, booking.CourtID)
	contact, contacterr := bookingstore.ContactOf(ctx, s.clients, booking)
	if err := errors.Join(cerr, courterr, contacterr); err != nil {
		s.logger.Error(op+": the client notice was not sent, a read failed",
			"error", err, "booking_id", booking.ID, "complex_id", complexID)
		sentry.CaptureMessage(fmt.Sprintf("%s NOTICE NOT SENT (a read failed, client not told): complex_id=%s", op, complexID))
		return noticeSubject{}, false
	}
	return noticeSubject{complex: complex, court: court, contact: contact}, true
}

// editCancelOutcome is what an owner's cancellation through the general edit
// did about the client's money. The edit never refunds, so the notice only
// reports what the row already says, and claims a refund was sent only when
// the row records one (refund_status 'full'). Every other paid case is the
// complex's to settle by hand: 'partial' is cash owed, and on a live booking
// 'pending' is only ever the owner's own mark (the refund pipeline writes it on
// bookings it cancels, which the edit cannot cancel again), so no automatic
// "your refund is back" notice will follow. A paid booking with no refund at
// all is reachable when one request cancels and marks paid together, since the
// guard reads the booking before the request's collection status lands.
func editCancelOutcome(booking *bookingstore.Booking) paymentstore.RefundOutcome {
	switch {
	case booking.CollectionStatus == bookingstore.CollectionStatusUnpaid:
		return paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "the booking was never paid"}
	case booking.RefundStatus == bookingstore.RefundStatusFull:
		return paymentstore.RefundOutcome{Result: paymentstore.RefundAlreadyIssued, Reason: "the refund was already issued"}
	default:
		return paymentstore.RefundOutcome{Result: paymentstore.RefundManual, Reason: "the complex settles the money by hand"}
	}
}

// finishEditCancel runs the rest of the owner's cancel flow for a booking the
// general edit just cancelled: the checkout link of an unpaid booking is
// closed, the client is told with what became of their money, the held slot is
// released, and future hours go back to the waitlist and the freed-slot
// listener. Those are Cancel's own steps (expireCheckoutPreference,
// notifyCancelledAndRelease), so both owner paths say and free the same
// things. The refund step is the one difference, and editCancelOutcome says why.
func (s *Service) finishEditCancel(ctx context.Context, complexID uuid.UUID, booking *bookingstore.Booking, previousStatus bookingstore.BookingStatus) {
	complex, err := s.complexes.GetByID(ctx, complexID)
	if err != nil {
		s.logger.Error("update booking: the cancellation was saved but its notice and release did not run, the complex could not be read",
			"error", err, "booking_id", booking.ID, "complex_id", complexID)
		sentry.CaptureMessage(fmt.Sprintf("CANCELLATION NOTICE NOT SENT (a read failed, client not told): complex_id=%s", complexID))
		return
	}
	outcome := editCancelOutcome(booking)
	if booking.CollectionStatus == bookingstore.CollectionStatusUnpaid {
		s.expireCheckoutPreference(ctx, booking, complex)
	}
	s.notifyCancelledAndRelease(ctx, booking, complex, outcome, notifications.SourceOwnerCancel, previousStatus)
}

// notifyNoShow tells the client the owner marked their booking as not attended.
func (s *Service) notifyNoShow(ctx context.Context, complexID uuid.UUID, booking *bookingstore.Booking) {
	subj, ok := s.readNoticeSubject(ctx, complexID, booking, "NO-SHOW")
	if !ok {
		return
	}
	s.notify.BookingNoShow(notifications.NoShow{
		Email:        subj.contact.Email,
		ComplexName:  subj.complex.Name,
		CourtName:    subj.court.Name,
		Date:         booking.Date.Format("02/01"),
		StartTime:    timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
		BookURL:      booklink.Book(s.cfg.PublicURL, subj.complex.Slug),
		ComplexPhone: subj.complex.Phone,
		OwnerID:      subj.complex.OwnerID.String(),
		BookingID:    booking.ID.String(),
	})
}

// notifyCounterPayment sends the client a receipt for a payment the owner took
// at the counter. booking is the row as the payment left it, so its balance is
// what is still owed.
func (s *Service) notifyCounterPayment(ctx context.Context, complexID uuid.UUID, booking *bookingstore.Booking, payment *paymentstore.Payment) {
	subj, ok := s.readNoticeSubject(ctx, complexID, booking, "PAYMENT RECEIPT")
	if !ok {
		return
	}
	balance := ""
	if due := booking.BalanceDue(); due > 0 {
		balance = notifications.FormatARS(due)
	}
	s.notify.PaymentReceived(notifications.PaymentReceipt{
		Email:         subj.contact.Email,
		ComplexName:   subj.complex.Name,
		CourtName:     subj.court.Name,
		Date:          booking.Date.Format("02/01"),
		StartTime:     timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
		Amount:        notifications.FormatARS(payment.Amount),
		Method:        notifications.PaymentMethodPhrase(payment.Method),
		BalanceAmount: balance,
		BookURL:       booklink.Book(s.cfg.PublicURL, subj.complex.Slug),
		ComplexPhone:  subj.complex.Phone,
		OwnerID:       subj.complex.OwnerID.String(),
		BookingID:     booking.ID.String(),
		PaymentID:     payment.ID.String(),
	})
}

// notifyManualRefund tells the client the owner returned returned centavos by
// hand, once the close-out committed.
func (s *Service) notifyManualRefund(ctx context.Context, complexID uuid.UUID, booking *bookingstore.Booking, returned int) {
	if returned <= 0 {
		return
	}
	subj, ok := s.readNoticeSubject(ctx, complexID, booking, "MANUAL REFUND")
	if !ok {
		return
	}
	if err := s.notify.DepositRefunded(notifications.Refund{
		Email:        subj.contact.Email,
		ComplexName:  subj.complex.Name,
		Amount:       notifications.FormatARS(returned),
		BookURL:      booklink.Book(s.cfg.PublicURL, subj.complex.Slug),
		Reason:       manualRefundReason,
		ByHand:       true,
		ComplexPhone: subj.complex.Phone,
		OwnerID:      subj.complex.OwnerID.String(),
		BookingID:    booking.ID.String(),
		RefundID:     manualRefundSubject,
	}); err != nil {
		s.logger.Error("manual refund: the client's notice was not queued",
			"error", err, "booking_id", booking.ID, "complex_id", complexID)
		sentry.CaptureMessage(fmt.Sprintf("MANUAL REFUND NOTICE NOT QUEUED (client not told): complex_id=%s", complexID))
	}
}
