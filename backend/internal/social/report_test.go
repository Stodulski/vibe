package social

import (
	"strings"
	"testing"
)

func storyReportPost(status Status, networks []Network, ig, fb []string) Post {
	p := Post{ID: "historia-1", Status: status, Kind: TypeStory, Networks: networks}
	for i := range max(len(ig), len(fb), 3) {
		item := StoryItem{Position: i}
		if i < len(ig) {
			item.IGMediaID = ig[i]
		}
		if i < len(fb) {
			item.FBPostID = fb[i]
		}
		p.StoryItems = append(p.StoryItems, item)
	}
	return p
}

func TestStoryReportNamesTheFormatAndCountsEachNetwork(t *testing.T) {
	t.Parallel()
	rep := BuildReport("owner@vibe.test", Report{
		Post:         storyReportPost(StatusPublished, []Network{NetworkInstagram, NetworkFacebook}, []string{"a", "b", "c"}, []string{"x", "y", "z"}),
		IGStoryLinks: []string{"https://www.instagram.com/stories/vibe/a/", "https://www.instagram.com/stories/vibe/b/"},
	})
	joined := strings.Join(rep.Lines, "\n")
	for _, want := range []string{
		"Formato: Historia.",
		"Instagram: 3 de 3 historias publicadas; expiran en 24 horas.",
		"Enlaces de Instagram: https://www.instagram.com/stories/vibe/a/ https://www.instagram.com/stories/vibe/b/",
		"Facebook: 3 de 3 historias publicadas; expiran en 24 horas.",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("report lines = %q, want a line containing %q", joined, want)
		}
	}
	if strings.Contains(joined, "facebook.com") {
		t.Fatalf("report lines = %q, a story must not link a Facebook post URL", joined)
	}
}

func TestStoryReportSaysHowManyStoriesWentOutWhenItStopped(t *testing.T) {
	t.Parallel()
	rep := BuildReport("owner@vibe.test", Report{
		Post: storyReportPost(StatusPartial, []Network{NetworkInstagram, NetworkFacebook}, []string{"a", "b"}, nil),
	})
	joined := strings.Join(rep.Lines, "\n")
	for _, want := range []string{
		"Estado: publicada parcialmente.",
		"Instagram: 2 de 3 historias publicadas; expiran en 24 horas.",
		"Facebook: no publicada.",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("report lines = %q, want a line containing %q", joined, want)
		}
	}
}

func TestStoryReportNamesTheLastErrorWhenAStoryFailed(t *testing.T) {
	t.Parallel()
	post := storyReportPost(StatusFailed, []Network{NetworkInstagram}, nil, nil)
	post.LastError = "the feed post caja was not published on instagram"
	rep := BuildReport("owner@vibe.test", Report{Post: post})
	joined := strings.Join(rep.Lines, "\n")
	for _, want := range []string{
		"Estado: fallida.",
		"Instagram: no publicada.",
		"Último error: the feed post caja was not published on instagram",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("report lines = %q, want a line containing %q", joined, want)
		}
	}
}
