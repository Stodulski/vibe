//go:build integration

package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/social"
	"github.com/stodulski/vibe-server/internal/social/store"
)

var due = time.Date(2026, 10, 20, 22, 0, 0, 0, time.UTC)

func samplePost(id string, networks ...social.Network) social.Post {
	return social.Post{
		ID: id, PublishAt: due, Networks: networks, Kind: social.TypeImage,
		Caption: "Caja de turnos", AltText: "Una caja", Status: social.StatusScheduled,
		NextAttemptAt: &due,
	}
}

func TestIntegration_PostsRoundTripAndFreezeOnceAPublishRuns(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	if err := posts.Upsert(ctx, samplePost("p1", social.NetworkInstagram, social.NetworkFacebook)); err != nil {
		t.Fatalf("Upsert() = %v", err)
	}
	media := social.Media{Name: "1.jpg", URL: "https://cdn.test/social/abc.jpg", ContentType: "image/jpeg", SHA256: "abc", SizeBytes: 9}
	if err := posts.StageMedia(ctx, "p1", media); err != nil {
		t.Fatalf("StageMedia() = %v", err)
	}
	staged, err := posts.StagedMedia(ctx, "p1", []string{"1.jpg", "missing.jpg"})
	if err != nil || len(staged) != 1 || staged[0] != media {
		t.Fatalf("StagedMedia() = %+v, %v; want the one staged record", staged, err)
	}

	// Copy the staged media into the post the way the handler does.
	p := samplePost("p1", social.NetworkInstagram, social.NetworkFacebook)
	p.Media = staged
	if err := posts.Upsert(ctx, p); err != nil {
		t.Fatalf("Upsert with media = %v", err)
	}

	claimed, err := posts.ClaimDue(ctx, due.Add(time.Minute), 10, 15*time.Minute)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("ClaimDue() = %d posts, %v; want 1", len(claimed), err)
	}
	got := claimed[0]
	if got.Status != social.StatusPublishing || got.Kind != social.TypeImage || len(got.Media) != 1 || got.Media[0] != media {
		t.Fatalf("claimed post = %+v, want publishing with its media", got)
	}
	if !slices.Equal(got.Networks, []social.Network{social.NetworkInstagram, social.NetworkFacebook}) {
		t.Fatalf("networks = %v, want instagram and facebook", got.Networks)
	}

	got.Status = social.StatusPartial
	got.IGMediaID = "m-1"
	if err := posts.RecordResult(ctx, got); err != nil {
		t.Fatalf("RecordResult() = %v", err)
	}
	frozen, err := posts.IsFrozen(ctx, "p1")
	if err != nil || !frozen {
		t.Fatalf("IsFrozen() = %v, %v; want true once a network has published", frozen, err)
	}
	if err := posts.Upsert(ctx, p); !errors.Is(err, social.ErrFrozen) {
		t.Fatalf("Upsert on a published post = %v, want ErrFrozen", err)
	}
}

func TestIntegration_CancelledPostIsReopenedByAnUpsert(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	if err := posts.Upsert(ctx, samplePost("gone", social.NetworkFacebook)); err != nil {
		t.Fatalf("Upsert() = %v", err)
	}
	cancelled, err := posts.CancelUnpublished(ctx, nil)
	if err != nil || !slices.Equal(cancelled, []string{"gone"}) {
		t.Fatalf("CancelUnpublished() = %v, %v; want [gone]", cancelled, err)
	}
	if n, _ := posts.ClaimDue(ctx, due.Add(time.Hour), 10, time.Minute); len(n) != 0 {
		t.Fatal("a cancelled post must not be claimed")
	}

	if err := posts.Upsert(ctx, samplePost("gone", social.NetworkFacebook)); err != nil {
		t.Fatalf("Upsert of a cancelled post = %v, want it reopened", err)
	}
	if n, _ := posts.ClaimDue(ctx, due.Add(time.Hour), 10, time.Minute); len(n) != 1 {
		t.Fatalf("claimed %d after reopening, want 1", len(n))
	}
}

func TestIntegration_CancelUnpublishedSparesKeptAndPublishedPosts(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	for _, id := range []string{"keep", "drop", "live"} {
		if err := posts.Upsert(ctx, samplePost(id, social.NetworkFacebook)); err != nil {
			t.Fatalf("Upsert %s = %v", id, err)
		}
	}
	live := samplePost("live", social.NetworkFacebook)
	live.Status = social.StatusPublished
	live.FBPostID = "fb-9"
	if err := posts.RecordResult(ctx, live); err != nil {
		t.Fatalf("RecordResult() = %v", err)
	}

	cancelled, err := posts.CancelUnpublished(ctx, []string{"keep"})
	if err != nil || !slices.Equal(cancelled, []string{"drop"}) {
		t.Fatalf("CancelUnpublished(keep) = %v, %v; want [drop]", cancelled, err)
	}
}

func TestIntegration_ClaimDueLeasesAndSkipsDraftsAndFuturePosts(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	draft := samplePost("draft", social.NetworkFacebook)
	draft.Draft = true
	future := samplePost("future", social.NetworkFacebook)
	later := due.Add(24 * time.Hour)
	future.PublishAt, future.NextAttemptAt = later, &later
	for _, p := range []social.Post{draft, future, samplePost("due", social.NetworkFacebook)} {
		if err := posts.Upsert(ctx, p); err != nil {
			t.Fatalf("Upsert %s = %v", p.ID, err)
		}
	}

	now := due.Add(time.Minute)
	first, err := posts.ClaimDue(ctx, now, 10, 15*time.Minute)
	if err != nil || len(first) != 1 || first[0].ID != "due" {
		t.Fatalf("first claim = %+v, %v; want only the due post", first, err)
	}
	if second, _ := posts.ClaimDue(ctx, now, 10, 15*time.Minute); len(second) != 0 {
		t.Fatal("a leased post must not be claimed again before its lease ends")
	}
	if again, _ := posts.ClaimDue(ctx, now.Add(16*time.Minute), 10, 15*time.Minute); len(again) != 1 {
		t.Fatal("a post whose lease expired must be claimed again after a crash")
	}
}

func TestIntegration_IsFrozenIsFalseForAnUnknownPost(t *testing.T) {
	f := datatest.Isolated(t)
	frozen, err := store.NewPosts(f.DB).IsFrozen(context.Background(), "nope")
	if err != nil || frozen {
		t.Fatalf("IsFrozen(unknown) = %v, %v; want false, nil", frozen, err)
	}
}
