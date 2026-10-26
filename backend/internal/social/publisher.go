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
	// Lookup returns the post with id, and whether it exists.
	Lookup(ctx context.Context, id string) (Post, bool, error)
}

// Reporter is told once when a post reaches a final status.
type Reporter interface {
	Report(ctx context.Context, r Report) error
}

// Report describes a post that reached a final status.
type Report struct {
	Post        Post
	IGPermalink string
	// IGStoryLinks are the permalinks of the Instagram stories that have one,
	// in publish order. Stories expire, so a missing link is normal.
	IGStoryLinks []string
	ThreadsLink  string
}

// quotaDeferDelay is how long a post waits when Instagram's daily limit is
// used up. The window is a rolling 24 hours, so an hour is a sensible check.
const quotaDeferDelay = time.Hour

// runDeadlineMargin is how much of a run must be left before it starts a Meta
// call. The cron job runs under a two-minute deadline (scheduler jobTimeout), and
// one publish (container create, poll until ready, publish) can take tens of
// seconds. A publish started with less than this left would be cut mid-flight,
// so the work waits for the next run instead, without spending an attempt.
const runDeadlineMargin = 45 * time.Second

// deadlineDeferDelay is how long work held back by runDeadlineMargin waits. It
// is short, so the next five-minute cron tick claims it.
const deadlineDeferDelay = time.Minute

// deadlineReason is the post's last error while it waits for a run with time left.
const deadlineReason = "the run is close to its deadline, retrying on a later run"

// saveTimeout bounds one write of a publish result. The write is detached from
// the run's context: a result Meta has already returned must be recorded even
// when the run's deadline has passed, or the post is republished after its lease.
const saveTimeout = 10 * time.Second

// nearDeadline reports whether ctx has a deadline with less than runDeadlineMargin left.
func nearDeadline(ctx context.Context) bool {
	deadline, ok := ctx.Deadline()
	return ok && time.Until(deadline) < runDeadlineMargin
}

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
	if post.Kind == TypeStory {
		p.publishStory(ctx, &post)
		return
	}
	if nearDeadline(ctx) {
		p.deferUntil(ctx, &post, deadlineReason, p.cfg.Now().Add(deadlineDeferDelay))
		return
	}
	if !p.instagramAllowed(ctx, &post) {
		return
	}
	errs, permanent, wait := p.publishPending(ctx, &post)
	if wait != nil && len(errs) == 0 {
		p.deferUntil(ctx, &post, wait.reason, wait.at)
		return
	}
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
// soon as Meta returns it. It returns the failures, whether any is permanent, and
// a wait when it stopped short because the run is nearly out of time.
func (p *Publisher) publishPending(ctx context.Context, post *Post) ([]error, bool, *waitUntil) {
	var errs []error
	permanent := false
	for _, network := range pendingNetworks(*post) {
		if nearDeadline(ctx) {
			return errs, permanent, &waitUntil{at: p.cfg.Now().Add(deadlineDeferDelay), reason: deadlineReason}
		}
		id, err := p.publishNetwork(ctx, *post, network)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", network, err))
			permanent = permanent || errors.Is(err, ErrPermanent)
			continue
		}
		setNetworkID(post, network, id)
		p.save(ctx, *post)
	}
	return errs, permanent, nil
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

// FailureLogMessage is the message of the error record written when a post ends
// partial or failed. cmd/api forwards this record, and only this one, to Sentry,
// so the text is the contract between the two packages.
const FailureLogMessage = "social: post did not fully publish"

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
		p.logFailure(*post)
		p.report(ctx, *post)
		return
	}
	post.Status = liveStatus(*post)
	next := now.Add(p.cfg.Backoff[post.Attempts-1])
	post.NextAttemptAt = &next
	p.save(ctx, *post)
}

// logFailure writes the one error record for a post that ended partial or
// failed. It is called only from the terminal branch of finish, so a post yields
// one record. The error is post.LastError, which scrub has already applied.
func (p *Publisher) logFailure(post Post) {
	p.logger.Error(FailureLogMessage, "post", post.ID, "status", string(post.Status), "type", string(post.Kind), "error", post.LastError)
}

// deferAttempt reschedules a post without spending an attempt.
func (p *Publisher) deferAttempt(ctx context.Context, post *Post, reason string, delay time.Duration) {
	p.deferUntil(ctx, post, reason, p.cfg.Now().Add(delay))
}

// deferUntil reschedules a post for at, without spending an attempt.
func (p *Publisher) deferUntil(ctx context.Context, post *Post, reason string, at time.Time) {
	post.Status = liveStatus(*post)
	post.LastError = reason
	post.NextAttemptAt = &at
	p.save(ctx, *post)
}

func (p *Publisher) maxAttempts() int {
	return len(p.cfg.Backoff) + 1
}

// save writes a post's state back. A failure is logged, not returned: the
// claim lease expires and the post is picked up again. The write is detached
// from the run's cancellation and deadline, so a result that arrives after the
// deadline is still recorded.
func (p *Publisher) save(ctx context.Context, post Post) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	if err := p.store.RecordResult(ctx, post); err != nil {
		p.logger.Error("social: could not record the publish result", "post", post.ID, "error", p.scrub(err.Error()))
	}
}

func (p *Publisher) report(ctx context.Context, post Post) {
	if p.reporter == nil {
		return
	}
	rep := Report{Post: post}
	if post.Kind == TypeStory {
		rep.IGStoryLinks = p.storyPermalinks(ctx, post)
	} else if post.IGMediaID != "" {
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

// storyPermalinks reads the Instagram permalink of each published story. A
// story whose link cannot be read is left out of the report, not retried.
func (p *Publisher) storyPermalinks(ctx context.Context, post Post) []string {
	var links []string
	for _, item := range post.StoryItems {
		if item.IGMediaID == "" {
			continue
		}
		link, err := p.platform.InstagramPermalink(ctx, item.IGMediaID)
		if err != nil {
			p.logger.Warn("social: no instagram story permalink for the report", "post", post.ID, "error", p.scrub(err.Error()))
			continue
		}
		if link != "" {
			links = append(links, link)
		}
	}
	return links
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
		if !networkDone(post, n) {
			out = append(out, n)
		}
	}
	return out
}

// networkDone reports whether every part of post is published on network. A
// story is done on a network when each of its items has an id there.
func networkDone(post Post, network Network) bool {
	if post.Kind == TypeStory {
		if len(post.StoryItems) == 0 {
			return false
		}
		for _, item := range post.StoryItems {
			if itemNetworkID(item, network) == "" {
				return false
			}
		}
		return true
	}
	switch network {
	case NetworkInstagram:
		return post.IGMediaID != ""
	case NetworkFacebook:
		return post.FBPostID != ""
	case NetworkThreads:
		return post.ThreadsMediaID != ""
	default:
		return false
	}
}

// hasNetworkID reports whether any network, or any story item, already has an id.
func hasNetworkID(post Post) bool {
	if post.IGMediaID != "" || post.FBPostID != "" || post.ThreadsMediaID != "" {
		return true
	}
	for _, item := range post.StoryItems {
		if item.IGMediaID != "" || item.FBPostID != "" {
			return true
		}
	}
	return false
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
