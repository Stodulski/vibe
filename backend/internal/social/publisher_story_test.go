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
// fails the names it is told to fail, as a transient Meta error. refuseFB names
// the files Facebook refuses for good.
type storyPlatform struct {
	igAttempts, fbAttempts []string
	igOrder, fbOrder       []string
	failIG                 map[string]bool
	refuseFB               map[string]bool
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
	if s.refuseFB[name] {
		return "", fmt.Errorf("%w: facebook refused %s", ErrPermanent, name)
	}
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
		claimed := p
		claimed.StoryItems = slices.Clone(p.StoryItems)
		out = append(out, claimed)
	}
	return out, nil
}

// The store keeps its own copy of the story items, as a database would: a
// publisher that edits its in-memory post must not change the stored row.
func (s *sequenceStore) RecordResult(ctx context.Context, p Post) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.StoryItems = slices.Clone(p.StoryItems)
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

func TestStorySavesEachIDEvenWhenTheRunContextIsCancelled(t *testing.T) {
	t.Parallel()
	rig := newSequenceRig(storyWith("historia-7", storyT0.Add(-time.Minute), []Network{NetworkInstagram}, "1.jpg", "2.jpg"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rig.pub.platform = cancelAfterPublish{Platform: rig.platform, cancel: cancel}

	if _, err := rig.pub.RunDue(ctx); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	got := rig.post("historia-7")
	if got.StoryItems[0].IGMediaID != "ig-1" || got.StoryItems[1].IGMediaID != "ig-2" {
		t.Fatalf("stored items = %+v, want both Instagram ids kept", got.StoryItems)
	}
}

func TestStoryWithLittleTimeLeftPublishesNothingAndSpendsNoAttempt(t *testing.T) {
	t.Parallel()
	names := []string{"1.jpg", "2.jpg", "3.jpg"}
	rig := newSequenceRig(storyWith("historia-8", storyT0.Add(-time.Minute), []Network{NetworkInstagram, NetworkFacebook}, names...))
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(20*time.Second))
	defer cancel()

	if _, err := rig.pub.RunDue(ctx); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	if len(rig.platform.igAttempts) != 0 || len(rig.platform.fbAttempts) != 0 {
		t.Fatalf("publishes instagram=%v facebook=%v, want none with 20s of the run left", rig.platform.igAttempts, rig.platform.fbAttempts)
	}
	if got := rig.post("historia-8"); got.Attempts != 0 || got.Status != StatusScheduled || got.LastError == "" {
		t.Fatalf("story = %+v, want deferred with no attempt spent", got)
	}
}

func TestStoryItemThatAlreadyHasAnIDIsNotSentAgainAfterTheLeaseExpires(t *testing.T) {
	t.Parallel()
	names := []string{"1.jpg", "2.jpg"}
	post := storyWith("historia-9", storyT0.Add(-2*time.Hour), []Network{NetworkInstagram}, names...)
	post.Status = StatusPublishing
	post.StoryItems[0].IGMediaID = "ig-saved"
	lapsed := storyT0.Add(-time.Hour)
	post.NextAttemptAt = &lapsed
	rig := newSequenceRig(post)

	rig.run(t)
	if !slices.Equal(rig.platform.igAttempts, []string{"2.jpg"}) {
		t.Fatalf("instagram attempts = %v, want only 2.jpg; 1.jpg already has an id", rig.platform.igAttempts)
	}
	if got := rig.post("historia-9"); got.StoryItems[0].IGMediaID != "ig-saved" {
		t.Fatalf("item 1 id = %q, want the saved one kept", got.StoryItems[0].IGMediaID)
	}
}

func TestStoryKeepsPublishingOnInstagramWhenTheFeedPostFailedOnFacebook(t *testing.T) {
	t.Parallel()
	rig := newSequenceRig(storyWith("historia-10", storyT0, []Network{NetworkInstagram, NetworkFacebook}, "1.jpg", "2.jpg"))
	rig.store.posts["historia-10"] = withAfter(rig.store.posts["historia-10"], "reel")
	rig.store.feeds["reel"] = Post{ID: "reel", Networks: []Network{NetworkInstagram, NetworkFacebook}, Status: StatusPartial, IGMediaID: "ig-reel"}
	rig.now = storyT0

	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, []string{"1.jpg"}) || len(rig.platform.fbAttempts) != 0 {
		t.Fatalf("first run: instagram = %v, facebook attempts = %v; want only 1.jpg on instagram and no facebook call", rig.platform.igOrder, rig.platform.fbAttempts)
	}
	waiting := rig.post("historia-10")
	frame2 := waiting.StoryItems[1].ScheduledAt
	if waiting.Status != StatusPartial || waiting.NextAttemptAt == nil || !waiting.NextAttemptAt.Equal(frame2) || waiting.Attempts != 0 {
		t.Fatalf("while instagram waits = %+v, want partial, no attempt spent, and the next attempt at %v", waiting, frame2)
	}
	if !strings.Contains(waiting.LastError, "facebook") {
		t.Fatalf("last error = %q, want the facebook reason kept", waiting.LastError)
	}

	rig.now = frame2
	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, []string{"1.jpg", "2.jpg"}) || len(rig.platform.fbAttempts) != 0 {
		t.Fatalf("after frame 2 is due: instagram = %v, facebook attempts = %v; want both frames on instagram and still no facebook call", rig.platform.igOrder, rig.platform.fbAttempts)
	}
	done := rig.post("historia-10")
	if done.Status != StatusPartial || done.NextAttemptAt != nil || done.StoryItems[1].IGMediaID == "" {
		t.Fatalf("after frame 2 = %+v, want partial and final, since facebook can never follow", done)
	}
}

