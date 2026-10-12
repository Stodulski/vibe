package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/lifecycle"
)

// mockLeadStore satisfies stores.RegistrationLeadStore for the route harness. The
// lead rules are covered in internal/leads and internal/lifecycle; here every
// write succeeds and nothing is found, which is all the route tests need.
type mockLeadStore struct{}

func (m *mockLeadStore) Upsert(context.Context, string, string, string, string) error { return nil }

func (m *mockLeadStore) DeleteOlderThan30Days(context.Context) (int64, error) { return 0, nil }

// mockLifecycleStore satisfies stores.LifecycleStore for the route harness. It
// answers as an empty database would: no owner is due a nudge, every write succeeds.
type mockLifecycleStore struct{}

func (m *mockLifecycleStore) ListCandidates(context.Context, lifecycle.Kind, int32) ([]lifecycle.Candidate, error) {
	return nil, nil
}

func (m *mockLifecycleStore) ClaimOwnerNudge(context.Context, uuid.UUID, lifecycle.Kind) (bool, error) {
	return false, nil
}

func (m *mockLifecycleStore) ReleaseOwnerNudge(context.Context, uuid.UUID, lifecycle.Kind) error {
	return nil
}

func (m *mockLifecycleStore) ListFinishSignupCandidates(context.Context, int32) ([]lifecycle.LeadCandidate, error) {
	return nil, nil
}

func (m *mockLifecycleStore) ClaimLeadNudge(context.Context, string) (bool, error) { return false, nil }

func (m *mockLifecycleStore) ReleaseLeadNudge(context.Context, string) error { return nil }

func (m *mockLifecycleStore) Suppress(context.Context, string, string) error { return nil }

func (m *mockLifecycleStore) TouchOwnerLastActive(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
