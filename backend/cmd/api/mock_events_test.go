package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
)

// mockEventStore is an empty store of open-play events: it holds none, so a
// lookup answers not found and the sweeps have nothing to do.
type mockEventStore struct {
	// GetEventFn answers GetEvent; unset, the event is not found.
	GetEventFn func(ctx context.Context, id uuid.UUID) (*eventstore.Event, error)
	// CancelEventFn answers CancelEvent; unset, the event is not found.
	CancelEventFn func(ctx context.Context, id, by uuid.UUID, reason *string) (*eventstore.Event, []uuid.UUID, error)
}

func (m *mockEventStore) CreateEvent(context.Context, *eventstore.Event, []uuid.UUID) error {
	return nil
}

func (m *mockEventStore) CancelEvent(ctx context.Context, id, by uuid.UUID, reason *string) (*eventstore.Event, []uuid.UUID, error) {
	if m.CancelEventFn != nil {
		return m.CancelEventFn(ctx, id, by, reason)
	}
	return nil, nil, data.ErrRecordNotFound
}

func (m *mockEventStore) GetEvent(ctx context.Context, id uuid.UUID) (*eventstore.Event, error) {
	if m.GetEventFn != nil {
		return m.GetEventFn(ctx, id)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockEventStore) ListEvents(context.Context, uuid.UUID, time.Time, time.Time, bool) ([]*eventstore.Event, error) {
	return nil, nil
}

func (m *mockEventStore) SignUp(context.Context, eventstore.SignUpInput) (*eventstore.Registration, string, error) {
	return nil, "", data.ErrRecordNotFound
}

func (m *mockEventStore) GetRegistration(context.Context, uuid.UUID) (*eventstore.Registration, error) {
	return nil, data.ErrRecordNotFound
}

func (m *mockEventStore) GetRegistrationByToken(context.Context, string) (*eventstore.Registration, error) {
	return nil, data.ErrRecordNotFound
}

func (m *mockEventStore) ListRegistrations(context.Context, uuid.UUID, bool) ([]*eventstore.RegistrationRow, error) {
	return nil, nil
}

func (m *mockEventStore) CancelRegistration(context.Context, uuid.UUID, string) (*eventstore.Registration, error) {
	return nil, eventstore.ErrNotCancellable
}

func (m *mockEventStore) ExpireHolds(context.Context) ([]eventstore.ExpiredHold, error) {
	return nil, nil
}

func (m *mockEventStore) OwnerCancelledStillPaid(context.Context, time.Duration, int) ([]eventstore.PendingRefund, error) {
	return nil, nil
}