func TestStoryKeepsPublishingOnOneNetworkWhenAFrameIsRefusedOnTheOther(t *testing.T) {
	t.Parallel()
	rig := newSequenceRig(storyWith("historia-11", storyT0, []Network{NetworkInstagram, NetworkFacebook}, "1.jpg", "2.jpg"))
	rig.platform.refuseFB = map[string]bool{"1.jpg": true}
	rig.now = storyT0

	rig.run(t)
	if !slices.Equal(rig.platform.igOrder, []string{"1.jpg"}) {
		t.Fatalf("instagram = %v, want 1.jpg", rig.platform.igOrder)
	}
	got := rig.post("historia-11")
	frame2 := got.StoryItems[1].ScheduledAt
	if got.Status != StatusPartial || got.NextAttemptAt == nil || !got.NextAttemptAt.Equal(frame2) || got.Attempts != 0 {
		t.Fatalf("after the refusal = %+v, want partial, deferred to %v, and no attempt spent", got, frame2)
	}
	if !slices.Equal(rig.platform.fbAttempts, []string{"1.jpg"}) || !strings.Contains(got.LastError, "facebook") {
		t.Fatalf("facebook attempts = %v, last error = %q; want one facebook call for 1.jpg and the refusal kept", rig.platform.fbAttempts, got.LastError)
	}
}

func TestStoryFailsWhenTheFeedPostIsADraft(t *testing.T) {
	t.Parallel()
	rig := newSequenceRig(storyWith("historia-12", storyT0, []Network{NetworkInstagram, NetworkFacebook}, "1.jpg", "2.jpg"))
	rig.store.posts["historia-12"] = withAfter(rig.store.posts["historia-12"], "draft-reel")
	feedAt := storyT0
	rig.store.feeds["draft-reel"] = Post{ID: "draft-reel", Networks: []Network{NetworkInstagram}, Draft: true, Status: StatusScheduled, NextAttemptAt: &feedAt}
	rig.now = storyT0

	rig.run(t)
	if len(rig.platform.igAttempts) != 0 || len(rig.platform.fbAttempts) != 0 {
		t.Fatalf("publishes instagram=%v facebook=%v; want none for a story that follows a draft", rig.platform.igAttempts, rig.platform.fbAttempts)
	}
	failed := rig.post("historia-12")
	if failed.Status != StatusFailed || failed.NextAttemptAt != nil {
		t.Fatalf("after the draft check = %+v, want a final failed story", failed)
	}
	if !strings.Contains(failed.LastError, "draft") {
		t.Fatalf("last error = %q, want the draft reason", failed.LastError)
	}
}

func withAfter(p Post, after string) Post {
	p.After = after
	return p
}
