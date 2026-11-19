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
	benefitsUnsub   = "https://api.vibe.test/api/v1/public/email/unsubscribe?token=tok"
	benefitsOptOut  = "https://vibe.test/baja?token=tok"
	benefitsPanel   = "https://app.vibe.test/dashboard"
	benefitsOnboard = "https://app.vibe.test/onboarding"
)

// sentBenefits sends one benefits email through a fake Brevo and returns the decoded
// request body the transport would have sent.
func sentBenefits(t *testing.T, firstName, segment, segmentURL string) map[string]any {
	t.Helper()
	var body []byte
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL
	if err := m.SendBenefitsCampaign(context.Background(), "ana@example.com", firstName, segment,
		segmentURL, benefitsPanel, benefitsUnsub, benefitsOptOut); err != nil {
		t.Fatalf("SendBenefitsCampaign(%s): %v", segment, err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decoding the Brevo request: %v", err)
	}
	return got
}

func TestBenefitsCampaignCarriesTheApprovedCopy(t *testing.T) {
	got := sentBenefits(t, "Ana", "create_complex", benefitsOnboard)

	body, _ := got["htmlContent"].(string)
	for href, label := range map[string]string{
		benefitsPanel:   "Ir a mi panel",
		benefitsOnboard: "Cargar mi complejo",
		benefitsOptOut:  "date de baja acá",
	} {
		if !strings.Contains(body, `href="`+href+`"`) {
			t.Errorf("no %q link to %s", label, href)
		}
	}
}

func TestBenefitsCampaignSegmentDecidesTheLastLine(t *testing.T) {
	cases := []struct {
		segment, line, label, url string
	}{
		{"create_complex", "Cargá tu complejo en 5 minutos.", "Cargar mi complejo", benefitsOnboard},
		{"connect_mp", "Conectá MercadoPago para cobrar señas.", "Conectar MercadoPago", "https://app.vibe.test/settings?tab=billing"},
		{"share_page", "Compartí tu página de reservas.", "Ver mi página", "https://vibe.test/c/cancha-norte"},
	}
	for _, tc := range cases {
		t.Run(tc.segment, func(t *testing.T) {
			body, _ := sentBenefits(t, "Ana", tc.segment, tc.url)["htmlContent"].(string)
			if !strings.Contains(body, tc.line) {
				t.Errorf("segment %s is missing its line %q", tc.segment, tc.line)
			}
			if !strings.Contains(body, `href="`+tc.url+`"`) || !strings.Contains(body, tc.label) {
				t.Errorf("segment %s is missing its link %q to %s", tc.segment, tc.label, tc.url)
			}
		})
	}
}

func TestBenefitsCampaignGreetingWithoutAName(t *testing.T) {
	body, _ := sentBenefits(t, "  ", "create_complex", benefitsOnboard)["htmlContent"].(string)
	if !strings.Contains(body, "Hola,") || strings.Contains(body, "Hola ,") {
		t.Errorf("a missing first name must greet with a bare \"Hola,\"")
	}
}

func TestBenefitsCampaignEscapesTheFirstName(t *testing.T) {
	body, _ := sentBenefits(t, "<b>Ana</b>", "create_complex", benefitsOnboard)["htmlContent"].(string)
	if strings.Contains(body, "<b>Ana</b>") {
		t.Error("a first name typed as markup reached the email unescaped")
	}
	if !strings.Contains(body, "&lt;b&gt;Ana&lt;/b&gt;") {
		t.Error("the escaped first name is missing from the greeting")
	}
}

func TestBenefitsCampaignCarriesListUnsubscribeHeadersOverBrevo(t *testing.T) {
	got := sentBenefits(t, "Ana", "share_page", "https://vibe.test/c/cancha-norte")

	headers, ok := got["headers"].(map[string]any)
	if !ok {
		t.Fatalf("no headers object in the Brevo request: %v", got)
	}
	lu, _ := headers["List-Unsubscribe"].(string)
	if !strings.Contains(lu, "<"+benefitsUnsub+">") {
		t.Errorf("List-Unsubscribe = %q, want the one-click endpoint", lu)
	}
	if headers["List-Unsubscribe-Post"] != "List-Unsubscribe=One-Click" {
		t.Errorf("List-Unsubscribe-Post = %v, want the RFC 8058 one-click value", headers["List-Unsubscribe-Post"])
	}
	reply, _ := got["replyTo"].(map[string]any)
	if reply["email"] != "hola@vibe.com.ar" {
		t.Errorf("replyTo = %v, want hola@vibe.com.ar", got["replyTo"])
	}
}

// The shared wrapper's conditional comments contain "!", so the rule is checked on the
// copy itself, as the lifecycle suite checks its copy.
func TestBenefitsCampaignCopyUsesNoForbiddenPunctuation(t *testing.T) {
	copyLines := []string{benefitsSubject, benefitsPreheader}
	for _, item := range benefitsItems {
		copyLines = append(copyLines, item.heading, item.text)
	}
	for _, seg := range benefitsSegments {
		copyLines = append(copyLines, seg.line, seg.label)
	}
	for _, line := range copyLines {
		for _, bad := range []string{"!", "¡", "—"} {
			if strings.Contains(line, bad) {
				t.Errorf("the copy %q contains %q", line, bad)
			}
		}
	}
}

func TestBenefitsCampaignUnknownSegmentIsAPermanentFailure(t *testing.T) {
	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	err := m.SendBenefitsCampaign(context.Background(), "ana@example.com", "Ana", "unknown", benefitsOnboard,
		benefitsPanel, benefitsUnsub, benefitsOptOut)
	if err == nil || !isPermanent(err) {
		t.Fatalf("an unknown segment = %v; want a permanent failure", err)
	}
}
