package campaign

// One-off campaign (2026-11). Remove after it has been sent: this file, its store,
// the notification task, the mailer copy, and the cron job that calls Run.

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/lifecycle"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// KitCampaignName keys the claims and the dedup keys of the social kit email.
const KitCampaignName = "social-kit-2026-11"

// KitCampaign is one pass of the social kit email: it tells every owner with a live
// complex that the panel has a kit of images for their social networks.
type KitCampaign struct {
	store  Store
	sched  Scheduler
	signer *lifecycle.Signer
	cfg    Config
	logger *slog.Logger
}

// NewKit returns the social kit campaign over the given store and scheduler. The
// store lists only owners whose complex is live, so every recipient has a slug.
func NewKit(store Store, sched Scheduler, signer *lifecycle.Signer, cfg Config, logger *slog.Logger) *KitCampaign {
	return &KitCampaign{store: store, sched: sched, signer: signer, cfg: cfg, logger: logger}
}

// Run makes one pass and reports how many emails it scheduled, with the same
// once-only claims and one-second spacing as the benefits campaign. Only production
// sends.
func (c *KitCampaign) Run(ctx context.Context) int {
	if !Enabled(c.cfg.Env) {
		return 0
	}
	return pass(ctx, KitCampaignName, c.store, c.schedule, c.logger)
}

func (c *KitCampaign) schedule(r Recipient, runAt time.Time) {
	key := jobs.DedupKey(notifications.TaskEmailSocialKitCampaign, "campaign:"+KitCampaignName+":"+r.UserID.String())
	c.sched.EnqueueAt(notifications.TaskEmailSocialKitCampaign, c.email(r), runAt, key)
}

// email assembles one owner's email: the image is their own "Link de reservas" post at the email size,
// the button opens the kit in the panel, and both opt-out links are signed for them.
func (c *KitCampaign) email(r Recipient) notifications.SocialKitCampaignEmail {
	token := c.signer.Sign(r.Email)
	return notifications.SocialKitCampaignEmail{
		Email:          r.Email,
		FirstName:      r.FirstName,
		ImageURL:       c.cfg.PublicBaseURL + "/kit/c/" + url.PathEscape(r.Slug) + "/reservar-post.png?email=1",
		KitURL:         c.cfg.FrontendURL + "/kit",
		UnsubscribeURL: c.cfg.BackendURL + "/api/v1/public/email/unsubscribe?token=" + url.QueryEscape(token),
		OptOutURL:      c.cfg.PublicBaseURL + "/baja?token=" + url.QueryEscape(token),
	}
}
