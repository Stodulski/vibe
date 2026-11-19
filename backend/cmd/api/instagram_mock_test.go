package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// mockInstagramStore is the Instagram connection store of the cmd/api
// harness: no complex is connected and nothing is due. The connection's own
// behaviour is covered in internal/complexes and its store.
type mockInstagramStore struct{}

func (m *mockInstagramStore) SaveInstagramConnection(context.Context, *complexstore.InstagramConnection) error {
	return nil
}

func (m *mockInstagramStore) GetInstagramConnection(context.Context, uuid.UUID) (*complexstore.InstagramConnection, error) {
	return nil, data.ErrRecordNotFound
}

func (m *mockInstagramStore) DeleteInstagramConnection(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockInstagramStore) ListInstagramTokensDueForRefresh(context.Context, time.Time, time.Time, time.Time, int) ([]complexstore.DueInstagramToken, []uuid.UUID, error) {
	return nil, nil, nil
}

func (m *mockInstagramStore) UpdateInstagramToken(context.Context, uuid.UUID, time.Time, string, time.Time, time.Time) (bool, error) {
	return false, nil
}

func (m *mockInstagramStore) MarkExpiredInstagramConnections(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (m *mockInstagramStore) MarkInstagramNeedsReconnect(context.Context, uuid.UUID, time.Time) (bool, error) {
	return false, nil
}

func (m *mockInstagramStore) InstagramComplexOwner(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
