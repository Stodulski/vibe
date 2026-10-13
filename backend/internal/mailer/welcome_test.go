package mailer

import (
	"encoding/json"
	"errors"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	welcomeAppURL    = "https://app.vibe.test"
	welcomePublicURL = "https://vibe.test"
)

// welcomeCapture sends one welcome through a fake Brevo and returns the request
// body exactly as Brevo received it.
func welcomeCapture(t *testing.T, firstName string) map[string]any {
	t.Helper()

	var raw []byte
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(brevo.Close)

	m := New(Config{
		BrevoAPIKey: "k",
		Sender:      "Vibe <noreply@vibe.com.ar>",
		AppURL:      welcomeAppURL,
		PublicURL:   welcomePublicURL,
	})
	m.apiURL = brevo.URL

	if err := m.SendWelcome(t.Context(), "ana@example.com", firstName); err != nil {
		t.Fatalf("SendWelcome: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("captured request body is not valid JSON: %v", err)
	}
	return got
}

// welcomeHTML returns the HTML part of one captured welcome.
func welcomeHTML(t *testing.T, firstName string) string {
	t.Helper()
	body, ok := welcomeCapture(t, firstName)["htmlContent"].(string)
	if !ok {
		t.Fatal("welcome has no htmlContent")
	}
	return body
}

// welcomeVisibleText is what a reader sees of the welcome body, one line per
// block. Markup, URLs, entities and the invisible padding of the hidden preheader
// are removed, so digits in attributes or addresses cannot be mistaken for copy,
// and two headings cannot read as one sentence.
func welcomeVisibleText(htmlBody string) string {
	if i := strings.Index(htmlBody, "<body"); i >= 0 {
		htmlBody = htmlBody[i:]
	}
	// Conditional comments carry the Outlook markup, so they go first. Inline tags
	// such as <strong> vanish, so "Hola, <strong>Ana</strong>." reads as "Hola, Ana.".
	text := regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(htmlBody, " ")
	text = regexp.MustCompile(`(?i)</?(p|h1|td|tr|table|div|br)\b[^>]*>`).ReplaceAllString(text, "\n")
	text = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`https?://\S+`).ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = regexp.MustCompile(`[\x{034f}\x{00ad}\x{200b}-\x{200f}\x{2060}-\x{2064}\x{feff}]`).ReplaceAllString(text, "")

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if words := strings.Fields(line); len(words) > 0 {
			lines = append(lines, strings.Join(words, " "))
		}
	}
	return strings.Join(lines, "\n")
}

func TestWelcomeSubjectNamesTheOwner(t *testing.T) {
	cases := []struct {
		name, firstName, want string
	}{
		{"with a first name", "Ana", "Bienvenido a Vibe, Ana"},
		{"a first name padded with spaces", "  Ana  ", "Bienvenido a Vibe, Ana"},
		{"no first name", "", "Bienvenido a Vibe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := welcomeCapture(t, c.firstName)["subject"]; got != c.want {
				t.Errorf("subject = %q, want %q", got, c.want)
			}
		})
	}
}

// The welcome is transactional: it replies to the support address and carries
// none of the nudge machinery. A List-Unsubscribe header or a /baja footer here
// would tell an owner who just created the account that it can be unsubscribed
// from its own registration email.
func TestWelcomeRepliesToSupportWithoutUnsubscribe(t *testing.T) {
	got := welcomeCapture(t, "Ana")

	reply, _ := got["replyTo"].(map[string]any)
	if reply["email"] != "hola@vibe.com.ar" {
		t.Errorf("replyTo = %v, want hola@vibe.com.ar", got["replyTo"])
	}
	if headers, ok := got["headers"]; ok {
		t.Errorf("welcome carries headers %v; a transactional email has none", headers)
	}
	if body := welcomeHTML(t, "Ana"); strings.Contains(body, "/baja") || strings.Contains(body, "date de baja") {
		t.Error("welcome has the lifecycle opt-out footer")
	}
}

