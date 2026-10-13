package main

import (
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/scheduler"
)

// The one-off benefits campaign (2026-10) is scheduled every ten minutes, and its
// run is a no-op when there is no campaign to run. Remove with the campaign.
func TestCampaignBenefitsIsScheduledEveryTenMinutes(t *testing.T) {
	f := newCronFixture(t)

	jobs := f.app.cronJobs()
	var found *scheduler.Job
	for i := range jobs {
		if jobs[i].Name == "campaign_benefits_2026_10" {
			found = &jobs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("campaign_benefits_2026_10 is not registered")
	}
	if found.Every != 10*time.Minute {
		t.Errorf("campaign_benefits_2026_10 runs every %v; want 10m", found.Every)
	}
	found.Run(t.Context())
}
