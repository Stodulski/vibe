package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

type fakeInbox struct {
	inserted []*paymentstore.WebhookEvent
	err      error
	// stopped is every event StopReplay was asked to close; stops answers
	// whether the store found a replay to close.
	stopped []uuid.UUID
	stops   bool
}

func (f *fakeInbox) StopReplay(_ context.Context, id uuid.UUID) (bool, error) {
	f.stopped = append(f.stopped, id)
	return f.stops, f.err
}

func (f *fakeInbox) Insert(_ context.Context, e *paymentstore.WebhookEvent) error {
	if f.err != nil {
		return f.err
	}
	e.ID = uuid.New()
	f.inserted = append(f.inserted, e)
	return nil
}

// A replay records one inbox row the payment worker dispatches as a payment
// webhook for exactly that payment, the same row a delivered webhook leaves.
func TestReplayRecordsAPaymentEventTheWorkerDispatches(t *testing.T) {
	inbox := &fakeInbox{}

	event, err := replay(t.Context(), inbox, "191310158388")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if len(inbox.inserted) != 1 || inbox.inserted[0] != event {
		t.Fatalf("want exactly one inbox row recorded; got %d", len(inbox.inserted))
	}
	if event.EventType != "payment" || event.ExternalID != "191310158388" || event.Provider != "mercadopago" {
		t.Errorf("the worker dispatches on the event type and the external id; got %+v", event)
	}
	var body struct {
		Type string `json:"type"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(event.Payload, &body); err != nil || body.Type != "payment" || body.Data.ID != "191310158388" {
		t.Errorf("the payload must read as a MercadoPago payment notification; got %s (%v)", event.Payload, err)
	}
}

// Anything but a MercadoPago payment id is refused before the database is
// touched: a booking id pasted by mistake would sit in the inbox failing.
func TestReplayRefusesWhatIsNotAPaymentID(t *testing.T) {
	for _, id := range []string{"", "abc", uuid.NewString(), "123 456", "-1", "12a"} {
		inbox := &fakeInbox{}
		if _, err := replay(t.Context(), inbox, id); err == nil {
			t.Errorf("replay(%q) must be refused", id)
		}
		if len(inbox.inserted) != 0 {
			t.Errorf("replay(%q) must record nothing; got %d rows", id, len(inbox.inserted))
		}
	}
}

// A refused write is reported, not swallowed: the operator must know nothing
// was queued.
func TestReplayReportsAFailedWrite(t *testing.T) {
	inbox := &fakeInbox{err: errors.New("database unavailable")}
	if _, err := replay(t.Context(), inbox, "123"); err == nil {
		t.Error("a failed write must be an error")
	}
}

// What the operator reads after a replay is the only guidance they get: the
// event's "processed" status says the processing ran, not that the booking was
// confirmed (a collector or amount refusal ends "processed" too), and a payment
// id MercadoPago does not know would be retried forever, so the output names
// the command that stops it.
func TestReplayTellsTheOperatorWhatProcessedMeansAndHowToStopIt(t *testing.T) {
	inbox := &fakeInbox{}
	event, err := replay(t.Context(), inbox, "123")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	var out strings.Builder
	report(&out, event)

	for _, want := range []string{
		event.ID.String(),
		"does not mean the payment was confirmed",
		"Sentry",
		"replaypayment -stop " + event.ID.String(),
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report must say %q; got:\n%s", want, out.String())
		}
	}
}

// -stop closes a replay that keeps failing, and says so when there was none to
// close; anything but an event id is refused before the database is touched.
func TestStopClosesAReplayOrSaysThereWasNone(t *testing.T) {
	id := uuid.New()

	inbox := &fakeInbox{stops: true}
	if err := stop(t.Context(), inbox, id.String()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(inbox.stopped) != 1 || inbox.stopped[0] != id {
		t.Errorf("want the event closed; got %v", inbox.stopped)
	}

	if err := stop(t.Context(), &fakeInbox{}, id.String()); err == nil {
		t.Error("stopping an event that is not an open replay must be an error")
	}

	bad := &fakeInbox{stops: true}
	if err := stop(t.Context(), bad, "191310158388"); err == nil || len(bad.stopped) != 0 {
		t.Errorf("a payment id is not an event id; err %v, stopped %v", err, bad.stopped)
	}
}
