package pricing

import (
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/timezone"
)

func TestCanRefund(t *testing.T) {
	// Use a fixed "now" by testing with dates far in the future (always refundable)
	// and dates in the past (never refundable).
	argTZ, _ := time.LoadLocation("America/Argentina/Buenos_Aires")

	t.Run("zero cancellation hours always allows refund", func(t *testing.T) {
		booking := &data.Booking{
			Date:      time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), // past
			StartTime: "10:00",
		}
		if !CanRefund(booking, 0, 15*time.Minute) {
			t.Error("want true when cancellationHours is 0")
		}
	})

	t.Run("negative cancellation hours always allows refund", func(t *testing.T) {
		booking := &data.Booking{
			Date:      time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			StartTime: "10:00",
		}
		if !CanRefund(booking, -1, 15*time.Minute) {
			t.Error("want true when cancellationHours is negative")
		}
	})

	t.Run("booking far in future is refundable", func(t *testing.T) {
		booking := &data.Booking{
			Date:      time.Date(2099, 6, 15, 0, 0, 0, 0, time.UTC),
			StartTime: "18:00",
		}
		if !CanRefund(booking, 24, 15*time.Minute) {
			t.Error("want true for booking far in the future")
		}
	})

	t.Run("booking in the past is not refundable", func(t *testing.T) {
		booking := &data.Booking{
			Date:      time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			StartTime: "10:00",
		}
		if CanRefund(booking, 24, 15*time.Minute) {
			t.Error("want false for booking in the past")
		}
	})

	t.Run("booking starting now with 1h window is not refundable", func(t *testing.T) {
		now := time.Now().In(argTZ)
		booking := &data.Booking{
			Date:      now,
			StartTime: now.Format("15:04"),
		}
		if CanRefund(booking, 1, 15*time.Minute) {
			t.Error("want false when booking starts now and cancellation window is 1h")
		}
	})

	t.Run("booking in 48h with 24h window is refundable", func(t *testing.T) {
		future := time.Now().In(argTZ).Add(48 * time.Hour)
		booking := &data.Booking{
			Date:      future,
			StartTime: future.Format("15:04"),
		}
		if !CanRefund(booking, 24, 15*time.Minute) {
			t.Error("want true when booking is in 48h with 24h cancellation window")
		}
	})
}

// TestRefundDeadline pins the formula CanRefund now delegates to, with fixed
// dates rather than a real clock — RefundDeadline takes no "now" at all, so
// there is nothing for a real clock to add but flakiness.
func TestRefundDeadline(t *testing.T) {
	argTZ, _ := time.LoadLocation("America/Argentina/Buenos_Aires")

	t.Run("zero cancellation hours: deadline is the booking start itself", func(t *testing.T) {
		booking := &data.Booking{
			Date:      time.Date(2026, 9, 6, 0, 0, 0, 0, argTZ),
			StartTime: "19:00",
		}
		want := time.Date(2026, 9, 6, 19, 0, 0, 0, argTZ)
		got := RefundDeadline(booking, 0, 15*time.Minute)
		if !got.Equal(want) {
			t.Errorf("want deadline %v, got %v", want, got)
		}
	})

	t.Run("window deadline later than grace end: deadline is the window", func(t *testing.T) {
		booking := &data.Booking{
			Date:      time.Date(2026, 9, 10, 0, 0, 0, 0, argTZ),
			StartTime: "19:00",
			CreatedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, argTZ),
		}
		// Booking start minus 24h; the grace period (ending 2026-09-01 10:15)
		// is long over by then.
		want := time.Date(2026, 9, 9, 19, 0, 0, 0, argTZ)
		got := RefundDeadline(booking, 24, 15*time.Minute)
		if !got.Equal(want) {
			t.Errorf("want deadline %v, got %v", want, got)
		}
	})

	t.Run("window already closed but grace still open: deadline is grace end", func(t *testing.T) {
		booking := &data.Booking{
			Date:      time.Date(2026, 9, 6, 0, 0, 0, 0, argTZ),
			StartTime: "19:00",
			CreatedAt: time.Date(2026, 9, 6, 18, 50, 0, 0, argTZ), // created a minute-scale moment before start
		}
		// windowDeadline = start - 24h = 2026-09-05 19:00, already past createdAt.
		// graceEnd = createdAt + 15m = 2026-09-06 19:05, which wins.
		want := time.Date(2026, 9, 6, 19, 5, 0, 0, argTZ)
		got := RefundDeadline(booking, 24, 15*time.Minute)
		if !got.Equal(want) {
			t.Errorf("want deadline %v, got %v", want, got)
		}
	})
}

