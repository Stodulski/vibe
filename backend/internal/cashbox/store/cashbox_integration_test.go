//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// checkViolation and uniqueViolation are the two SQLSTATEs this file's
// constraint assertions name. Spelled once so a call site reads as "which
// constraint" rather than "which magic string" — same discipline as
// internal/data/schema_constraints_integration_test.go's own constants.
const (
	checkViolation  = "23514"
	uniqueViolation = "23505"
)

// wantPgError fails the test unless err is a *pgconn.PgError carrying exactly
// sqlState and constraint. It asserts the constraint NAME, not just the
// SQLSTATE: cash_movements alone carries four CHECK constraints plus a
// trigger that also raises 23514 (cash_movements_check_void, via RAISE ...
// USING CONSTRAINT — the same mechanism that lets a trigger populate
// ConstraintName the way a real constraint does), so "some 23514" would keep
// passing against a row refused for the wrong reason.
func wantPgError(t *testing.T, err error, sqlState, constraint string) {
	t.Helper()

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want a *pgconn.PgError refused by %s; got %T: %v", constraint, err, err)
	}
	if pgErr.Code != sqlState || pgErr.ConstraintName != constraint {
		t.Fatalf("want %s (%s); got %q (%s): %s", constraint, sqlState, pgErr.ConstraintName, pgErr.Code, pgErr.Message)
	}
}

// openSession is the one open-a-session step every test below starts from.
func openSession(t *testing.T, f *datatest.Fixture, openingCash int64) *cashboxstore.CashSession {
	t.Helper()

	session := &cashboxstore.CashSession{
		ComplexID:   f.ComplexID,
		OpenedBy:    f.UserID,
		OpeningCash: openingCash,
	}
	if err := f.Stores.Cashbox.OpenSession(f.Scoped(context.Background()), session); err != nil {
		t.Fatalf("opening session: %v", err)
	}
	return session
}

// closeSession is Close's ordinary call shape for every test below that does
// not care about the exact closedAt instant it writes: "now" is as good as
// any other value, since none of them assert against it. See
// TestIntegration_CloseWritesTheExactClosedAtItWasGiven for the one test that
// does care.
func closeSession(t *testing.T, f *datatest.Fixture, ctx context.Context, sessionID uuid.UUID, countedCash, cashBookingPayments, cashManualRefunds int64, note *string) *cashboxstore.CashSession {
	t.Helper()

	closed, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, sessionID, f.UserID, countedCash, cashBookingPayments, cashManualRefunds, time.Now(), note)
	if err != nil {
		t.Fatalf("closing session: %v", err)
	}
	return closed
}

// TestIntegration_OnlyOneOpenSessionPerComplex pins idx_cash_sessions_one_open
// (db/migrations/003_cashbox.sql) both through the store's own friendly
// pre-check and, bypassing it with a raw insert, through the unique index
// itself — the actual guarantee under a race.
func TestIntegration_OnlyOneOpenSessionPerComplex(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	openSession(t, f, 10000)

	second := &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: 5000}
	err := f.Stores.Cashbox.OpenSession(ctx, second)
	if !errors.Is(err, cashboxstore.ErrSessionAlreadyOpen) {
		t.Fatalf("want ErrSessionAlreadyOpen from the store's pre-check; got %v", err)
	}

	// The pre-check cannot see a concurrent insert; the index is what actually
	// closes that race. A raw INSERT bypasses the store entirely to prove the
	// index refuses it on its own — same complex_id as the open session
	// above, so it is the unique index refusing it, not row-level security
	// (this insert runs on the fixture's own transaction, already scoped to
	// f.ComplexID by Isolated's setup, the same connection every other raw
	// fixture write in this suite uses — see datatest.Fixture.Scoped).
	_, err = f.DB.Exec(context.Background(),
		`INSERT INTO cash_sessions (complex_id, opened_by, opening_cash) VALUES ($1, $2, $3)`,
		f.ComplexID, f.UserID, 7000)
	wantPgError(t, err, uniqueViolation, "idx_cash_sessions_one_open")
}

