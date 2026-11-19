package complexes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/instagram"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// This file queues and publishes the stories of complexes that connected
// Instagram and turned stories on (instagram_stories.go has the settings).
//
// Two kinds of story are queued as pending rows of instagram_stories:
//
//   - daily: the day's free hours, once per venue day, at or after the time the
//     owner picked. The cron job queues it (QueueDailyStories).
//   - freed: a slot a cancellation put back on sale. The booking service calls
//     SlotCancelled, which only inserts a row: nothing is published inside the
//     request that cancelled.
//
// PublishDue publishes them. The cron job runs every minute under a two-minute
// deadline, and Meta asks for a container to be checked for up to five minutes
// before it is published, so one run never waits for a container: a run fetches
// the image, checks the quota, creates the container and checks it once; a
// container still processing is stored on the row and a later run checks it
// again and publishes it. Waiting for a container, for quiet hours to end or
// for a full quota spends no attempt; only failures do.

// The publishing rules.
const (
	// storyMinLead is how far ahead a freed slot must start for its story to
	// be published; storyMaxLead how far ahead it may start at most.
	storyMinLead = time.Hour
	storyMaxLead = 48 * time.Hour
	// Quiet hours, venue time: nothing is published from 23:00 to 08:00.
	storyQuietStartHour = 23
	storyQuietEndHour   = 8
	// storyQuotaWait is how long a story waits when the account's publishing
	// limit is used up (a rolling 24 hours).
	storyQuotaWait = time.Hour
	// storyContainerRecheck is when a container still processing is checked
	// again, and storyContainerPatience how long it may stay processing before
	// the attempt counts as failed (Meta advises up to five minutes).
	storyContainerRecheck  = time.Minute
	storyContainerPatience = 10 * time.Minute
	// storyDeferDelay is the wait after a failure that is not the story's own
	// (a database error, an open circuit breaker): no attempt is spent.
	storyDeferDelay = 5 * time.Minute
	// storyLease is how long a run holds the stories it claimed.
	storyLease      = 3 * time.Minute
	storyClaimBatch = 20
	storyDailyBatch = 500
	// storyDeadlineMargin is how much of the run must be left to start a story.
	storyDeadlineMargin = 30 * time.Second
	maxStoryReason      = 500
	maxStoryImageBytes  = 16 << 20
)

// defaultStoryBackoff is the wait before each retry of a retryable failure; its
// length plus one is the attempt budget.
var defaultStoryBackoff = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour}

// StoryPublishingAPI publishes on a complex's Instagram account.
// *instagram.Client satisfies it.
type StoryPublishingAPI interface {
	CreateStory(ctx context.Context, igUserID, token, imageURL string) (string, error)
	ContainerStatus(ctx context.Context, containerID, token string) (string, error)
	PublishContainer(ctx context.Context, igUserID, token, containerID string) (string, error)
	PublishingLimit(ctx context.Context, igUserID, token string) (used, total int, err error)
}

// InstagramStoriesConfig tunes the publisher.
type InstagramStoriesConfig struct {
	// Available is config.Instagram.Available(); off, nothing is queued or
	// published.
	Available bool
	// PublicURL is the storefront origin the story images are served from.
	PublicURL string
	Backoff   []time.Duration
	Now       func() time.Time
}

// InstagramStoriesDeps is what the publisher needs from outside.
type InstagramStoriesDeps struct {
	API   StoryPublishingAPI
	Store InstagramStoryStore
	// Images fetches the story image before Instagram is asked to; it needs an
	// explicit timeout.
	Images *http.Client
	Logger *slog.Logger
}

// InstagramStories queues and publishes complexes' stories.
type InstagramStories struct {
	api    StoryPublishingAPI
	store  InstagramStoryStore
	images *http.Client
	logger *slog.Logger
	cfg    InstagramStoriesConfig
}

// NewInstagramStories returns the publisher.
func NewInstagramStories(deps InstagramStoriesDeps, cfg InstagramStoriesConfig) *InstagramStories {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if len(cfg.Backoff) == 0 {
		cfg.Backoff = defaultStoryBackoff
	}
	if deps.Images == nil {
		deps.Images = &http.Client{Timeout: 20 * time.Second}
	}
	return &InstagramStories{api: deps.API, store: deps.Store, images: deps.Images, logger: deps.Logger, cfg: cfg}
}

