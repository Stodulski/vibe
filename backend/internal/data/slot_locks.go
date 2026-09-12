package data

import (
	"context"
	"time"

	"github.com/google/uuid"
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

// SlotLockModel implements SlotLockStore against PostgreSQL.
type SlotLockModel struct {
	DB *DB
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
func (m *SlotLockModel) AcquireLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, endTime string, bookingID *uuid.UUID, ttl time.Duration) error {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	expiresAt := time.Now().Add(ttl)
	result, err := m.DB.Exec(ctx, `
		INSERT INTO slot_locks (court_id, date, start_time, end_time, booking_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (court_id, date, start_time) DO UPDATE
		SET end_time   = EXCLUDED.end_time,
		    booking_id = EXCLUDED.booking_id,
		    locked_at  = NOW(),
		    expires_at = EXCLUDED.expires_at
		WHERE slot_locks.expires_at <= NOW()`,
		courtID, date, startTime, endTime, bookingID, expiresAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrSlotLocked
	}
	return nil
}

// ReleaseLock removes a specific slot lock.
func (m *SlotLockModel) ReleaseLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime string) error {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`DELETE FROM slot_locks WHERE court_id = $1 AND date = $2 AND start_time = $3`,
		courtID, date, startTime)
	return err
}

// ReleaseByBooking removes all slot locks for a given booking.
func (m *SlotLockModel) ReleaseByBooking(ctx context.Context, bookingID uuid.UUID) error {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`DELETE FROM slot_locks WHERE booking_id = $1`, bookingID)
	return err
}

// CleanExpired removes all expired slot locks. Returns the count of removed locks.
func (m *SlotLockModel) CleanExpired(ctx context.Context) (int64, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	result, err := m.DB.Exec(ctx,
		`DELETE FROM slot_locks WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
