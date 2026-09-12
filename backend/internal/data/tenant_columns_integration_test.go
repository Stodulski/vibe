//go:build integration

package data_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Two things have to hold for the tenant boundary to be worth its name, and
// until 005_tenant_columns.sql only the first of them did:
//
//  1. Row-level security refuses a cross-tenant read. That is what
//     rls_integration_test.go proves, and it holds for every table.
//  2. The QUERY refuses it too, on its own, with a predicate a human reading
//     the SQL can see. That is what this file proves, by asking the by-id
//     queries the cross-tenant question with the policies deliberately switched
//     off — app.bypass_tenant = 'on', which makes RLS admit everything — so the
//     only thing that can refuse is the predicate itself.
//
// The second is not redundancy for its own sake. A policy can be relaxed for a
// migration, a path can take a bypass it did not need (the cron sweeps, the
// superadmin console and the MercadoPago webhook all legitimately hold one),
// and a future reader of a by-id query has no way to tell from the query
// whether it is scoped. After this, they do.

// bypassed returns a context whose SQL session sees every tenant, so RLS
// cannot be what refuses anything below.
func bypassed() context.Context { return data.ContextWithTenantBypass(context.Background()) }

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// queriesOverApp returns sqlc's generated queries over the application pool, so
// the statements under test are the exact ones the stores run.
func queriesOverApp(f *rlsFixture) *db.Queries { return db.New(f.App) }

func TestTheByIDQueriesRefuseAnotherTenantWithoutRLS(t *testing.T) {
	f := newRLSFixture(t)
	q := queriesOverApp(f)
	ctx, cancel := context.WithTimeout(bypassed(), 15*time.Second)
	defer cancel()

	// Each case reads one of tenant B's rows while naming tenant A as the
	// complex. The session is bypassed, so every one of these rows IS visible
	// to the policies; only the predicate can say no.
	t.Run("booking", func(t *testing.T) {
		if _, err := q.GetBookingByID(ctx, db.GetBookingByIDParams{
			ID: pgUUID(f.B.BookingID), ComplexID: pgUUID(f.A.ComplexID),
		}); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("want no rows for tenant B's booking read as tenant A; got %v", err)
		}
		if _, err := q.GetBookingByID(ctx, db.GetBookingByIDParams{
			ID: pgUUID(f.B.BookingID), ComplexID: pgUUID(f.B.ComplexID),
		}); err != nil {
			t.Errorf("tenant B reading its own booking: %v", err)
		}
	})

	t.Run("court", func(t *testing.T) {
		if _, err := q.GetCourtByID(ctx, db.GetCourtByIDParams{
			ID: pgUUID(f.B.CourtID), ComplexID: pgUUID(f.A.ComplexID),
		}); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("want no rows for tenant B's court read as tenant A; got %v", err)
		}
		if _, err := q.GetCourtByID(ctx, db.GetCourtByIDParams{
			ID: pgUUID(f.B.CourtID), ComplexID: pgUUID(f.B.ComplexID),
		}); err != nil {
			t.Errorf("tenant B reading its own court: %v", err)
		}
	})

	t.Run("client", func(t *testing.T) {
		if _, err := q.GetClientByID(ctx, db.GetClientByIDParams{
			ID: pgUUID(f.B.ClientID), ComplexID: pgUUID(f.A.ComplexID),
		}); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("want no rows for tenant B's client read as tenant A; got %v", err)
		}
		if _, err := q.GetClientByID(ctx, db.GetClientByIDParams{
			ID: pgUUID(f.B.ClientID), ComplexID: pgUUID(f.B.ComplexID),
		}); err != nil {
			t.Errorf("tenant B reading its own client: %v", err)
		}
	})

	// The payment lookup is the one that moves money, and it is the one that
	// most needs both halves: it runs from the MercadoPago webhook, which holds
	// the bypass by design because it resolves its own tenant from a payment id.
	t.Run("payment", func(t *testing.T) {
		paymentID := f.seedPayment(ctx, t, f.B)

		tx, err := f.App.Begin(ctx)
		if err != nil {
			t.Fatalf("opening a transaction for the locking read: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		qtx := db.New(tx)

		if _, err := qtx.GetPaymentByIDForUpdate(ctx, db.GetPaymentByIDForUpdateParams{
			ID: pgUUID(paymentID), ComplexID: pgUUID(f.A.ComplexID),
		}); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("want no rows for tenant B's payment read as tenant A; got %v", err)
		}
		if _, err := qtx.GetPaymentByIDForUpdate(ctx, db.GetPaymentByIDForUpdateParams{
			ID: pgUUID(paymentID), ComplexID: pgUUID(f.B.ComplexID),
		}); err != nil {
			t.Errorf("tenant B reading its own payment: %v", err)
		}
	})
}

