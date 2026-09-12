package bookings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

var errDatabase = errors.New("database unavailable")

// The refund path writes 'refunded' onto a row it locks for itself, so the
// booking struct this handler is holding never learns about it. Reading the
// answer off that struct told every successfully refunded client that nothing
// had been refunded — and then the refund email arrived minutes later.
func TestPublicCancelReportsTheRefundThatActuallyHappened(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.refunds.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 250_000}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["refunded"] != true {
		t.Errorf(`a refunded cancellation must report "refunded": true; got %v`, body["refunded"])
	}
	refund, ok := body["refund"].(map[string]any)
	if !ok {
		t.Fatalf("the response must carry the refund outcome; got %v", body["refund"])
	}
	if refund["status"] != string(paymentstore.RefundIssued) {
		t.Errorf("want status %q; got %v", paymentstore.RefundIssued, refund["status"])
	}
	if refund["amount"] != float64(250_000) {
		t.Errorf("want the refunded amount; got %v", refund["amount"])
	}
	bookingBody, _ := body["booking"].(map[string]any)
	if bookingBody["refund_status"] != bookingstore.RefundStatusFull {
		t.Errorf("the refund status must reflect the row the refund wrote; got %v", bookingBody["refund_status"])
	}
}

// A split refund — the MercadoPago row auto-refunded, a cash row still owed
// by hand — is written refund_status 'partial' by the refund pipeline (the
// partial_refund state), and the cancellation response has to match what was actually
// persisted rather than reporting 'refunded' for a booking that still owes
// money. paymentStatusAfter's ManualAmountCentavos case has to run before its
// MoneyReturned() case, or this booking would report 'refunded' too — the
// same defect this whole function exists to close, just for the manual half.
func TestPublicCancelReportsPartialRefundWhenCashIsStillOwed(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.refunds.outcome = paymentstore.RefundOutcome{
		Result: paymentstore.RefundIssued, AmountCentavos: 150_000, ManualAmountCentavos: 350_000,
	}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	// The automatic half genuinely came back, so this legacy field stays true.
	if body["refunded"] != true {
		t.Errorf(`the automatic half came back, so "refunded" must stay true; got %v`, body["refunded"])
	}
	bookingBody, _ := body["booking"].(map[string]any)
	if bookingBody["refund_status"] != bookingstore.RefundStatusPartial {
		t.Errorf("the refund status must reflect the row the refund pipeline wrote; got %v", bookingBody["refund_status"])
	}
	refund, ok := body["refund"].(map[string]any)
	if !ok {
		t.Fatalf("the response must carry the refund outcome; got %v", body["refund"])
	}
	if refund["manual_amount"] != float64(350_000) {
		t.Errorf("the response must name the cash still owed; got %v", refund["manual_amount"])
	}
}

// Every outcome a cancellation can reach has to say something about the money.
// Six of them used to say nothing at all.
func TestEveryCancellationOutcomeTellsTheClientSomething(t *testing.T) {
	all := []paymentstore.RefundResult{
		paymentstore.RefundNone,
		paymentstore.RefundNotEligible,
		paymentstore.RefundIssued,
		paymentstore.RefundAlreadyIssued,
		paymentstore.RefundQueued,
		paymentstore.RefundManual,
	}

	for _, result := range all {
		t.Run(string(result), func(t *testing.T) {
			e := refundEnvelope(paymentstore.RefundOutcome{Result: result, AmountCentavos: 1000})

			if e["status"] != string(result) {
				t.Errorf("the machine-readable status must survive; got %v", e["status"])
			}
			message, _ := e["message"].(string)
			if message == "" {
				t.Errorf("outcome %q leaves the client with nothing to read", result)
			}
			if e["amount"] != 1000 {
				t.Errorf("the amount in question must be reported; got %v", e["amount"])
			}
		})
	}
}