// WithinStandardWindow had no test at all, and it is not decoration: it is the
// flag the booking-confirmation email uses to choose which cancellation
// sentence the client is told (internal/payments/process.go and
// internal/bookings/create.go both pass it as InStandardWindow). A wrong answer
// here is a promise about a refund, made in writing, at the moment of booking.
//
// Every case is expressed relative to now, because the function reads the clock
// itself. The margins are minutes wide so the boundary cases cannot flake on a
// slow machine, and the times are built in Argentina — the zone the function
// resolves the booking's wall-clock start in.
func TestWithinStandardWindow(t *testing.T) {
	now := time.Now().In(timezone.Argentina)

	tests := []struct {
		name              string
		start             time.Time
		cancellationHours int
		want              bool
	}{
		{
			name:              "a complex with no window at all always allows it",
			start:             now.Add(-30 * 24 * time.Hour),
			cancellationHours: 0,
			want:              true,
		},
		{
			name:              "a negative window is read the same way as none",
			start:             now.Add(-30 * 24 * time.Hour),
			cancellationHours: -1,
			want:              true,
		},
		{
			name:              "two days out against a one-day window",
			start:             now.Add(48 * time.Hour),
			cancellationHours: 24,
			want:              true,
		},
		{
			name:              "two hours out against a one-day window is past the deadline",
			start:             now.Add(2 * time.Hour),
			cancellationHours: 24,
			want:              false,
		},
		{
			name:              "a booking that has already started",
			start:             now.Add(-1 * time.Hour),
			cancellationHours: 24,
			want:              false,
		},
		{
			name:              "two minutes on the free side of the deadline",
			start:             now.Add(24*time.Hour + 2*time.Minute),
			cancellationHours: 24,
			want:              true,
		},
		{
			name:              "two minutes on the far side of the same deadline",
			start:             now.Add(24*time.Hour - 2*time.Minute),
			cancellationHours: 24,
			want:              false,
		},
		{
			name:              "an hour-long window, half an hour out",
			start:             now.Add(30 * time.Minute),
			cancellationHours: 1,
			want:              false,
		},
		{
			name:              "an hour-long window, two hours out",
			start:             now.Add(2 * time.Hour),
			cancellationHours: 1,
			want:              true,
		},
		{
			name:              "a week-long window is honoured as a week",
			start:             now.Add(8 * 24 * time.Hour),
			cancellationHours: 168,
			want:              true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := WithinStandardWindow(bookingStartingAt(tc.start), tc.cancellationHours); got != tc.want {
				t.Errorf("WithinStandardWindow(start=%s, hours=%d) = %v, want %v",
					tc.start.Format(time.RFC3339), tc.cancellationHours, got, tc.want)
			}
		})
	}
}

// The one property that separates this function from CanRefund, and the reason
// both exist. A client who books a slot starting in two hours is inside the
// grace period and will get their money back, but they are NOT inside the
// complex's standard window — so the confirmation email must not tell them they
// have until 24 hours before the game, which is a deadline that has already
// passed.
func TestWithinStandardWindowIgnoresTheGracePeriodCanRefundHonours(t *testing.T) {
	now := time.Now().In(timezone.Argentina)
	booking := bookingStartingAt(now.Add(2 * time.Hour))
	booking.CreatedAt = now // booked this instant: squarely inside any grace period

	if WithinStandardWindow(booking, 24) {
		t.Error("the standard window is about the game's start, not about when the booking was made")
	}
	if !CanRefund(booking, 24, 15*time.Minute) {
		t.Error("the grace period is what makes this refundable, and CanRefund is the one that reads it")
	}
}

// bookingStartingAt builds a booking whose wall-clock start is instant, in the
// zone WithinStandardWindow resolves it in. StartTime carries only HH:MM, which
// is why every margin above is minutes rather than seconds.
func bookingStartingAt(instant time.Time) *data.Booking {
	inArg := instant.In(timezone.Argentina)
	return &data.Booking{
		Date:      inArg,
		StartTime: inArg.Format("15:04"),
		CreatedAt: inArg.Add(-30 * 24 * time.Hour),
	}
}
