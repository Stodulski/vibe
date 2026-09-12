package payments

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// AutoRefundIfPaid used to return nothing, so every caller had to guess. These
// cases are the whole answer space: each one is a different sentence to the
// client and a different instruction to the operator, and four of them used to be
// indistinguishable bare returns.
func TestAutoRefundNamesWhatBecameOfTheMoney(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*fixture, *data.Booking, *paymentstore.Payment)
		want    paymentstore.RefundResult
	}{
		{
			name:    "MercadoPago accepted the refund",
			prepare: func(*fixture, *data.Booking, *paymentstore.Payment) {},
			want:    paymentstore.RefundIssued,
		},
		{
			name: "the booking was never paid",
			prepare: func(_ *fixture, b *data.Booking, _ *paymentstore.Payment) {
				b.CollectionStatus = data.CollectionStatusUnpaid
			},
			want: paymentstore.RefundNone,
		},
		{
			name:    "the booking is already refunded",
			prepare: func(_ *fixture, b *data.Booking, _ *paymentstore.Payment) { b.RefundStatus = data.RefundStatusFull },
			want:    paymentstore.RefundAlreadyIssued,
		},
		{
			name:    "a refund for this booking is already in flight",
			prepare: func(_ *fixture, b *data.Booking, _ *paymentstore.Payment) { b.RefundStatus = data.RefundStatusPending },
			want:    paymentstore.RefundQueued,
		},
		{
			name: "the payment row is already refunded",
			prepare: func(_ *fixture, _ *data.Booking, p *paymentstore.Payment) {
				p.Status = "refunded"
				p.RefundAmount = p.Amount
			},
			want: paymentstore.RefundAlreadyIssued,
		},
		{
			name: "the booking was paid in cash",
			prepare: func(_ *fixture, _ *data.Booking, p *paymentstore.Payment) {
				p.MPPaymentID = nil
				p.Method = "cash"
			},
			want: paymentstore.RefundManual,
		},
		{
			name: "the booking reads as paid with no payment record",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.payments.bookingErr = data.ErrRecordNotFound
			},
			want: paymentstore.RefundManual,
		},
		{
			name: "the payment could not be read at all",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.payments.bookingErr = errDatabase
			},
			want: paymentstore.RefundManual,
		},
		{
			name: "the claim was refused by the database, so nothing is queued",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.payments.claimErr = errDatabase
			},
			want: paymentstore.RefundManual,
		},
		{
			name: "another claim already holds the refund",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.payments.claimErr = paymentstore.ErrRefundInFlight
			},
			want: paymentstore.RefundQueued,
		},
		{
			name: "the claim found the payment already refunded",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.payments.claimErr = paymentstore.ErrAlreadyRefunded
			},
			want: paymentstore.RefundAlreadyIssued,
		},
		{
			name: "MercadoPago rejected the refund",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.provider.refundErr = errProvider
			},
			want: paymentstore.RefundQueued,
		},
		{
			name: "the retry budget is spent",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.provider.refundErr = errProvider
				f.payments.exhausted = true
			},
			want: paymentstore.RefundManual,
		},
		{
			name: "the refund was issued but could not be recorded",
			prepare: func(f *fixture, _ *data.Booking, _ *paymentstore.Payment) {
				f.payments.successErr = errRecord
			},
			want: paymentstore.RefundQueued,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			f.payments.byBooking = payment
			f.complexes.complex = linkedComplex(complexID, "")
			f.complexes.complex.Name = "Vibe"
			f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
			tt.prepare(f, booking, payment)

			got := f.handler.AutoRefundIfPaid(t.Context(), booking)

			if got.Result != tt.want {
				t.Errorf("want outcome %q; got %q (reason %q)", tt.want, got.Result, got.Reason)
			}
			if got.Result != paymentstore.RefundIssued && got.Result != paymentstore.RefundNone && got.Reason == "" {
				t.Error("an outcome that is not the happy path must say why")
			}
		})
	}
}

// The branch that cost a client their cash. AutoRefundIfPaid returned here with
// no log, no alert and nothing queued, so a cancellation the client had been told
// qualified for a refund left no trace that money was owed.
func TestARefundThatCannotBeIssuedIsReportedRatherThanDropped(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.MPPaymentID = nil
	payment.Method = "cash"
	f.payments.byBooking = payment
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}

	outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

	if outcome.Result != paymentstore.RefundManual {
		t.Fatalf("cash owed back is a manual refund; got %q", outcome.Result)
	}
	if !outcome.NeedsAHuman() {
		t.Error("the outcome must say a person has to act")
	}
	if outcome.AmountCentavos != payment.Amount+payment.ServiceFee {
		t.Errorf("the outcome must name what is owed; want %d, got %d",
			payment.Amount+payment.ServiceFee, outcome.AmountCentavos)
	}
	logged := f.logs.String()
	if !strings.Contains(logged, "cannot issue") {
		t.Errorf("a refund nothing can issue must be logged as an error; got %q", logged)
	}
	if !strings.Contains(logged, booking.ID.String()) {
		t.Error("the log must name the booking whose money is owed")
	}
}

