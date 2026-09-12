package payments

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// captureTransport is a sentry.Transport that records every event handed to
// it instead of sending anything over the network. sentry.Client.processEvent
// calls Transport.SendEvent synchronously (no background worker involved),
// so an event recorded here is visible to the test immediately after the
// call to sentry.CaptureMessage returns.
//
// internal/bookings has the same double in its own sentry_capture_test.go:
// test helpers do not cross package boundaries, and neither package should
// grow a non-test dependency just to share one.
type captureTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (c *captureTransport) Flush(_ time.Duration) bool              { return true }
func (c *captureTransport) FlushWithContext(_ context.Context) bool { return true }
func (c *captureTransport) Configure(_ sentry.ClientOptions)        {}
func (c *captureTransport) Close()                                  {}
func (c *captureTransport) SendEvent(event *sentry.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

// messages returns the message of every captured event, so a test can assert
// on what an operator would actually read rather than on a bare count.
func (c *captureTransport) messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.events))
	for _, e := range c.events {
		out = append(out, e.Message)
	}
	return out
}

// withCapturedSentryEvents binds a client backed by captureTransport to the
// current hub for the duration of the test, and restores whatever client was
// bound before (nil in every test today, since nothing else in this suite
// initializes Sentry). It returns the transport so the test can assert on
// what was captured.
func withCapturedSentryEvents(t *testing.T) *captureTransport {
	t.Helper()

	tr := &captureTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Dsn: "", Transport: tr})
	if err != nil {
		t.Fatalf("building a test sentry client: %v", err)
	}

	hub := sentry.CurrentHub()
	previous := hub.Client()
	hub.BindClient(client)
	t.Cleanup(func() { hub.BindClient(previous) })

	return tr
}

// findMessage returns the first captured message containing needle.
func findMessage(messages []string, needle string) (string, bool) {
	for _, m := range messages {
		if strings.Contains(m, needle) {
			return m, true
		}
	}
	return "", false
}

// TestAnUnreadableSellerTokenRefusesBeforeMercadoPagoIsCalled is the twin of
// TestAMissingSellerTokenRefusesBeforeMercadoPagoIsCalled, and the reason
// refuseForCredential tells the two apart at all.
//
// "MISSING" means this venue never connected MercadoPago. "UNREADABLE" means
// the venue *is* connected and its stored credential no longer decrypts — a
// retired or wrong key, corruption, tampering — so every refund for it would
// keep failing until someone re-keys the row. Those are different incidents
// with different fixes, and only the second is an emergency.
//
// The complex_id is asserted alongside the reason because an alert that says
// UNREADABLE without naming the venue tells the operator nothing actionable.
//
// Mutation: collapse refuseForCredential's reason to a constant "MISSING"
// (delete the errors.Is(err, mpcred.ErrMPCredentialUnreadable) branch) and
// re-run — this test must fail.
func TestAnUnreadableSellerTokenRefusesBeforeMercadoPagoIsCalled(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.payments.byBooking = payment
	// Connected — the venue has a credential stored — but the stored bytes no
	// longer decrypt, so the accessor refuses with ErrMPCredentialUnreadable.
	f.complexes.complex = data.NewComplexWithUnreadableCredentialForTest(complexID)
	f.clients.client = &data.Client{ID: booking.ClientID}

	sentryEvents := withCapturedSentryEvents(t)

	f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.provider.refunds) != 0 || len(f.provider.callers) != 0 {
		t.Fatalf("an unreadable credential must refuse before MercadoPago is called; got refunds=%v tokens=%v",
			f.provider.refunds, f.provider.callers)
	}
	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("the refusal must be recorded against the already-committed claim; got %d", len(f.payments.recordedFailure))
	}
	cause := f.payments.recordedFailure[0].cause
	if !strings.Contains(cause, "seller credential unreadable") {
		t.Errorf("the cause must name the credential as unreadable; got %q", cause)
	}
	if strings.Contains(cause, "unavailable") {
		t.Errorf("an unreadable credential does not self-heal and must not be classified as the transient "+
			"outage arm, or it would stop spending the retry budget that keeps re-alerting; got %q", cause)
	}

	messages := sentryEvents.messages()
	alert, found := findMessage(messages, "SELLER CREDENTIAL")
	if !found {
		t.Fatalf("a credential refusal must alert on this same attempt; captured %q", messages)
	}
	if !strings.Contains(alert, "UNREADABLE") {
		t.Errorf("a credential that exists but will not decrypt must be reported as UNREADABLE, "+
			"not confused with a venue that never connected; got %q", alert)
	}
	if !strings.Contains(alert, complexID.String()) {
		t.Errorf("the alert must name the complex whose credential is unreadable, "+
			"or the operator cannot act on it; want complex_id=%s, got %q", complexID, alert)
	}
}

