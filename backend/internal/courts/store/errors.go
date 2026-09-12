package store

import "errors"

// Sentinel errors returned by this store.
var (
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
	// ErrCourtHasActiveBookings is returned by Store.SoftDelete when the
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
)
