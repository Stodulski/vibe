package data

import "errors"

// Sentinel errors returned by store methods across the data package.
var (
	// ErrRecordNotFound is returned when a lookup by ID or unique key matches no row.
	ErrRecordNotFound = errors.New("record not found")
	// ErrDuplicateSlug is returned when a complex's public slug is already
	// taken.
	//
	// complexes.slug is UNIQUE across the whole table (complexes_slug_key), not
	// only across live rows: a soft-deleted venue keeps its slug. That is
	// deliberate — the slug is the public URL clients hold in links, messages
	// and search results, so handing it to a different venue would silently
	// redirect one business's inbound traffic to another. Reusing a deleted
	// complex's slug is therefore this error, not an ordinary insert. Contrast
	// ErrDuplicateCourtName, whose constraint is partial.
	ErrDuplicateSlug = errors.New("duplicate slug")
	// ErrDuplicateBooking is returned when a booking would overlap an existing one.
	ErrDuplicateBooking = errors.New("duplicate booking")
	// ErrSlotUnavailable is returned when the requested court slot cannot be booked.
	ErrSlotUnavailable = errors.New("slot unavailable")
	// ErrInvalidCursor is returned when a pagination cursor cannot be parsed.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrAlreadyRefunded is returned when a refund is attempted on an already-refunded payment.
	ErrAlreadyRefunded = errors.New("payment already refunded")
	// ErrRefundInFlight is returned when a refund is attempted on a payment another
	// claim has already reserved.
	//
	// It is deliberately not ErrAlreadyRefunded: that one means the money is back and
	// there is nothing left to do, while this one means the money has not necessarily
	// moved yet but a durable attempt already exists and will be worked exactly once.
	// A caller must not queue a second attempt for either, but only the first may tell
	// a client their refund is done.
	ErrRefundInFlight = errors.New("refund already in flight")
	// ErrCooldownActive is returned when a token resend is attempted before its cooldown expires.
	ErrCooldownActive = errors.New("cooldown active")
	// ErrSlotLocked is returned when the requested slot is held by another transaction.
	ErrSlotLocked = errors.New("slot is locked")
	// ErrSlotAlreadyBlocked is returned when a blocked-slot insert overlaps an existing blocked slot.
	ErrSlotAlreadyBlocked = errors.New("slot already blocked")
	// ErrSlotHasBooking is returned when a blocked-slot insert would close hours
	// a live booking already holds.
	//
	// It is the mirror of ErrSlotUnavailable, which is what the booking side
	// answers when the block got there first, and it is deliberately a
	// different sentinel: the two refusals have to be told apart to be
	// explained ("someone already booked this" versus "the court is closed"),
	// and only one of them means the caller should look for another court.
	// blocked_slots_no_overlapping_span cannot raise it — EXCLUDE is
	// single-table — so it comes from a query InsertBlockedSlot runs inside the
	// same transaction, under the same court-day lock the booking path takes.
	ErrSlotHasBooking = errors.New("slot already booked")
	// ErrDuplicateCourtName is returned when a complex already has a live court with that name.
	//
	// courts_active_name_unique (db/migrations/001_init.sql) is what detects it. The
	// constraint is partial — soft-deleted courts are excluded — so reusing the
	// name of a deleted court is not this error, it is an ordinary insert.
	ErrDuplicateCourtName = errors.New("duplicate court name")
	// ErrOverlappingPriceRule is returned when a price rule would overlap another
	// rule for the same court and weekday.
	//
	// court_prices_no_overlapping_rule (db/migrations/001_init.sql) is what detects it.
	// Overlapping rules make findPrice's first-match loop depend on which row the
	// plan emits first, so the price a client is shown and the price they are
	// charged can differ; the constraint is what makes that unrepresentable.
	// Adjacent rules that share an endpoint (08:00-12:00 then 12:00-23:00) do not
	// overlap and are not this error.
	ErrOverlappingPriceRule = errors.New("overlapping price rule")
	// ErrNoManualRefundOwed is returned by RecordManualRefund when the booking
	// it locks does not read 'partial_refund' at write time — either it never
	// did, or another request already closed it out.
	ErrNoManualRefundOwed = errors.New("no manual refund owed")
	// ErrCourtHasActiveBookings is returned by CourtModel.SoftDelete when the
	// court still owes someone their hours.
	//
	// It used to be a separate HasActiveBookingsByCourt call the handler made
	// before SoftDelete, with nothing serializing the two: a booking that
	// committed in the gap between them survived on a court the owner had just
	// watched disappear from their own dashboard (H-02). SoftDelete now asks
	// and acts in one statement, so this error means the database itself
	// refused the delete at the moment it tried it, not that a caller-side
	// check some time earlier said no.
	ErrCourtHasActiveBookings = errors.New("court has active bookings")
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
