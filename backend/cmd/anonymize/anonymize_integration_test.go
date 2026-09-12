//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is the one test that proves the two halves of the promise the runbook
// makes to a person who asks to be removed: their identity is gone, and the
// venue's books are not.

type fixture struct {
	pool      *pgxpool.Pool
	complexID uuid.UUID
	clientID  uuid.UUID
	bookingID uuid.UUID
	paymentID uuid.UUID
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set, skipping")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// newFixture seeds one venue with one client who has a booking carrying notes
// and a payment against it — the shape of the person the runbook is about.
func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{pool: openPool(t)}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if _, err := f.pool.Exec(ctx, `SET app.bypass_tenant = 'on'`); err != nil {
		t.Fatalf("declaring the cross-tenant scope: %v", err)
	}

	suffix := uuid.NewString()
	var ownerID uuid.UUID
	mustScan := func(dst *uuid.UUID, query string, args ...any) {
		t.Helper()
		if err := f.pool.QueryRow(ctx, query, args...).Scan(dst); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	mustScan(&ownerID, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, $2, 'Owner', 'Anon', '+5491100000000', 'owner', true)
		RETURNING id`, "anon-"+suffix+"@example.test", []byte("not-a-real-hash"))

	mustScan(&f.complexID, `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, 'Anon Padel', $2, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000001')
		RETURNING id`, ownerID, "anon-"+suffix)

	var courtID uuid.UUID
	mustScan(&courtID, `INSERT INTO courts (complex_id, name) VALUES ($1, 'Cancha 1') RETURNING id`, f.complexID)

	mustScan(&f.clientID, `
		INSERT INTO clients (complex_id, first_name, last_name, phone, email, notes)
		VALUES ($1, 'Ana', 'Diaz', $2, $3, 'prefiere la cancha 2')
		RETURNING id`, f.complexID, "+54911"+suffix[:8], "ana-"+suffix+"@example.test")

	mustScan(&f.bookingID, `
		INSERT INTO bookings (complex_id, court_id, client_id, date, start_time, duration_minutes, price, notes)
		VALUES ($1, $2, $3, CURRENT_DATE + 120, '18:00', 90, 500000, 'Ana Diaz, llamar al +5491112345678')
		RETURNING id`, f.complexID, courtID, f.clientID)

	mustScan(&f.paymentID, `
		INSERT INTO payments (booking_id, complex_id, amount, method, status)
		VALUES ($1, $2, 150000, 'mercadopago', 'deposit_paid')
		RETURNING id`, f.bookingID, f.complexID)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cleanupCancel()
		if _, err := f.pool.Exec(cleanupCtx, `SET app.bypass_tenant = 'on'`); err != nil {
			t.Errorf("cleaning up: %v", err)
			return
		}
		for _, stmt := range []string{
			`DELETE FROM payments WHERE complex_id = $1`,
			`DELETE FROM booking_link_tokens WHERE complex_id = $1`,
			`DELETE FROM bookings WHERE complex_id = $1`,
			`DELETE FROM clients WHERE complex_id = $1`,
			`DELETE FROM courts WHERE complex_id = $1`,
			`DELETE FROM audit_log WHERE complex_id = $1`,
			`DELETE FROM complexes WHERE id = $1`,
		} {
			if _, err := f.pool.Exec(cleanupCtx, stmt, f.complexID); err != nil {
				t.Errorf("cleaning up (%s): %v", stmt, err)
			}
		}
		if _, err := f.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, ownerID); err != nil {
			t.Errorf("cleaning up the owner: %v", err)
		}
	})

	return f
}

// inTx runs fn in its own transaction and commits, which is what -apply does.
func (f *fixture) inTx(t *testing.T, fn func(tx pgx.Tx) error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("opening the transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		t.Fatalf("running: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("committing: %v", err)
	}
}

func TestAnonymizeErasesThePersonAndKeepsTheMoney(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var result Result
	f.inTx(t, func(tx pgx.Tx) error {
		var err error
		result, err = anonymizeClient(ctx, tx, f.clientID)
		return err
	})

	if !result.ClientFound {
		t.Fatal("the seeded client must be found")
	}
	if result.Complex != f.complexID {
		t.Errorf("want complex %s; got %s", f.complexID, result.Complex)
	}
	if result.BookingNotesCleared != 1 {
		t.Errorf("want one booking's notes cleared; got %d", result.BookingNotesCleared)
	}

	if _, err := f.pool.Exec(ctx, `SET app.bypass_tenant = 'on'`); err != nil {
		t.Fatalf("declaring the cross-tenant scope: %v", err)
	}

	// The identity is gone.
	var firstName, lastName, phone string
	var email, notes *string
	err := f.pool.QueryRow(ctx,
		`SELECT first_name, last_name, phone, email::text, notes FROM clients WHERE id = $1`, f.clientID,
	).Scan(&firstName, &lastName, &phone, &email, &notes)
	if err != nil {
		t.Fatalf("reading the client back: %v", err)
	}
	if firstName != anonymizedFirstName || lastName != anonymizedLastName {
		t.Errorf("want the placeholder name; got %q %q", firstName, lastName)
	}
	if phone != anonymizedPhone(f.clientID) {
		t.Errorf("want the derived placeholder phone; got %q", phone)
	}
	if email != nil {
		t.Errorf("want the email erased; got %q", *email)
	}
	if notes != nil {
		t.Errorf("want the client notes erased; got %q", *notes)
	}

	// So is the name and the number somebody typed onto the booking.
	var bookingNotes *string
	if err := f.pool.QueryRow(ctx, `SELECT notes FROM bookings WHERE id = $1`, f.bookingID).Scan(&bookingNotes); err != nil {
		t.Fatalf("reading the booking back: %v", err)
	}
	if bookingNotes != nil {
		t.Errorf("want the booking notes erased; got %q", *bookingNotes)
	}

	// The money is not. The booking row, its client_id, and the payment
	// against it all survive: the venue's books have to still add up, and the
	// row they point at now names nobody.
	var bookingClient uuid.UUID
	var price int
	if err := f.pool.QueryRow(ctx,
		`SELECT client_id, price FROM bookings WHERE id = $1`, f.bookingID).Scan(&bookingClient, &price); err != nil {
		t.Fatalf("the booking must survive: %v", err)
	}
	if bookingClient != f.clientID || price != 500000 {
		t.Errorf("the booking must be unchanged apart from its notes; got client %s price %d", bookingClient, price)
	}

	var amount int
	var status string
	if err := f.pool.QueryRow(ctx,
		`SELECT amount, status::text FROM payments WHERE id = $1`, f.paymentID).Scan(&amount, &status); err != nil {
		t.Fatalf("the payment must survive: %v", err)
	}
	if amount != 150000 || status != "deposit_paid" {
		t.Errorf("the payment must be untouched; got %d %s", amount, status)
	}
}

// Running it twice must not fail. An operator who is not sure whether the first
// run landed will run it again, and the second run has to be a quiet no-change
// rather than a unique-constraint error on the placeholder phone.
func TestAnonymizeIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for attempt := range 2 {
		var result Result
		f.inTx(t, func(tx pgx.Tx) error {
			var err error
			result, err = anonymizeClient(ctx, tx, f.clientID)
			return err
		})
		if !result.ClientFound {
			t.Fatalf("attempt %d: the client must still be found", attempt+1)
		}
		if attempt == 1 && result.BookingNotesCleared != 0 {
			t.Errorf("the second run has no notes left to clear; got %d", result.BookingNotesCleared)
		}
	}
}

// An id that matches nothing must be reported as such rather than as a
// successful no-op: "we anonymized nobody" and "we anonymized the person who
// asked" must not look the same in the record of the request.
func TestAnonymizeReportsAnUnknownClient(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var result Result
	f.inTx(t, func(tx pgx.Tx) error {
		var err error
		result, err = anonymizeClient(ctx, tx, uuid.New())
		return err
	})

	if result.ClientFound {
		t.Error("an id matching no row must not report a client")
	}
}
