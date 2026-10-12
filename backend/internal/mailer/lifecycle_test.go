package mailer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	nudgeUnsub = "https://api.vibe.test/api/v1/public/email/unsubscribe?token=tok"
	nudgeOptIn = "https://vibe.test/baja?token=tok"
	nudgeCTA   = "https://app.vibe.test/onboarding"
)

// sentBrevo sends one lifecycle nudge through a fake Brevo and returns the decoded
// request body the transport would have sent.
func sentBrevo(t *testing.T, kind, firstName, complexName string) map[string]any {
	t.Helper()
	var body []byte
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL
	if err := m.SendLifecycleNudge(context.Background(), "ana@example.com", kind, firstName, complexName,
		nudgeCTA, nudgeUnsub, nudgeOptIn); err != nil {
		t.Fatalf("SendLifecycleNudge(%s): %v", kind, err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decoding the Brevo request: %v", err)
	}
	return got
}

func TestLifecycleNudgeCopyPerKind(t *testing.T) {
	cases := []struct {
		kind, subject, body, cta string
	}{
		{"verify_email", "Te falta confirmar tu email en Vibe",
			"Empezaste a crear tu cuenta en Vibe y te falta confirmar tu email. Tocá el botón y seguís donde quedaste.",
			"Confirmar mi email"},
		{"create_complex", "Tu complejo te espera en Vibe",
			"Ya tenés tu cuenta. Te falta cargar tu complejo: nombre, dirección y horarios. Te lleva unos minutos.",
			"Cargar mi complejo"},
		{"add_courts", "Cargá tus canchas en Vibe",
			"Padel Norte ya está en Vibe. Cargá tus canchas y precios para empezar a usar el turnero y tomar reservas.",
			"Cargar mis canchas"},
		{"inactive", "¿Te ayudo a arrancar con Vibe?",
			"Vi que hace unos días no entrás a Vibe. Si algo te frenó, respondé este mail y te ayudo a dejar Padel Norte andando. Mientras tanto, podés cargar tus turnos de la semana en el turnero.",
			"Ir a mi panel"},
		{"finish_signup", "Tu cuenta de Vibe quedó a medio camino",
			"Empezaste a crear tu cuenta en Vibe y no llegaste a terminarla. Vibe es gratis para tu complejo: turnero, caja y reservas online con seña. Si algo te frenó, respondé este mail.",
			"Terminar mi cuenta gratis"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			got := sentBrevo(t, tc.kind, "Ana", "Padel Norte")
			if got["subject"] != tc.subject {
				t.Errorf("subject = %v, want %q", got["subject"], tc.subject)
			}
			html, _ := got["htmlContent"].(string)
			if !strings.Contains(html, "Hola Ana. "+tc.body) {
				t.Errorf("body does not carry the approved copy:\nwant %q", "Hola Ana. "+tc.body)
			}
			if !strings.Contains(html, ">"+tc.cta+"</a>") {
				t.Errorf("button label %q missing", tc.cta)
			}
			if !strings.Contains(html, "Máximo, de Vibe") {
				t.Error("the email is not signed Máximo, de Vibe")
			}
		})
	}
}

func TestLifecycleNudgeGreetingWithoutAName(t *testing.T) {
	got := sentBrevo(t, "finish_signup", "", "")
	html, _ := got["htmlContent"].(string)
	if !strings.Contains(html, "Hola. Empezaste") {
		t.Fatal("an email with no name must greet with a bare \"Hola.\"")
	}
	if strings.Contains(html, "Hola ,") || strings.Contains(html, "Hola  ") {
		t.Fatal("an empty name left a gap in the greeting")
	}
}

func TestLifecycleNudgeEscapesWhatTheRecipientTypedAndTheComplexName(t *testing.T) {
	got := sentBrevo(t, "add_courts", "<img src=x>", "A&B Padel")
	html, _ := got["htmlContent"].(string)
	if strings.Contains(html, "<img src=x>") {
		t.Fatal("a first name reached the HTML unescaped")
	}
	if !strings.Contains(html, "A&amp;B Padel") {
		t.Fatal("the complex name was not escaped")
	}
}