// TestIntegration_MovementRequiresAnOpenSession proves a movement cannot land
// in a session that is already closed.
func TestIntegration_MovementRequiresAnOpenSession(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)
	closeSession(t, f, ctx, session.ID, 10000, 0, 0, nil)

	movement := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "other_income", Method: "cash", Amount: 1000, CreatedBy: f.UserID,
	}
	err := f.Stores.Cashbox.InsertMovement(ctx, movement)
	if !errors.Is(err, cashboxstore.ErrSessionNotOpen) {
		t.Fatalf("want ErrSessionNotOpen for a movement against a closed session; got %v", err)
	}
}

// TestIntegration_ClosedSessionCannotBeUpdated pins the STORE's own guard: a
// second Close call against an already-closed session is refused before it
// ever reaches the database, because Store.Close locks the row and checks
// ClosedAt itself. It does NOT exercise
// cash_sessions_forbid_update_after_close (db/migrations/003_cashbox.sql) —
// that trigger only fires on an UPDATE the Go-side check never lets through,
// since every write path checks first. See
// TestIntegration_RawUpdateOfAClosedSessionIsRefusedByTheTrigger below for a
// test that actually reaches the trigger.
func TestIntegration_ClosedSessionCannotBeUpdated(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)
	closeSession(t, f, ctx, session.ID, 10000, 0, 0, nil)

	_, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 9999, 0, 0, time.Now(), nil)
	if !errors.Is(err, cashboxstore.ErrSessionNotOpen) {
		t.Fatalf("want ErrSessionNotOpen for closing an already-closed session; got %v", err)
	}
}

// TestIntegration_RawUpdateOfAClosedSessionIsRefusedByTheTrigger is the test
// TestIntegration_ClosedSessionCannotBeUpdated's own comment used to
// misdescribe itself as being: it bypasses the store entirely with a raw
// UPDATE against a closed session's row, so
// cash_sessions_forbid_update_after_close (db/migrations/003_cashbox.sql) is
// what actually refuses it, not Store.Close's pre-check.
func TestIntegration_RawUpdateOfAClosedSessionIsRefusedByTheTrigger(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)
	closeSession(t, f, ctx, session.ID, 10000, 0, 0, nil)

	_, err := f.DB.Exec(context.Background(),
		`UPDATE cash_sessions SET closing_note = 'tampered' WHERE id = $1`, session.ID)
	wantPgError(t, err, checkViolation, "cash_sessions_closed_is_immutable")
}

// TestIntegration_OpeningNoteSurvivesACloseWithItsOwnNote pins the note split
// (opening_note/closing_note, owner correction on pos-cashbox T2) against the
// real UPDATE: CloseCashSession only ever SETs closing_note, so the opening
// note Open wrote must still be there afterward, unmodified, alongside the
// new closing note.
func TestIntegration_OpeningNoteSurvivesACloseWithItsOwnNote(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	opening := "short today, missing envelope"
	session := &cashboxstore.CashSession{ComplexID: f.ComplexID, OpenedBy: f.UserID, OpeningCash: 10000, OpeningNote: &opening}
	if err := f.Stores.Cashbox.OpenSession(ctx, session); err != nil {
		t.Fatalf("opening session: %v", err)
	}

	closingNote := "left 500 for tomorrow"
	closed := closeSession(t, f, ctx, session.ID, 10000, 0, 0, &closingNote)

	if closed.OpeningNote == nil || *closed.OpeningNote != opening {
		t.Errorf("want the opening note %q to survive the close; got %+v", opening, closed.OpeningNote)
	}
	if closed.ClosingNote == nil || *closed.ClosingNote != closingNote {
		t.Errorf("want the closing note %q on the closed session; got %+v", closingNote, closed.ClosingNote)
	}
}