// SlotCancelled queues the story of a slot a cancellation freed, when the
// complex wants freed stories, its Instagram is usable, the slot starts at
// least an hour after the story could go out and within 48 hours, and the
// day's cap is not reached. It never fails the cancellation: every problem is
// logged. ctx is scoped to the complex here.
func (s *InstagramStories) SlotCancelled(ctx context.Context, complexID, courtID uuid.UUID, slug string, startsAt, endsAt time.Time) {
	if !s.cfg.Available || slug == "" {
		return
	}
	ctx = data.ContextWithTenant(ctx, complexID)
	now := s.cfg.Now()
	if startsAt.Sub(now) > storyMaxLead {
		return
	}
	publishAt := now
	if quietHours(now) {
		publishAt = quietHoursEnd(now)
	}
	if startsAt.Sub(publishAt) < storyMinLead {
		return
	}
	if !s.usableAccount(ctx, complexID) {
		return
	}
	slot := startsAt.In(timezone.Argentina)
	day, hour := slot.Format("2006-01-02"), slot.Format("15:04")
	q := url.Values{
		"fecha":    {day},
		"hora":     {hour},
		"cancha":   {courtID.String()},
		"duracion": {strconv.Itoa(int(endsAt.Sub(startsAt).Minutes()))},
	}
	starts := startsAt
	queued, err := s.store.QueueFreedStory(ctx, complexstore.NewStory{
		ComplexID:    complexID,
		Kind:         complexstore.StoryKindFreed,
		DedupKey:     "freed:" + courtID.String() + ":" + day + ":" + hour,
		ImageURL:     s.imageURL(slug, "liberado", q),
		QueuedDay:    timezone.Day(now.In(timezone.Argentina)),
		SlotStartsAt: &starts,
		ScheduledFor: publishAt,
	})
	if err != nil {
		s.logger.Error("instagram stories: could not queue a freed slot", "complex_id", complexID, "error", err)
		return
	}
	if queued {
		s.logger.Info("instagram stories: freed slot queued", "complex_id", complexID, "court_id", courtID, "starts_at", startsAt.UTC())
	}
}

// usableAccount reports whether the complex has a connection Vibe may publish
// with: present, not flagged and not expired.
func (s *InstagramStories) usableAccount(ctx context.Context, complexID uuid.UUID) bool {
	acct, err := s.store.StoryAccount(ctx, complexID)
	if err != nil {
		if !errors.Is(err, data.ErrRecordNotFound) {
			s.logger.Error("instagram stories: could not read the connection", "complex_id", complexID, "error", err)
		}
		return false
	}
	return !acct.NeedsReconnect && acct.ExpiresAt.After(s.cfg.Now())
}

// QueueDailyStories queues today's daily story for every complex whose time
// has come, and returns how many it queued. Nothing is queued in quiet hours:
// a day's story that did not go out by 23:00 is not published the next
// morning. ctx must cross tenants.
func (s *InstagramStories) QueueDailyStories(ctx context.Context) int {
	if !s.cfg.Available {
		return 0
	}
	now := s.cfg.Now()
	if quietHours(now) {
		return 0
	}
	venue := now.In(timezone.Argentina)
	day := venue.Format("2006-01-02")
	dedup := "daily:" + day
	due, err := s.store.ListDailyStoriesDue(ctx, now, venue.Format("15:04"), dedup, storyDailyBatch)
	if err != nil {
		s.logger.Error("instagram stories: could not list the daily stories due", "error", err)
		return 0
	}
	queued := 0
	for _, d := range due {
		ok, err := s.store.QueueStory(ctx, complexstore.NewStory{
			ComplexID:    d.ComplexID,
			Kind:         complexstore.StoryKindDaily,
			DedupKey:     dedup,
			ImageURL:     s.imageURL(d.Slug, "libres", url.Values{"fecha": {day}}),
			QueuedDay:    timezone.Day(venue),
			ScheduledFor: now,
		})
		if err != nil {
			s.logger.Error("instagram stories: could not queue a daily story", "complex_id", d.ComplexID, "error", err)
			continue
		}
		if ok {
			queued++
		}
	}
	return queued
}

