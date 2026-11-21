package store

import (
	"context"
	"errors"
	"fmt"
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

// ExpireHold writes b — which the caller has moved to 'cancelled' with the
// expiry note — and records it as cancelled by hold expiry, in one
// transaction. It is Update for the release sweep: a booking cancelled
// without the record would never be asked about again, nor reopened by its
// late payment.
func (m *Store) ExpireHold(ctx context.Context, b *Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return err
	}

	return m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		dbBooking, err := q.UpdateBooking(ctx, db.UpdateBookingParams{
			ID:               data.UUIDToPg(b.ID),
			Status:           db.BookingStatus(b.Status),
			CollectionStatus: string(b.CollectionStatus),
			RefundStatus:     string(b.RefundStatus),
			Notes:            data.TextToPg(b.Notes),
			//nolint:gosec // G115: DepositAmount bounded to Price; far below int32 range.
			DepositAmount:  int32(b.DepositAmount),
			RefundIntentAt: data.TimePtrToPg(b.RefundIntentAt),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return err
		}
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
// MercadoPago about and has failed on fewer than maxFailures times, oldest
// first, enriched like GetExpiredPendingEnriched (the seller credential is
// what MercadoPago is asked with). Only bookings with a checkout link — a
// payment row carrying a preference — are returned: nothing else could have
// been paid online.
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
		  AND b.status = 'cancelled'
		  AND EXISTS (
		        SELECT 1 FROM payments p
		        WHERE p.booking_id = b.id AND p.mp_preference_id IS NOT NULL AND p.mp_preference_id <> '')
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
// booking's reconciliation, and returns the new count.
func (m *Store) RecordHoldReconcileFailure(ctx context.Context, bookingID uuid.UUID) (int, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	n, err := m.Q.RecordBookingHoldReconcileFailure(ctx, data.UUIDToPg(bookingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, data.ErrRecordNotFound
	}
	return int(n), err
}