// TestIntegration_VoidOfVoidRefused pins cash_movements_no_void_of_void.
func TestIntegration_VoidOfVoidRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	original := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense",
		Category: "supplies", Method: "cash", Amount: 2000, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, original); err != nil {
		t.Fatalf("inserting original: %v", err)
	}

	void := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "supplies", Method: "cash", Amount: 2000, CreatedBy: f.UserID,
		VoidsMovementID: &original.ID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, void); err != nil {
		t.Fatalf("voiding original: %v", err)
	}

	voidOfVoid := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense",
		Category: "supplies", Method: "cash", Amount: 2000, CreatedBy: f.UserID,
		VoidsMovementID: &void.ID,
	}
	err := f.Stores.Cashbox.InsertMovement(ctx, voidOfVoid)
	if !errors.Is(err, cashboxstore.ErrVoidOfVoid) {
		t.Fatalf("want ErrVoidOfVoid when voiding a void; got %v", err)
	}
}

// TestIntegration_DoubleVoidRefused pins the UNIQUE constraint on
// voids_movement_id: an original may be voided once.
func TestIntegration_DoubleVoidRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	original := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense",
		Category: "supplies", Method: "cash", Amount: 1500, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, original); err != nil {
		t.Fatalf("inserting original: %v", err)
	}

	firstVoid := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "supplies", Method: "cash", Amount: 1500, CreatedBy: f.UserID,
		VoidsMovementID: &original.ID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, firstVoid); err != nil {
		t.Fatalf("first void: %v", err)
	}

	secondVoid := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "supplies", Method: "cash", Amount: 1500, CreatedBy: f.UserID,
		VoidsMovementID: &original.ID,
	}
	err := f.Stores.Cashbox.InsertMovement(ctx, secondVoid)
	if !errors.Is(err, cashboxstore.ErrAlreadyVoided) {
		t.Fatalf("want ErrAlreadyVoided for a second void of the same original; got %v", err)
	}
}

// TestIntegration_VoidMustCarryTheOppositeKindAndTheOriginalsOwnFields pins
// cash_movements_void_opposite_kind / cash_movements_void_matches_original:
// even bypassing the service (which always composes a faithful void), the
// database itself refuses a void that does not mirror its original.
func TestIntegration_VoidMustCarryTheOppositeKindAndTheOriginalsOwnFields(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	original := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense",
		Category: "supplies", Method: "cash", Amount: 1500, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, original); err != nil {
		t.Fatalf("inserting original: %v", err)
	}

	sameKind := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "expense", // wrong: must be income
		Category: "supplies", Method: "cash", Amount: 1500, CreatedBy: f.UserID,
		VoidsMovementID: &original.ID,
	}
	err := f.Stores.Cashbox.InsertMovement(ctx, sameKind)
	wantPgError(t, err, checkViolation, "cash_movements_void_opposite_kind")

	wrongAmount := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "supplies", Method: "cash", Amount: 999, CreatedBy: f.UserID, // wrong amount
		VoidsMovementID: &original.ID,
	}
	err = f.Stores.Cashbox.InsertMovement(ctx, wrongAmount)
	wantPgError(t, err, checkViolation, "cash_movements_void_matches_original")
}

// TestIntegration_CategoryMustMatchKindForANonVoidRow pins
// cash_movements_category_kind_consistent.
func TestIntegration_CategoryMustMatchKindForANonVoidRow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	mismatched := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "supplies", // an expense category on an income row
		Method:   "cash", Amount: 1000, CreatedBy: f.UserID,
	}
	err := f.Stores.Cashbox.InsertMovement(ctx, mismatched)
	wantPgError(t, err, checkViolation, "cash_movements_category_kind_consistent")
}

// TestIntegration_MethodCannotBeMercadopago pins cash_movements_method_not_online.
func TestIntegration_MethodCannotBeMercadopago(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	movement := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "other_income", Method: "mercadopago", Amount: 1000, CreatedBy: f.UserID,
	}
	err := f.Stores.Cashbox.InsertMovement(ctx, movement)
	wantPgError(t, err, checkViolation, "cash_movements_method_not_online")
}