func (s *InstagramStories) imageURL(slug, piece string, q url.Values) string {
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/historias/c/" + url.PathEscape(slug) + "/" + piece + ".jpg?" + q.Encode()
}

// StoryRunOutcome is what one publishing run did, for its log line.
type StoryRunOutcome struct {
	Claimed, Published, Skipped, Failed, Retrying, Waiting int
}

// PublishDue works through the due stories. ctx must cross tenants (the cron
// wrapper sets the bypass).
func (s *InstagramStories) PublishDue(ctx context.Context) StoryRunOutcome {
	var out StoryRunOutcome
	if !s.cfg.Available {
		return out
	}
	now := s.cfg.Now()
	stories, err := s.store.ClaimDueStories(ctx, now, now.Add(storyLease), storyClaimBatch)
	if err != nil {
		s.logger.Error("instagram stories: could not claim due stories", "error", err)
		return out
	}
	out.Claimed = len(stories)
	for i := range stories {
		st := &stories[i]
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < storyDeadlineMargin {
			// Hand it back for the next run rather than start a Meta call
			// the deadline would cut short.
			s.save(ctx, st, s.waiting(st, s.cfg.Now()), "")
			out.Waiting++
			continue
		}
		state, token := s.process(ctx, st)
		s.save(ctx, st, state, token)
		switch state.Status {
		case complexstore.StoryPublished:
			out.Published++
		case complexstore.StorySkipped:
			out.Skipped++
		case complexstore.StoryFailed:
			out.Failed++
		default:
			if state.Attempts > st.Attempts {
				out.Retrying++
			} else {
				out.Waiting++
			}
		}
	}
	return out
}

// errStoryImageGone is the image route's 404: nothing true to show.
var errStoryImageGone = errors.New("the story image has nothing to show")

// process takes one story as far as it can go in this run, and returns its new
// state and the token it used (to be scrubbed from anything stored or logged).
func (s *InstagramStories) process(ctx context.Context, st *complexstore.QueuedStory) (complexstore.StoryState, string) {
	now := s.cfg.Now()
	acct, stop := s.gate(ctx, st, now)
	if stop != nil {
		return *stop, ""
	}
	token := acct.AccessToken

	if st.ContainerID == "" {
		if stop := s.createContainer(ctx, st, acct, now); stop != nil {
			return *stop, token
		}
	}

	status, err := s.api.ContainerStatus(ctx, st.ContainerID, token)
	if err != nil {
		return s.failed(ctx, st, err, true), token
	}
	switch status {
	case instagram.ContainerFinished:
	case instagram.ContainerInProgress, "":
		if st.ContainerCreatedAt != nil && now.Sub(*st.ContainerCreatedAt) > storyContainerPatience {
			return s.failed(ctx, st, fmt.Errorf("%w: the container was still processing after %s", instagram.ErrUnavailable, storyContainerPatience), false), token
		}
		return s.waiting(st, now.Add(storyContainerRecheck)), token
	default:
		// ERROR, EXPIRED, or anything Meta adds: a new container may work.
		return s.failed(ctx, st, fmt.Errorf("%w: the container ended as %s", instagram.ErrUnavailable, status), false), token
	}

	mediaID, err := s.api.PublishContainer(ctx, acct.InstagramUserID, token, st.ContainerID)
	if err != nil {
		return s.failed(ctx, st, err, instagram.GraphCode(err) != instagram.CodeResourceMissing), token
	}
	published := s.cfg.Now()
	return complexstore.StoryState{
		Status: complexstore.StoryPublished, Attempts: st.Attempts + 1, NextAttemptAt: published,
		ContainerID: st.ContainerID, ContainerCreatedAt: st.ContainerCreatedAt,
		MediaID: mediaID, PublishedAt: &published,
	}, token
}

