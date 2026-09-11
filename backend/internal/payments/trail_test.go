package payments

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
)

// only returns the single entry recorded under action, failing if the trail
// holds none or more than one. Everything below asserts on the entry's
// contents: a double that counted calls could not tell a correct entry from an
// empty one, and an empty entry is exactly what a handler recording the wrong
// scope or a nil value would produce.
func only(t *testing.T, f *fixture, action string) audit.Entry {
	t.Helper()
	found := f.audit.find(action)
	if len(found) != 1 {
		t.Fatalf("want exactly one %q audit entry; got %d (all: %v)", action, len(found), recordedActions(f))
	}
	return found[0]
}

func recordedActions(f *fixture) []string {
	var names []string
	for _, e := range f.audit.entries {
		names = append(names, e.Action)
	}
	return names
}

// assertScope checks the parts of an entry that decide whether anybody can
// ever read it: the trail is served scoped to one complex and filtered by
// entity type (internal/audit/handler.go), so an entry with the wrong scope is
// an entry nobody sees.
func assertScope(t *testing.T, e audit.Entry, complexID, bookingID uuid.UUID) {
	t.Helper()
	if e.EntityType != "booking" {
		t.Errorf("want entity type %q; got %q", "booking", e.EntityType)
	}
	if e.EntityID == nil || *e.EntityID != bookingID {
		t.Errorf("the entry must name the booking it is about; got %v, want %v", e.EntityID, bookingID)
	}
	if e.ComplexID == nil || *e.ComplexID != complexID {
		t.Errorf("the entry must be scoped to the complex whose money moved; got %v, want %v", e.ComplexID, complexID)
	}
	// Nothing in this module runs on an authenticated session or on a client
	// connection: the webhook body is worked off the durable inbox, and the
	// refund queue and intent sweep are cron jobs. An invented user or address
	// here would be a lie about who acted.
	if e.UserID != nil {
		t.Errorf("no money path here has an authenticated user; got %v", *e.UserID)
	}
	if e.IPAddress != "" {
		t.Errorf("no money path here has a client address; got %q", e.IPAddress)
	}
}

// Money arriving is the first half of the money path and it had no entry at
// all: the trail knew about bookings, courts, complexes and admin reads, and
// nothing about payments.
//
// Mutation-verified: delete the h.record call in processApprovedPayment and
// this test fails on the missing entry.
func TestAConfirmedPaymentIsRecorded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111
	f.complexes.complex = linkedComplex(complexID, sellerID)
	f.bookings.booking = booking
	f.clients.client = &data.Client{ID: booking.ClientID, FirstName: "Ana"}
	f.courts.court = &data.Court{ID: booking.CourtID, Name: "Court 1"}

	if err := f.handler.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("confirming an approved payment must not fail: %v", err)
	}

	entry := only(t, f, "payment_confirmed")
	assertScope(t, entry, complexID, booking.ID)

	got := value(t, entry)
	if got["actor"] != actorProvider {
		t.Errorf("want the actor named %q; got %v", actorProvider, got["actor"])
	}
	if got["mp_payment_id"] != "mp-123" {
		t.Errorf("want MercadoPago's reference for this money; got %v", got["mp_payment_id"])
	}
	if f.payments.inserted == nil {
		t.Fatal("the fixture must record a payment row, or the assertion below proves nothing")
	}
	if got["payment_id"] != f.payments.inserted.ID.String() {
		t.Errorf("want the payment row this confirmed; got %v, want %v", got["payment_id"], f.payments.inserted.ID)
	}
	// The sum the client was actually charged, read off the committed row
	// rather than off whatever the handler was still holding.
	want := f.payments.inserted.Amount + f.payments.inserted.ServiceFee
	if got["amount_centavos"] != float64(want) {
		t.Errorf("want the amount that settled, %d; got %v", want, got["amount_centavos"])
	}
	if got["result"] != "confirmed" {
		t.Errorf("want result %q; got %v", "confirmed", got["result"])
	}
}

