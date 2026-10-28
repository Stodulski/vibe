package bookings

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// TestAnInboundWhatsAppMessageIsLoggedWithoutItsBody pins what the webhook may
// write about a client's message. The text is the client's own words, so it
// stays out of the log. The line keeps the message id, the length of the text,
// and the sender's last four digits, which is all anyone reading the log needs.
func TestAnInboundWhatsAppMessageIsLoggedWithoutItsBody(t *testing.T) {
	const body = "Hola, quiero cancelar mi turno de las 21 hs"
	const messageID = "wamid.HBgLNTQ5MTE1NTU1NDU2NxUCABIYIEE"
	raw := `{"id":"` + messageID + `","from":"5491155554567","type":"text","text":{"body":"` + body + `"}}`

	var msg waMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("the test message does not decode: %v", err)
	}

	f := newFixture(t)
	var logs bytes.Buffer
	f.service.logger = slog.New(slog.NewJSONHandler(&logs, nil))

	f.service.processWhatsAppMessage(msg)

	if out := logs.String(); strings.Contains(out, body) || strings.Contains(out, "cancelar") {
		t.Fatalf("the client's message body reached the log: %s", out)
	}

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &line); err != nil {
		t.Fatalf("want one JSON log line; got %q: %v", logs.String(), err)
	}
	if _, ok := line["text"]; ok {
		t.Error("the log line carries a text field")
	}
	if got := line["message_id"]; got != messageID {
		t.Errorf("want message_id %q in the log; got %v", messageID, got)
	}
	if got := line["text_len"]; got != float64(len(body)) {
		t.Errorf("want text_len %d in the log; got %v", len(body), got)
	}
	if got := line["from"]; got != "***4567" {
		t.Errorf("want the sender masked to its last four digits; got %v", got)
	}
}
