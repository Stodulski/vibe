package main

import (
	"context"

	"github.com/stodulski/vibe-server/internal/social"
	socialmeta "github.com/stodulski/vibe-server/internal/social/meta"
	socialthreads "github.com/stodulski/vibe-server/internal/social/threads"
)

// socialPlatform is the publisher's view of Meta and Threads together. Facebook
// and Instagram go through the Meta client, Threads through its own client.
type socialPlatform struct {
	meta    *socialmeta.Client
	threads *socialthreads.Client
}

func (p socialPlatform) InstagramQuota(ctx context.Context) (int, int, error) {
	return p.meta.InstagramQuota(ctx)
}

func (p socialPlatform) PublishInstagram(ctx context.Context, post social.Post) (string, error) {
	return p.meta.PublishInstagram(ctx, post)
}

func (p socialPlatform) PublishFacebook(ctx context.Context, post social.Post) (string, error) {
	return p.meta.PublishFacebook(ctx, post)
}

func (p socialPlatform) InstagramPermalink(ctx context.Context, mediaID string) (string, error) {
	return p.meta.InstagramPermalink(ctx, mediaID)
}

func (p socialPlatform) PublishThreads(ctx context.Context, post social.Post) (string, error) {
	return p.threads.PublishThreads(ctx, post)
}

func (p socialPlatform) ThreadsPermalink(ctx context.Context, mediaID string) (string, error) {
	return p.threads.ThreadsPermalink(ctx, mediaID)
}
