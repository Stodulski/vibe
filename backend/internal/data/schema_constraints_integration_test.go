//go:build integration

package data_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// SQLSTATEs the constraints in db/migrations/001_init.sql raise. checkViolation ("23514")
// is declared in midnight_integration_test.go and reused here.
const (
	notNullViolation    = "23502"
	foreignKeyViolation = "23503"
	uniqueViolation     = "23505"
	exclusionViolation  = "23P01"
)

// Constraint names, spelled once. Every assertion below names one of these
// rather than settling for a SQLSTATE, because a SQLSTATE proves only that
// something refused the row. bookings alone now carries five CHECK constraints —
// duration_minutes > 0, price >= 0, deposit_amount >= 0, bookings_time_range_check
// and bookings_deposit_within_price — plus a trigger that raises 23514 as well,
// so a test that accepted "some 23514" would keep passing against a row rejected
// for entirely the wrong reason. That is the failure mode these constants exist
// to prevent.
const (
	paymentsRefundWithinAmountPaid   = "payments_refund_within_amount_paid"
	bookingsDepositWithinPrice       = "bookings_deposit_within_price"
	complexesCancellationHoursRange  = "complexes_cancellation_hours_range"
	clientsCountersNonNegative       = "clients_counters_non_negative"
	schedulesOpenWindowNotEmpty      = "complex_schedules_open_window_not_empty"
	refreshTokensTokenHashKey        = "refresh_tokens_token_hash_key"
	courtsActiveNameUniqueName       = "courts_active_name_unique"
	courtPricesNoOverlappingName     = "court_prices_no_overlapping_rule"
	bookingsCourtInSameComplex       = "bookings_court_in_same_complex"
	bookingsClientInSameComplex      = "bookings_client_in_same_complex"
	paymentsBookingInSameComplex     = "payments_booking_in_same_complex"
	statusNoTerminalReentry          = "bookings_status_no_terminal_reentry"
	collectionStatusNoReturnToUnpaid = "bookings_collection_status_no_return_to_unpaid"
	refundNeedsCollectedMoney        = "bookings_refund_needs_collected_money"
	bookingsDurationMinutesPermitted = "bookings_duration_minutes_permitted"
)

// refusedByConstraint reports whether err is the named constraint refusing a
// write with the given SQLSTATE, and describes the mismatch when it is not.
//
// It asserts the constraint NAME, not just the SQLSTATE: the same
// name-not-code discipline, extended to the unique, foreign-key, exclusion and
// trigger refusals the schema carries. The trigger cases work because the
// RAISE statements pass USING CONSTRAINT, which populates the same error field
// PostgreSQL fills in for a real constraint and pgx exposes as ConstraintName.
// checkViolation is the SQLSTATE PostgreSQL raises for a failed CHECK.
const checkViolation = "23514"

func refusedByConstraint(t *testing.T, err error, sqlState, constraint string) {
	t.Helper()

	if err == nil {
		t.Fatalf("the write must be refused by %s; it was accepted", constraint)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("the write must be refused by %s; got a non-PostgreSQL error: %v", constraint, err)
	}
	if pgErr.Code != sqlState || pgErr.ConstraintName != constraint {
		t.Fatalf("the write must be refused by %s (%s); it was refused by %q (%s): %s",
			constraint, sqlState, pgErr.ConstraintName, pgErr.Code, pgErr.Message)
	}
}

// accepted fails when a write the schema is supposed to allow was refused.
//
// Half of a constraint's correctness is what it lets through. Every section
// below asserts at least one legal neighbour of the row it rejects, because a
// constraint that refuses everything also refuses every reported bad row and
// would pass a test suite that only ever checked for failure.
func accepted(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s must be accepted; it was refused: %v", what, err)
	}
}

// exec runs one statement against the fixture's pool and returns the error.
func exec(f *datatest.Fixture, query string, args ...any) error {
	_, err := f.DB.Exec(context.Background(), query, args...)
	return err
}

