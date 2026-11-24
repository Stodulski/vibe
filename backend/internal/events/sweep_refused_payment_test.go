package events

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"

	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
)

// captureTransport records every Sentry event instead of sending it.
type captureTransport struct {
	mu       sync.Mutex
	messages []string
}

func (c *captureTransport) Flush(_ time.Duration) bool              { return true }
func (c *captureTransport) FlushWithContext(_ context.Context) bool { return true }
func (c *captureTransport) Configure(_ sentry.ClientOptions)        {}
func (c *captureTransport) Close()                                  {}
func (c *captureTransport) SendEvent(event *sentry.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, event.Message)
}

func (c *captureTransport) find(needle string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.messages {
		if strings.Contains(m, needle) {
			return m, true
		}
	}
	return "", false
}

func withCapturedSentry(t *testing.T) *captureTransport {
	t.Helper()
	tr := &captureTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: tr})
	if err != nil {
		t.Fatal(err)
	}
	hub := sentry.CurrentHub()
	previous := hub.Client()
	hub.BindClient(client)
	t.Cleanup(func() { hub.BindClient(previous) })
	return tr
}

// A paid spot whose payment the processing refuses (the collector or the
// amount does not check out) is handed over, recorded by nothing, and the
// check closed as answered: the payer kept their money captured and lost the
// spot with nobody told the registration was lost. The sweep now looks for
// the payment in the ledger after handing it over and alerts, naming the
// registration, the payment and the command that replays it.
func TestAPaidRegistrationThePaymentProcessingRefusesIsAlerted(t *testing.T) {
	captured := withCapturedSentry(t)
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.found = map[string][]mp.Payment{reg.ID.String(): {approved(191310158388, reg)}}

	f.svc.ExpireHolds(t.Context())

	alert, ok := captured.find("PAID REGISTRATION NOT RECORDED")
	if !ok {
		t.Fatalf("a paid registration the processing did not record must alert; captured %v", captured.messages)
	}
	for _, want := range []string{reg.ID.String(), "mp_payment_id=191310158388", "go run ./cmd/replaypayment 191310158388"} {
		if !strings.Contains(alert, want) {
			t.Errorf("the alert must carry %q; got %q", want, alert)
		}
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("a payer must never be told a spot they paid for expired; got %+v", f.notify.cancelled)
	}
}

// A payment the processing recorded (the spot confirmed) raises nothing.
func TestARecordedRegistrationPaymentRaisesNoAlert(t *testing.T) {
	captured := withCapturedSentry(t)
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.found = map[string][]mp.Payment{reg.ID.String(): {approved(2, reg)}}
	f.reconcile.confirms = map[string]*eventstore.Registration{"2": reg}

	f.svc.ExpireHolds(t.Context())

	if alert, ok := captured.find("PAID REGISTRATION NOT RECORDED"); ok {
		t.Errorf("a recorded payment must not alert; got %q", alert)
	}
}
