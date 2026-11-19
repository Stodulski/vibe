package pricing

import (
	"strconv"
	"testing"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// TestLinkLiveNeverRejectsARefundEligibleCancellation is task 9.1/9.3: the
// single most important test in this change. It is a matrix over every
// combination of cancellationHours, grace period and expiresAt/booking-date
// direction this file's own vocabulary can produce, and it asserts one
// property: whenever CanRefund is true, LinkLive must also be true,
// regardless of expiresAt. Token expiry must never be able to block a
// refund-eligible cancellation.
//
// Mutation, run and recorded (task 9.3): replace the disjunct with
// `now.Before(expiresAt)` alone (drop `|| CanRefund(...)`) in LinkLive — every
// cancellationHours == 0 row and every past-expiresAt grace-period row must
// then fail, because a cancellationHours == 0 complex has no deadline at all
// and CanRefund is unconditionally true for it.
func TestLinkLiveNeverRejectsARefundEligibleCancellation(t *testing.T) {
	argTZ, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	now := time.Now().In(argTZ)

	cancellationHoursValues := []int{0, 1, 24, 168}
	graceValues := []time.Duration{0, 15 * time.Minute, 72 * time.Hour}
	expiresAtValues := map[string]time.Time{
		"past":   now.Add(-48 * time.Hour),
		"future": now.Add(48 * time.Hour),
	}
	bookingDateValues := map[string]time.Time{
		"past":   now.Add(-72 * time.Hour),
		"future": now.Add(72 * time.Hour),
	}

	for _, cancellationHours := range cancellationHoursValues {
		for _, grace := range graceValues {
			for expiresAtName, expiresAt := range expiresAtValues {
				for bookingDateName, bookingDate := range bookingDateValues {
					name := fmtCase(cancellationHours, grace, expiresAtName, bookingDateName)
					t.Run(name, func(t *testing.T) {
						booking := &bookingstore.Booking{
							Date:      bookingDate,
							StartTime: bookingDate.Format("15:04"),
							CreatedAt: now.Add(-1 * time.Hour),
						}

						canRefund := CanRefund(booking, cancellationHours, grace)
						linkLive := LinkLive(booking, expiresAt, cancellationHours, grace, now)

						if canRefund && !linkLive {
							t.Errorf("CanRefund=true but LinkLive=false (cancellationHours=%d grace=%v expiresAt=%s bookingDate=%s) — "+
								"a refund-eligible cancellation must never be rejected as expired-token",
								cancellationHours, grace, expiresAtName, bookingDateName)
						}
					})
				}
			}
		}
	}
}

// fmtCase names a matrix cell for t.Run.
func fmtCase(cancellationHours int, grace time.Duration, expiresAtName, bookingDateName string) string {
	return "hours=" + strconv.Itoa(cancellationHours) + "/grace=" + grace.String() +
		"/expires=" + expiresAtName + "/booking=" + bookingDateName
}
