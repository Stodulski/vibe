package social

import (
	"fmt"
	"testing"
	"time"
)

func storyFields(media ...string) Fields {
	f := validFields()
	f.Type = "story"
	f.Caption = ""
	f.AltText = ""
	f.Networks = []string{"instagram", "facebook"}
	f.Media = media
	return f
}

func TestStoryAcceptsSequencesOfOneToSixFiles(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"one image":  {"1.jpg"},
		"four mixed": {"1.jpg", "2.mp4", "3.jpeg", "4.jpg"},
		"six videos": {"1.mp4", "2.mp4", "3.mp4", "4.mp4", "5.mp4", "6.mp4"},
	}
	for name, media := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			post, errs := ValidatePost("historia", storyFields(media...))
			if errs != nil {
				t.Fatalf("ValidatePost() errors = %v, want none", errs)
			}
			if len(post.Media) != len(media) || len(post.StoryItems) != len(media) {
				t.Fatalf("media = %d items = %d, want %d of each", len(post.Media), len(post.StoryItems), len(media))
			}
		})
	}
}

func TestStoryRejectsNoFilesOrMoreThanSix(t *testing.T) {
	t.Parallel()
	seven := []string{"1.jpg", "2.jpg", "3.jpg", "4.jpg", "5.jpg", "6.jpg", "7.jpg"}
	cases := map[string][]string{"no files": nil, "seven files": seven}
	for name, media := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, errs := ValidatePost("historia", storyFields(media...))
			if _, ok := errs["media"]; !ok {
				t.Fatalf("errors = %v, want a media error", errs)
			}
		})
	}
}

func TestStoryRejectsAFileOfAnotherTypeAnywhereInTheSequence(t *testing.T) {
	t.Parallel()
	_, errs := ValidatePost("historia", storyFields("1.jpg", "2.png", "3.mp4"))
	if _, ok := errs["media"]; !ok {
		t.Fatalf("errors = %v, want a media error for the png", errs)
	}
}

func TestStoryItemsArePublishedOneMinuteApartInOrder(t *testing.T) {
	t.Parallel()
	post, errs := ValidatePost("historia", storyFields("1.jpg", "2.mp4", "3.jpg", "4.jpg"))
	if errs != nil {
		t.Fatalf("ValidatePost() errors = %v", errs)
	}
	for i, item := range post.StoryItems {
		want := post.PublishAt.Add(time.Duration(i) * 60 * time.Second)
		if item.Position != i || !item.ScheduledAt.Equal(want) {
			t.Fatalf("item %d = %+v, want position %d at %v", i, item, i, want)
		}
	}
	if !post.StoryItems[0].ScheduledAt.Equal(post.PublishAt) {
		t.Fatalf("first story at %v, want the publish time %v", post.StoryItems[0].ScheduledAt, post.PublishAt)
	}
}

func TestStoryAcceptsAfterAnotherFeedPost(t *testing.T) {
	t.Parallel()
	f := storyFields("1.jpg")
	f.After = "2026-10-20-caja"
	post, errs := ValidatePost("historia-1", f)
	if errs != nil {
		t.Fatalf("ValidatePost() errors = %v", errs)
	}
	if post.After != "2026-10-20-caja" {
		t.Fatalf("after = %q, want the feed post id", post.After)
	}
}

func TestAfterIsRejectedUnlessTheTypeIsAStory(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"image", "carousel", "reel"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			f := validFields()
			f.Type = kind
			if kind == "carousel" {
				f.Media = []string{"1.jpg", "2.jpg"}
			}
			if kind == "reel" {
				f.Media = []string{"clip.mp4"}
			}
			f.After = "2026-10-20-caja"
			_, errs := ValidatePost("p", f)
			if _, ok := errs["after"]; !ok {
				t.Fatalf("errors = %v, want an after error on a %s", errs, kind)
			}
		})
	}
}

func TestAfterMustBeAnotherValidPostID(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"not a post id": "Caja_Turnos",
		"its own id":    "historia-1",
	}
	for name, after := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := storyFields("1.jpg")
			f.After = after
			_, errs := ValidatePost("historia-1", f)
			if _, ok := errs["after"]; !ok {
				t.Fatalf("errors = %v, want an after error for %q", errs, after)
			}
		})
	}
}

func TestStoryWithSixFilesKeepsEverySchedule(t *testing.T) {
	t.Parallel()
	media := make([]string, 0, MaxStoryFiles)
	for i := range MaxStoryFiles {
		media = append(media, fmt.Sprintf("%d.jpg", i+1))
	}
	post, errs := ValidatePost("historia-6", storyFields(media...))
	if errs != nil || len(post.StoryItems) != MaxStoryFiles {
		t.Fatalf("ValidatePost() = %d items, errors %v; want %d items", len(post.StoryItems), errs, MaxStoryFiles)
	}
}
