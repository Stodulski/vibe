package social

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const storyPost = `{"publish_at":"2026-10-20T19:00:00-03:00","networks":["instagram","facebook"],"type":"story","media":["1.mp4"]}`

func TestPutPostStoresAStoryWithoutACaption(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "historia-1", "1.mp4")

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, storyPost), "historia-1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", w.Code, w.Body.String())
	}
	stored, ok := rg.store.posts["historia-1"]
	if !ok || stored.Kind != TypeStory || stored.Caption != "" || len(stored.Media) != 1 || stored.Media[0].Name != "1.mp4" {
		t.Fatalf("stored post = %+v, want a captionless story with its video", stored)
	}
}

func TestPutPostStoresAStorySequenceInOrderAfterItsFeedPost(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "historia-seq", "1.jpg")
	rg.stage(t, "historia-seq", "2.mp4")
	rg.stage(t, "historia-seq", "3.jpg")
	body := `{"publish_at":"2026-10-20T19:00:00-03:00","networks":["instagram","facebook"],"type":"story","media":["1.jpg","2.mp4","3.jpg"],"after":"2026-10-20-caja"}`

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, body), "historia-seq")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", w.Code, w.Body.String())
	}
	stored := rg.store.posts["historia-seq"]
	if len(stored.Media) != 3 || stored.Media[1].Name != "2.mp4" || len(stored.StoryItems) != 3 || stored.After != "2026-10-20-caja" {
		t.Fatalf("stored = %+v, want three files in order with the feed post", stored)
	}
}

func TestPutPostRefusesAStoryToThreads(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "historia-2", "1.jpg")
	body := `{"publish_at":"2026-10-20T19:00:00-03:00","networks":["threads"],"type":"story","media":["1.jpg"]}`

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, body), "historia-2")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s; want 422", w.Code, w.Body.String())
	}
	if _, ok := rg.store.posts["historia-2"]; ok {
		t.Fatal("a refused story was stored")
	}
}