// insertBooking writes a booking row directly, bypassing bookingstore.Store entirely,
// so the assertions below prove the storage layer rather than one store method.
func insertBookingRow(f *datatest.Fixture, complexID, courtID, clientID uuid.UUID, price, deposit int) error {
	return exec(f, `
		INSERT INTO bookings
			(complex_id, court_id, client_id, date, start_time,
			 duration_minutes, price, deposit_amount, status, collection_status)
		VALUES ($1, $2, $3, CURRENT_DATE + 21, '10:00',
			 90, $4, $5, 'confirmed', 'deposit_paid')`,
		complexID, courtID, clientID, price, deposit)
}

// ==================== MONEY RELATIONSHIPS ====================

// A payment of 1000 accepted a refund of 50000. refund_amount carried only
// CHECK (refund_amount >= 0), so the column knew how to be non-negative and
// nothing at all about the payment it belonged to.
func TestARefundCannotExceedWhatThePaymentCollected(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{})
	p := f.CreatePayment(t, b.ID, 1000, 70, nil)

	err := exec(f, `UPDATE payments SET refund_amount = 50000 WHERE id = $1`, p.ID)
	refusedByConstraint(t, err, checkViolation, paymentsRefundWithinAmountPaid)

	if _, refunded := f.ReadPaymentState(t, p.ID); refunded != 0 {
		t.Errorf("a refused refund must leave the payment untouched; refund_amount is %d", refunded)
	}

	// The bound is amount + service_fee, not amount. claimable() in refunds.go
	// computes totalPaid that way because a full refund returns the service fee
	// too, so a cap of `amount` would refuse refunds the product intends to make.
	accepted(t, exec(f, `UPDATE payments SET refund_amount = 1070 WHERE id = $1`, p.ID),
		"a full refund of amount + service_fee")
	accepted(t, exec(f, `UPDATE payments SET refund_amount = 0 WHERE id = $1`, p.ID),
		"resetting refund_amount to zero")
}

// A booking priced 1000 accepted a deposit of 999999. That is not a cosmetic
// number: ConfirmPayment in internal/bookings/actions.go adds DepositAmount to
// the cash it receives and calls the booking fully_paid when the sum clears the
// price, so an inflated deposit settles a booking nobody paid for.
func TestADepositCannotExceedTheBookingPrice(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{Price: 1000, DepositAmount: 300})

	err := exec(f, `UPDATE bookings SET deposit_amount = 999999 WHERE id = $1`, b.ID)
	refusedByConstraint(t, err, checkViolation, bookingsDepositWithinPrice)

	err = insertBookingRow(f, f.ComplexID, f.CourtID, f.ClientID, 1000, 999999)
	refusedByConstraint(t, err, checkViolation, bookingsDepositWithinPrice)

	// A deposit equal to the price is a booking paid in full, which is ordinary.
	accepted(t, exec(f, `UPDATE bookings SET deposit_amount = 1000 WHERE id = $1`, b.ID),
		"a deposit equal to the price")
}

