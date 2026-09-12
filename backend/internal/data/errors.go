package data

import "errors"

// Sentinel errors returned by store methods across the data package.
var (
	// ErrRecordNotFound is returned when a lookup by ID or unique key matches no row.
	ErrRecordNotFound = errors.New("record not found")
	// ErrDuplicateBooking is returned when a booking would overlap an existing one.
	ErrDuplicateBooking = errors.New("duplicate booking")
	// ErrSlotUnavailable is returned when the requested court slot cannot be booked.
	ErrSlotUnavailable = errors.New("slot unavailable")
	// ErrInvalidCursor is returned when a pagination cursor cannot be parsed.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrCooldownActive is returned when a token resend is attempted before its cooldown expires.
	ErrCooldownActive = errors.New("cooldown active")
	// ErrSlotLocked is returned when the requested slot is held by another transaction.
	ErrSlotLocked = errors.New("slot is locked")
	// ErrBookingNotConfirmable is returned by PaymentModel.InsertAndConfirmBooking
	// when the booking it re-reads, inside the same transaction the payment is
	// about to be inserted in, is no longer one a payment can be recorded
	// against (cancelled, completed, or a no-show).
	//
	// ConfirmPayment reads booking.Status well before this transaction opens —
	// it is what the handler bases its own "cannot confirm a cancelled
	// booking" check on — and a client cancellation that commits in that gap
	// used to reach InsertAndConfirmBooking anyway: the UPDATE tried to move
	// the row from 'cancelled' back to 'confirmed',
	// bookings_forbid_status_reversal refused it, and the plain database error
	// fell through to a 500 after the owner had already taken the client's
	// cash (H-15). This sentinel is what lets the handler answer that
	// specific situation with a 409 instead.
	ErrBookingNotConfirmable = errors.New("booking is no longer confirmable")
	// ErrBookingCancelled is the cancelled half of that same re-read, split out
	// because the two halves have opposite consequences for the client's money.
	//
	// H-23. A public booking that goes unpaid past the payment expiry stops
	// holding its slot, and the next InsertSafe to want those hours cancels it
	// outright (ReleaseStalePendingOverlaps). That path deliberately does NOT
	// expire the MercadoPago preference — its own comment says so — so a
	// payment for the cancelled booking can still arrive, and when it does the
	// money has to go back. The webhook refunds on ErrSlotUnavailable, and
	// while guardBookingConfirmable answered every unconfirmable status with
	// one sentinel, this case reached the generic branch instead: no refund,
	// and the event requeued forever against a write that can never succeed.
	// The client paid and got nothing.
	//
	// Completed and no_show stay on ErrBookingNotConfirmable, and must: the
	// client owes for those hours, so a late payment against them is money the
	// venue keeps, not money to send back.
	ErrBookingCancelled = errors.New("booking was cancelled")
)
