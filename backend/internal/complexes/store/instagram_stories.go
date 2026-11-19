package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Story kinds and statuses, as instagram_stories stores them.
const (
	StoryKindDaily = "daily"
	StoryKindFreed = "freed"

	StoryPending   = "pending"
	StoryPublished = "published"
	StorySkipped   = "skipped"
	StoryFailed    = "failed"
)

// StorySettings is what the owner chose for the complex's stories.
type StorySettings struct {
	DailyEnabled  bool
	DailyTime     string
	FreedEnabled  bool
	FreedDailyCap int
}

// DefaultStorySettings is what a complex that never saved its settings has:
// nothing publishes until the owner turns a story type on.
func DefaultStorySettings() StorySettings {
	return StorySettings{DailyEnabled: false, DailyTime: "10:00", FreedEnabled: false, FreedDailyCap: 5}
}

// NewStory is a story to queue.
type NewStory struct {
	ComplexID    uuid.UUID
	Kind         string
	DedupKey     string
	ImageURL     string
	QueuedDay    time.Time
	SlotStartsAt *time.Time
	ScheduledFor time.Time
}

// RecentStory is one line of the panel's history.
type RecentStory struct {
	Kind         string
	Status       string
	ScheduledFor time.Time
	PublishedAt  *time.Time
	Reason       *string
}

// QueuedStory is a story a publisher run claimed.
type QueuedStory struct {
	ID                 uuid.UUID
	ComplexID          uuid.UUID
	Kind               string
	ImageURL           string
	QueuedDay          time.Time
	SlotStartsAt       *time.Time
	ScheduledFor       time.Time
	Attempts           int
	ContainerID        string
	ContainerCreatedAt *time.Time
}

// StoryState is what a run writes back to a story; it releases the lease.
type StoryState struct {
	Status             string
	Attempts           int
	NextAttemptAt      time.Time
	ContainerID        string
	ContainerCreatedAt *time.Time
	MediaID            string
	PublishedAt        *time.Time
	Reason             string
}

// DailyStoryDue is a complex whose daily story is due now.
type DailyStoryDue struct {
	ComplexID uuid.UUID
	Slug      string
}

// StoryAccount is the connection a story is published with, token opened.
type StoryAccount struct {
	InstagramUserID string
	AccessToken     string
	ExpiresAt       time.Time
	NeedsReconnect  bool
}

// InstagramStoryStore keeps the story settings and the story queue.
type InstagramStoryStore struct {
	DB   *data.DB
	Q    *db.Queries
	Keys *crypto.Keyring
}

// GetStorySettings returns the complex's settings, or the defaults when the
// owner never saved any.
func (s *InstagramStoryStore) GetStorySettings(ctx context.Context, complexID uuid.UUID) (StorySettings, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	row, err := s.Q.GetInstagramStorySettings(ctx, data.UUIDToPg(complexID))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return DefaultStorySettings(), nil
	case err != nil:
		return StorySettings{}, fmt.Errorf("complexes: read story settings: %w", err)
	}
	return StorySettings{DailyEnabled: row.DailyEnabled, DailyTime: row.DailyTime, FreedEnabled: row.FreedEnabled, FreedDailyCap: int(row.FreedDailyCap)}, nil
}

// SaveStorySettings creates or replaces the complex's settings.
func (s *InstagramStoryStore) SaveStorySettings(ctx context.Context, complexID uuid.UUID, in StorySettings) (StorySettings, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	row, err := s.Q.UpsertInstagramStorySettings(ctx, db.UpsertInstagramStorySettingsParams{
		ComplexID:     data.UUIDToPg(complexID),
		DailyEnabled:  in.DailyEnabled,
		DailyTime:     in.DailyTime,
		FreedEnabled:  in.FreedEnabled,
		FreedDailyCap: int32(min(max(in.FreedDailyCap, 0), 1000)), //nolint:gosec // G115: bounded above.
	})
	if err != nil {
		return StorySettings{}, fmt.Errorf("complexes: save story settings: %w", err)
	}
	return StorySettings{DailyEnabled: row.DailyEnabled, DailyTime: row.DailyTime, FreedEnabled: row.FreedEnabled, FreedDailyCap: int(row.FreedDailyCap)}, nil
}