func TestWelcomeLinksEveryStepTutorialAndButton(t *testing.T) {
	body := welcomeHTML(t, "Ana")

	for _, u := range []string{
		welcomeAppURL + "/dashboard",
		welcomeAppURL + "/onboarding",
		welcomeAppURL + "/courts",
		welcomeAppURL + "/settings?tab=billing",
		welcomePublicURL + "/tutoriales/grilla-de-reservas",
		welcomePublicURL + "/tutoriales/configuracion-del-complejo",
		welcomePublicURL + "/tutoriales/precios-por-horario",
		welcomePublicURL + "/tutoriales/reportes-de-facturacion",
	} {
		if !strings.Contains(body, `href="`+u+`"`) {
			t.Errorf("welcome has no link to %s", u)
		}
	}
}

func TestWelcomeRendersEverySection(t *testing.T) {
	text := welcomeVisibleText(welcomeHTML(t, "Ana"))

	for _, want := range []string{
		"Bienvenido a Vibe.",
		"Hola, Ana.",
		"Ya tenés tu cuenta de Vibe. Es gratis para tu complejo y la podés usar desde el celular o la compu.",
		"Con Vibe podés:",
		"Ver todos los turnos del día en una sola pantalla.",
		"Cobrar en mostrador y cerrar la caja de cada turno.",
		"Cobrar señas online con MercadoPago.",
		"Tener tu página para que tus clientes reserven.",
		"Ver los reportes del mes, listos para tu contador.",
		"Tus primeros pasos:",
		"Cargá tu complejo: nombre, dirección y horarios.",
		"Sumá tus canchas y sus precios.",
		"Conectá MercadoPago para cobrar señas.",
		"Ir a mi panel",
		"Tutoriales para arrancar:",
		"La grilla de reservas: todo el día en una pantalla",
		"Configurá tu complejo: horarios y seña",
		"Precios por horario: cobrá la noche más cara",
		"El reporte del mes, listo para el contador",
		"Si te trabás en algo, respondé este mail y te ayudamos.",
		"Máximo, de Vibe",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("welcome is missing %q", want)
		}
	}
}

// The copy rules the brief sets for this email: voseo, no exclamation marks,
// no dashes, no digits or prices, sentences of at most twenty words.
func TestWelcomeCopyFollowsTheVoiceRules(t *testing.T) {
	got := welcomeCapture(t, "Ana")
	subject, _ := got["subject"].(string)
	text := welcomeVisibleText(welcomeHTML(t, "Ana"))

	for _, s := range []string{subject, text} {
		if strings.ContainsAny(s, "!¡—–$") {
			t.Errorf("welcome copy has a forbidden mark: %q", s)
		}
		// The three step numerals are list numbering the brief asks for; any other
		// digit would be a count or a price.
		if regexp.MustCompile(`\d`).MatchString(regexp.MustCompile(`\b\d\. `).ReplaceAllString(s, "")) {
			t.Errorf("welcome copy has a digit, numbers and prices are not allowed: %q", s)
		}
	}
	for _, sentence := range regexp.MustCompile(`[.:;!?\n]+`).Split(text, -1) {
		if n := len(strings.Fields(sentence)); n > 20 {
			t.Errorf("a sentence has %d words, the limit is 20: %q", n, sentence)
		}
	}
}

// The four tutorials are read from the landing's own data, so a slug or title
// renamed there breaks this test instead of sending owners to a 404.
func TestWelcomeTutorialsMatchTheLandingSource(t *testing.T) {
	path := filepath.Join("..", "..", "..", "landing", "src", "data", "tutoriales.ts")
	src, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("landing source not found at %s; the backend was checked out without the landing", path)
		}
		t.Fatalf("reading the landing tutorials: %v", err)
	}

	for _, tut := range welcomeTutorials {
		re := regexp.MustCompile(`slug:\s*'` + regexp.QuoteMeta(tut.slug) + `',\s*title:\s*'` + regexp.QuoteMeta(tut.title) + `',`)
		if !re.Match(src) {
			t.Errorf("tutorial %q (%q) is not in landing/src/data/tutoriales.ts; the welcome email links to it", tut.slug, tut.title)
		}
	}
}
