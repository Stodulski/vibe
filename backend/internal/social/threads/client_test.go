package threads

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/social"
)

const secret = "app-secret-do-not-leak"

// fakeStore holds one Threads connection.
type fakeStore struct {
	conn social.ThreadsConnection
	ok   bool
}

func (s *fakeStore) LoadThreads(context.Context) (social.ThreadsConnection, bool, error) {
	return s.conn, s.ok, nil
}

func (s *fakeStore) SaveThreads(_ context.Context, c social.ThreadsConnection) error {
	s.conn = c
	s.ok = true
	return nil
}

func connected() *fakeStore {
	return &fakeStore{ok: true, conn: social.ThreadsConnection{UserID: "42", AccessToken: "tok-secret"}}
}

// fakeGraph serves the Threads endpoints the client uses. Containers report
// IN_PROGRESS on their first poll and FINISHED after that, unless the test
// marks them as errors.
type fakeGraph struct {
	mu         sync.Mutex
	calls      []call
	polls      map[string]int
	errorIDs   map[string]bool
	apiErr     string
	publishID  string
	searchBody string
}

type call struct {
	method string
	path   string
	query  url.Values
	form   url.Values
}

func (g *fakeGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	form, _ := url.ParseQuery(string(body))
	g.calls = append(g.calls, call{method: r.Method, path: r.URL.Path, query: r.URL.Query(), form: form})
	w.Header().Set("Content-Type", "application/json")

	if g.apiErr != "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(g.apiErr))
		return
	}
	switch {
	case r.URL.Path == "/oauth/access_token":
		_, _ = w.Write([]byte(`{"access_token":"short-1","user_id":42}`))
	case r.URL.Path == "/access_token":
		_, _ = w.Write([]byte(`{"access_token":"long-1","token_type":"bearer","expires_in":5184000}`))
	case r.URL.Path == "/refresh_access_token":
		_, _ = w.Write([]byte(`{"access_token":"long-2","token_type":"bearer","expires_in":5184000}`))
	case r.URL.Path == "/v1.0/me":
		_, _ = w.Write([]byte(`{"id":"42","username":"vibe.padel"}`))
	case strings.HasSuffix(r.URL.Path, "/threads_publish"):
		_, _ = w.Write([]byte(`{"id":"` + g.publishID + `"}`))
	case strings.HasSuffix(r.URL.Path, "/threads") && r.Method == http.MethodPost:
		id := "c" + strconv.Itoa(len(g.calls))
		_, _ = w.Write([]byte(`{"id":"` + id + `"}`))
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1.0/c"):
		id := strings.TrimPrefix(r.URL.Path, "/v1.0/")
		if g.errorIDs[id] {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ERROR", "error_message": "media could not be processed"})
			return
		}
		g.polls[id]++
		status := "IN_PROGRESS"
		if g.polls[id] > 1 {
			status = "FINISHED"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	case r.URL.Path == "/v1.0/keyword_search":
		_, _ = w.Write([]byte(g.searchBody))
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1.0/m"):
		_, _ = w.Write([]byte(`{"permalink":"https://www.threads.net/@vibe.padel/post/abc"}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newClient(t *testing.T, g *fakeGraph, store *fakeStore) *Client {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	return New(Config{
		AppID:       "app-1",
		AppSecret:   secret,
		RedirectURI: "https://api.vibe.test/api/v1/social/threads/callback",
		Base:        srv.URL,
		PollEvery:   time.Millisecond,
		PollFor:     time.Second,
	}, store)
}

func newGraph() *fakeGraph {
	return &fakeGraph{polls: map[string]int{}, errorIDs: map[string]bool{}, publishID: "m1"}
}

func post(kind social.Kind, caption string, media ...string) social.Post {
	p := social.Post{ID: "p1", Kind: kind, Caption: caption}
	for _, m := range media {
		p.Media = append(p.Media, social.Media{Name: m, URL: "https://cdn.test/" + m})
	}
	return p
}

func TestAuthorizeURLNamesTheScopesAndTheRedirect(t *testing.T) {
	t.Parallel()
	c := New(Config{AppID: "app-1", RedirectURI: "https://api.vibe.test/cb"}, connected())
	u, err := url.Parse(c.AuthorizeURL("state-1"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	if u.Host != "threads.net" || u.Path != "/oauth/authorize" {
		t.Fatalf("authorize host and path = %s%s", u.Host, u.Path)
	}
	if q.Get("client_id") != "app-1" || q.Get("redirect_uri") != "https://api.vibe.test/cb" ||
		q.Get("scope") != "threads_basic,threads_content_publish,threads_keyword_search" || q.Get("response_type") != "code" || q.Get("state") != "state-1" {
		t.Fatalf("query = %v", q)
	}
}

func TestOAuthExchangesReturnTheTokensAndTheirLifetimes(t *testing.T) {
	t.Parallel()
	g := newGraph()
	c := newClient(t, g, connected())
	ctx := context.Background()

	short, err := c.Exchange(ctx, "code-1")
	if err != nil || short.AccessToken != "short-1" || short.UserID != "42" {
		t.Fatalf("exchange = %+v, %v", short, err)
	}
	long, err := c.LongLived(ctx, short.AccessToken)
	if err != nil || long.AccessToken != "long-1" || long.ExpiresIn != 60*24*time.Hour {
		t.Fatalf("long-lived = %+v, %v; want 60 days", long, err)
	}
	fresh, err := c.Refresh(ctx, long.AccessToken)
	if err != nil || fresh.AccessToken != "long-2" || fresh.ExpiresIn != 60*24*time.Hour {
		t.Fatalf("refresh = %+v, %v", fresh, err)
	}
	id, username, err := c.Me(ctx, fresh.AccessToken)
	if err != nil || id != "42" || username != "vibe.padel" {
		t.Fatalf("me = %s %s %v", id, username, err)
	}
	if got := g.calls[0].form.Get("grant_type"); got != "authorization_code" {
		t.Fatalf("exchange grant_type = %q", got)
	}
	if got := g.calls[1].query.Get("grant_type"); got != "th_exchange_token" {
		t.Fatalf("long-lived grant_type = %q", got)
	}
	if got := g.calls[2].query.Get("grant_type"); got != "th_refresh_token" {
		t.Fatalf("refresh grant_type = %q", got)
	}
}

func TestTextPostWaitsForTheContainerThenPublishes(t *testing.T) {
	t.Parallel()
	g := newGraph()
	c := newClient(t, g, connected())
	p := post(social.TypeText, "Solo texto")

	id, err := c.PublishThreads(context.Background(), p)
	if err != nil || id != "m1" {
		t.Fatalf("publish = %q, %v; want m1", id, err)
	}
	create := g.calls[0]
	if create.path != "/v1.0/42/threads" || create.form.Get("media_type") != "TEXT" || create.form.Get("text") != "Solo texto" {
		t.Fatalf("create = %+v; want a TEXT container for the caption", create)
	}
	if g.polls["c1"] < 2 {
		t.Fatal("the container must be polled until FINISHED, not published at once")
	}
	last := g.calls[len(g.calls)-1]
	if last.path != "/v1.0/42/threads_publish" || last.form.Get("creation_id") != "c1" {
		t.Fatalf("last call = %+v; want threads_publish of c1", last)
	}
}

func TestThreadsTextAndTopicOverrideTheCaption(t *testing.T) {
	t.Parallel()
	g := newGraph()
	c := newClient(t, g, connected())
	p := post(social.TypeImage, "Caption largo", "foto.jpg")
	p.ThreadsText = "Texto para Threads"
	p.ThreadsTopic = "padel"
	p.AltText = "Una cancha"

	if _, err := c.PublishThreads(context.Background(), p); err != nil {
		t.Fatalf("publish: %v", err)
	}
	create := g.calls[0]
	if create.form.Get("media_type") != "IMAGE" || create.form.Get("image_url") != "https://cdn.test/foto.jpg" {
		t.Fatalf("create = %+v; want an IMAGE container", create)
	}
	if create.form.Get("text") != "Texto para Threads" || create.form.Get("topic_tag") != "padel" || create.form.Get("alt_text") != "Una cancha" {
		t.Fatalf("form = %v; want threads_text, topic and alt text", create.form)
	}
}

func TestCarouselCreatesEachChildThenTheCarousel(t *testing.T) {
	t.Parallel()
	g := newGraph()
	c := newClient(t, g, connected())
	p := post(social.TypeCarousel, "Dos fotos", "1.jpg", "2.jpg")

	if _, err := c.PublishThreads(context.Background(), p); err != nil {
		t.Fatalf("publish: %v", err)
	}
	var children []call
	var parent call
	for _, cl := range g.calls {
		switch {
		case cl.form.Get("is_carousel_item") == "true":
			children = append(children, cl)
		case cl.form.Get("media_type") == "CAROUSEL":
			parent = cl
		}
	}
	if len(children) != 2 || children[0].form.Get("media_type") != "IMAGE" {
		t.Fatalf("children = %+v; want two IMAGE items", children)
	}
	if parent.form.Get("children") == "" || !strings.Contains(parent.form.Get("children"), "c") {
		t.Fatalf("parent = %+v; want a CAROUSEL naming its children", parent)
	}
}

func TestReelPublishesTheVideoAfterItFinishes(t *testing.T) {
	t.Parallel()
	g := newGraph()
	c := newClient(t, g, connected())
	p := post(social.TypeReel, "Un reel", "clip.mp4")

	if _, err := c.PublishThreads(context.Background(), p); err != nil {
		t.Fatalf("publish: %v", err)
	}
	create := g.calls[0]
	if create.form.Get("media_type") != "VIDEO" || create.form.Get("video_url") != "https://cdn.test/clip.mp4" {
		t.Fatalf("create = %+v; want a VIDEO container", create)
	}
	if g.polls["c1"] < 2 {
		t.Fatal("a video must finish processing before it is published")
	}
}

func TestAContainerThatErrorsIsNotPublished(t *testing.T) {
	t.Parallel()
	g := newGraph()
	g.errorIDs["c1"] = true
	c := newClient(t, g, connected())

	_, err := c.PublishThreads(context.Background(), post(social.TypeText, "Texto"))
	if err == nil || !strings.Contains(err.Error(), "media could not be processed") {
		t.Fatalf("err = %v; want Threads' processing error", err)
	}
	for _, cl := range g.calls {
		if strings.HasSuffix(cl.path, "threads_publish") {
			t.Fatal("a failed container must never be published")
		}
	}
}

func TestPublishNeedsAConnectedAccount(t *testing.T) {
	t.Parallel()
	g := newGraph()
	c := newClient(t, g, &fakeStore{})

	_, err := c.PublishThreads(context.Background(), post(social.TypeText, "Texto"))
	if err == nil || errors.Is(err, social.ErrPermanent) {
		t.Fatalf("err = %v; want a retryable error when Threads is not connected", err)
	}
	if len(g.calls) != 0 {
		t.Fatal("without a connection nothing may be sent")
	}
}

func TestPermalinkReadsThePublishedMedia(t *testing.T) {
	t.Parallel()
	c := newClient(t, newGraph(), connected())
	link, err := c.ThreadsPermalink(context.Background(), "m1")
	if err != nil || link != "https://www.threads.net/@vibe.padel/post/abc" {
		t.Fatalf("permalink = %q, %v", link, err)
	}
}

func TestClientErrorsArePermanentButTokenErrorsAreNot(t *testing.T) {
	t.Parallel()
	g := newGraph()
	g.apiErr = `{"error":{"message":"Invalid parameter","code":100}}`
	c := newClient(t, g, connected())
	if _, err := c.PublishThreads(context.Background(), post(social.TypeText, "x")); !errors.Is(err, social.ErrPermanent) {
		t.Fatalf("a 400 with code 100 must be permanent; got %v", err)
	}

	g.apiErr = `{"error":{"message":"Session has expired","code":190}}`
	if _, err := c.PublishThreads(context.Background(), post(social.TypeText, "x")); err == nil || errors.Is(err, social.ErrPermanent) {
		t.Fatalf("a token error must wait for a reconnect, not end the post; got %v", err)
	}
}

func TestErrorsNeverShowTheToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	c := New(Config{Base: srv.URL}, connected())

	_, err := c.Refresh(context.Background(), "tok-secret")
	if err == nil || strings.Contains(err.Error(), "tok-secret") || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v; a transport error must not carry the token or the secret", err)
	}
}

func TestKeywordSearchReadsPostsWithTheConnectedAccount(t *testing.T) {
	t.Parallel()
	g := newGraph()
	g.searchBody = `{"data":[{"id":"p1","text":"hola cancha","permalink":"https://www.threads.net/@jugador/post/p1","timestamp":"2026-10-10T18:04:05+0000","username":"jugador","has_replies":true}]}`
	store := connected()
	c := newClient(t, g, store)

	posts, err := c.KeywordSearch(context.Background(), "complejo de pádel", social.KeywordSearchRecent)
	if err != nil {
		t.Fatalf("KeywordSearch() error = %v", err)
	}
	if len(posts) != 1 || posts[0].ID != "p1" || posts[0].Username != "jugador" || !posts[0].HasReplies {
		t.Fatalf("posts = %+v; want the one post decoded", posts)
	}
	if want := time.Date(2026, 10, 10, 18, 4, 5, 0, time.UTC); !posts[0].Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v; want %v", posts[0].Timestamp, want)
	}
	call := g.calls[len(g.calls)-1]
	if call.path != "/v1.0/keyword_search" || call.query.Get("q") != "complejo de pádel" ||
		call.query.Get("search_type") != "RECENT" || call.query.Get("access_token") != "tok-secret" {
		t.Fatalf("request = %+v; want q, RECENT and the connected token", call)
	}
	if !strings.Contains(call.query.Get("fields"), "has_replies") {
		t.Fatalf("fields = %q; want has_replies", call.query.Get("fields"))
	}
}