// TestIntegration_CloseComputesExpectedCashFromOpeningPlusCashMovements pins
// the arithmetic Store.Close performs under the session lock: opening_cash +
// cash income - cash expense (a non-cash movement must be excluded), plus
// whatever cashBookingPaymentsInWindow the caller supplies (the service's own
// job — see reports_window_integration_test.go for that half).
func TestIntegration_CloseComputesExpectedCashFromOpeningPlusCashMovements(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	insert := func(kind, category, method string, amount int) {
		t.Helper()
		m := &cashboxstore.CashMovement{
			ComplexID: f.ComplexID, SessionID: session.ID, Kind: kind,
			Category: category, Method: method, Amount: amount, CreatedBy: f.UserID,
		}
		if err := f.Stores.Cashbox.InsertMovement(ctx, m); err != nil {
			t.Fatalf("inserting %s/%s movement: %v", kind, category, err)
		}
	}
	insert("income", "other_income", "cash", 3000)
	insert("expense", "supplies", "cash", 800)
	// A transfer movement, deliberately large, to prove it is excluded.
	insert("income", "other_income", "transfer", 500000)

	const cashBookingPayments = 12000
	closed := closeSession(t, f, ctx, session.ID, 24200, cashBookingPayments, 0, nil)

	// 10000 (opening) + 3000 (cash income) - 800 (cash expense) + 12000 (booking payments) = 24200
	var wantExpected int64 = 24200
	if closed.ExpectedCash == nil || *closed.ExpectedCash != wantExpected {
		t.Fatalf("want expected_cash=%d; got %+v", wantExpected, closed.ExpectedCash)
	}
	if closed.Difference == nil || *closed.Difference != 0 {
		t.Errorf("counted_cash was set equal to expected_cash; want difference=0, got %+v", closed.Difference)
	}
}

// TestIntegration_CloseSubtractsCashManualRefundsInWindow pins the other half
// of Store.Close's arithmetic (cash-manual-refunds): opening_cash + cash
// income - cash expense + cashBookingPaymentsInWindow -
// cashManualRefundsInWindow, the caller-supplied figure the service reads
// through reportstore.Store.ManualRefundSummaryByMethodWindow.
func TestIntegration_CloseSubtractsCashManualRefundsInWindow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	const cashBookingPayments = 12000
	const cashManualRefunds = 5000
	closed, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID,
		7000, cashBookingPayments, cashManualRefunds, time.Now(), nil)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 10000 (opening) + 12000 (booking payments) - 5000 (manual refunds) = 17000
	var wantExpected int64 = 17000
	if closed.ExpectedCash == nil || *closed.ExpectedCash != wantExpected {
		t.Fatalf("want expected_cash=%d; got %+v", wantExpected, closed.ExpectedCash)
	}
}

