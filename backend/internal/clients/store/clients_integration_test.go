//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// TestIntegration_ClientTotalBookingsCountsBookingsCreatedOutsideTheMPWebhook
// is the real-database regression for the bug found via the frontend audit:
// clients.total_bookings used to be a denormalized counter that only the
// MercadoPago webhook incremented (and, as it turned out, that increment
// never even persisted — UpdateClient's SET list never included the column).
// Any booking confirmed another way — the owner's manual "Nueva reserva",
// cash/transfer confirmation, or a fixture inserted directly like this one —
// left it at 0 forever, while the client detail's own recent-bookings list
// showed the real bookings right below it.
//
// GetByID now computes total_bookings live from the bookings table, so this
// asserts that count directly: confirmed/completed/no_show bookings count,
// pending and cancelled ones don't, regardless of how they were inserted.
func TestIntegration_ClientTotalBookingsCountsBookingsCreatedOutsideTheMPWebhook(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	insertBooking := func(t *testing.T, status string, startTime string, durationMinutes int) {
		t.Helper()
		_, err := f.Pool.Exec(ctx, `
			INSERT INTO bookings (complex_id, court_id, client_id, date, start_time, duration_minutes, price, status)
			VALUES ($1, $2, $3, '2026-01-15', $4, $5, 15000, $6)`,
			f.ComplexID, f.CourtID, f.ClientID, startTime, durationMinutes, status,
		)
		if err != nil {
			t.Fatalf("inserting %s booking: %v", status, err)
		}
	}

	// Two bookings that must count, inserted the way this bug's real-world
	// trigger looked: never touched by internal/payments/process.go at all.
	insertBooking(t, "confirmed", "09:00", 90)
	insertBooking(t, "completed", "10:30", 90)
	// Must count too — a no_show still happened.
	insertBooking(t, "no_show", "12:00", 90)
	// Must NOT count — neither reached, nor will.
	insertBooking(t, "pending", "14:00", 90)
	insertBooking(t, "cancelled", "16:00", 90)

	client, err := f.Stores.Clients.GetByID(ctx, f.ClientID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if client.TotalBookings != 3 {
		t.Errorf("TotalBookings = %d, want 3 (confirmed + completed + no_show; pending and cancelled excluded)", client.TotalBookings)
	}
}

// TestIntegration_UpdateClientScopesToTenant asserts UpdateClient's tenant
// predicate: an update issued under another complex's context touches zero
// rows rather than editing a client that belongs to someone else.
func TestIntegration_UpdateClientScopesToTenant(t *testing.T) {
	f := datatest.NewFixture(t)

	foreignCtx := data.ContextWithTenant(context.Background(), uuid.New())
	err := f.Stores.Clients.Update(foreignCtx, &store.Client{
		ID:        f.ClientID,
		FirstName: "Someone Else",
		LastName:  "Diaz",
		Phone:     "+5491100000099",
	})
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("Update for another tenant: want ErrRecordNotFound; got %v", err)
	}

	ownCtx := data.ContextWithTenant(context.Background(), f.ComplexID)
	got, err := f.Stores.Clients.GetByID(ownCtx, f.ClientID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.FirstName != "Ana" {
		t.Errorf("FirstName = %q, want unchanged %q (the foreign update must not have touched the row)", got.FirstName, "Ana")
	}
}
