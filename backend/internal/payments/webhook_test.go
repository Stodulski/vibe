package payments

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
)

// TestWebhookStillConfirmsABookingThroughThePlatformAppOwnerToken guards the
// one caller Phase 9's CreatePreference refusal must not touch:
// processPaymentWebhook calls GetPayment as mp.AsPlatform, deliberately,
// because MercadoPago's marketplace API lets the app owner read any payment
// under its own app_id and the seller who collected the money is not known
// until the payment itself says so. If a future edit "makes the four
// [mp.MPClient] methods consistent" and adds CreatePreference's refusal to
// GetPayment too, this test is the one that fails — payment confirmation,
// the step that marks a booking paid, would stop working entirely.
//
// It used to be an empty seller token, which meant the same thing to
// mp.MPClient without saying so anywhere. mp.Caller only moved the decision
// into the open; the assertion is still that the platform is who asks.
func TestWebhookStillConfirmsABookingThroughThePlatformAppOwnerToken(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.ExternalReference = booking.ID.String()
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111

	f.provider.payment = mpPayment
	f.bookings.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID, Name: "Vibe", Slug: "vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.provider.callers) == 0 || f.provider.callers[0] != mp.AsPlatform() {
		t.Fatalf("GetPayment must still be called as the platform; got %d call(s), first %+v",
			len(f.provider.callers), f.provider.callers)
	}
	if booking.Status != "confirmed" || booking.CollectionStatus != data.CollectionStatusDepositPaid {
		t.Errorf("the booking must be confirmed by the webhook path; got status=%q collection_status=%q", booking.Status, booking.CollectionStatus)
	}
	if f.payments.inserted == nil {
		t.Error("the payment must be recorded")
	}
}

// TestWebhookRequeuesWhenTheLinkTokenMintFails pins task 3.4: a failed mint
// for the confirmation-email's booking link token must not send that email,
// and must leave the webhook event retryable rather than marked processed —
// exactly the same shape TestAWebhookProcessingFailureLeavesTheEventRetryable
// covers for a database or provider failure.
func TestWebhookRequeuesWhenTheLinkTokenMintFails(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.ExternalReference = booking.ID.String()
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111

	f.provider.payment = mpPayment
	f.bookings.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID, Name: "Vibe", Slug: "vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.linkTokens.mintErr = errDatabase

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if w.Code != http.StatusOK {
		t.Fatalf("a recorded event must still be acknowledged; got %d", w.Code)
	}
	if len(f.webhookEvents.processed) != 0 {
		t.Error("an event whose confirmation-token mint failed must not be marked processed")
	}
	if len(f.webhookEvents.failed) != 1 {
		t.Fatalf("a failed mint must requeue the event; got %d", len(f.webhookEvents.failed))
	}
	if len(f.notify.confirmations) != 0 {
		t.Errorf("no confirmation email may be sent when the link token mint failed; got %d", len(f.notify.confirmations))
	}
}

// These tests are about one sentence: a 200 from this endpoint must mean the
// event is durably ours, not that the bytes arrived.
//
// The endpoint used to answer 200 unconditionally and then process in a detached
// goroutine where every failure was a bare log line. MercadoPago never redelivers
// what it has already been told is fine, so a single transient failure left the
// client's money captured, the booking pending until the expiry cron cancelled
// it, no refund, and one log line as the only trace.

// Nothing may be written for a request MercadoPago did not sign. The endpoint is
// unauthenticated and reachable by anyone, so recording before verifying would
// hand any caller on the internet a write into our database.
func TestAForgedWebhookIsNotRecorded(t *testing.T) {
	f := newFixture(t)
	f.provider.signatureErr = errProvider

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if len(f.webhookEvents.inserted) != 0 {
		t.Errorf("a forged webhook must not be recorded; %d rows written", len(f.webhookEvents.inserted))
	}
	if len(f.webhookEvents.claimed) != 0 || len(f.webhookEvents.processed) != 0 {
		t.Error("a forged webhook must not be claimed or processed")
	}
	if len(f.locks.attempts) != 0 {
		t.Error("an unverified webhook must not even reach the idempotency lock")
	}
	// MercadoPago retries anything that is not 2xx, and retrying a forged request
	// forever helps nobody. Nothing was written, so this 200 acknowledges a
	// request we dropped rather than one we took responsibility for.
	if w.Code != http.StatusOK {
		t.Errorf("want 200 so the provider stops retrying; got %d", w.Code)
	}
}

