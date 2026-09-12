//go:build integration

package datatest

import (
	"context"
	"encoding/base64"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/crypto"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/stores"
)

// CredentialKeyring is the MercadoPago credential keyring every
// integration test's Stores is built with. It exists so a test that seeds a
// real (sealed) credential — Phase 22's raw-column and GetWithMPConnected
// coverage — can exercise the actual encrypt/decrypt path rather than
// hitting ErrNilKeyring on every touch, matching how cmd/api wires a real
// keyring at boot (main.go).
func CredentialKeyring(t *testing.T) *crypto.Keyring {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = 0xAB
	}
	kr, err := crypto.ParseKeyring("e2e:" + base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("building test credential keyring: %v", err)
	}
	return kr
}

// SetupTestDB opens a pool against the database named by DATABASE_URL, skipping the
// test when the variable is unset. Same shape as cmd/api/testutils_integration_test.go
// so both suites behave identically when the E2E database is not running.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing DSN: %v", err)
	}
	// The concurrency test needs two connections held at once, plus room for the
	// verification reads that run beside them.
	config.MaxConns = 8
	// Same exec mode as the API pool (cmd/api/main.go). In this mode pgx
	// infers parameter types from the Go values rather than from the prepared
	// statement, so a []byte bound to a jsonb column is sent as bytea and
	// rejected. The default statement-cache mode knows the column types and
	// hid exactly that defect in paymentstore.WebhookEvents.Insert until MercadoPago's
	// webhook simulator hit the running API.
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging database: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// Fixture is one complex's worth of real rows — owner, complex, court and client —
// on which a test can hang the bookings and payments it actually cares about.
//
// Everything it creates is reachable from ComplexID or UserID, so cleanup deletes
// exactly this test's rows instead of truncating shared tables. That keeps a failing
// test's neighbours intact and keeps the database usable for the next test without a
// global reset between every case.
type Fixture struct {
	Pool      *pgxpool.Pool
	Stores    stores.Stores
	UserID    uuid.UUID
	ComplexID uuid.UUID
	CourtID   uuid.UUID
	ClientID  uuid.UUID
}

// NewFixture creates the owner, complex, court and client a test hangs its
// own rows on, and registers the cleanup that removes them again.
func NewFixture(t *testing.T) *Fixture {
	t.Helper()

	pool := SetupTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()

	f := &Fixture{Pool: pool, Stores: stores.New(pool, stores.Config{Keys: CredentialKeyring(t)})}

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, $2, 'Owner', 'Test', '+5491100000000', 'owner', true)
		RETURNING id`,
		"owner-"+suffix+"@example.test", []byte("not-a-real-hash"),
	).Scan(&f.UserID)
	if err != nil {
		t.Fatalf("creating owner: %v", err)
	}

	// Registered before the child rows exist so it runs last (t.Cleanup is LIFO),
	// after the scoped delete below has cleared everything that references it.
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.UserID); err != nil {
			t.Errorf("deleting owner: %v", err)
		}
	})

	err = pool.QueryRow(ctx, `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, 'Test Complex', $2, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000001')
		RETURNING id`,
		f.UserID, "test-complex-"+suffix,
	).Scan(&f.ComplexID)
	if err != nil {
		t.Fatalf("creating complex: %v", err)
	}

	t.Cleanup(func() { f.deleteComplexData(t) })

	err = pool.QueryRow(ctx, `
		INSERT INTO courts (complex_id, name)
		VALUES ($1, 'Court 1')
		RETURNING id`, f.ComplexID).Scan(&f.CourtID)
	if err != nil {
		t.Fatalf("creating court: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO clients (complex_id, first_name, last_name, phone)
		VALUES ($1, 'Ana', 'Diaz', $2)
		RETURNING id`, f.ComplexID, "+54911"+suffix[:8]).Scan(&f.ClientID)
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	return f
}

