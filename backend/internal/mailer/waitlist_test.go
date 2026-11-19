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

	for _, want := range []string{
		"20/11",
		"Vibe Padel",
		`href="` + bookURL + `"`,
	} {
		if !strings.Contains(sent.HTMLContent, want) {
			t.Errorf("body is missing %q", want)
		}
	}
}
