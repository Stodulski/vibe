package social

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

// storyPlatform records every Instagram and Facebook publish in call order and
// fails the names it is told to fail, as a transient Meta error.
type storyPlatform struct {
	igAttempts, fbAttempts []string
	igOrder, fbOrder       []string
	failIG                 map[string]bool
	quotaUsed, quotaTotal  int
	seq                    int
}

func (s *storyPlatform) InstagramQuota(context.Context) (int, int, error) {
	total := s.quotaTotal
	if total == 0 {
		total = 50
	}
	return s.quotaUsed, total, nil
}

func (s *storyPlatform) PublishInstagram(_ context.Context, p Post) (string, error) {
	name := p.Media[0].Name
	s.igAttempts = append(s.igAttempts, name)
	if s.failIG[name] {
		return "", errors.New("meta 500: try again")
	}
	s.igOrder = append(s.igOrder, name)
	s.seq++
	return fmt.Sprintf("ig-%d", s.seq), nil
}

func (s *storyPlatform) PublishFacebook(_ context.Context, p Post) (string, error) {
	name := p.Media[0].Name
	s.fbAttempts = append(s.fbAttempts, name)
	s.fbOrder = append(s.fbOrder, name)
	s.seq++
	return fmt.Sprintf("fb-%d", s.seq), nil
}

func (s *storyPlatform) InstagramPermalink(_ context.Context, id string) (string, error) {
	return "https://www.instagram.com/stories/vibe/" + id + "/", nil
}

func (s *storyPlatform) PublishThreads(context.Context, Post) (string, error) {
	return "", errors.New("threads is not used by stories")
}

func (s *storyPlatform) ThreadsPermalink(context.Context, string) (string, error) {
	return "", nil
}

// sequenceStore is an in-memory store that also knows the feed posts a story
// can follow. Feeds are read through Lookup only, never claimed.
type sequenceStore struct {
	posts map[string]Post
	feeds map[string]Post
}

func (s *sequenceStore) ClaimDue(_ context.Context, now time.Time, limit int, lease time.Duration) ([]Post, error) {
	var out []Post
	for id, p := range s.posts {
		if len(out) == limit || p.Draft || p.NextAttemptAt == nil || p.NextAttemptAt.After(now) {
			continue
		}
		if p.Status != StatusScheduled && p.Status != StatusPartial && p.Status != StatusPublishing {
			continue
		}
		p.Status = StatusPublishing
		until := now.Add(lease)
		p.NextAttemptAt = &until
		s.posts[id] = p
		out = append(out, p)
	}
	return out, nil
}

func (s *sequenceStore) RecordResult(_ context.Context, p Post) error {
	s.posts[p.ID] = p
	return nil
}

func (s *sequenceStore) Lookup(_ context.Context, id string) (Post, bool, error) {
	p, ok := s.feeds[id]
	return p, ok, nil
}

// Lookup on the plain fake finds nothing; only the sequence store has feed posts.
func (s *fakePublishStore) Lookup(context.Context, string) (Post, bool, error) {
	return Post{}, false, nil
}

type sequenceRig struct {
	pub      *Publisher
	platform *storyPlatform
	store    *sequenceStore
	reporter *fakeReporter
	now      time.Time
}

// t0 is the moment the story tests run at, unless a test moves the clock.
var storyT0 = time.Date(2026, 10, 20, 22, 0, 0, 0, time.UTC)

func newSequenceRig(posts ...Post) *sequenceRig {
	rig := &sequenceRig{
		platform: &storyPlatform{failIG: map[string]bool{}},
		store:    &sequenceStore{posts: map[string]Post{}, feeds: map[string]Post{}},
		reporter: &fakeReporter{},
		now:      storyT0,
	}
	for _, p := range posts {
		rig.store.posts[p.ID] = p
	}
	rig.pub = NewPublisher(PublisherConfig{
		Enabled:    true,
		Secret:     testSecret,
		Batch:      10,
		Lease:      15 * time.Minute,
		Backoff:    []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour},
		DeferDelay: 5 * time.Minute,
		Now:        func() time.Time { return rig.now },
	}, rig.platform, rig.store, rig.reporter, slog.New(slog.DiscardHandler))
	return rig
}

