package social

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

var t0 = time.Date(2026, 10, 20, 22, 0, 0, 0, time.UTC)

type fakePlatform struct {
	igErr, fbErr   error
	quotaErr       error
	used, total    int
	igCalls        int
	fbCalls        int
	lastKind       Kind
	permalink      string
	permalinkCalls int
	thErr          error
	thCalls        int
	thKind         Kind
	thLink         string
}

func (f *fakePlatform) PublishThreads(_ context.Context, p Post) (string, error) {
	f.thCalls++
	f.thKind = p.Kind
	if f.thErr != nil {
		return "", f.thErr
	}
	return "th-1", nil
}

func (f *fakePlatform) ThreadsPermalink(context.Context, string) (string, error) {
	return f.thLink, nil
}

func (f *fakePlatform) InstagramQuota(context.Context) (int, int, error) {
	return f.used, f.total, f.quotaErr
}

func (f *fakePlatform) PublishInstagram(_ context.Context, p Post) (string, error) {
	f.igCalls++
	f.lastKind = p.Kind
	if f.igErr != nil {
		return "", f.igErr
	}
	return "ig-1", nil
}

func (f *fakePlatform) PublishFacebook(_ context.Context, p Post) (string, error) {
	f.fbCalls++
	f.lastKind = p.Kind
	if f.fbErr != nil {
		return "", f.fbErr
	}
	return "fb-1", nil
}

func (f *fakePlatform) InstagramPermalink(context.Context, string) (string, error) {
	f.permalinkCalls++
	return f.permalink, nil
}

type fakePublishStore struct {
	posts      map[string]Post
	claimCalls int
	saves      int
}

func (s *fakePublishStore) ClaimDue(_ context.Context, now time.Time, limit int, lease time.Duration) ([]Post, error) {
	s.claimCalls++
	var out []Post
	for id, p := range s.posts {
		live := p.Status == StatusScheduled || p.Status == StatusPartial || p.Status == StatusPublishing
		if !live || p.Draft || p.PublishAt.After(now) || p.NextAttemptAt == nil || p.NextAttemptAt.After(now) {
			continue
		}
		if len(out) == limit {
			break
		}
		claimed := now.Add(lease)
		p.Status = StatusPublishing
		p.NextAttemptAt = &claimed
		s.posts[id] = p
		out = append(out, p)
	}
	return out, nil
}

func (s *fakePublishStore) RecordResult(_ context.Context, p Post) error {
	s.saves++
	s.posts[p.ID] = p
	return nil
}

type fakeReporter struct{ reports []Report }

func (r *fakeReporter) Report(_ context.Context, rep Report) error {
	r.reports = append(r.reports, rep)
	return nil
}

const testSecret = "tok-secret-value"

type harness struct {
	pub      *Publisher
	platform *fakePlatform
	store    *fakePublishStore
	reporter *fakeReporter
	now      time.Time
}

func newHarness(t *testing.T, enabled bool, posts ...Post) *harness {
	t.Helper()
	h := &harness{
		platform: &fakePlatform{used: 0, total: 50, permalink: "https://www.instagram.com/p/x/"},
		store:    &fakePublishStore{posts: map[string]Post{}},
		reporter: &fakeReporter{},
		now:      t0,
	}
	for _, p := range posts {
		h.store.posts[p.ID] = p
	}
	h.pub = NewPublisher(PublisherConfig{
		Enabled:    enabled,
		Secret:     testSecret,
		Batch:      10,
		Lease:      15 * time.Minute,
		Backoff:    []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour},
		DeferDelay: 5 * time.Minute,
		Now:        func() time.Time { return h.now },
	}, h.platform, h.store, h.reporter, slog.New(slog.DiscardHandler))
	return h
}

func duePost(id string, networks ...Network) Post {
	at := t0.Add(-time.Minute)
	return Post{
		ID: id, PublishAt: at, Networks: networks, Kind: TypeImage, Caption: "Caja",
		Media: []Media{{Name: "1.jpg", URL: "https://cdn.test/1.jpg"}}, Status: StatusScheduled, NextAttemptAt: &at,
	}
}

func (h *harness) post(id string) Post { return h.store.posts[id] }

func TestPublishesBothNetworksAndReportsOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, duePost("p1", NetworkInstagram, NetworkFacebook))

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	got := h.post("p1")
	if got.Status != StatusPublished || got.IGMediaID != "ig-1" || got.FBPostID != "fb-1" || got.PublishedAt == nil {
		t.Fatalf("post after publish = %+v, want published with both ids", got)
	}
	if len(h.reporter.reports) != 1 || h.reporter.reports[0].Post.Status != StatusPublished {
		t.Fatalf("reports = %+v, want exactly one published report", h.reporter.reports)
	}
	if h.reporter.reports[0].IGPermalink != "https://www.instagram.com/p/x/" {
		t.Fatalf("report permalink = %q, want the Instagram permalink", h.reporter.reports[0].IGPermalink)
	}
}

func TestCarouselAndReelReachThePlatformWithTheirKind(t *testing.T) {
	t.Parallel()
	carousel := duePost("car", NetworkFacebook)
	carousel.Kind = TypeCarousel
	h := newHarness(t, true, carousel)

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	if h.platform.fbCalls != 1 || h.platform.lastKind != TypeCarousel {
		t.Fatalf("facebook calls = %d kind = %q, want one carousel publish", h.platform.fbCalls, h.platform.lastKind)
	}
}

