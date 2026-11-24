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
// ran out, and (047) which were released by their visitor after a failed
// payment (CancelReleasedCheckout), asked about again but never reopened. Two paths write it, both in the write that cancels the booking —
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
// Only while the booking is still pending and unpaid and its window (the
// store's PaymentExpiry) has ended: b was read before the sweep asked
// MercadoPago, and a client cancellation or a payment committed since must not
// be turned into an expiry (a reopenable booking, or a paid one cancelled),
// nor a hold ExtendHold kept live since (its payment is in process). Such a
// booking is left as it is and ErrEditConflict returned.
func (m *Store) ExpireHold(ctx context.Context, b *Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return err
	}

	return m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		dbBooking, err := q.ExpireUnpaidBooking(ctx, db.ExpireUnpaidBookingParams{
			ID:            data.UUIDToPg(b.ID),
			Notes:         data.TextToPg(b.Notes),
			WindowSeconds: m.PaymentExpiry.Seconds(),
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

// CancelReleasedCheckout cancels b — the caller has moved it to 'cancelled'
// with the release note — and records it as released by its visitor, in one
// transaction. It is UpdateIfUnchanged for a visitor's release after a failed
// payment (bookings.Service.ReleaseCheckout): the release asks MercadoPago
// once, and its payment search can lag an approval, so the record has the
// release sweep ask once more five minutes on. The record's cause is
// 'released' (047_booking_hold_release_cause.sql), which no reopening reader
// accepts: a late payment found then is refunded, as on any booking its
// client cancelled.
//
// Only while the row still carries the updated_at b was read with; otherwise
// nothing is written and ErrEditConflict returned.
func (m *Store) CancelReleasedCheckout(ctx context.Context, b *Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return err
	}
	return m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		dbBooking, err := q.UpdateBookingIfUnchanged(ctx, db.UpdateBookingIfUnchangedParams{
			ID:               data.UUIDToPg(b.ID),
			Status:           db.BookingStatus(b.Status),
			CollectionStatus: string(b.CollectionStatus),
			RefundStatus:     string(b.RefundStatus),
			Notes:            data.TextToPg(b.Notes),
			//nolint:gosec // G115: DepositAmount is bounded like Price (see UpdateIfUnchanged).
			DepositAmount:     int32(b.DepositAmount),
			RefundIntentAt:    data.TimePtrToPg(b.RefundIntentAt),
			ExpectedUpdatedAt: data.TimeToPg(b.UpdatedAt),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrEditConflict
			}
			return err
		}
		if err := q.InsertBookingCheckoutRelease(ctx, db.InsertBookingCheckoutReleaseParams{
			BookingID: data.UUIDToPg(b.ID),
			ComplexID: data.UUIDToPg(b.ComplexID),
		}); err != nil {
			return fmt.Errorf("bookings: record checkout release: %w", err)
		}
		b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)
		b.ClientSnapshot = snapshotFromDB(dbBooking)
		b.ContactEmail = dbBooking.ClientEmail.String
		return nil
	})
}

// ExpiredHoldsToReconcile returns up to limit bookings cancelled by hold
// expiry, or released by their visitor (ReleasedByClient), within the last lookback that the release sweep has not yet asked
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
		       cx.phone, cx.owner_id,
		       e.cause = 'released', e.failed_attempts
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
	result := make([]*CronBooking, 0, limit)
	for rows.Next() {
		var released bool
		var failures int32
		cb, err := scanCronBooking(rows, m.Keys, &released, &failures)
		if err != nil {
			return nil, err
		}
		cb.ReleasedByClient = released
		cb.HoldReconcileFailures = int(failures)
		result = append(result, cb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bookings: scan expired holds: %w", err)
	}
	return result, nil
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

// ExtendHold keeps a storefront booking's hold live for liveFor more while its
// payment is in process (S8). Every reader decides a hold has lapsed from
// created_at plus the payment window, so the hold is extended by moving
// created_at forward to now + liveFor - window, never backwards and never past
// now: with a window shorter than liveFor the hold is kept for one whole
// window from now, and the next sweep extends it again. Only a booking still
// pending and unpaid is touched; any other is left as it is and
// ErrEditConflict returned.
func (m *Store) ExtendHold(ctx context.Context, bookingID, complexID uuid.UUID, liveFor, window time.Duration) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, complexID); err != nil {
		return err
	}
	return m.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		tag, err := tx.Exec(ctx, `
			UPDATE bookings
			SET created_at = GREATEST(created_at, LEAST(NOW(),
			        NOW() + make_interval(secs => $2::float8) - make_interval(secs => $3::float8)))
			WHERE id = $1
			  AND status = 'pending'
			  AND collection_status = 'unpaid'
			  AND created_by IS NULL`,
			data.UUIDToPg(bookingID), liveFor.Seconds(), window.Seconds())
		if err != nil {
			return fmt.Errorf("extend hold: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return data.ErrEditConflict
		}
		return nil
	})
}

// PaymentsLapsingSoon returns up to limit storefront bookings past the hold
// stage, still pending and unpaid, with a checkout row carrying a MercadoPago
// preference, whose payment window ends within ahead from now or ended less
// than lookback ago, soonest first, enriched like GetExpiredPendingEnriched.
//
// It is what the in-process sweep asks MercadoPago about (S8): a payment in
// process on one of them extends its hold before the hours read as free.
// Nothing is written. Only bookings with a checkout are returned, so the
// MercadoPago calls stay bounded to the people who reached it.
func (m *Store) PaymentsLapsingSoon(ctx context.Context, window, ahead, lookback time.Duration, limit int) ([]*CronBooking, error) {
	return m.getUnpaidPublicPendingEnriched(ctx, fmt.Sprintf(`b.created_at + make_interval(secs => %[1]f) < NOW() + make_interval(secs => %[2]f)
		  AND b.created_at + make_interval(secs => %[1]f) > NOW() - make_interval(secs => %[3]f)
		  AND EXISTS (SELECT 1 FROM payments p
		              WHERE p.booking_id = b.id AND p.mp_preference_id IS NOT NULL AND p.mp_preference_id <> '')
		ORDER BY b.created_at
		LIMIT $1`, window.Seconds(), ahead.Seconds(), lookback.Seconds()), limit)
}

// ReopenableByLatePaymentSQL is the SQL test, over the bookings row aliased
// alias, for a booking a late online payment may reopen (S5): cancelled only
// by hold expiry (a booking_hold_expiries row whose cause is 'expired', not a
// visitor's release), never paid, owing no refund
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
		AND EXISTS (SELECT 1 FROM booking_hold_expiries e WHERE e.booking_id = %[1]s.id AND e.cause = 'expired'))`, alias)
}

// HoldExpiryNote is the note both expiry paths append to a booking they
// cancel: the release sweep (bookings.Service) and the inline release
// (ReleaseStalePendingOverlaps).
const HoldExpiryNote = "Cancelado automáticamente: tiempo de pago expirado"

// holdExpiryNote matches HoldExpiryNote, and the form the release sweep wrote
// before 2026-11-21, which carried the window as a raw Go duration in
// parentheses ("(10m0s)"): bookings cancelled then still carry it.
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
