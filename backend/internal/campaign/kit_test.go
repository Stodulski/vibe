package campaign

import (
	"context"
	"testing"

	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/lifecycle"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// One-off campaign (2026-11). Remove with the campaign.

func kitPayloadOf(t *testing.T, s scheduled) notifications.SocialKitCampaignEmail {
	t.Helper()
	p, ok := s.payload.(notifications.SocialKitCampaignEmail)
	if !ok {
		t.Fatalf("payload is %T; want notifications.SocialKitCampaignEmail", s.payload)
	}
	return p
}

func TestKitOutsideProductionNothingIsReadOrSent(t *testing.T) {
	store := newFakeStore(recipient("", "cancha-norte"))
	sched := &fakeScheduler{}
	cfg := testConfig
	cfg.Env = "staging"
	c := NewKit(store, sched, lifecycle.NewSigner("secret", ""), cfg, discard())

	if n := c.Run(context.Background()); n != 0 {
		t.Fatalf("Run = %d outside production; want 0", n)
	}
	if store.listCalls != 0 {
		t.Error("the recipients were read outside production")
	}
	if len(sched.got) != 0 {
		t.Errorf("%d emails were scheduled outside production", len(sched.got))
	}
}

func TestKitEveryRecipientGetsTheirOwnPostAndTheKitLink(t *testing.T) {
	a := recipient("", "cancha-norte")
	b := recipient("", "los-aromos")
	store := newFakeStore(a, b)
	sched := &fakeScheduler{}
	signer := lifecycle.NewSigner("secret", "")
	c := NewKit(store, sched, signer, testConfig, discard())

	if n := c.Run(context.Background()); n != 2 {
		t.Fatalf("Run = %d; want 2", n)
	}
	if len(sched.got) != 2 {
		t.Fatalf("scheduled %d emails; want 2", len(sched.got))
	}
	for i, r := range []Recipient{a, b} {
		got := sched.got[i]
		if got.jobType != notifications.TaskEmailSocialKitCampaign {
			t.Errorf("job type = %q; want %q", got.jobType, notifications.TaskEmailSocialKitCampaign)
		}
		wantKey := jobs.DedupKey(notifications.TaskEmailSocialKitCampaign, "campaign:social-kit-2026-11:"+r.UserID.String())
		if got.dedupKey != wantKey {
			t.Errorf("dedup key for owner %d is not the kit campaign key for that owner", i)
		}
		p := kitPayloadOf(t, got)
		if p.Email != r.Email || p.FirstName != "Ana" {
			t.Errorf("payload = %+v; want the recipient's address and name", p)
		}
		if want := "https://vibe.test/kit/c/" + r.Slug + "/reservar-post.png?email=1"; p.ImageURL != want {
			t.Errorf("image = %q; want %q", p.ImageURL, want)
		}
		if p.KitURL != "https://app.vibe.test/kit" {
			t.Errorf("kit link = %q; want the panel's kit page", p.KitURL)
		}
		assertTokenFor(t, signer, p.UnsubscribeURL, r.Email)
		assertTokenFor(t, signer, p.OptOutURL, r.Email)
	}
	if sched.got[1].runAt.Sub(sched.got[0].runAt).Seconds() != 1 {
		t.Errorf("the two sends are %v apart; want one second", sched.got[1].runAt.Sub(sched.got[0].runAt))
	}
}

func TestKitASecondPassEnqueuesNothingForOwnersAlreadyClaimed(t *testing.T) {
	store := newFakeStore(recipient("", "a"), recipient("", "b"))
	sched := &fakeScheduler{}
	c := NewKit(store, sched, lifecycle.NewSigner("secret", ""), testConfig, discard())

	c.Run(context.Background())
	if n := c.Run(context.Background()); n != 0 {
		t.Fatalf("second Run = %d; want 0", n)
	}
	if len(sched.got) != 2 {
		t.Fatalf("scheduled %d emails across both passes; want 2", len(sched.got))
	}
}
