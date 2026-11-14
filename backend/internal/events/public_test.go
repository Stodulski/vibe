package events

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

func visitor() Actor { return Actor{IP: "198.51.100.9"} }

func signUpInput() SignUpInput {
	return SignUpInput{FirstName: "Ana", LastName: "Diaz", Phone: "+5491100000000", Email: "ana@example.com"}
}

// S3: sign-up holds a spot at the event's price and opens a MercadoPago checkout
// that carries the registration, expires with the hold, and returns the person to
// the event page with their credential.
func TestSignUpHoldsASpotAndOpensTheCheckout(t *testing.T) {
	f := newFixture(t)

	got, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.store.signedUp) != 1 || f.store.signedUp[0].MaxHoldsPerIP != 3 || f.store.signedUp[0].IP == "" {
		t.Errorf("sign-up = %+v; want the per-address cap passed on", f.store.signedUp)
	}
	if len(f.checkout.input) != 1 {
		t.Fatalf("one preference expected; got %d", len(f.checkout.input))
	}
	in := f.checkout.input[0]
	if in.RegistrationID != got.Registration.ID || in.Amount != got.Total || in.MarketplaceFee != got.ServiceFee ||
		in.Amount != f.store.event.Price+got.ServiceFee {
		t.Errorf("preference = %+v total=%d", in, got.Total)
	}
	if in.ExpiresIn <= 0 || in.ExpiresIn > 15*time.Minute {
		t.Errorf("the checkout must expire with the hold; expires in %s", in.ExpiresIn)
	}
	if !strings.Contains(in.BackURLs.Success, "/c/vibe/events/"+f.store.event.ID.String()) ||
		!strings.Contains(in.BackURLs.Success, "token=") || strings.Contains(in.BackURLs.Failure, "token=") {
		t.Errorf("back urls = %+v", in.BackURLs)
	}
	if len(f.payments.inserted) != 1 || f.payments.inserted[0].RegistrationID == nil ||
		*f.payments.inserted[0].RegistrationID != got.Registration.ID || f.payments.inserted[0].Status != "unpaid" ||
		f.payments.inserted[0].MPPreferenceID == nil || *f.payments.inserted[0].MPPreferenceID != "pref-1" {
		t.Errorf("the checkout row = %+v", f.payments.inserted)
	}
	if got.Preference.InitPoint == "" || got.Token == "" {
		t.Errorf("result = %+v", got)
	}
}

// Email is optional everywhere; the phone is the required contact.
func TestSignUpWithoutAnEmailIsAccepted(t *testing.T) {
	f := newFixture(t)
	in := signUpInput()
	in.Email = ""
	got, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, in)
	if err != nil || got.Registration.Email != nil {
		t.Fatalf("registration = %+v (%v); want no email recorded", got, err)
	}
	if f.checkout.input[0].PayerEmail != "" {
		t.Error("no email, none sent to MercadoPago")
	}
}

// S4: capacity closes sign-up, and the person is told why.
func TestSignUpAnswersConflictWhenTheEventIsFullOrClosed(t *testing.T) {
	for err, want := range map[error]string{
		eventstore.ErrEventFull:         "the event is full",
		eventstore.ErrEventClosed:       "sign-up for this event is closed",
		eventstore.ErrAlreadyRegistered: "this phone is already signed up for the event",
	} {
		f := newFixture(t)
		f.store.signUpErr = err
		_, got := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput())
		var conflict *ConflictError
		if !errors.As(got, &conflict) || conflict.Message != want {
			t.Errorf("%v: got %v, want conflict %q", err, got, want)
		}
		if len(f.checkout.input) != 0 {
			t.Error("a refused sign-up opens no checkout")
		}
	}
}

// A venue that cannot take money or a blocked phone never holds a spot.
func TestSignUpRefusesBeforeHoldingASpot(t *testing.T) {
	f := newFixture(t)
	f.complexes.complex = complexstore.NewComplexForTest(f.complex.ID, nil, nil)
	f.complexes.complex.IsActive = true
	if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput()); !errors.Is(err, ErrMercadoPagoNotConnected) {
		t.Fatalf("want ErrMercadoPagoNotConnected; got %v", err)
	}

	f = newFixture(t)
	f.clients.blocked = true
	if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput()); !errors.Is(err, ErrClientBlocked) {
		t.Fatalf("want ErrClientBlocked; got %v", err)
	}
	if len(f.store.signedUp) != 0 {
		t.Error("nothing may be held when the sign-up is refused up front")
	}
}

func TestSignUpOfAnotherComplexsEventOrAnInactiveComplexIsNotFound(t *testing.T) {
	f := newFixture(t)
	f.store.event.ComplexID = uuid.New()
	if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput()); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want not found; got %v", err)
	}
	f = newFixture(t)
	f.complex.IsActive = false
	if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput()); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want not found; got %v", err)
	}
}

