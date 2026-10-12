package lifecycle

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/notifications"
)

// ownerKinds is the order of precedence. A pass sends an owner at most one nudge,
// the first of these that is due for them.
var ownerKinds = []Kind{KindVerifyEmail, KindCreateComplex, KindAddCourts, KindInactive}

// perKindLimit bounds one pass's work per kind, so a backlog cannot turn into a
// burst of email. The next pass takes the rest.
const perKindLimit = 100

// NudgeSender enqueues one lifecycle email. notifications.Service satisfies it.
type NudgeSender interface {
	LifecycleNudge(e notifications.LifecycleNudgeEmail, subjectID string)
}

// VerificationIssuer mints a fresh email-verification token for an unverified
// owner. auth.Service satisfies it.
type VerificationIssuer interface {
	IssueVerificationToken(ctx context.Context, userID uuid.UUID) (string, error)
}

// NudgeConfig holds the origins the emails link to.
type NudgeConfig struct {
	// FrontendURL is the owner panel origin (https://app.vibe.com.ar).
	FrontendURL string
	// PublicBaseURL is the storefront origin, where the /baja page lives.
	PublicBaseURL string
	// BackendURL is the API origin the List-Unsubscribe header points at.
	BackendURL string
}

// Nudger runs one pass of the lifecycle emails: it finds who is due, claims each
// nudge before sending it, and enqueues the email.
type Nudger struct {
	owners OwnerNudgeStore
	leads  LeadNudgeStore
	notify NudgeSender
	verify VerificationIssuer
	signer *Signer
	cfg    NudgeConfig
	logger *slog.Logger
}

// NewNudger returns a nudger over the given stores and sender.
func NewNudger(owners OwnerNudgeStore, leads LeadNudgeStore, notify NudgeSender, verify VerificationIssuer,
	signer *Signer, cfg NudgeConfig, logger *slog.Logger,
) *Nudger {
	return &Nudger{owners: owners, leads: leads, notify: notify, verify: verify, signer: signer, cfg: cfg, logger: logger}
}

// Run makes one pass and reports how many emails it enqueued. A failed step is
// logged and the pass continues; the next pass retries whatever it missed,
// because a claim is released whenever its email never reached the queue.
func (n *Nudger) Run(ctx context.Context) int {
	sent := 0
	picked := make(map[uuid.UUID]bool)

	for _, kind := range ownerKinds {
		candidates, err := n.owners.ListCandidates(ctx, kind, perKindLimit)
		if err != nil {
			n.logger.Error("lifecycle: listing nudge candidates failed", "kind", kind, "error", err)
			continue
		}
		for _, c := range candidates {
			if picked[c.UserID] {
				continue
			}
			if n.nudgeOwner(ctx, kind, c) {
				picked[c.UserID] = true
				sent++
			}
		}
	}

	leads, err := n.leads.ListFinishSignupCandidates(ctx, perKindLimit)
	if err != nil {
		n.logger.Error("lifecycle: listing lead candidates failed", "error", err)
		return sent
	}
	for _, l := range leads {
		if n.nudgeLead(ctx, l) {
			sent++
		}
	}
	return sent
}

func (n *Nudger) nudgeOwner(ctx context.Context, kind Kind, c Candidate) bool {
	claimed, err := n.owners.ClaimOwnerNudge(ctx, c.UserID, kind)
	if err != nil {
		n.logger.Error("lifecycle: claiming nudge failed", "kind", kind, "error", err)
		return false
	}
	if !claimed {
		return false
	}

	e, ok := n.ownerEmail(ctx, kind, c)
	if !ok {
		// Nothing reached the queue, so the claim must not stand: the next pass
		// should try this owner again.
		if err := n.owners.ReleaseOwnerNudge(ctx, c.UserID, kind); err != nil {
			n.logger.Error("lifecycle: releasing unsent nudge failed", "kind", kind, "error", err)
		}
		return false
	}
	n.notify.LifecycleNudge(e, c.UserID.String())
	return true
}

// ownerEmail builds the email for an owner nudge. For verify_email it mints the
// link first, which is the one step here that can refuse.
func (n *Nudger) ownerEmail(ctx context.Context, kind Kind, c Candidate) (notifications.LifecycleNudgeEmail, bool) {
	var cta string
	switch kind {
	case KindVerifyEmail:
		plain, err := n.verify.IssueVerificationToken(ctx, c.UserID)
		if err != nil {
			n.logger.Info("lifecycle: no verification link issued this pass", "error", err)
			return notifications.LifecycleNudgeEmail{}, false
		}
		cta = n.cfg.FrontendURL + "/verify-email?token=" + url.QueryEscape(plain)
	case KindCreateComplex:
		cta = n.cfg.FrontendURL + "/onboarding"
	case KindAddCourts:
		cta = n.cfg.FrontendURL + "/courts"
	case KindInactive:
		cta = n.cfg.FrontendURL + "/dashboard"
	default:
		return notifications.LifecycleNudgeEmail{}, false
	}
	return n.email(kind, c.Email, c.FirstName, c.ComplexName, cta), true
}

func (n *Nudger) nudgeLead(ctx context.Context, l LeadCandidate) bool {
	claimed, err := n.leads.ClaimLeadNudge(ctx, l.Email)
	if err != nil {
		n.logger.Error("lifecycle: claiming lead nudge failed", "error", err)
		return false
	}
	if !claimed {
		return false
	}
	e := n.email(KindFinishSignup, l.Email, l.FirstName, "", n.cfg.FrontendURL+"/register")
	n.notify.LifecycleNudge(e, l.Email)
	return true
}

// email assembles one nudge, with both opt-out links signed for its recipient.
func (n *Nudger) email(kind Kind, to, firstName, complexName, cta string) notifications.LifecycleNudgeEmail {
	token := n.signer.Sign(to)
	return notifications.LifecycleNudgeEmail{
		Kind:           string(kind),
		Email:          to,
		FirstName:      firstName,
		ComplexName:    complexName,
		CTAURL:         cta,
		UnsubscribeURL: n.cfg.BackendURL + "/api/v1/public/email/unsubscribe?token=" + url.QueryEscape(token),
		OptOutURL:      n.cfg.PublicBaseURL + "/baja?token=" + url.QueryEscape(token),
	}
}
