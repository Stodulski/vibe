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
	"unicode/utf8"

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
		case e.method == http.MethodGet && (e.path == "/v25.0/parent" || e.path == "/v25.0/child-1" || e.path == "/v25.0/child-2"):
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

func TestInstagramCarouselWaitsForEveryChildBeforeTheParent(t *testing.T) {
	t.Parallel()
	var childPolls atomic.Int32
	fake, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media" && e.params.Get("is_carousel_item") == "true":
			return http.StatusOK, `{"id":"child-1"}`
		case e.method == http.MethodGet && e.path == "/v25.0/child-1":
			if childPolls.Add(1) == 1 {
				return http.StatusOK, `{"status_code":"IN_PROGRESS"}`
			}
			return http.StatusOK, `{"status_code":"FINISHED"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			if childPolls.Load() < 2 {
				return http.StatusBadRequest, `{"error":{"message":"child not ready","code":100}}`
			}
			return http.StatusOK, `{"id":"parent"}`
		case e.method == http.MethodGet && e.path == "/v25.0/parent":
			return http.StatusOK, `{"status_code":"FINISHED"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media_publish":
			return http.StatusOK, `{"id":"m3"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	post := imagePost(social.TypeCarousel, "1.jpg", "2.jpg")
	id, err := client.PublishInstagram(context.Background(), post)
	if err != nil || id != "m3" {
		t.Fatalf("PublishInstagram() = %q, %v; want m3 after the children finish", id, err)
	}
	if got := childPolls.Load(); got < 2 {
		t.Fatalf("child polls = %d, want the child polled until FINISHED", got)
	}
	var parentAt, lastChildPoll int
	for i, c := range fake.calls() {
		if c.params.Get("media_type") == "CAROUSEL" {
			parentAt = i
		}
		if c.path == "/v25.0/child-1" {
			lastChildPoll = i
		}
	}
	if parentAt < lastChildPoll {
		t.Fatalf("parent created at call %d before the last child poll at %d", parentAt, lastChildPoll)
	}
}

func TestInstagramPublishRetriesWhileTheMediaIsNotReady(t *testing.T) {
	t.Parallel()
	var publishes atomic.Int32
	_, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			return http.StatusOK, `{"id":"c1"}`
		case e.method == http.MethodGet && e.path == "/v25.0/c1":
			return http.StatusOK, `{"status_code":"FINISHED"}`
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media_publish":
			if publishes.Add(1) < 3 {
				return http.StatusBadRequest, `{"error":{"message":"Media ID is not available","code":9007,"error_subcode":2207027}}`
			}
			return http.StatusOK, `{"id":"m4"}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	id, err := client.PublishInstagram(context.Background(), imagePost(social.TypeImage, "1.jpg"))
	if err != nil || id != "m4" {
		t.Fatalf("PublishInstagram() = %q, %v; want m4 on the third media_publish", id, err)
	}
	if got := publishes.Load(); got != 3 {
		t.Fatalf("media_publish calls = %d, want 3", got)
	}
}

func TestMediaNotReadyIsRetryableWhenItOutlastsTheWait(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if code, body, ok := standardAccount(e); ok {
			return code, body
		}
		switch {
		case e.method == http.MethodPost && e.path == "/v25.0/ig-1/media":
			return http.StatusOK, `{"id":"c1"}`
		case e.method == http.MethodGet && e.path == "/v25.0/c1":
			return http.StatusOK, `{"status_code":"FINISHED"}`
		}
		return http.StatusBadRequest, `{"error":{"message":"Media ID is not available","code":9007,"error_subcode":2207027}}`
	})

	_, err := client.PublishInstagram(context.Background(), imagePost(social.TypeImage, "1.jpg"))
	if err == nil || errors.Is(err, social.ErrPermanent) {
		t.Fatalf("PublishInstagram() = %v, want a retryable not-ready error", err)
	}
}

