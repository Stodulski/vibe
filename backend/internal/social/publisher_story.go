package social

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// waitUntil is a reason to stop now and try again at a later time, which
// spends no attempt.
type waitUntil struct {
	at     time.Time
	reason string
}

// earlierWait keeps whichever of the current wait and a new one comes first.
func earlierWait(current *waitUntil, at time.Time, reason string) *waitUntil {
	if current == nil || at.Before(current.at) {
		return &waitUntil{at: at, reason: reason}
	}
	return current
}

// publishStory publishes each network's sequence of stories in order. Each
// network is independent of the others. On one network, a story goes out only
// after the stories before it have an id there, and a story that already has
// an id is never sent again.
func (p *Publisher) publishStory(ctx context.Context, post *Post) {
	if len(post.Media) != len(post.StoryItems) {
		p.finish(ctx, post, []error{fmt.Errorf("%w: the story's files and its schedule do not match", ErrPermanent)}, true)
		return
	}
	var (
		errs      []error
		permanent bool
		wait      *waitUntil
	)
	for _, network := range pendingNetworks(*post) {
		gate, err := p.storyGate(ctx, *post, network)
		if err != nil {
			errs = append(errs, err)
			permanent = permanent || errors.Is(err, ErrPermanent)
			continue
		}
		if gate != nil {
			wait = earlierWait(wait, gate.at, gate.reason)
			continue
		}
		seqErrs, seqPermanent, seqWait := p.publishSequence(ctx, post, network)
		errs = append(errs, seqErrs...)
		permanent = permanent || seqPermanent
		if seqWait != nil {
			wait = earlierWait(wait, seqWait.at, seqWait.reason)
		}
	}
	if len(errs) == 0 && wait != nil {
		p.deferUntil(ctx, post, wait.reason, wait.at)
		return
	}
	p.finish(ctx, post, errs, permanent)
}

// storyGate reports whether a story sequence may start on network yet. It
// returns nil when it may. A feed post that is still on its way is a wait. A
// feed post that can no longer reach the network is a permanent failure, with
// the reason in its error.
func (p *Publisher) storyGate(ctx context.Context, post Post, network Network) (*waitUntil, error) {
	if post.After == "" {
		return nil, nil
	}
	feed, found, err := p.store.Lookup(ctx, post.After)
	if err != nil {
		return nil, fmt.Errorf("%s: look up feed post %s: %w", network, post.After, err)
	}
	wait := &waitUntil{
		at:     p.cfg.Now().Add(p.cfg.DeferDelay),
		reason: fmt.Sprintf("waiting for feed post %s on %s", post.After, network),
	}
	switch {
	case !found:
		return wait, nil
	case !slices.Contains(feed.Networks, network):
		return nil, fmt.Errorf("%w: feed post %s does not publish to %s, so the story cannot follow it", ErrPermanent, post.After, network)
	case networkDone(feed, network):
		return nil, nil
	case feed.NextAttemptAt == nil:
		return nil, fmt.Errorf("%w: feed post %s was not published on %s (status %s), so the story cannot follow it", ErrPermanent, post.After, network, feed.Status)
	default:
		return wait, nil
	}
}

// publishSequence sends the items of one network in order. It stops at the
// first item that is not due, that cannot be sent now, or that fails. It returns
// the failures, whether one is permanent, and any wait for a later time.
func (p *Publisher) publishSequence(ctx context.Context, post *Post, network Network) ([]error, bool, *waitUntil) {
	now := p.cfg.Now()
	for i := range post.StoryItems {
		item := &post.StoryItems[i]
		if itemNetworkID(*item, network) != "" {
			continue
		}
		if now.Before(item.ScheduledAt) {
			return nil, false, &waitUntil{at: item.ScheduledAt, reason: "the next story is not due yet"}
		}
		if nearDeadline(ctx) {
			return nil, false, &waitUntil{at: p.cfg.Now().Add(deadlineDeferDelay), reason: deadlineReason}
		}
		if network == NetworkInstagram {
			wait, err := p.storyQuota(ctx)
			switch {
			case err != nil:
				return []error{fmt.Errorf("%s story %d: %w", network, i+1, err)}, errors.Is(err, ErrPermanent), nil
			case wait != nil:
				return nil, false, wait
			}
		}
		view := *post
		view.Kind = TypeStory
		view.Media = []Media{post.Media[i]}
		id, err := p.publishNetwork(ctx, view, network)
		if err != nil {
			return []error{fmt.Errorf("%s story %d: %w", network, i+1, err)}, errors.Is(err, ErrPermanent), nil
		}
		setItemNetworkID(item, network, id)
		p.save(ctx, *post)
	}
	return nil, false, nil
}

// storyQuota checks Instagram's daily limit before each story, because stories
// count toward it. A used-up limit or an open breaker is a wait, not a failure.
func (p *Publisher) storyQuota(ctx context.Context) (*waitUntil, error) {
	used, total, err := p.platform.InstagramQuota(ctx)
	now := p.cfg.Now()
	switch {
	case errors.Is(err, circuitbreaker.ErrOpen):
		return &waitUntil{at: now.Add(p.cfg.DeferDelay), reason: "meta is unavailable, retrying later"}, nil
	case err != nil:
		return nil, err
	case used >= total:
		return &waitUntil{at: now.Add(quotaDeferDelay), reason: "instagram daily publishing limit reached"}, nil
	default:
		return nil, nil
	}
}

// itemNetworkID is the id a story item has on network, empty when none.
func itemNetworkID(item StoryItem, network Network) string {
	switch network {
	case NetworkInstagram:
		return item.IGMediaID
	case NetworkFacebook:
		return item.FBPostID
	case NetworkThreads:
		return ""
	default:
		return ""
	}
}

// setItemNetworkID records the id a network gave a story item.
func setItemNetworkID(item *StoryItem, network Network, id string) {
	switch network {
	case NetworkInstagram:
		item.IGMediaID = id
	case NetworkFacebook:
		item.FBPostID = id
	case NetworkThreads:
		// A story never goes to Threads, so a Threads id is never recorded on one.
	}
}
