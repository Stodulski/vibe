package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestSlotLock_StructFields verifies the SlotLock struct can be instantiated
// with all expected fields. The model methods (AcquireLock, ReleaseLock,
// ReleaseByBooking, CleanExpired) all require a database connection.
func TestSlotLock_StructFields(t *testing.T) {
	bookingID := uuid.New()
	now := time.Now()
	lock := SlotLock{
		ID:        uuid.New(),
		CourtID:   uuid.New(),
		Date:      now,
		StartTime: "09:00",
		EndTime:   "10:30",
		BookingID: &bookingID,
		LockedBy:  "user-123",
		LockedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	if lock.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if lock.CourtID == uuid.Nil {
		t.Error("CourtID should not be nil")
	}
	if lock.StartTime != "09:00" {
		t.Errorf("StartTime = %q, want %q", lock.StartTime, "09:00")
	}
	if lock.EndTime != "10:30" {
		t.Errorf("EndTime = %q, want %q", lock.EndTime, "10:30")
	}
	if lock.BookingID == nil {
		t.Error("BookingID should not be nil")
	}
	if lock.LockedBy != "user-123" {
		t.Errorf("LockedBy = %q, want %q", lock.LockedBy, "user-123")
	}
	if !lock.ExpiresAt.After(lock.LockedAt) {
		t.Error("ExpiresAt should be after LockedAt")
	}
}

// TestSlotLock_NilBookingID verifies that BookingID can be nil
// (locks without a specific booking).
func TestSlotLock_NilBookingID(t *testing.T) {
	lock := SlotLock{
		ID:        uuid.New(),
		CourtID:   uuid.New(),
		StartTime: "09:00",
		EndTime:   "10:30",
		BookingID: nil,
	}

	if lock.BookingID != nil {
		t.Error("BookingID should be nil")
	}
}

// TestSlotLockModel_RequiresDB documents that all SlotLocks methods
// require a database connection. There is no pure logic to test.
func TestSlotLockModel_RequiresDB(t *testing.T) {
	t.Skip("SlotLocks methods (AcquireLock, ReleaseLock, ReleaseByBooking, CleanExpired) all require *pgxpool.Pool")
}
