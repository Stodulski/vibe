// Package slotguard holds the double-booking defence: the advisory and row
// locks every writer that puts hours on a court must take, the calendar
// arithmetic those locks are keyed on, and the overlap question asked under
// them.
//
// It takes primitive arguments only — a transaction, a court id, instants —
// so that it sits below every domain store rather than inside one. The two
// writes that can put a booking on a court (bookings store) and the write
// that takes hours off sale (courts store) therefore contend on the same
// lock without either package importing the other.
package slotguard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// This package holds the double-booking defence itself, shared by the only two
// writes that can put a booking on a court: the insert (bookings store
// InsertSafe) and the payment confirmation that turns a pending booking into
// one which holds its slot (payments store InsertAndConfirmBooking /
// ConfirmWebhookPayment).
//
// Both take the same advisory lock on the same court and day and run the same
// overlap query inside their own transaction, so an insert and a confirmation
// racing for the same hours are serialized against each other rather than only
// against their own kind. Confirming without this check was how two confirmed
// bookings could end up ninety minutes on top of each other: a stale pending
// booking stops holding its slot, another client books over it, and the first
// one's webhook then confirmed it anyway.
//
// A third writer takes the same lock for a different reason: the blocked-slot
// write (courts store InsertBlockedSlot). It puts no booking on the court, it
// takes hours off sale — but bookings and blocks are two tables, and
// bookings_no_overlapping_span, like every EXCLUDE constraint, sees only one.
// The two invariants the database cannot carry are therefore carried here, by
// each side asking about the other under a lock they share.

// LockCourtDay serializes every writer touching one court on one day for the rest
// of the caller's transaction.
//
// The key is derived exactly as it always was — hashtext(court_id || date) — so
// the insert path and the confirmation path contend on the same lock. Changing
// how this key is built in one place and not the other would silently take the
// defence apart, which is why both callers go through here.
func LockCourtDay(ctx context.Context, tx pgx.Tx, courtID uuid.UUID, date time.Time) error {
	_, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1 || $2))`,
		courtID.String(), date.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	return nil
}

// LockCourtDays takes LockCourtDay for every local day the half-open range
// [start, end) touches, and it is what a writer whose hours can outlive their
// own date must call instead.
//
// One lock per date was enough while every write occupied exactly one date.
// A booking may cross midnight now, which ended that: one filed under D running 23:00 for two
// hours occupies D and D+1, while a maintenance block on D+1 keys its lock on
// D+1 — so the two contend on nothing, each runs its overlap check before the
// other's row exists, and both commit. The span comparison blocked_slots.span added
// is date-free and would have seen the collision; it just never ran at a
// moment when there was one to see. Serializing on every day the hours touch
// is what gives that check something to look at.
//
// The days are locked in ascending order, and that ordering is the deadlock
// argument: every writer takes the days it needs lowest-first, so no two can
// hold one another's next lock.
func LockCourtDays(ctx context.Context, tx pgx.Tx, courtID uuid.UUID, start, end time.Time) error {
	for _, day := range LocalDays(start, end) {
		if err := LockCourtDay(ctx, tx, courtID, day); err != nil {
			return err
		}
	}
	return nil
}

// LockCourtLive takes a row-level share lock on the court and refuses with
// data.ErrRecordNotFound when it is gone, for the rest of the caller's
// transaction.
//
// H-22. The advisory lock above serializes the writers that have a day to key
// it on. The courts store's SoftDelete has none — it removes the court
// outright — so it can never take that lock, and the two writes contended on
// nothing at all: under READ COMMITTED the booking's checks ran against a
// snapshot taken before the delete committed and the delete's ran against one
// taken before the booking did. Both were right about what they could see.
// Both committed.
//
// The court row is the one object both writes can hold, which is what makes
// this the place to serialize them. Callers must take it AFTER the advisory
// locks: every writer that takes both takes them advisory-then-row, and
// SoftDelete takes only the row, so no two of them can hold one another's next
// lock.
//
// FOR SHARE, not FOR UPDATE: this is a reader of the court, not a writer of
// it. Two bookings on different days of the same court have no reason to queue
// behind each other, and a share lock still blocks the delete's FOR UPDATE,
// which is the only conflict that matters here.
func LockCourtLive(ctx context.Context, tx pgx.Tx, courtID uuid.UUID) error {
	var id pgtype.UUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM courts WHERE id = $1 AND deleted_at IS NULL FOR SHARE`,
		data.UUIDToPg(courtID)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("lock court: %w", err)
	}
	return nil
}

// LocalDays lists, ascending, the local calendar days the half-open range
// [start, end) covers.
//
// The range is half-open on both sides of the comparison — '[)' is what
// bookings.span and blocked_slots.span are built with — so hours ending
// exactly at midnight belong to the day they started in and nothing is locked
// on their account. An empty or inverted range is one day, the one it starts
// on, rather than none: a writer that locks nothing is serialized against
// nothing.
func LocalDays(start, end time.Time) []time.Time {
	first := LocalMidnight(start)
	last := LocalMidnight(end.Add(-time.Nanosecond))
	if last.Before(first) {
		last = first
	}

	var days []time.Time
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return days
}

