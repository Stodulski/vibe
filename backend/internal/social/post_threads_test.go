package social

import (
	"strings"
	"testing"
)

func threadsFields() Fields {
	return Fields{
		PublishAt: "2026-10-20T12:00:00-03:00",
		Networks:  []string{"instagram", "threads"},
		Type:      "image",
		Caption:   "Hola padeleros",
		Media:     []string{"a.jpg"},
	}
}

func repeat(s string, n int) string { return strings.Repeat(s, n) }

func TestTextPostGoesToThreadsAlone(t *testing.T) {
	f := Fields{
		PublishAt: "2026-10-20T12:00:00-03:00",
		Networks:  []string{"threads"},
		Type:      "text",
		Caption:   "Solo texto",
	}
	post, errs := ValidatePost("texto", f)
	if errs != nil {
		t.Fatalf("a Threads-only text post is valid; got %v", errs)
	}
	if post.Kind != TypeText {
		t.Fatalf("kind = %q; want text", post.Kind)
	}

	f.Networks = []string{"threads", "instagram"}
	if _, errs := ValidatePost("texto", f); errs["type"] == "" {
		t.Fatalf("a text post naming another network must fail on type; got %v", errs)
	}
}

func TestTextPostListsNoFiles(t *testing.T) {
	f := Fields{
		PublishAt: "2026-10-20T12:00:00-03:00",
		Networks:  []string{"threads"},
		Type:      "text",
		Caption:   "Solo texto",
		Media:     []string{"a.jpg"},
	}
	if _, errs := ValidatePost("texto", f); errs["media"] == "" {
		t.Fatalf("a text post with media must fail on media; got %v", errs)
	}
}

func TestCoverIsAllowedOnlyForReels(t *testing.T) {
	f := threadsFields()
	f.Cover = "cover.jpg"
	if _, errs := ValidatePost("foto", f); errs["cover"] == "" {
		t.Fatalf("a cover on an image post must fail on cover; got %v", errs)
	}

	reel := Fields{
		PublishAt: "2026-10-20T12:00:00-03:00",
		Networks:  []string{"instagram"},
		Type:      "reel",
		Caption:   "Un reel",
		Media:     []string{"clip.mp4"},
		Cover:     "cover.jpg",
	}
	post, errs := ValidatePost("reel", reel)
	if errs != nil {
		t.Fatalf("a reel with a JPEG cover is valid; got %v", errs)
	}
	if post.Cover == nil || post.Cover.Name != "cover.jpg" {
		t.Fatalf("cover = %+v; want cover.jpg", post.Cover)
	}
	for _, m := range post.Media {
		if m.Name == "cover.jpg" {
			t.Fatal("the cover must not be listed among the media")
		}
	}
}

func TestCoverIsAPlainJPEGNotListedAsMedia(t *testing.T) {
	reel := Fields{
		PublishAt: "2026-10-20T12:00:00-03:00",
		Networks:  []string{"instagram"},
		Type:      "reel",
		Caption:   "Un reel",
		Media:     []string{"clip.mp4"},
	}
	for _, name := range []string{"cover.png", "../up.jpg", "clip.mp4", "clip.mp4 "} {
		reel.Cover = name
		if _, errs := ValidatePost("reel", reel); errs["cover"] == "" {
			t.Fatalf("cover %q must fail on cover; got %v", name, errs)
		}
	}

	reel.Cover = "clip.mp4"
	if _, errs := ValidatePost("reel", reel); errs["cover"] == "" {
		t.Fatalf("a cover that is also a media file must fail; got %v", errs)
	}
}

func TestThreadsLengthCountsEmojiAsFour(t *testing.T) {
	if got := ThreadsLength("ab😀"); got != 6 {
		t.Fatalf("ThreadsLength(\"ab😀\") = %d; want 6", got)
	}
	if got := ThreadsLength("ñandú"); got != 5 {
		t.Fatalf("accented letters count once; got %d", got)
	}
}

func TestThreadsTextLimitsAndRequirement(t *testing.T) {
	f := threadsFields()
	f.Caption = repeat("a", 600)
	if _, errs := ValidatePost("largo", f); errs["threads_text"] == "" {
		t.Fatalf("a caption over 500 characters needs threads_text; got %v", errs)
	}

	f.ThreadsText = repeat("a", 501)
	if _, errs := ValidatePost("largo", f); errs["threads_text"] == "" {
		t.Fatalf("threads_text over 500 characters must fail; got %v", errs)
	}

	f.ThreadsText = repeat("a", 480) + "."
	post, errs := ValidatePost("largo", f)
	if errs != nil {
		t.Fatalf("a 481 character threads_text is valid; got %v", errs)
	}
	if post.ThreadsText != f.ThreadsText {
		t.Fatalf("threads text = %q; want the trimmed value", post.ThreadsText)
	}
}

func TestThreadsTextCountsEmojiByBytes(t *testing.T) {
	f := threadsFields()
	f.ThreadsText = repeat("😀", 126) // 504 by the Threads rule
	if _, errs := ValidatePost("emoji", f); errs["threads_text"] == "" {
		t.Fatalf("126 emoji exceed 500 by the Threads rule; got %v", errs)
	}
	f.ThreadsText = repeat("😀", 125) // exactly 500
	if _, errs := ValidatePost("emoji", f); errs != nil {
		t.Fatalf("125 emoji are exactly 500; got %v", errs)
	}
}

func TestTextPostFallsBackToTheCaptionForThreads(t *testing.T) {
	f := Fields{
		PublishAt: "2026-10-20T12:00:00-03:00",
		Networks:  []string{"threads"},
		Type:      "text",
		Caption:   repeat("b", 300),
	}
	if _, errs := ValidatePost("corto", f); errs != nil {
		t.Fatalf("a 300 character caption is a valid Threads text; got %v", errs)
	}
}

func TestThreadsFieldsNeedThreadsInNetworks(t *testing.T) {
	f := threadsFields()
	f.Networks = []string{"instagram"}
	f.ThreadsText = "hola"
	if _, errs := ValidatePost("sin", f); errs["threads_text"] == "" {
		t.Fatalf("threads_text without threads must fail; got %v", errs)
	}
	f.ThreadsText = ""
	f.ThreadsTopic = "padel"
	if _, errs := ValidatePost("sin", f); errs["threads_topic"] == "" {
		t.Fatalf("threads_topic without threads must fail; got %v", errs)
	}
}

func TestThreadsTopicRules(t *testing.T) {
	f := threadsFields()
	f.ThreadsTopic = "padel"
	post, errs := ValidatePost("tema", f)
	if errs != nil || post.ThreadsTopic != "padel" {
		t.Fatalf("a plain topic is valid; got %v, topic %q", errs, post.ThreadsTopic)
	}
	for _, bad := range []string{"a.b", "a&b", repeat("t", 51), repeat("😀", 13)} {
		f.ThreadsTopic = bad
		if _, errs := ValidatePost("tema", f); errs["threads_topic"] == "" {
			t.Fatalf("topic %q must fail; got %v", bad, errs)
		}
	}
	f.ThreadsTopic = repeat("t", 50)
	if _, errs := ValidatePost("tema", f); errs != nil {
		t.Fatalf("a 50 character topic is valid; got %v", errs)
	}
}