// deleteComplexData removes every row this fixture's complex owns, child tables first.
//
// The deletes are explicit rather than a CASCADE from the complex because the FK graph
// is not a tree: payments.booking_id and bookings.court_id have no ON DELETE action, so
// a cascade from complexes would have to unlink them in exactly the right order to
// avoid a foreign-key violation. Naming the order here is both safer and readable.
func (f *Fixture) deleteComplexData(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statements := []string{
		`DELETE FROM failed_refunds WHERE complex_id = $1`,
		`DELETE FROM payments WHERE complex_id = $1`,
		`DELETE FROM slot_locks WHERE court_id IN (SELECT id FROM courts WHERE complex_id = $1)`,
		`DELETE FROM bookings WHERE complex_id = $1`,
		`DELETE FROM blocked_slots WHERE court_id IN (SELECT id FROM courts WHERE complex_id = $1)`,
		`DELETE FROM clients WHERE complex_id = $1`,
		`DELETE FROM court_prices WHERE court_id IN (SELECT id FROM courts WHERE complex_id = $1)`,
		`DELETE FROM courts WHERE complex_id = $1`,
		`DELETE FROM audit_log WHERE complex_id = $1`,
		`DELETE FROM complexes WHERE id = $1`,
	}

	for _, statement := range statements {
		if _, err := f.Pool.Exec(ctx, statement, f.ComplexID); err != nil {
			t.Errorf("cleanup %q: %v", statement, err)
		}
	}
}

// BookingOptions describes the booking a test wants; the zero value is a confirmed,
// deposit-paid booking a week out, which is what the refund tests need.
type BookingOptions struct {
	StartTime string
	// EndTime is how a fixture spells the length it wants, not a column: there
	// is no bookings.end_time column. It is turned into
	// DurationMinutes by minutesBetween and never stored, so "18:00" to
	// "19:30" reads as ninety minutes and "23:00" to "01:00" as two hours
	// crossing midnight.
	EndTime          string
	Status           string
	CollectionStatus string
	RefundStatus     string
	Price            int
	DepositAmount    int
	// Public marks the booking as created by an anonymous visitor (created_by NULL).
	Public bool
}

// minutesBetween is the duration a fixture's StartTime and EndTime describe.
//
// `span` — the column the exclusion constraint and SlotTaken compare — is
// generated from start plus duration_minutes, and there is no stored end at
// all. This used to be a fixed 90, so
// a fixture written as 09:00-12:00
// actually spanned 09:00-10:30 in the database and a "collision" test could
// pass or fail for reasons unrelated to what it asserted. An end before its
// start is a next-day close.
func minutesBetween(start, end string) int {
	const layout = "15:04"
	s, err := time.Parse(layout, start)
	if err != nil {
		panic("fixture StartTime is not HH:MM: " + start)
	}
	e, err := time.Parse(layout, end)
	if err != nil {
		panic("fixture EndTime is not HH:MM: " + end)
	}
	if !e.After(s) {
		e = e.Add(24 * time.Hour)
	}
	return int(e.Sub(s).Minutes())
}

func (o BookingOptions) withDefaults() BookingOptions {
	if o.StartTime == "" {
		o.StartTime = "18:00"
	}
	if o.EndTime == "" {
		o.EndTime = "19:30"
	}
	if o.Status == "" {
		o.Status = "confirmed"
	}
	if o.CollectionStatus == "" {
		o.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	}
	if o.RefundStatus == "" {
		o.RefundStatus = bookingstore.RefundStatusNone
	}
	if o.Price == 0 {
		o.Price = 500_000
	}
	if o.DepositAmount == 0 {
		o.DepositAmount = 150_000
	}
	return o
}

// NewBooking builds an unsaved Booking wired to this fixture's complex, court and
// client. Tests that exercise an insert path use this and insert it themselves.
func (f *Fixture) NewBooking(opts BookingOptions) *bookingstore.Booking {
	opts = opts.withDefaults()

	b := &bookingstore.Booking{
		ComplexID:        f.ComplexID,
		CourtID:          f.CourtID,
		ClientID:         f.ClientID,
		Date:             time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour),
		StartTime:        opts.StartTime,
		DurationMinutes:  minutesBetween(opts.StartTime, opts.EndTime),
		Price:            opts.Price,
		DepositAmount:    opts.DepositAmount,
		Status:           opts.Status,
		CollectionStatus: opts.CollectionStatus,
		RefundStatus:     opts.RefundStatus,
	}
	if !opts.Public {
		owner := f.UserID
		b.CreatedBy = &owner
	}
	return b
}

