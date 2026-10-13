package meta

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stodulski/vibe-server/internal/social"
)

func TestInstagramStoryImageSendsNoCaptionOrAltText(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			return http.StatusOK, `{"id":"c-story"}`
		case e.method == http.MethodGet && e.path == "/v25.0/c-story":
			return http.StatusOK, `{"status_code":"FINISHED"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media_publish":
			return http.StatusOK, `{"id":"ig-story-1"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishInstagram(context.Background(), imagePost(social.TypeStory, "1.jpg"))
	if err != nil || id != "ig-story-1" {
		t.Fatalf("PublishInstagram() = %q, %v; want ig-story-1", id, err)
	}
	create := fake.calls()[1]
	if create.params.Get("media_type") != "STORIES" || create.params.Get("image_url") != "https://cdn.test/1.jpg" {
		t.Fatalf("container params = %v, want media_type STORIES and image_url", create.params)
	}
	for _, key := range []string{"caption", "alt_text", "video_url", "share_to_feed"} {
		if _, ok := create.params[key]; ok {
			t.Fatalf("story container sent %q, want it absent: %v", key, create.params)
		}
	}
}

func TestInstagramStoryVideoWaitsForTheVideoContainer(t *testing.T) {
	t.Parallel()
	var polls atomic.Int32
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			return http.StatusOK, `{"id":"c-video"}`
		case e.method == http.MethodGet && e.path == "/v25.0/c-video":
			if polls.Add(1) < 2 {
				return http.StatusOK, `{"status_code":"IN_PROGRESS"}`
			}
			return http.StatusOK, `{"status_code":"FINISHED"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media_publish":
			return http.StatusOK, `{"id":"ig-story-2"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishInstagram(context.Background(), imagePost(social.TypeStory, "clip.mp4"))
	if err != nil || id != "ig-story-2" {
		t.Fatalf("PublishInstagram() = %q, %v; want ig-story-2", id, err)
	}
	create := fake.calls()[1]
	if create.params.Get("media_type") != "STORIES" || create.params.Get("video_url") != "https://cdn.test/clip.mp4" {
		t.Fatalf("container params = %v, want media_type STORIES and video_url", create.params)
	}
	if _, ok := create.params["image_url"]; ok {
		t.Fatalf("video story container sent image_url: %v", create.params)
	}
	if got := polls.Load(); got != 2 {
		t.Fatalf("polls = %d, want 2 (one IN_PROGRESS, one FINISHED)", got)
	}
}

func TestFacebookPhotoStoryUploadsUnpublishedPhotoThenPublishesIt(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch e.path {
		case "/v25.0/page-1/photos":
			return http.StatusOK, `{"id":"photo-7"}`
		case "/v25.0/page-1/photo_stories":
			return http.StatusOK, `{"success":true,"post_id":1234}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishFacebook(context.Background(), imagePost(social.TypeStory, "1.jpg"))
	if err != nil || id != "1234" {
		t.Fatalf("PublishFacebook() = %q, %v; want 1234 from a numeric post_id", id, err)
	}
	photo, story := fake.calls()[1], fake.calls()[2]
	if photo.params.Get("published") != "false" || photo.params.Get("url") != "https://cdn.test/1.jpg" || photo.params.Get("access_token") != pageToken {
		t.Fatalf("photo params = %v, want an unpublished photo with the page token", photo.params)
	}
	if story.params.Get("photo_id") != "photo-7" || story.params.Get("access_token") != pageToken {
		t.Fatalf("photo_stories params = %v, want photo_id photo-7 with the page token", story.params)
	}
	if _, ok := story.params["message"]; ok {
		t.Fatalf("photo story sent a message, but stories take none: %v", story.params)
	}
}

func TestFacebookPhotoStoryRefusalIsAnError(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch e.path {
		case "/v25.0/page-1/photos":
			return http.StatusOK, `{"id":"photo-8"}`
		case "/v25.0/page-1/photo_stories":
			return http.StatusOK, `{"success":false}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	if _, err := client.PublishFacebook(context.Background(), imagePost(social.TypeStory, "1.jpg")); err == nil {
		t.Fatal("PublishFacebook() accepted a refused photo story")
	}
}

func TestFacebookVideoStoryRunsTheSessionThenPublishesIt(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.path == "/v25.0/page-1/video_stories" && e.params.Get("upload_phase") == "start":
			return http.StatusOK, `{"video_id":"vid-7","upload_url":"x"}`
		case e.path == "/upload/v25.0/vid-7":
			return http.StatusOK, `{"success":true}`
		case e.path == "/v25.0/page-1/video_stories" && e.params.Get("upload_phase") == "finish":
			return http.StatusOK, `{"success":true,"post_id":"page-1_777"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishFacebook(context.Background(), imagePost(social.TypeStory, "clip.mp4"))
	if err != nil || id != "page-1_777" {
		t.Fatalf("PublishFacebook() = %q, %v; want page-1_777", id, err)
	}
	var upload, finish exchange
	for _, c := range fake.calls() {
		switch {
		case c.path == "/upload/v25.0/vid-7":
			upload = c
		case c.params.Get("upload_phase") == "finish":
			finish = c
		}
	}
	if upload.header.Get("file_url") != "https://cdn.test/clip.mp4" || upload.header.Get("Authorization") != "OAuth "+pageToken {
		t.Fatalf("upload headers = %v, want file_url and an OAuth page token", upload.header)
	}
	if finish.params.Get("video_id") != "vid-7" {
		t.Fatalf("finish params = %v, want video_id vid-7", finish.params)
	}
	if _, ok := finish.params["description"]; ok {
		t.Fatalf("video story finish sent a description: %v", finish.params)
	}
}
