package pricing

import (
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Whether a cancellation earns the client their deposit back is a money rule,
// which is why it lives here rather than with the booking handlers: the
// payments module needs the same answer, and the two must never disagree.

// BookingStart is the instant a booking's game begins: its date and wall-clock
// start, read in Argentina, the same combination booking_starts_at makes in SQL.
func BookingStart(booking *bookingstore.Booking) time.Time {
	//nolint:errcheck // StartTime is a validated "HH:MM" column; an unparseable one yields midnight, which only narrows the window
	startTimeObj, _ := time.Parse("15:04", booking.StartTime)
	return time.Date(
		booking.Date.Year(), booking.Date.Month(), booking.Date.Day(),
		startTimeObj.Hour(), startTimeObj.Minute(), 0, 0, timezone.Argentina,
	)
}

// HasStarted reports whether the booking's game has begun at now. A booking
// that has started can no longer be cancelled by its client, and nothing
// about it is refundable any more (see RefundDeadline).
func HasStarted(booking *bookingstore.Booking, now time.Time) bool {
	return !now.Before(BookingStart(booking))
}

// WithinStandardWindow checks if the booking is within the standard cancellation window
// (e.g. more than 24h before the game). Does NOT consider the grace period.
func WithinStandardWindow(booking *bookingstore.Booking, cancellationHours int) bool {
	if cancellationHours <= 0 {
		return true
	}
	deadline := BookingStart(booking).Add(-time.Duration(cancellationHours) * time.Hour)
	return time.Now().In(timezone.Argentina).Before(deadline)
}

// RefundDeadline reports the instant after which cancelling this booking no
// longer earns the deposit back — the later of the two moments CanRefund
// honours: the grace period from booking creation, and the complex's own
// cancellation window before the booking starts.
//
// Neither ever reaches past the booking's own start. A complex with no window
// at all (cancellationHours <= 0) refunds until the start and not after, and a
// grace period that would run into the game (a booking made minutes before
// it) ends at the start too: hours already being played are not taken back.
//
// CanRefund calls this rather than recomputing the same moments, so the two
// can never disagree about where the window sits.
func RefundDeadline(booking *bookingstore.Booking, cancellationHours int, gracePeriod time.Duration) time.Time {
	bookingStart := BookingStart(booking)
	if cancellationHours <= 0 {
		return bookingStart
	}

	graceEnd := booking.CreatedAt.In(timezone.Argentina).Add(gracePeriod)
	deadline := bookingStart.Add(-time.Duration(cancellationHours) * time.Hour)
	if graceEnd.After(deadline) {
		deadline = graceEnd
	}
	if deadline.After(bookingStart) {
		return bookingStart
	}
	return deadline
}

// CanRefund reports whether cancelling this booking earns the deposit back.
//
// It is true within the complex's own cancellation window, and also within a
// grace period after the booking was made — so a client who books a slot less
// than the window away can still change their mind immediately — and never
// once the booking has started (see RefundDeadline).
func CanRefund(booking *bookingstore.Booking, cancellationHours int, gracePeriod time.Duration) bool {
	return time.Now().In(timezone.Argentina).Before(RefundDeadline(booking, cancellationHours, gracePeriod))
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
