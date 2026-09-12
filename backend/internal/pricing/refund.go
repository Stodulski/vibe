package pricing

import (
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Whether a cancellation earns the client their deposit back is a money rule,
// which is why it lives here rather than with the booking handlers: the
// payments module needs the same answer, and the two must never disagree.

// WithinStandardWindow checks if the booking is within the standard cancellation window
// (e.g. more than 24h before the game). Does NOT consider the grace period.
func WithinStandardWindow(booking *bookingstore.Booking, cancellationHours int) bool {
	if cancellationHours <= 0 {
		return true
	}
	argTZ := timezone.Argentina
	//nolint:errcheck // StartTime is a validated "HH:MM" column; an unparseable one yields midnight, which only narrows the window
	startTimeObj, _ := time.Parse("15:04", booking.StartTime)
	bookingStart := time.Date(
		booking.Date.Year(), booking.Date.Month(), booking.Date.Day(),
		startTimeObj.Hour(), startTimeObj.Minute(), 0, 0, argTZ,
	)
	deadline := bookingStart.Add(-time.Duration(cancellationHours) * time.Hour)
	return time.Now().In(argTZ).Before(deadline)
}

// RefundDeadline reports the instant after which cancelling this booking no
// longer earns the deposit back — the later of the two moments CanRefund
// honours: the grace period from booking creation, and the complex's own
// cancellation window before the booking starts.
//
// For a complex with no window at all (cancellationHours <= 0) CanRefund is
// unconditionally true, so there is no real deadline; this reports the
// booking's own start as the nearest thing to one, which is what a client
// display means by "refundable until it starts".
//
// CanRefund calls this rather than recomputing the same two moments, so the
// two can never disagree about where the window sits.
func RefundDeadline(booking *bookingstore.Booking, cancellationHours int, gracePeriod time.Duration) time.Time {
	argTZ := timezone.Argentina
	//nolint:errcheck // StartTime is a validated "HH:MM" column; an unparseable one yields midnight, which only narrows the window
	startTimeObj, _ := time.Parse("15:04", booking.StartTime)
	bookingStart := time.Date(
		booking.Date.Year(), booking.Date.Month(), booking.Date.Day(),
		startTimeObj.Hour(), startTimeObj.Minute(), 0, 0, argTZ,
	)

	if cancellationHours <= 0 {
		return bookingStart
	}

	graceEnd := booking.CreatedAt.In(argTZ).Add(gracePeriod)
	windowDeadline := bookingStart.Add(-time.Duration(cancellationHours) * time.Hour)
	if graceEnd.After(windowDeadline) {
		return graceEnd
	}
	return windowDeadline
}

// CanRefund reports whether cancelling this booking earns the deposit back.
//
// It is true within the complex's own cancellation window, and also within a
// grace period after the booking was made — so a client who books a slot less
// than the window away can still change their mind immediately.
func CanRefund(booking *bookingstore.Booking, cancellationHours int, gracePeriod time.Duration) bool {
	if cancellationHours <= 0 {
		return true
	}

	argTZ := timezone.Argentina
	now := time.Now().In(argTZ)
	return now.Before(RefundDeadline(booking, cancellationHours, gracePeriod))
}

// LinkLive reports whether a booking's public-route access token should still
// be honored.
//
// The right-hand term is the same call the refund dispatch makes. "Expired"
// therefore cannot be true while a refund is owed — not because end+buffer
// happens to sit after both deadlines, but because A implies (A or B) for
// every value of every number in this file, including cancellationHours == 0
// and a negative buffer. This is a tautology, not a numeric-dominance claim:
// see design.md Decision 2, which found the proposal's original ground
// (both CanRefund deadlines fall strictly before booking end) false for a
// cancellationHours == 0 complex, where CanRefund is unconditionally true and
// no deadline exists at all.
func LinkLive(b *bookingstore.Booking, expiresAt time.Time, cancellationHours int, grace time.Duration, now time.Time) bool {
	return now.Before(expiresAt) || CanRefund(b, cancellationHours, grace)
}