// CreateBooking inserts a booking through the real store and returns it.
func (f *Fixture) CreateBooking(t *testing.T, opts BookingOptions) *bookingstore.Booking {
	t.Helper()

	b := f.NewBooking(opts)
	if err := f.Stores.Bookings.Insert(context.Background(), b); err != nil {
		t.Fatalf("creating booking: %v", err)
	}
	return b
}

// CreatePayment inserts a payment through the real store and returns it. A nil
// mpPaymentID produces the cash-style row that carries no MercadoPago identifier.
func (f *Fixture) CreatePayment(t *testing.T, bookingID uuid.UUID, amount, serviceFee int, mpPaymentID *string) *paymentstore.Payment {
	t.Helper()

	method := "cash"
	if mpPaymentID != nil {
		method = "mercadopago"
	}

	p := &paymentstore.Payment{
		BookingID:   bookingID,
		ComplexID:   f.ComplexID,
		Amount:      amount,
		ServiceFee:  serviceFee,
		Method:      method,
		Status:      "deposit_paid",
		MPPaymentID: mpPaymentID,
	}
	if err := f.Stores.Payments.Insert(context.Background(), p); err != nil {
		t.Fatalf("creating payment: %v", err)
	}
	return p
}

// ReadPaymentState re-reads a payment's money-carrying columns straight from the
// database, bypassing any struct a store method may have mutated in memory.
func (f *Fixture) ReadPaymentState(t *testing.T, paymentID uuid.UUID) (status string, refundAmount int) {
	t.Helper()

	err := f.Pool.QueryRow(context.Background(),
		`SELECT status, COALESCE(refund_amount, 0) FROM payments WHERE id = $1`, paymentID,
	).Scan(&status, &refundAmount)
	if err != nil {
		t.Fatalf("reading payment %s: %v", paymentID, err)
	}
	return status, refundAmount
}

// ReadBookingState re-reads a booking's status columns straight from the database.
func (f *Fixture) ReadBookingState(t *testing.T, bookingID uuid.UUID) (status, collectionStatus, refundStatus string) {
	t.Helper()

	err := f.Pool.QueryRow(context.Background(),
		`SELECT status, collection_status, refund_status FROM bookings WHERE id = $1`, bookingID,
	).Scan(&status, &collectionStatus, &refundStatus)
	if err != nil {
		t.Fatalf("reading booking %s: %v", bookingID, err)
	}
	return status, collectionStatus, refundStatus
}