// severedConnection is a request body that fails partway, the way a reset or
// truncated connection does. Nothing else can produce that failure in a test:
// every ordinary reader either succeeds or is empty, which is exactly why the
// handler answering 200 to an unreadable body survived this long.
type severedConnection struct{ err error }

func (b severedConnection) Read([]byte) (int, error) { return 0, b.err }

// A body we could not read is not a delivery we can decide anything about, and
// the 200 that used to be answered here is how a payment gets lost: MercadoPago
// never resends what it has been told is fine, and nothing was recorded to retry
// from. It is the same defect the durable inbox fixed, one step earlier — before
// there is even a row to be durable about.
//
// The oversized body is the one read failure a redelivery cannot repair, and it
// is dropped on purpose rather than refused.
func TestAWebhookWhoseBodyCannotBeReadIsNotAcknowledgedAsHandled(t *testing.T) {
	tests := []struct {
		name     string
		body     func() *http.Request
		wantCode func(int) bool
		want     string
	}{
		{
			name: "the connection gave out mid-body",
			body: func() *http.Request {
				return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/",
					severedConnection{err: errors.New("connection reset by peer")})
			},
			wantCode: func(code int) bool { return code >= 500 },
			want:     "a 5xx so MercadoPago redelivers",
		},
		{
			name: "the body is over the size limit",
			body: func() *http.Request {
				return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/",
					strings.NewReader(strings.Repeat("a", webhookBodyLimit+1)))
			},
			// Nothing MercadoPago sends is a megabyte, and the same bytes would
			// come back on every retry, so this one is dropped deliberately.
			wantCode: func(code int) bool { return code == http.StatusOK },
			want:     "200, dropping bytes MercadoPago did not send",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)

			w := httptest.NewRecorder()
			f.handler.MercadoPagoWebhook(w, tt.body())

			if !tt.wantCode(w.Code) {
				t.Errorf("want %s; got %d", tt.want, w.Code)
			}
			// Either way the body never became anything: nothing may be recorded,
			// and nothing may be worked.
			if len(f.webhookEvents.inserted) != 0 {
				t.Errorf("an unread body must not be recorded; %d rows written", len(f.webhookEvents.inserted))
			}
			if len(f.locks.attempts) != 0 {
				t.Error("an unread body must not reach the idempotency lock")
			}
		})
	}
}

// Every request reaches the signature check, and nothing about a request is
// decided or said before it does.
//
// The handler used to return on a malformed body first, which meant a caller
// nobody had authenticated could steer it to an exit — and get its own bytes
// quoted back in an error log — without the trust boundary ever running. The
// boundary has to be the first thing that answers for a request, not the first
// thing that answers for the requests that happen to parse.
func TestNothingIsDecidedAboutAWebhookBeforeItsSignatureIsChecked(t *testing.T) {
	f := newFixture(t)
	f.provider.signatureErr = errProvider

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, `{"type":"payment","data":{`))

	if len(f.provider.verified) != 1 {
		t.Fatalf("a malformed body must still be taken to the signature check; got %d checks", len(f.provider.verified))
	}
	if strings.Contains(f.logs.String(), "will not parse") {
		t.Error("an unverified request's body must not be reported on: the signature check refused it, and that is the only thing to say")
	}
	if len(f.webhookEvents.inserted) != 0 {
		t.Errorf("nothing may be recorded for a request that failed the signature check; %d rows written", len(f.webhookEvents.inserted))
	}
	if w.Code != http.StatusOK {
		t.Errorf("want 200 so the provider stops retrying a forged request; got %d", w.Code)
	}
}

