package social

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
)

const sixtyDays = 60 * 24 * time.Hour

type fakeThreadsAPI struct {
	userID, username, longToken, refreshToken string
	exchangeErr, longErr, refreshErr, meErr   error
	exchangedCode, refreshedWith              string
}

func (f *fakeThreadsAPI) AuthorizeURL(state string) string {
	return "https://threads.net/oauth/authorize?state=" + url.QueryEscape(state)
}

func (f *fakeThreadsAPI) Exchange(_ context.Context, code string) (ThreadsToken, error) {
	f.exchangedCode = code
	if f.exchangeErr != nil {
		return ThreadsToken{}, f.exchangeErr
	}
	return ThreadsToken{AccessToken: "short-1", UserID: f.userID}, nil
}

func (f *fakeThreadsAPI) LongLived(context.Context, string) (ThreadsToken, error) {
	if f.longErr != nil {
		return ThreadsToken{}, f.longErr
	}
	return ThreadsToken{AccessToken: f.longToken, ExpiresIn: sixtyDays}, nil
}

func (f *fakeThreadsAPI) Refresh(_ context.Context, token string) (ThreadsToken, error) {
	f.refreshedWith = token
	if f.refreshErr != nil {
		return ThreadsToken{}, f.refreshErr
	}
	return ThreadsToken{AccessToken: f.refreshToken, ExpiresIn: sixtyDays}, nil
}

func (f *fakeThreadsAPI) Me(context.Context, string) (string, string, error) {
	if f.meErr != nil {
		return "", "", f.meErr
	}
	return f.userID, f.username, nil
}

type fakeThreadsStore struct {
	conn  ThreadsConnection
	ok    bool
	saves int
}

func (s *fakeThreadsStore) LoadThreads(context.Context) (ThreadsConnection, bool, error) {
	return s.conn, s.ok, nil
}

func (s *fakeThreadsStore) SaveThreads(_ context.Context, c ThreadsConnection) error {
	s.saves++
	s.conn = c
	s.ok = true
	return nil
}

func newThreadsAuth(configured bool, api *fakeThreadsAPI, store *fakeThreadsStore) *ThreadsAuth {
	logger := slog.New(slog.DiscardHandler)
	return NewThreadsAuth(api, store, ThreadsAuthConfig{
		Configured:  configured,
		FrontendURL: "https://app.vibe.test",
		Now:         func() time.Time { return t0 },
	}, httpx.NewResponder(logger), logger)
}

func callbackRequest(state string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/social/threads/callback", nil)
	if state != "" {
		r.AddCookie(&http.Cookie{Name: threadsStateCookie, Value: state})
	}
	return r
}

func cookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestConnectRedirectsToConsentWithAStateCookie(t *testing.T) {
	t.Parallel()
	a := newThreadsAuth(true, &fakeThreadsAPI{}, &fakeThreadsStore{})

	w := httptest.NewRecorder()
	a.Connect(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/social/threads/connect", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d; want 302", w.Code)
	}
	cookie := cookieNamed(w, threadsStateCookie)
	if cookie == nil || cookie.Value == "" {
		t.Fatal("connect must set the state cookie")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != threadsCallbackPath {
		t.Fatalf("state cookie = %+v; want HttpOnly, SameSite=Lax, path %s", cookie, threadsCallbackPath)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	if loc.Query().Get("state") != cookie.Value {
		t.Fatalf("the consent page must carry the same state the cookie holds")
	}
}

func TestConnectNeedsTheAppConfigured(t *testing.T) {
	t.Parallel()
	a := newThreadsAuth(false, &fakeThreadsAPI{}, &fakeThreadsStore{})

	w := httptest.NewRecorder()
	a.Connect(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/social/threads/connect", nil))
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d; want 501 when THREADS_APP_ID is not set", w.Code)
	}
}

func TestCallbackStoresTheLongLivedTokenAndSaysSo(t *testing.T) {
	t.Parallel()
	api := &fakeThreadsAPI{userID: "42", username: "vibe.padel", longToken: "long-1"}
	store := &fakeThreadsStore{}
	a := newThreadsAuth(true, api, store)

	w := httptest.NewRecorder()
	a.Callback(w, callbackRequest("s1"), ThreadsCallback{Code: "abc#_", State: "s1"})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "@vibe.padel") {
		t.Fatalf("status = %d, body = %s; want 200 naming the account", w.Code, w.Body.String())
	}
	if api.exchangedCode != "abc" {
		t.Fatalf("exchanged %q; want the code without Meta's #_ suffix", api.exchangedCode)
	}
	if store.saves != 1 {
		t.Fatalf("saves = %d; want 1", store.saves)
	}
	got := store.conn
	if got.AccessToken != "long-1" || got.UserID != "42" || got.Username != "vibe.padel" {
		t.Fatalf("stored = %+v; want the long-lived token of account 42", got)
	}
	if !got.ConnectedAt.Equal(t0) || !got.ExpiresAt.Equal(t0.Add(sixtyDays)) {
		t.Fatalf("connected %v, expires %v; want now and 60 days later", got.ConnectedAt, got.ExpiresAt)
	}
	if c := cookieNamed(w, threadsStateCookie); c == nil || c.MaxAge >= 0 {
		t.Fatal("the callback must clear the state cookie")
	}
}

