//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// A storefront booking still waiting for its MercadoPago payment is not a
// booking the owner has yet: owner decision 2026-11-08, after an owner saw a
// visitor's abandoned checkouts on the calendar and cancelled them by hand.
// These tests pin that the owner's reads leave it out, while a booking the
// owner made, or one the visitor paid, still shows.

type ownerReadCase struct {
	name    string
	opts    datatest.BookingOptions
	visible bool
}

func ownerReadCases() []ownerReadCase {
	unpaid := bookingstore.CollectionStatus(bookingstore.CollectionStatusUnpaid)
	none := bookingstore.RefundStatus(bookingstore.RefundStatusNone)
	return []ownerReadCase{
		{"a public booking waiting for payment", datatest.BookingOptions{
			StartTime: "09:00", EndTime: "10:30", Status: "pending", CollectionStatus: unpaid, RefundStatus: none, Public: true,
		}, false},
		{"a public booking cancelled without ever being paid", datatest.BookingOptions{
			StartTime: "09:00", EndTime: "10:30", Status: "cancelled", CollectionStatus: unpaid, RefundStatus: none, Public: true,
		}, false},
		{"an owner booking not yet paid", datatest.BookingOptions{
			StartTime: "09:00", EndTime: "10:30", Status: "pending", CollectionStatus: unpaid, RefundStatus: none,
		}, true},
		{"a public booking whose deposit was paid", datatest.BookingOptions{
			StartTime: "09:00", EndTime: "10:30", Status: "confirmed",
			CollectionStatus: bookingstore.CollectionStatusDepositPaid, RefundStatus: none, Public: true,
		}, true},
	}
}

func TestOwnerReadsLeaveOutUnpaidPublicBookings(t *testing.T) {
	for _, tc := range ownerReadCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := datatest.Isolated(t)
			ctx := f.Scoped(context.Background())
			b := f.NewBooking(tc.opts)
			if err := f.Stores.Bookings.Insert(ctx, b); err != nil {
				t.Fatalf("inserting the booking: %v", err)
			}

			listed, _, err := f.Stores.Bookings.GetByComplex(ctx, f.ComplexID, b.Date, b.Date, data.Filters{Limit: 50})
			if err != nil {
				t.Fatalf("GetByComplex: %v", err)
			}
			if got := hasBooking(listed, b.ID); got != tc.visible {
				t.Errorf("calendar list shows it = %v, want %v", got, tc.visible)
			}

			history, err := f.Stores.Bookings.GetByClient(ctx, f.ComplexID, f.ClientID, 50)
			if err != nil {
				t.Fatalf("GetByClient: %v", err)
			}
			if got := hasBooking(history, b.ID); got != tc.visible {
				t.Errorf("client history shows it = %v, want %v", got, tc.visible)
			}

			if tc.opts.Status == "cancelled" {
				return // the counts below already leave cancelled bookings out
			}
			want := 0
			if tc.visible {
				want = 1
			}
			stats, err := f.Stores.Bookings.GetDashboardStats(ctx, f.ComplexID, b.Date)
			if err != nil {
				t.Fatalf("GetDashboardStats: %v", err)
			}
			if stats.TodayBookings != want {
				t.Errorf("dashboard counts %d bookings that day, want %d", stats.TodayBookings, want)
			}
			wantPending := 0
			if tc.visible && tc.opts.Status == "pending" {
				wantPending = 1
			}
			if stats.PendingBookings != wantPending {
				t.Errorf("dashboard counts %d pending, want %d", stats.PendingBookings, wantPending)
			}

			occupancy, err := f.Stores.Bookings.GetOccupancyByHourDay(ctx, f.ComplexID, b.Date, b.Date)
			if err != nil {
				t.Fatalf("GetOccupancyByHourDay: %v", err)
			}
			total := 0
			for _, p := range occupancy {
				total += p.BookingCount
			}
			if total != want {
				t.Errorf("occupancy counts %d bookings, want %d", total, want)
			}
		})
	}
}

func hasBooking(list []*bookingstore.Booking, id uuid.UUID) bool {
	for _, b := range list {
		if b.ID == id {
			return true
		}
	}
	return false
}
