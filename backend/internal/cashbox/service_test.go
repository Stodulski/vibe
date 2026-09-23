package cashbox

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

func testActor() Actor {
	id := uuid.New()
	return Actor{UserID: &id, IP: "127.0.0.1"}
}

func TestOpenPropagatesAlreadyOpenError(t *testing.T) {
	store := &stubStore{openSessionErr: cashboxstore.ErrSessionAlreadyOpen}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	_, err := svc.Open(t.Context(), uuid.New(), testActor(), uuid.New(), OpenInput{OpeningCash: 10000})
	if !errors.Is(err, cashboxstore.ErrSessionAlreadyOpen) {
		t.Fatalf("want ErrSessionAlreadyOpen; got %v", err)
	}
}

func TestOpenRecordsAnAuditEntry(t *testing.T) {
	store := &stubStore{}
	rec := &stubRecorder{}
	svc := newTestService(store, &stubPayments{}, rec)

	complexID := uuid.New()
	actor := testActor()
	session, err := svc.Open(t.Context(), complexID, actor, uuid.New(), OpenInput{OpeningCash: 5000})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(rec.entries) != 1 {
		t.Fatalf("want 1 audit entry; got %d", len(rec.entries))
	}
	entry := rec.entries[0]
	if entry.Action != "open" || entry.EntityType != "cash_session" {
		t.Errorf("want action=open entity_type=cash_session; got action=%s entity_type=%s", entry.Action, entry.EntityType)
	}
	if entry.EntityID == nil || *entry.EntityID != session.ID {
		t.Errorf("audit entry's EntityID does not name the opened session")
	}
}

// --- Close -------------------------------------------------------------

func TestCloseRefusesASessionThatIsAlreadyClosed(t *testing.T) {
	closedAt := time.Now()
	store := &stubStore{byID: &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), ClosedAt: &closedAt}}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	_, err := svc.Close(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), CloseInput{CountedCash: 1000})
	if !errors.Is(err, cashboxstore.ErrSessionNotOpen) {
		t.Fatalf("want ErrSessionNotOpen; got %v", err)
	}
}

func TestCloseSendsNoClosingNoteWhenTheRequestSendsNone(t *testing.T) {
	store := &stubStore{
		byID:          &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now()},
		closedSession: &cashboxstore.CashSession{},
	}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	_, err := svc.Close(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), CloseInput{CountedCash: 1000})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if store.closeArgs == nil || store.closeArgs.closingNote != nil {
		t.Errorf("want no closing note passed to the store when the request sends none; got %+v", store.closeArgs)
	}
}

// TestCloseNeverTouchesTheOpeningNote is the point of the split
// (opening_note/closing_note, owner correction on pos-cashbox T2): a closing
// note must never erase what the opener recorded. The service passes only
// ClosingInput.ClosingNote to the store — never the session's own
// OpeningNote — and the session Close returns still carries the opening note
// Open wrote, untouched, alongside the new closing note.
func TestCloseNeverTouchesTheOpeningNote(t *testing.T) {
	opening := "short today"
	closingNote := "left 500 for tomorrow"
	sessionID := uuid.New()

	store := &stubStore{
		byID: &cashboxstore.CashSession{ID: sessionID, ComplexID: uuid.New(), OpenedAt: time.Now(), OpeningNote: &opening},
		// The real CloseCashSession UPDATE only ever SETs closing_note, so the
		// row it returns still carries whatever Open wrote for opening_note —
		// the stub mirrors that here rather than the service somehow
		// preserving it, since the service does not even see this value.
		closedSession: &cashboxstore.CashSession{ID: sessionID, OpeningNote: &opening, ClosingNote: &closingNote},
	}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	closed, err := svc.Close(t.Context(), uuid.New(), sessionID, testActor(), uuid.New(), CloseInput{CountedCash: 1000, ClosingNote: &closingNote})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	if store.closeArgs == nil || store.closeArgs.closingNote == nil || *store.closeArgs.closingNote != closingNote {
		t.Fatalf("want the store call to carry the closing note %q; got %+v", closingNote, store.closeArgs)
	}
	if closed.OpeningNote == nil || *closed.OpeningNote != opening {
		t.Errorf("want the opening note %q to survive the close untouched; got %+v", opening, closed.OpeningNote)
	}
	if closed.ClosingNote == nil || *closed.ClosingNote != closingNote {
		t.Errorf("want the closing note %q on the closed session; got %+v", closingNote, closed.ClosingNote)
	}
}

