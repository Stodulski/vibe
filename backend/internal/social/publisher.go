package social

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// Platform is what the publisher needs from Meta.
type Platform interface {
	// InstagramQuota returns how many containers the account has published in
	// the last 24 hours and the most it may publish in that window.
	InstagramQuota(ctx context.Context) (used, total int, err error)
	// PublishInstagram publishes post to Instagram and returns the media id.
	PublishInstagram(ctx context.Context, post Post) (string, error)
	// PublishFacebook publishes post to the Page and returns the post id.
	PublishFacebook(ctx context.Context, post Post) (string, error)
	// InstagramPermalink returns the public URL of a published Instagram media.
	InstagramPermalink(ctx context.Context, mediaID string) (string, error)
	// PublishThreads publishes post to the Threads account and returns the media id.
	PublishThreads(ctx context.Context, post Post) (string, error)
	// ThreadsPermalink returns the public URL of a published Threads post.
	ThreadsPermalink(ctx context.Context, mediaID string) (string, error)
}

// PublishStore is the part of the post store the publisher uses.
type PublishStore interface {
	// ClaimDue locks up to limit live posts that are due, marking them
	// publishing until now+lease so a crashed run is picked up again later.
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Post, error)
	// RecordResult writes a post's publishing state back.
	RecordResult(ctx context.Context, p Post) error
}

// Reporter is told once when a post reaches a final status.
type Reporter interface {
	Report(ctx context.Context, r Report) error
}

// Report describes a post that reached a final status.
type Report struct {
	Post        Post
	IGPermalink string
	ThreadsLink string
}

// quotaDeferDelay is how long a post waits when Instagram's daily limit is
// used up. The window is a rolling 24 hours, so an hour is a sensible check.
const quotaDeferDelay = time.Hour

// PublisherConfig tunes the publisher. Backoff holds the wait before each
// retry, so its length plus one is the attempt budget. Secret is scrubbed from
// every error the publisher stores or logs.
type PublisherConfig struct {
	Enabled    bool
	Secret     string
	Batch      int
	Lease      time.Duration
	Backoff    []time.Duration
	DeferDelay time.Duration
	Now        func() time.Time
}

// Publisher publishes the due posts. A network that has an id is never
// published again, and a post reaches a final status after its last attempt.
type Publisher struct {
	cfg      PublisherConfig
	platform Platform
	store    PublishStore
	reporter Reporter
	logger   *slog.Logger
}

// NewPublisher returns a Publisher. A nil reporter reports nothing.
func NewPublisher(cfg PublisherConfig, platform Platform, store PublishStore, reporter Reporter, logger *slog.Logger) *Publisher {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Publisher{cfg: cfg, platform: platform, store: store, reporter: reporter, logger: logger}
}

// RunDue publishes every due post once and returns how many it claimed. It
// does nothing when the publisher is disabled.
func (p *Publisher) RunDue(ctx context.Context) (int, error) {
	if !p.cfg.Enabled {
		p.logger.Info("social: publishing skipped, no social publishing credential is set")
		return 0, nil
	}
	due, err := p.store.ClaimDue(ctx, p.cfg.Now(), p.cfg.Batch, p.cfg.Lease)
	if err != nil {
		return 0, fmt.Errorf("social: claim due posts: %w", err)
	}
	for _, post := range due {
		p.publish(ctx, post)
	}
	return len(due), nil
}

func (p *Publisher) publish(ctx context.Context, post Post) {
	if !p.instagramAllowed(ctx, &post) {
		return
	}
	errs, permanent := p.publishPending(ctx, &post)
	p.finish(ctx, &post, errs, permanent)
}

// instagramAllowed checks the daily quota before any Instagram call. It settles
// the post itself and returns false when the run must stop here.
func (p *Publisher) instagramAllowed(ctx context.Context, post *Post) bool {
	if !slices.Contains(pendingNetworks(*post), NetworkInstagram) {
		return true
	}
	used, total, err := p.platform.InstagramQuota(ctx)
	switch {
	case errors.Is(err, circuitbreaker.ErrOpen):
		p.deferAttempt(ctx, post, "meta is unavailable, retrying later", p.cfg.DeferDelay)
		return false
	case err != nil:
		p.finish(ctx, post, []error{err}, errors.Is(err, ErrPermanent))
		return false
	case used >= total:
		p.deferAttempt(ctx, post, "instagram daily publishing limit reached", quotaDeferDelay)
		return false
	default:
		return true
	}
}

// publishPending publishes each network that has no id yet, saving each id as
// soon as Meta returns it. It returns the failures and whether any is permanent.
func (p *Publisher) publishPending(ctx context.Context, post *Post) ([]error, bool) {
	var errs []error
	permanent := false
	for _, network := range pendingNetworks(*post) {
		id, err := p.publishNetwork(ctx, *post, network)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", network, err))
			permanent = permanent || errors.Is(err, ErrPermanent)
			continue
		}
		setNetworkID(post, network, id)
		p.save(ctx, *post)
	}
	return errs, permanent
}

