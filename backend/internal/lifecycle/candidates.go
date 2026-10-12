package lifecycle

import (
	"context"

	"github.com/google/uuid"
)

// Kind names one lifecycle email. The owner kinds are the values owner_nudges
// stores; finish_signup is the one that goes to a lead.
type Kind string

// The nudge kinds.
const (
	KindVerifyEmail   Kind = "verify_email"
	KindCreateComplex Kind = "create_complex"
	KindAddCourts     Kind = "add_courts"
	KindInactive      Kind = "inactive"
	KindFinishSignup  Kind = "finish_signup"
)

// Candidate is an owner due a nudge, with what the email needs to name them.
type Candidate struct {
	UserID      uuid.UUID
	Email       string
	FirstName   string
	ComplexName string
}

// LeadCandidate is a registration lead due the finish_signup nudge.
type LeadCandidate struct {
	Email     string
	FirstName string
}

// OwnerNudgeStore lists owners due a nudge and records the nudges sent.
type OwnerNudgeStore interface {
	ListCandidates(ctx context.Context, kind Kind, limit int32) ([]Candidate, error)
	// ClaimOwnerNudge records that a nudge is going out. It reports false when
	// the owner already has one of this kind, which makes each kind once-only.
	ClaimOwnerNudge(ctx context.Context, userID uuid.UUID, kind Kind) (bool, error)
	ReleaseOwnerNudge(ctx context.Context, userID uuid.UUID, kind Kind) error
}

// LeadNudgeStore lists leads due the finish_signup nudge and records it sent.
type LeadNudgeStore interface {
	ListFinishSignupCandidates(ctx context.Context, limit int32) ([]LeadCandidate, error)
	ClaimLeadNudge(ctx context.Context, email string) (bool, error)
	ReleaseLeadNudge(ctx context.Context, email string) error
}