func TestAPartialFailureRetriesOnlyTheMissingNetwork(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, duePost("p1", NetworkInstagram, NetworkFacebook))
	h.platform.fbErr = errors.New("meta 500: try again")

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	first := h.post("p1")
	if first.Status != StatusPartial || first.IGMediaID != "ig-1" || first.Attempts != 1 {
		t.Fatalf("after first failure = %+v, want partial with the Instagram id and one attempt", first)
	}
	if first.NextAttemptAt == nil || !first.NextAttemptAt.Equal(t0.Add(5*time.Minute)) {
		t.Fatalf("next attempt = %v, want +5m", first.NextAttemptAt)
	}
	if len(h.reporter.reports) != 0 {
		t.Fatal("a retryable failure must not report yet")
	}

	h.platform.fbErr = nil
	h.now = t0.Add(5 * time.Minute)
	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() on retry = %v", err)
	}
	done := h.post("p1")
	if done.Status != StatusPublished || done.FBPostID != "fb-1" {
		t.Fatalf("after retry = %+v, want published", done)
	}
	if h.platform.igCalls != 1 {
		t.Fatalf("instagram publish calls = %d, want 1: a network that has an id is never re-posted", h.platform.igCalls)
	}
	if len(h.reporter.reports) != 1 {
		t.Fatalf("reports = %d, want one terminal report", len(h.reporter.reports))
	}
}

func TestGivesUpAfterFourAttemptsWithBackoff(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, duePost("p1", NetworkInstagram, NetworkFacebook))
	h.platform.fbErr = errors.New("meta 500")

	steps := []time.Duration{0, 5 * time.Minute, 20 * time.Minute, 80 * time.Minute}
	for _, at := range steps {
		h.now = t0.Add(at)
		if _, err := h.pub.RunDue(context.Background()); err != nil {
			t.Fatalf("RunDue() at %v = %v", at, err)
		}
	}
	got := h.post("p1")
	if got.Attempts != 4 || got.Status != StatusPartial || got.NextAttemptAt != nil {
		t.Fatalf("after four attempts = %+v, want terminal partial with four attempts", got)
	}
	if len(h.reporter.reports) != 1 || h.reporter.reports[0].Post.Status != StatusPartial {
		t.Fatalf("reports = %+v, want one terminal partial report", h.reporter.reports)
	}
}

func TestAPermanentRefusalIsTerminalAtOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, duePost("p1", NetworkFacebook))
	h.platform.fbErr = fmt.Errorf("%w: meta 400: invalid image", ErrPermanent)

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	got := h.post("p1")
	if got.Status != StatusFailed || got.Attempts != 1 {
		t.Fatalf("after permanent refusal = %+v, want failed after one attempt", got)
	}
	if len(h.reporter.reports) != 1 {
		t.Fatalf("reports = %d, want one failed report", len(h.reporter.reports))
	}
}

func TestQuotaReachedDefersWithoutAnAttempt(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, duePost("p1", NetworkInstagram))
	h.platform.used, h.platform.total = 50, 50

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	got := h.post("p1")
	if h.platform.igCalls != 0 || got.Attempts != 0 || got.Status != StatusScheduled {
		t.Fatalf("igCalls=%d attempts=%d status=%q, want no publish, no attempt, still scheduled",
			h.platform.igCalls, got.Attempts, got.Status)
	}
	if got.NextAttemptAt == nil || !got.NextAttemptAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("next attempt = %v, want one hour later", got.NextAttemptAt)
	}
	if len(h.reporter.reports) != 0 {
		t.Fatal("a deferred post is not finished and must not report")
	}
}

func TestAnOpenBreakerDefersWithoutAnAttempt(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, duePost("p1", NetworkInstagram, NetworkFacebook))
	h.platform.igErr = fmt.Errorf("meta: %w", circuitbreaker.ErrOpen)

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	got := h.post("p1")
	if got.Attempts != 0 || got.FBPostID != "fb-1" || got.Status != StatusPartial {
		t.Fatalf("after open breaker = %+v, want no attempt, facebook done, partial", got)
	}
	if got.NextAttemptAt == nil || !got.NextAttemptAt.Equal(t0.Add(5*time.Minute)) {
		t.Fatalf("next attempt = %v, want five minutes later", got.NextAttemptAt)
	}
}

func TestPublishedNetworksAreNeverRepeated(t *testing.T) {
	t.Parallel()
	p := duePost("p1", NetworkInstagram, NetworkFacebook)
	p.Status = StatusPartial
	p.IGMediaID = "ig-old"
	h := newHarness(t, true, p)

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	if h.platform.igCalls != 0 {
		t.Fatalf("instagram publish calls = %d, want 0 for a network that already has an id", h.platform.igCalls)
	}
	if got := h.post("p1"); got.IGMediaID != "ig-old" || got.Status != StatusPublished {
		t.Fatalf("post = %+v, want the old id kept and published", got)
	}
}

func TestDisabledPublisherClaimsNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, duePost("p1", NetworkFacebook))

	n, err := h.pub.RunDue(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("RunDue() = %d, %v; want 0, nil", n, err)
	}
	if h.store.claimCalls != 0 || h.platform.fbCalls != 0 {
		t.Fatal("a publisher without a token must not claim or publish")
	}
}

func TestErrorTextNeverCarriesTheToken(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, duePost("p1", NetworkFacebook))
	h.platform.fbErr = fmt.Errorf("request with %s failed", testSecret)

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("RunDue() = %v", err)
	}
	last := h.post("p1").LastError
	if last == "" || strings.Contains(last, testSecret) || !strings.Contains(last, "[redacted]") {
		t.Fatalf("last error = %q, must not carry the token", last)
	}
}