// The refund decision is made three lines above the enqueue call, in the same
// function, and used not to travel with it: the client was told their booking
// was off and nothing about their deposit, then found out about the money — or
// did not — from a separate email that only fires when one is actually sent.
func TestPublicCancelTellsTheClientAboutTheirMoney(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", Slug: "vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, Phone: "+5491155551234"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1"}
	f.refunds.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 500_000}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.cancelled) != 1 {
		t.Fatalf("want one cancellation notification; got %d", len(f.notify.cancelled))
	}
	sent := f.notify.cancelled[0]

	if !strings.Contains(sent.RefundLine, "$5.000") {
		t.Errorf("the cancellation says nothing about the deposit; got refund line %q", sent.RefundLine)
	}
	if sent.RefundAmount != "$5.000" {
		t.Errorf("want the refund amount carried; got %q", sent.RefundAmount)
	}
	// The client gave a number, so WhatsApp has to be reachable: without a
	// phone on the payload the channel is not disabled, it is unrepresentable.
	if sent.Phone != "+5491155551234" {
		t.Errorf("the cancellation cannot reach WhatsApp; got phone %q", sent.Phone)
	}
	if sent.BookPath != "vibe/book" || sent.BookURL != "https://vibe.test/vibe/book" {
		t.Errorf("the cancellation offers no way back; got %q / %q", sent.BookPath, sent.BookURL)
	}
}

// refundNotice is what the cancellation email and the cancellation WhatsApp
// message quote, and it is the whole of what the client is told about their
// money — there is no envelope field beside it in either channel. Every
// outcome must therefore produce a sentence, and every sentence that is about
// an amount must name it.
func TestRefundNoticeNamesTheMoneyForEveryOutcome(t *testing.T) {
	for _, result := range []paymentstore.RefundResult{
		paymentstore.RefundNotEligible,
		paymentstore.RefundIssued,
		paymentstore.RefundAlreadyIssued,
		paymentstore.RefundQueued,
		paymentstore.RefundManual,
	} {
		t.Run(string(result), func(t *testing.T) {
			line, _ := refundNotice(paymentstore.RefundOutcome{Result: result, AmountCentavos: 500_000})
			if line == "" {
				t.Fatalf("outcome %q leaves the client with nothing to read", result)
			}
			if !strings.Contains(line, "$5.000") {
				t.Errorf("outcome %q does not name the money; got %q", result, line)
			}
			for _, forbidden := range []string{"reembolso", "enlace", "club"} {
				if strings.Contains(strings.ToLower(line), forbidden) {
					t.Errorf("outcome %q uses the forbidden word %q: %q", result, forbidden, line)
				}
			}
		})
	}

	t.Run("nothing was ever paid", func(t *testing.T) {
		line, amount := refundNotice(paymentstore.RefundOutcome{Result: paymentstore.RefundNone})
		if line == "" {
			t.Error("a cancellation with no payment still has to say so")
		}
		if amount != "" {
			t.Errorf("there is no money here to name; got %q", amount)
		}
	})

	// The automatic half came back and a cash balance is still owed by hand.
	// Dropping the second is how somebody comes to believe they were made
	// whole.
	t.Run("split outcome carries both halves", func(t *testing.T) {
		line, amount := refundNotice(paymentstore.RefundOutcome{
			Result:               paymentstore.RefundIssued,
			AmountCentavos:       500_000,
			ManualAmountCentavos: 350_000,
		})
		if !strings.Contains(line, "$5.000") || !strings.Contains(line, "$3.500") {
			t.Errorf("a split refund must name both halves; got %q", line)
		}
		if amount != "$5.000" {
			t.Errorf("the preview line quotes the automatic half; got %q", amount)
		}
	})

	// Money exists and is staying where it is. Naming an amount in the preview
	// line of an email about not getting it back reads as a promise.
	t.Run("an ineligible cancellation promises nothing in the preview", func(t *testing.T) {
		line, amount := refundNotice(paymentstore.RefundOutcome{Result: paymentstore.RefundNotEligible, AmountCentavos: 500_000})
		if !strings.Contains(line, "$5.000") {
			t.Errorf("the body still says which money is being kept; got %q", line)
		}
		if amount != "" {
			t.Errorf("the preview line must not read as a refund; got %q", amount)
		}
	})
}

// Expiring the MercadoPago preference is not a refund and must not be gated on
// the refund window. It was: an out-of-window cancellation freed the slot and
// left the checkout link live, so the court could be sold twice and the first
// client charged for hours somebody else already owned.
func TestAnOutOfWindowCancelStillExpiresTheCheckoutLink(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := outOfWindowBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	preferenceID := "pref-1"
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.payment = &paymentstore.Payment{BookingID: booking.ID, MPPreferenceID: &preferenceID}
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 — the booking is still cancelled; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.checkout.expired) != 1 || f.checkout.expired[0] != preferenceID {
		t.Errorf("the checkout link must be closed whether or not a refund is owed; got %v", f.checkout.expired)
	}
	if len(f.refunds.refunded) != 0 {
		t.Error("the refund policy itself is unchanged: outside the window nothing is refunded")
	}
}

