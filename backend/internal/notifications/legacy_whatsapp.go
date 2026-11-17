package notifications

import (
	"context"
	"encoding/json"
)

// legacyWhatsAppTasks are the task types the WhatsApp channel enqueued before
// it was removed. Nothing enqueues them any more, but rows of these types may
// still be waiting in the jobs table.
//
// The drain handler below completes them. Without it the pool would hand each
// row back on every poll ("no handler registered") until MaxAge killed it, and
// the phone numbers in those payloads would sit in the queue until then.
//
// Remove this file, the registerLegacyWhatsAppDrain call in RegisterWorkers and
// the "wa:" prefix in ScrubsPayload one release after the removal, once the
// jobs table holds no wa:* rows.
var legacyWhatsAppTasks = []string{
	"wa:booking_confirmation",
	"wa:reminder_2h",
	"wa:booking_cancelled",
	"wa:deposit_refunded",
	"wa:series_cancelled",
}

// registerLegacyWhatsAppDrain registers a handler for each legacy WhatsApp task
// type. The handler discards the payload and succeeds: the message it would have
// sent no longer exists to be sent. It logs the task type only, never the
// payload, which holds a phone number.
func (s *Service) registerLegacyWhatsAppDrain() {
	for _, taskType := range legacyWhatsAppTasks {
		s.queue.RegisterHandler(taskType, func(_ context.Context, _ json.RawMessage) error {
			s.logger.Info("notifications: drained legacy WhatsApp task", "task_type", taskType)
			return nil
		})
	}
}