// A confirmation the database refused is retried, so recording it would put a
// payment in the trail that never happened.
//
// Mutation-verified: move the h.record call in processApprovedPayment above
// the ConfirmWebhookPayment/InsertAndConfirmBooking block and this fails.
func TestAConfirmationTheDatabaseRefusedIsNotRecorded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111
	f.complexes.complex = linkedComplex(complexID, sellerID)
	f.bookings.booking = booking
	f.payments.insertErr = errDatabase

	if err := f.handler.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err == nil {
		t.Fatal("a refused write must come back as an error so the event is retried")
	}

	if entries := f.audit.find("payment_confirmed"); len(entries) != 0 {
		t.Errorf("a confirmation that did not commit must not be recorded as one that did; got %d entries", len(entries))
	}
}

// One entry per refund attempt, covering the whole answer space
// AutoRefundIfPaid can produce — including the two outcomes that need a person
// to act on them, which are the ones somebody comes looking for afterwards.
//
// The wrapper is what makes this hold for all of them: the exits are spread
// across four functions, and a per-exit record would go silent the next time
// one is added.
//
// Mutation-verified: delete the h.record call in AutoRefundIfPaid and every
// case fails.
func TestEveryRefundOutcomeReachesTheTrail(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(*fixture, *data.Booking, *data.Payment)
		want       data.RefundResult
		wantAmount int
		wantReason bool
	}{
		{
			name:       "MercadoPago accepted the refund",
			prepare:    func(*fixture, *data.Booking, *data.Payment) {},
			want:       data.RefundIssued,
			wantAmount: 150_000,
		},
		{
			name:       "the booking was never paid",
			prepare:    func(_ *fixture, b *data.Booking, _ *data.Payment) { b.CollectionStatus = data.CollectionStatusUnpaid },
			want:       data.RefundNone,
			wantReason: true,
		},
		{
			name: "the booking was paid in cash, so a person has to return it",
			prepare: func(_ *fixture, _ *data.Booking, p *data.Payment) {
				p.MPPaymentID = nil
				p.Method = "cash"
			},
			want:       data.RefundManual,
			wantAmount: 150_000,
			wantReason: true,
		},
		{
			name: "MercadoPago rejected the refund, so it is queued",
			prepare: func(f *fixture, _ *data.Booking, _ *data.Payment) {
				f.provider.refundErr = errProvider
			},
			want:       data.RefundQueued,
			wantAmount: 150_000,
			wantReason: true,
		},
		{
			name: "the retry budget is spent and nothing automatic is left",
			prepare: func(f *fixture, _ *data.Booking, _ *data.Payment) {
				f.provider.refundErr = errProvider
				f.payments.exhausted = true
			},
			want:       data.RefundManual,
			wantAmount: 150_000,
			wantReason: true,
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
			f.clients.client = &data.Client{ID: booking.ClientID, FirstName: "Ana"}
			tt.prepare(f, booking, payment)

			outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

			entry := only(t, f, "refund")
			assertScope(t, entry, complexID, booking.ID)

			got := value(t, entry)
			if got["actor"] != actorSystem {
				t.Errorf("this system issued the refund, so the actor is %q; got %v", actorSystem, got["actor"])
			}
			if got["result"] != string(tt.want) {
				t.Errorf("want result %q; got %v", tt.want, got["result"])
			}
			// The entry and the answer the caller acts on must agree, or the
			// trail describes a different refund from the one that happened.
			if got["result"] != string(outcome.Result) {
				t.Errorf("the entry disagrees with the outcome returned: %v vs %q", got["result"], outcome.Result)
			}
			if got["amount_centavos"] != float64(tt.wantAmount) {
				t.Errorf("want amount %d; got %v", tt.wantAmount, got["amount_centavos"])
			}
			if tt.wantReason && got["reason"] == "" {
				t.Error("an outcome that is not the happy path must say why, in the trail as well as in the log")
			}
		})
	}
}