// MercadoPago sends its own webhook for a refund we asked for. Between the claim
// and the recording, that webhook sees a payment still reading 'refund_pending'
// and used to send the client a second copy of the same email.
func TestARefundWeIssuedIsNotAnnouncedTwice(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.ServiceFee = 100_000
	// The state a claim of ours leaves behind while the provider call is in flight.
	payment.Status = "refund_pending"
	f.bookings.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	err := f.handler.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmountRefunded: 2500,
	}, "mp-123")
	if err != nil {
		t.Fatalf("recording a refund we initiated must not fail: %v", err)
	}

	if len(f.notify.refunds) != 0 {
		t.Errorf("the claim that issued this refund tells the client, not the provider's echo of it; got %d notifications", len(f.notify.refunds))
	}
}

// A partial refund is money the client got back. The notification used to be
// gated on the refund being full, so they were told nothing at all.
func TestAPartialRefundStillTellsTheClient(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.ServiceFee = 100_000 // total paid: 250_000
	f.bookings.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	// MercadoPago refunded 1000.00 of the 2500.00 the client paid.
	err := f.handler.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmountRefunded: 1000,
	}, "mp-123")
	if err != nil {
		t.Fatalf("recording a partial refund must not fail: %v", err)
	}

	if len(f.notify.refunds) != 1 {
		t.Fatalf("a partial refund is still the client's money coming back; got %d notifications", len(f.notify.refunds))
	}
	if got := f.notify.refunds[0].Amount; got != "$1.000" {
		t.Errorf("the client must be told what actually came back; got %q", got)
	}
}

// A refund that arrives for a booking whose payment we never recorded cancels the
// booking and marks it refunded — and used to say nothing to anybody, because the
// amount had no payment row to come from. MercadoPago's own copy has it.
func TestARefundWithNoPaymentRecordStillTellsTheClient(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	err := f.handler.processRefundedPaymentFromBooking(t.Context(), booking, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmountRefunded: 2500,
	}, "mp-123")
	if err != nil {
		t.Fatalf("cancelling after an unrecorded refund must not fail: %v", err)
	}

	if len(f.notify.refunds) != 1 {
		t.Fatalf("the client must be told their money came back; got %d notifications", len(f.notify.refunds))
	}
	if got := f.notify.refunds[0].Amount; got != "$2.500" {
		t.Errorf("the amount must come from MercadoPago's own figure; got %q", got)
	}
}

// Money arriving for a cancelled booking is refunded for what the client was
// actually charged. It used to be reconstructed from booking.DepositAmount, so a
// deposit that had changed since checkout produced a payment row — and therefore
// a refund, since the claim reads the amount off that row — for money nobody paid.
//
// This also proves the amount-equality check is not extended to this branch
// (payment-collector-verification spec, "the amount-equality check is not
// extended"): a deposit that moved since checkout must not refuse a legitimate
// refund from the booking's own collector.
func TestACancelledBookingRefundsWhatTheClientActuallyPaid(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = "cancelled"
	booking.DepositAmount = 150_000 // what the booking says today
	sellerID := "111111111"
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID}

	// What MercadoPago says the client was charged: 3000.00, not the 2500.00 the
	// booking's current deposit plus service fee would imply.
	const paidPesos = 3000.0
	_ = f.handler.processApprovedPayment(t.Context(), booking,
		&mp.Payment{ID: 123, Status: "approved", TransactionAmount: paidPesos, CollectorID: 111111111}, "mp-123")

	if f.payments.inserted == nil {
		t.Fatal("the payment that arrived must be recorded before it is refunded")
	}
	got := f.payments.inserted.Amount + f.payments.inserted.ServiceFee
	if want := int(paidPesos * 100); got != want {
		t.Errorf("the recorded payment must total what the client paid; want %d, got %d", want, got)
	}
}