// The other half of the same guard: a paid booking cancelled outside the window
// keeps its deposit, and the client is told so rather than told nothing.
func TestAnOutOfWindowCancelSaysTheDepositIsKept(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := outOfWindowBooking(complexID)
	booking.DepositAmount = 150_000
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	refund, ok := decode(t, w)["refund"].(map[string]any)
	if !ok {
		t.Fatalf("the response must carry the refund outcome; got %s", w.Body.String())
	}
	if refund["status"] != string(paymentstore.RefundNotEligible) {
		t.Errorf("a deposit kept by policy is not the same as no deposit at all; got %v", refund["status"])
	}
	if refund["message"] == "" || refund["message"] == nil {
		t.Error("the client must be told why their deposit is not coming back")
	}
}

// A cancellation frees the court everywhere except the one table PublicBook
// consults, because the checkout locks it took were never released outside its
// own failure paths.
func TestCancellingReleasesTheSlotsItWasHolding(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.DurationMinutes = 120 // crosses a 30-minute grid boundary, unlike the usual 90-minute booking
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	// A booking now takes one lock over its whole span, addressed by its own
	// start time, regardless of how long it runs.
	want := booking.CourtID.String() + "@" + booking.Date.Format("2006-01-02") + " 18:00"
	if len(f.locks.released) != 1 || f.locks.released[0] != want {
		t.Fatalf("the booking's held slot must be released; want [%q], got %v", want, f.locks.released)
	}
}

// A release the database refuses must not take the cancellation down with it:
// the lock's TTL is the backstop, and the booking is already cancelled.
func TestAFailedSlotReleaseDoesNotFailTheCancellation(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.locks.releaseErr = errDatabase
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
}

// cancel-info answered from the cancellation window alone, so a booking paid in
// cash was told it qualified for a refund the refund path then declined to make.
func TestCancelInfoDoesNotPromiseARefundItCannotIssue(t *testing.T) {
	mpID := "mp-123"
	tests := []struct {
		name             string
		collectionStatus string
		refundStatus     string
		payment          *paymentstore.Payment
		ledger           []*paymentstore.Payment
		paymentErr       error
		wantRefund       bool
		wantMethod       string
	}{
		{
			name:             "paid through MercadoPago",
			collectionStatus: bookingstore.CollectionStatusDepositPaid,
			refundStatus:     bookingstore.RefundStatusNone,
			payment:          &paymentstore.Payment{Status: "deposit_paid", MPPaymentID: &mpID},
			wantRefund:       true,
			wantMethod:       refundByMercadoPago,
		},
		{
			name:             "paid in cash",
			collectionStatus: bookingstore.CollectionStatusDepositPaid,
			refundStatus:     bookingstore.RefundStatusNone,
			payment:          &paymentstore.Payment{Status: "deposit_paid", Method: "cash"},
			wantRefund:       true,
			wantMethod:       refundByHand,
		},
		{
			name:             "never paid",
			collectionStatus: bookingstore.CollectionStatusUnpaid,
			refundStatus:     bookingstore.RefundStatusNone,
			wantRefund:       false,
			wantMethod:       refundNotApplicable,
		},
		{
			// The automatic half already came back; what remains is exactly
			// the cash/transfer balance only a person can return.
			name:             "already partially refunded, cash still owed",
			collectionStatus: bookingstore.CollectionStatusFullyPaid,
			refundStatus:     bookingstore.RefundStatusPartial,
			wantRefund:       true,
			wantMethod:       refundByHand,
		},
		{
			name:             "paid, and the payment cannot be read",
			collectionStatus: bookingstore.CollectionStatusDepositPaid,
			refundStatus:     bookingstore.RefundStatusNone,
			paymentErr:       errDatabase,
			wantRefund:       true,
			wantMethod:       refundByHand,
		},
		{
			// A deposit paid through MercadoPago plus a balance the owner
			// confirmed in cash: two payment rows on one booking. A person has
			// to act on the cash row regardless of the MercadoPago one, so
			// this answers refundByHand rather than refundByMercadoPago —
			// promising an automatic refund here is exactly the failure this
			// endpoint used to have.
			name:             "paid through MercadoPago and in cash",
			collectionStatus: bookingstore.CollectionStatusFullyPaid,
			refundStatus:     bookingstore.RefundStatusNone,
			ledger: []*paymentstore.Payment{
				{Status: "deposit_paid", MPPaymentID: &mpID},
				{Status: "deposit_paid", Method: "cash"},
			},
			wantRefund: true,
			wantMethod: refundByHand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := futureBooking(complexID)
			booking.CollectionStatus = tt.collectionStatus
			booking.RefundStatus = tt.refundStatus
			f.store.booking = booking
			f.linkResolver.booking = booking
			f.payments.payment = tt.payment
			f.payments.ledger = tt.ledger
			f.payments.getErr = tt.paymentErr
			f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
			f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

			w := httptest.NewRecorder()
			f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet,
				"/?token=test-token", ""))

			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
			}
			body := decode(t, w)
			if body["can_refund"] != tt.wantRefund {
				t.Errorf("can_refund: want %v, got %v", tt.wantRefund, body["can_refund"])
			}
			if body["refund_method"] != tt.wantMethod {
				t.Errorf("refund_method: want %q, got %v — this is the promise that has to match what the refund path can do",
					tt.wantMethod, body["refund_method"])
			}
		})
	}
}

