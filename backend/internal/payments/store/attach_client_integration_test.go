//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Owner rule (2026-11-10): a storefront booking gets its client when its
// payment is recorded, inside the same transaction, and never before. These
// pin every write that records a payment on such a booking — the MercadoPago
// webhook settling its checkout row, the counter confirming it, and a late
// payment on a booking already cancelled — and that none of them ever blocks
// or unblocks a client (S7).

func pendingSnapshotBooking(t *testing.T, f *datatest.Fixture, phone string) *bookingstore.Booking {
	t.Helper()
	b := f.NewBooking(datatest.BookingOptions{
		Public:           true,
		Status:           "pending",
		CollectionStatus: bookingstore.CollectionStatusUnpaid,
	})
	b.ClientID = uuid.Nil
	b.ClientSnapshot = &bookingstore.ClientSnapshot{FirstName: "Ana", LastName: "Perez", Phone: phone, Email: "ana@example.com"}
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	return b
}

type clientRow struct {
	id                 uuid.UUID
	first, last, email string
	isBlocked          bool
}

func clientByPhone(t *testing.T, f *datatest.Fixture, phone string) (clientRow, int) {
	t.Helper()
	ctx := context.Background()
	var n int
	if err := f.DB.QueryRow(ctx, `SELECT COUNT(*) FROM clients WHERE complex_id = $1 AND phone = $2`, f.ComplexID, phone).Scan(&n); err != nil {
		t.Fatalf("counting clients: %v", err)
	}
	var c clientRow
	if n == 1 {
		if err := f.DB.QueryRow(ctx, `
			SELECT id, first_name, last_name, COALESCE(email::text, ''), is_blocked
			FROM clients WHERE complex_id = $1 AND phone = $2`, f.ComplexID, phone,
		).Scan(&c.id, &c.first, &c.last, &c.email, &c.isBlocked); err != nil {
			t.Fatalf("reading client: %v", err)
		}
	}
	return c, n
}

func bookingClientColumns(t *testing.T, f *datatest.Fixture, id uuid.UUID) (clientID *uuid.UUID, snapshotPhone *string) {
	t.Helper()
	if err := f.DB.QueryRow(context.Background(),
		`SELECT client_id, client_phone FROM bookings WHERE id = $1`, id).Scan(&clientID, &snapshotPhone); err != nil {
		t.Fatalf("reading booking: %v", err)
	}
	return clientID, snapshotPhone
}

func assertAttached(t *testing.T, f *datatest.Fixture, b *bookingstore.Booking, phone string) clientRow {
	t.Helper()
	c, n := clientByPhone(t, f, phone)
	if n != 1 {
		t.Fatalf("want exactly one client for the phone once paid; got %d", n)
	}
	clientID, snapPhone := bookingClientColumns(t, f, b.ID)
	if clientID == nil || *clientID != c.id {
		t.Errorf("want the booking to name client %s; got %v", c.id, clientID)
	}
	if snapPhone != nil {
		t.Errorf("want the snapshot cleared once the client is attached; got %q", *snapPhone)
	}
	if b.ClientID != c.id || b.ClientSnapshot != nil {
		t.Errorf("want the in-memory booking to carry the client for the post-commit notice; got %v %+v", b.ClientID, b.ClientSnapshot)
	}
	return c
}

func TestInsertAndConfirmBookingAttachesTheClientFromTheSnapshot(t *testing.T) {
	f := datatest.Isolated(t)
	phone := "+5491100" + uuid.NewString()[:6]
	b := pendingSnapshotBooking(t, f, phone)

	if err := f.ConfirmBooking(f.Stores, b); err != nil {
		t.Fatalf("ConfirmBooking: %v", err)
	}

	c := assertAttached(t, f, b, phone)
	if c.first != "Ana" || c.last != "Perez" || c.email != "ana@example.com" {
		t.Errorf("want the client created from the snapshot; got %+v", c)
	}
	if c.isBlocked {
		t.Error("a payment must never block a client")
	}

	// A redelivery, or a second payment, leaves the same client.
	b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	if err := f.ConfirmBooking(f.Stores, b); err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	if again := assertAttached(t, f, b, phone); again.id != c.id {
		t.Errorf("want the same client on a second payment; got %s then %s", c.id, again.id)
	}
}

