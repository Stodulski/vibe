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
// The note is HoldExpiryNote, the one the release cron writes, so a cancelled
// row reads the same whichever path got there first. Every released booking that went past
// the hold stage (no hold token, so it may carry a checkout link) is also
// recorded in booking_hold_expiries in the same statement, as the cron's own
// expiry does (Store.ExpireHold). The cron's other work — closing the
// MercadoPago checkout link, asking MercadoPago whether it was paid, emailing
// the visitor — does not happen here, inside a visitor's booking transaction:
// the record hands it to the release sweep, which closes the link and asks
// MercadoPago on its next run (bookings.Service.ReconcileExpiredHolds), so an
// approved payment whose webhook never arrived is still found and processed,
// and tells the visitor their booking expired once no payment was found.
// A payment that does reach the confirmation path is refunded with
// RefundCauseSlotTaken when these hours went to b, and reopens the booking
// when they are still free.
func ReleaseStalePendingOverlaps(ctx context.Context, tx pgx.Tx, b *Booking, hold time.Duration) error {
	_, err := tx.Exec(ctx, `
		WITH released AS (
			UPDATE bookings
			SET status = 'cancelled',
			    notes  = COALESCE(notes || ' | ', '') || $6::text
			WHERE court_id = $1
			  AND span && tstzrange(
			        booking_starts_at($2, $3),
			        booking_starts_at($2, $3) + make_interval(mins => $4),
			        '[)')
			  AND status = 'pending'
			  AND collection_status = 'unpaid'
			  AND created_by IS NULL
			  AND created_at < NOW() - make_interval(secs => $5)
			RETURNING id, complex_id, hold_token_hash
		)
		INSERT INTO booking_hold_expiries (booking_id, complex_id)
		SELECT id, complex_id FROM released WHERE hold_token_hash IS NULL
		ON CONFLICT (booking_id) DO NOTHING`,
		data.UUIDToPg(b.CourtID), data.DateToPg(b.Date), data.TimeStrToPg(b.StartTime),
		b.DurationMinutes, hold.Seconds(), HoldExpiryNote,
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
		)
		-- The hours a fixed weekly series is holding are taken too, whether or not
		-- its booking exists yet. An occurrence of the candidate's own series on
		-- its own date is the candidate itself — the job is about to settle it.
		OR EXISTS(
			SELECT 1
			FROM booking_series_occurrences(NULL, ARRAY[$1]::uuid[], $2::date - 1, $2::date + 1) o
			WHERE o.span && tstzrange(
			        booking_starts_at($2, $3),
			        booking_starts_at($2, $3) + make_interval(mins => $4),
			        '[)')
			  AND NOT (o.series_id IS NOT DISTINCT FROM $7::uuid AND o.occurs_on = $2::date)
		)
		-- So are the hours an open-play event holds on the court.
		OR EXISTS(
			SELECT 1
			FROM court_event_occupancy(NULL, ARRAY[$1]::uuid[], $2::date - 1, $2::date + 1) e
			WHERE e.span && tstzrange(
			        booking_starts_at($2, $3),
			        booking_starts_at($2, $3) + make_interval(mins => $4),
			        '[)')
		)`,
		data.UUIDToPg(b.CourtID), data.DateToPg(b.Date), data.TimeStrToPg(b.StartTime),
		b.DurationMinutes, data.UUIDToPg(exclude), hold.Seconds(), data.UUIDPtrToPg(b.SeriesID),
	).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check bookings: %w", err)
	}
	return taken, nil
}
