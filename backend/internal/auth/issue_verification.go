package auth

import (
	"context"

	"github.com/google/uuid"
)

// IssueVerificationToken mints a verification token for an account and returns
// its plaintext, for a caller that sends its own email (the owner lifecycle
// nudge). It takes the same cooldown as a resend, so a recent resend makes it
// refuse with data.ErrCooldownActive rather than mint a second live link.
func (s *Service) IssueVerificationToken(ctx context.Context, userID uuid.UUID) (string, error) {
	plaintext := uuid.New().String()
	if err := s.verifications.InsertWithCooldown(ctx, userID, hashToken(plaintext)); err != nil {
		return "", err
	}
	return plaintext, nil
}
