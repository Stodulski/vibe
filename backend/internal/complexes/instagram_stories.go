package complexes

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// This file is the owner's side of the stories Vibe publishes on a complex's
// connected Instagram account: the settings (Configuración > Instagram) and
// the latest stories. Connecting Instagram publishes nothing by itself; the
// owner turns each story type on here. The queueing and publishing live in
// instagram_story_publisher.go.

// recentStoriesShown is how many stories the panel lists.
const recentStoriesShown = 10

// The hours an owner may choose for the daily story: quiet hours are
// 23:00-08:00, and the story should go out with time left in the day.
const (
	storyDailyEarliest = "08:00"
	storyDailyLatest   = "22:00"
	storyCapMin        = 1
	storyCapMax        = 10
)

var storyTimeRX = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// InstagramStoryStore keeps the story settings and the story queue.
// complexstore.InstagramStoryStore satisfies it.
type InstagramStoryStore interface {
	GetStorySettings(ctx context.Context, complexID uuid.UUID) (complexstore.StorySettings, error)
	SaveStorySettings(ctx context.Context, complexID uuid.UUID, in complexstore.StorySettings, dailyStartsOn time.Time) (complexstore.StorySettings, error)
	ListRecentStories(ctx context.Context, complexID uuid.UUID, limit int) ([]complexstore.RecentStory, error)
	QueueFreedStory(ctx context.Context, story complexstore.NewStory) (bool, error)
	QueueStory(ctx context.Context, story complexstore.NewStory) (bool, error)
	ListDailyStoriesDue(ctx context.Context, now, today time.Time, nowHHMM, dedupKey string, limit int) ([]complexstore.DailyStoryDue, error)
	ClaimDueStories(ctx context.Context, now, lockedUntil time.Time, limit int) ([]complexstore.QueuedStory, error)
	SaveStoryState(ctx context.Context, id uuid.UUID, st complexstore.StoryState) error
	SaveStoryContainer(ctx context.Context, id uuid.UUID, containerID string, createdAt time.Time) error
	StoryAccount(ctx context.Context, complexID uuid.UUID) (*complexstore.StoryAccount, error)
	MarkStoryAccountNeedsReconnect(ctx context.Context, complexID uuid.UUID, oldExpiresAt time.Time) error
}

// StorySettings handles GET /api/v1/complexes/{id}/instagram/stories.
func (ig *Instagram) StorySettings(w http.ResponseWriter, r *http.Request) {
	complex, ok := ig.storyRoute(w, r)
	if !ok {
		return
	}
	settings, err := ig.stories.GetStorySettings(r.Context(), complex)
	if err != nil {
		ig.respond.ServerError(w, r, err)
		return
	}
	recent, err := ig.stories.ListRecentStories(r.Context(), complex, recentStoriesShown)
	if err != nil {
		ig.respond.ServerError(w, r, err)
		return
	}
	items := make([]httpx.Envelope, 0, len(recent))
	for _, s := range recent {
		var publishedAt any
		if s.PublishedAt != nil {
			publishedAt = s.PublishedAt.UTC()
		}
		items = append(items, httpx.Envelope{
			"kind": s.Kind, "status": s.Status, "scheduled_for": s.ScheduledFor.UTC(),
			"published_at": publishedAt, "reason": s.Reason,
		})
	}
	ig.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"settings": storySettingsBody(settings), "recent": items})
}

// UpdateStorySettings handles PUT /api/v1/complexes/{id}/instagram/stories.
func (ig *Instagram) UpdateStorySettings(w http.ResponseWriter, r *http.Request) {
	complex, ok := ig.storyRoute(w, r)
	if !ok {
		return
	}
	var input struct {
		DailyEnabled  *bool   `json:"daily_enabled"`
		DailyTime     *string `json:"daily_time"`
		FreedEnabled  *bool   `json:"freed_enabled"`
		FreedDailyCap *int    `json:"freed_daily_cap"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		ig.respond.BadRequest(w, r, err)
		return
	}
	v := validator.New()
	v.Check(input.DailyEnabled != nil, "daily_enabled", "must be provided")
	v.Check(input.FreedEnabled != nil, "freed_enabled", "must be provided")
	if v.Check(input.DailyTime != nil, "daily_time", "must be provided"); input.DailyTime != nil {
		t := *input.DailyTime
		v.Check(validator.Matches(t, storyTimeRX) && t >= storyDailyEarliest && t <= storyDailyLatest,
			"daily_time", "must be a time between 08:00 and 22:00 (HH:MM)")
	}
	if v.Check(input.FreedDailyCap != nil, "freed_daily_cap", "must be provided"); input.FreedDailyCap != nil {
		v.Check(*input.FreedDailyCap >= storyCapMin && *input.FreedDailyCap <= storyCapMax,
			"freed_daily_cap", "must be between 1 and 10")
	}
	if !v.Valid() {
		ig.respond.FailedValidation(w, r, v.Errors)
		return
	}
	saved, err := ig.stories.SaveStorySettings(r.Context(), complex, complexstore.StorySettings{
		DailyEnabled: *input.DailyEnabled, DailyTime: *input.DailyTime,
		FreedEnabled: *input.FreedEnabled, FreedDailyCap: *input.FreedDailyCap,
	}, dailyStoryStartsOn(ig.cfg.Now(), *input.DailyTime))
	if err != nil {
		ig.respond.ServerError(w, r, err)
		return
	}
	ig.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"settings": storySettingsBody(saved)})
}

// storyRoute applies what both story routes share: the feature must be
// offered to this owner (the same rule as connect), and the guard must have
// put the complex in the context.
func (ig *Instagram) storyRoute(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	if !ig.offeredTo(r) || ig.stories == nil {
		ig.respond.Refuse(w, r, httpx.NotImplemented("instagram stories are not available"))
		return uuid.Nil, false
	}
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		ig.respond.ServerError(w, r, errors.New("missing complex in context"))
		return uuid.Nil, false
	}
	return complex.ID, true
}

// dailyStoryStartsOn is the venue day a daily story turned on at now starts:
// today when today's daily_time is still ahead, tomorrow when it has come.
// Turning it on after its time must not publish today's story at once. The
// store applies it only to the save that turns the story on.
func dailyStoryStartsOn(now time.Time, dailyTime string) time.Time {
	venue := now.In(timezone.Argentina)
	day := timezone.Day(venue)
	if venue.Format("15:04") >= dailyTime {
		day = timezone.Day(venue.AddDate(0, 0, 1))
	}
	return day
}

func storySettingsBody(s complexstore.StorySettings) httpx.Envelope {
	return httpx.Envelope{
		"daily_enabled": s.DailyEnabled, "daily_time": s.DailyTime,
		"freed_enabled": s.FreedEnabled, "freed_daily_cap": s.FreedDailyCap,
	}
}