// TestIntegration_ExpectedCashAboveInt32RangeIsStoredAndReadBackExactly pins
// the overflow fix: cash_sessions.opening_cash/counted_cash/expected_cash/
// difference are BIGINT (db/migrations/003_cashbox.sql), not INTEGER, exactly
// so a session-level total above 2,147,483,647 centavos (2^31-1, what an
// INTEGER column silently wrapped at before that migration) survives Close
// and every later read intact. A wrapped expected_cash would have been
// permanent: the session's close-state columns are set once and
// cash_sessions_forbid_update_after_close means there is never a second write
// that could fix it — see that migration's own comment on this table.
func TestIntegration_ExpectedCashAboveInt32RangeIsStoredAndReadBackExactly(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	// 3,000,000,000 centavos (30 million ARS) is comfortably past
	// math.MaxInt32 (2,147,483,647) and within what request validation
	// lets a single opening_cash or counted_cash input be
	// (internal/cashbox/handlers.go's maxSessionCash) — this is the
	// session-level total the fix is actually about, reached the way a real
	// one would be: a big opening float, not a single oversized keystroke.
	const bigOpeningCash int64 = 3_000_000_000
	session := openSession(t, f, bigOpeningCash)

	closed := closeSession(t, f, ctx, session.ID, bigOpeningCash, 0, 0, nil)

	if closed.ExpectedCash == nil || *closed.ExpectedCash != bigOpeningCash {
		t.Fatalf("want expected_cash=%d stored exactly; got %+v (an INTEGER column would have wrapped this)",
			bigOpeningCash, closed.ExpectedCash)
	}
	if closed.CountedCash == nil || *closed.CountedCash != bigOpeningCash {
		t.Fatalf("want counted_cash=%d stored exactly; got %+v", bigOpeningCash, closed.CountedCash)
	}
	if closed.Difference == nil || *closed.Difference != 0 {
		t.Errorf("counted_cash was set equal to expected_cash; want difference=0, got %+v", closed.Difference)
	}

	// Re-read straight from the database, bypassing whatever the Go struct
	// already holds, to prove the GENERATED difference column itself — not
	// just this process's in-memory arithmetic — carries the wide value too.
	var rawOpening, rawExpected, rawCounted, rawDifference int64
	err := f.DB.QueryRow(context.Background(),
		`SELECT opening_cash, expected_cash, counted_cash, difference FROM cash_sessions WHERE id = $1`,
		session.ID,
	).Scan(&rawOpening, &rawExpected, &rawCounted, &rawDifference)
	if err != nil {
		t.Fatalf("re-reading the session row: %v", err)
	}
	if rawOpening != bigOpeningCash || rawExpected != bigOpeningCash || rawCounted != bigOpeningCash || rawDifference != 0 {
		t.Fatalf("want opening_cash=expected_cash=counted_cash=%d and difference=0 straight from Postgres; got opening=%d expected=%d counted=%d difference=%d",
			bigOpeningCash, rawOpening, rawExpected, rawCounted, rawDifference)
	}
}

// TestIntegration_CloseWritesTheExactClosedAtItWasGiven pins the close-
// snapshot-skew fix: CloseCashSession used to SET closed_at = NOW(), a value
// computed by Postgres strictly after the service had already read booking
// payments over a window ending at its OWN, earlier "now" — so a payment
// landing in that gap was silently excluded from the stored expected_cash
// while still falling inside [opened_at, closed_at) the next time a summary
// was rebuilt. closed_at is now a parameter (db/queries/cashbox.sql), and
// this proves the row actually carries the exact instant it was given rather
// than a fresh timestamp Postgres picked on its own — the other half of the
// fix is internal/cashbox's own
// TestCloseWritesTheSameInstantItReadTheBookingPaymentsWindowThrough, which
// proves the SERVICE reads that window and picks closedAt from the very same
// time.Now() call.
func TestIntegration_CloseWritesTheExactClosedAtItWasGiven(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)

	// Deliberately not time.Now(): a value far enough from "whenever this
	// test happens to run" that it could only appear in the row if the query
	// actually used it, never by coincidence with a live NOW().
	closedAt := time.Date(2030, time.January, 2, 3, 4, 5, 123456000, time.UTC)

	closed, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 10000, 0, 0, closedAt, nil)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closed.ClosedAt == nil || !closed.ClosedAt.Equal(closedAt) {
		t.Fatalf("want closed_at=%s (the exact instant given to Close); got %+v", closedAt, closed.ClosedAt)
	}

	// Re-read straight from the database: the returned struct alone would not
	// catch a query that wrote NOW() but happened to RETURN the caller's own
	// value some other way.
	var rawClosedAt time.Time
	if err := f.DB.QueryRow(context.Background(),
		`SELECT closed_at FROM cash_sessions WHERE id = $1`, session.ID,
	).Scan(&rawClosedAt); err != nil {
		t.Fatalf("re-reading the session row: %v", err)
	}
	if !rawClosedAt.Equal(closedAt) {
		t.Fatalf("want closed_at=%s straight from Postgres; got %s", closedAt, rawClosedAt)
	}
}