// A non-Graph body keeps its text in the message, so the failure can be read.
func TestNonGraphErrorBodyIsKeptInTheMessage(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		return http.StatusUnprocessableEntity, "{\"unexpected\":\n  \"shape\"}"
	})

	_, err := client.account(context.Background())
	want := `meta answered 422 (code 0): Unprocessable Entity: body {"unexpected": "shape"}`
	if err == nil || !errors.Is(err, social.ErrPermanent) || !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("account() = %v, want a permanent refusal ending %q", err, want)
	}
}

// A long body is cut to at most 300 bytes, on a rune boundary, with an ellipsis.
func TestLongNonGraphBodyIsCutToAUTF8Snippet(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		return http.StatusUnprocessableEntity, strings.Repeat("ñ", 400)
	})

	_, err := client.account(context.Background())
	const prefix = "meta answered 422 (code 0): Unprocessable Entity: body "
	if err == nil {
		t.Fatal("account() = nil, want an error")
	}
	i := strings.Index(err.Error(), prefix)
	if i < 0 {
		t.Fatalf("account() = %v, want the body snippet after %q", err, prefix)
	}
	snippet := err.Error()[i+len(prefix):]
	if len(snippet) > 300 || !utf8.ValidString(snippet) || !strings.HasSuffix(snippet, "…") {
		t.Fatalf("snippet = %q (%d bytes), want at most 300 valid UTF-8 bytes ending in an ellipsis", snippet, len(snippet))
	}
	if kept := strings.TrimSuffix(snippet, "…"); strings.Trim(kept, "ñ") != "" || kept == "" {
		t.Fatalf("snippet head = %q, want only the start of the body", kept)
	}
}

// A rupload answer carries its reason in debug_info, which becomes the message.
func TestRuploadDebugInfoBecomesTheMessage(t *testing.T) {
	t.Parallel()
	body := `{"debug_info":{"retriable":false,"type":"OAuthException","message":"Invalid upload session"}}`
	_, client := newFake(t, func(e exchange) (int, string) {
		return http.StatusUnprocessableEntity, body
	})

	_, err := client.account(context.Background())
	want := "meta answered 422 (code 0): OAuthException: Invalid upload session: body " + body
	if err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("account() = %v, want it to end %q", err, want)
	}
}

// Each step of a reel and of a video story names itself in its error, and a
// refused step stays permanent.
func TestEachPublishStepNamesItselfInItsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		step string
		kind social.Kind
		fail string
	}{
		{"reel start", social.TypeReel, "reel start"},
		{"reel upload", social.TypeReel, "reel upload"},
		{"reel finish", social.TypeReel, "reel finish"},
		{"video story start", social.TypeStory, "video story start"},
		{"video story upload", social.TypeStory, "video story upload"},
		{"video story finish", social.TypeStory, "video story finish"},
	}
	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			t.Parallel()
			_, client := newFake(t, func(e exchange) (int, string) {
				if code, body, ok := standardAccount(e); ok {
					return code, body
				}
				refuse := func(step string) (int, string) {
					if tc.fail == step {
						return http.StatusUnprocessableEntity, `{"debug_info":{"message":"refused"}}`
					}
					return 0, ""
				}
				switch {
				case e.path == "/v25.0/page-1/video_reels" && e.params.Get("upload_phase") == "start":
					if code, body := refuse("reel start"); code != 0 {
						return code, body
					}
					return http.StatusOK, `{"video_id":"vid-9"}`
				case e.path == "/upload/v25.0/vid-9":
					if code, body := refuse("reel upload"); code != 0 {
						return code, body
					}
					if code, body := refuse("video story upload"); code != 0 {
						return code, body
					}
					return http.StatusOK, `{"success":true}`
				case e.path == "/v25.0/page-1/video_reels" && e.params.Get("upload_phase") == "finish":
					if code, body := refuse("reel finish"); code != 0 {
						return code, body
					}
					return http.StatusOK, `{"success":true}`
				case e.path == "/v25.0/page-1/video_stories" && e.params.Get("upload_phase") == "start":
					if code, body := refuse("video story start"); code != 0 {
						return code, body
					}
					return http.StatusOK, `{"video_id":"vid-9"}`
				case e.path == "/v25.0/page-1/video_stories" && e.params.Get("upload_phase") == "finish":
					if code, body := refuse("video story finish"); code != 0 {
						return code, body
					}
					return http.StatusOK, `{"success":true,"post_id":"page-1_99"}`
				}
				return http.StatusNotFound, `{"error":{"message":"no route"}}`
			})

			_, err := client.PublishFacebook(context.Background(), imagePost(tc.kind, "clip.mp4"))
			if err == nil || !strings.HasPrefix(err.Error(), tc.step+": ") {
				t.Fatalf("PublishFacebook() = %v, want an error starting with %q", err, tc.step+": ")
			}
			if !errors.Is(err, social.ErrPermanent) {
				t.Fatalf("PublishFacebook() = %v, want the wrapped refusal to stay ErrPermanent", err)
			}
		})
	}
}