// The whole fix, in one assertion. If the event could not be recorded we must not
// tell MercadoPago we have it: a 200 here is how the payment gets lost, because
// the provider will never send it again.
func TestAWebhookThatCannotBeRecordedIsRefusedSoMercadoPagoRetries(t *testing.T) {
	f := newFixture(t)
	f.webhookEvents.insertErr = errDatabase

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if w.Code < 500 {
		t.Errorf("an event we failed to record must be refused with 5xx so MercadoPago redelivers it; got %d", w.Code)
	}
	// Processing an event we could not record would be the old defect wearing a
	// new coat: work happening with nothing durable behind it.
	if len(f.locks.attempts) != 0 {
		t.Error("nothing may be processed when the event was not recorded")
	}
	if f.payments.confirmed != nil || f.payments.inserted != nil {
		t.Error("no payment may be written for an event we never recorded")
	}
}

// A verified delivery is written and committed first, and only then acted on.
func TestAWebhookIsRecordedBeforeItIsProcessed(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if w.Code != http.StatusOK {
		t.Fatalf("a recorded event must be acknowledged; got %d", w.Code)
	}
	if len(f.webhookEvents.inserted) != 1 {
		t.Fatalf("the delivery must be recorded exactly once; got %d rows", len(f.webhookEvents.inserted))
	}

	recorded := f.webhookEvents.inserted[0]
	if recorded.Provider != mercadoPagoProvider {
		t.Errorf("the event must name its provider; got %q", recorded.Provider)
	}
	if recorded.ExternalID != "mp-123" {
		t.Errorf("the event must carry MercadoPago's data.id; got %q", recorded.ExternalID)
	}
	if recorded.EventType != "payment" {
		t.Errorf("the event must carry its type; got %q", recorded.EventType)
	}
	if string(recorded.Payload) != webhookBody {
		t.Errorf("the event must keep the body exactly as delivered; got %s", recorded.Payload)
	}

	recordedAt := f.trace.indexOf("record")
	processedAt := f.trace.indexOf("process")
	if recordedAt == -1 {
		t.Fatal("the event was never recorded")
	}
	if processedAt == -1 {
		t.Fatal("the recorded event was never processed")
	}
	if recordedAt > processedAt {
		t.Errorf("the event must be recorded before any processing is attempted; trace was %v", f.trace.calls)
	}

	if len(f.webhookEvents.claimed) != 1 {
		t.Errorf("the recorded event must be claimed before it is worked; got %v", f.webhookEvents.claimed)
	}
	if len(f.webhookEvents.processed) != 1 || f.webhookEvents.processed[0] != recorded.ID {
		t.Errorf("a delivery that was handled must be marked processed; got %v", f.webhookEvents.processed)
	}
}

// A failure while processing must leave the event queued for another attempt. The
// point of recording it is that a MercadoPago or Postgres blip costs a retry
// rather than a payment.
func TestAWebhookProcessingFailureLeavesTheEventRetryable(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*fixture)
	}{
		{
			name:    "MercadoPago cannot be reached",
			prepare: func(f *fixture) { f.provider.paymentErr = errProvider },
		},
		{
			name:    "the database is unavailable",
			prepare: func(f *fixture) { f.payments.mpIDErr = errDatabase },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.prepare(f)

			w := httptest.NewRecorder()
			f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

			// The event is already ours, so the acknowledgement stands.
			if w.Code != http.StatusOK {
				t.Fatalf("a recorded event must still be acknowledged; got %d", w.Code)
			}
			if len(f.webhookEvents.inserted) != 1 {
				t.Fatalf("the delivery must be recorded; got %d rows", len(f.webhookEvents.inserted))
			}
			if len(f.webhookEvents.processed) != 0 {
				t.Error("an event whose processing failed must not be marked processed")
			}
			if len(f.webhookEvents.failed) != 1 {
				t.Fatalf("a failed attempt must requeue the event, never drop it; got %d", len(f.webhookEvents.failed))
			}
			requeued := f.webhookEvents.failed[0]
			if requeued.id != f.webhookEvents.inserted[0].ID {
				t.Errorf("the requeued attempt names the wrong event: %v", requeued.id)
			}
			if requeued.cause == "" {
				t.Error("the requeued attempt must record why it failed")
			}
		})
	}
}

