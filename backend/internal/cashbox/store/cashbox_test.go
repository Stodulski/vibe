package store

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestCashIncomeAndExpenseCountsOnlyCashRows(t *testing.T) {
	totals := []MovementTotal{
		{Method: "cash", Kind: "income", Category: "other_income", Total: 5000},
		{Method: "cash", Kind: "expense", Category: "supplies", Total: 1200},
		{Method: "cash", Kind: "expense", Category: "salaries", Total: 800},
		// Every non-cash row must be ignored, however large.
		{Method: "transfer", Kind: "income", Category: "other_income", Total: 1000000},
		{Method: "credit_card", Kind: "expense", Category: "maintenance", Total: 1000000},
	}

	income, expense := CashIncomeAndExpense(totals)
	if income != 5000 {
		t.Errorf("want cash income 5000; got %d", income)
	}
	if expense != 2000 {
		t.Errorf("want cash expense 2000 (1200+800); got %d", expense)
	}
}

func TestCashIncomeAndExpenseWithNoCashRowsIsZero(t *testing.T) {
	totals := []MovementTotal{
		{Method: "transfer", Kind: "income", Category: "other_income", Total: 5000},
	}
	income, expense := CashIncomeAndExpense(totals)
	if income != 0 || expense != 0 {
		t.Errorf("want 0, 0; got %d, %d", income, expense)
	}
}

func TestCashIncomeAndExpenseIncludesAVoidByItsOwnOppositeKind(t *testing.T) {
	// A 1000 cash expense (supplies), voided: the void is a 1000 cash INCOME
	// row under the same category. Net effect: income and expense both show
	// 1000, and the two cancel out in expected cash (opening + income -
	// expense), exactly what the feature document's "voids included by their
	// kind" decision asks for.
	totals := []MovementTotal{
		{Method: "cash", Kind: "expense", Category: "supplies", Total: 1000},
		{Method: "cash", Kind: "income", Category: "supplies", Total: 1000},
	}
	income, expense := CashIncomeAndExpense(totals)
	if income != 1000 || expense != 1000 {
		t.Errorf("want income=1000 expense=1000 so they net to zero; got income=%d expense=%d", income, expense)
	}
}

func TestTranslateSessionWriteMapsTheOneOpenSessionConstraint(t *testing.T) {
	err := translateSessionWrite(&pgconn.PgError{ConstraintName: oneOpenSessionIndex})
	if !errors.Is(err, ErrSessionAlreadyOpen) {
		t.Errorf("want ErrSessionAlreadyOpen; got %v", err)
	}
}

func TestTranslateSessionWriteMapsTheClosedImmutableConstraint(t *testing.T) {
	err := translateSessionWrite(&pgconn.PgError{ConstraintName: sessionClosedImmutable})
	if !errors.Is(err, ErrSessionNotOpen) {
		t.Errorf("want ErrSessionNotOpen; got %v", err)
	}
}

func TestTranslateSessionWriteLeavesAnUnknownConstraintUnchanged(t *testing.T) {
	original := &pgconn.PgError{ConstraintName: "some_other_constraint"}
	if got := translateSessionWrite(original); got != error(original) { //nolint:errorlint // exact identity is the point: an unrecognised error must pass through unwrapped.
		t.Errorf("want the original error returned unchanged; got %v", got)
	}
}

func TestTranslateSessionWritePassesThroughANonPgError(t *testing.T) {
	plain := errors.New("boom")
	if got := translateSessionWrite(plain); !errors.Is(got, plain) {
		t.Errorf("want the original error; got %v", got)
	}
}

func TestTranslateMovementWriteMapsTheVoidsUniqueConstraint(t *testing.T) {
	err := translateMovementWrite(&pgconn.PgError{ConstraintName: voidsMovementUnique})
	if !errors.Is(err, ErrAlreadyVoided) {
		t.Errorf("want ErrAlreadyVoided; got %v", err)
	}
}

func TestTranslateMovementWriteMapsTheVoidOfVoidConstraint(t *testing.T) {
	err := translateMovementWrite(&pgconn.PgError{ConstraintName: voidOfVoidConstraint})
	if !errors.Is(err, ErrVoidOfVoid) {
		t.Errorf("want ErrVoidOfVoid; got %v", err)
	}
}

func TestCashSessionIsOpen(t *testing.T) {
	open := &CashSession{}
	if !open.IsOpen() {
		t.Error("a session with no ClosedAt must report open")
	}

	closedAt := time.Now()
	closed := &CashSession{ClosedAt: &closedAt}
	if closed.IsOpen() {
		t.Error("a session with ClosedAt set must report closed")
	}
}

func TestCashMovementIsVoid(t *testing.T) {
	m := &CashMovement{}
	if m.IsVoid() {
		t.Error("a movement with no VoidsMovementID must not report as a void")
	}
	id := m.ID
	m.VoidsMovementID = &id
	if !m.IsVoid() {
		t.Error("a movement with VoidsMovementID set must report as a void")
	}
}