// TestIntegration_ExpectedCashExcludesANonCashMovementEvenAloneInTheSession
// proves a session with only a transfer movement reconciles to just its
// opening float.
func TestIntegration_ExpectedCashExcludesANonCashMovementEvenAloneInTheSession(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 5000)

	movement := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "other_income", Method: "qr_wallet", Amount: 100000, CreatedBy: f.UserID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, movement); err != nil {
		t.Fatalf("inserting movement: %v", err)
	}

	closed := closeSession(t, f, ctx, session.ID, 5000, 0, 0, nil)
	if closed.ExpectedCash == nil || *closed.ExpectedCash != 5000 {
		t.Fatalf("want expected_cash=5000 (opening only, the QR wallet movement excluded); got %+v", closed.ExpectedCash)
	}
}

// TestIntegration_TenantIsolationOnByIDLookup is the store-level half of
// tenant isolation for the new tables: a session id that exists, scoped to
// a DIFFERENT complex than the one asked for, must answer
// data.ErrRecordNotFound — the same explicit half of the guarantee
// GetBookingByID's own comment (db/queries/bookings.sql) documents, ahead of
// row-level security itself. A full row-level-security-as-vibe_app proof
// (internal/data/rls_integration_test.go's two-pool fixture) is not
// duplicated here; the policies on cash_sessions/cash_movements are the
// identical column-comparison pattern that suite already proves for every
// other tenant-scoped table.
func TestIntegration_TenantIsolationOnByIDLookup(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 1000)

	otherComplexID := uuid.New()
	_, err := f.Stores.Cashbox.GetByID(ctx, otherComplexID, session.ID)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("a session looked up under a different complex_id must answer ErrRecordNotFound; got %v", err)
	}
}

// TestIntegration_ListByComplexPagination is a light smoke test of the
// keyset query — the sqlc query and the has_cursor branch it depends on both
// compile against the real schema.
func TestIntegration_ListByComplexPagination(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	first := openSession(t, f, 1000)
	// data.BuildTimestampCursor (internal/data/filters.go) encodes the cursor
	// at one-second (RFC3339) precision, so two sessions opened within the
	// same wall-clock second would make the boundary ambiguous — nothing to
	// do with this query, the same as any other keyset-paginated list in this
	// codebase. Backdating first's opened_at is what every other such test
	// does (see BackdateBookingCreatedAt) to keep the two pages unambiguous.
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE cash_sessions SET opened_at = opened_at - INTERVAL '2 seconds' WHERE id = $1`, first.ID); err != nil {
		t.Fatalf("backdating first session: %v", err)
	}
	closeSession(t, f, ctx, first.ID, 1000, 0, 0, nil)
	second := openSession(t, f, 2000)
	closeSession(t, f, ctx, second.ID, 2000, 0, 0, nil)

	page, meta, err := f.Stores.Cashbox.ListByComplex(ctx, f.ComplexID, data.Filters{Limit: 1})
	if err != nil {
		t.Fatalf("ListByComplex: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("want a page of 1; got %d", len(page))
	}
	if page[0].ID != second.ID {
		t.Errorf("want the most recently opened session first; got %s, want %s", page[0].ID, second.ID)
	}
	if !meta.HasMore {
		t.Error("want has_more=true with a second session still unread")
	}

	nextPage, _, err := f.Stores.Cashbox.ListByComplex(ctx, f.ComplexID, data.Filters{Limit: 1, Cursor: meta.NextCursor})
	if err != nil {
		t.Fatalf("ListByComplex (page 2): %v", err)
	}
	if len(nextPage) != 1 || nextPage[0].ID != first.ID {
		t.Fatalf("want the first-opened session on page 2; got %+v", nextPage)
	}
}