// QueueFreedStory queues a freed-slot story when the complex has freed stories
// on and fewer than its cap queued on story.QueuedDay. The settings row is
// locked first, so concurrent cancellations of one complex count one after
// the other. It reports whether a story was queued; a story already queued
// under the same dedup key is not queued again.
func (s *InstagramStoryStore) QueueFreedStory(ctx context.Context, story NewStory) (bool, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()
	queued := false
	err := s.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		settings, err := q.LockInstagramStorySettings(ctx, data.UUIDToPg(story.ComplexID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // never saved: freed stories are off
		}
		if err != nil {
			return err
		}
		if !settings.FreedEnabled {
			return nil
		}
		count, err := q.CountInstagramStoriesQueuedOnDay(ctx, db.CountInstagramStoriesQueuedOnDayParams{
			ComplexID: data.UUIDToPg(story.ComplexID), Kind: StoryKindFreed, QueuedDay: data.DateToPg(story.QueuedDay),
		})
		if err != nil {
			return err
		}
		if count >= settings.FreedDailyCap {
			return nil
		}
		n, err := q.InsertInstagramStory(ctx, insertParams(story))
		queued = n > 0
		return err
	})
	if err != nil {
		return false, fmt.Errorf("complexes: queue freed story: %w", err)
	}
	return queued, nil
}

// QueueStory queues a story unless one with its dedup key exists, and reports
// whether it queued it.
func (s *InstagramStoryStore) QueueStory(ctx context.Context, story NewStory) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	n, err := s.Q.InsertInstagramStory(ctx, insertParams(story))
	if err != nil {
		return false, fmt.Errorf("complexes: queue story: %w", err)
	}
	return n > 0, nil
}

func insertParams(story NewStory) db.InsertInstagramStoryParams {
	return db.InsertInstagramStoryParams{
		ComplexID:    data.UUIDToPg(story.ComplexID),
		Kind:         story.Kind,
		DedupKey:     story.DedupKey,
		ImageUrl:     story.ImageURL,
		QueuedDay:    data.DateToPg(story.QueuedDay),
		SlotStartsAt: data.TimePtrToPg(story.SlotStartsAt),
		ScheduledFor: data.TimeToPg(story.ScheduledFor),
	}
}

// ListDailyStoriesDue returns the complexes whose daily story for dedupKey's
// day is due at nowHHMM (venue time) and not queued yet.
func (s *InstagramStoryStore) ListDailyStoriesDue(ctx context.Context, now time.Time, nowHHMM, dedupKey string, limit int) ([]DailyStoryDue, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	rows, err := s.Q.ListComplexesDueForDailyStory(ctx, db.ListComplexesDueForDailyStoryParams{
		NowHhmm: nowHHMM, Now: data.TimeToPg(now), DedupKey: dedupKey,
		RowLimit: int32(min(limit, 10_000)), //nolint:gosec // G115: bounded above.
	})
	if err != nil {
		return nil, fmt.Errorf("complexes: list daily stories due: %w", err)
	}
	out := make([]DailyStoryDue, 0, len(rows))
	for _, r := range rows {
		out = append(out, DailyStoryDue{ComplexID: data.PgToUUID(r.ComplexID), Slug: r.Slug})
	}
	return out, nil
}

// ClaimDueStories leases up to limit due pending stories until lockedUntil.
func (s *InstagramStoryStore) ClaimDueStories(ctx context.Context, now, lockedUntil time.Time, limit int) ([]QueuedStory, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	rows, err := s.Q.ClaimDueInstagramStories(ctx, db.ClaimDueInstagramStoriesParams{
		LockedUntil: data.TimeToPg(lockedUntil), Now: data.TimeToPg(now),
		RowLimit: int32(min(limit, 1000)), //nolint:gosec // G115: bounded above.
	})
	if err != nil {
		return nil, fmt.Errorf("complexes: claim due stories: %w", err)
	}
	out := make([]QueuedStory, 0, len(rows))
	for _, r := range rows {
		st := QueuedStory{
			ID: data.PgToUUID(r.ID), ComplexID: data.PgToUUID(r.ComplexID), Kind: r.Kind,
			ImageURL: r.ImageUrl, QueuedDay: data.PgToDate(r.QueuedDay), SlotStartsAt: data.PgToTimePtr(r.SlotStartsAt),
			ScheduledFor: data.PgToTime(r.ScheduledFor), Attempts: int(r.Attempts),
			ContainerCreatedAt: data.PgToTimePtr(r.ContainerCreatedAt),
		}
		if id := data.PgToTextPtr(r.ContainerID); id != nil {
			st.ContainerID = *id
		}
		out = append(out, st)
	}
	return out, nil
}

