package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// mockInstagramStoryStore is the Instagram story store of the cmd/api harness:
// default settings, nothing queued, no account. The stories' own behaviour is
// covered in internal/complexes and its store.
type mockInstagramStoryStore struct{}

func (m *mockInstagramStoryStore) GetStorySettings(context.Context, uuid.UUID) (complexstore.StorySettings, error) {
	return complexstore.DefaultStorySettings(), nil
}

func (m *mockInstagramStoryStore) SaveStorySettings(_ context.Context, _ uuid.UUID, in complexstore.StorySettings, _ time.Time) (complexstore.StorySettings, error) {
	return in, nil
}

func (m *mockInstagramStoryStore) ListRecentStories(context.Context, uuid.UUID, int) ([]complexstore.RecentStory, error) {
	return nil, nil
}

func (m *mockInstagramStoryStore) QueueFreedStory(context.Context, complexstore.NewStory) (bool, error) {
	return false, nil
}

func (m *mockInstagramStoryStore) QueueStory(context.Context, complexstore.NewStory) (bool, error) {
	return false, nil
}

func (m *mockInstagramStoryStore) ListDailyStoriesDue(context.Context, time.Time, time.Time, string, string, int) ([]complexstore.DailyStoryDue, error) {
	return nil, nil
}

func (m *mockInstagramStoryStore) ClaimDueStories(context.Context, time.Time, time.Time, int) ([]complexstore.QueuedStory, error) {
	return nil, nil
}

func (m *mockInstagramStoryStore) SaveStoryState(context.Context, uuid.UUID, complexstore.StoryState) error {
	return nil
}

func (m *mockInstagramStoryStore) SaveStoryContainer(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}

func (m *mockInstagramStoryStore) StoryAccount(context.Context, uuid.UUID) (*complexstore.StoryAccount, error) {
	return nil, data.ErrRecordNotFound
}

func (m *mockInstagramStoryStore) MarkStoryAccountNeedsReconnect(context.Context, uuid.UUID, time.Time) error {
	return nil
}