// LocalMidnight is the start of t's day on the product's wall-clock, which is
// the calendar the lock key, `date` columns and the generated spans all speak.
func LocalMidnight(t time.Time) time.Time {
	local := t.In(timezone.Argentina)
	y, m, d := local.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, timezone.Argentina)
}

// LocalRange projects a stored (date, "HH:MM") pair plus a duration onto the
// two instants the database would compute for it.
//
// It is the Go half of `booking_starts_at(date, start_time)` (db/migrations/001_init.sql),
// the function both generated spans are built from, and it is deliberately the
// only place that combines a date with a time of day: doing it at each call
// site is how a second, silently disagreeing definition of "when does this
// start" gets written. The date argument carries only a calendar day — pgx
// decodes a `date` column as midnight UTC, and a handler parses one as
// midnight in Argentina — so its Y/M/D is re-anchored here rather than trusted
// as an instant.
func LocalRange(date time.Time, startTime string, d time.Duration) (start, end time.Time) {
	start = LocalInstant(date, startTime)
	return start, start.Add(d)
}

// LocalInstant is the moment a stored (date, "HH:MM") pair names on the
// product's wall-clock. See LocalRange for why it is one function rather than
// an expression at each call site.
//
// The time of day is carried into time.Date as a field rather than added to
// midnight afterwards, which is what `(date + start_time) AT TIME ZONE 'zone'`
// does: the wall clock is normalized first and the zone resolves it second.
// Adding a duration to a midnight instant resolves the zone first, and the two
// disagree by an hour across a DST boundary. Argentina has had none since 2009,
// so today this is a distinction without a difference — but it is the same
// distinction that made a booking's end time unreliable, and the cheaper of the
// two spellings is also the wrong one.
func LocalInstant(date time.Time, hhmm string) time.Time {
	y, m, d := date.Date()
	seconds := data.TimeStrToPg(hhmm).Microseconds / 1_000_000
	return time.Date(y, m, d, 0, 0, int(seconds), 0, timezone.Argentina)
}

// ReleasedBookingStatuses are the booking statuses whose hours are back on
// sale. It is the SQL half of the definition the schema settled: a no_show
// is a booking whose court went unused — the same physical fact as a
// cancellation, recorded differently because the client owes for it — while a
// completed booking did take its hours and cannot release them.
//
// Every query that asks "did this booking take the court's hours out of
// circulation" spells the question `status NOT IN ` +
// ReleasedBookingStatuses, so the answer cannot drift between them again. It
// had: SlotTaken excluded no_show, the dashboard counters, the occupancy
// heatmap, the upcoming list and both deletion guards did not, and because
// the schema deliberately lets a no_show and its resale share one
// (court, date, start_time), every one of those counted that hour twice.
//
// Two copies cannot import this constant and are named here instead: the sqlc
// query GetBookedSlots in db/queries/bookings.sql, and the partial unique index
// idx_bookings_no_double, since retired by bookings_no_overlapping_span. A .sql
// file compiled by sqlc and an index definition stored inside Postgres have no
// way to read a Go constant; both carry a comment pointing back here.
//
// It is deliberately NOT the predicate for money. GetPaymentSummary counts what
// the venue collected, and a forfeited deposit is money the venue holds even
// though the court was released — see the comment there.
const ReleasedBookingStatuses = `('cancelled', 'no_show')`

// SpanTaken reports whether a live booking already covers an arbitrary stretch
// of a court's calendar. The caller must hold LockCourtDays for the local days
// that stretch touches.
//
// It is the bookings store's SlotTaken question asked from the other side of
// the fence: the blocked-slot write has a span in hand — the one blocked_slots
// generated for the row it just inserted — rather than a booking to build one
// from, and it wants to know whether taking those hours off sale would strand
// a client who already holds them.
//
// The predicate is the canonical one, spelled the same way SlotTaken and
// GetBookedSlots (db/queries/bookings.sql) spell it, carve-out included: an
// owner must not be refused a block by hours the storefront is already
// offering as free, and the storefront's definition of free is this one.
func SpanTaken(ctx context.Context, tx pgx.Tx, courtID uuid.UUID, span pgtype.Range[pgtype.Timestamptz], hold time.Duration) (bool, error) {
	var taken bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM bookings
			WHERE court_id = $1
			  AND span && $2::tstzrange
			  AND status NOT IN `+ReleasedBookingStatuses+`
			  AND NOT (
			    status = 'pending'
			    AND collection_status = 'unpaid'
			    AND created_by IS NULL
			    AND created_at < NOW() - make_interval(secs => $3)
			  )
		)`,
		data.UUIDToPg(courtID), span, hold.Seconds(),
	).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check bookings: %w", err)
	}
	return taken, nil
}
