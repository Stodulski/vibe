//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// openCheckout writes the payment row sign-up leaves once MercadoPago created
// the checkout preference.
func openCheckout(t *testing.T, f *datatest.Fixture, reg *eventstore.Registration) {
	t.Helper()
	pref := "pref-" + reg.ID.String()
	if err := f.Stores.Payments.Insert(f.Scoped(context.Background()), &paymentstore.Payment{
		RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee,
		Method: "mercadopago", Status: "unpaid", MPPreferenceID: &pref,
	}); err != nil {
		t.Fatal(err)
	}
}

func dueChecks(t *testing.T, f *datatest.Fixture) []eventstore.PaymentCheck {
	t.Helper()
	due, err := f.Stores.Events.DuePaymentChecks(f.Scoped(context.Background()), 50)
	if err != nil {
		t.Fatal(err)
	}
	return due
}

// S6: every hold the sweep expires is marked due a payment check, with the
// expiry notice owed once MercadoPago holds no payment, checkout row or not
// (the row's write can fail after the checkout was created). A hold still
// inside its window is untouched.
func TestTheSweepMarksEveryExpiredHoldDueANotifyingPaymentCheck(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 4)

	withRow, err := signUp(f, ev, "+5491100000021")
	if err != nil {
		t.Fatal(err)
	}
	openCheckout(t, f, withRow)
	withoutRow, err := signUp(f, ev, "+5491100000022")
	if err != nil {
		t.Fatal(err)
	}
	live, err := signUp(f, ev, "+5491100000023")
	if err != nil {
		t.Fatal(err)
	}
	expireHold(t, f, withRow.ID)
	expireHold(t, f, withoutRow.ID)

	swept, err := f.Stores.Events.ExpireHolds(ctx)
	if err != nil || len(swept) != 2 {
		t.Fatalf("swept = %+v (%v); want the two holds past their window", swept, err)
	}
	due := dueChecks(t, f)
	if len(due) != 2 {
		t.Fatalf("due = %+v; want both expired holds", due)
	}
	for _, d := range due {
		if d.RegistrationID == live.ID || !d.Notify || d.Failures != 0 {
			t.Errorf("due check %+v; want an expired hold, notifying, no failures", d)
		}
		if got, _ := f.Stores.Events.GetRegistration(ctx, d.RegistrationID); got.Status != eventstore.RegistrationExpired {
			t.Errorf("registration %s status = %s, want expired", d.RegistrationID, got.Status)
		}
	}
}

// S6: a hold a new sign-up of the same phone expires is still checked, silently:
// the person is signing up again and is not told the old spot expired.
func TestAHoldExpiredByASignUpOfItsPhoneIsDueASilentPaymentCheck(t *testing.T) {
	f := datatest.Isolated(t)
	ev := eventWithCapacity(t, f, 4)
	old, err := signUp(f, ev, "+5491100000024")
	if err != nil {
		t.Fatal(err)
	}
	expireHold(t, f, old.ID)
	if _, err := signUp(f, ev, "+5491100000024"); err != nil {
		t.Fatal(err)
	}
	due := dueChecks(t, f)
	if len(due) != 1 || due[0].RegistrationID != old.ID || due[0].Notify {
		t.Fatalf("due = %+v; want the old hold, not notifying", due)
	}
}

// S6: a failed check is pushed back and counted, and a closed check is closed
// once: the second close reports it was not this call's, and a closed check
// can no longer be deferred.
func TestAPaymentCheckIsDeferredOnFailureAndClosedOnce(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 4)
	reg, err := signUp(f, ev, "+5491100000031")
	if err != nil {
		t.Fatal(err)
	}
	expireHold(t, f, reg.ID)
	if _, err := f.Stores.Events.ExpireHolds(ctx); err != nil {
		t.Fatal(err)
	}

	failures, err := f.Stores.Events.DeferPaymentCheck(ctx, reg.ID)
	if err != nil || failures != 1 {
		t.Fatalf("defer = %d (%v); want one failure counted", failures, err)
	}
	if due := dueChecks(t, f); len(due) != 0 {
		t.Fatalf("a deferred check is not due at once; due = %+v", due)
	}
	// Its new time comes.
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE open_play_registrations SET payment_check_due_at = NOW() - interval '1 second' WHERE id = $1`, reg.ID); err != nil {
		t.Fatal(err)
	}
	if due := dueChecks(t, f); len(due) != 1 || due[0].Failures != 1 {
		t.Fatalf("due = %+v; want the check again, carrying its failure", due)
	}

	if closed, err := f.Stores.Events.ClosePaymentCheck(ctx, reg.ID); err != nil || !closed {
		t.Fatalf("first close = %v (%v); want closed by this call", closed, err)
	}
	if closed, err := f.Stores.Events.ClosePaymentCheck(ctx, reg.ID); err != nil || closed {
		t.Fatalf("second close = %v (%v); want already closed", closed, err)
	}
	if _, err := f.Stores.Events.DeferPaymentCheck(ctx, reg.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("deferring a closed check = %v; want ErrRecordNotFound", err)
	}
	if due := dueChecks(t, f); len(due) != 0 {
		t.Fatalf("a closed check is not due; due = %+v", due)
	}
}

// S6: a late payment that confirmed the registration answered the question, so
// its check is never due even though nothing closed it.
func TestAConfirmedRegistrationIsNeverDueAPaymentCheck(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 4)
	reg, err := signUp(f, ev, "+5491100000041")
	if err != nil {
		t.Fatal(err)
	}
	expireHold(t, f, reg.ID)
	if _, err := f.Stores.Events.ExpireHolds(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pay(f, reg, "mp-late-confirm"); err != nil {
		t.Fatalf("a late payment with room confirms: %v", err)
	}
	if due := dueChecks(t, f); len(due) != 0 {
		t.Fatalf("a confirmed registration is not due a check; due = %+v", due)
	}
}
