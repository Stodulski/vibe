package notifications

import "context"

// TaskEmailBenefitsCampaign is the one-off "beneficios de Vibe" email. The campaign
// package claims each owner durably and schedules the task itself, with a run time
// spaced one second apart, so it does not go through Service.
//
// One-off campaign (2026-10). Remove after it has been sent.
const TaskEmailBenefitsCampaign = "email:campaign_benefits_2026_10"

// BenefitsCampaignEmail is one owner's copy, fully resolved before it is queued: the
// address, the segment that picks the last line, and the links. Both opt-out links
// are signed at schedule time.
type BenefitsCampaignEmail struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	// Segment is one of the campaign's segment names; it picks the last line.
	Segment string `json:"segment"`
	// SegmentURL is the target of that line's link.
	SegmentURL string `json:"segment_url"`
	// PanelURL is the button's target, the owner's dashboard.
	PanelURL string `json:"panel_url"`
	// UnsubscribeURL is the one-click endpoint, sent as the List-Unsubscribe header.
	UnsubscribeURL string `json:"unsubscribe_url"`
	// OptOutURL is the /baja page, linked from the footer.
	OptOutURL string `json:"opt_out_url"`
}

// registerCampaignWorkers wires the delivery of the campaign emails. RegisterWorkers
// calls it.
func (s *Service) registerCampaignWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailBenefitsCampaign, func(ctx context.Context, p BenefitsCampaignEmail) error {
		return s.mailer.SendBenefitsCampaign(ctx, p.Email, p.FirstName, p.Segment, p.SegmentURL,
			p.PanelURL, p.UnsubscribeURL, p.OptOutURL)
	}))
	s.queue.RegisterHandler(handle(TaskEmailSocialKitCampaign, func(ctx context.Context, p SocialKitCampaignEmail) error {
		return s.mailer.SendSocialKitCampaign(ctx, p.Email, p.FirstName, p.ImageURL, p.KitURL,
			p.UnsubscribeURL, p.OptOutURL)
	}))
}

// TaskEmailSocialKitCampaign is the one-off email announcing the social kit. The
// campaign package schedules it the same way as the benefits email.
//
// One-off campaign (2026-11). Remove after it has been sent.
const TaskEmailSocialKitCampaign = "email:campaign_social_kit_2026_11"

// SocialKitCampaignEmail is one owner's copy, fully resolved before it is queued.
type SocialKitCampaignEmail struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	// ImageURL is the owner's own "Link de reservas" post, shown in the email.
	ImageURL string `json:"image_url"`
	// KitURL is the button's target, the kit page in the panel.
	KitURL string `json:"kit_url"`
	// UnsubscribeURL is the one-click endpoint, sent as the List-Unsubscribe header.
	UnsubscribeURL string `json:"unsubscribe_url"`
	// OptOutURL is the /baja page, linked from the footer.
	OptOutURL string `json:"opt_out_url"`
}