func TestLifecycleNudgeCarriesListUnsubscribeHeadersOverBrevo(t *testing.T) {
	got := sentBrevo(t, "create_complex", "Ana", "")
	headers, ok := got["headers"].(map[string]any)
	if !ok {
		t.Fatalf("no headers object in the Brevo request: %v", got)
	}
	lu, _ := headers["List-Unsubscribe"].(string)
	if !strings.Contains(lu, "<"+nudgeUnsub+">") || !strings.Contains(lu, "<mailto:hola@vibe.com.ar?subject=baja>") {
		t.Errorf("List-Unsubscribe = %q, want the one-click URL and the mailto fallback", lu)
	}
	if headers["List-Unsubscribe-Post"] != "List-Unsubscribe=One-Click" {
		t.Errorf("List-Unsubscribe-Post = %v, want the RFC 8058 one-click value", headers["List-Unsubscribe-Post"])
	}
	reply, _ := got["replyTo"].(map[string]any)
	if reply["email"] != "hola@vibe.com.ar" {
		t.Errorf("replyTo = %v, want hola@vibe.com.ar", got["replyTo"])
	}
}

func TestLifecycleNudgeFooterLinksTheOptOutPage(t *testing.T) {
	got := sentBrevo(t, "inactive", "Ana", "Padel Norte")
	html, _ := got["htmlContent"].(string)
	if !strings.Contains(html, "Si no querés recibir más estos mails, ") {
		t.Fatal("the footer sentence is missing")
	}
	if !strings.Contains(html, `href="`+nudgeOptIn+`"`) || !strings.Contains(html, "date de baja acá") {
		t.Fatal("the footer does not link the /baja page")
	}
}

func TestLifecycleNudgeCopyUsesNoForbiddenPunctuation(t *testing.T) {
	for _, kind := range []string{"verify_email", "create_complex", "add_courts", "inactive", "finish_signup"} {
		c, ok := nudgeCopyFor(kind, "Ana", "Padel Norte")
		if !ok {
			t.Fatalf("no copy for %s", kind)
		}
		for field, text := range map[string]string{
			"subject": c.subject, "preheader": c.preheader, "body": c.body, "cta": c.cta,
		} {
			if strings.Contains(text, "!") {
				t.Errorf("%s %s uses an exclamation mark: %q", kind, field, text)
			}
			if strings.Contains(text, "—") {
				t.Errorf("%s %s uses an em dash: %q", kind, field, text)
			}
		}
	}
}

func TestLifecycleNudgeUnknownKindIsAPermanentFailure(t *testing.T) {
	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	err := m.SendLifecycleNudge(context.Background(), "ana@example.com", "nope", "", "", nudgeCTA, nudgeUnsub, nudgeOptIn)
	if err == nil {
		t.Fatal("an unknown kind was sent")
	}
	if !isPermanent(err) {
		t.Fatalf("err = %v; an unknown kind must not be retried", err)
	}
}

func TestMIMEMessageCarriesTheListHeadersAndRefusesInjection(t *testing.T) {
	m := New(Config{SMTPHost: "smtp.example.com", SMTPPort: 587, Sender: "Vibe <noreply@vibe.com.ar>"})
	msg := m.mimeMessage("ana@example.com", "Asunto", "<p>hola</p>", "hola", outbound{
		headers: map[string]string{
			"List-Unsubscribe":      "<" + nudgeUnsub + ">, <mailto:hola@vibe.com.ar?subject=baja>",
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			"X-Injected":            "a\r\nBcc: evil@example.com",
		},
		replyTo: "hola@vibe.com.ar",
	})

	for _, want := range []string{
		"\r\nList-Unsubscribe: <" + nudgeUnsub + ">, <mailto:hola@vibe.com.ar?subject=baja>\r\n",
		"\r\nList-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n",
		"\r\nReply-To: hola@vibe.com.ar\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("MIME message is missing %q", want)
		}
	}
	if strings.Contains(msg, "\r\nBcc:") {
		t.Fatal("a header value injected a Bcc header")
	}
}

func TestMIMEMessageWithoutOptionsHasNoListHeaders(t *testing.T) {
	m := New(Config{SMTPHost: "smtp.example.com", SMTPPort: 587, Sender: "Vibe <noreply@vibe.com.ar>"})
	msg := m.mimeMessage("ana@example.com", "Asunto", "<p>hola</p>", "hola", outbound{})
	if strings.Contains(msg, "List-Unsubscribe") || strings.Contains(msg, "Reply-To") {
		t.Fatal("a transactional email gained list headers it was not given")
	}
}