// refund_amount was the only nullable money column in the schema, and its two
// readers disagreed about what NULL meant: PaymentSummaryByMethod COALESCEs it
// to 0, while PaymentDetails scans it into a Go int and fails the whole monthly
// export. Same row, two answers, one of them a 500.
func TestRefundAmountCannotBeNull(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{})
	p := f.CreatePayment(t, b.ID, 1000, 70, nil)

	err := exec(f, `UPDATE payments SET refund_amount = NULL WHERE id = $1`, p.ID)
	if err == nil {
		t.Fatal("refund_amount = NULL must be refused; it was accepted")
	}

	// NOT NULL raises 23502 and names the column rather than a constraint, so
	// this is the one assertion in the file keyed on ColumnName. It is still a
	// name, not just a code: payments has other NOT NULL columns.
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != notNullViolation || pgErr.ColumnName != "refund_amount" {
		t.Fatalf("the refusal must be a NOT NULL violation on refund_amount (%s); got %v",
			notNullViolation, err)
	}

	// Every payment ever inserted must now read as a number rather than a NULL,
	// which is the property PaymentDetails depends on.
	var nulls int
	if scanErr := f.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM payments WHERE refund_amount IS NULL`).Scan(&nulls); scanErr != nil {
		t.Fatalf("counting null refunds: %v", scanErr)
	}
	if nulls != 0 {
		t.Errorf("no payment may carry a NULL refund_amount; %d do", nulls)
	}
}

// ==================== UNARY DOMAIN RULES ====================

// bookings.duration_minutes was ANY positive number of a court's own fixed
// slot length; now that a booking's length is chosen per request, only 60, 90
// and 120 are real. This constraint was once added NOT VALID
// because rows created under the old consecutive-slot model — 2 x 90 = 180
// minutes, for example — are real historical data the migration must not
// refuse to run over. NOT VALID skips checking existing rows but still checks
// every INSERT and every UPDATE that touches the column, which is what this
// test is about: a constraint an operator cannot see enforced anywhere would
// not be worth adding.
func TestABookingsDurationMustBeOneTheGridSells(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{})

	// 0 is excluded here: it is refused by the pre-existing
	// bookings_duration_minutes_check (> 0) before this constraint is even
	// reached, and that is TestRefundAmountCannotBeNull's neighbourhood, not
	// this constraint's.
	for _, minutes := range []int{45, 61, 180} {
		t.Run(fmt.Sprintf("duration=%d", minutes), func(t *testing.T) {
			err := exec(f, `UPDATE bookings SET duration_minutes = $2 WHERE id = $1`, b.ID, minutes)
			refusedByConstraint(t, err, checkViolation, bookingsDurationMinutesPermitted)
		})
	}

	for _, minutes := range []int{60, 90, 120} {
		accepted(t, exec(f, `UPDATE bookings SET duration_minutes = $2 WHERE id = $1`, b.ID, minutes),
			fmt.Sprintf("a %d-minute booking", minutes))
	}
}

// cancellation_hours = -500 reads to every comparison as "always cancellable,
// refund always due". The bounds mirror internal/complexes/handlers.go exactly.
//
// Zero is on the refused list (complexes_cancellation_hours_range), and it is the value this
// test exists for. Negative was always nonsense; zero was legal, reachable and
// harmful: pricing.CanRefund short-circuits to true on cancellationHours <= 0
// and pricing.LinkLive ORs that in, so a complex at zero hands out booking
// links that never expire. The handler refuses it too — that is what
// TestCreateRejectsAZeroCancellationWindow covers — but a handler is one door.
// This test is here because zero must be unreachable through every other one:
// a direct UPDATE, a raw INSERT, a future endpoint, a fixture.
func TestACancellationWindowMustBeAWindow(t *testing.T) {
	f := datatest.Isolated(t)

	for _, hours := range []int{-500, -1, 0, 169} {
		t.Run(fmt.Sprintf("update/hours=%d", hours), func(t *testing.T) {
			err := exec(f, `UPDATE complexes SET cancellation_hours = $2 WHERE id = $1`, f.ComplexID, hours)
			refusedByConstraint(t, err, checkViolation, complexesCancellationHoursRange)
		})
	}

	// The INSERT path, separately. An UPDATE-only assertion would pass against a
	// constraint that somehow only fired on update, and the shape this migration
	// is defending against is precisely a *new* write path — a fresh endpoint or
	// a seed script naming the column — rather than an edit to an existing row.
	// The Create handler wrote 0 exactly this way: `CancellationHours int` takes
	// Go's zero when the JSON field is absent, and InsertComplex names the column
	// so the DEFAULT 24 never applies.
	for _, hours := range []int{-1, 0, 169} {
		t.Run(fmt.Sprintf("insert/hours=%d", hours), func(t *testing.T) {
			// f.UserID already owns f.ComplexID and complexes_owner_id_key
			// permits only one live complex per owner, so this insert needs
			// its own owner — reusing f.UserID would risk failing on the
			// unique index rather than proving anything about
			// cancellation_hours.
			ownerID := f.InsertOwner(t)
			err := exec(f, `
				INSERT INTO complexes (owner_id, name, slug, address, city, province, phone, cancellation_hours)
				VALUES ($1, 'Zero Window', $2, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000009', $3)`,
				ownerID, fmt.Sprintf("zero-window-%d-%s", hours, uuid.NewString()), hours)
			refusedByConstraint(t, err, checkViolation, complexesCancellationHoursRange)
		})
	}

	// One hour is the floor, not a value the constraint tolerates by accident:
	// asserting it is accepted is what stops a stricter bound (>= 2, or > 1)
	// from passing this test while quietly refusing the smallest legal window.
	for _, hours := range []int{1, 24, 168} {
		accepted(t, exec(f, `UPDATE complexes SET cancellation_hours = $2 WHERE id = $1`, f.ComplexID, hours),
			fmt.Sprintf("a %d-hour cancellation window", hours))
	}

	// The column DEFAULT is 24, so a row that never names cancellation_hours is
	// still legal. That is the path every existing fixture takes, and a bound
	// that broke it would break the suite rather than the product.
	//
	// The cleanup is registered before the insert rather than after, because
	// accepted() ends in t.Fatalf and a cleanup registered afterwards would
	// never run on the failing path — leaving a stray complex behind for every
	// later test in a suite that shares one database.
	// complexes_owner_id_key allows only one live complex per owner, and
	// f.UserID already owns f.ComplexID, so this insert needs its own owner
	// or it would hit 23505 rather than prove anything about
	// cancellation_hours.
	defaultOwnerID := f.InsertOwner(t)
	defaultSlug := "default-window-" + uuid.NewString()
	t.Cleanup(func() {
		if err := exec(f, `DELETE FROM complexes WHERE slug = $1`, defaultSlug); err != nil {
			t.Errorf("cleaning up %s: %v", defaultSlug, err)
		}
	})
	accepted(t, exec(f, `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, 'Default Window', $2, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000009')`,
		defaultOwnerID, defaultSlug),
		"a complex that lets cancellation_hours take its DEFAULT of 24")
}

