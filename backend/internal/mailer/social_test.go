package mailer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSocialPublishReportCarriesTheLinesAndEscapesThem checks that the report
// email sends the subject as given, each line in the body, and escapes markup
// in the text so a caption or error cannot inject HTML into the operator's inbox.
func TestSocialPublishReportCarriesTheLinesAndEscapesThem(t *testing.T) {
	var sent struct {
		Subject     string `json:"subject"`
		HTMLContent string `json:"htmlContent"`
	}
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"social-1"}`))
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "key", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL

	subject := "Publicación en redes: caja-turnos publicada"
	lines := []string{
		"Publicación: caja-turnos.",
		"Instagram: https://www.instagram.com/p/abc/",
		"Último error: <b>boom</b>",
	}
	if err := m.SendSocialPublishReport(t.Context(), "hola@vibe.com.ar", subject, lines); err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent.Subject != subject {
		t.Errorf("subject = %q; want %q", sent.Subject, subject)
	}
	if !strings.Contains(sent.HTMLContent, "Instagram: https://www.instagram.com/p/abc/") {
		t.Error("body is missing the Instagram line")
	}
	if strings.Contains(sent.HTMLContent, "<b>boom</b>") || !strings.Contains(sent.HTMLContent, "&lt;b&gt;boom&lt;/b&gt;") {
		t.Error("an error line must be escaped, not rendered as markup")
	}
}
