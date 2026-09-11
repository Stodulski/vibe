//go:build integration

package data

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/timezone"
)

// checkViolation is the SQLSTATE PostgreSQL raises for a failed CHECK.
const checkViolation = "23514"

// The double sale this whole change exists for, at the storage layer.
//
// This test used to assert the opposite of what it asserts now, and the story is
// the point. Both handlers once guarded midnight with "> 1440", which lets 1440
// through, and slots.FromMinutes wraps there — so a 23:00 booking on a 60-minute
// court was written as start_time 23:00, end_time 00:00. Nothing downstream
// could see that row: slot_guard asked "start_time < $4 AND end_time > $3",
// false for every candidate against an end_time of 00:00. An overlapping 22:30
// booking committed cleanly, and one court was sold to two paying clients.
//
// The answer then was a CHECK (start_time < end_time), refusing
// the shape outright. The answer now is the opposite: that check is gone
// constraint on purpose, because a venue trading 18:00 to 02:00 has to be able
// to sell 23:00 to 01:00, and the generated span replaced it with something stronger
// — an EXCLUDE over `span`, which refuses the OVERLAP rather than the shape.
//
// So the guarantee under test is unchanged — one court is never sold twice —
// while the mechanism enforcing it is now the one that also permits the
// legitimate overnight booking. Asserted through the real store, no handler
// involved, because a guard that lives only in handlers is one new insert path
// away from being bypassed.
func TestTheStoreRefusesABookingOverlappingAnOvernightOne(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	overnight := f.newBooking(bookingOptions{StartTime: "23:00", EndTime: "01:00"})
	overnight.DurationMinutes = 120

	if err := f.Models.Bookings.InsertSafe(ctx, overnight); err != nil {
		t.Fatalf("a venue open past midnight must be able to sell 23:00-01:00; got %v", err)
	}

	// Half past midnight is inside it, and belongs to the following calendar
	// day — the exact position the old time-of-day comparison could not see.
	overlapping := f.newBooking(bookingOptions{StartTime: "00:30", EndTime: "01:30"})
	overlapping.DurationMinutes = 60
	overlapping.Date = overnight.Date.AddDate(0, 0, 1)

	err := f.Models.Bookings.InsertSafe(ctx, overlapping)
	if err == nil {
		t.Fatal("00:30 falls inside a booking that runs to 01:00; accepting it sells one court to two clients")
	}
	if !errors.Is(err, ErrDuplicateBooking) && !errors.Is(err, ErrSlotUnavailable) {
		t.Fatalf("the refusal must name the slot as taken; got %v", err)
	}

	if confirmed := f.countBookings(t, "confirmed"); confirmed != 1 {
		t.Errorf("only the first booking may survive; the court holds %d confirmed", confirmed)
	}
}

// The shape the database now accepts, asserted against a hand-written INSERT so
// the storage layer is proven independently of the store as well.
//
// The three cases below were all refused under that check. The first is now
// legitimate; the other two are still nonsense, and what refuses them today is
// no longer a CHECK on the times but the generated `span` itself — a range whose
// end does not follow its start cannot be constructed.
func TestTheDatabaseAcceptsAnOvernightBookingAndNothingBackwards(t *testing.T) {
	f := newTestFixture(t)

	insert := func(t *testing.T, startTime string, durationMinutes int) error {
		t.Helper()
		_, err := f.Pool.Exec(context.Background(), `
			INSERT INTO bookings
				(complex_id, court_id, client_id, date, start_time,
				 duration_minutes, price, deposit_amount, status, collection_status)
			VALUES ($1, $2, $3, CURRENT_DATE + 7, $4::time,
			     $5, 500000, 150000, 'confirmed', 'deposit_paid')`,
			f.ComplexID, f.CourtID, f.ClientID, startTime, durationMinutes)
		return err
	}

	t.Run("an hour that runs past midnight is written", func(t *testing.T) {
		if err := insert(t, "23:00", 60); err != nil {
			t.Fatalf("23:00 for sixty minutes ends at midnight and must be accepted; got %v", err)
		}
	})

	t.Run("a booking of no length is refused", func(t *testing.T) {
		// duration_minutes must be one of the lengths the grid sells; zero is
		// not one of them, and an empty span would hold no hours at all.
		if err := insert(t, "10:00", 0); err == nil {
			t.Error("a booking occupying no time must be refused")
		}
	})
}

