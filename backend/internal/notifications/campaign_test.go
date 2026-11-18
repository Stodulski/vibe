package notifications

import (
	"encoding/json"
	"testing"
)

// The benefits email is delivered by a registered worker. A task type with no
// handler would fail silently for every owner, so the registration is pinned here.
// One-off campaign (2026-10). Remove with the campaign.
func TestBenefitsCampaignWorkerDeliversTheEmail(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()

	payload, _ := json.Marshal(BenefitsCampaignEmail{
		Email:     "ana@example.com",
		FirstName: "Ana",
		Segment:   "create_complex",
	})
	h, ok := q.handlers[TaskEmailBenefitsCampaign]
	if !ok {
		t.Fatalf("no worker is registered for %s", TaskEmailBenefitsCampaign)
	}
	if err := h(t.Context(), payload); err != nil {
		t.Fatalf("delivering the benefits email: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "benefits_campaign" || mailer.sent[0].to != "ana@example.com" {
		t.Errorf("the benefits email was not delivered; got %v", mailer.sent)
	}
}

// The social kit email is delivered by a registered worker too. One-off campaign
// (2026-11). Remove with the campaign.
func TestSocialKitCampaignWorkerDeliversTheEmail(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()

	payload, _ := json.Marshal(SocialKitCampaignEmail{
		Email:     "ana@example.com",
		FirstName: "Ana",
		ImageURL:  "https://vibe.test/kit/c/a/reservar-post.png",
	})
	h, ok := q.handlers[TaskEmailSocialKitCampaign]
	if !ok {
		t.Fatalf("no worker is registered for %s", TaskEmailSocialKitCampaign)
	}
	if err := h(t.Context(), payload); err != nil {
		t.Fatalf("delivering the social kit email: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "social_kit_campaign" || mailer.sent[0].to != "ana@example.com" {
		t.Errorf("the social kit email was not delivered; got %v", mailer.sent)
	}
}
