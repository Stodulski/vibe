//go:build integration

package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

func signUp(f *datatest.Fixture, ev *eventstore.Event, phone string) (*eventstore.Registration, error) {
	return signUpFrom(f, ev, phone, "203.0.113.7")
}

// signUpFrom is signUp from a given address. Sign-ups from one address queue on
// its advisory lock, so a race between payers needs one address each to reach
// the event's own lock.
func signUpFrom(f *datatest.Fixture, ev *eventstore.Event, phone, ip string) (*eventstore.Registration, error) {
	reg, _, err := f.Stores.Events.SignUp(f.Scoped(context.Background()), eventstore.SignUpInput{
		EventID: ev.ID, FirstName: "Ana", LastName: "Diaz", Phone: phone, ServiceFee: 20_000,
		IP: ip, MaxHoldsPerIP: 100,
	})
	return reg, err
}

// expireHold makes a hold's window end, as if its payment window passed.
func expireHold(t *testing.T, f *datatest.Fixture, regID uuid.UUID) {
	t.Helper()
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE open_play_registrations SET held_until = NOW() - interval '1 minute' WHERE id = $1`, regID); err != nil {
		t.Fatal(err)
	}
}

// pay writes the checkout row sign-up leaves and settles an approved payment
// against the registration, as the webhook does.
func pay(f *datatest.Fixture, reg *eventstore.Registration, mpID string) error {
	ctx := f.Scoped(context.Background())
	checkout := &paymentstore.Payment{
		RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee,
		Method: "mercadopago", Status: "unpaid",
	}
	if err := f.Stores.Payments.Insert(ctx, checkout); err != nil {
		return err
	}
	checkout.Status, checkout.MPPaymentID = "deposit_paid", &mpID
	return f.Stores.Payments.ConfirmRegistrationPayment(ctx, checkout, reg.ID)
}

func eventWithCapacity(t *testing.T, f *datatest.Fixture, capacity int) *eventstore.Event {
	t.Helper()
	e := newEvent(f, "19:00", 120)
	e.Capacity = capacity
	if err := f.Stores.Events.CreateEvent(f.Scoped(context.Background()), e, []uuid.UUID{f.CourtID}); err != nil {
		t.Fatal(err)
	}
	return e
}

// S3/S4: signing up holds a spot, and capacity closes the sign-up: the spots
// taken are the confirmed ones plus the holds still running.
func TestSignUpHoldsASpotAndCapacityClosesIt(t *testing.T) {
	f := datatest.Isolated(t)
	ev := eventWithCapacity(t, f, 2)

	first, err := signUp(f, ev, "+5491100000001")
	if err != nil || first.Status != eventstore.RegistrationHeld || first.Price != ev.Price {
		t.Fatalf("first sign-up = %+v (%v); want a held spot at the event's price", first, err)
	}
	if _, err := signUp(f, ev, "+5491100000002"); err != nil {
		t.Fatal(err)
	}
	if _, err := signUp(f, ev, "+5491100000003"); !errors.Is(err, eventstore.ErrEventFull) {
		t.Fatalf("a third person must be refused, the event is full; got %v", err)
	}
	if _, err := signUp(f, ev, "+5491100000001"); !errors.Is(err, eventstore.ErrAlreadyRegistered) {
		t.Fatalf("the same phone cannot hold two spots; got %v", err)
	}
	got, err := f.Stores.Events.GetEvent(f.Scoped(context.Background()), ev.ID)
	if err != nil || got.HeldCount != 2 || got.SpotsLeft() != 0 {
		t.Fatalf("event = %+v (%v); want 2 held and no spots left", got, err)
	}
}

// S4: two payers racing for the last spot cannot both get it. Every sign-up
// takes the event row lock before it counts, so exactly capacity of them win.
func TestTwoPayersRacingForTheLastSpotCannotExceedCapacity(t *testing.T) {
	f := datatest.Shared(t)
	ev := eventWithCapacity(t, f, 3)

	const people = 10
	var wg sync.WaitGroup
	results := make([]error, people)
	for i := range people {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = signUpFrom(f, ev, fmt.Sprintf("+54911000%04d", i), fmt.Sprintf("198.51.100.%d", i+1))
		}()
	}
	wg.Wait()

	won, full := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, eventstore.ErrEventFull):
			full++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if won != 3 || full != people-3 {
		t.Fatalf("%d won and %d were refused; want exactly the capacity (3) to win", won, full)
	}
}

// Payments confirming at the same moment hold to the same limit: a hold that
// ran out and was taken over cannot be confirmed over the new holder.
func TestAHoldThatRanOutCannotBeConfirmedOverSomeoneWhoTookTheSpot(t *testing.T) {
	f := datatest.Shared(t)
	ev := eventWithCapacity(t, f, 1)

	slow, err := signUp(f, ev, "+5491100000001")
	if err != nil {
		t.Fatal(err)
	}
	expireHold(t, f, slow.ID)

	taker, err := signUp(f, ev, "+5491100000002")
	if err != nil {
		t.Fatalf("with the first hold run out the spot is free again: %v", err)
	}
	if err := pay(f, slow, "mp-slow"); !errors.Is(err, paymentstore.ErrRegistrationUnavailable) {
		t.Fatalf("a late payment over a taken spot must be refused so it can be refunded; got %v", err)
	}
	if err := pay(f, taker, "mp-taker"); err != nil {
		t.Fatalf("the person holding the spot is confirmed: %v", err)
	}
	var confirmed int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM open_play_registrations WHERE event_id = $1 AND status = 'confirmed'`, ev.ID).Scan(&confirmed); err != nil || confirmed != 1 {
		t.Fatalf("confirmed = %d (%v); want exactly the one", confirmed, err)
	}
}

