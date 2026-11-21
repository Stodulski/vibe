//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
)

// attachSnapshot runs AttachClientFromSnapshot the way the payment write does,
// inside its own transaction.
func attachSnapshot(t *testing.T, f *datatest.Fixture, b *bookingstore.Booking) {
	t.Helper()
	ctx := f.Scoped(context.Background())
	if err := f.DB.WithTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		return bookingstore.AttachClientFromSnapshot(ctx, tx, q, b)
	}); err != nil {
		t.Fatalf("AttachClientFromSnapshot: %v", err)
	}
}

// A returning client books online and types a different email than the one
// the venue has on file. The client row keeps its own email (an
// unauthenticated request never overwrites it), and the booking's notices go
// to the address typed for this booking. Attaching the client used to clear the
// typed email with the rest of the snapshot, so every notice of the booking —
// confirmation, reminder, cancellation, refund — went to the old address.
func TestTheEmailTypedForABookingOutlivesAttachingItsClient(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	phone := "+5491100" + uuid.NewString()[:6]

	if _, err := f.Stores.Clients.GetOrCreate(ctx, f.ComplexID, "Ana", "Perez", phone, "old@example.com", false); err != nil {
		t.Fatalf("seeding the returning client: %v", err)
	}

	b := snapshotBooking(f, phone)
	b.ClientSnapshot.Email = "new@example.com"
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	attachSnapshot(t, f, b)

	if b.ContactEmail != "new@example.com" {
		t.Errorf("the attached booking must carry the email typed for it; got %q", b.ContactEmail)
	}

	got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.HasClient() {
		t.Fatal("the booking must name its client once attached")
	}
	contact, err := bookingstore.ContactOf(ctx, f.Stores.Clients, got)
	if err != nil {
		t.Fatalf("ContactOf: %v", err)
	}
	if contact.Email != "new@example.com" {
		t.Errorf("notices must go to the email typed for this booking; got %q", contact.Email)
	}

	client, err := f.Stores.Clients.GetByID(ctx, got.ClientID)
	if err != nil {
		t.Fatalf("reading the client: %v", err)
	}
	if client.Email == nil || *client.Email != "old@example.com" {
		t.Errorf("the client's own email must not be overwritten by a storefront booking; got %v", client.Email)
	}
}

// A booking with no email of its own is told at the client's.
func TestABookingWithNoTypedEmailUsesTheClients(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	phone := "+5491100" + uuid.NewString()[:6]

	if _, err := f.Stores.Clients.GetOrCreate(ctx, f.ComplexID, "Ana", "Perez", phone, "old@example.com", false); err != nil {
		t.Fatalf("seeding the returning client: %v", err)
	}
	b := snapshotBooking(f, phone)
	b.ClientSnapshot.Email = ""
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	attachSnapshot(t, f, b)

	got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	contact, err := bookingstore.ContactOf(ctx, f.Stores.Clients, got)
	if err != nil {
		t.Fatalf("ContactOf: %v", err)
	}
	if contact.Email != "old@example.com" {
		t.Errorf("want the client's email when the booking has none; got %q", contact.Email)
	}
}
