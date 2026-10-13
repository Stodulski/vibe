// Package campaign sends the one-off "beneficios de Vibe" email to every registered
// owner, once. It runs as a cron pass, like the lifecycle nudges.
//
// One-off campaign (2026-10). Remove after it has been sent: the package, its
// store, the campaign_claims table's writer, the notification task, the mailer
// copy, and the cron job that calls Run.
package campaign

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/lifecycle"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// CampaignName keys the claims and the dedup keys of this campaign.
const CampaignName = "benefits-2026-10"

// batchSize bounds one read of the recipient list.
const batchSize = 200

// The last line of the email. Each owner gets exactly one, decided by their live
// complexes: none, a live one without MercadoPago, or a connected one.
const (
	SegmentCreateComplex = "create_complex"
	SegmentConnectMP     = "connect_mp"
	SegmentSharePage     = "share_page"
)

// Recipient is an owner the campaign has not reached yet, with what the email names.
type Recipient struct {
	UserID    uuid.UUID
	Email     string
	FirstName string
	Slug      string
	Segment   string
}

// Store lists the owners still to be reached and records each one as reached.
type Store interface {
	ListRecipients(ctx context.Context, limit int32) ([]Recipient, error)
	// Claim reports false when the owner was already claimed, which makes the
	// campaign once-only per owner whatever the queue does with redeliveries.
	Claim(ctx context.Context, userID uuid.UUID) (bool, error)
}

// Scheduler enqueues one email to run at a given time. *jobs.Enqueuer satisfies it.
type Scheduler interface {
	EnqueueAt(jobType string, payload any, runAt time.Time, dedupKey string)
}

// Config holds the origins the email's links point at.
type Config struct {
	// Env gates sending: only "production" sends.
	Env string
	// FrontendURL is the owner panel origin.
	FrontendURL string
	// PublicBaseURL is the storefront origin, where /c/<slug> and /baja live.
	PublicBaseURL string
	// BackendURL is the API origin the List-Unsubscribe header points at.
	BackendURL string
}

// Campaign is one pass of the benefits email.
type Campaign struct {
	store  Store
	sched  Scheduler
	signer *lifecycle.Signer
	cfg    Config
	logger *slog.Logger
}

// New returns a campaign over the given store and scheduler.
func New(store Store, sched Scheduler, signer *lifecycle.Signer, cfg Config, logger *slog.Logger) *Campaign {
	return &Campaign{store: store, sched: sched, signer: signer, cfg: cfg, logger: logger}
}

// Enabled reports whether the campaign may send in an environment. Only production
// sends, as IndexNow does, so a dev or staging deploy cannot reach a real owner.
func Enabled(env string) bool {
	return env == "production"
}

// Run makes one pass and reports how many emails it scheduled. Each owner is claimed
// durably before its email is scheduled, and claimed owners leave the list, so a
// pass after the first one schedules nothing for owners already reached. Sends are
// spaced one second apart, in the order the owners were listed.
//
// A failed read or claim ends the pass; the next pass picks up whatever is left.
func (c *Campaign) Run(ctx context.Context) int {
	if !Enabled(c.cfg.Env) {
		return 0
	}
	now := time.Now()
	sent := 0
	for {
		batch, err := c.store.ListRecipients(ctx, batchSize)
		if err != nil {
			c.logger.Error("campaign: listing recipients failed", "campaign", CampaignName, "error", err)
			return sent
		}
		claimed := 0
		for _, r := range batch {
			ok, err := c.store.Claim(ctx, r.UserID)
			if err != nil {
				c.logger.Error("campaign: claiming an owner failed", "campaign", CampaignName, "error", err)
				return sent
			}
			if !ok {
				continue
			}
			c.schedule(r, now.Add(time.Duration(sent)*time.Second))
			sent++
			claimed++
		}
		if claimed == 0 || len(batch) < batchSize {
			return sent
		}
	}
}

func (c *Campaign) schedule(r Recipient, runAt time.Time) {
	key := jobs.DedupKey(notifications.TaskEmailBenefitsCampaign, "campaign:"+CampaignName+":"+r.UserID.String())
	c.sched.EnqueueAt(notifications.TaskEmailBenefitsCampaign, c.email(r), runAt, key)
}

// email assembles one owner's email, with both opt-out links signed for them.
func (c *Campaign) email(r Recipient) notifications.BenefitsCampaignEmail {
	token := c.signer.Sign(r.Email)
	return notifications.BenefitsCampaignEmail{
		Email:          r.Email,
		FirstName:      r.FirstName,
		Segment:        r.Segment,
		SegmentURL:     segmentURL(c.cfg, r.Segment, r.Slug),
		PanelURL:       c.cfg.FrontendURL + "/dashboard",
		UnsubscribeURL: c.cfg.BackendURL + "/api/v1/public/email/unsubscribe?token=" + url.QueryEscape(token),
		OptOutURL:      c.cfg.PublicBaseURL + "/baja?token=" + url.QueryEscape(token),
	}
}

// segmentURL is the target of the last line's link.
func segmentURL(cfg Config, segment, slug string) string {
	switch segment {
	case SegmentConnectMP:
		return cfg.FrontendURL + "/settings?tab=billing"
	case SegmentSharePage:
		return cfg.PublicBaseURL + "/c/" + url.PathEscape(slug)
	default:
		return cfg.FrontendURL + "/onboarding"
	}
}