// The retry queue is where a refund finally succeeds or finally runs out of
// attempts, and an exhausted attempt is money owed with nothing automatic left
// behind it.
//
// Mutation-verified: delete either h.record call in RetryFailedRefunds and the
// matching case fails.
func TestTheRetryQueueRecordsHowEachAttemptEnded(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(*fixture)
		want       data.RefundResult
		wantAmount int
	}{
		{
			name:       "the retry succeeded",
			prepare:    func(*fixture) {},
			want:       data.RefundIssued,
			wantAmount: 150_000,
		},
		{
			name: "the provider rejected it again and it stays queued",
			prepare: func(f *fixture) {
				f.provider.refundErr = errProvider
			},
			want:       data.RefundQueued,
			wantAmount: 150_000,
		},
		{
			name: "the attempt ran out of retries",
			prepare: func(f *fixture) {
				f.provider.refundErr = errProvider
				f.payments.exhausted = true
			},
			want:       data.RefundManual,
			wantAmount: 150_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			f.failedRefunds.pending = []*data.FailedRefund{{
				ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
				PaymentID: payment.ID, Amount: 150_000, MPPaymentID: "mp-123",
			}}
			f.payments.byBooking = payment
			f.bookings.booking = booking
			f.clients.client = &data.Client{ID: booking.ClientID, FirstName: "Ana"}
			f.complexes.complex = linkedComplex(complexID, "")
			tt.prepare(f)

			f.handler.RetryFailedRefunds(t.Context())

			entry := only(t, f, "refund_retry")
			assertScope(t, entry, complexID, booking.ID)

			got := value(t, entry)
			if got["actor"] != actorSystem {
				t.Errorf("the retry job is this system; want actor %q, got %v", actorSystem, got["actor"])
			}
			if got["result"] != string(tt.want) {
				t.Errorf("want result %q; got %v", tt.want, got["result"])
			}
			if got["amount_centavos"] != float64(tt.wantAmount) {
				t.Errorf("want amount %d; got %v", tt.wantAmount, got["amount_centavos"])
			}
			if got["payment_id"] != payment.ID.String() {
				t.Errorf("want the payment the attempt is against; got %v", got["payment_id"])
			}
			if got["mp_payment_id"] != "mp-123" {
				t.Errorf("want MercadoPago's reference; got %v", got["mp_payment_id"])
			}
		})
	}
}

// A chargeback and a buyer-initiated refund are both money leaving the venue's
// account without anybody here deciding it, and they have very different
// consequences — so the trail has to tell them apart, and has to say that
// MercadoPago did it rather than this system.
//
// Mutation-verified: make providerRefundAction return "refund_external"
// unconditionally and the chargeback case fails.
func TestMoneyMercadoPagoTookBackIsRecordedAsItsOwnDoing(t *testing.T) {
	tests := []struct {
		name       string
		mpStatus   string
		wantAction string
	}{
		{name: "a chargeback", mpStatus: "charged_back", wantAction: "chargeback"},
		{name: "a refund taken out through MercadoPago", mpStatus: "refunded", wantAction: "refund_external"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			payment.ServiceFee = 100_000 // total paid: 250_000
			f.bookings.booking = booking
			f.complexes.complex = &data.Complex{ID: complexID, Name: "Vibe"}
			f.clients.client = &data.Client{ID: booking.ClientID, FirstName: "Ana"}

			err := f.handler.processRefundedPayment(t.Context(), payment, &mp.Payment{
				ID: 123, Status: tt.mpStatus, TransactionAmount: 2500, TransactionAmountRefunded: 2500,
			}, "mp-123")
			if err != nil {
				t.Fatalf("recording the provider's refund must not fail: %v", err)
			}

			entry := only(t, f, tt.wantAction)
			assertScope(t, entry, complexID, booking.ID)

			got := value(t, entry)
			if got["actor"] != actorProvider {
				t.Errorf("nobody here decided this; want actor %q, got %v", actorProvider, got["actor"])
			}
			if got["amount_centavos"] != float64(250_000) {
				t.Errorf("want MercadoPago's own figure for what moved; got %v", got["amount_centavos"])
			}
			if got["payment_id"] != payment.ID.String() {
				t.Errorf("want the payment this reverses; got %v", got["payment_id"])
			}
		})
	}
}

