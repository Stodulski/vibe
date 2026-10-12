package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	waitliststore "github.com/stodulski/vibe-server/internal/waitlist/store"
)

// mockWaitlistStore satisfies stores.WaitlistStore for the route harness. The
// waitlist's own behaviour is covered in internal/waitlist; here it answers
// empty, which is all the route and wiring tests need.
type mockWaitlistStore struct{}

func (m *mockWaitlistStore) Insert(context.Context, uuid.UUID, time.Time, string) error { return nil }

func (m *mockWaitlistStore) ClaimUnnotified(context.Context, uuid.UUID, time.Time) ([]waitliststore.Claimed, error) {
	return nil, nil
}

func (m *mockWaitlistStore) DeleteBefore(context.Context, time.Time) (int64, error) { return 0, nil }