// cancel-info's refund_amount is a preview of what the automatic refund path
// would actually pay if the client cancels right now — the deposit plus the
// service fee on the unrefunded MercadoPago row, exactly what
// AutoRefundIfPaid would send back. paid_amount reports the same figure here
// because nothing has been refunded yet.
func TestCancelInfoReportsTheRefundAndPaidAmountInsideTheWindow(t *testing.T) {
	mpID := "mp-123"
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.payment = &paymentstore.Payment{
		Status: "deposit_paid", MPPaymentID: &mpID, Amount: 150_000, ServiceFee: 12_000,
	}
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1", Sport: "padel", CourtType: "indoor"}

	w := httptest.NewRecorder()
	f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet,
		"/?token=test-token", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["can_refund"] != true {
		t.Fatalf("a paid booking inside the window must be refundable; got %v", body["can_refund"])
	}
	if body["refund_amount"] != float64(162_000) {
		t.Errorf("refund_amount: want the deposit plus the service fee (162000); got %v", body["refund_amount"])
	}
	if body["paid_amount"] != float64(162_000) {
		t.Errorf("paid_amount: want what was actually paid (162000); got %v", body["paid_amount"])
	}
}

// Outside the refund window the automatic refund path pays nothing, but the
// client still paid — so refund_amount must read 0 while paid_amount still
// tells them what they are giving up.
func TestCancelInfoReportsNoRefundButPaidAmountOutsideTheWindow(t *testing.T) {
	mpID := "mp-123"
	f := newFixture(t)
	complexID := uuid.New()
	booking := outOfWindowBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.payment = &paymentstore.Payment{
		Status: "deposit_paid", MPPaymentID: &mpID, Amount: 150_000, ServiceFee: 12_000,
	}
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1", Sport: "padel", CourtType: "indoor"}

	w := httptest.NewRecorder()
	f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet,
		"/?token=test-token", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["can_refund"] != false {
		t.Fatalf("outside the window nothing is refunded automatically; got %v", body["can_refund"])
	}
	if body["refund_amount"] != float64(0) {
		t.Errorf("refund_amount: want 0 outside the window; got %v", body["refund_amount"])
	}
	if body["paid_amount"] != float64(162_000) {
		t.Errorf("paid_amount: the client still paid this even though it won't come back; got %v", body["paid_amount"])
	}
}

// A booking nobody has paid for yet has neither money to return nor money
// already paid.
func TestCancelInfoReportsZeroAmountsForAnUnpaidBooking(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1", Sport: "padel", CourtType: "indoor"}

	w := httptest.NewRecorder()
	f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet,
		"/?token=test-token", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["refund_amount"] != float64(0) {
		t.Errorf("refund_amount: an unpaid booking has nothing to return; got %v", body["refund_amount"])
	}
	if body["paid_amount"] != float64(0) {
		t.Errorf("paid_amount: an unpaid booking has nothing paid; got %v", body["paid_amount"])
	}
}

