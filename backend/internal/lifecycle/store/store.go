// Package store persists the state the owner lifecycle emails read and write: the
// suppression list, the record of each nudge sent, the lead claims and the owner
// activity stamp.
package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/lifecycle"
)

// Store is the sqlc-backed lifecycle store. None of its own tables is tenant data,
// but the candidate queries read complexes, courts and bookings, which are. The
// cron wrapper runs them on the cross-tenant bypass, the same as every other sweep.
type Store struct {
	Q *db.Queries
}

// Suppress records an address that opted out of lifecycle email. Idempotent.
func (s *Store) Suppress(ctx context.Context, email, reason string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.InsertEmailSuppression(ctx, db.InsertEmailSuppressionParams{Email: email, Reason: reason})
}

// ListCandidates returns the owners due a nudge of kind, oldest first, at most limit.
// The queries already exclude anyone who has that nudge, anyone suppressed, and anyone
// whose state no longer matches the kind.
func (s *Store) ListCandidates(ctx context.Context, kind lifecycle.Kind, limit int32) ([]lifecycle.Candidate, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var out []lifecycle.Candidate
	switch kind {
	case lifecycle.KindVerifyEmail:
		rows, err := s.Q.ListVerifyEmailNudgeCandidates(ctx, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, candidate(r.ID, r.Email, r.FirstName, r.ComplexName))
		}
	case lifecycle.KindCreateComplex:
		rows, err := s.Q.ListCreateComplexNudgeCandidates(ctx, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, candidate(r.ID, r.Email, r.FirstName, r.ComplexName))
		}
	case lifecycle.KindAddCourts:
		rows, err := s.Q.ListAddCourtsNudgeCandidates(ctx, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, candidate(r.ID, r.Email, r.FirstName, r.ComplexName))
		}
	case lifecycle.KindConnectMP:
		rows, err := s.Q.ListConnectMPNudgeCandidates(ctx, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, candidate(r.ID, r.Email, r.FirstName, r.ComplexName))
		}
	case lifecycle.KindInactive:
		rows, err := s.Q.ListInactiveNudgeCandidates(ctx, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, candidate(r.ID, r.Email, r.FirstName, r.ComplexName))
		}
	case lifecycle.KindFinishSignup:
		return nil, fmt.Errorf("lifecycle store: %q is a lead kind, use ListFinishSignupCandidates", kind)
	default:
		return nil, fmt.Errorf("lifecycle store: no owner candidate query for kind %q", kind)
	}
	return out, nil
}

func candidate(id pgtype.UUID, email, firstName, complexName string) lifecycle.Candidate {
	return lifecycle.Candidate{UserID: data.PgToUUID(id), Email: email, FirstName: firstName, ComplexName: complexName}
}

// ClaimOwnerNudge records that a nudge of kind is going to owner userID. It reports
// false when that owner already has one of this kind.
func (s *Store) ClaimOwnerNudge(ctx context.Context, userID uuid.UUID, kind lifecycle.Kind) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	n, err := s.Q.ClaimOwnerNudge(ctx, db.ClaimOwnerNudgeParams{UserID: data.UUIDToPg(userID), Kind: string(kind)})
	return n == 1, err
}

// ReleaseOwnerNudge removes a claim whose email never reached the queue.
func (s *Store) ReleaseOwnerNudge(ctx context.Context, userID uuid.UUID, kind lifecycle.Kind) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.ReleaseOwnerNudge(ctx, db.ReleaseOwnerNudgeParams{UserID: data.UUIDToPg(userID), Kind: string(kind)})
}

// ListFinishSignupCandidates returns the leads due the finish_signup nudge, oldest
// first, at most limit.
func (s *Store) ListFinishSignupCandidates(ctx context.Context, limit int32) ([]lifecycle.LeadCandidate, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.Q.ListFinishSignupNudgeCandidates(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]lifecycle.LeadCandidate, 0, len(rows))
	for _, r := range rows {
		out = append(out, lifecycle.LeadCandidate{Email: r.Email, FirstName: r.FirstName})
	}
	return out, nil
}

// ClaimLeadNudge sets the lead's nudged_at once. It reports false when it was
// already set, or when the lead does not exist.
func (s *Store) ClaimLeadNudge(ctx context.Context, email string) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	n, err := s.Q.ClaimRegistrationLeadNudge(ctx, email)
	return n == 1, err
}

// ReleaseLeadNudge clears a lead's nudged_at, for a claim whose email never queued.
func (s *Store) ReleaseLeadNudge(ctx context.Context, email string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.ReleaseRegistrationLeadNudge(ctx, email)
}

// TouchOwnerLastActive stamps an owner's activity, at most once an hour. It reports
// whether the stamp was written; the database refuses a write inside the hour.
func (s *Store) TouchOwnerLastActive(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	n, err := s.Q.TouchOwnerLastActive(ctx, data.UUIDToPg(id))
	return n == 1, err
}