func (p *Publisher) publishNetwork(ctx context.Context, post Post, network Network) (string, error) {
	var id string
	var err error
	switch network {
	case NetworkInstagram:
		id, err = p.platform.PublishInstagram(ctx, post)
	case NetworkFacebook:
		id, err = p.platform.PublishFacebook(ctx, post)
	case NetworkThreads:
		id, err = p.platform.PublishThreads(ctx, post)
	default:
		return "", fmt.Errorf("%w: unknown network %q", ErrPermanent, network)
	}
	if err != nil {
		return "", fmt.Errorf("publish %s: %w", network, err)
	}
	return id, nil
}

func setNetworkID(post *Post, network Network, id string) {
	switch network {
	case NetworkInstagram:
		post.IGMediaID = id
	case NetworkFacebook:
		post.FBPostID = id
	case NetworkThreads:
		post.ThreadsMediaID = id
	}
}

// finish settles one run. Every network published makes the post published.
// A breaker that is open defers without an attempt. A permanent refusal or the
// last attempt makes the post final. Anything else schedules the next retry.
func (p *Publisher) finish(ctx context.Context, post *Post, errs []error, permanent bool) {
	now := p.cfg.Now()
	if len(pendingNetworks(*post)) == 0 {
		post.Status = StatusPublished
		post.PublishedAt = &now
		post.NextAttemptAt = nil
		post.LastError = ""
		p.save(ctx, *post)
		p.report(ctx, *post)
		return
	}
	if len(errs) == 0 {
		errs = []error{errors.New("a network was left unpublished without an error")}
	}
	if allBreakerOpen(errs) {
		p.deferAttempt(ctx, post, "meta is unavailable, retrying later", p.cfg.DeferDelay)
		return
	}

	post.Attempts++
	post.LastError = p.scrub(errors.Join(errs...).Error())
	if permanent || post.Attempts >= p.maxAttempts() {
		post.Status = terminalStatus(*post)
		post.NextAttemptAt = nil
		p.save(ctx, *post)
		p.report(ctx, *post)
		return
	}
	post.Status = liveStatus(*post)
	next := now.Add(p.cfg.Backoff[post.Attempts-1])
	post.NextAttemptAt = &next
	p.save(ctx, *post)
}

// deferAttempt reschedules a post without spending an attempt.
func (p *Publisher) deferAttempt(ctx context.Context, post *Post, reason string, delay time.Duration) {
	next := p.cfg.Now().Add(delay)
	post.Status = liveStatus(*post)
	post.LastError = reason
	post.NextAttemptAt = &next
	p.save(ctx, *post)
}

func (p *Publisher) maxAttempts() int {
	return len(p.cfg.Backoff) + 1
}

// save writes a post's state back. A failure is logged, not returned: the
// claim lease expires and the post is picked up again.
func (p *Publisher) save(ctx context.Context, post Post) {
	if err := p.store.RecordResult(ctx, post); err != nil {
		p.logger.Error("social: could not record the publish result", "post", post.ID, "error", p.scrub(err.Error()))
	}
}

func (p *Publisher) report(ctx context.Context, post Post) {
	if p.reporter == nil {
		return
	}
	rep := Report{Post: post}
	if post.IGMediaID != "" {
		link, err := p.platform.InstagramPermalink(ctx, post.IGMediaID)
		if err != nil {
			p.logger.Warn("social: no instagram permalink for the report", "post", post.ID, "error", p.scrub(err.Error()))
		}
		rep.IGPermalink = link
	}
	if post.ThreadsMediaID != "" {
		link, err := p.platform.ThreadsPermalink(ctx, post.ThreadsMediaID)
		if err != nil {
			p.logger.Warn("social: no threads permalink for the report", "post", post.ID, "error", p.scrub(err.Error()))
		}
		rep.ThreadsLink = link
	}
	if err := p.reporter.Report(ctx, rep); err != nil {
		p.logger.Error("social: could not queue the report", "post", post.ID, "error", p.scrub(err.Error()))
	}
}

// scrub removes the access token from text before it is stored or logged.
func (p *Publisher) scrub(text string) string {
	if p.cfg.Secret == "" {
		return text
	}
	return strings.ReplaceAll(text, p.cfg.Secret, "[redacted]")
}

func pendingNetworks(post Post) []Network {
	var out []Network
	for _, n := range post.Networks {
		switch {
		case n == NetworkInstagram && post.IGMediaID == "":
			out = append(out, n)
		case n == NetworkFacebook && post.FBPostID == "":
			out = append(out, n)
		case n == NetworkThreads && post.ThreadsMediaID == "":
			out = append(out, n)
		}
	}
	return out
}

func hasNetworkID(post Post) bool {
	return post.IGMediaID != "" || post.FBPostID != "" || post.ThreadsMediaID != ""
}

// liveStatus is the status of a post that will be tried again.
func liveStatus(post Post) Status {
	if hasNetworkID(post) {
		return StatusPartial
	}
	return StatusScheduled
}

// terminalStatus is the status of a post that will not be tried again.
func terminalStatus(post Post) Status {
	if hasNetworkID(post) {
		return StatusPartial
	}
	return StatusFailed
}

func allBreakerOpen(errs []error) bool {
	if len(errs) == 0 {
		return false
	}
	for _, err := range errs {
		if !errors.Is(err, circuitbreaker.ErrOpen) {
			return false
		}
	}
	return true
}