// A hold that ran out frees its spot for the next person, is closed by the
// sweep, and still confirms when its payment arrives while the event has room.
func TestAnUnpaidHoldReleasesItsSpotWhenItsWindowEnds(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 1)

	held, err := signUp(f, ev, "+5491100000001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signUp(f, ev, "+5491100000002"); !errors.Is(err, eventstore.ErrEventFull) {
		t.Fatalf("while the hold runs the event is full; got %v", err)
	}
	expireHold(t, f, held.ID)

	if got, _ := f.Stores.Events.GetEvent(ctx, ev.ID); got.SpotsLeft() != 1 || got.HeldCount != 0 {
		t.Fatalf("a hold past its window stops counting at once: %+v", got)
	}
	swept, err := f.Stores.Events.ExpireHolds(ctx)
	if err != nil || len(swept) != 1 || swept[0].ID != held.ID {
		t.Fatalf("the sweep must close the hold; got %+v (%v)", swept, err)
	}
	// Nobody took the spot, so the late payment is still honoured.
	if err := pay(f, held, "mp-late"); err != nil {
		t.Fatalf("a late payment with room left confirms: %v", err)
	}
	if got, _ := f.Stores.Events.GetRegistration(ctx, held.ID); got.Status != eventstore.RegistrationConfirmed {
		t.Fatalf("status = %s", got.Status)
	}
}

// The same person can sign up again after their own hold ran out.
func TestAPersonWhoseHoldRanOutCanSignUpAgain(t *testing.T) {
	f := datatest.Isolated(t)
	ev := eventWithCapacity(t, f, 2)
	again, err := signUp(f, ev, "+5491100000009")
	if err != nil {
		t.Fatal(err)
	}
	expireHold(t, f, again.ID)
	if _, err := signUp(f, ev, "+5491100000009"); err != nil {
		t.Fatalf("a person whose hold ran out can sign up again: %v", err)
	}
}