// A negative counter is not a smaller number, it is evidence that an increment
// ran against the wrong row — and it silently inverts the no-show history an
// owner uses to decide who to refuse.
func TestClientCountersCannotGoNegative(t *testing.T) {
	f := datatest.Isolated(t)

	err := exec(f, `UPDATE clients SET total_bookings = -10 WHERE id = $1`, f.ClientID)
	refusedByConstraint(t, err, checkViolation, clientsCountersNonNegative)

	err = exec(f, `UPDATE clients SET no_shows = -10 WHERE id = $1`, f.ClientID)
	refusedByConstraint(t, err, checkViolation, clientsCountersNonNegative)

	accepted(t, exec(f, `UPDATE clients SET total_bookings = 0, no_shows = 0 WHERE id = $1`, f.ClientID),
		"counters at zero")
}

// ==================== complex_schedules ====================

// The schema review asked for CHECK (open_time < close_time) here. That would
// have deleted a shipped feature: internal/bookings/public.go adds 1440 to
// close_time when it does not exceed open_time, which is how this table spells
// "closes after midnight". What is genuinely wrong is a zero-length window on an
// open day, because the same wrap turns it into a 24-hour one.
func TestAnOpenDayCannotHaveAZeroLengthWindow(t *testing.T) {
	f := datatest.Isolated(t)

	err := exec(f, `
		INSERT INTO complex_schedules (complex_id, day, open_time, close_time, is_closed)
		VALUES ($1, 'monday', '10:00', '10:00', false)`, f.ComplexID)
	refusedByConstraint(t, err, checkViolation, schedulesOpenWindowNotEmpty)

	// Both shapes a naive open_time < close_time would have rejected.
	accepted(t, exec(f, `
		INSERT INTO complex_schedules (complex_id, day, open_time, close_time, is_closed)
		VALUES ($1, 'tuesday', '20:00', '02:00', false)`, f.ComplexID),
		"a venue that closes after midnight (20:00-02:00)")

	accepted(t, exec(f, `
		INSERT INTO complex_schedules (complex_id, day, open_time, close_time, is_closed)
		VALUES ($1, 'sunday', '00:00', '00:00', true)`, f.ComplexID),
		"a closed day stored as 00:00/00:00")
}

// ==================== UNIQUENESS ====================

