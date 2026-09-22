//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// openSession is the one open-a-session step every test below starts from.
func openSession(t *testing.T, f *datatest.Fixture, openingCash int) *cashboxstore.CashSession {
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
	// index refuses it on its own.
	_, err = f.DB.Exec(context.Background(),
		`INSERT INTO cash_sessions (complex_id, opened_by, opening_cash) VALUES ($1, $2, $3)`,
		f.ComplexID, f.UserID, 7000)
	if err == nil {
		t.Fatal("want the unique index to refuse a second open session inserted directly; got no error")
	}
}

// TestIntegration_MovementRequiresAnOpenSession proves a movement cannot land
// in a session that is already closed.
func TestIntegration_MovementRequiresAnOpenSession(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)
	if _, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 10000, 0, nil); err != nil {
		t.Fatalf("closing session: %v", err)
	}

	movement := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "other_income", Method: "cash", Amount: 1000, CreatedBy: f.UserID,
	}
	err := f.Stores.Cashbox.InsertMovement(ctx, movement)
	if !errors.Is(err, cashboxstore.ErrSessionNotOpen) {
		t.Fatalf("want ErrSessionNotOpen for a movement against a closed session; got %v", err)
	}
}

// TestIntegration_ClosedSessionCannotBeUpdated pins
// cash_sessions_forbid_update_after_close: a second close of the same
// session — the concurrent-close race — must be refused, not silently
// overwrite the first close's snapshot.
func TestIntegration_ClosedSessionCannotBeUpdated(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	session := openSession(t, f, 10000)
	if _, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 10000, 0, nil); err != nil {
		t.Fatalf("closing session: %v", err)
	}

	_, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 9999, 0, nil)
	if !errors.Is(err, cashboxstore.ErrSessionNotOpen) {
		t.Fatalf("want ErrSessionNotOpen for closing an already-closed session; got %v", err)
	}
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
	closed, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 10000, 0, &closingNote)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

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
	if err := f.Stores.Cashbox.InsertMovement(ctx, sameKind); err == nil {
		t.Error("want a same-kind void refused")
	}

	wrongAmount := &cashboxstore.CashMovement{
		ComplexID: f.ComplexID, SessionID: session.ID, Kind: "income",
		Category: "supplies", Method: "cash", Amount: 999, CreatedBy: f.UserID, // wrong amount
		VoidsMovementID: &original.ID,
	}
	if err := f.Stores.Cashbox.InsertMovement(ctx, wrongAmount); err == nil {
		t.Error("want a void with a different amount refused")
	}
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
	if err := f.Stores.Cashbox.InsertMovement(ctx, mismatched); err == nil {
		t.Error("want an income movement with an expense category refused")
	}
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
	if err := f.Stores.Cashbox.InsertMovement(ctx, movement); err == nil {
		t.Error("want a mercadopago cash movement refused")
	}
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
	closed, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 24200, cashBookingPayments, nil)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 10000 (opening) + 3000 (cash income) - 800 (cash expense) + 12000 (booking payments) = 24200
	wantExpected := 24200
	if closed.ExpectedCash == nil || *closed.ExpectedCash != wantExpected {
		t.Fatalf("want expected_cash=%d; got %+v", wantExpected, closed.ExpectedCash)
	}
	if closed.Difference == nil || *closed.Difference != 0 {
		t.Errorf("counted_cash was set equal to expected_cash; want difference=0, got %+v", closed.Difference)
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

	closed, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, session.ID, f.UserID, 5000, 0, nil)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
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
	if _, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, first.ID, f.UserID, 1000, 0, nil); err != nil {
		t.Fatalf("closing first session: %v", err)
	}
	second := openSession(t, f, 2000)
	if _, err := f.Stores.Cashbox.Close(ctx, f.ComplexID, second.ID, f.UserID, 2000, 0, nil); err != nil {
		t.Fatalf("closing second session: %v", err)
	}

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
