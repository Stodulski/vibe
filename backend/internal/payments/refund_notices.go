package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Why a refund the webhook issued on its own came back, as the client reads it
// in the refund notice. Each is sent only after the refund is recorded, so the
// client is never told about money that has not moved.
const (
	// duplicatePaymentReason: a second payment landed on a booking another
	// payment had already settled (refundSettledBookingPayment).
	duplicatePaymentReason = "Recibimos dos pagos por la misma reserva, así que te devolvimos el que sobraba."
	// latePaymentReason: the payment arrived for a booking already cancelled.
	latePaymentReason = "El pago llegó cuando la reserva ya estaba cancelada."
	// slotTakenReason: the hours were sold to somebody else while the payment
	// was in flight, so the booking could not be confirmed.
	slotTakenReason = "No pudimos confirmar tu reserva: el turno se ocupó antes de que se acreditara tu pago."
)

// slotLostCause says why a payment the confirmation refused is going back:
// the booking had already been cancelled (its payment window ran out), or its
// hours went to somebody else.
func slotLostCause(err error) paymentstore.RefundCause {
	if errors.Is(err, bookingstore.ErrBookingCancelled) {
		return paymentstore.RefundCauseLatePayment
	}
	return paymentstore.RefundCauseSlotTaken
}

// clientReason is the sentence the refund notice gives for cause. A refund
// somebody asked for, or a cause that is not a booking's, gives none.
func clientReason(cause paymentstore.RefundCause) string {
	switch cause {
	case paymentstore.RefundCauseLatePayment:
		return latePaymentReason
	case paymentstore.RefundCauseSlotTaken:
		return slotTakenReason
	case paymentstore.RefundCauseDuplicatePayment:
		return duplicatePaymentReason
	case paymentstore.RefundCauseNone, paymentstore.RefundCauseRegistrationSpotGone,
		paymentstore.RefundCauseRegistrationExtraPayment:
		return ""
	}
	return ""
}

// announceAutomaticRefund tells the client a booking's refund is back, with the
// reason its cause gives, and tells the owner when the refund was a second
// payment on a booking that still holds its court: their booking stands and a
// payment of it went back. Both paths that record a refund call it, the webhook
// and the retry job, and both key it on the payment and the total written
// (refundKey), so one refund is one notice whichever path records it.
func (s *Service) announceAutomaticRefund(ctx context.Context, booking *bookingstore.Booking, mpPaymentID string, refundTotal int, cause paymentstore.RefundCause) {
	key := refundKey(mpPaymentID, refundTotal)
	s.sendRefundNotificationWithReason(ctx, booking, refundTotal, key, false, clientReason(cause))
	if cause == paymentstore.RefundCauseDuplicatePayment && liveBooking(booking.Status) {
		s.announceDuplicatePaymentRefund(ctx, booking, refundTotal, key)
	}
}

// announceDuplicatePaymentRefund tells the owner that a client's second payment
// on a live booking went back through MercadoPago. It runs after the refund is
// recorded; a read that fails costs the notice and is logged.
func (s *Service) announceDuplicatePaymentRefund(ctx context.Context, booking *bookingstore.Booking, refundCentavos int, refundID string) {
	complex, err := s.complexes.GetByID(ctx, booking.ComplexID)
	if err != nil {
		s.logger.Error("duplicate payment refund: the owner was not told, the complex could not be read",
			"error", err, "booking_id", booking.ID)
		return
	}
	court, err := s.courts.GetByID(ctx, booking.CourtID)
	if err != nil {
		s.logger.Error("duplicate payment refund: the owner was not told, the court could not be read",
			"error", err, "booking_id", booking.ID)
		return
	}
	client, err := bookingstore.ContactOf(ctx, s.clients, booking)
	if err != nil {
		s.logger.Error("duplicate payment refund: the owner was not told, the client could not be read",
			"error", err, "booking_id", booking.ID)
		return
	}
	s.notify.OwnerDuplicatePaymentRefunded(notifications.OwnerDuplicatePaymentRefund{
		OwnerID:     complex.OwnerID.String(),
		ComplexName: complex.Name,
		CourtName:   court.Name,
		ClientName:  client.FullName(),
		Date:        booking.Date.Format("02/01"),
		StartTime:   timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
		Amount:      formatARS(refundCentavos),
		BookingID:   booking.ID.String(),
		RefundID:    refundID,
	})
}

// announceRefundOwedByHand tells the owner that owedCentavos of booking are owed
// back by hand, and the client that the complex returns them. It runs once the
// booking was moved to 'partial', which is what lets the owner close the
// balance out. attemptID is the exhausted refund attempt that left it owed, and
// keys both notices.
func (s *Service) announceRefundOwedByHand(ctx context.Context, booking *bookingstore.Booking, owedCentavos int, attemptID uuid.UUID) {
	complex, err := s.complexes.GetByID(ctx, booking.ComplexID)
	if err != nil {
		s.logger.Error("refund owed by hand: nobody was told, the complex could not be read",
			"error", err, "booking_id", booking.ID)
		return
	}
	courtName := ""
	if court, err := s.courts.GetByID(ctx, booking.CourtID); err == nil {
		courtName = court.Name
	} else {
		s.logger.Warn("refund owed by hand: the court could not be read, the notice names none",
			"error", err, "booking_id", booking.ID)
	}
	n := notifications.RefundOwed{
		ComplexName:  complex.Name,
		CourtName:    courtName,
		Date:         booking.Date.Format("02/01"),
		StartTime:    timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
		Amount:       formatARS(owedCentavos),
		BookURL:      booklink.Book(s.cfg.PublicURL, complex.Slug),
		ComplexPhone: complex.Phone,
		OwnerID:      complex.OwnerID.String(),
		BookingID:    booking.ID.String(),
		Subject:      attemptID.String(),
	}
	// The owner is told who is owed even when the client cannot be reached.
	if client, err := bookingstore.ContactOf(ctx, s.clients, booking); err == nil {
		n.Email, n.ClientName = client.Email, client.FullName()
	} else {
		s.logger.Error("refund owed by hand: the client could not be read, only the owner is told",
			"error", err, "booking_id", booking.ID)
	}
	s.notify.RefundOwedByVenue(n)
}