// GetRefreshTokenByHash is a sqlc :one query. A second row with the same hash
// makes the session a refresh returns depend on which row the plan reaches
// first — two users, one hash, arbitrary identity. Both sibling token tables
// have carried UNIQUE since the initial schema; this one carried a plain index.
func TestARefreshTokenHashIsUnique(t *testing.T) {
	f := datatest.Isolated(t)
	hash := []byte("hash-" + uuid.NewString())

	accepted(t, exec(f, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '7 days')`, f.UserID, hash),
		"the first token with this hash")

	err := exec(f, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '7 days')`, f.UserID, hash)
	refusedByConstraint(t, err, uniqueViolation, refreshTokensTokenHashKey)

	accepted(t, exec(f, `DELETE FROM refresh_tokens WHERE user_id = $1`, f.UserID),
		"cleaning up this fixture's tokens")
}

// Two courts with one name is the cheapest-looking finding on the list and the
// most expensive to leave: every confirmation, reminder and export line
// identifies a court by its name, so the ambiguity ends up in a year of records
// rather than in one table.
func TestACourtNameIsUniqueWithinItsComplex(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// Through the real store, so the domain error the handler will see is proven
	// too rather than only the constraint underneath it.
	err := f.Stores.Courts.Insert(f.Scoped(ctx), &courtstore.Court{
		ComplexID: f.ComplexID, Name: "Court 1", Sport: "padel", CourtType: "indoor",
	})
	if !errors.Is(err, courtstore.ErrDuplicateCourtName) {
		t.Fatalf("a second live court called %q must return ErrDuplicateCourtName; got %v", "Court 1", err)
	}

	// And directly, so a future insert path that skips courtstore.Store is covered.
	err = exec(f, `INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 1')`, f.ComplexID)
	refusedByConstraint(t, err, uniqueViolation, courtsActiveNameUniqueName)

	// The uniqueness is per complex. Another tenant's "Court 1" is a different
	// court, and the constraint must not reach across the tenant boundary.
	other := datatest.Isolated(t)
	accepted(t, exec(other, `INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 1 bis')`, other.ComplexID),
		"a court name in a different complex")

	// SoftDeleteCourt sets deleted_at rather than removing the row, so the index
	// is partial: an owner who deletes a court and adds a new one with the same
	// name is doing something ordinary.
	accepted(t, exec(f, `UPDATE courts SET deleted_at = NOW() WHERE id = $1`, f.CourtID),
		"soft-deleting the original court")
	accepted(t, exec(f, `INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 1')`, f.ComplexID),
		"reusing the name of a soft-deleted court")
}

// ==================== OVERLAPPING PRICE RULES ====================

// findPrice returns the first rule whose window contains the slot, and "first"
// is whichever row the query happened to emit. GetCourtPrices and
// GetPricesByCourtIDs sort differently and neither sort is total across
// overlapping rules, so the price the availability grid shows and the price the
// booking handler charges come from two different sorts of the same ambiguity.
func TestTwoPriceRulesCannotCoverTheSameMinute(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	base := &courtstore.CourtPrice{CourtID: f.CourtID, Price: 10_000, DayType: "monday", TimeFrom: "08:00", TimeTo: "23:00"}
	accepted(t, f.Stores.Courts.InsertPrice(ctx, base), "the first price rule for monday")

	// An identical rule — the case the review reported.
	dup := &courtstore.CourtPrice{CourtID: f.CourtID, Price: 99_000, DayType: "monday", TimeFrom: "08:00", TimeTo: "23:00"}
	if err := f.Stores.Courts.InsertPrice(ctx, dup); !errors.Is(err, courtstore.ErrOverlappingPriceRule) {
		t.Fatalf("an identical price rule must return ErrOverlappingPriceRule; got %v", err)
	}

	// A partial overlap, which a composite UNIQUE would have let through while
	// leaving the non-determinism entirely intact for every minute from 10:00.
	err := exec(f, `
		INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
		VALUES ($1, 15000, 'monday', '10:00', '14:00')`, f.CourtID)
	refusedByConstraint(t, err, exclusionViolation, courtPricesNoOverlappingName)

	// Half-open bounds. Adjacent rules sharing an endpoint are the normal way to
	// price a peak window and must stay legal.
	accepted(t, exec(f, `
		INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
		VALUES ($1, 10000, 'friday', '08:00', '12:00'), ($1, 15000, 'friday', '12:00', '23:00')`, f.CourtID),
		"adjacent price rules 08:00-12:00 and 12:00-23:00")

	// The same window on another weekday, and on another court, are different
	// rules — the constraint is scoped to (court, weekday).
	accepted(t, exec(f, `
		INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
		VALUES ($1, 12000, 'saturday', '08:00', '23:00')`, f.CourtID),
		"the same window on a different weekday")

	other := datatest.Isolated(t)
	accepted(t, exec(other, `
		INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
		VALUES ($1, 12000, 'monday', '08:00', '23:00')`, other.CourtID),
		"the same window on a different court")
}

