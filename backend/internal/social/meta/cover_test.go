package meta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/social"
)

// reelGraph records the form of each container created and reports every
// container FINISHED, so a reel can run to publication.
type reelGraph struct {
	mu    sync.Mutex
	forms []map[string]string
}

func (g *reelGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/media_publish"):
		_, _ = w.Write([]byte(`{"id":"ig-reel-1"}`))
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/media"):
		_ = r.ParseForm()
		form := map[string]string{}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		g.forms = append(g.forms, form)
		_, _ = w.Write([]byte(`{"id":"container-1"}`))
	default:
		_ = json.NewEncoder(w).Encode(map[string]string{"status_code": "FINISHED"})
	}
}

func TestInstagramReelSendsItsCoverAsCoverURL(t *testing.T) {
	t.Parallel()
	g := &reelGraph{}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	c := New(Config{Token: "system-token", APIBase: srv.URL, PollEvery: time.Millisecond, PollFor: time.Second})
	c.acct = &account{pageID: "page-1", pageToken: "page-token", igUserID: "ig-1"}

	post := social.Post{
		ID: "reel", Kind: social.TypeReel, Caption: "Un reel",
		Media: []social.Media{{Name: "clip.mp4", URL: "https://cdn.test/clip.mp4"}},
		Cover: &social.Media{Name: "cover.jpg", URL: "https://cdn.test/cover.jpg"},
	}
	id, err := c.PublishInstagram(context.Background(), post)
	if err != nil || id != "ig-reel-1" {
		t.Fatalf("publish = %q, %v", id, err)
	}
	if len(g.forms) != 1 {
		t.Fatalf("containers = %d; want one reel container", len(g.forms))
	}
	if got := g.forms[0]["cover_url"]; got != "https://cdn.test/cover.jpg" {
		t.Fatalf("cover_url = %q; want the uploaded cover", got)
	}
	if g.forms[0]["media_type"] != "REELS" || g.forms[0]["video_url"] != "https://cdn.test/clip.mp4" {
		t.Fatalf("form = %v; want a REELS container for the video", g.forms[0])
	}
}

func TestInstagramReelWithoutACoverSendsNoCoverURL(t *testing.T) {
	t.Parallel()
	g := &reelGraph{}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	c := New(Config{Token: "system-token", APIBase: srv.URL, PollEvery: time.Millisecond, PollFor: time.Second})
	c.acct = &account{pageID: "page-1", pageToken: "page-token", igUserID: "ig-1"}

	post := social.Post{ID: "reel", Kind: social.TypeReel, Caption: "Un reel",
		Media: []social.Media{{Name: "clip.mp4", URL: "https://cdn.test/clip.mp4"}}}
	if _, err := c.PublishInstagram(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, ok := g.forms[0]["cover_url"]; ok {
		t.Fatal("a reel without a cover must not send cover_url")
	}
}