func TestCallbackRecordsThatTheConsentAskedForKeywordSearch(t *testing.T) {
	t.Parallel()
	api := &fakeThreadsAPI{userID: "42", username: "vibe.padel", longToken: "long-1"}
	store := &fakeThreadsStore{}
	a := newThreadsAuth(true, api, store)

	w := httptest.NewRecorder()
	a.Callback(w, callbackRequest("s1"), ThreadsCallback{Code: "abc", State: "s1"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if !store.conn.KeywordSearchRequested {
		t.Fatal("a connection made by this build asks for threads_keyword_search and must say so")
	}
}

func TestCallbackRejectsAStateThatDoesNotMatch(t *testing.T) {
	t.Parallel()
	api := &fakeThreadsAPI{userID: "42", longToken: "long-1"}
	store := &fakeThreadsStore{}
	a := newThreadsAuth(true, api, store)

	w := httptest.NewRecorder()
	a.Callback(w, callbackRequest("s1"), ThreadsCallback{Code: "abc", State: "other"})
	if w.Code != http.StatusBadRequest || store.saves != 0 || api.exchangedCode != "" {
		t.Fatalf("status = %d, saves = %d, exchanged %q; want 400 with nothing done", w.Code, store.saves, api.exchangedCode)
	}
}

func TestCallbackNeedsTheBrowserThatStartedTheConnection(t *testing.T) {
	t.Parallel()
	api := &fakeThreadsAPI{userID: "42", longToken: "long-1"}
	store := &fakeThreadsStore{}
	a := newThreadsAuth(true, api, store)

	w := httptest.NewRecorder()
	a.Callback(w, callbackRequest(""), ThreadsCallback{Code: "abc", State: "s1"})
	if w.Code != http.StatusBadRequest || store.saves != 0 {
		t.Fatalf("status = %d, saves = %d; want 400 and nothing stored", w.Code, store.saves)
	}
}

func TestCallbackStopsWhenTheOwnerDeniesAccess(t *testing.T) {
	t.Parallel()
	api := &fakeThreadsAPI{}
	store := &fakeThreadsStore{}
	a := newThreadsAuth(true, api, store)

	w := httptest.NewRecorder()
	a.Callback(w, callbackRequest("s1"), ThreadsCallback{ErrorCode: "access_denied", State: "s1"})
	if w.Code != http.StatusBadRequest || store.saves != 0 || api.exchangedCode != "" {
		t.Fatalf("status = %d; want 400 with no exchange and nothing stored", w.Code)
	}
}

func TestCallbackStoresNothingWhenMetaRefuses(t *testing.T) {
	t.Parallel()
	api := &fakeThreadsAPI{exchangeErr: errors.New("meta answered 400")}
	store := &fakeThreadsStore{}
	a := newThreadsAuth(true, api, store)

	w := httptest.NewRecorder()
	a.Callback(w, callbackRequest("s1"), ThreadsCallback{Code: "abc", State: "s1"})
	if w.Code != http.StatusBadGateway || store.saves != 0 {
		t.Fatalf("status = %d, saves = %d; want 502 and nothing stored", w.Code, store.saves)
	}
}