// A start answer without a video id stops before any upload is attempted.
func TestMissingVideoIDStopsBeforeTheUpload(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind social.Kind
		want string
	}{
		{social.TypeReel, "reel start: meta returned no video id"},
		{social.TypeStory, "video story start: meta returned no video id"},
	}
	for _, tc := range cases {
		fake, client := newFake(t, func(e exchange) (int, string) {
			if code, body, ok := standardAccount(e); ok {
				return code, body
			}
			return http.StatusOK, `{"success":true}`
		})

		_, err := client.PublishFacebook(context.Background(), imagePost(tc.kind, "clip.mp4"))
		if err == nil || err.Error() != tc.want {
			t.Fatalf("PublishFacebook(%s) = %v, want %q", tc.kind, err, tc.want)
		}
		if errors.Is(err, social.ErrPermanent) {
			t.Fatalf("PublishFacebook(%s) = %v, want a retryable error like the other not-accepted answers", tc.kind, err)
		}
		for _, c := range fake.calls() {
			if strings.HasPrefix(c.path, "/upload/") {
				t.Fatalf("upload was attempted without a video id: %v", c)
			}
		}
	}
}

// The access token is never in the message: body text is redacted, and only the
// body is ever included, never the request.
func TestTheAccessTokenIsRedactedFromTheMessage(t *testing.T) {
	t.Parallel()
	body := `{"debug_info":{"type":"OAuthException","message":"rejected access_token=XYZ"},` +
		`"echo":"access_token=XYZ&x=1","json":{"access_token":"JSONVAL"}}`
	_, client := newFake(t, func(e exchange) (int, string) {
		return http.StatusUnprocessableEntity, body
	})

	_, err := client.account(context.Background())
	if err == nil {
		t.Fatal("account() = nil, want an error")
	}
	msg := err.Error()
	for _, secret := range []string{"XYZ", "JSONVAL", systemToken, pageToken} {
		if strings.Contains(msg, secret) {
			t.Fatalf("error %q leaks %q", msg, secret)
		}
	}
	for _, want := range []string{"rejected access_token=[redacted]", `"echo":"access_token=[redacted]&x=1"`, `"json":{"access_token":"[redacted]"}`} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q, want it to contain %q", msg, want)
		}
	}
}

// A Graph error keeps its exact message: the new body text never replaces it.
func TestGraphErrorMessageIsUnchanged(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		return http.StatusBadRequest, `{"error":{"message":"Invalid image","code":100}}`
	})

	_, err := client.account(context.Background())
	const want = "social: permanent platform refusal: meta answered 400 (code 100): Invalid image"
	if err == nil || err.Error() != want {
		t.Fatalf("account() = %v, want exactly %q", err, want)
	}
}

// assertTokenHidden fails when err is nil, when it still holds the token in
// any form, or when it lacks the marker that replaced the token.
func assertTokenHidden(t *testing.T, err error, token string) {
	t.Helper()
	if err == nil {
		t.Fatal("want an error")
	}
	for _, form := range []string{token, url.QueryEscape(token)} {
		if strings.Contains(err.Error(), form) {
			t.Fatalf("error %q leaks the token form %q", err, form)
		}
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("error %q, want the token replaced by [redacted]", err)
	}
}

