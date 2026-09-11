package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

func TestNewWAClient(t *testing.T) {
	client := NewWAClient("token", "phone-id", "verify-token", "app-secret", nil)

	if client.token != "token" {
		t.Errorf("want token %q; got %q", "token", client.token)
	}
	if client.phoneID != "phone-id" {
		t.Errorf("want phoneID %q; got %q", "phone-id", client.phoneID)
	}
	if client.verifyToken != "verify-token" {
		t.Errorf("want verifyToken %q; got %q", "verify-token", client.verifyToken)
	}
	if client.appSecret != "app-secret" {
		t.Errorf("want appSecret %q; got %q", "app-secret", client.appSecret)
	}
}

func TestVerifyWebhook(t *testing.T) {
	client := NewWAClient("token", "phone-id", "my-verify-token", "app-secret", nil)

	t.Run("valid verification request", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/webhook?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=challenge-123", nil)

		challenge, err := client.VerifyWebhook(req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if challenge != "challenge-123" {
			t.Errorf("want challenge %q; got %q", "challenge-123", challenge)
		}
	})

	t.Run("invalid mode", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/webhook?hub.mode=unsubscribe&hub.verify_token=my-verify-token&hub.challenge=abc", nil)

		_, err := client.VerifyWebhook(req)
		if err == nil {
			t.Error("expected error for invalid mode")
		}
	})

	t.Run("wrong verify token", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/webhook?hub.mode=subscribe&hub.verify_token=wrong-token&hub.challenge=abc", nil)

		_, err := client.VerifyWebhook(req)
		if err == nil {
			t.Error("expected error for wrong verify token")
		}
	})
}

func TestVerifySignature(t *testing.T) {
	appSecret := "test-app-secret"
	client := NewWAClient("token", "phone-id", "verify-token", appSecret, nil)

	t.Run("valid signature", func(t *testing.T) {
		body := []byte(`{"entry":[]}`)

		mac := hmac.New(sha256.New, []byte(appSecret))
		mac.Write(body)
		expectedHex := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Hub-Signature-256", "sha256="+expectedHex)

		err := client.VerifySignature(req, body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing signature header", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		err := client.VerifySignature(req, []byte(`{}`))
		if err == nil {
			t.Error("expected error for missing signature header")
		}
	})

	t.Run("invalid signature format", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Hub-Signature-256", "md5=abc123")
		err := client.VerifySignature(req, []byte(`{}`))
		if err == nil {
			t.Error("expected error for invalid signature format")
		}
	})

	t.Run("wrong signature", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Hub-Signature-256", "sha256=0000000000000000000000000000000000000000000000000000000000000000")
		err := client.VerifySignature(req, []byte(`{"entry":[]}`))
		if err == nil {
			t.Error("expected error for wrong signature")
		}
	})
}

// The four template tests below pin parameter order and count, which is the
// one thing about a WhatsApp template that fails silently. Meta binds
// parameters positionally: swap two and the message goes out reading
// "Cancha: 15/03" with a 200 OK and no error anywhere. docs/whatsapp-templates.md
// holds the approved body text these positions correspond to.

// bodyParams returns the body component's parameter texts, in order.
func bodyParams(t *testing.T, msg TemplateMessage) []string {
	t.Helper()
	if len(msg.Components) == 0 || msg.Components[0].Type != "body" {
		t.Fatalf("want a body component first; got %+v", msg.Components)
	}
	out := make([]string, 0, len(msg.Components[0].Parameters))
	for _, p := range msg.Components[0].Parameters {
		if p.Type != "text" {
			t.Errorf("want every parameter to be text; got %q", p.Type)
		}
		out = append(out, p.Text)
	}
	return out
}

