package meta

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/social"
)

const (
	systemToken = "system-token-value"
	pageToken   = "page-token-value"
)

type exchange struct {
	method string
	path   string
	params url.Values
	header http.Header
}

// fakeGraph records every request and answers from a script.
type fakeGraph struct {
	mu     sync.Mutex
	seen   []exchange
	answer func(e exchange) (int, string)
}

func (f *fakeGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := url.Values{}
	for k, v := range r.URL.Query() {
		params[k] = v
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		_ = r.ParseForm()
		for k, v := range r.PostForm {
			params[k] = v
		}
	}
	e := exchange{method: r.Method, path: r.URL.Path, params: params, header: r.Header.Clone()}

	f.mu.Lock()
	f.seen = append(f.seen, e)
	f.mu.Unlock()

	status, body := f.answer(e)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (f *fakeGraph) calls() []exchange {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.seen)
}

func newFake(t *testing.T, answer func(e exchange) (int, string)) (*fakeGraph, *Client) {
	t.Helper()
	fake := &fakeGraph{answer: answer}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	client := New(Config{
		Token:      systemToken,
		APIBase:    srv.URL + "/v25.0",
		UploadBase: srv.URL + "/upload/v25.0",
		HTTP:       srv.Client(),
		PollEvery:  time.Millisecond,
		PollFor:    time.Second,
	})
	return fake, client
}

// standardAccount answers the Page discovery every publishing call makes first.
func standardAccount(e exchange) (int, string, bool) {
	if e.path == "/v25.0/me/accounts" {
		return http.StatusOK, `{"data":[{"id":"page-1","name":"Vibe","access_token":"` + pageToken +
			`","instagram_business_account":{"id":"ig-1"}}]}`, true
	}
	return 0, "", false
}

func imagePost(kind social.Kind, media ...string) social.Post {
	p := social.Post{ID: "p1", Kind: kind, Caption: "Caja de turnos", AltText: "Una caja", Networks: []social.Network{social.NetworkInstagram, social.NetworkFacebook}}
	for _, m := range media {
		p.Media = append(p.Media, social.Media{Name: m, URL: "https://cdn.test/" + m})
	}
	return p
}

func TestAccountDiscoveryPicksThePageAndItsInstagramAccount(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		return http.StatusNotFound, `{"error":{"message":"unexpected"}}`
	})

	acct, err := client.account(context.Background())
	if err != nil {
		t.Fatalf("account() = %v", err)
	}
	if acct.pageID != "page-1" || acct.pageToken != pageToken || acct.igUserID != "ig-1" {
		t.Fatalf("account = %+v, want page-1, the page token and ig-1", acct)
	}
}

func TestAmbiguousPagesNeedAnExplicitPageID(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		return http.StatusOK, `{"data":[` +
			`{"id":"a","access_token":"x","instagram_business_account":{"id":"ig-a"}},` +
			`{"id":"b","access_token":"y","instagram_business_account":{"id":"ig-b"}}]}`
	})

	_, err := client.account(context.Background())
	if !errors.Is(err, social.ErrPermanent) {
		t.Fatalf("account() = %v, want a permanent refusal asking for META_SOCIAL_PAGE_ID", err)
	}
}