// gate decides whether the story may be published now and returns the account
// to publish with. A non-nil state stops the story here: quiet hours, the
// owner turned it off, its day or its slot passed, or the connection is gone.
func (s *InstagramStories) gate(ctx context.Context, st *complexstore.QueuedStory, now time.Time) (*complexstore.StoryAccount, *complexstore.StoryState) {
	stop := func(state complexstore.StoryState) (*complexstore.StoryAccount, *complexstore.StoryState) {
		return nil, &state
	}
	if quietHours(now) {
		w := s.waiting(st, quietHoursEnd(now))
		w.Reason = "quiet hours"
		return stop(w)
	}
	settings, err := s.store.GetStorySettings(ctx, st.ComplexID)
	if err != nil {
		return stop(s.deferred(st, now, err))
	}
	switch {
	case st.Kind == complexstore.StoryKindDaily && !settings.DailyEnabled,
		st.Kind == complexstore.StoryKindFreed && !settings.FreedEnabled:
		return stop(s.skipped(st, "the owner turned this story off"))
	case st.Kind == complexstore.StoryKindDaily && !sameDay(st.QueuedDay, now):
		return stop(s.skipped(st, "the day is over"))
	case st.Kind == complexstore.StoryKindFreed && (st.SlotStartsAt == nil || st.SlotStartsAt.Sub(now) < storyMinLead):
		return stop(s.skipped(st, "the slot starts in less than an hour"))
	}
	acct, err := s.store.StoryAccount(ctx, st.ComplexID)
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		return stop(s.skipped(st, "instagram is not connected"))
	case err != nil:
		return stop(s.deferred(st, now, err))
	case acct.NeedsReconnect || !acct.ExpiresAt.After(now):
		return stop(s.skipped(st, "instagram needs to be connected again"))
	}
	return acct, nil
}

// createContainer fetches the image, checks the quota and creates the story's
// container, recording it on st. A non-nil state stops the story here.
func (s *InstagramStories) createContainer(ctx context.Context, st *complexstore.QueuedStory, acct *complexstore.StoryAccount, now time.Time) *complexstore.StoryState {
	stop := func(state complexstore.StoryState) *complexstore.StoryState { return &state }
	if err := s.fetchImage(ctx, st.ImageURL); err != nil {
		if errors.Is(err, errStoryImageGone) {
			return stop(s.skipped(st, "nothing free to show any more"))
		}
		return stop(s.failed(ctx, st, err, false))
	}
	used, total, err := s.api.PublishingLimit(ctx, acct.InstagramUserID, acct.AccessToken)
	if err != nil {
		return stop(s.failed(ctx, st, err, false))
	}
	if used >= total {
		w := s.waiting(st, now.Add(storyQuotaWait))
		w.Reason = "instagram daily publishing limit reached"
		return stop(w)
	}
	id, err := s.api.CreateStory(ctx, acct.InstagramUserID, acct.AccessToken, st.ImageURL)
	if err != nil {
		return stop(s.failed(ctx, st, err, false))
	}
	created := now
	st.ContainerID, st.ContainerCreatedAt = id, &created
	return nil
}