// When MercadoPago will not create the checkout the hold is released at once.
func TestSignUpReleasesTheSpotWhenTheCheckoutCannotBeCreated(t *testing.T) {
	f := newFixture(t)
	f.checkout.err = errors.New("mercadopago unavailable")

	_, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput())
	if !errors.Is(err, ErrCheckoutUnavailable) {
		t.Fatalf("want ErrCheckoutUnavailable; got %v", err)
	}
	if len(f.store.released) != 1 || f.store.participant[0] != eventstore.CancelledByParticipant {
		t.Errorf("the held spot must be released; got %v", f.store.released)
	}
	if len(f.payments.inserted) != 0 {
		t.Error("no checkout row for a checkout that does not exist")
	}
}

// signedUpAndPaid returns the credential of a confirmed spot made `age` ago for an
// event starting in `startsIn`.
func signedUpAndPaid(t *testing.T, f *fixture, age, startsIn time.Duration) string {
	t.Helper()
	start := time.Now().Add(startsIn)
	f.store.event.Date = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	f.store.event.StartTime = start.In(timezoneAR()).Format("15:04")
	reg := &eventstore.Registration{
		ID: uuid.New(), EventID: f.store.event.ID, ComplexID: f.complex.ID, FirstName: "Ana", Phone: "+5491100000000",
		Status: eventstore.RegistrationConfirmed, Price: 800_000, ServiceFee: 20_000, CreatedAt: time.Now().Add(-age),
	}
	f.store.remember(reg, "tok")
	return "tok"
}

func timezoneAR() *time.Location {
	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		return time.FixedZone("ART", -3*3600)
	}
	return loc
}

// S6: a participant cancelling follows the complex's cancellation policy as a
// booking does. The complex asks for 24 hours' notice and gives a 15-minute grace
// after signing up.
func TestParticipantCancelFollowsTheComplexCancellationPolicy(t *testing.T) {
	cases := []struct {
		name        string
		age         time.Duration
		startsIn    time.Duration
		wantRefund  bool
		wantOutcome paymentstore.RefundResult
	}{
		{"well ahead of the window is refunded", 48 * time.Hour, 72 * time.Hour, true, paymentstore.RefundIssued},
		{"inside the 24 hours and past the grace keeps the payment", 48 * time.Hour, 6 * time.Hour, false, paymentstore.RefundNotEligible},
		{"inside the 24 hours but within the grace after signing up is refunded", 5 * time.Minute, 6 * time.Hour, true, paymentstore.RefundIssued},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}
			token := signedUpAndPaid(t, f, tc.age, tc.startsIn)

			got, err := f.svc.CancelRegistration(t.Context(), visitor(), token)
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome.Result != tc.wantOutcome {
				t.Errorf("outcome = %s, want %s", got.Outcome.Result, tc.wantOutcome)
			}
			if refunded := len(f.payments.refunded) == 1; refunded != tc.wantRefund {
				t.Errorf("refund asked of MercadoPago = %v, want %v", refunded, tc.wantRefund)
			}
			if got.View.Registration.Status != eventstore.RegistrationCancelled || f.store.participant[0] != eventstore.CancelledByParticipant {
				t.Errorf("the spot must be released as the participant's own cancellation; got %+v", got.View.Registration)
			}
			if len(f.notify.cancelled) != 1 || f.notify.cancelled[0].ByOwner {
				t.Errorf("notices = %+v", f.notify.cancelled)
			}
			if !tc.wantRefund && !strings.Contains(f.notify.cancelled[0].RefundLine, "fuera de plazo") {
				t.Errorf("the person must be told the payment is kept; line = %q", f.notify.cancelled[0].RefundLine)
			}
		})
	}
}

// A spot still being paid has no money to return, and cancelling it frees the spot.
func TestCancellingAHeldSpotReleasesItWithoutARefundNotice(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput())
	if err != nil {
		t.Fatal(err)
	}
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundNone}

	cancelled, err := f.svc.CancelRegistration(t.Context(), visitor(), got.Token)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Outcome.Result != paymentstore.RefundNone || len(f.store.released) != 1 {
		t.Errorf("outcome = %+v released = %v", cancelled.Outcome, f.store.released)
	}
	if len(f.notify.cancelled) != 1 || f.notify.cancelled[0].RefundLine != "" {
		t.Errorf("a spot never paid carries no refund sentence; notices = %+v", f.notify.cancelled)
	}
}

func TestCancellingAnOverRegistrationOrAnUnknownTokenIsRefused(t *testing.T) {
	f := newFixture(t)
	token := signedUpAndPaid(t, f, time.Hour, 72*time.Hour)
	if _, err := f.svc.CancelRegistration(t.Context(), visitor(), token); err != nil {
		t.Fatal(err)
	}
	var state *StateError
	if _, err := f.svc.CancelRegistration(t.Context(), visitor(), token); !errors.As(err, &state) {
		t.Fatalf("a second cancel must be refused; got %v", err)
	}
	if _, err := f.svc.CancelRegistration(t.Context(), visitor(), "nope"); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("an unknown token is not found; got %v", err)
	}
}

func TestStatusReportsWhatCancellingWouldDo(t *testing.T) {
	f := newFixture(t)
	token := signedUpAndPaid(t, f, time.Hour, 6*time.Hour)
	view, err := f.svc.Status(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Cancellation.CanCancel || view.Cancellation.WithinWindow {
		t.Errorf("cancellation = %+v; want cancellable and out of window", view.Cancellation)
	}
}