// The NULL branch is what every bypassed caller passes, and it must still work:
// the cron sweeps and the webhook have no tenant to name, and a predicate that
// refused them would break the paths RLS deliberately lets through.
func TestTheByIDQueriesStillAnswerAnUnscopedCaller(t *testing.T) {
	f := newRLSFixture(t)
	q := queriesOverApp(f)
	ctx, cancel := context.WithTimeout(bypassed(), 15*time.Second)
	defer cancel()

	if _, err := q.GetBookingByID(ctx, db.GetBookingByIDParams{
		ID: pgUUID(f.B.BookingID), ComplexID: data.TenantParam(bypassed()),
	}); err != nil {
		t.Fatalf("a bypassed caller passing no tenant must still read the booking: %v", err)
	}
}

// And through the store, with the policies on rather than off: the predicate
// and RLS agree about the same row.
func TestTheStoreRefusesAnotherTenantsBookingByID(t *testing.T) {
	f := newRLSFixture(t)
	ctx := data.ContextWithTenant(context.Background(), f.A.ComplexID)

	if _, err := f.Stores.Bookings.GetByID(ctx, f.B.BookingID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound reading tenant B's booking as tenant A; got %v", err)
	}
	if _, err := f.Stores.Bookings.GetByID(ctx, f.A.BookingID); err != nil {
		t.Fatalf("tenant A reading its own booking: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The four child tables
// ---------------------------------------------------------------------------

// The column is derived, and the trigger is what makes it so: nothing has to
// supply it, and a caller that supplies the wrong tenant has it overwritten
// rather than stored. A denormalised copy that can disagree with its parent is
// worse than no copy at all — it is a filter that quietly matches the wrong
// rows.
func TestTheChildTablesDeriveTheirTenantFromTheirParent(t *testing.T) {
	f := newRLSFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var priceComplex, blockComplex, tokenComplex uuid.UUID

	// court_prices: inserted without naming a complex at all.
	err := f.Admin.QueryRow(ctx, `
		INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
		VALUES ($1, 1000, 'monday', '08:00', '09:00')
		RETURNING complex_id`, f.B.CourtID).Scan(&priceComplex)
	if err != nil {
		t.Fatalf("inserting a price band: %v", err)
	}
	if priceComplex != f.B.ComplexID {
		t.Errorf("court_prices.complex_id is %s, want the court's own %s", priceComplex, f.B.ComplexID)
	}

	// blocked_slots: inserted naming tenant A's complex, which is a lie the
	// trigger must not store.
	err = f.Admin.QueryRow(ctx, `
		INSERT INTO blocked_slots (court_id, complex_id, date, start_time, end_time)
		VALUES ($1, $2, CURRENT_DATE + 30, '08:00', '09:00')
		RETURNING complex_id`, f.B.CourtID, f.A.ComplexID).Scan(&blockComplex)
	if err != nil {
		t.Fatalf("inserting a blocked slot: %v", err)
	}
	if blockComplex != f.B.ComplexID {
		t.Errorf("blocked_slots.complex_id is %s; a supplied tenant must be overwritten by the court's own %s",
			blockComplex, f.B.ComplexID)
	}

	// booking_link_tokens hangs off a booking rather than a court.
	err = f.Admin.QueryRow(ctx, `
		INSERT INTO booking_link_tokens (booking_id, token_hash, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 day')
		RETURNING complex_id`, f.B.BookingID, []byte(uuid.NewString())).Scan(&tokenComplex)
	if err != nil {
		t.Fatalf("inserting a link token: %v", err)
	}
	if tokenComplex != f.B.ComplexID {
		t.Errorf("booking_link_tokens.complex_id is %s, want the booking's own %s", tokenComplex, f.B.ComplexID)
	}
}

// The policies now compare a column instead of running an EXISTS subquery, so
// they are worth re-proving on all four tables rather than assuming the shape
// carried over.
func TestTheChildTablesAreStillIsolated(t *testing.T) {
	f := newRLSFixture(t)
	seedCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := f.Admin.Exec(seedCtx, `
		INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
		VALUES ($1, 1000, 'monday', '10:00', '11:00')`, f.B.CourtID); err != nil {
		t.Fatalf("seeding tenant B's price band: %v", err)
	}
	if _, err := f.Admin.Exec(seedCtx, `
		INSERT INTO blocked_slots (court_id, date, start_time, end_time)
		VALUES ($1, CURRENT_DATE + 31, '10:00', '11:00')`, f.B.CourtID); err != nil {
		t.Fatalf("seeding tenant B's blocked slot: %v", err)
	}
	if _, err := f.Admin.Exec(seedCtx, `
		INSERT INTO slot_locks (court_id, date, start_time, end_time, expires_at)
		VALUES ($1, CURRENT_DATE + 31, '10:00', '11:00', NOW() + INTERVAL '1 hour')`, f.B.CourtID); err != nil {
		t.Fatalf("seeding tenant B's slot lock: %v", err)
	}
	if _, err := f.Admin.Exec(seedCtx, `
		INSERT INTO booking_link_tokens (booking_id, token_hash, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 day')`, f.B.BookingID, []byte(uuid.NewString())); err != nil {
		t.Fatalf("seeding tenant B's link token: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		for _, stmt := range []string{
			`DELETE FROM booking_link_tokens WHERE complex_id = $1`,
			`DELETE FROM slot_locks WHERE complex_id = $1`,
			`DELETE FROM blocked_slots WHERE complex_id = $1`,
			`DELETE FROM court_prices WHERE complex_id = $1`,
		} {
			if _, err := f.Admin.Exec(cleanupCtx, stmt, f.B.ComplexID); err != nil {
				t.Errorf("cleaning up (%s): %v", stmt, err)
			}
		}
	})

	asA := data.ContextWithTenant(context.Background(), f.A.ComplexID)
	asB := data.ContextWithTenant(context.Background(), f.B.ComplexID)

	for _, table := range []string{"court_prices", "blocked_slots", "slot_locks", "booking_link_tokens"} {
		t.Run(table, func(t *testing.T) {
			var seen int
			if err := f.App.QueryRow(asA, `SELECT COUNT(*)::int FROM `+table).Scan(&seen); err != nil {
				t.Fatalf("counting %s as tenant A: %v", table, err)
			}
			if seen != 0 {
				t.Errorf("tenant A sees %d row(s) of tenant B's %s", seen, table)
			}

			if err := f.App.QueryRow(asB, `SELECT COUNT(*)::int FROM `+table).Scan(&seen); err != nil {
				t.Fatalf("counting %s as tenant B: %v", table, err)
			}
			if seen == 0 {
				t.Errorf("tenant B cannot see its own %s: the policy refuses the owner of the row", table)
			}
		})
	}
}

// The composite foreign key is the half the trigger cannot cover: a statement
// that disables or bypasses the trigger still cannot leave a row naming one
// tenant and a court belonging to another.
func TestAChildRowCannotNameAForeignTenant(t *testing.T) {
	f := newRLSFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := f.Admin.Exec(ctx, `ALTER TABLE court_prices DISABLE TRIGGER set_complex_id`); err != nil {
		t.Fatalf("disabling the trigger: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if _, err := f.Admin.Exec(cleanupCtx, `ALTER TABLE court_prices ENABLE TRIGGER set_complex_id`); err != nil {
			t.Errorf("re-enabling the trigger: %v", err)
		}
	})

	_, err := f.Admin.Exec(ctx, `
		INSERT INTO court_prices (court_id, complex_id, price, day_type, time_from, time_to)
		VALUES ($1, $2, 1000, 'monday', '12:00', '13:00')`, f.B.CourtID, f.A.ComplexID)
	if err == nil {
		t.Fatal("a price band naming tenant A on tenant B's court was stored; the composite FK did not refuse it")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("insert failed with %v, want SQLSTATE 23503 (foreign_key_violation)", err)
	}
}

// seedPayment writes one payment row for tn with the admin pool and registers
// its removal. The rlsFixture's own seed does not create payments — nothing
// before this file needed one — and the cross-tenant payment read is the case
// most worth proving, so it is seeded here rather than added to every fixture.
func (f *rlsFixture) seedPayment(ctx context.Context, t *testing.T, tn tenant) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.Admin.QueryRow(ctx, `
		INSERT INTO payments (booking_id, complex_id, amount, method, status)
		VALUES ($1, $2, 500000, 'mercadopago', 'unpaid')
		RETURNING id`, tn.BookingID, tn.ComplexID).Scan(&id)
	if err != nil {
		t.Fatalf("seeding a payment: %v", err)
	}
	t.Cleanup(func() {
		// WithoutCancel rather than a fresh background context: the cleanup has
		// to outlive the deadline the test ran under, and this is the one
		// derived from it rather than a second, unrelated one.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cleanupCancel()
		if _, err := f.Admin.Exec(cleanupCtx, `DELETE FROM payments WHERE id = $1`, id); err != nil {
			t.Errorf("cleaning up the payment: %v", err)
		}
	})
	return id
}