// cancel-info must carry the same court and schedule detail the success page
// shows — sport, court type, the end as an instant, duration and, when the
// complex has one, its address — so the cancel page never has to fall back to
// a cached copy of the booking.
func TestCancelInfoCarriesTheSameCourtDetailAsTheSuccessPage(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{
		ID: complexID, Name: "Vibe", Address: "Av. Siempre Viva 742", CancellationHours: 24,
	}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1", Sport: "padel", CourtType: "outdoor"}

	w := httptest.NewRecorder()
	f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet,
		"/?token=test-token", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	bookingBody, ok := decode(t, w)["booking"].(map[string]any)
	if !ok {
		t.Fatalf("the response must carry a booking object; got %s", w.Body.String())
	}
	if bookingBody["sport"] != "padel" {
		t.Errorf("sport: want %q; got %v", "padel", bookingBody["sport"])
	}
	if bookingBody["court_type"] != "outdoor" {
		t.Errorf("court_type: want %q; got %v", "outdoor", bookingBody["court_type"])
	}
	wantEndsAt := booking.EndsAt.Format(time.RFC3339)
	if bookingBody["ends_at"] != wantEndsAt {
		t.Errorf("ends_at: want %q; got %v", wantEndsAt, bookingBody["ends_at"])
	}
	if bookingBody["duration_minutes"] != float64(90) {
		t.Errorf("duration_minutes: want 90; got %v", bookingBody["duration_minutes"])
	}
	if bookingBody["complex_address"] != "Av. Siempre Viva 742" {
		t.Errorf("complex_address: want %q; got %v", "Av. Siempre Viva 742", bookingBody["complex_address"])
	}
}

