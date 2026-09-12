package bookings

import (
	"encoding/json"
	"io"
	"net/http"
)

// maskPhone returns only the last 4 digits of a phone number for safe logging.
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "***" + phone[len(phone)-4:]
}

// WhatsAppVerify handles GET /api/v1/webhooks/whatsapp, Meta's subscription
// handshake: it echoes back the challenge Meta sends.
func (h *Handler) WhatsAppVerify(w http.ResponseWriter, r *http.Request) {
	challenge, err := h.whatsapp.VerifyWebhook(r)
	if err != nil {
		h.logger.Error("wa verify: verification failed", "error", err)
		h.respond.Error(w, r, http.StatusForbidden, "verification failed")
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	//nolint:gosec // G705: challenge is only reached after VerifyWebhook confirms the caller supplied the correct
	// hub.verify_token secret; echoing hub.challenge verbatim as text/plain is the WhatsApp/Meta Graph API webhook
	// verification handshake requirement, not a reflected-XSS vector (text/plain, no HTML/script execution context).
	if _, err := w.Write([]byte(challenge)); err != nil {
		h.logger.Error("wa verify: failed to write response", "error", err)
	}
}

// WhatsAppWebhook handles POST /api/v1/webhooks/whatsapp, receiving clients'
// replies to a booking confirmation.
//
// The replies are logged and nothing else: cancelling is done through the web
// app. Processing still happens in the background so Meta gets its fast 200 and
// does not redeliver.
func (h *Handler) WhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit body to 1MB to prevent memory exhaustion.
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("wa webhook: failed to read body", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Verify HMAC-SHA256 signature.
	if err := h.whatsapp.VerifySignature(r, body); err != nil {
		h.logger.Error("wa webhook: signature verification failed",
			"error", err,
			"remote_addr", r.RemoteAddr,
		)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Parse the nested Meta webhook structure.
	var webhook waWebhookPayload
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.Error("wa webhook: failed to parse body", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Detached on purpose: Meta redelivers any webhook it is not acknowledged for
	// quickly enough, so the 200 below must not wait on the processing.
	h.svc.DispatchWhatsAppMessages(webhook)

	w.WriteHeader(http.StatusOK)
}

type waWebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				Messages []waMessage `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

type waMessage struct {
	From      string `json:"from"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      *struct {
		Body string `json:"body"`
	} `json:"text"`
	Button *struct {
		Text    string `json:"text"`
		Payload string `json:"payload"`
	} `json:"button"`
	Interactive *struct {
		ButtonReply *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"button_reply"`
	} `json:"interactive"`
}

// DispatchWhatsAppMessages works a verified delivery on the application's
// tracked goroutines, after the handler has already acknowledged it.
func (s *Service) DispatchWhatsAppMessages(webhook waWebhookPayload) {
	s.run(func() {
		s.processWhatsAppMessages(webhook)
	})
}

func (s *Service) processWhatsAppMessages(webhook waWebhookPayload) {
	for _, entry := range webhook.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				s.processWhatsAppMessage(msg)
			}
		}
	}
}

// processWhatsAppMessage logs an inbound WhatsApp message. Confirmations and
// cancellations are handled through the web app only, so no ctx-bound action
// is taken here; a detached context is not needed for a synchronous log call.
func (s *Service) processWhatsAppMessage(msg waMessage) {
	phone := msg.From

	// Extract response text from text message, button reply, or interactive reply.
	var responseText string
	switch {
	case msg.Button != nil:
		responseText = msg.Button.Payload
	case msg.Interactive != nil && msg.Interactive.ButtonReply != nil:
		responseText = msg.Interactive.ButtonReply.ID
	case msg.Text != nil:
		responseText = msg.Text.Body
	}

	if responseText == "" {
		return
	}

	// WhatsApp messages from clients are logged but no action is taken.
	// Confirmations and cancellations are handled through the web app only.
	s.logger.Info("wa webhook: received message", "from", maskPhone(phone), "text", responseText)
}
