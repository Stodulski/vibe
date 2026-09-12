package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stodulski/vibe-server/internal/data"
	slotguard "github.com/stodulski/vibe-server/internal/data/slotguard"
)

// This file holds the two halves of the double-booking defence that take a
// *Booking, and so cannot live in internal/data/slotguard with the locks and
// the calendar arithmetic: those take primitive arguments only, precisely so
// that the courts store can share them without importing this package.
//
// The payments store calls both of these under the same locks this one does —
// see slotguard's own file comment for why an insert and a confirmation have to
// be serialized against each other rather than only against their own kind.

// ReleaseStalePendingOverlaps cancels the public, unpaid bookings older than
// hold that overlap the hours b wants, so the exclusion constraint no longer
// counts them. It is the carve-out SlotTaken applies, made true in the table
// rather than only in the query; see InsertSafe for why the two must agree.
// The caller must hold lockCourtDay for that court and date.
//
// The note is the one the release cron writes, so a cancelled row reads the
// same whichever path got there first. The cron's other work — expiring the
// MercadoPago preference and emailing the visitor — does not happen here; a
// payment that arrives anyway is refused by guardSlotStillFree and refunded.
func ReleaseStalePendingOverlaps(ctx context.Context, tx pgx.Tx, b *Booking, hold time.Duration) error {
	_, err := tx.Exec(ctx, `
		UPDATE bookings
		SET status = 'cancelled',
		    notes  = COALESCE(notes || ' | ', '') || 'Cancelado automáticamente: tiempo de pago expirado'
		WHERE court_id = $1
		  AND span && tstzrange(
		        booking_starts_at($2, $3),
		        booking_starts_at($2, $3) + make_interval(mins => $4),
		        '[)')
		  AND status = 'pending'
		  AND collection_status = 'unpaid'
		  AND created_by IS NULL
		  AND created_at < NOW() - make_interval(secs => $5)`,
		data.UUIDToPg(b.CourtID), data.DateToPg(b.Date), data.TimeStrToPg(b.StartTime),
		b.DurationMinutes, hold.Seconds(),
	)
	if err != nil {
		return fmt.Errorf("release stale pending bookings: %w", err)
	}
	return nil
}

// SlotTaken reports whether a live booking already covers the hours b wants on
// its court and date. The caller must hold lockCourtDay for that court and date.
//
// exclude is the booking that is allowed to be found — the row being confirmed,
// which of course overlaps itself. uuid.Nil excludes nothing, which is what an
// insert wants.
//
// hold is how long an unpaid public booking keeps its slot, and it is a query
// parameter rather than a literal in the SQL on purpose: the carve-out below
// stops a stale pending booking from blocking the court, and the only thing that
// makes that safe is the cancellation cron cancelling it on the very same
// schedule. Those two numbers are the configured payment expiry, once, or the
// carve-out opens a window nothing closes.
// The overlap is asked of `span`, the same generated column the exclusion
// constraint uses, and the candidate's own range is built the identical way —
// its start instant plus its duration. The two cannot disagree about what
// overlaps what, which is the point: this pre-check exists to turn a genuine
// collision into ErrSlotUnavailable instead of a raw constraint violation, and
// it can only do that if it sees the same collisions the constraint does.
//
// It used to filter `date = $2` and compare times of day. Both were wrong once
// a booking could cross midnight: an existing 23:00–00:00 row has end_time
// '00:00', so `end_time > $3` was false against almost any candidate and the
// row was invisible here; and a booking filed under yesterday that runs into
// today was excluded by the date filter before the comparison even ran.
func SlotTaken(ctx context.Context, tx pgx.Tx, b *Booking, hold time.Duration, exclude uuid.UUID) (bool, error) {
	var taken bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM bookings
			WHERE court_id = $1
			  AND span && tstzrange(
			        booking_starts_at($2, $3),
			        booking_starts_at($2, $3) + make_interval(mins => $4),
			        '[)')
			  AND status NOT IN `+slotguard.ReleasedBookingStatuses+`
			  AND id <> $5
			  AND NOT (
			    status = 'pending'
			    AND collection_status = 'unpaid'
			    AND created_by IS NULL
			    AND created_at < NOW() - make_interval(secs => $6)
			  )
		)`,
		data.UUIDToPg(b.CourtID), data.DateToPg(b.Date), data.TimeStrToPg(b.StartTime),
		b.DurationMinutes, data.UUIDToPg(exclude), hold.Seconds(),
	).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check bookings: %w", err)
	}
	return taken, nil
}