// storyWith builds a story whose items are spaced one minute apart from at.
func storyWith(id string, at time.Time, networks []Network, names ...string) Post {
	p := Post{
		ID: id, PublishAt: at, Networks: networks, Kind: TypeStory,
		Status: StatusScheduled, NextAttemptAt: &at,
	}
	for i, name := range names {
		p.Media = append(p.Media, Media{Name: name, URL: "https://cdn.test/" + name})
		p.StoryItems = append(p.StoryItems, StoryItem{Position: i, ScheduledAt: at.Add(time.Duration(i) * StoryGap)})
	}
	return p
}

func (r *sequenceRig) run(t *testing.T) {
	t.Helper()
	if _, err := r.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
}

func (r *sequenceRig) post(id string) Post { return r.store.posts[id] }

func TestStorySequencePublishesEveryFileInOrderOnBothNetworks(t *testing.T) {
	t.Parallel()
	names := []string{"1.jpg", "2.mp4", "3.jpg"}
	rig := newSequenceRig(storyWith("historia", storyT0.Add(-time.Minute), []Network{NetworkInstagram, NetworkFacebook}, names...))
	rig.now = storyT0.Add(10 * time.Minute)

	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, names) || !slices.Equal(rig.platform.fbOrder, names) {
		t.Fatalf("instagram = %v, facebook = %v; want every file in order on both", rig.platform.igOrder, rig.platform.fbOrder)
	}
	done := rig.post("historia")
	if done.Status != StatusPublished || len(rig.reporter.reports) != 1 {
		t.Fatalf("after the sequence = %+v, reports = %d; want published and one report", done, len(rig.reporter.reports))
	}
	for i, item := range done.StoryItems {
		if item.IGMediaID == "" || item.FBPostID == "" {
			t.Fatalf("item %d = %+v, want an id on both networks", i, item)
		}
	}
}

func TestStoryItemWaitsForItsScheduledTime(t *testing.T) {
	t.Parallel()
	rig := newSequenceRig(storyWith("historia-2", storyT0, []Network{NetworkInstagram}, "1.jpg", "2.jpg"))
	rig.now = storyT0

	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, []string{"1.jpg"}) {
		t.Fatalf("first run published %v, want only the item that is due", rig.platform.igOrder)
	}
	if next := rig.post("historia-2").NextAttemptAt; next == nil || !next.Equal(storyT0.Add(StoryGap)) {
		t.Fatalf("next attempt = %v, want the second story's time %v", next, storyT0.Add(StoryGap))
	}

	rig.now = storyT0.Add(StoryGap - time.Second)
	rig.run(t)
	if len(rig.platform.igOrder) != 1 {
		t.Fatalf("a run before the second story's time published %v", rig.platform.igOrder)
	}

	rig.now = storyT0.Add(StoryGap)
	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, []string{"1.jpg", "2.jpg"}) {
		t.Fatalf("after the second story's time published %v, want both in order", rig.platform.igOrder)
	}
}

func TestStoryRetryNeverRepostsAnItemThatHasAnId(t *testing.T) {
	t.Parallel()
	names := []string{"1.jpg", "2.jpg", "3.jpg"}
	rig := newSequenceRig(storyWith("historia-3", storyT0.Add(-time.Minute), []Network{NetworkInstagram}, names...))
	rig.now = storyT0.Add(10 * time.Minute)
	rig.platform.failIG["2.jpg"] = true

	rig.run(t)
	if !slices.Equal(rig.platform.igAttempts, []string{"1.jpg", "2.jpg"}) {
		t.Fatalf("attempts = %v; a failed item must stop the sequence before item 3", rig.platform.igAttempts)
	}
	first := rig.post("historia-3")
	if first.Status != StatusPartial || first.Attempts != 1 || first.StoryItems[0].IGMediaID == "" || first.StoryItems[1].IGMediaID != "" {
		t.Fatalf("after the failure = %+v, want partial with item 1 published and item 2 pending", first)
	}

	rig.platform.failIG["2.jpg"] = false
	rig.now = rig.now.Add(5 * time.Minute)
	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, []string{"1.jpg", "2.jpg", "3.jpg"}) {
		t.Fatalf("instagram published %v; want each file exactly once, in order", rig.platform.igOrder)
	}
	if done := rig.post("historia-3"); done.Status != StatusPublished {
		t.Fatalf("after the retry = %+v, want published", done)
	}
}

