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

// mockPlaceWaitlistStore satisfies stores.PlaceWaitlistStore for the route
// harness. The place waitlist's own behaviour is covered in internal/waitlist;
// here every place can be waited for and nothing is waiting.
type mockPlaceWaitlistStore struct{}

func (m *mockPlaceWaitlistStore) WaitlistableEvent(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return nil
}

func (m *mockPlaceWaitlistStore) WaitlistableCategoryEvent(context.Context, uuid.UUID, uuid.UUID, time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *mockPlaceWaitlistStore) WaitlistableCourt(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockPlaceWaitlistStore) InsertEvent(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}

func (m *mockPlaceWaitlistStore) InsertSlot(context.Context, waitliststore.NewSlotEntry) error {
	return nil
}

func (m *mockPlaceWaitlistStore) ClaimFreedEvents(context.Context, time.Time, int) ([]waitliststore.FreedEventEntry, error) {
	return nil, nil
}

func (m *mockPlaceWaitlistStore) ListWaitingSlots(context.Context, time.Time, int) ([]waitliststore.WaitingSlotEntry, error) {
	return nil, nil
}

func (m *mockPlaceWaitlistStore) Claim(context.Context, []uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *mockPlaceWaitlistStore) DeletePast(context.Context, time.Time) (int64, error) { return 0, nil }