func TestConfirmWebhookPaymentAttachesTheClientFromTheSnapshot(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	phone := "+5491100" + uuid.NewString()[:6]
	b := pendingSnapshotBooking(t, f, phone)
	mpID := "mp-" + uuid.NewString()[:8]
	p := f.CreatePayment(t, b.ID, b.DepositAmount, 10_500, &mpID)

	b.Status = "confirmed"
	b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	p.Status = "deposit_paid"
	if err := f.Stores.Payments.ConfirmWebhookPayment(ctx, p, b); err != nil {
		t.Fatalf("ConfirmWebhookPayment: %v", err)
	}
	assertAttached(t, f, b, phone)
}

// A payment that lands after the booking was cancelled is recorded so it can be
// refunded, and that makes the booking visible to the owner: it names a client.
func TestALatePaymentOnACancelledBookingAttachesTheClient(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	phone := "+5491100" + uuid.NewString()[:6]
	b := pendingSnapshotBooking(t, f, phone)
	f.MarkStatus(t, b.ID, "cancelled")
	b.Status = "cancelled"

	p := &paymentstore.Payment{
		BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount, ServiceFee: 10_500,
		Method: "mercadopago", Status: "deposit_paid",
	}
	b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	if err := f.Stores.Payments.InsertAndConfirmBooking(ctx, p, b); err != nil {
		t.Fatalf("InsertAndConfirmBooking on a cancelled booking: %v", err)
	}
	assertAttached(t, f, b, phone)
}

// An existing client matched by phone keeps its name (the snapshot came from an
// unauthenticated request) and keeps whatever block the owner set: a payment
// neither blocks nor unblocks anyone.
func TestAttachingAnExistingClientNeitherRenamesNorChangesTheBlock(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		f := datatest.Isolated(t)
		phone := "+5491100" + uuid.NewString()[:6]
		var existing uuid.UUID
		if err := f.DB.QueryRow(context.Background(), `
			INSERT INTO clients (complex_id, first_name, last_name, phone, is_blocked)
			VALUES ($1, 'Beatriz', 'Gomez', $2, $3) RETURNING id`, f.ComplexID, phone, blocked).Scan(&existing); err != nil {
			t.Fatalf("seeding client: %v", err)
		}
		b := pendingSnapshotBooking(t, f, phone)

		if err := f.ConfirmBooking(f.Stores, b); err != nil {
			t.Fatalf("ConfirmBooking: %v", err)
		}
		c := assertAttached(t, f, b, phone)
		if c.id != existing {
			t.Errorf("want the existing client attached; got %s", c.id)
		}
		if c.first != "Beatriz" || c.last != "Gomez" {
			t.Errorf("the public snapshot must not rename an existing client; got %s %s", c.first, c.last)
		}
		if c.isBlocked != blocked {
			t.Errorf("is_blocked changed from %v to %v on a payment", blocked, c.isBlocked)
		}
		if c.email != "ana@example.com" {
			t.Errorf("want the missing email backfilled, as GetOrCreate does; got %q", c.email)
		}
	}
}

// An owner's booking already names its client; recording its payment changes
// nothing about who it is for.
func TestConfirmingABookingWithAClientLeavesItAlone(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid})
	if err := f.ConfirmBooking(f.Stores, b); err != nil {
		t.Fatalf("ConfirmBooking: %v", err)
	}
	clientID, _ := bookingClientColumns(t, f, b.ID)
	if clientID == nil || *clientID != f.ClientID || b.ClientID != f.ClientID {
		t.Errorf("want the owner booking's client unchanged; got %v", clientID)
	}
}

// A payment write that read its booking before a concurrent one (a redelivered
// webhook, the counter at the same moment) attached the client finds no
// snapshot left; it must still come away carrying the client the other write
// stored, or its notice and its audit entry would name nobody.
func TestALaterPaymentWriteOnAStaleCopyCarriesTheAttachedClient(t *testing.T) {
	f := datatest.Isolated(t)
	phone := "+5491100" + uuid.NewString()[:6]
	b := pendingSnapshotBooking(t, f, phone)
	stale := *b
	stale.ClientSnapshot = &bookingstore.ClientSnapshot{FirstName: "Ana", LastName: "Perez", Phone: phone, Email: "ana@example.com"}

	if err := f.ConfirmBooking(f.Stores, b); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	c := assertAttached(t, f, b, phone)

	stale.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	if err := f.ConfirmBooking(f.Stores, &stale); err != nil {
		t.Fatalf("second confirm on the stale copy: %v", err)
	}
	if stale.ClientID != c.id || stale.ClientSnapshot != nil {
		t.Errorf("want the stale copy to carry client %s; got %v %+v", c.id, stale.ClientID, stale.ClientSnapshot)
	}
}