// ==================== CROSS-TENANT COMPOSITE KEYS ====================

// The one that moves money between tenants. Every revenue query in reports.go
// scopes by payments.complex_id, so a payment whose complex_id differs from its
// own booking's complex_id lands in one owner's monthly total while its booking
// sits in another's — and neither report looks wrong.
func TestAPaymentCannotBelongToADifferentComplexThanItsBooking(t *testing.T) {
	f := datatest.Shared(t)
	other := datatest.Shared(t)

	b := f.CreateBooking(t, datatest.BookingOptions{})
	p := f.CreatePayment(t, b.ID, 1000, 70, nil)

	err := exec(f, `UPDATE payments SET complex_id = $2 WHERE id = $1`, p.ID, other.ComplexID)
	refusedByConstraint(t, err, foreignKeyViolation, paymentsBookingInSameComplex)

	err = exec(f, `
		INSERT INTO payments (booking_id, complex_id, amount, method, status)
		VALUES ($1, $2, 500, 'cash', 'deposit_paid')`, b.ID, other.ComplexID)
	refusedByConstraint(t, err, foreignKeyViolation, paymentsBookingInSameComplex)

	// The revenue query the misrouted row would have polluted still sees nothing
	// belonging to the other tenant.
	var stolen int
	if scanErr := f.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM payments WHERE complex_id = $1`, other.ComplexID).Scan(&stolen); scanErr != nil {
		t.Fatalf("counting the other tenant's payments: %v", scanErr)
	}
	if stolen != 0 {
		t.Errorf("no payment may have been routed to the other tenant; %d were", stolen)
	}
}

// bookings.court_id and client_id were foreign keys to courts and clients with
// no relationship at all to bookings.complex_id, so a row naming one tenant's
// complex and another tenant's court was not a bug to be found in a handler — it
// was a shape the schema endorsed.
func TestABookingCannotMixTenants(t *testing.T) {
	f := datatest.Shared(t)
	other := datatest.Shared(t)

	b := f.CreateBooking(t, datatest.BookingOptions{})

	err := exec(f, `UPDATE bookings SET court_id = $2 WHERE id = $1`, b.ID, other.CourtID)
	refusedByConstraint(t, err, foreignKeyViolation, bookingsCourtInSameComplex)

	err = exec(f, `UPDATE bookings SET client_id = $2 WHERE id = $1`, b.ID, other.ClientID)
	refusedByConstraint(t, err, foreignKeyViolation, bookingsClientInSameComplex)

	err = insertBookingRow(f, f.ComplexID, other.CourtID, f.ClientID, 1000, 300)
	refusedByConstraint(t, err, foreignKeyViolation, bookingsCourtInSameComplex)

	err = insertBookingRow(f, f.ComplexID, f.CourtID, other.ClientID, 1000, 300)
	refusedByConstraint(t, err, foreignKeyViolation, bookingsClientInSameComplex)

	// The booking that belongs entirely to one tenant is still ordinary.
	accepted(t, insertBookingRow(f, f.ComplexID, f.CourtID, f.ClientID, 1000, 300),
		"a booking whose complex, court and client are all the same tenant's")
}

// ==================== STATUS TRANSITIONS ====================

// The worst row in the review. The client's money has been returned and the
// booking now claims nobody paid and the court is theirs; idx_bookings_no_double
// catches it only if somebody else already bought that exact minute.
func TestACancelledRefundedBookingCannotBeResoldToItsOwnClient(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{})

	accepted(t, exec(f,
		`UPDATE bookings SET status = 'cancelled', refund_status = 'full' WHERE id = $1`, b.ID),
		"cancelling and refunding the booking")

	err := exec(f,
		`UPDATE bookings SET status = 'confirmed', collection_status = 'unpaid' WHERE id = $1`, b.ID)
	refusedByConstraint(t, err, checkViolation, statusNoTerminalReentry)

	// Each half is refused on its own too, so neither can be walked back in two
	// statements instead of one.
	err = exec(f, `UPDATE bookings SET status = 'confirmed' WHERE id = $1`, b.ID)
	refusedByConstraint(t, err, checkViolation, statusNoTerminalReentry)

	// The BEFORE ROW trigger runs ahead of constraint checking, so this write is
	// refused by the trigger's rule rather than by
	// bookings_refund_needs_collected_money, which the same row would also
	// violate. Asserting the name is what pins that order.
	err = exec(f, `UPDATE bookings SET collection_status = 'unpaid' WHERE id = $1`, b.ID)
	refusedByConstraint(t, err, checkViolation, collectionStatusNoReturnToUnpaid)

	status, collectionStatus, refundStatus := f.ReadBookingState(t, b.ID)
	if status != "cancelled" || refundStatus != bookingstore.RefundStatusFull {
		t.Errorf("a refused reversal must leave the booking cancelled+fully refunded; it reads %s+%s",
			status, refundStatus)
	}
	// And the deposit the fixture collected is still on the row: the payment_status split
	// put the refund on its own axis so it stops overwriting this one.
	if collectionStatus != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("the refund must not have erased what was collected; it reads %s", collectionStatus)
	}
}

// A refund is money going back, so there has to be money that came in. The
// single payment_status enum could not say this at all — 'refunded' and
// 'unpaid' were two values of one column and nothing related them — and it is
// what makes the rewritten trigger rule exactly as strong as the one it
// replaced: without it a booking could sit at (unpaid, full) and be written to
// (unpaid, none), which the old enum refused as refunded -> unpaid and a rule
// on the collection axis alone would wave through.
func TestARefundCannotExistWithoutMoneyHavingBeenCollected(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{CollectionStatus: bookingstore.CollectionStatusUnpaid})

	for _, refund := range []string{bookingstore.RefundStatusPending, bookingstore.RefundStatusPartial, bookingstore.RefundStatusFull} {
		err := exec(f, `UPDATE bookings SET refund_status = $2 WHERE id = $1`, b.ID, refund)
		refusedByConstraint(t, err, checkViolation, refundNeedsCollectedMoney)
	}

	if _, collectionStatus, refundStatus := f.ReadBookingState(t, b.ID); refundStatus != bookingstore.RefundStatusNone {
		t.Errorf("the booking must still read (%s, none); it reads (%s, %s)",
			bookingstore.CollectionStatusUnpaid, collectionStatus, refundStatus)
	}
}

// completed -> pending resurrects a finished booking into the working set of
// the expired-pending sweep (GetExpiredPendingEnriched), which cancels pending
// bookings on a timer.
func TestAFinishedBookingCannotBeResurrected(t *testing.T) {
	f := datatest.Isolated(t)

	for _, terminal := range []string{"cancelled", "completed", "no_show"} {
		for _, live := range []string{"pending", "confirmed"} {
			t.Run(terminal+"->"+live, func(t *testing.T) {
				b := f.CreateBooking(t, datatest.BookingOptions{StartTime: "08:00", EndTime: "09:30"})
				accepted(t, exec(f, `UPDATE bookings SET status = $2::booking_status WHERE id = $1`, b.ID, terminal),
					"reaching "+terminal)

				err := exec(f, `UPDATE bookings SET status = $2::booking_status WHERE id = $1`, b.ID, live)
				refusedByConstraint(t, err, checkViolation, statusNoTerminalReentry)

				if status, _, _ := f.ReadBookingState(t, b.ID); status != terminal {
					t.Errorf("the booking must still read %s; it reads %s", terminal, status)
				}
				accepted(t, exec(f, `DELETE FROM bookings WHERE id = $1`, b.ID), "cleaning up")
			})
		}
	}
}

// The other half of the trigger's correctness: every transition the money paths
// actually perform must still go through. These are not hypothetical — each one
// is a line in internal/payments or internal/bookings, and a trigger that
// refused any of them would break the product to protect it.
func TestTheTransitionsTheMoneyPathsPerformStillGoThrough(t *testing.T) {
	f := datatest.Isolated(t)

	tests := []struct {
		name                       string
		from, fromCollection       string
		fromRefund                 string
		to, toCollection, toRefund string
		why                        string
	}{
		{"pending to confirmed on payment", "pending", "unpaid", "none", "confirmed", "deposit_paid", "none",
			"internal/payments/process.go, the MercadoPago approval webhook"},
		{"confirmed to cancelled", "confirmed", "deposit_paid", "none", "cancelled", "deposit_paid", "none",
			"internal/bookings/actions.go, an owner cancelling"},
		{"pending expired by the cron", "pending", "unpaid", "none", "cancelled", "unpaid", "none",
			"cmd/api/cron.go, the unpaid-booking reaper"},
		{"a deposit refunded outright", "confirmed", "deposit_paid", "none", "cancelled", "deposit_paid", "full",
			"internal/payments/refund.go, which skips the pending refund state entirely"},
		{"a full payment refunded outright", "confirmed", "fully_paid", "none", "cancelled", "fully_paid", "full",
			"internal/payments/refund.go on a fully paid booking"},
		{"refund on a played booking", "completed", "fully_paid", "none", "completed", "fully_paid", "full",
			"internal/payments/refund.go, which must not cancel a completed booking"},
		{"cash after a chargeback", "completed", "fully_paid", "full", "completed", "fully_paid", "none",
			"internal/bookings/actions.go ConfirmPayment on a charged-back booking; the refund axis " +
				"is deliberately not monotonic, and this is the write that needs it"},
		{"completed to no_show", "completed", "fully_paid", "none", "no_show", "fully_paid", "none",
			"the one terminal-to-terminal move the handler matrix allows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := f.CreateBooking(t, datatest.BookingOptions{
				StartTime: "08:00", EndTime: "09:30",
				Status: bookingstore.BookingStatus(tt.from), CollectionStatus: bookingstore.CollectionStatus(tt.fromCollection), RefundStatus: bookingstore.RefundStatus(tt.fromRefund),
			})

			err := exec(f, `
				UPDATE bookings SET status = $2::booking_status, collection_status = $3, refund_status = $4
				WHERE id = $1`, b.ID, tt.to, tt.toCollection, tt.toRefund)
			accepted(t, err, fmt.Sprintf("%s+%s+%s -> %s+%s+%s (%s)",
				tt.from, tt.fromCollection, tt.fromRefund, tt.to, tt.toCollection, tt.toRefund, tt.why))

			accepted(t, exec(f, `DELETE FROM bookings WHERE id = $1`, b.ID), "cleaning up")
		})
	}
}

// A write that does not touch either status column must not pay for the trigger
// at all: the WHEN clause keeps it out of the way of every notes edit, reminder
// flag and deposit adjustment the application performs.
func TestTheTriggerIgnoresWritesThatDoNotChangeStatus(t *testing.T) {
	f := datatest.Isolated(t)
	b := f.CreateBooking(t, datatest.BookingOptions{})

	accepted(t, exec(f, `UPDATE bookings SET status = 'cancelled' WHERE id = $1`, b.ID),
		"cancelling the booking")

	accepted(t, exec(f, `UPDATE bookings SET notes = 'cancelled by owner' WHERE id = $1`, b.ID),
		"editing the notes of a cancelled booking")
	accepted(t, exec(f, `UPDATE bookings SET reminder_sent_2h = true WHERE id = $1`, b.ID),
		"marking a reminder sent on a cancelled booking")
	accepted(t, exec(f, `UPDATE bookings SET status = 'cancelled' WHERE id = $1`, b.ID),
		"rewriting the same status")
}