// BackdateBookingCreatedAt moves a booking's created_at into the past.
//
// It is how a test produces the stale pending booking the carve-out is about
// without waiting out a real payment expiry. Only updated_at carries a trigger on
// this table, so created_at can simply be written.
func (f *Fixture) BackdateBookingCreatedAt(t *testing.T, id uuid.UUID, age time.Duration) {
	t.Helper()

	tag, err := f.Pool.Exec(context.Background(),
		`UPDATE bookings SET created_at = NOW() - make_interval(secs => $2) WHERE id = $1`,
		id, age.Seconds())
	if err != nil {
		t.Fatalf("backdating booking %s: %v", id, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("backdating booking %s: want 1 row affected, got %d", id, tag.RowsAffected())
	}
}

// ConfirmBooking runs a booking through the real payment-confirmation path — the
// one a MercadoPago webhook takes when it confirms a paid booking — and returns
// whatever the store decided. models is passed in so a test can confirm through a
// differently configured set of stores.
func (f *Fixture) ConfirmBooking(models stores.Stores, b *bookingstore.Booking) error {
	b.Status = "confirmed"
	b.CollectionStatus = bookingstore.CollectionStatusDepositPaid

	payment := &paymentstore.Payment{
		BookingID:  b.ID,
		ComplexID:  f.ComplexID,
		Amount:     b.DepositAmount,
		ServiceFee: 10_500,
		Method:     "mercadopago",
		Status:     "deposit_paid",
	}
	return models.Payments.InsertAndConfirmBooking(context.Background(), payment, b)
}

// CountBookings counts this fixture's bookings on its court in the given status.
func (f *Fixture) CountBookings(t *testing.T, status string) int {
	t.Helper()

	var n int
	err := f.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM bookings WHERE court_id = $1 AND status = $2::booking_status`,
		f.CourtID, status,
	).Scan(&n)
	if err != nil {
		t.Fatalf("counting %s bookings: %v", status, err)
	}
	return n
}

// BackdateFailedRefundUpdatedAt moves a failed refund's updated_at into the past.
//
// failed_refunds carries a BEFORE UPDATE trigger that overwrites updated_at with NOW()
// on every write, so an ordinary UPDATE cannot age a row. The trigger is disabled for
// the duration of this single statement — inside a transaction, so it is restored even
// if the UPDATE fails.
func (f *Fixture) BackdateFailedRefundUpdatedAt(t *testing.T, id uuid.UUID, age time.Duration) {
	t.Helper()

	ctx := context.Background()

	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin backdate tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `ALTER TABLE failed_refunds DISABLE TRIGGER set_updated_at`); err != nil {
		t.Fatalf("disabling updated_at trigger: %v", err)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE failed_refunds SET updated_at = NOW() - $2::interval WHERE id = $1`,
		id, age.String())
	if err != nil {
		t.Fatalf("backdating failed refund: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("backdating failed refund %s: want 1 row affected, got %d", id, tag.RowsAffected())
	}

	if _, err := tx.Exec(ctx, `ALTER TABLE failed_refunds ENABLE TRIGGER set_updated_at`); err != nil {
		t.Fatalf("re-enabling updated_at trigger: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit backdate tx: %v", err)
	}
}

// BlockCourtDay takes the very lock InsertSafe and the confirmation guard take,
// on its own connection, and holds it until the returned function is called.
//
// It is the starting gate: transactions queue behind it in the order they ask,
// so a test can replay a chosen interleaving of two genuinely concurrent
// writers instead of hoping the scheduler produces the interesting one. It
// lives here because the bookings store's race tests and the courts store's
// blocked-slot race tests queue behind the same lock.
func (f *Fixture) BlockCourtDay(t *testing.T, date time.Time) (release func()) {
	t.Helper()

	ctx := context.Background()
	conn, err := f.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring the gate connection: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		conn.Release()
		t.Fatalf("beginning the gate transaction: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1 || $2))`,
		f.CourtID.String(), date.Format("2006-01-02"),
	); err != nil {
		_ = tx.Rollback(ctx)
		conn.Release()
		t.Fatalf("taking the gate lock: %v", err)
	}

	var once sync.Once
	release = func() {
		once.Do(func() {
			_ = tx.Rollback(ctx)
			conn.Release()
		})
	}
	t.Cleanup(release)
	return release
}

// WaitForLockWaiters blocks until exactly want transactions are queued on an
// advisory lock in this database, so the test knows a goroutine has really
// reached the gate rather than merely been started.
//
// A writer that never appears in that queue is itself the failure — it is not
// serialized against anything — so the timeout is reported and the caller
// carries on to its own assertions rather than stopping here.
func (f *Fixture) WaitForLockWaiters(t *testing.T, want int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting int
		err := f.Pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM pg_locks
			WHERE locktype = 'advisory' AND NOT granted
			  AND database = (SELECT oid FROM pg_database WHERE datname = current_database())`,
		).Scan(&waiting)
		if err != nil {
			t.Fatalf("reading lock waiters: %v", err)
		}
		if waiting == want {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("both the insert and the confirmation must queue on the court lock; want %d waiting, got %d — a writer that never takes the lock is serialized against nothing", want, waiting)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// MarkStatus moves a booking to a terminal status the way the owner-facing
// update does, through a plain UPDATE that the status-reversal trigger sees.
func (f *Fixture) MarkStatus(t *testing.T, id uuid.UUID, status string) {
	t.Helper()

	_, err := f.Pool.Exec(context.Background(),
		`UPDATE bookings SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		t.Fatalf("marking booking %s: %v", status, err)
	}
}