func TestInstagramImageWaitsForAFinishedContainer(t *testing.T) {
	t.Parallel()
	var polls atomic.Int32
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			return http.StatusOK, `{"id":"c1"}`
		case e.method == http.MethodGet && e.path == "/v25.0/c1":
			if polls.Add(1) < 2 {
				return http.StatusOK, `{"status_code":"IN_PROGRESS"}`
			}
			return http.StatusOK, `{"status_code":"FINISHED"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media_publish":
			return http.StatusOK, `{"id":"m1"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishInstagram(context.Background(), imagePost(social.TypeImage, "1.jpg"))
	if err != nil || id != "m1" {
		t.Fatalf("PublishInstagram() = %q, %v; want m1", id, err)
	}
	create := fake.calls()[1]
	if create.params.Get("image_url") != "https://cdn.test/1.jpg" || create.params.Get("alt_text") != "Una caja" ||
		create.params.Get("caption") != "Caja de turnos" {
		t.Fatalf("container params = %v, want image_url, caption and alt_text", create.params)
	}
	if got := polls.Load(); got != 2 {
		t.Fatalf("polls = %d, want 2 (one IN_PROGRESS, one FINISHED)", got)
	}
	if got := fake.calls()[len(fake.calls())-1].params.Get("creation_id"); got != "c1" {
		t.Fatalf("media_publish creation_id = %q, want c1", got)
	}
}

func TestInstagramCarouselCreatesChildrenThenTheParent(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media" && e.params.Get("is_carousel_item") == "true":
			if e.params.Get("image_url") == "https://cdn.test/1.jpg" {
				return http.StatusOK, `{"id":"child-1"}`
			}
			return http.StatusOK, `{"id":"child-2"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			return http.StatusOK, `{"id":"parent"}`
		case e.method == http.MethodGet && e.path == "/v25.0/parent":
			return http.StatusOK, `{"status_code":"FINISHED"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media_publish":
			return http.StatusOK, `{"id":"m2"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	post := imagePost(social.TypeCarousel, "1.jpg", "2.jpg")
	post.AltText = ""
	id, err := client.PublishInstagram(context.Background(), post)
	if err != nil || id != "m2" {
		t.Fatalf("PublishInstagram() = %q, %v; want m2", id, err)
	}
	var parent url.Values
	for _, c := range fake.calls() {
		if c.params.Get("media_type") == "CAROUSEL" {
			parent = c.params
		}
	}
	if parent.Get("children") != "child-1,child-2" || parent.Get("caption") != "Caja de turnos" {
		t.Fatalf("parent params = %v, want children child-1,child-2", parent)
	}
	if parent.Has("alt_text") {
		t.Fatal("alt_text is for single images only and must not be sent on a carousel")
	}
}

func TestInstagramReelFailsWhenProcessingFails(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			return http.StatusOK, `{"id":"reel-c"}`
		case e.method == http.MethodGet && e.path == "/v25.0/reel-c":
			return http.StatusOK, `{"status_code":"ERROR"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	_, err := client.PublishInstagram(context.Background(), imagePost(social.TypeReel, "clip.mp4"))
	if err == nil || errors.Is(err, social.ErrPermanent) {
		t.Fatalf("PublishInstagram() = %v, want a retryable processing error", err)
	}
}

func TestClientErrorsAreMarkedPermanentAndServerErrorsAreNot(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		return http.StatusBadRequest, `{"error":{"message":"Invalid image","code":100}}`
	})
	_, err := client.PublishInstagram(context.Background(), imagePost(social.TypeImage, "1.jpg"))
	if !errors.Is(err, social.ErrPermanent) {
		t.Fatalf("400 = %v, want ErrPermanent", err)
	}

	_, client = newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		return http.StatusInternalServerError, `{"error":{"message":"oops"}}`
	})
	_, err = client.PublishInstagram(context.Background(), imagePost(social.TypeImage, "1.jpg"))
	if err == nil || errors.Is(err, social.ErrPermanent) {
		t.Fatalf("500 = %v, want a retryable error", err)
	}
}