// The staff path refunds regardless of the window, and reports the outcome the
// owner has to act on.
func TestStaffCancelReportsARefundOnlyAPersonCanMake(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.refunds.outcome = paymentstore.RefundOutcome{
		Result: paymentstore.RefundManual, AmountCentavos: 150_000, Reason: "paid in cash",
	}

	w := httptest.NewRecorder()
	f.handler.Cancel(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	refund, ok := decode(t, w)["refund"].(map[string]any)
	if !ok {
		t.Fatalf("the owner must be told what the cancellation owes; got %s", w.Body.String())
	}
	if refund["status"] != string(paymentstore.RefundManual) {
		t.Errorf("want a manual refund; got %v", refund["status"])
	}
	if refund["amount"] != float64(150_000) {
		t.Errorf("the owner must be told how much they owe; got %v", refund["amount"])
	}
}

// The staff path refunds regardless of the complex's window, so most owner
// cancellations are immediately followed by real money moving — and the client
// used to be told only that their booking was off.
func TestStaffCancelTellsTheClientAboutTheirMoney(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.clients.client = &clientstore.Client{ID: booking.ClientID, Phone: "+5491155551234"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1"}
	f.refunds.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 500_000}

	// RequireComplexOwner puts the complex on the request, so that is the one
	// Cancel reads — not the store's.
	r := ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{}`)
	r = httpx.ContextSetComplex(r, &complexstore.Complex{
		ID: complexID, Name: "Vibe", Slug: "vibe", CancellationHours: 24,
	})

	w := httptest.NewRecorder()
	f.handler.Cancel(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.cancelled) != 1 {
		t.Fatalf("want one cancellation notification; got %d", len(f.notify.cancelled))
	}
	sent := f.notify.cancelled[0]

	if !strings.Contains(sent.RefundLine, "$5.000") {
		t.Errorf("the cancellation says nothing about the deposit; got refund line %q", sent.RefundLine)
	}
	if sent.RefundAmount != "$5.000" {
		t.Errorf("want the refund amount carried; got %q", sent.RefundAmount)
	}
	if sent.Phone != "+5491155551234" {
		t.Errorf("the cancellation cannot reach WhatsApp; got phone %q", sent.Phone)
	}
	if sent.BookPath != "vibe/book" || sent.BookURL != "https://vibe.test/vibe/book" {
		t.Errorf("the cancellation offers no way back; got %q / %q", sent.BookPath, sent.BookURL)
	}
}

// The reconciliation sweep (refund-intent-durability spec) selects only on
// the refund-intent marker, so a cancellation that decided no refund is owed
// must never carry one — the marker write and the refund dispatch read one
// shared owesRefund expression, not two, so the sweep cannot be told this
// booking is an orphan.
func TestAnOutOfWindowPublicCancelNeverSetsTheRefundIntentMarker(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := outOfWindowBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.updated) != 1 {
		t.Fatalf("want exactly one Update call; got %d", len(f.store.updated))
	}
	if f.store.updated[0].RefundIntentAt != nil {
		t.Errorf("an out-of-window cancellation must not set the refund-intent marker — the sweep would treat this booking as an orphan and refund money the venue is entitled to keep; got %v", *f.store.updated[0].RefundIntentAt)
	}
}

// The staff path refunds regardless of the window by product decision
// (actions.go's own comment on the asymmetry), so its refund-intent marker
// must be set unconditionally whenever the booking was paid — the same
// owesRefund expression the refund dispatch already uses unconditionally.
func TestStaffCancelAlwaysSetsTheRefundIntentMarkerWhenPaid(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.Cancel(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.updated) != 1 {
		t.Fatalf("want exactly one Update call; got %d", len(f.store.updated))
	}
	if f.store.updated[0].RefundIntentAt == nil {
		t.Error("a paid staff cancellation must set the refund-intent marker regardless of any window — the staff path refunds unconditionally by product decision")
	}
}

// outOfWindowBooking returns a booking two hours away against a 24-hour window,
// created long enough ago that the grace period has passed.
func outOfWindowBooking(complexID uuid.UUID) *bookingstore.Booking {
	b := futureBooking(complexID)
	now := time.Now().In(timezone.Argentina)
	b.Date = now
	b.StartTime = now.Add(2 * time.Hour).Format("15:04")
	b.CreatedAt = now.Add(-24 * time.Hour)
	return b
}

// The credential read used to be `if tok, err := complex.SellerAccessToken(); err == nil`,
// which threw the error on the floor. A stored credential that no longer
// decrypts then left sellerToken as "", and mp.UpdatePreferenceExpired read an
// empty seller token as "authenticate as the platform" — so the call went out as
// the wrong party, failed at MercadoPago, and the checkout link stayed payable
// on a booking that no longer exists. mp.Caller has since made that arm
// something a call site has to write down; expiryCaller still must not write it
// down for this one.
//
// The assertion is on who the call was made as, not merely on whether a call
// happened: the stub records it now, because a stub that discards its arguments
// cannot tell "refused" from "called as somebody else".
func TestAnUnreadableCredentialNeverExpiresThePreferenceAsThePlatform(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := outOfWindowBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	preferenceID := "pref-1"
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.payment = &paymentstore.Payment{BookingID: booking.ID, MPPreferenceID: &preferenceID}
	unreadable := complexstore.NewComplexWithUnreadableCredentialForTest(complexID)
	unreadable.Name = "Vibe"
	unreadable.CancellationHours = 24
	f.complexes.complex = unreadable
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	tr := withCapturedSentryEvents(t)

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("the booking is still cancelled; want 200, got %d (%s)", w.Code, w.Body.String())
	}
	for _, a := range f.checkout.attempts {
		if a.caller == mp.AsPlatform() {
			t.Errorf("an unreadable credential must not fall through to the platform; "+
				"UpdatePreferenceExpired was called as the platform for preference %q", a.preferenceID)
		}
	}
	if len(f.checkout.attempts) != 0 {
		t.Errorf("nothing may be sent to MercadoPago on a credential that cannot be read; got %d attempt(s)", len(f.checkout.attempts))
	}
	msg, found := findMessage(tr.messages(), "CREDENTIAL UNREADABLE")
	if !found {
		t.Fatalf("a checkout left payable on a cancelled booking must alert; captured %v", tr.messages())
	}
	if !strings.Contains(msg, preferenceID) || !strings.Contains(msg, complexID.String()) {
		t.Errorf("the alert must name the complex and the preference an operator has to close; got %q", msg)
	}
	if strings.Contains(msg, booking.ID.String()) {
		t.Errorf("specs/booking-link-credential forbids a resolved booking id reaching Sentry from a public route; got %q", msg)
	}
}

// One transient failure at MercadoPago used to be the whole story on the request
// path: a single attempt, a log line, and a live checkout. The cron that expires
// the same preferences has retried since it was written.
func TestAFailedPreferenceExpiryIsRetried(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := outOfWindowBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	preferenceID := "pref-1"
	sellerToken := "seller-token"
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.payment = &paymentstore.Payment{BookingID: booking.ID, MPPreferenceID: &preferenceID}
	complex := complexstore.NewComplexForTest(complexID, &sellerToken, nil)
	complex.Name = "Vibe"
	complex.CancellationHours = 24
	f.complexes.complex = complex
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.checkout.expireFailFirst = true

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.checkout.attempts) != 2 {
		t.Fatalf("a failed expiry must be retried once; got %d attempt(s)", len(f.checkout.attempts))
	}
	if len(f.checkout.expired) != 1 || f.checkout.expired[0] != preferenceID {
		t.Errorf("the retry must actually close the link; expired = %v", f.checkout.expired)
	}
	wantCaller := mustSeller(t, sellerToken)
	for i, a := range f.checkout.attempts {
		if a.caller != wantCaller {
			t.Errorf("attempt %d was not made as the complex's own seller (token %q)", i, sellerToken)
		}
	}
}

// When both attempts fail the link is still live, which is money about to be
// taken for hours somebody else now owns. Nothing above a log line said so.
func TestAPreferenceThatCannotBeExpiredAlerts(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := outOfWindowBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	preferenceID := "pref-1"
	sellerToken := "seller-token"
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.payment = &paymentstore.Payment{BookingID: booking.ID, MPPreferenceID: &preferenceID}
	complex := complexstore.NewComplexForTest(complexID, &sellerToken, nil)
	complex.Name = "Vibe"
	complex.CancellationHours = 24
	f.complexes.complex = complex
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.checkout.expireErr = errPreferenceExpiryUnavailable
	tr := withCapturedSentryEvents(t)

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("a failed expiry must not fail the cancellation; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.checkout.attempts) != 2 {
		t.Errorf("want the attempt and its retry; got %d", len(f.checkout.attempts))
	}
	msg, found := findMessage(tr.messages(), "PREFERENCE EXPIRATION FAILED")
	if !found {
		t.Fatalf("a checkout that could not be closed must alert; captured %v", tr.messages())
	}
	if !strings.Contains(msg, preferenceID) || !strings.Contains(msg, complexID.String()) {
		t.Errorf("the alert must name the complex and the preference; got %q", msg)
	}
	if strings.Contains(msg, booking.ID.String()) {
		t.Errorf("specs/booking-link-credential forbids a resolved booking id reaching Sentry from a public route; got %q", msg)
	}
}

// A client closing the tab while MercadoPago is slow is the ordinary case, not
// an exotic one. It cancels the request context, and every slot-lock release in
// this package used to run on that context: bookingstore.SlotLocks.ReleaseLock hands it
// to queryContext and then to pgx, so the DELETE failed with context.Canceled
// before it reached PostgreSQL.
//
// Nothing but AcquireLock reads slot_locks, so the result was invisible in every
// direction that gets looked at: the court read free in availability and in every
// staff view, and the only place that refused to sell it was the public booking
// page — for the whole SlotLockTTL, because of a client who did nothing wrong.
func TestSlotsAreReleasedEvenWhenTheClientDisconnected(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	ctx, disconnect := context.WithCancel(t.Context())
	// The tab closes while MercadoPago is being asked for the checkout, and the
	// checkout then fails. Both halves matter: the cancellation is what used to
	// swallow the release, and the failure is what makes the handler reach it.
	f.checkout.onCreate = disconnect
	f.checkout.err = errCheckoutUnavailable

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, httptest.NewRequestWithContext(ctx, http.MethodPost, "/",
		strings.NewReader(publicBookBody(complexID, courtID, 120))))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when the checkout cannot be created; got %d (%s)", w.Code, w.Body.String())
	}
	// A booking now takes one lock over its whole span, not one per fixed-size
	// chunk, regardless of how long the booking is.
	if len(f.locks.acquired) != 1 {
		t.Fatalf("the fixture must have held the slot before the failure; got %v", f.locks.acquired)
	}
	if len(f.locks.abandoned) != 0 {
		t.Errorf("a release that never reached the database leaves the court unsellable for the whole TTL; abandoned %v",
			f.locks.abandoned)
	}
	if len(f.locks.released) != 1 {
		t.Errorf("the held slot must be freed; released %v, abandoned %v", f.locks.released, f.locks.abandoned)
	}
}

// The same disconnect on the cancellation path. A client cancels, MercadoPago is
// slow returning the deposit, they close the tab — and the locks their own
// booking was holding stayed held.
func TestCancellingReleasesTheSlotsEvenWhenTheClientDisconnected(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.DurationMinutes = 120 // crosses a 30-minute grid boundary, unlike the usual 90-minute booking
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	ctx, disconnect := context.WithCancel(t.Context())
	f.refunds.onRefund = disconnect

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, httptest.NewRequestWithContext(ctx, http.MethodPost, "/",
		strings.NewReader(`{"token":"test-token"}`)))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.refunds.refunded) != 1 {
		t.Fatalf("the fixture must have gone through the refund path; got %v", f.refunds.refunded)
	}
	if len(f.locks.abandoned) != 0 {
		t.Errorf("the cancelled booking's slot was left locked; abandoned %v", f.locks.abandoned)
	}
	// One lock over the whole span, not one per fixed-size chunk.
	if len(f.locks.released) != 1 {
		t.Errorf("the booking's slot must be freed; released %v, abandoned %v",
			f.locks.released, f.locks.abandoned)
	}
}

// errCheckoutUnavailable stands in for MercadoPago failing to mint a checkout
// preference — the failure path that frees the slots the request was holding.
var errCheckoutUnavailable = errors.New("mercadopago checkout unavailable")

// The third disconnect on the same request. A client cancels an unpaid booking
// and closes the tab; expireCheckoutPreference began with a payment read on that
// same cancelled context, so it returned before it even knew the preference id,
// and the checkout link stayed payable on a booking that no longer exists.
func TestTheCheckoutLinkIsClosedEvenWhenTheClientDisconnected(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	preferenceID := "pref-1"
	sellerToken := "seller-token"
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.payment = &paymentstore.Payment{BookingID: booking.ID, MPPreferenceID: &preferenceID}
	complex := complexstore.NewComplexForTest(complexID, &sellerToken, nil)
	complex.Name = "Vibe"
	complex.CancellationHours = 24
	f.complexes.complex = complex
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	// The tab closes as soon as the booking's cancellation is written, which is
	// the step immediately before the checkout link is closed.
	ctx, disconnect := context.WithCancel(t.Context())
	f.store.onUpdate = disconnect

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, httptest.NewRequestWithContext(ctx, http.MethodPost, "/",
		strings.NewReader(`{"token":"test-token"}`)))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.checkout.expired) != 1 || f.checkout.expired[0] != preferenceID {
		t.Errorf("a cancelled booking must not keep a payable checkout link; expired %v, attempts %v",
			f.checkout.expired, f.checkout.attempts)
	}
}

// A staff booking with nothing collected must say so, on both channels. The
// confirmation used to carry no money at all: a client had no idea whether
// they owed the venue the whole price or nothing, and the owner's copy of it
// went out announcing a booking the owner had just entered themselves.
func TestStaffConfirmationSaysWhatIsOwedAndDoesNotEmailTheOwnerAboutTheirOwnEntry(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	// RequireComplexOwner puts the complex on the request, so that is the one
	// Create reads. This one has a written address and no coordinates: the
	// case that used to lose the WhatsApp channel outright.
	r := ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()},
		staffBookBodyWithPrice(courtID, "18:00", 90, 2_000_000))
	r = httpx.ContextSetComplex(r, &complexstore.Complex{
		ID: complexID, Name: "Vibe", Slug: "vibe",
		Address: "Av. Santa Fe 1200", City: "Buenos Aires", CancellationHours: 24,
	})

	w := httptest.NewRecorder()
	f.handler.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.confirmed) != 1 {
		t.Fatalf("the client must be told about the booking; got %d", len(f.notify.confirmed))
	}
	sent := f.notify.confirmed[0]

	// A staff booking is not a sale the owner needs an email about: they are
	// the person who just made it.
	if sent.Source != notifications.SourceStaffCreate {
		t.Errorf("the staff route must identify itself; got source %q", sent.Source)
	}
	if sent.DepositAmount != "$0" {
		t.Errorf("a booking with nothing collected must report a $0 deposit; got %q", sent.DepositAmount)
	}
	if sent.BalanceAmount != "$20.000" {
		t.Errorf("the confirmation must say what is owed at the venue; got %q", sent.BalanceAmount)
	}
	if sent.CancellationLine == "" {
		t.Error("the confirmation must state the cancellation rule")
	}
	// A complex with no coordinates used to lose the WhatsApp channel entirely.
	if sent.MapsQuery == "" {
		t.Error("a complex with only a written address must still get a map button")
	}
}
