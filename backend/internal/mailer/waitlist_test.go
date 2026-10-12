package mailer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWaitlistSlotFreedEmailSaysWhatHappenedAndLinksTheComplex captures the
// message the Brevo transport receives and checks the copy the waitlist promises:
// the subject, the day, the complex, the call to act, and the storefront link.
func TestWaitlistSlotFreedEmailSaysWhatHappenedAndLinksTheComplex(t *testing.T) {
	var sent struct {
		Subject     string `json:"subject"`
		HTMLContent string `json:"htmlContent"`
	}
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"waitlist-1"}`))
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "key", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL

	const bookURL = "https://vibe.test/c/vibe-padel"
	if err := m.SendWaitlistSlotFreed(t.Context(), "ana@example.com", "Vibe Padel", "20/11", bookURL); err != nil {
		t.Fatalf("send: %v", err)
	}

	if want := "Se liberó un turno en Vibe Padel"; sent.Subject != want {
		t.Errorf("subject = %q; want %q", sent.Subject, want)
	}
	for _, want := range []string{
		"Se liberó un turno el <strong>20/11</strong> en <strong>Vibe Padel</strong>. Reservalo antes que otro.",
		"Ver horarios",
		`href="` + bookURL + `"`,
	} {
		if !strings.Contains(sent.HTMLContent, want) {
			t.Errorf("body is missing %q", want)
		}
	}
}
