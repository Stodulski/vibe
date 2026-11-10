//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// snapshotBooking is a storefront booking the way POST /book now stores it:
// pending, unpaid, no client, and a snapshot of who it is for.
func snapshotBooking(f *datatest.Fixture, phone string) *bookingstore.Booking {
	b := f.NewBooking(datatest.BookingOptions{
		Public:           true,
		Status:           "pending",
		CollectionStatus: bookingstore.CollectionStatusUnpaid,
	})
	b.ClientID = uuid.Nil
	b.ClientSnapshot = &bookingstore.ClientSnapshot{FirstName: "Ana", LastName: "Perez", Phone: phone, Email: "ana@example.com"}
	return b
}

func countClientsWithPhone(t *testing.T, f *datatest.Fixture, phone string) int {
	t.Helper()
	var n int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM clients WHERE complex_id = $1 AND phone = $2`, f.ComplexID, phone).Scan(&n); err != nil {
		t.Fatalf("counting clients: %v", err)
	}
	return n
}

func TestInsertSafeStoresASnapshotBookingWithNoClient(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	phone := "+5491100" + uuid.NewString()[:6]

	b := snapshotBooking(f, phone)
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}

	if n := countClientsWithPhone(t, f, phone); n != 0 {
		t.Errorf("a pending storefront booking must create no client; found %d", n)
	}
	var clientID *uuid.UUID
	var first, last, storedPhone, email *string
	if err := f.DB.QueryRow(context.Background(), `
		SELECT client_id, client_first_name, client_last_name, client_phone, client_email::text
		FROM bookings WHERE id = $1`, b.ID).Scan(&clientID, &first, &last, &storedPhone, &email); err != nil {
		t.Fatalf("reading the row: %v", err)
	}
	if clientID != nil {
		t.Errorf("want client_id NULL; got %v", *clientID)
	}
	if first == nil || *first != "Ana" || last == nil || *last != "Perez" ||
		storedPhone == nil || *storedPhone != phone || email == nil || *email != "ana@example.com" {
		t.Errorf("want the snapshot stored; got %v %v %v %v", first, last, storedPhone, email)
	}

	got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.HasClient() {
		t.Errorf("want no client read back; got %v", got.ClientID)
	}
	if got.ClientSnapshot == nil || *got.ClientSnapshot != *b.ClientSnapshot {
		t.Errorf("want the snapshot read back; got %+v", got.ClientSnapshot)
	}
	if got.ClientName != "Ana Perez" || got.ClientPhone != phone {
		t.Errorf("want the enrichment from the snapshot; got %q %q", got.ClientName, got.ClientPhone)
	}
	if got.ClientIDOrNil() != nil {
		t.Error("ClientIDOrNil must be nil for a booking with no client")
	}
}

func TestABookingNeedsAClientOrASnapshot(t *testing.T) {
	f := datatest.Isolated(t)
	b := snapshotBooking(f, "+5491100000001")
	b.ClientSnapshot = nil
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), b); err == nil {
		t.Fatal("a booking with neither a client nor a snapshot must be refused (bookings_client_or_snapshot)")
	}
}

// The expiry sweep and the MercadoPago-disconnect release read unpaid holds
// with the client's contact; a hold with no client reads its snapshot instead
// of dropping out of an inner join and holding its slot forever.
func TestUnpaidHoldsWithNoClientAreReadWithTheirSnapshot(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	phone := "+5491100" + uuid.NewString()[:6]

	b := snapshotBooking(f, phone)
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	f.BackdateBookingCreatedAt(t, b.ID, time.Hour)

	expired, err := f.Stores.Bookings.GetExpiredPendingEnriched(ctx, 15*time.Minute)
	if err != nil {
		t.Fatalf("GetExpiredPendingEnriched: %v", err)
	}
	holds, err := f.Stores.Bookings.GetUnpaidPublicPendingEnriched(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetUnpaidPublicPendingEnriched: %v", err)
	}
	for name, list := range map[string][]*bookingstore.CronBooking{"expired": expired, "holds": holds} {
		var found *bookingstore.CronBooking
		for _, cb := range list {
			if cb.ID == b.ID {
				found = cb
			}
		}
		if found == nil {
			t.Errorf("%s: the hold with no client is missing", name)
			continue
		}
		if found.ClientPhone != phone || found.ClientEmail != "ana@example.com" {
			t.Errorf("%s: want the snapshot contact; got %q %q", name, found.ClientPhone, found.ClientEmail)
		}
	}
}

// A public cancel resolves the booking through its link, which reads no
// snapshot; the write itself hands it back so the notice can name the client.
func TestUpdateReturnsTheSnapshotOfABookingWithNoClient(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	b := snapshotBooking(f, "+5491100000002")
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	b.ClientSnapshot = nil
	b.Status = "cancelled"
	if err := f.Stores.Bookings.Update(ctx, b); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if b.ClientSnapshot == nil || b.ClientSnapshot.Phone != "+5491100000002" {
		t.Errorf("want the snapshot back from the write; got %+v", b.ClientSnapshot)
	}
}