// The same event from the path that has no payment row: the refund arrived
// before this system ever recorded the payment it reverses. The entry must
// still be written — that is the case an owner is least able to reconstruct
// from anywhere else — and must not invent a payment id it does not have.
//
// Mutation-verified: delete the h.record call at the end of
// processRefundedPaymentFromBooking and this fails.
func TestARefundWithNoPaymentRowIsRecordedWithoutInventingOne(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	f.complexes.complex = &data.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &data.Client{ID: booking.ClientID, FirstName: "Ana"}

	err := f.handler.processRefundedPaymentFromBooking(t.Context(), booking, &mp.Payment{
		ID: 123, Status: "charged_back", TransactionAmount: 2500,
	}, "mp-123")
	if err != nil {
		t.Fatalf("cancelling after an unrecorded refund must not fail: %v", err)
	}

	entry := only(t, f, "chargeback")
	assertScope(t, entry, complexID, booking.ID)

	got := value(t, entry)
	if got["actor"] != actorProvider {
		t.Errorf("want actor %q; got %v", actorProvider, got["actor"])
	}
	if _, present := got["payment_id"]; present {
		t.Errorf("there is no payment row on this path, so none may be recorded; got %v", got["payment_id"])
	}
	if got["amount_centavos"] != float64(250_000) {
		t.Errorf("want MercadoPago's figure for what was charged; got %v", got["amount_centavos"])
	}
	if got["reason"] == "" {
		t.Error("the entry must say why it names no payment row")
	}
}

// Money that lands for a booking that was already cancelled goes straight back
// through the same claim path, and never touches AutoRefundIfPaid — so its
// entry has to be written here or nowhere.
//
// Mutation-verified: delete the success-path h.record call in
// refundCancelledBookingPayment and this fails.
func TestARefundForAnAlreadyCancelledBookingIsRecorded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	booking.Status = "cancelled"
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111
	f.complexes.complex = linkedComplex(complexID, sellerID)
	f.bookings.booking = booking
	f.clients.client = &data.Client{ID: booking.ClientID, FirstName: "Ana"}

	if err := f.handler.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("refunding a cancelled booking's payment must not fail: %v", err)
	}

	if len(f.provider.refunds) != 1 {
		t.Fatalf("the fixture must actually issue the refund; got %v", f.provider.refunds)
	}
	entry := only(t, f, "refund")
	assertScope(t, entry, complexID, booking.ID)

	got := value(t, entry)
	if got["actor"] != actorSystem {
		t.Errorf("this system issued it; want actor %q, got %v", actorSystem, got["actor"])
	}
	if got["result"] != string(data.RefundIssued) {
		t.Errorf("want result %q; got %v", data.RefundIssued, got["result"])
	}
	if got["mp_payment_id"] != "mp-123" {
		t.Errorf("want MercadoPago's reference; got %v", got["mp_payment_id"])
	}
	// A payment that arrived for a cancelled booking is never a confirmation.
	if entries := f.audit.find("payment_confirmed"); len(entries) != 0 {
		t.Errorf("a cancelled booking's payment must not be recorded as a confirmation; got %d", len(entries))
	}
}

// TestAHandlerWithNoRecorderIsRefusedAtConstruction pins where a missing audit
// recorder is caught.
//
// It used to be caught at the first refund, as a nil dereference deep inside
// the money path — the integration fixture had drifted and nothing noticed until
// a test panicked. The alternative, making record nil-safe, is worse: an audit
// trail that silently drops entries still answers every query, and every answer
// is short. Refusing at construction is what makes "there is no entry" mean "it
// did not happen".
func TestAHandlerWithNoRecorderIsRefusedAtConstruction(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a handler built with no audit recorder must be refused, not left to " +
				"drop the money path's entries or panic on the first refund")
		}
	}()

	NewHandler(Dependencies{}, Config{})
}
