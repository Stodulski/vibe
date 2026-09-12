//go:build integration

package data_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// H-22. These cover the other pairing the court-day advisory lock does not
// reach: deleting a court against a booking being created on it.
//
// H-02 folded the handler's separate HasActiveBookingsByCourt pre-check into
// SoftDelete's own UPDATE, so the check and the act became one statement. That
// closes the case it was written for — a booking that has ALREADY COMMITTED is
// seen by the UPDATE's own predicate — and it closes nothing about two writers
// still in flight. BookingModel.InsertSafe takes an advisory lock on
// (court, day); SoftDelete took no lock of its own at all. The two writers
// never contended on one object, so under READ COMMITTED each one's check was
// correct against its own snapshot and the pair was still wrong: both
// committed, and a client held hours on a court the owner had just watched
// disappear.
//
// Both directions are replayed rather than raced for, and both have to be
// covered, because they fail for different reasons:
//
//   - the delete arriving first is a stale-read problem, fixed by having the
//     booking read the court under FOR SHARE so its read is a locking one;
//   - the booking arriving first is an EvalPlanQual problem, and it is the
//     subtle one. When an UPDATE blocks on a row another transaction has
//     locked, PostgreSQL re-evaluates the qual against the updated row once
//     that transaction commits — but the subquery in that qual still runs
//     against the command's ORIGINAL snapshot. A NOT EXISTS over `bookings`
//     therefore does NOT see the booking that just committed. Locking the
//     court row is not enough on its own; the booking question has to be asked
//     by a SEPARATE statement taken after the lock is held, because READ
//     COMMITTED gives each statement a fresh snapshot and EvalPlanQual does
//     not.

// holdCourtRow takes a FOR SHARE lock on the fixture's court row and holds it
// until the returned release runs, standing in for a booking transaction that
// has read the court and not yet committed.
func holdCourtRow(t *testing.T, f *testFixture) (release func()) {
	t.Helper()

	ctx := context.Background()
	conn, err := f.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring the gate connection: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		conn.Release()
		t.Fatalf("beginning the gate transaction: %v", err)
	}

	var id any
	if err := tx.QueryRow(ctx,
		`SELECT id FROM courts WHERE id = $1 FOR SHARE`, f.CourtID).Scan(&id); err != nil {
		_ = tx.Rollback(ctx)
		conn.Release()
		t.Fatalf("taking the court row lock: %v", err)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = tx.Rollback(ctx)
			conn.Release()
		})
	}
}

func countBookingsOnCourt(t *testing.T, f *testFixture) int {
	t.Helper()

	var n int
	if err := f.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM bookings WHERE court_id = $1`, f.CourtID).Scan(&n); err != nil {
		t.Fatalf("counting bookings: %v", err)
	}
	return n
}

func courtIsDeleted(t *testing.T, f *testFixture) bool {
	t.Helper()

	var deleted bool
	if err := f.Pool.QueryRow(context.Background(),
		`SELECT deleted_at IS NOT NULL FROM courts WHERE id = $1`, f.CourtID).Scan(&deleted); err != nil {
		t.Fatalf("reading the court: %v", err)
	}
	return deleted
}

func pendingBooking(f *testFixture, date time.Time) *data.Booking {
	return &data.Booking{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Date: date, StartTime: "10:00", DurationMinutes: 60,
		Price: 500_000, DepositAmount: 150_000,
		Status: "pending", CollectionStatus: data.CollectionStatusUnpaid,
		RefundStatus: data.RefundStatusNone,
	}
}

// waitForRowLockWaiters blocks until want transactions are queued on a
// `courts` row lock, so the test knows SoftDelete has really reached the row
// rather than merely been started.
//
// A delete that never appears in that queue is itself the failure — it takes
// no lock on the court, so nothing serializes it against a booking in flight —
// and the timeout is reported so the caller carries on to its own assertion
// rather than stopping on the diagnosis.
func waitForRowLockWaiters(t *testing.T, f *testFixture, want int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting int
		err := f.Pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM pg_locks
			WHERE NOT granted
			  AND locktype IN ('tuple', 'transactionid')
			  AND pid <> pg_backend_pid()`,
		).Scan(&waiting)
		if err != nil {
			t.Fatalf("reading row lock waiters: %v", err)
		}
		if waiting >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("the delete must queue on the court row; want %d waiting, got %d — a delete that never locks the court is serialized against nothing", want, waiting)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestBookingOnACourtDeletedMidTransactionIsRefused is the direction where the
// delete gets there first.
//
// The gate holds the court-day advisory lock, so InsertSafe queues on it
// before it has looked at anything. The court is deleted from outside that
// lock — standing for the owner's DELETE, which takes no advisory lock and so
// is not serialized against the booking by anything — and only then is the
// gate released.
//
// A booking that never reads the court under a lock resumes, sees the snapshot
// it took before the delete committed, and inserts onto a court that is gone.
func TestBookingOnACourtDeletedMidTransactionIsRefused(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	date := bookedDay(20)

	release := blockCourtDay(t, f, date)

	result := make(chan error, 1)
	go func() { result <- f.Models.Bookings.InsertSafe(ctx, pendingBooking(f, date)) }()

	// The booking must be inside its transaction, queued on the court-day
	// lock, before the court is deleted. Without the lock it is already done.
	waitForLockWaiters(t, f, 1)

	if err := f.Models.Courts.SoftDelete(ctx, f.CourtID); err != nil {
		t.Fatalf("deleting the court while the booking is mid-transaction: %v", err)
	}
	release()

	if err := <-result; !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("a booking on a court deleted mid-transaction: got err = %v, want ErrRecordNotFound. "+
			"The client now holds hours on a court the owner has already removed.", err)
	}
	if n := countBookingsOnCourt(t, f); n != 0 {
		t.Errorf("the refused booking must leave no row behind; found %d", n)
	}
}

// TestDeletingACourtWhileABookingCommitsIsRefused is the other direction, and
// the one a row lock alone does not fix.
//
// The gate holds the court row under FOR SHARE, standing in for a booking
// transaction that has read the court and not yet committed. SoftDelete queues
// behind it. The booking is then committed from outside — the write that
// transaction was about to make — and only then is the gate released.
//
// A SoftDelete whose booking question rides along in the UPDATE's own WHERE
// clause resumes under EvalPlanQual, re-checks that qual against its ORIGINAL
// snapshot, does not see the booking that just committed, and deletes the
// court anyway.
func TestDeletingACourtWhileABookingCommitsIsRefused(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	date := bookedDay(21)

	release := holdCourtRow(t, f)

	result := make(chan error, 1)
	go func() { result <- f.Models.Courts.SoftDelete(ctx, f.CourtID) }()

	// The delete must be queued on the court row before the booking commits.
	// A delete that never locks that row is already finished here.
	waitForRowLockWaiters(t, f, 1)

	insertLiveBooking(t, f, date, "10:00", 60)
	release()

	if err := <-result; !errors.Is(err, courtstore.ErrCourtHasActiveBookings) {
		t.Fatalf("deleting a court while a booking for it was committing: got err = %v, "+
			"want ErrCourtHasActiveBookings. The booking survives on a deleted court.", err)
	}
	if courtIsDeleted(t, f) {
		t.Error("the refused delete must leave the court live")
	}
}