// assertButton checks that the component at position pos is the URL button at
// the given template index, carrying exactly one parameter with suffix.
func assertButton(t *testing.T, msg TemplateMessage, pos int, index, suffix string) {
	t.Helper()
	if len(msg.Components) <= pos {
		t.Fatalf("want a component at position %d; got %d components", pos, len(msg.Components))
	}
	c := msg.Components[pos]
	if c.Type != "button" || c.SubType != "url" || c.Index != index {
		t.Errorf("want a url button at index %q; got type=%q sub_type=%q index=%q", index, c.Type, c.SubType, c.Index)
	}
	if len(c.Parameters) != 1 {
		t.Fatalf("want exactly one button parameter; got %d", len(c.Parameters))
	}
	if c.Parameters[0].Text != suffix {
		t.Errorf("want button %s param %q; got %q", index, suffix, c.Parameters[0].Text)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBookingConfirmationTemplate(t *testing.T) {
	const (
		mapsQuery  = "-34.603722,-58.381592"
		cancelPath = "vibe-norte/book/cancel?token=abc-123"
	)
	msg := BookingConfirmationTemplate(
		"Cancha 1", "Vibe Norte", "15/03", "09:00 a 10:30",
		"$5.000", "$15.000",
		"con devolución hasta 24 horas antes del turno.",
		mapsQuery, cancelPath,
	)

	if msg.TemplateName != "booking_confirmation" {
		t.Errorf("want template name %q; got %q", "booking_confirmation", msg.TemplateName)
	}
	if msg.LanguageCode != "es_AR" {
		t.Errorf("want language code %q; got %q", "es_AR", msg.LanguageCode)
	}
	if len(msg.Components) != 3 {
		t.Fatalf("want 3 components (body + 2 buttons); got %d", len(msg.Components))
	}

	want := []string{
		"Cancha 1", "Vibe Norte", "15/03", "09:00 a 10:30",
		"$5.000", "$15.000",
		"con devolución hasta 24 horas antes del turno.",
	}
	if got := bodyParams(t, msg); !equalStrings(got, want) {
		t.Errorf("body parameters are wrong or out of order;\n got %q\nwant %q", got, want)
	}

	assertButton(t, msg, 1, "0", mapsQuery)  // Ver ubicación
	assertButton(t, msg, 2, "1", cancelPath) // Cancelar reserva
}

func TestReminder2hTemplate(t *testing.T) {
	const (
		mapsQuery  = "Vibe+Este%2C+Av.+Pellegrini+1200%2C+Rosario"
		cancelPath = "vibe-este/book/cancel?token=def-456"
	)
	msg := Reminder2hTemplate(
		"Cancha 3", "Vibe Este", "20/03", "16:00 a 17:30",
		"Av. Pellegrini 1200, Rosario",
		"$15.000",
		mapsQuery, cancelPath,
	)

	if msg.TemplateName != "reminder_2h" {
		t.Errorf("want template name %q; got %q", "reminder_2h", msg.TemplateName)
	}
	if msg.LanguageCode != "es_AR" {
		t.Errorf("want language code %q; got %q", "es_AR", msg.LanguageCode)
	}
	if len(msg.Components) != 3 {
		t.Fatalf("want 3 components (body + 2 buttons); got %d", len(msg.Components))
	}

	want := []string{
		"Cancha 3", "Vibe Este", "20/03", "16:00 a 17:30",
		"Av. Pellegrini 1200, Rosario",
		"$15.000",
	}
	if got := bodyParams(t, msg); !equalStrings(got, want) {
		t.Errorf("body parameters are wrong or out of order;\n got %q\nwant %q", got, want)
	}

	assertButton(t, msg, 1, "0", mapsQuery)  // Cómo llegar
	assertButton(t, msg, 2, "1", cancelPath) // Cancelar reserva
}

func TestBookingCancelledTemplate(t *testing.T) {
	const bookPath = "vibe-sur/book"
	msg := BookingCancelledTemplate(
		"Cancha 2", "Vibe Sur", "18/03", "21:00 a 22:30",
		"$5.000 enviados, se acreditan en los próximos días hábiles.",
		bookPath,
	)

	if msg.TemplateName != "booking_cancelled" {
		t.Errorf("want template name %q; got %q", "booking_cancelled", msg.TemplateName)
	}
	if msg.LanguageCode != "es_AR" {
		t.Errorf("want language code %q; got %q", "es_AR", msg.LanguageCode)
	}
	if len(msg.Components) != 2 {
		t.Fatalf("want 2 components (body + 1 button); got %d", len(msg.Components))
	}

	want := []string{
		"Cancha 2", "Vibe Sur", "18/03", "21:00 a 22:30",
		"$5.000 enviados, se acreditan en los próximos días hábiles.",
	}
	if got := bodyParams(t, msg); !equalStrings(got, want) {
		t.Errorf("body parameters are wrong or out of order;\n got %q\nwant %q", got, want)
	}

	assertButton(t, msg, 1, "0", bookPath) // Nueva reserva
}

func TestDepositRefundedTemplate(t *testing.T) {
	const bookPath = "vibe-sur/book"
	msg := DepositRefundedTemplate("$5.000", "Vibe Sur", bookPath)

	if msg.TemplateName != "deposit_returned" {
		t.Errorf("want template name %q; got %q", "deposit_returned", msg.TemplateName)
	}
	if msg.LanguageCode != "es_AR" {
		t.Errorf("want language code %q; got %q", "es_AR", msg.LanguageCode)
	}
	if len(msg.Components) != 2 {
		t.Fatalf("want 2 components (body + 1 button); got %d", len(msg.Components))
	}

	want := []string{"$5.000", "Vibe Sur"}
	if got := bodyParams(t, msg); !equalStrings(got, want) {
		t.Errorf("body parameters are wrong or out of order;\n got %q\nwant %q", got, want)
	}

	assertButton(t, msg, 1, "0", bookPath) // Nueva reserva
}

// The two rules Meta applies to every outbound message are applied in
// SendTemplate, which is the one place every template passes through — a
// template added later cannot forget either.
func TestSendTemplateNormalisesRecipientAndParameters(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWAClient("token", "phone-id", "verify", "secret", circuitbreaker.New(circuitbreaker.Config{}))
	client.baseURL = server.URL

	msg := DepositRefundedTemplate("$5.000", "Vibe\tNorte    Palermo", "vibe-norte/book")
	// An Argentine mobile as internal/validator stores it: valid E.164, no 9.
	if err := client.SendTemplate(t.Context(), "+541155551234", msg); err != nil {
		t.Fatalf("SendTemplate: %v", err)
	}

	var sent struct {
		To       string `json:"to"`
		Template struct {
			Components []struct {
				Parameters []struct {
					Text string `json:"text"`
				} `json:"parameters"`
			} `json:"components"`
		} `json:"template"`
	}
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("the request body is not the JSON Meta expects: %v", err)
	}

	if sent.To != "+5491155551234" {
		t.Errorf("the recipient was not normalised for WhatsApp; got %q", sent.To)
	}
	got := sent.Template.Components[0].Parameters[1].Text
	if got != "Vibe Norte Palermo" {
		t.Errorf("a parameter Meta would refuse went out as %q", got)
	}
	if strings.ContainsAny(got, "\n\t\r") || strings.Contains(got, "    ") {
		t.Errorf("the parameter still carries whitespace Meta refuses: %q", got)
	}
}

// Meta expires a Graph API version roughly two years after it ships, and calls
// against an expired one fail with 400s that say nothing about the cause. This
// deployment sat on v19.0 — expired 21 May 2026 — with nothing saying so.
func TestGraphAPIVersionIsNotExpired(t *testing.T) {
	client := NewWAClient("token", "phone-id", "verify", "secret", nil)
	if client.baseURL != graphAPIBaseURL {
		t.Errorf("the client must call the named version; got %q", client.baseURL)
	}
	for _, expired := range []string{"/v19.0", "/v20.0", "/v21.0"} {
		if strings.HasSuffix(graphAPIBaseURL, expired) {
			t.Errorf("graphAPIBaseURL points at %s, which Meta has expired", expired)
		}
	}
}