func TestInstagramQuotaReadsUsageAndTotal(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		if e.path == "/v25.0/ig-1/content_publishing_limit" {
			return http.StatusOK, `{"data":[{"quota_usage":3,"config":{"quota_total":50,"quota_duration":86400}}]}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	used, total, err := client.InstagramQuota(context.Background())
	if err != nil || used != 3 || total != 50 {
		t.Fatalf("InstagramQuota() = %d, %d, %v; want 3, 50", used, total, err)
	}
}

func TestFacebookPhotoReturnsThePostIDWithThePageToken(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		if e.path == "/v25.0/page-1/photos" {
			return http.StatusOK, `{"id":"photo-1","post_id":"page-1_555"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishFacebook(context.Background(), imagePost(social.TypeImage, "1.jpg"))
	if err != nil || id != "page-1_555" {
		t.Fatalf("PublishFacebook() = %q, %v; want the post id", id, err)
	}
	photo := fake.calls()[len(fake.calls())-1]
	if photo.params.Get("access_token") != pageToken || photo.params.Get("alt_text_custom") != "Una caja" {
		t.Fatalf("photo params = %v, want the page token and alt text", photo.params)
	}
}

func TestFacebookCarouselUploadsUnpublishedPhotosThenPostsThem(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch e.path {
		case "/v25.0/page-1/photos":
			if e.params.Get("url") == "https://cdn.test/1.jpg" {
				return http.StatusOK, `{"id":"ph-1"}`
			}
			return http.StatusOK, `{"id":"ph-2"}`
		case "/v25.0/page-1/feed":
			return http.StatusOK, `{"id":"page-1_777"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishFacebook(context.Background(), imagePost(social.TypeCarousel, "1.jpg", "2.jpg"))
	if err != nil || id != "page-1_777" {
		t.Fatalf("PublishFacebook() = %q, %v; want page-1_777", id, err)
	}
	feed := fake.calls()[len(fake.calls())-1]
	if feed.params.Get("attached_media[0]") != `{"media_fbid":"ph-1"}` || feed.params.Get("attached_media[1]") != `{"media_fbid":"ph-2"}` {
		t.Fatalf("feed params = %v, want both photos attached in order", feed.params)
	}
	for _, c := range fake.calls() {
		if c.path == "/v25.0/page-1/photos" && c.params.Get("published") != "false" {
			t.Fatal("carousel photos must be uploaded unpublished")
		}
	}
}

func TestFacebookReelRunsTheThreeStepSession(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.path == "/v25.0/page-1/video_reels" && e.params.Get("upload_phase") == "start":
			return http.StatusOK, `{"video_id":"vid-9","upload_url":"x"}`
		case e.path == "/upload/v25.0/vid-9":
			return http.StatusOK, `{"success":true}`
		case e.path == "/v25.0/page-1/video_reels" && e.params.Get("upload_phase") == "finish":
			return http.StatusOK, `{"success":true}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishFacebook(context.Background(), imagePost(social.TypeReel, "clip.mp4"))
	if err != nil || id != "vid-9" {
		t.Fatalf("PublishFacebook() = %q, %v; want vid-9", id, err)
	}
	var upload, finish exchange
	for _, c := range fake.calls() {
		switch {
		case c.path == "/upload/v25.0/vid-9":
			upload = c
		case c.params.Get("upload_phase") == "finish":
			finish = c
		}
	}
	if upload.header.Get("file_url") != "https://cdn.test/clip.mp4" || upload.header.Get("Authorization") != "OAuth "+pageToken {
		t.Fatalf("upload headers = %v, want file_url and an OAuth page token", upload.header)
	}
	if finish.params.Get("video_state") != "PUBLISHED" || finish.params.Get("description") != "Caja de turnos" {
		t.Fatalf("finish params = %v, want PUBLISHED with the caption", finish.params)
	}
}

func TestTransportErrorsNeverShowTheToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := srv.URL
	srv.Close()
	client := New(Config{Token: systemToken, APIBase: base + "/v25.0", HTTP: &http.Client{Timeout: time.Second}})

	_, _, err := client.InstagramQuota(context.Background())
	if err == nil || strings.Contains(err.Error(), systemToken) {
		t.Fatalf("InstagramQuota() = %v, want an error that never shows the token", err)
	}
}

func TestAnOpenBreakerIsReportedWithoutAnotherCall(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, func(e exchange) (int, string) {
		return http.StatusInternalServerError, `{"error":{"message":"down"}}`
	})
	client.cb = circuitbreaker.New(circuitbreaker.Config{Name: "meta_social", MaxFailures: 1, ResetTimeout: time.Hour, HalfOpenMaxReqs: 1})

	_, _, _ = client.InstagramQuota(context.Background())
	_, _, err := client.InstagramQuota(context.Background())
	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Fatalf("second call = %v, want circuitbreaker.ErrOpen", err)
	}
	if n := len(fake.calls()); n != 1 {
		t.Fatalf("server calls = %d, want 1: an open breaker must not reach Meta", n)
	}
}
