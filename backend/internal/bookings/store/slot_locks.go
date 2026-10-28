package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
)

// SlotLock represents a short-lived reservation on a court slot held during checkout.
type SlotLock struct {
	ID        uuid.UUID  `json:"id"`
	CourtID   uuid.UUID  `json:"court_id"`
	Date      time.Time  `json:"date"`
	StartTime string     `json:"start_time"`
	EndTime   string     `json:"end_time"`
	BookingID *uuid.UUID `json:"booking_id,omitempty"`
	LockedBy  string     `json:"locked_by"`
	LockedAt  time.Time  `json:"locked_at"`
	ExpiresAt time.Time  `json:"expires_at"`
}

// SlotLocks implements SlotLockStore against PostgreSQL.
type SlotLocks struct {
	DB *data.DB
}

// AcquireLock attempts to lock a slot atomically. Returns ErrSlotLocked if the
// slot is held by a lock that has not expired.
//
// The expiry is enforced here, on the way in, rather than only by the sweeper.
// It used to be ON CONFLICT DO NOTHING, which made expires_at decorative: a lock
// whose TTL had passed an hour ago still refused every new acquisition, so the
// slot stayed unsellable from the public booking page until the five-minute
// cron happened to delete it. With a fifteen-minute TTL that turned a checkout
// somebody abandoned into up to five extra minutes of a court nobody could buy,
// and if the sweeper was not running it never ended.
//
// The takeover is one statement so it stays atomic: the WHERE clause on the
// DO UPDATE makes PostgreSQL refuse the update — reporting zero rows affected —
// when the existing lock is still live, and two callers racing for an expired
// lock serialise on the same row, so exactly one of them wins it.
//
// holder is the token that names this checkout. It is stored in locked_by, and
// ReleaseHeldLock frees the lock only for the holder that is recorded there. The
// takeover writes the new holder too: a lock taken over must never still name
// the checkout whose TTL ran out, or that checkout could free the new one.
func (m *SlotLocks) AcquireLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, endTime string, bookingID *uuid.UUID, holder string, ttl time.Duration) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	expiresAt := time.Now().Add(ttl)
	result, err := m.DB.Exec(ctx, `
		INSERT INTO slot_locks (court_id, date, start_time, end_time, booking_id, locked_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (court_id, date, start_time) DO UPDATE
		SET end_time   = EXCLUDED.end_time,
		    booking_id = EXCLUDED.booking_id,
		    locked_by  = EXCLUDED.locked_by,
		    locked_at  = NOW(),
		    expires_at = EXCLUDED.expires_at
		WHERE slot_locks.expires_at <= NOW()`,
		courtID, date, startTime, endTime, bookingID, holder, expiresAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrSlotLocked
	}
	return nil
}

// ReleaseLock removes a specific slot lock, whoever holds it. The cancel path
// relies on that: cancelling a booking deliberately frees the slot.
func (m *SlotLocks) ReleaseLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`DELETE FROM slot_locks WHERE court_id = $1 AND date = $2 AND start_time = $3`,
		courtID, date, startTime)
	return err
}

// ReleaseHeldLock removes the slot lock only if it is still held by holder.
//
// A checkout whose TTL ran out while MercadoPago was slow can find the slot
// already taken over by another checkout. An unconditional delete would free
// that live lock too, and a second buyer could then book a slot somebody has
// already started paying for. The holder check makes the statement a no-op in
// that case: zero rows deleted, the other checkout's lock left in place.
func (m *SlotLocks) ReleaseHeldLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, holder string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`DELETE FROM slot_locks WHERE court_id = $1 AND date = $2 AND start_time = $3 AND locked_by = $4`,
		courtID, date, startTime, holder)
	return err
}

// ReleaseByBooking removes all slot locks for a given booking.
func (m *SlotLocks) ReleaseByBooking(ctx context.Context, bookingID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`DELETE FROM slot_locks WHERE booking_id = $1`, bookingID)
	return err
}

// CleanExpired removes all expired slot locks. Returns the count of removed locks.
func (m *SlotLocks) CleanExpired(ctx context.Context) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	result, err := m.DB.Exec(ctx,
		`DELETE FROM slot_locks WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