// The request's token never reaches an error, whatever form the answer echoes it
// in: raw, query-escaped, in JSON, in escaped JSON, or in a Graph message.
func TestThePageTokenOfTheRequestIsRedactedInAnyForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		token  string
		status int
		body   string
	}{
		{"json member", "EAABpage-secret", http.StatusUnprocessableEntity, `{"token":"EAABpage-secret"}`},
		{"message text", "EAABpage-secret", http.StatusUnprocessableEntity, `{"message":"page token EAABpage-secret echoed"}`},
		{"query escaped pair", "EAABpage-secret", http.StatusUnprocessableEntity, `access_token%3DEAABpage-secret&x=1`},
		{"escaped json", "EAABpage-secret", http.StatusUnprocessableEntity, `{"x":"{\"access_token\":\"EAABpage-secret\"}"}`},
		{"graph message", "EAABpage-secret", http.StatusBadRequest, `{"error":{"message":"bad EAABpage-secret","code":190}}`},
		{"query escaped token", "EAAB|p+1", http.StatusUnprocessableEntity, `{"echo":"EAAB%7Cp%2B1"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, client := newFake(t, func(e exchange) (int, string) {
				if e.path == "/v25.0/me/accounts" {
					return http.StatusOK, `{"data":[{"id":"page-1","access_token":"` + tc.token + `","instagram_business_account":{"id":"ig-1"}}]}`
				}
				return tc.status, tc.body
			})
			_, err := client.PublishFacebook(context.Background(), imagePost(social.TypeImage, "1.jpg"))
			assertTokenHidden(t, err, tc.token)
		})
	}
}

// The upload step sends the token in its Authorization header, not in the query.
// Its answer is redacted just the same.
func TestTheTokenIsRedactedOnTheUploadStep(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if e.path == "/v25.0/me/accounts" {
			return http.StatusOK, `{"data":[{"id":"page-1","access_token":"EAABpage-secret","instagram_business_account":{"id":"ig-1"}}]}`
		}
		switch {
		case e.path == "/v25.0/page-1/video_reels":
			return http.StatusOK, `{"video_id":"vid-9"}`
		case e.path == "/upload/v25.0/vid-9":
			return http.StatusUnprocessableEntity, `{"debug_info":{"type":"OAuthException","message":"EAABpage-secret rejected"}}`
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	})

	_, err := client.PublishFacebook(context.Background(), imagePost(social.TypeReel, "clip.mp4"))
	assertTokenHidden(t, err, "EAABpage-secret")
	if err != nil && !strings.HasPrefix(err.Error(), "reel upload: ") {
		t.Fatalf("PublishFacebook() = %v, want the upload step named", err)
	}
}

// Every token the client holds is redacted, not only the request's own. A page
// token echoed on a system-token call, and a system token echoed on a page call,
// never reach an error.
func TestEveryHeldTokenIsRedactedFromAnyRequest(t *testing.T) {
	t.Parallel()
	quota := func(c *Client) error {
		_, _, err := c.InstagramQuota(context.Background())
		return err
	}
	photo := func(c *Client) error {
		_, err := c.PublishFacebook(context.Background(), imagePost(social.TypeImage, "1.jpg"))
		return err
	}
	cases := []struct {
		name  string
		token string
		body  string
		call  func(*Client) error
	}{
		{"page token on a system call as json", pageToken, `{"token":"page-token-value"}`, quota},
		{"page token on a system call as text", pageToken, `Bad token page-token-value here`, quota},
		{"system token on a page call as json", systemToken, `{"token":"system-token-value"}`, photo},
		{"system token on a page call as text", systemToken, `Bad token system-token-value here`, photo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, client := newFake(t, func(e exchange) (int, string) {
				if code, body, ok := standardAccount(e); ok {
					return code, body
				}
				return http.StatusUnprocessableEntity, tc.body
			})
			assertTokenHidden(t, tc.call(client), tc.token)
		})
	}
}
