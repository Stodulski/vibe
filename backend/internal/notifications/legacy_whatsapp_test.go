package notifications

import (
	"context"
	"encoding/json"
	"testing"
)

// The drain handler exists so rows the removed WhatsApp channel left in the
// queue complete instead of bouncing until they expire. Each legacy type must
// have a handler that succeeds, and must send nothing.
func TestLegacyWhatsAppTasksAreDrainedWithoutSending(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()

	payload, _ := json.Marshal(map[string]string{"phone": "+5411"})
	for _, taskType := range legacyWhatsAppTasks {
		h, ok := q.handlers[taskType]
		if !ok {
			t.Errorf("no drain handler registered for %s; the row would bounce until it expired", taskType)
			continue
		}
		if err := h(context.Background(), payload); err != nil {
			t.Errorf("draining %s returned %v; want nil so the row completes", taskType, err)
		}
	}
	if len(mailer.sent) != 0 {
		t.Errorf("draining sent mail: %v", mailer.sent)
	}
}

// The legacy list is what ScrubsPayload keeps covering, so it must name the
// five task types the removed channel enqueued, each with the wa: prefix.
func TestLegacyWhatsAppTasksAreTheFiveRemovedTypes(t *testing.T) {
	want := []string{
		"wa:booking_confirmation",
		"wa:reminder_2h",
		"wa:booking_cancelled",
		"wa:deposit_refunded",
		"wa:series_cancelled",
	}
	if len(legacyWhatsAppTasks) != len(want) {
		t.Fatalf("legacyWhatsAppTasks = %v, want %v", legacyWhatsAppTasks, want)
	}
	for i, taskType := range want {
		if legacyWhatsAppTasks[i] != taskType {
			t.Errorf("legacyWhatsAppTasks[%d] = %q, want %q", i, legacyWhatsAppTasks[i], taskType)
		}
	}
}
