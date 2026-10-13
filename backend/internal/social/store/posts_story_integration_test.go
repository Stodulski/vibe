//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/social"
	"github.com/stodulski/vibe-server/internal/social/store"
)

func storySample(id string, names ...string) social.Post {
	p := social.Post{
		ID: id, PublishAt: due, Networks: []social.Network{social.NetworkInstagram, social.NetworkFacebook},
		Kind: social.TypeStory, Status: social.StatusScheduled, NextAttemptAt: &due, After: "2026-10-20-caja",
	}
	for i, name := range names {
		p.Media = append(p.Media, social.Media{Name: name, URL: "https://cdn.test/social/" + name, ContentType: "image/jpeg", SHA256: "abc", SizeBytes: 9})
		p.StoryItems = append(p.StoryItems, social.StoryItem{Position: i, ScheduledAt: due.Add(time.Duration(i) * social.StoryGap)})
	}
	return p
}

func TestIntegration_StorySequenceKeepsItsItemsAndAfterAndFreezesOncePublished(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	if err := posts.Upsert(ctx, storySample("historia-1", "1.jpg", "2.mp4", "3.jpg")); err != nil {
		t.Fatalf("Upsert(story sequence) = %v", err)
	}
	got, ok, err := posts.Lookup(ctx, "historia-1")
	if err != nil || !ok {
		t.Fatalf("Lookup() = %v, %v; want the story", ok, err)
	}
	if got.Kind != social.TypeStory || got.Caption != "" || got.After != "2026-10-20-caja" || len(got.StoryItems) != 3 {
		t.Fatalf("looked up = %+v, want a captionless story with 3 items after the feed post", got)
	}
	if !got.StoryItems[2].ScheduledAt.Equal(due.Add(2 * social.StoryGap)) {
		t.Fatalf("item 3 at %v, want %v", got.StoryItems[2].ScheduledAt, due.Add(2*social.StoryGap))
	}

	got.Status = social.StatusPartial
	got.StoryItems[0].IGMediaID = "ig-story-1"
	if err := posts.RecordResult(ctx, got); err != nil {
		t.Fatalf("RecordResult() = %v", err)
	}
	reread, _, err := posts.Lookup(ctx, "historia-1")
	if err != nil || reread.StoryItems[0].IGMediaID != "ig-story-1" || reread.StoryItems[1].IGMediaID != "" {
		t.Fatalf("after RecordResult the items = %+v, %v; want only item 1 with an id", reread.StoryItems, err)
	}
	frozen, err := posts.IsFrozen(ctx, "historia-1")
	if err != nil || !frozen {
		t.Fatalf("IsFrozen() = %v, %v; want true once one story has an id", frozen, err)
	}
	if err := posts.Upsert(ctx, storySample("historia-1", "1.jpg")); !errors.Is(err, social.ErrFrozen) {
		t.Fatalf("Upsert on a story with an id = %v, want ErrFrozen", err)
	}
}

func TestIntegration_StoryIsStoredWithoutACaptionAndClaimed(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	story := storySample("historia-2", "1.mp4")
	story.After = ""
	if err := posts.Upsert(ctx, story); err != nil {
		t.Fatalf("Upsert(story without caption) = %v", err)
	}
	claimed, err := posts.ClaimDue(ctx, due.Add(time.Minute), 10, 15*time.Minute)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("ClaimDue() = %d posts, %v; want the story", len(claimed), err)
	}
	if got := claimed[0]; got.Kind != social.TypeStory || got.Caption != "" || got.After != "" || len(got.StoryItems) != 1 {
		t.Fatalf("claimed = %+v, want a captionless story with one item and no after", got)
	}
}

func TestIntegration_DatabaseRefusesAStoryToThreadsAndAfterOnAnotherType(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	toThreads := storySample("historia-threads", "1.jpg")
	toThreads.Networks = []social.Network{social.NetworkThreads}
	toThreads.After = ""
	if err := posts.Upsert(ctx, toThreads); err == nil {
		t.Fatal("Upsert(story to threads) succeeded, want the database to refuse it")
	}

	image := samplePost("imagen-con-after", social.NetworkInstagram)
	image.After = "2026-10-20-caja"
	if err := posts.Upsert(ctx, image); err == nil {
		t.Fatal("Upsert(image with after) succeeded, want the database to refuse it")
	}
}