// One address cannot hold the whole event hostage with unpaid holds.
func TestOneAddressCannotHoldMoreThanItsShareOfSpots(t *testing.T) {
	f := datatest.Isolated(t)
	ev := eventWithCapacity(t, f, 50)
	ctx := f.Scoped(context.Background())
	for i := range 3 {
		_, _, err := f.Stores.Events.SignUp(ctx, eventstore.SignUpInput{
			EventID: ev.ID, FirstName: "A", LastName: "B", Phone: "+54911000000" + string(rune('1'+i)),
			IP: "198.51.100.1", MaxHoldsPerIP: 3,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _, err := f.Stores.Events.SignUp(ctx, eventstore.SignUpInput{
		EventID: ev.ID, FirstName: "A", LastName: "B", Phone: "+549110000009", IP: "198.51.100.1", MaxHoldsPerIP: 3,
	})
	if !errors.Is(err, eventstore.ErrHoldLimit) {
		t.Fatalf("want ErrHoldLimit; got %v", err)
	}
}

// Sign-up closes with the event: a cancelled event, or one that has begun.
func TestSignUpIsClosedForACancelledEvent(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 5)
	if _, _, err := f.Stores.Events.CancelEvent(ctx, ev.ID, f.UserID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := signUp(f, ev, "+5491100000001"); !errors.Is(err, eventstore.ErrEventClosed) {
		t.Fatalf("want ErrEventClosed; got %v", err)
	}
	if _, _, err := f.Stores.Events.CancelEvent(ctx, ev.ID, f.UserID, nil); !errors.Is(err, eventstore.ErrEventNotActive) {
		t.Fatalf("a second cancel answers ErrEventNotActive; got %v", err)
	}
}

// S5: cancelling an event ends every registration and names the paid ones, whose
// money then goes back through the refund lifecycle on a payment that has no booking.
func TestCancellingAnEventEndsEveryRegistrationAndNamesThePaidOnes(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 5)

	paid, _ := signUp(f, ev, "+5491100000001")
	if err := pay(f, paid, "mp-paid"); err != nil {
		t.Fatal(err)
	}
	holding, _ := signUp(f, ev, "+5491100000002")

	cancelled, owed, err := f.Stores.Events.CancelEvent(ctx, ev.ID, f.UserID, nil)
	if err != nil || cancelled.Status != eventstore.EventCancelled {
		t.Fatalf("CancelEvent = %+v (%v)", cancelled, err)
	}
	if len(owed) != 1 || owed[0] != paid.ID {
		t.Fatalf("only the paid registration is owed a refund; got %v", owed)
	}
	for _, id := range []uuid.UUID{paid.ID, holding.ID} {
		got, _ := f.Stores.Events.GetRegistration(ctx, id)
		if got.Status != eventstore.RegistrationCancelled || got.CancelledBy == nil || *got.CancelledBy != eventstore.CancelledByOwner {
			t.Errorf("registration %s = %+v; want cancelled by the owner", id, got)
		}
	}

	// The refund runs through the claim lifecycle on a payment with no booking.
	rows, err := f.Stores.Events.ListRegistrations(ctx, ev.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	var payment *eventstore.RegistrationPayment
	for _, r := range rows {
		if r.ID == paid.ID {
			payment = r.Payment
		}
	}
	if payment == nil || payment.Status != "deposit_paid" {
		t.Fatalf("the paid participant's payment must show on the list; got %+v", payment)
	}
	claim, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID)
	if err != nil || claim.BookingID != uuid.Nil || claim.RegistrationID == nil || *claim.RegistrationID != paid.ID {
		t.Fatalf("claim = %+v (%v); want it to name the registration and no booking", claim, err)
	}
	total, err := f.Stores.Payments.RecordRefundSuccess(ctx, *claim, 0)
	if err != nil || total != paid.Price+paid.ServiceFee {
		t.Fatalf("RecordRefundSuccess = %d (%v)", total, err)
	}
	if status, refunded := f.ReadPaymentState(t, payment.ID); status != "refunded" || refunded != total {
		t.Fatalf("payment = %s/%d", status, refunded)
	}
}

// A registration's payment lands in the same cash ledger as an online deposit.
func TestARegistrationPaymentCountsInTheComplexCash(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 5)
	reg, _ := signUp(f, ev, "+5491100000001")
	if err := pay(f, reg, "mp-1"); err != nil {
		t.Fatal(err)
	}

	summary, err := f.Stores.Bookings.GetPaymentSummary(ctx, f.ComplexID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if summary.ByMethod["mercadopago"] != reg.Price {
		t.Fatalf("the day's MercadoPago takings = %d; want the event price %d", summary.ByMethod["mercadopago"], reg.Price)
	}
}