// A refund issued against the platform token would come out of the wrong
// account, and MercadoPago refusing it would look exactly like a network
// blip. sellerCredential used to collapse this into "" and let the caller
// fall back to it, logged only at Error and otherwise unremarkable; it now
// refuses before MercadoPago is ever called (seller-credential-integrity
// spec, "MISSING refuses on the first attempt with a credential-specific
// alert").
//
// Mutation: revert AutoRefundIfPaid to call the pre-slice-2 getSellerToken
// (falls back to "" and still calls RefundPayment) — this test must fail,
// because f.provider.refunds would then be non-empty.
func TestAMissingSellerTokenRefusesBeforeMercadoPagoIsCalled(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.payments.byBooking = payment
	// Connected once — a public booking cannot exist otherwise — but the
	// credentials are gone now: no mpAccessToken, so SellerAccessToken()
	// answers ErrMPNotConnected exactly as a disconnected venue would.
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}

	sentryEvents := withCapturedSentryEvents(t)

	outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.provider.refunds) != 0 || len(f.provider.callers) != 0 {
		t.Fatalf("a missing credential must refuse before MercadoPago is called; got refunds=%v tokens=%v",
			f.provider.refunds, f.provider.callers)
	}
	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("the refusal must be recorded against the already-committed claim; got %d", len(f.payments.recordedFailure))
	}
	if cause := f.payments.recordedFailure[0].cause; !strings.Contains(cause, "seller credential missing") {
		t.Errorf("the cause must name the credential as missing, not confuse it with unreadable or unavailable; got %q", cause)
	}
	if outcome.Result != paymentstore.RefundQueued {
		t.Errorf("a first-attempt refusal is queued, not exhausted; got %q", outcome.Result)
	}

	logged := f.logs.String()
	if !strings.Contains(logged, "level=ERROR") || !strings.Contains(logged, "reason=MISSING") {
		t.Errorf("a credential refusal must be reported as an error naming the arm; got %q", logged)
	}

	messages := sentryEvents.messages()
	alert, found := findMessage(messages, "SELLER CREDENTIAL")
	if !found {
		t.Fatalf("a credential refusal must alert on this same attempt; captured %q", messages)
	}
	if !strings.Contains(alert, "MISSING") || !strings.Contains(alert, complexID.String()) {
		t.Errorf("the alert must name both the arm and the complex, or the operator cannot act on it; got %q", alert)
	}
}

// The guard on 'refund_pending' used to suppress the notification and let the
// write run anyway. It cannot: while that status stands, a claim of ours holds
// this refund and RecordRefundSuccess is coming to close it — with different
// arithmetic on the same column. One refund, one owner, and the owner is the
// claim.
func TestTheWebhookLeavesTheRecordToTheClaimThatHoldsIt(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.ServiceFee = 100_000 // total paid: 250_000
	payment.Status = "refund_pending"
	f.bookings.booking = booking
	f.payments.byBooking = payment
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	err := f.handler.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmount: 2500, TransactionAmountRefunded: 1000,
	}, "mp-123")
	if err != nil {
		t.Fatalf("standing down is not a failure: %v", err)
	}

	if f.payments.updated != nil {
		t.Errorf("the payment row belongs to the claim in flight; the webhook wrote refund_amount=%d status=%q",
			f.payments.updated.RefundAmount, f.payments.updated.Status)
	}
	if f.payments.confirmed != nil {
		t.Errorf("the webhook committed a payment+booking transaction on a claimed refund; status=%q",
			f.payments.confirmed.Status)
	}
	if f.bookings.updated != nil {
		t.Error("the booking's money state belongs to whoever records the claim")
	}
	if len(f.realtime.published) != 0 {
		t.Error("nothing changed here, so nothing may be broadcast as changed")
	}
	if len(f.notify.refunds) != 0 {
		t.Errorf("the claim tells the client, not the provider's echo; got %d", len(f.notify.refunds))
	}
	// The in-memory struct must be untouched too: the caller goes on using it.
	if payment.Status != "refund_pending" || payment.RefundAmount != 0 {
		t.Errorf("the webhook mutated the payment it was handed: status=%q refund_amount=%d",
			payment.Status, payment.RefundAmount)
	}
}

// What that write cost when it ran. MercadoPago moved 1000 of the 2500 paid and
// told us so twice — once as the response to our own refund call, once as the
// webhook. The webhook assigned refund_amount = 1000; RecordRefundSuccess then
// added its own 1000 to it. The row read 2000 back on a payment that returned
// half of that, so the 1500 still genuinely owed showed as 500 — and the client
// was told a number that never moved.
func TestAPartialRefundIsNotCountedTwiceWhenTheWebhookRacesTheRecorder(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.ServiceFee = 100_000 // total paid: 250_000
	payment.Status = "refund_pending"
	f.bookings.booking = booking
	f.payments.byBooking = payment
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	const moved = 100_000 // 1000.00, what MercadoPago actually returned

	// The provider's webhook lands first, while our claim is still in flight.
	if err := f.handler.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmount: 2500, TransactionAmountRefunded: 1000,
	}, "mp-123"); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	// Then the claim's own recorder closes the attempt out, adding its figure to
	// whatever the row holds — exactly as internal/data/refunds.go does.
	refundTotal, err := f.payments.RecordRefundSuccess(t.Context(), paymentstore.RefundClaim{
		PaymentID: payment.ID, BookingID: booking.ID, ComplexID: complexID,
		MPPaymentID: "mp-123", RefundCentavos: moved,
	}, 0)
	if err != nil {
		t.Fatalf("recording the claim: %v", err)
	}

	if refundTotal != moved {
		t.Errorf("the row must hold what MercadoPago moved (%d); it holds %d", moved, refundTotal)
	}
	if payment.RefundAmount != moved {
		t.Errorf("refund_amount = %d, want %d — the same money counted once", payment.RefundAmount, moved)
	}
	if payment.Status == "refunded" {
		t.Error("a payment that returned 1000 of 2500 is not refunded")
	}
}