func TestClosePassesTheCashPortionOfBookingPaymentsOnly(t *testing.T) {
	store := &stubStore{
		byID:          &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now()},
		closedSession: &cashboxstore.CashSession{},
	}
	payments := &stubPayments{summaries: []reportstore.PaymentMethodSummary{
		{Method: "cash", Amount: 30000, ServiceFee: 0},
		{Method: "mercadopago", Amount: 90000, ServiceFee: 4500},
	}}
	svc := newTestService(store, payments, &stubRecorder{})

	_, err := svc.Close(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), CloseInput{CountedCash: 1000})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if store.closeArgs == nil || store.closeArgs.cashBookingPayments != 30000 {
		t.Errorf("want cashBookingPayments=30000 (mercadopago excluded); got %+v", store.closeArgs)
	}
}

// TestCloseWritesTheSameInstantItReadTheBookingPaymentsWindowThrough pins the
// close-snapshot-skew fix: the window Close reads booking payments over must
// end at EXACTLY the instant it asks the store to write as closed_at, or a
// payment landing between two different "now" reads would be missing from
// the stored expected_cash while still falling inside a summary later
// rebuilt over [opened_at, closed_at) — see Service.Close's own comment.
func TestCloseWritesTheSameInstantItReadTheBookingPaymentsWindowThrough(t *testing.T) {
	store := &stubStore{
		byID:          &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now()},
		closedSession: &cashboxstore.CashSession{},
	}
	payments := &stubPayments{}
	svc := newTestService(store, payments, &stubRecorder{})

	_, err := svc.Close(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), CloseInput{CountedCash: 1000})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if store.closeArgs == nil {
		t.Fatal("want the store's Close to have been called")
	}
	if !payments.lastTo.Equal(store.closeArgs.closedAt) {
		t.Errorf("want the booking-payments window end (%v) to equal the closed_at written to the store (%v)",
			payments.lastTo, store.closeArgs.closedAt)
	}
}

// --- VoidMovement --------------------------------------------------------

func TestVoidMovementCarriesTheOppositeKindAndTheOriginalsFields(t *testing.T) {
	original := &cashboxstore.CashMovement{
		ID: uuid.New(), SessionID: uuid.New(), Kind: "expense", Category: "supplies",
		Method: "cash", Amount: 5000,
	}
	openSession := &cashboxstore.CashSession{ID: uuid.New()}
	store := &stubStore{movementByID: original, openSession: openSession}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	void, err := svc.VoidMovement(t.Context(), uuid.New(), original.SessionID, original.ID, testActor(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("VoidMovement: %v", err)
	}
	if void.Kind != "income" {
		t.Errorf("want the void's kind to be the opposite of the original (income); got %s", void.Kind)
	}
	if void.Category != original.Category || void.Method != original.Method || void.Amount != original.Amount {
		t.Errorf("want the void to reuse the original's category/method/amount; got %+v", void)
	}
	if void.VoidsMovementID == nil || *void.VoidsMovementID != original.ID {
		t.Errorf("want VoidsMovementID to name the original")
	}
	if void.SessionID != openSession.ID {
		t.Errorf("want the void to land in the currently open session (%s), not the original's (%s)", openSession.ID, original.SessionID)
	}
}

func TestVoidMovementRefusesAMovementFromAnotherSessionThanThePathNames(t *testing.T) {
	original := &cashboxstore.CashMovement{ID: uuid.New(), SessionID: uuid.New(), Kind: "expense", Category: "supplies", Method: "cash", Amount: 100}
	store := &stubStore{movementByID: original}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	// pathSessionID deliberately does not match original.SessionID.
	_, err := svc.VoidMovement(t.Context(), uuid.New(), uuid.New(), original.ID, testActor(), uuid.New(), nil)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound for a session/movement mismatch; got %v", err)
	}
}

func TestVoidMovementWithNoOpenSessionAnswersErrSessionNotOpen(t *testing.T) {
	original := &cashboxstore.CashMovement{ID: uuid.New(), SessionID: uuid.New(), Kind: "income", Category: "other_income", Method: "cash", Amount: 100}
	store := &stubStore{movementByID: original, getOpenErr: data.ErrRecordNotFound}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	_, err := svc.VoidMovement(t.Context(), uuid.New(), original.SessionID, original.ID, testActor(), uuid.New(), nil)
	if !errors.Is(err, cashboxstore.ErrSessionNotOpen) {
		t.Fatalf("want ErrSessionNotOpen; got %v", err)
	}
}

