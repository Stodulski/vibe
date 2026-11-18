package mailer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWaitlistPlaceFreedEmailSaysWhatFreedAndWhoWins captures the message the
// Brevo transport receives and checks the copy the place waitlist promises: the
// subject, what freed, that the first to pay wins, and the direct link.
func TestWaitlistPlaceFreedEmailSaysWhatFreedAndWhoWins(t *testing.T) {
	var sent struct {
		Subject     string `json:"subject"`
		HTMLContent string `json:"htmlContent"`
	}
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"waitlist-place-1"}`))
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "key", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL

	const bookURL = "https://vibe.test/c/vibe-padel?date=2026-11-18"
	if err := m.SendWaitlistPlaceFreed(t.Context(), "ana@example.com", "Vibe Padel",
		"Cancha 1 el miércoles 18/11 a las 19:00", bookURL); err != nil {
		t.Fatalf("send: %v", err)
	}

	for _, want := range []string{
		"Cancha 1 el miércoles 18/11 a las 19:00",
		`href="` + strings.ReplaceAll(bookURL, "&", "&amp;") + `"`,
	} {
		if !strings.Contains(sent.HTMLContent, want) {
			t.Errorf("body is missing %q", want)
		}
	}
	for _, banned := range []string{"!", "—"} {
		if strings.Contains(sent.Subject, banned) {
			t.Errorf("subject carries %q", banned)
		}
	}
}
