package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// This file holds the booking_hold_expiries record (044_booking_hold_expiries.sql):
// which storefront bookings were cancelled only because their payment window
// ran out. Two paths write it, both in the write that cancels the booking —
// ExpireHold below for the release sweep, and ReleaseStalePendingOverlaps for
// a new booking landing on stale hours — and two read it: the release sweep,
// which asks MercadoPago about each one once (ExpiredHoldsToReconcile), and
// the payment confirmation, which reopens such a booking when a late payment
// finds its hours still free (GetByID's CancelledByHoldExpiry).

// ExpireHold cancels b with its notes — the caller has moved it to
// 'cancelled' with the expiry note — and records it as cancelled by hold
// expiry, in one transaction. It is Update for the release sweep: a booking
// cancelled without the record would never be asked about again, nor reopened
// by its late payment.
//
// Only while the booking is still pending and unpaid: b was read before the
// sweep asked MercadoPago, and a client cancellation or a payment committed
// since must not be turned into an expiry (a reopenable booking, or a paid one
// cancelled). Such a booking is left as it is and ErrEditConflict returned.
func (m *Store) ExpireHold(ctx context.Context, b *Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return err
	}

	return m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		dbBooking, err := q.ExpireUnpaidBooking(ctx, db.ExpireUnpaidBookingParams{
			ID:    data.UUIDToPg(b.ID),
			Notes: data.TextToPg(b.Notes),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrEditConflict
			}
			return err
		}
		b.Status = BookingStatus(dbBooking.Status)
		if err := q.InsertBookingHoldExpiry(ctx, db.InsertBookingHoldExpiryParams{
			BookingID: data.UUIDToPg(b.ID),
			ComplexID: data.UUIDToPg(b.ComplexID),
		}); err != nil {
			return fmt.Errorf("bookings: record hold expiry: %w", err)
		}
		b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)
		b.ClientSnapshot = snapshotFromDB(dbBooking)
		b.ContactEmail = dbBooking.ClientEmail.String
		return nil
	})
}

// ExpiredHoldsToReconcile returns up to limit bookings cancelled by hold
// expiry within the last lookback that the release sweep has not yet asked
// MercadoPago about, has failed on fewer than maxFailures times and whose
// next attempt is due (RecordHoldReconcileFailure backs it off), oldest
// first, enriched like GetExpiredPendingEnriched (the seller credential is
// what MercadoPago is asked with).
//
// A booking with no payment row is returned too. Only a storefront booking
// that went past the hold stage is ever recorded here (an untouched slot hold
// never is), and MercadoPago is searched by the booking id alone: the public
// path creates the checkout link before it saves the payment row, and only
// logs a failed save, so a missing row does not mean nothing could be paid.
// One that never reached MercadoPago costs one search that finds nothing.
func (m *Store) ExpiredHoldsToReconcile(ctx context.Context, lookback time.Duration, limit, maxFailures int) ([]*CronBooking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT `+BookingColumns+`,
		       COALESCE(cl.phone, b.client_phone, ''), COALESCE(NULLIF(b.client_email, ''), cl.email, ''), cx.name, co.name,
		       cx.slug, cx.address, cx.city, cx.latitude, cx.longitude,
		       cx.mp_access_token, cx.mp_refresh_token, cx.cancellation_hours,
		       cx.phone, cx.owner_id
		FROM booking_hold_expiries e
		JOIN bookings b ON b.id = e.booking_id
		LEFT JOIN clients cl ON cl.id = b.client_id
		JOIN complexes cx ON cx.id = b.complex_id
		JOIN courts co ON co.id = b.court_id
		WHERE e.reconciled_at IS NULL
		  AND e.expired_at > NOW() - make_interval(secs => $1)
		  AND e.failed_attempts < $3
		  AND e.next_attempt_at <= NOW()
		  AND b.status = 'cancelled'
		ORDER BY e.expired_at
		LIMIT $2`, lookback.Seconds(), limit, maxFailures)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCronBookings(rows, m.Keys)
}

// MarkHoldReconciled records that MercadoPago was asked about the booking's
// checkout and any approved payment was handed over, so the sweep does not
// ask again.
func (m *Store) MarkHoldReconciled(ctx context.Context, bookingID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	return m.Q.MarkBookingHoldReconciled(ctx, data.UUIDToPg(bookingID))
}

// RecordHoldReconcileFailure counts one more run that could not settle the
// booking's reconciliation, pushes its next attempt back (five minutes more
// per failure, at most two hours), and returns the new count.
func (m *Store) RecordHoldReconcileFailure(ctx context.Context, bookingID uuid.UUID) (int, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	n, err := m.Q.RecordBookingHoldReconcileFailure(ctx, data.UUIDToPg(bookingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, data.ErrRecordNotFound
	}
	return int(n), err
}

// ReopenableByLatePaymentSQL is the SQL test, over the bookings row aliased
// alias, for a booking a late online payment may reopen (S5): cancelled only
// by hold expiry (a booking_hold_expiries row), never paid, owing no refund
// and not yet started. The first four are exactly what the reversal trigger
// of 044_booking_hold_expiries.sql lets through; a reader looser than the
// trigger sent a payment the trigger then refused (a second payment on an
// already refunded booking) into a raw 23514 the webhook retried forever,
// instead of the refund a cancelled booking gets. The start is the one rule
// only the application keeps: a hand-over that arrives after the hours began
// is refunded as late rather than confirming a slot already in the past.
// GetByID and the payment confirmation's guard both use it, so the two
// cannot disagree.
func ReopenableByLatePaymentSQL(alias string) string {
	return fmt.Sprintf(`(%[1]s.status = 'cancelled'
		AND %[1]s.collection_status = 'unpaid'
		AND %[1]s.refund_status = 'none'
		AND %[1]s.refund_intent_at IS NULL
		AND booking_starts_at(%[1]s.date, %[1]s.start_time) > NOW()
		AND EXISTS (SELECT 1 FROM booking_hold_expiries e WHERE e.booking_id = %[1]s.id))`, alias)
}

// holdExpiryNote matches the note both expiry paths append: the release
// sweep's carries the window in parentheses, the inline release's does not.
var holdExpiryNote = regexp.MustCompile(`(?:\s*\|\s*)?Cancelado automáticamente: tiempo de pago expirado(?: \([^)]*\))?`)

// WithoutHoldExpiryNote returns notes with the hold-expiry note taken off, or
// nil when nothing else is left. A booking a late payment reopened is live
// again, and a note saying it was cancelled would only mislead the owner.
func WithoutHoldExpiryNote(notes *string) *string {
	if notes == nil {
		return nil
	}
	rest := strings.TrimSpace(holdExpiryNote.ReplaceAllString(*notes, ""))
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "|"))
	if rest == "" {
		return nil
	}
	return &rest
}
