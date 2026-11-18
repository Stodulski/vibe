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

// One-off campaign (2026-11). Remove with the campaign.

const (
	kitTestImage = "https://vibe.test/kit/c/cancha-norte/reservar-post.png"
	kitTestURL   = "https://app.vibe.test/kit"
)

// sentKit sends one social kit email through a fake Brevo and returns the decoded
// request body.
func sentKit(t *testing.T, firstName string) map[string]any {
	t.Helper()
	var body []byte
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL
	if err := m.SendSocialKitCampaign(context.Background(), "ana@example.com", firstName,
		kitTestImage, kitTestURL, benefitsUnsub, benefitsOptOut); err != nil {
		t.Fatalf("SendSocialKitCampaign: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decoding the Brevo request: %v", err)
	}
	return got
}

func TestSocialKitCampaignCarriesTheApprovedCopy(t *testing.T) {
	got := sentKit(t, "Ana")

	body, _ := got["htmlContent"].(string)
	if !strings.Contains(body, `src="`+kitTestImage+`"`) {
		t.Error("the email does not show the owner's own post")
	}
	if strings.Count(body, `href="`+kitTestURL+`"`) < 2 {
		t.Error("the image and the button must both open the kit in the panel")
	}
	if !strings.Contains(body, `href="`+benefitsOptOut+`"`) {
		t.Error("the footer has no opt-out link")
	}
}

func TestSocialKitCampaignEscapesTheFirstName(t *testing.T) {
	body, _ := sentKit(t, "<b>Ana</b>")["htmlContent"].(string)
	if strings.Contains(body, "<b>Ana</b>") || !strings.Contains(body, "&lt;b&gt;Ana&lt;/b&gt;") {
		t.Error("a first name typed as markup must reach the email escaped")
	}
}

func TestSocialKitCampaignCarriesListUnsubscribeHeaders(t *testing.T) {
	got := sentKit(t, "Ana")
	headers, _ := got["headers"].(map[string]any)
	lu, _ := headers["List-Unsubscribe"].(string)
	if !strings.Contains(lu, "<"+benefitsUnsub+">") {
		t.Errorf("List-Unsubscribe = %q, want the one-click endpoint", lu)
	}
	if headers["List-Unsubscribe-Post"] != "List-Unsubscribe=One-Click" {
		t.Errorf("List-Unsubscribe-Post = %v", headers["List-Unsubscribe-Post"])
	}
	reply, _ := got["replyTo"].(map[string]any)
	if reply["email"] != "hola@vibe.com.ar" {
		t.Errorf("replyTo = %v, want hola@vibe.com.ar", got["replyTo"])
	}
}

func TestSocialKitCampaignCopyUsesNoForbiddenPunctuation(t *testing.T) {
	for _, line := range []string{kitSubject, kitPreheader, kitImageAlt} {
		for _, bad := range []string{"!", "¡", "—"} {
			if strings.Contains(line, bad) {
				t.Errorf("the copy %q contains %q", line, bad)
			}
		}
	}
}