func TestStoryAfterWaitsForTheFeedPostToBePublished(t *testing.T) {
	t.Parallel()
	feedAt := storyT0.Add(time.Hour)
	rig := newSequenceRig(storyWith("historia-4", storyT0.Add(-time.Minute), []Network{NetworkInstagram}, "1.jpg"))
	rig.store.posts["historia-4"] = withAfter(rig.store.posts["historia-4"], "caja")
	rig.store.feeds["caja"] = Post{ID: "caja", Networks: []Network{NetworkInstagram}, Status: StatusScheduled, NextAttemptAt: &feedAt}
	rig.now = storyT0.Add(10 * time.Minute)

	rig.run(t)
	if len(rig.platform.igAttempts) != 0 {
		t.Fatalf("the story published %v before its feed post", rig.platform.igAttempts)
	}
	waiting := rig.post("historia-4")
	if waiting.Status != StatusScheduled || waiting.NextAttemptAt == nil || waiting.LastError == "" {
		t.Fatalf("while waiting = %+v, want scheduled with a reason and a retry time", waiting)
	}

	feedDone := Post{ID: "caja", Networks: []Network{NetworkInstagram}, Status: StatusPublished, IGMediaID: "ig-feed"}
	rig.store.feeds["caja"] = feedDone
	rig.now = rig.now.Add(5 * time.Minute)
	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, []string{"1.jpg"}) {
		t.Fatalf("after the feed post published, instagram = %v, want the story", rig.platform.igOrder)
	}
}

func TestStoryFailsWhenTheFeedPostFailedForGood(t *testing.T) {
	t.Parallel()
	rig := newSequenceRig(storyWith("historia-5", storyT0.Add(-time.Minute), []Network{NetworkInstagram}, "1.jpg"))
	rig.store.posts["historia-5"] = withAfter(rig.store.posts["historia-5"], "caja")
	rig.store.feeds["caja"] = Post{ID: "caja", Networks: []Network{NetworkInstagram}, Status: StatusFailed}
	rig.now = storyT0.Add(10 * time.Minute)

	rig.run(t)
	if len(rig.platform.igAttempts) != 0 {
		t.Fatalf("the story published %v though its feed post failed", rig.platform.igAttempts)
	}
	failed := rig.post("historia-5")
	if failed.Status != StatusFailed || failed.NextAttemptAt != nil {
		t.Fatalf("after the feed failed = %+v, want a final failed story", failed)
	}
	if !strings.Contains(failed.LastError, "caja") || !strings.Contains(failed.LastError, "not published") {
		t.Fatalf("last error = %q, want the feed post and the reason", failed.LastError)
	}
	if len(rig.reporter.reports) != 1 {
		t.Fatalf("reports = %d, want one terminal report", len(rig.reporter.reports))
	}
}

func TestStoryFailsWhenTheFeedPostDoesNotGoToItsNetwork(t *testing.T) {
	t.Parallel()
	rig := newSequenceRig(storyWith("historia-6", storyT0.Add(-time.Minute), []Network{NetworkInstagram}, "1.jpg"))
	rig.store.posts["historia-6"] = withAfter(rig.store.posts["historia-6"], "caja")
	rig.store.feeds["caja"] = Post{ID: "caja", Networks: []Network{NetworkFacebook}, Status: StatusScheduled}
	rig.now = storyT0.Add(10 * time.Minute)

	rig.run(t)
	if got := rig.post("historia-6"); got.Status != StatusFailed || !strings.Contains(got.LastError, "does not publish to instagram") {
		t.Fatalf("story = %+v, want failed because the feed post never reaches instagram", got)
	}
}

func withAfter(p Post, after string) Post {
	p.After = after
	return p
}