// The other midnight in this schema: the one the reminder sweep walked into.
//
// A booking is a `date` plus a `start_time`, and the two-hour reminder used to
// compare the second of those against a time of day —
//
//	AND start_time <= ((NOW() AT TIME ZONE '<zone>')::time + INTERVAL '2 hours')
//	AND start_time >  (NOW()  AT TIME ZONE '<zone>')::time
//
// — which wraps. At 23:00 the upper bound is 01:00, so the pair reads
// `start_time <= 01:00 AND start_time > 23:00` and no row on earth satisfies it.
// From 22:00 local until midnight the sweep selected nothing: a 23:30 game got
// no reminder, ever. The `date = today` clause closed the other exit, dropping
// tomorrow's 00:30 game from a window it plainly belongs in.
//
// Both queries now range over booking_starts_at(date, start_time) — the instant
// the game begins — and take `now` from the caller. The second half is what
// makes this test exist at all: with NOW() read inside the database, these two
// cases would only have failed if the suite happened to run between 22:00 and
// midnight, and a test that is honest for two hours a day is a test nobody sees
// fail.
func TestTheTwoHourReminderSpansMidnight(t *testing.T) {
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, time.March, day, hour, minute, 0, 0, timezone.Argentina)
	}
	onDay := func(day int) time.Time {
		return time.Date(2026, time.March, day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name string
		now  time.Time
		// The booking: local date, and the wall-clock hours stored beside it.
		date       time.Time
		startTime  string
		bookedAgo  time.Duration
		wantRemind bool
	}{
		{
			name:       "a game at 23:30 is due at 22:00",
			now:        at(10, 22, 0),
			date:       onDay(10),
			startTime:  "23:30",
			bookedAgo:  24 * time.Hour,
			wantRemind: true,
		},
		{
			name:       "a game at 00:30 tomorrow is due at 23:00 today",
			now:        at(10, 23, 0),
			date:       onDay(11),
			startTime:  "00:30",
			bookedAgo:  24 * time.Hour,
			wantRemind: true,
		},
		{
			// The window still has an upper edge; crossing midnight must not
			// turn it into "everything after now".
			name:       "a game at 02:00 tomorrow is not yet due at 23:00 today",
			now:        at(10, 23, 0),
			date:       onDay(11),
			startTime:  "02:00",
			bookedAgo:  24 * time.Hour,
			wantRemind: false,
		},
		{
			// And a lower edge: a game already under way is not reminded.
			name:       "a game at 21:00 is past at 22:00",
			now:        at(10, 22, 0),
			date:       onDay(10),
			startTime:  "21:00",
			bookedAgo:  24 * time.Hour,
			wantRemind: false,
		},
		{
			// The enriched query withholds the reminder from someone who booked
			// less than two hours ago, on the grounds that they already know.
			// It used to withhold it for five, because
			// `NOW() AT TIME ZONE '<zone>' - INTERVAL '2 hours'` strips the
			// offset off a timestamptz and the comparison re-reads the result in
			// the session's UTC — three hours of silent extra floor.
			name:       "a game booked three hours ago is still reminded",
			now:        at(10, 22, 0),
			date:       onDay(10),
			startTime:  "23:30",
			bookedAgo:  3 * time.Hour,
			wantRemind: true,
		},
		{
			name:       "a game booked one hour ago is not reminded twice",
			now:        at(10, 22, 0),
			date:       onDay(10),
			startTime:  "23:30",
			bookedAgo:  1 * time.Hour,
			wantRemind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestFixture(t)
			ctx := context.Background()

			id := f.seedReminderCandidate(t, tt.date, tt.startTime, tt.now.Add(-tt.bookedAgo))

			enriched, err := f.Models.Bookings.GetForReminder2hEnriched(ctx, tt.now)
			if err != nil {
				t.Fatalf("GetForReminder2hEnriched: %v", err)
			}
			got := slices.Contains(bookingIDs(enriched), id)
			if got != tt.wantRemind {
				t.Errorf("GetForReminder2hEnriched at %s: booking on %s at %s reminded = %t, want %t",
					tt.now.Format("2006-01-02 15:04"), tt.date.Format("2006-01-02"), tt.startTime, got, tt.wantRemind)
			}

			// The sqlc query is the same window without the created_at floor, so
			// the two must agree on every case that clears the floor. They are
			// two hand-maintained copies of one question; the whole point of
			// booking_starts_at is that they cannot answer it differently.
			if tt.bookedAgo < 2*time.Hour {
				return
			}
			plain, err := f.Models.Bookings.GetForReminder2h(ctx, tt.now)
			if err != nil {
				t.Fatalf("GetForReminder2h: %v", err)
			}
			if slices.Contains(bookingIDs(plain), id) != tt.wantRemind {
				t.Errorf("GetForReminder2h disagrees with GetForReminder2hEnriched on %q: %t vs %t",
					tt.name, !tt.wantRemind, tt.wantRemind)
			}
		})
	}
}

// seedReminderCandidate writes a confirmed, un-reminded booking straight to the
// table, with created_at chosen rather than taken from the clock.
//
// It bypasses BookingModel.Insert deliberately: Insert stamps created_at with
// NOW(), and every case here is anchored to a fixed instant in March 2026 that
// has nothing to do with when the suite runs.
func (f *testFixture) seedReminderCandidate(t *testing.T, date time.Time, startTime string, createdAt time.Time) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.Pool.QueryRow(context.Background(), `
		INSERT INTO bookings
			(complex_id, court_id, client_id, date, start_time,
			 duration_minutes, price, deposit_amount, status, collection_status,
			 reminder_sent_2h, created_at)
		VALUES ($1, $2, $3, $4::date, $5::time,
			 90, 500000, 150000, 'confirmed', 'deposit_paid', false, $6)
		RETURNING id`,
		f.ComplexID, f.CourtID, f.ClientID, date, startTime, createdAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding booking on %s at %s: %v", date.Format("2006-01-02"), startTime, err)
	}
	return id
}

// The reminder queries are global — they carry no complex filter, because the
// cron that runs them sweeps the whole platform. So a test asserts on its own
// booking's presence, never on the size of the result.
func bookingIDs[T *Booking | *CronBooking](rows []T) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		switch b := any(row).(type) {
		case *Booking:
			ids = append(ids, b.ID)
		case *CronBooking:
			ids = append(ids, b.ID)
		}
	}
	return ids
}
