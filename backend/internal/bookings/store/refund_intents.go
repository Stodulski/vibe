package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// GetRefundIntentOrphans, ClaimRefundIntent and ClearRefundIntent are the
// reconciliation sweep's store layer (refund-intent-durability spec),
// separate from the ordinary BookingUpdater.Update the rest of the codebase
// uses — the same separation refunds.go already gives paymentstore.Payments's refund
// lifecycle apart from payments.go.
//
// All three are hand-written SQL rather than sqlc: GetRefundIntentOrphans'
// predicate is the whole point of this file and has to be readable as exactly
// what it does not name (see below), and ClaimRefundIntent is a
// compare-and-swap sqlc's :one/:many/:exec generators have no shape for.

// GetRefundIntentOrphans returns cancelled bookings whose refund-intent
// marker has stood for longer than olderThan, oldest marker first.
//
// The predicate names refund_intent_at and nothing else. status,
// collection_status, refund_status and payments are absent from it, so no
// value of any of them can pull a row into this result set — a booking whose cancellation path
// decided no refund was owed (an out-of-window public cancellation) never had
// the marker set in the first place, so it is unrepresentable here rather
// than merely unlikely. See design.md, "the sweep selects on the marker and
// nothing else", and refund-intent-durability's "the sweep never refunds a
// deliberately declined refund".
func (m *Store) GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*Booking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT id, complex_id, court_id, client_id, date, start_time,
		       duration_minutes, price, deposit_amount, status,
		       collection_status, refund_status,
		       reminder_sent_2h, notes, created_by, created_at, updated_at,
		       refund_intent_at
		FROM bookings
		WHERE refund_intent_at IS NOT NULL
		  AND refund_intent_at < NOW() - $1::interval
		ORDER BY refund_intent_at ASC
		LIMIT $2`,
		olderThan.String(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query refund intent orphans: %w", err)
	}
	defer rows.Close()

	var result []*Booking
	for rows.Next() {
		var b db.Booking
		if err := rows.Scan(
			&b.ID, &b.ComplexID, &b.CourtID, &b.ClientID,
			&b.Date, &b.StartTime, &b.DurationMinutes,
			&b.Price, &b.DepositAmount, &b.Status,
			&b.CollectionStatus, &b.RefundStatus,
			&b.ReminderSent2h, &b.Notes, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
			&b.RefundIntentAt,
		); err != nil {
			return nil, fmt.Errorf("scan refund intent orphan: %w", err)
		}
		result = append(result, BookingFromDB(b))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ClaimRefundIntent reserves an orphaned refund intent for this sweep run
// before AutoRefundIfPaid is called on it.
//
// It is a conditional UPDATE on the exact refund_intent_at value
// GetRefundIntentOrphans read — MarkProcessing's exact idiom
// (failed_refunds.go), matching the predicate GetRefundIntentOrphans selected
// on so two sweep runs racing on the same row cannot both proceed. Setting
// the column to NOW() re-leases it for another grace period, exactly as
// MarkProcessing re-stamps updated_at.
//
// ErrRecordNotFound means another instance already took this row — the
// caller's move is to leave it alone and take the next one, same as
// MarkProcessing's callers do.
func (m *Store) ClaimRefundIntent(ctx context.Context, id uuid.UUID, seen time.Time) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := m.DB.Exec(ctx, `
		UPDATE bookings
		SET refund_intent_at = NOW()
		WHERE id = $1
		  AND refund_intent_at = $2`,
		data.UUIDToPg(id), data.TimeToPg(seen),
	)
	if err != nil {
		return fmt.Errorf("claim refund intent: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return data.ErrRecordNotFound
	}
	return nil
}

// ClearRefundIntent clears a booking's refund-intent marker.
//
// AutoRefundIfPaid calls it on every exit that commits no ClaimRefund; a
// committed ClaimRefund already cleared the marker inside its own
// transaction (refunds.go), so nothing on that path calls this again. The
// sweep itself clears nothing — it only claims and then calls
// AutoRefundIfPaid, so the marker's lifecycle is identical whether the call
// originated from a request or from the sweep.
func (m *Store) ClearRefundIntent(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`UPDATE bookings SET refund_intent_at = NULL WHERE id = $1`,
		data.UUIDToPg(id),
	)
	if err != nil {
		return fmt.Errorf("clear refund intent: %w", err)
	}
	return nil
}