// SaveStoryState writes back what a run did to a story and releases its lease.
func (s *InstagramStoryStore) SaveStoryState(ctx context.Context, id uuid.UUID, st StoryState) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	err := s.Q.SaveInstagramStoryState(ctx, db.SaveInstagramStoryStateParams{
		ID:                 data.UUIDToPg(id),
		Status:             st.Status,
		Attempts:           int32(min(st.Attempts, 1000)), //nolint:gosec // G115: bounded above.
		NextAttemptAt:      data.TimeToPg(st.NextAttemptAt),
		ContainerID:        optionalText(st.ContainerID),
		ContainerCreatedAt: data.TimePtrToPg(st.ContainerCreatedAt),
		MediaID:            optionalText(st.MediaID),
		PublishedAt:        data.TimePtrToPg(st.PublishedAt),
		Reason:             optionalText(st.Reason),
	})
	if err != nil {
		return fmt.Errorf("complexes: save story state: %w", err)
	}
	return nil
}

// SaveStoryContainer records the container Meta created for a story, keeping
// the run's lease.
func (s *InstagramStoryStore) SaveStoryContainer(ctx context.Context, id uuid.UUID, containerID string, createdAt time.Time) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	err := s.Q.SaveInstagramStoryContainer(ctx, db.SaveInstagramStoryContainerParams{
		ContainerID: containerID, ContainerCreatedAt: data.TimeToPg(createdAt), ID: data.UUIDToPg(id),
	})
	if err != nil {
		return fmt.Errorf("complexes: save story container: %w", err)
	}
	return nil
}

func optionalText(s string) pgtype.Text {
	if s == "" {
		return data.TextToPg(nil)
	}
	return data.TextToPg(&s)
}

// ListRecentStories returns the complex's latest stories, newest first.
func (s *InstagramStoryStore) ListRecentStories(ctx context.Context, complexID uuid.UUID, limit int) ([]RecentStory, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	rows, err := s.Q.ListRecentInstagramStories(ctx, db.ListRecentInstagramStoriesParams{
		ComplexID: data.UUIDToPg(complexID), RowLimit: int32(min(limit, 100)), //nolint:gosec // G115: bounded above.
	})
	if err != nil {
		return nil, fmt.Errorf("complexes: list recent stories: %w", err)
	}
	out := make([]RecentStory, 0, len(rows))
	for _, r := range rows {
		out = append(out, RecentStory{
			Kind: r.Kind, Status: r.Status, ScheduledFor: data.PgToTime(r.ScheduledFor),
			PublishedAt: data.PgToTimePtr(r.PublishedAt), Reason: data.PgToTextPtr(r.Reason),
		})
	}
	return out, nil
}

// StoryAccount returns the complex's connection with its token opened, or
// data.ErrRecordNotFound when it has none.
func (s *InstagramStoryStore) StoryAccount(ctx context.Context, complexID uuid.UUID) (*StoryAccount, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	row, err := s.Q.GetInstagramStoryAccount(ctx, data.UUIDToPg(complexID))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, data.ErrRecordNotFound
	case err != nil:
		return nil, fmt.Errorf("complexes: read story account: %w", err)
	}
	token, err := s.Keys.Open(instagramTokenAAD(complexID), row.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("complexes: open instagram token: %w", err)
	}
	return &StoryAccount{
		InstagramUserID: row.InstagramUserID, AccessToken: token,
		ExpiresAt: data.PgToTime(row.ExpiresAt), NeedsReconnect: row.NeedsReconnect,
	}, nil
}

// MarkStoryAccountNeedsReconnect flags the complex's connection after Meta
// refused its token while publishing.
func (s *InstagramStoryStore) MarkStoryAccountNeedsReconnect(ctx context.Context, complexID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	if _, err := s.Q.MarkInstagramStoryAccountNeedsReconnect(ctx, data.UUIDToPg(complexID)); err != nil {
		return fmt.Errorf("complexes: mark story account reconnect: %w", err)
	}
	return nil
}