// TestAnUnfetchableComplexIsTreatedAsATransientOutage covers the third arm of
// sellerCredential, and it is the one that looks covered and is not.
//
// payments_test.go already sets f.complexes.err, but that case drives the
// collector-verification path in processApprovedPayment and never reaches
// sellerCredential — so the alert here had never fired in a test.
//
// The three arms are three different incidents. UNAVAILABLE means the database
// would not answer, and the next attempt may well succeed — so, unlike the
// other two, it must not spend a retry. UNREADABLE means the stored credential
// will not decrypt. MISSING means the venue never connected. An operator paged
// at 03:00 acts differently on each, so collapsing them costs the alert its
// only purpose.
//
// Mutation: drop "seller credential unavailable" from providerOutageMarkers
// (failed_refunds.go) and re-run — the cause assertion below must fail,
// because transientProviderFailure would then answer false for this arm.
func TestAnUnfetchableComplexIsTreatedAsATransientOutage(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.payments.byBooking = payment
	f.complexes.err = errors.New("connection refused")
	f.clients.client = &data.Client{ID: booking.ClientID}

	sentryEvents := withCapturedSentryEvents(t)

	f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.provider.refunds) != 0 || len(f.provider.callers) != 0 {
		t.Fatalf("an unfetchable complex must refuse before MercadoPago is called; got refunds=%v tokens=%v",
			f.provider.refunds, f.provider.callers)
	}
	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("the refusal must be recorded against the already-committed claim; got %d", len(f.payments.recordedFailure))
	}
	// internal/data.transientProviderFailure is unexported and this test lives
	// across the package boundary, so the marker itself is pinned here and its
	// effect on the retry budget (internal/data/failed_refunds_test.go's
	// TestTransientProviderFailure) is proven where that function lives.
	cause := f.payments.recordedFailure[0].cause
	if !strings.Contains(cause, "seller credential unavailable") {
		t.Errorf("the cause must carry the one marker providerOutageMarkers matches for this arm, "+
			"or a transient database blip would wrongly spend the retry budget; got %q", cause)
	}

	messages := sentryEvents.messages()
	alert, found := findMessage(messages, "SELLER CREDENTIAL")
	if !found {
		t.Fatalf("a credential refusal must alert on this same attempt; captured %q", messages)
	}
	if !strings.Contains(alert, "UNAVAILABLE") {
		t.Errorf("a complex the database would not return must be reported as UNAVAILABLE, "+
			"not as a venue with no credential; got %q", alert)
	}
	if !strings.Contains(alert, complexID.String()) {
		t.Errorf("the alert must name the complex, or the operator cannot act on it; "+
			"want complex_id=%s, got %q", complexID, alert)
	}
}

// The refund email is the one message that tells a client how much money came
// back, so the amount has to read as pesos do here: dot between thousands,
// comma before the centavos.
func TestFormatARSWritesPesosTheArgentineWay(t *testing.T) {
	cases := map[int]string{
		0:        "$0",
		3000:     "$30",
		300050:   "$3.000,50",
		12345678: "$123.456,78",
		-150000:  "-$1.500",
	}
	for centavos, want := range cases {
		if got := formatARS(centavos); got != want {
			t.Errorf("formatARS(%d) = %q, want %q", centavos, got, want)
		}
	}
}
