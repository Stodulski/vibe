// Package store reads the one-off benefits campaign's recipients and records its
// claims. One-off campaign (2026-10). Remove after it has been sent.
package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/campaign"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Store is the sqlc-backed campaign store. The recipient query reads complexes,
// which are tenant data, so the cron wrapper runs it on the cross-tenant bypass.
type Store struct {
	Q *db.Queries
}

// ListRecipients returns the owners the campaign has not reached yet, at most limit.
func (s *Store) ListRecipients(ctx context.Context, limit int32) ([]campaign.Recipient, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.Q.ListCampaignRecipients(ctx, db.ListCampaignRecipientsParams{
		Campaign: campaign.CampaignName,
		MaxRows:  limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]campaign.Recipient, 0, len(rows))
	for _, r := range rows {
		out = append(out, campaign.Recipient{
			UserID:    data.PgToUUID(r.ID),
			Email:     r.Email,
			FirstName: r.FirstName,
			Slug:      r.Slug,
			Segment:   r.Segment,
		})
	}
	return out, nil
}

// Claim records that owner userID is being sent the campaign. It reports false when
// the owner already has it.
func (s *Store) Claim(ctx context.Context, userID uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	n, err := s.Q.ClaimCampaign(ctx, db.ClaimCampaignParams{
		Campaign: campaign.CampaignName,
		UserID:   data.UUIDToPg(userID),
	})
	return n == 1, err
}