// The durable inbox deliberately does not deduplicate: MercadoPago sends several
// notifications for the same payment as its status moves, so each delivery is its
// own row. Deduplication stays where it already was — the advisory lock and the
// existing-payment check — and this test is what proves the inbox did not quietly
// take that job over or break it.
func TestDuplicateDeliveryIsRecordedTwiceButConfirmsOnce(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111
	mpPayment.ExternalReference = booking.ID.String()

	f.provider.payment = mpPayment
	f.bookings.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID, Name: "Vibe", Slug: "vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	first := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(first, webhookRequest(t, webhookBody))

	if f.payments.inserted == nil {
		t.Fatal("the first delivery must confirm the booking")
	}
	if len(f.notify.confirmations) != 1 {
		t.Fatalf("the first delivery must tell the client once; got %d", len(f.notify.confirmations))
	}
	confirmedPayment := f.payments.inserted

	// The payment is now on record, which is what the second delivery finds.
	f.payments.byMPID = confirmedPayment

	second := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(second, webhookRequest(t, webhookBody))

	if second.Code != http.StatusOK {
		t.Errorf("a duplicate delivery is still recorded and acknowledged; got %d", second.Code)
	}
	if len(f.webhookEvents.inserted) != 2 {
		t.Errorf("each delivery is its own row — the table is an inbox, not an idempotency key; got %d rows", len(f.webhookEvents.inserted))
	}
	if f.payments.inserted != confirmedPayment {
		t.Error("a duplicate delivery must not record the payment a second time")
	}
	if len(f.notify.confirmations) != 1 {
		t.Errorf("a duplicate delivery must not confirm the booking twice; got %d confirmations", len(f.notify.confirmations))
	}
	if len(f.webhookEvents.processed) != 2 {
		t.Errorf("both deliveries reached a decision and must be closed out; got %v", f.webhookEvents.processed)
	}
}