// fetchImage asks the story image route for the image Instagram will fetch. A
// 404 means there is nothing true to show; anything but a 200 is retried.
func (s *InstagramStories) fetchImage(ctx context.Context, imageURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return fmt.Errorf("story image: %w", err)
	}
	resp, err := s.images.Do(req)
	if err != nil {
		return fmt.Errorf("story image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxStoryImageBytes)); err != nil {
		return fmt.Errorf("story image: read: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return errStoryImageGone
	default:
		return fmt.Errorf("story image answered %d", resp.StatusCode)
	}
}

// waiting keeps the story pending until at, spending no attempt and keeping
// its container.
func (s *InstagramStories) waiting(st *complexstore.QueuedStory, at time.Time) complexstore.StoryState {
	return complexstore.StoryState{
		Status: complexstore.StoryPending, Attempts: st.Attempts, NextAttemptAt: at,
		ContainerID: st.ContainerID, ContainerCreatedAt: st.ContainerCreatedAt,
	}
}

// deferred is a wait after a failure that is not the story's own.
func (s *InstagramStories) deferred(st *complexstore.QueuedStory, now time.Time, err error) complexstore.StoryState {
	s.logger.Error("instagram stories: deferring a story", "story_id", st.ID, "complex_id", st.ComplexID, "error", err)
	w := s.waiting(st, now.Add(storyDeferDelay))
	w.Reason = "temporarily unavailable, retrying later"
	return w
}

// skipped is the final state of a story there is nothing (or no right) to
// publish.
func (s *InstagramStories) skipped(st *complexstore.QueuedStory, reason string) complexstore.StoryState {
	return complexstore.StoryState{
		Status: complexstore.StorySkipped, Attempts: st.Attempts, NextAttemptAt: s.cfg.Now(),
		ContainerID: st.ContainerID, ContainerCreatedAt: st.ContainerCreatedAt, Reason: reason,
	}
}

// failed classifies a failed attempt. An invalid token fails the story and
// flags the connection; Meta being down, throttling, a container not ready or
// missing, or a failed image fetch is retried with backoff until the attempts
// run out; any other refusal fails at once. An open circuit breaker is a wait
// that spends no attempt. keepContainer keeps the container for the retry.
func (s *InstagramStories) failed(ctx context.Context, st *complexstore.QueuedStory, err error, keepContainer bool) complexstore.StoryState {
	now := s.cfg.Now()
	if errors.Is(err, circuitbreaker.ErrOpen) {
		w := s.waiting(st, now.Add(storyDeferDelay))
		w.Reason = "instagram is unavailable, retrying later"
		return w
	}
	code := instagram.GraphCode(err)
	state := complexstore.StoryState{Status: complexstore.StoryFailed, Attempts: st.Attempts + 1, NextAttemptAt: now, Reason: err.Error()}
	if keepContainer {
		state.ContainerID, state.ContainerCreatedAt = st.ContainerID, st.ContainerCreatedAt
	}
	if code == instagram.CodeInvalidToken {
		if markErr := s.store.MarkStoryAccountNeedsReconnect(ctx, st.ComplexID); markErr != nil {
			s.logger.Error("instagram stories: could not flag the connection for reconnect", "complex_id", st.ComplexID, "error", markErr)
		}
		return state
	}
	retryable := code == instagram.CodeMediaNotReady || code == instagram.CodeResourceMissing ||
		errors.Is(err, instagram.ErrUnavailable) || !errors.Is(err, instagram.ErrRejected)
	if retryable && state.Attempts <= len(s.cfg.Backoff) {
		state.Status = complexstore.StoryPending
		state.NextAttemptAt = now.Add(s.cfg.Backoff[state.Attempts-1])
	}
	return state
}

// save writes the story's state back, with the token scrubbed from its
// reason, and logs how it ended. A failed write is logged; the lease expires
// and a later run takes the story again.
func (s *InstagramStories) save(ctx context.Context, st *complexstore.QueuedStory, state complexstore.StoryState, token string) {
	state.Reason = scrubStoryReason(state.Reason, token)
	if err := s.store.SaveStoryState(ctx, st.ID, state); err != nil {
		s.logger.Error("instagram stories: could not save a story", "story_id", st.ID, "complex_id", st.ComplexID, "error", err)
	}
	args := []any{"story_id", st.ID, "complex_id", st.ComplexID, "kind", st.Kind, "status", state.Status, "attempts", state.Attempts}
	switch {
	case state.Status == complexstore.StoryPublished:
		s.logger.Info("instagram stories: published", args...)
	case state.Status == complexstore.StoryFailed:
		s.logger.Warn("instagram stories: failed", append(args, "reason", state.Reason)...)
	case state.Reason != "":
		s.logger.Info("instagram stories: not published", append(args, "reason", state.Reason)...)
	}
}

// scrubStoryReason removes the token from a reason and bounds its length.
func scrubStoryReason(reason, token string) string {
	if token != "" {
		reason = strings.ReplaceAll(reason, token, "[redacted]")
	}
	if len(reason) > maxStoryReason {
		reason = reason[:maxStoryReason]
	}
	return reason
}

// quietHours reports whether t is between 23:00 and 08:00 venue time.
func quietHours(t time.Time) bool {
	h := t.In(timezone.Argentina).Hour()
	return h >= storyQuietStartHour || h < storyQuietEndHour
}

// quietHoursEnd is the 08:00 (venue time) that ends the quiet hours t is in.
func quietHoursEnd(t time.Time) time.Time {
	v := t.In(timezone.Argentina)
	end := time.Date(v.Year(), v.Month(), v.Day(), storyQuietEndHour, 0, 0, 0, timezone.Argentina)
	if v.Hour() >= storyQuietStartHour {
		end = end.AddDate(0, 0, 1)
	}
	return end
}

// sameDay reports whether day (a date) is t's venue day.
func sameDay(day, t time.Time) bool {
	v := t.In(timezone.Argentina)
	y, m, d := day.Date()
	return y == v.Year() && m == v.Month() && d == v.Day()
}
