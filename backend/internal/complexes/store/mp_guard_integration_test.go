//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// connectedUserID reads mp_user_id straight off the row, bypassing the store's
// accessors, so a test sees exactly what a refused write left behind.
func connectedUserID(t *testing.T, f *datatest.Fixture) *string {
	t.Helper()

	var userID *string
	err := f.DB.QueryRow(f.Scoped(context.Background()),
		`SELECT mp_user_id FROM complexes WHERE id = $1`, f.ComplexID).Scan(&userID)
	if err != nil {
		t.Fatalf("reading mp_user_id: %v", err)
	}
	return userID
}

// Disconnecting is refused while a booking is live, and the refusal leaves the
// connection exactly as it was: the check and the clear are one transaction, so
// a refusal cannot have cleared anything.
func TestIntegration_DisconnectMPCredentialsIsRefusedWhileBookingsAreActive(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	if err := f.Stores.Complexes.UpdateMPCredentials(ctx, f.ComplexID, "access", "refresh", "seller-1", 0); err != nil {
		t.Fatalf("UpdateMPCredentials: %v", err)
	}
	booking := f.CreateBooking(t, datatest.BookingOptions{})

	err := f.Stores.Complexes.DisconnectMPCredentials(ctx, f.ComplexID)
	if !errors.Is(err, complexstore.ErrActiveBookings) {
		t.Fatalf("DisconnectMPCredentials with a live booking = %v, want ErrActiveBookings", err)
	}
	if got := connectedUserID(t, f); got == nil || *got != "seller-1" {
		t.Errorf("a refused disconnect changed mp_user_id to %v; it must stay seller-1", got)
	}

	// Once the hours are released the same call goes through.
	f.MarkStatus(t, booking.ID, "cancelled")
	if err := f.Stores.Complexes.DisconnectMPCredentials(ctx, f.ComplexID); err != nil {
		t.Fatalf("DisconnectMPCredentials after the booking was cancelled: %v", err)
	}
	if got := connectedUserID(t, f); got != nil {
		t.Errorf("mp_user_id = %q after a successful disconnect, want NULL", *got)
	}
}

// The reconnect guard: a different seller account is refused while bookings are
// live and nothing is written, the same account only refreshes its tokens, and a
// first connection or a quiet complex is never blocked.
func TestIntegration_ConnectMPCredentialsGuardsAgainstSwappingTheSellerAccount(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	complexes := f.Stores.Complexes

	// First connection: nothing to protect, even with a booking on the books.
	booking := f.CreateBooking(t, datatest.BookingOptions{})
	if err := complexes.ConnectMPCredentials(ctx, f.ComplexID, "access-1", "refresh-1", "seller-1", 3600); err != nil {
		t.Fatalf("first ConnectMPCredentials: %v", err)
	}

	// The same account again, bookings live: tokens refresh.
	if err := complexes.ConnectMPCredentials(ctx, f.ComplexID, "access-2", "refresh-2", "seller-1", 3600); err != nil {
		t.Fatalf("reconnecting the same account with active bookings must work: %v", err)
	}
	complex, err := complexes.GetByID(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got, err := complex.SellerAccessToken(); err != nil || got != "access-2" {
		t.Errorf("SellerAccessToken() = (%q, %v), want (access-2, nil): the same-account reconnect did not refresh", got, err)
	}

	// A different account, bookings live: refused, and nothing is stored.
	err = complexes.ConnectMPCredentials(ctx, f.ComplexID, "access-3", "refresh-3", "seller-2", 3600)
	if !errors.Is(err, complexstore.ErrActiveBookings) {
		t.Fatalf("connecting a different account with a live booking = %v, want ErrActiveBookings", err)
	}
	complex, err = complexes.GetByID(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got, err := complex.SellerAccessToken(); err != nil || got != "access-2" {
		t.Errorf("SellerAccessToken() = (%q, %v) after a refused swap, want the old access-2", got, err)
	}
	if got := connectedUserID(t, f); got == nil || *got != "seller-1" {
		t.Errorf("a refused swap changed mp_user_id to %v; it must stay seller-1", got)
	}

	// With the hours released the swap is allowed.
	f.MarkStatus(t, booking.ID, "cancelled")
	if err := complexes.ConnectMPCredentials(ctx, f.ComplexID, "access-3", "refresh-3", "seller-2", 3600); err != nil {
		t.Fatalf("connecting a different account with no live booking: %v", err)
	}
	if got := connectedUserID(t, f); got == nil || *got != "seller-2" {
		t.Errorf("mp_user_id = %v, want seller-2", got)
	}
}