// A redelivery must leave exactly one payment row behind.
//
// MercadoPago delivers the same payment notification more than once, on its own
// schedule, and the sweeper replays any attempt that died after the money was
// already recorded. If confirmation is not idempotent against that, the second
// pass writes a second payment row: the first is orphaned carrying the
// preference id, and every later read of "the payment for this booking" gets
// whichever row the ordering happens to hand it — including the refund path,
// which computes what to send back from that row.
//
// Counting rows is the whole assertion. "No error" would pass against the bug,
// because writing a second row is not an error: it succeeds.
//
// The store stub mirrors the payments table's partial unique index on
// mp_payment_id (see stubPayments.rememberMPID), so the second delivery finds
// what the first one wrote, exactly as it would against Postgres.
func TestARedeliveredWebhookLeavesExactlyOnePaymentRow(t *testing.T) {
	tests := []struct {
		name string
		// status is what the booking is in when the first delivery lands.
		status string
		// wantInserts is how many payment rows the webhook may create. Zero when
		// the public booking flow already wrote the checkout row, which the
		// webhook settles in place rather than duplicating.
		wantInserts int
		prepare     func(*fixture, *data.Booking)
	}{
		{
			name:        "no checkout row: the webhook writes the only payment",
			status:      "pending",
			wantInserts: 1,
		},
		{
			name:        "a checkout row exists: the webhook settles it in place",
			status:      "pending",
			wantInserts: 0,
			prepare: func(f *fixture, b *data.Booking) {
				f.payments.byBooking = &data.Payment{
					ID: uuid.New(), BookingID: b.ID, ComplexID: b.ComplexID,
					Amount: b.DepositAmount, Method: "mercadopago", Status: "pending",
				}
			},
		},
		{
			name:        "the booking was already cancelled: the money is recorded once and sent back once",
			status:      "cancelled",
			wantInserts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, mpPayment := pendingBooking(complexID)
			booking.Status = tt.status
			mpPayment.ExternalReference = booking.ID.String()
			mpPayment.CollectorID = 111111111

			complex := linkedComplex(complexID, "111111111")
			complex.Name = "Vibe"
			complex.Slug = "vibe"

			f.provider.payment = mpPayment
			f.bookings.booking = booking
			f.complexes.complex = complex
			f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
			f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
			if tt.prepare != nil {
				tt.prepare(f, booking)
			}

			for delivery := 1; delivery <= 2; delivery++ {
				w := httptest.NewRecorder()
				f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))
				if w.Code != http.StatusOK {
					t.Fatalf("delivery %d: a recorded event must be acknowledged; got %d", delivery, w.Code)
				}
			}

			if got := len(f.payments.insertedRows); got != tt.wantInserts {
				t.Errorf("two deliveries of one payment must create %d payment row(s); got %d", tt.wantInserts, got)
			}
			if len(f.webhookEvents.inserted) != 2 {
				t.Errorf("each delivery is still its own inbox row; got %d", len(f.webhookEvents.inserted))
			}

			if tt.status == "cancelled" {
				// The money came back once. A second claim on the same payment is
				// a second refund of one capture.
				if got := len(f.payments.claimed); got != 1 {
					t.Errorf("a redelivery must not claim the refund again; got %d claims", got)
				}
				if got := len(f.provider.refunds); got != 1 {
					t.Errorf("a redelivery must not send the money back twice; got %d refunds", got)
				}
				return
			}
			if got := len(f.notify.confirmations); got != 1 {
				t.Errorf("a redelivery must not confirm the booking to the client twice; got %d", got)
			}
		})
	}
}

// The sweeper is the half of the fix the inline dispatch cannot provide: without
// it, an event recorded by a process that then died would be durably recorded and
// permanently forgotten.
func TestTheSweeperWorksEventsLeftBehind(t *testing.T) {
	f := newFixture(t)
	abandoned := &data.WebhookEvent{
		ID:         uuid.New(),
		Provider:   mercadoPagoProvider,
		ExternalID: "mp-123",
		EventType:  "payment",
		Payload:    json.RawMessage(webhookBody),
		Status:     "processing",
	}
	f.webhookEvents.pending = []*data.WebhookEvent{abandoned}

	f.handler.ProcessPendingWebhookEvents(t.Context())

	if len(f.webhookEvents.claimed) != 1 || f.webhookEvents.claimed[0] != abandoned.ID {
		t.Fatalf("the sweeper must claim what it picks up; got %v", f.webhookEvents.claimed)
	}
	if len(f.locks.attempts) != 1 || f.locks.attempts[0] != "mp_webhook:mp-123" {
		t.Errorf("the swept event must be processed under the same idempotency lock; got %v", f.locks.attempts)
	}
	if len(f.webhookEvents.processed) != 1 {
		t.Errorf("a swept event that was handled must be closed out; got %v", f.webhookEvents.processed)
	}
}

// Two workers seeing the same due row on the same tick: only the one that wins
// the claim may touch the payment.
func TestASweptEventAlreadyClaimedElsewhereIsLeftAlone(t *testing.T) {
	f := newFixture(t)
	f.webhookEvents.claimRefused = true
	f.webhookEvents.pending = []*data.WebhookEvent{{
		ID: uuid.New(), Provider: mercadoPagoProvider, ExternalID: "mp-123",
		EventType: "payment", Payload: json.RawMessage(webhookBody), Status: "pending",
	}}

	f.handler.ProcessPendingWebhookEvents(t.Context())

	if len(f.locks.attempts) != 0 {
		t.Error("an event another worker claimed must not be processed here as well")
	}
	if len(f.webhookEvents.processed) != 0 || len(f.webhookEvents.failed) != 0 {
		t.Error("an event we did not claim is not ours to close out")
	}
}