// TestVoidMovementRefusesASaleIncome pins the pos-cashbox T4b rule: a 'sale'
// category income can only be voided by voiding the sale
// (internal/sales.Service.Void), never through this endpoint directly —
// otherwise the sale's stock would never be restored.
func TestVoidMovementRefusesASaleIncome(t *testing.T) {
	original := &cashboxstore.CashMovement{ID: uuid.New(), SessionID: uuid.New(), Kind: "income", Category: "sale", Method: "cash", Amount: 1500}
	store := &stubStore{movementByID: original}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	_, err := svc.VoidMovement(t.Context(), uuid.New(), original.SessionID, original.ID, testActor(), uuid.New(), nil)
	if !errors.Is(err, cashboxstore.ErrCannotVoidSaleManually) {
		t.Fatalf("want ErrCannotVoidSaleManually; got %v", err)
	}
}

func TestVoidMovementOfAnIncomeMovementIsAnExpense(t *testing.T) {
	original := &cashboxstore.CashMovement{ID: uuid.New(), SessionID: uuid.New(), Kind: "income", Category: "other_income", Method: "transfer", Amount: 2000}
	openSession := &cashboxstore.CashSession{ID: uuid.New()}
	store := &stubStore{movementByID: original, openSession: openSession}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	void, err := svc.VoidMovement(t.Context(), uuid.New(), original.SessionID, original.ID, testActor(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("VoidMovement: %v", err)
	}
	if void.Kind != "expense" {
		t.Errorf("want expense; got %s", void.Kind)
	}
}

// --- buildSummary / expected cash ---------------------------------------

func TestCurrentComputesALiveExpectedCashFromCashMovementsAndCashBookingPayments(t *testing.T) {
	session := &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now(), OpeningCash: 10000}
	store := &stubStore{
		openSession: session,
		sumTotals: []cashboxstore.MovementTotal{
			{Method: "cash", Kind: "income", Category: "other_income", Total: 2000, Count: 1},
			{Method: "cash", Kind: "expense", Category: "supplies", Total: 500, Count: 1},
			// A transfer movement must not affect expected cash.
			{Method: "transfer", Kind: "income", Category: "other_income", Total: 99999, Count: 1},
		},
	}
	payments := &stubPayments{summaries: []reportstore.PaymentMethodSummary{
		{Method: "cash", Amount: 8000, ServiceFee: 0},
		{Method: "mercadopago", Amount: 50000, ServiceFee: 2500},
	}}
	svc := newTestService(store, payments, &stubRecorder{})

	current, err := svc.Current(t.Context(), session.ComplexID)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}

	// 10000 (opening) + 2000 (cash income) - 500 (cash expense) + 8000 (cash booking payments) = 19500
	var want int64 = 19500
	if current.Summary.ExpectedCash != want {
		t.Errorf("want expected_cash=%d; got %d", want, current.Summary.ExpectedCash)
	}
	if current.Summary.CountedCash != nil || current.Summary.Difference != nil {
		t.Errorf("an open session must not report counted_cash/difference; got %+v", current.Summary)
	}
}

// TestCurrentSubtractsOnlyTheCashPortionOfManualRefunds pins the
// cash-manual-refunds arithmetic (internal/cashbox/service.go's
// sumCashManualRefunds): a live summary's expected cash subtracts the cash
// row(s) of ManualRefundSummaryByMethodWindow, and a transfer row must not
// touch it.
func TestCurrentSubtractsOnlyTheCashPortionOfManualRefunds(t *testing.T) {
	session := &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now(), OpeningCash: 10000}
	store := &stubStore{openSession: session}
	payments := &stubPayments{
		summaries: []reportstore.PaymentMethodSummary{
			{Method: "cash", Amount: 8000, ServiceFee: 0},
		},
		manualRefunds: []reportstore.ManualRefundMethodSummary{
			{Method: "cash", Amount: 3000, Count: 1},
			{Method: "transfer", Amount: 99999, Count: 1},
		},
	}
	svc := newTestService(store, payments, &stubRecorder{})

	current, err := svc.Current(t.Context(), session.ComplexID)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}

	// 10000 (opening) + 8000 (cash booking payments) - 3000 (cash manual
	// refund; the transfer row excluded) = 15000
	var want int64 = 15000
	if current.Summary.ExpectedCash != want {
		t.Errorf("want expected_cash=%d; got %d", want, current.Summary.ExpectedCash)
	}
	if current.Summary.CashManualRefunds != 3000 {
		t.Errorf("want CashManualRefunds=3000; got %d", current.Summary.CashManualRefunds)
	}
}

