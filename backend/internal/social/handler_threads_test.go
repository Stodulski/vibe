package social

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const reelWithCover = `{"publish_at":"2026-10-20T19:00:00-03:00","networks":["instagram"],` +
	`"type":"reel","caption":"Un reel","alt_text":"","media":["clip.mp4"],"cover":"cover.jpg","draft":false}`

const threadsTextPost = `{"publish_at":"2026-10-20T19:00:00-03:00","networks":["threads"],` +
	`"type":"text","caption":"Solo texto para Threads","alt_text":"","media":[],"draft":false}`

func TestPutPostAttachesTheStagedCover(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "p1", "clip.mp4")
	rg.stage(t, "p1", "cover.jpg")

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, reelWithCover), "p1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", w.Code, w.Body.String())
	}
	stored := rg.store.posts["p1"]
	if stored.Cover == nil || stored.Cover.Name != "cover.jpg" || stored.Cover.SHA256 != "s" {
		t.Fatalf("cover = %+v; want the staged cover.jpg record", stored.Cover)
	}
	if len(stored.Media) != 1 || stored.Media[0].Name != "clip.mp4" {
		t.Fatalf("media = %+v; want only the video", stored.Media)
	}
}

func TestPutPostNamesTheCoverThatWasNeverUploaded(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "p1", "clip.mp4")

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, reelWithCover), "p1")
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "cover.jpg") {
		t.Fatalf("status = %d, body = %s; want 422 naming cover.jpg", w.Code, w.Body.String())
	}
}

func TestPutPostStoresAThreadsTextPostWithoutMedia(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, threadsTextPost), "p1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", w.Code, w.Body.String())
	}
	if stored := rg.store.posts["p1"]; stored.Kind != TypeText || len(stored.Networks) != 1 {
		t.Fatalf("stored = %+v; want a text post for threads", stored)
	}
}