// ---------------------------------------------------------------------------
// Time-ordered ids
// ---------------------------------------------------------------------------

// A v4 id is inserted at a random point of the B-tree, which is the wrong shape
// for a table that grows without limit and is read by time. The four that do
// mint v7 now; the rest keep v4 deliberately, and this says which is which so a
// default flipped by accident is visible.
func TestTheGrowthTablesMintTimeOrderedIDs(t *testing.T) {
	f := newRLSFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	want := map[string]string{
		"bookings":       "uuidv7()",
		"payments":       "uuidv7()",
		"audit_log":      "uuidv7()",
		"webhook_events": "uuidv7()",
		// Bounded by how many venues exist rather than by traffic: changing
		// these would be churn with nothing on the other side of it.
		"complexes": "gen_random_uuid()",
		"courts":    "gen_random_uuid()",
		"clients":   "gen_random_uuid()",
	}

	for table, wantDefault := range want {
		var got string
		err := f.Admin.QueryRow(ctx, `
			SELECT pg_get_expr(d.adbin, d.adrelid)
			FROM pg_attrdef d
			JOIN pg_class c ON c.oid = d.adrelid
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = d.adnum
			WHERE c.relname = $1 AND a.attname = 'id'`, table).Scan(&got)
		if err != nil {
			t.Fatalf("reading %s.id's default: %v", table, err)
		}
		if got != wantDefault {
			t.Errorf("%s.id defaults to %s, want %s", table, got, wantDefault)
		}
	}
}

// The property that makes a v7 key worth the change: two rows created in order
// sort in that order by id, so an index on the primary key is an index on time.
func TestTwoBookingsMintedInOrderSortByID(t *testing.T) {
	f := newRLSFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var first, second uuid.UUID
	for i, dst := range []*uuid.UUID{&first, &second} {
		err := f.Admin.QueryRow(ctx, `
			INSERT INTO bookings (complex_id, court_id, client_id, date, start_time, duration_minutes, price)
			VALUES ($1, $2, $3, CURRENT_DATE + 90, $4, 60, 500000)
			RETURNING id`,
			f.A.ComplexID, f.A.CourtID, f.A.ClientID, []string{"08:00", "09:00"}[i]).Scan(dst)
		if err != nil {
			t.Fatalf("inserting booking %d: %v", i, err)
		}
	}

	if first.Version() != 7 || second.Version() != 7 {
		t.Fatalf("want version 7 ids; got %d and %d", first.Version(), second.Version())
	}
	if first.String() >= second.String() {
		t.Errorf("the later booking's id (%s) must sort after the earlier one's (%s)", second, first)
	}
}