func TestClosePassesCashManualRefundsSubtracted(t *testing.T) {
	store := &stubStore{
		byID:          &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now()},
		closedSession: &cashboxstore.CashSession{},
	}
	payments := &stubPayments{
		manualRefunds: []reportstore.ManualRefundMethodSummary{
			{Method: "cash", Amount: 4500, Count: 1},
			{Method: "transfer", Amount: 999, Count: 1},
		},
	}
	svc := newTestService(store, payments, &stubRecorder{})

	_, err := svc.Close(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), CloseInput{CountedCash: 1000})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if store.closeArgs == nil || store.closeArgs.cashManualRefunds != 4500 {
		t.Errorf("want cashManualRefunds=4500 (transfer excluded); got %+v", store.closeArgs)
	}
	if !payments.lastManualTo.Equal(store.closeArgs.closedAt) {
		t.Errorf("want the manual-refund window end (%v) to equal the closed_at written to the store (%v)",
			payments.lastManualTo, store.closeArgs.closedAt)
	}
}

// TestCurrentReturnsTheManualRefundSummaryError pins that buildSummary does
// not swallow a ManualRefundSummaryByMethodWindow failure: Current must
// surface it rather than answering with a summary computed on partial data.
func TestCurrentReturnsTheManualRefundSummaryError(t *testing.T) {
	wantErr := errors.New("manual refund summary boom")
	session := &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now()}
	store := &stubStore{openSession: session}
	payments := &stubPayments{manualRefundsErr: wantErr}
	svc := newTestService(store, payments, &stubRecorder{})

	_, err := svc.Current(t.Context(), session.ComplexID)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want the ManualRefundSummaryByMethodWindow error; got %v", err)
	}
}

// TestGetReturnsTheManualRefundSummaryError is Current's sibling for Get,
// which also rebuilds the summary through buildSummary.
func TestGetReturnsTheManualRefundSummaryError(t *testing.T) {
	wantErr := errors.New("manual refund summary boom")
	session := &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now()}
	store := &stubStore{byID: session}
	payments := &stubPayments{manualRefundsErr: wantErr}
	svc := newTestService(store, payments, &stubRecorder{})

	_, err := svc.Get(t.Context(), session.ComplexID, session.ID)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want the ManualRefundSummaryByMethodWindow error; got %v", err)
	}
}

// TestCloseReturnsTheManualRefundSummaryErrorWithoutClosingTheStore pins that
// Close reads ManualRefundSummaryByMethodWindow before ever calling the
// store's own Close: a read failure here must never let a session close on
// an expected-cash figure computed without knowing what was manually
// refunded.
func TestCloseReturnsTheManualRefundSummaryErrorWithoutClosingTheStore(t *testing.T) {
	wantErr := errors.New("manual refund summary boom")
	store := &stubStore{
		byID:          &cashboxstore.CashSession{ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: time.Now()},
		closedSession: &cashboxstore.CashSession{},
	}
	payments := &stubPayments{manualRefundsErr: wantErr}
	svc := newTestService(store, payments, &stubRecorder{})

	_, err := svc.Close(t.Context(), uuid.New(), uuid.New(), testActor(), uuid.New(), CloseInput{CountedCash: 1000})
	if !errors.Is(err, wantErr) {
		t.Fatalf("want the ManualRefundSummaryByMethodWindow error; got %v", err)
	}
	if store.closeArgs != nil {
		t.Errorf("want the store's Close never called when the manual-refund read fails; got %+v", store.closeArgs)
	}
}

func TestGetOnAClosedSessionEchoesTheStoredSnapshotRatherThanRecomputing(t *testing.T) {
	var countedCash, expectedCash, difference int64 = 5000, 4500, 500
	closedAt := time.Now()
	session := &cashboxstore.CashSession{
		ID: uuid.New(), ComplexID: uuid.New(), OpenedAt: closedAt.Add(-time.Hour),
		OpeningCash: 1000, ClosedAt: &closedAt, CountedCash: &countedCash, ExpectedCash: &expectedCash, Difference: &difference,
	}
	// A movement total that, if this were recomputed live, would change the
	// answer — proving Get trusts the stored snapshot instead.
	store := &stubStore{
		byID: session,
		sumTotals: []cashboxstore.MovementTotal{
			{Method: "cash", Kind: "income", Category: "other_income", Total: 999999, Count: 1},
		},
	}
	svc := newTestService(store, &stubPayments{}, &stubRecorder{})

	detail, err := svc.Get(t.Context(), session.ComplexID, session.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if detail.Summary.ExpectedCash != expectedCash {
		t.Errorf("want the stored snapshot %d; got %d (recomputed live instead of trusted)", expectedCash, detail.Summary.ExpectedCash)
	}
	if detail.Summary.CountedCash == nil || *detail.Summary.CountedCash != countedCash {
		t.Errorf("want counted_cash=%d; got %+v", countedCash, detail.Summary.CountedCash)
	}
}
