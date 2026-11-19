package complexes

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/instagram"
)

const (
	igShortToken = "IGQVJshort-secret-token"
	igLongToken  = "IGQVJlong-secret-token"
	igFreshToken = "IGQVJrefreshed-secret-token"
	igAppSecret  = "app-secret-value"
)

// fakeInstagram is an httptest stand-in for Instagram's four endpoints.
type fakeInstagram struct {
	srv       *httptest.Server
	exchanges atomic.Int32
	refreshes atomic.Int32
	lastCode  atomic.Value
	// meStatus, when set, is the status /me answers with.
	meStatus int
	// meID is the app-scoped id /me reports; it defaults to the exchanged one.
	meID string
	// refreshFails names the tokens whose refresh fails, with the status Meta
	// answers: 400 is a refusal (the token was revoked), 500 an outage.
	refreshFails map[string]int
	// refreshCodes is the Graph code a failing refresh carries (190 when unset).
	refreshCodes map[string]int
}

func newFakeInstagram(t *testing.T) *fakeInstagram {
	t.Helper()
	f := &fakeInstagram{meID: "1020", refreshFails: map[string]int{}, refreshCodes: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		f.exchanges.Add(1)
		_ = r.ParseForm()
		f.lastCode.Store(r.PostForm.Get("code"))
		if r.PostForm.Get("client_secret") != igAppSecret || r.PostForm.Get("grant_type") != "authorization_code" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error_type":"OAuthException","code":400,"error_message":"bad request"}`))
			return
		}
		// The shape the documentation shows: wrapped in "data", user_id a string.
		_, _ = w.Write([]byte(`{"data":[{"access_token":"` + igShortToken + `","user_id":"1020","permissions":"instagram_business_basic"}]}`))
	})
	mux.HandleFunc("GET /access_token", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("grant_type") != "ig_exchange_token" || r.URL.Query().Get("access_token") != igShortToken {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","code":190}}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"` + igLongToken + `","token_type":"bearer","expires_in":5183944}`))
	})
	mux.HandleFunc("GET /refresh_access_token", func(w http.ResponseWriter, r *http.Request) {
		f.refreshes.Add(1)
		tok := r.URL.Query().Get("access_token")
		if status := f.refreshFails[tok]; r.URL.Query().Get("grant_type") != "ig_refresh_token" || status != 0 {
			if status == 0 {
				status = http.StatusBadRequest
			}
			code := f.refreshCodes[tok]
			if code == 0 {
				code = instagram.CodeInvalidToken
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"Error validating access token","code":` + strconv.Itoa(code) + `}}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"` + igFreshToken + `-` + tok + `","token_type":"bearer","expires_in":5183944}`))
	})
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		if f.meStatus != 0 {
			w.WriteHeader(f.meStatus)
			return
		}
		if r.URL.Query().Get("access_token") != igLongToken {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Flat, user_id a number: the other shape seen in the wild.
		_, _ = w.Write([]byte(`{"id":"` + f.meID + `","user_id":17841400000000001,"username":"club.padel","account_type":"Business"}`))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// memIGStore is an in-memory InstagramStore. Which rows are due is the
// store's SQL and is tested against Postgres; here every listed row is due.
type memIGStore struct {
	mu     sync.Mutex
	conns  map[uuid.UUID]*complexstore.InstagramConnection
	owners map[uuid.UUID]uuid.UUID
	marked int
	// raceOnUpdate makes every token update lose the race, as when a reconnect
	// stored a newer token first.
	raceOnUpdate bool
}

func newMemIGStore() *memIGStore {
	return &memIGStore{conns: map[uuid.UUID]*complexstore.InstagramConnection{}, owners: map[uuid.UUID]uuid.UUID{}}
}

func (s *memIGStore) SaveInstagramConnection(_ context.Context, c *complexstore.InstagramConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *c
	s.conns[c.ComplexID] = &cp
	return nil
}

func (s *memIGStore) GetInstagramConnection(_ context.Context, id uuid.UUID) (*complexstore.InstagramConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.conns[id]
	if !ok {
		return nil, data.ErrRecordNotFound
	}
	cp := *c
	return &cp, nil
}

func (s *memIGStore) DeleteInstagramConnection(_ context.Context, id uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.conns[id]
	delete(s.conns, id)
	return ok, nil
}

func (s *memIGStore) ListInstagramTokensDueForRefresh(context.Context, time.Time, time.Time, time.Time, int) ([]complexstore.DueInstagramToken, []uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var due []complexstore.DueInstagramToken
	for id, c := range s.conns {
		due = append(due, complexstore.DueInstagramToken{ComplexID: id, AccessToken: c.AccessToken, ExpiresAt: c.ExpiresAt})
	}
	return due, nil, nil
}

func (s *memIGStore) UpdateInstagramToken(_ context.Context, id uuid.UUID, old time.Time, token string, expires, refreshed time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.raceOnUpdate {
		return false, nil
	}
	c, ok := s.conns[id]
	if !ok || !c.ExpiresAt.Equal(old) {
		return false, nil
	}
	c.AccessToken, c.ExpiresAt, c.RefreshedAt = token, expires, refreshed
	return true, nil
}

func (s *memIGStore) MarkInstagramNeedsReconnect(_ context.Context, id uuid.UUID, old time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.conns[id]
	if !ok || !c.ExpiresAt.Equal(old) || c.NeedsReconnect {
		return false, nil
	}
	c.NeedsReconnect = true
	return true, nil
}

func (s *memIGStore) InstagramComplexOwner(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, ok := s.owners[id]
	if !ok {
		return uuid.Nil, data.ErrRecordNotFound
	}
	return owner, nil
}

func (s *memIGStore) MarkExpiredInstagramConnections(context.Context, time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marked++
	return 0, nil
}

type igHarness struct {
	ig      *Instagram
	meta    *fakeInstagram
	store   *memIGStore
	audit   *stubRecorder
	logs    *bytes.Buffer
	owner   uuid.UUID
	complex *complexstore.Complex
}

func newIGHarness(t *testing.T, available bool) *igHarness {
	t.Helper()
	meta := newFakeInstagram(t)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := &igHarness{
		meta: meta, store: newMemIGStore(), audit: &stubRecorder{}, logs: logs,
		owner: uuid.New(), complex: &complexstore.Complex{ID: uuid.New()},
	}
	client := instagram.New(instagram.Config{
		AppID: "990602627938098", AppSecret: igAppSecret, RedirectURI: "https://api.vibe.test" + InstagramCallbackPath,
		AuthorizeBase: meta.srv.URL, APIBase: meta.srv.URL, GraphBase: meta.srv.URL,
	})
	h.ig = NewInstagram(InstagramDeps{
		API: client, Store: h.store, Attempts: NewInstagramOAuthAttempts(nil, "test"),
		Audit: h.audit, Respond: httpx.NewResponder(logger), Logger: logger,
	}, InstagramConfig{Available: available, Secure: true, FrontendURL: "https://app.vibe.test"})
	h.store.owners[h.complex.ID] = h.owner
	return h
}

// connect runs POST .../instagram/connect as the owner and returns the state
// and the cookie the browser keeps.
func (h *igHarness) connect(t *testing.T) (state string, cookie *http.Cookie) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ig.Connect(w, ownerRequest(t, http.MethodPost, "/api/v1/complexes/x/instagram/connect", h.owner, h.complex, nil, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("connect = %d %s", w.Code, w.Body.String())
	}
	var body struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("connect body: %v", err)
	}
	u, err := url.Parse(body.AuthorizeURL)
	if err != nil {
		t.Fatalf("authorize_url: %v", err)
	}
	state = u.Query().Get("state")
	for _, c := range w.Result().Cookies() {
		if c.Name == instagramStateCookie {
			cookie = c
		}
	}
	if state == "" || cookie == nil || cookie.Value != state {
		t.Fatalf("connect must answer a state and set it as the cookie; state %q cookie %+v", state, cookie)
	}
	return state, cookie
}

// callback runs Meta's redirect and returns where it sent the browser.
func (h *igHarness) callback(t *testing.T, in InstagramCallback, cookie *http.Cookie) *url.URL {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, InstagramCallbackPath, nil)
	if cookie != nil {
		r.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	w := httptest.NewRecorder()
	h.ig.Callback(w, r, in)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("callback = %d, want 303", w.Code)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location: %v", err)
	}
	return loc
}

func outcome(loc *url.URL) string {
	if loc.Query().Get("instagram") == "connected" {
		return "connected"
	}
	return "error:" + loc.Query().Get("reason")
}

func (h *igHarness) assertNoTokenInLogs(t *testing.T) {
	t.Helper()
	for _, secret := range []string{igShortToken, igLongToken, igFreshToken, igAppSecret} {
		if strings.Contains(h.logs.String(), secret) {
			t.Fatalf("a secret reached the logs: %q\n%s", secret, h.logs.String())
		}
	}
}

func TestInstagramConnectStoresTheAccountForTheComplexThatStartedIt(t *testing.T) {
	h := newIGHarness(t, true)
	state, cookie := h.connect(t)

	loc := h.callback(t, InstagramCallback{Code: "AQBx-code#_", State: state}, cookie)
	if got := outcome(loc); got != "connected" {
		t.Fatalf("callback outcome = %s, want connected", got)
	}
	if loc.Path != "/settings" || loc.Query().Get("tab") != "instagram" {
		t.Fatalf("callback redirected to %s, want the panel's settings instagram tab", loc)
	}
	if got := h.meta.lastCode.Load(); got != "AQBx-code" {
		t.Fatalf("exchanged code %q, want the code without Meta's #_ suffix", got)
	}

	conn, err := h.store.GetInstagramConnection(t.Context(), h.complex.ID)
	if err != nil {
		t.Fatalf("no connection stored for the complex that started the flow: %v", err)
	}
	if conn.AccessToken != igLongToken {
		t.Fatalf("stored token %q, want the long-lived one", conn.AccessToken)
	}
	if conn.InstagramUserID != "17841400000000001" || conn.Username != "club.padel" || conn.AccountType != "BUSINESS" {
		t.Fatalf("stored account %+v, want the professional account /me reported", conn)
	}
	if d := time.Until(conn.ExpiresAt); d < 59*24*time.Hour || d > 61*24*time.Hour {
		t.Fatalf("expires in %v, want about 60 days", d)
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].Action != "instagram_connect" ||
		*h.audit.entries[0].UserID != h.owner || *h.audit.entries[0].ComplexID != h.complex.ID {
		t.Fatalf("audit = %+v, want one instagram_connect by the owner for the complex", h.audit.entries)
	}
	auditJSON, _ := json.Marshal(h.audit.entries)
	if strings.Contains(string(auditJSON), igLongToken) {
		t.Fatalf("the token reached the audit trail")
	}

	// The status the panel reads says connected and carries no token.
	w := httptest.NewRecorder()
	h.ig.Status(w, ownerRequest(t, http.MethodGet, "/", h.owner, h.complex, nil, ""))
	body := decode(t, w)
	if body["enabled"] != true || body["connected"] != true || body["username"] != "club.padel" {
		t.Fatalf("status = %v", body)
	}
	if strings.Contains(w.Body.String(), "secret-token") {
		t.Fatalf("status leaked a token: %s", w.Body.String())
	}
	h.assertNoTokenInLogs(t)
}

func TestInstagramCallbackRefusesAStateThatIsNotThisBrowsersOrIsSpent(t *testing.T) {
	t.Run("another browser's cookie", func(t *testing.T) {
		h := newIGHarness(t, true)
		state, cookie := h.connect(t)
		_, otherCookie := h.connect(t)
		_ = otherCookie
		forged := &http.Cookie{Name: cookie.Name, Value: "someone-elses-state"}
		if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: state}, forged)); got != "error:expired" {
			t.Fatalf("outcome = %s, want error:expired", got)
		}
		if h.meta.exchanges.Load() != 0 || len(h.store.conns) != 0 {
			t.Fatalf("a mismatched cookie still exchanged (%d) or stored (%d)", h.meta.exchanges.Load(), len(h.store.conns))
		}
		// The refusal did not burn the owner's own attempt.
		if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: state}, cookie)); got != "connected" {
			t.Fatalf("the owner's own browser then got %s, want connected", got)
		}
	})

	t.Run("no cookie", func(t *testing.T) {
		h := newIGHarness(t, true)
		state, _ := h.connect(t)
		if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: state}, nil)); got != "error:expired" {
			t.Fatalf("outcome = %s, want error:expired", got)
		}
		if len(h.store.conns) != 0 {
			t.Fatal("stored a connection without the state cookie")
		}
	})

	t.Run("replayed", func(t *testing.T) {
		h := newIGHarness(t, true)
		state, cookie := h.connect(t)
		if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: state}, cookie)); got != "connected" {
			t.Fatalf("first callback = %s", got)
		}
		h.store.conns = map[uuid.UUID]*complexstore.InstagramConnection{}
		if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: state}, cookie)); got != "error:expired" {
			t.Fatalf("replayed callback = %s, want error:expired", got)
		}
		if h.meta.exchanges.Load() != 1 || len(h.store.conns) != 0 {
			t.Fatalf("a replayed state exchanged again (%d exchanges) or stored", h.meta.exchanges.Load())
		}
	})

	t.Run("unknown state in the right shape", func(t *testing.T) {
		h := newIGHarness(t, true)
		h.connect(t)
		forged := h.complex.ID.String() + ".not-a-registered-random-part"
		cookie := &http.Cookie{Name: instagramStateCookie, Value: forged}
		if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: forged}, cookie)); got != "error:expired" {
			t.Fatalf("outcome = %s, want error:expired", got)
		}
		if h.meta.exchanges.Load() != 0 {
			t.Fatal("an unknown state reached the code exchange")
		}
	})

	t.Run("attempt moved to another complex", func(t *testing.T) {
		h := newIGHarness(t, true)
		state, _ := h.connect(t)
		_, random, _ := strings.Cut(state, ".")
		other := uuid.New()
		moved := other.String() + "." + random
		cookie := &http.Cookie{Name: instagramStateCookie, Value: moved}
		if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: moved}, cookie)); got != "error:expired" {
			t.Fatalf("outcome = %s, want error:expired", got)
		}
		if _, err := h.store.GetInstagramConnection(t.Context(), other); err == nil {
			t.Fatal("a connection was stored for a complex that never started one")
		}
	})
}

func TestInstagramDeniedConsentSpendsTheAttemptAndStoresNothing(t *testing.T) {
	h := newIGHarness(t, true)
	state, cookie := h.connect(t)
	loc := h.callback(t, InstagramCallback{State: state, Error: "access_denied"}, cookie)
	if got := outcome(loc); got != "error:denied" {
		t.Fatalf("outcome = %s, want error:denied", got)
	}
	if h.meta.exchanges.Load() != 0 || len(h.store.conns) != 0 || len(h.audit.entries) != 0 {
		t.Fatal("a denied consent exchanged, stored or audited something")
	}
	if got := outcome(h.callback(t, InstagramCallback{Code: "late", State: state}, cookie)); got != "error:expired" {
		t.Fatalf("the attempt is still usable after the denial: %s", got)
	}
}

func TestInstagramMetaFailuresStoreNothingAndLeakNoToken(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(*fakeInstagram)
		outcome string
	}{
		{"meta down", func(f *fakeInstagram) { f.meStatus = http.StatusInternalServerError }, "error:unavailable"},
		{"meta refuses", func(f *fakeInstagram) { f.meStatus = http.StatusBadRequest }, "error:failed"},
		{"token for another account", func(f *fakeInstagram) { f.meID = "9999" }, "error:failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newIGHarness(t, true)
			tc.setup(h.meta)
			state, cookie := h.connect(t)
			if got := outcome(h.callback(t, InstagramCallback{Code: "c", State: state}, cookie)); got != tc.outcome {
				t.Fatalf("outcome = %s, want %s", got, tc.outcome)
			}
			if len(h.store.conns) != 0 || len(h.audit.entries) != 0 {
				t.Fatal("a failed connection stored or audited something")
			}
			h.assertNoTokenInLogs(t)
		})
	}
}

func TestInstagramFeatureOffRefusesConnectAndSaysDisabled(t *testing.T) {
	h := newIGHarness(t, false)

	w := httptest.NewRecorder()
	h.ig.Connect(w, ownerRequest(t, http.MethodPost, "/", h.owner, h.complex, nil, ""))
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("connect with the feature off = %d, want 501", w.Code)
	}
	if len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "authorize") {
		t.Fatalf("a refused connect still started a flow: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	h.ig.Status(w, ownerRequest(t, http.MethodGet, "/", h.owner, h.complex, nil, ""))
	if body := decode(t, w); body["enabled"] != false {
		t.Fatalf("status with the feature off = %v, want enabled:false", body)
	}

	// A flow started before the flag was turned off does not finish.
	loc := h.callback(t, InstagramCallback{Code: "c", State: h.complex.ID.String() + ".x"}, &http.Cookie{Name: instagramStateCookie, Value: h.complex.ID.String() + ".x"})
	if got := outcome(loc); got != "error:unavailable" || h.meta.exchanges.Load() != 0 {
		t.Fatalf("callback with the feature off = %s after %d exchanges", got, h.meta.exchanges.Load())
	}
}

func TestInstagramStatusReportsAnExpiredTokenAsNeedingReconnect(t *testing.T) {
	h := newIGHarness(t, true)
	h.store.conns[h.complex.ID] = &complexstore.InstagramConnection{
		ComplexID: h.complex.ID, Username: "club", AccessToken: igLongToken,
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	w := httptest.NewRecorder()
	h.ig.Status(w, ownerRequest(t, http.MethodGet, "/", h.owner, h.complex, nil, ""))
	body := decode(t, w)
	if body["connected"] != false || body["needs_reconnect"] != true {
		t.Fatalf("status of an expired connection = %v, want connected:false needs_reconnect:true", body)
	}
	if strings.Contains(w.Body.String(), igLongToken) {
		t.Fatalf("status leaked the token")
	}
}

func TestInstagramDisconnectDeletesAndAuditsOnlyWhatExisted(t *testing.T) {
	h := newIGHarness(t, true)
	h.store.conns[h.complex.ID] = &complexstore.InstagramConnection{ComplexID: h.complex.ID, AccessToken: igLongToken}

	for range 2 {
		w := httptest.NewRecorder()
		h.ig.Disconnect(w, ownerRequest(t, http.MethodDelete, "/", h.owner, h.complex, nil, ""))
		if w.Code != http.StatusOK {
			t.Fatalf("disconnect = %d", w.Code)
		}
	}
	if len(h.store.conns) != 0 {
		t.Fatal("the connection is still stored after disconnect")
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].Action != "instagram_disconnect" {
		t.Fatalf("audit = %+v, want exactly one instagram_disconnect", h.audit.entries)
	}
}

func TestInstagramRefreshKeepsGoingPastAMetaErrorAndRetriesIt(t *testing.T) {
	h := newIGHarness(t, true)
	expires := time.Now().Add(10 * 24 * time.Hour).Truncate(time.Second)
	good1, bad, good2 := uuid.New(), uuid.New(), uuid.New()
	for id, tok := range map[uuid.UUID]string{good1: "tok-good-1", bad: "tok-bad", good2: "tok-good-2"} {
		h.store.conns[id] = &complexstore.InstagramConnection{ComplexID: id, AccessToken: tok, ExpiresAt: expires}
	}
	h.meta.refreshFails["tok-bad"] = http.StatusInternalServerError

	out := h.ig.RefreshTokens(t.Context())
	if out.Refreshed != 2 || out.Failed != 1 {
		t.Fatalf("outcome = %+v, want 2 refreshed and 1 failed", out)
	}
	for _, id := range []uuid.UUID{good1, good2} {
		c := h.store.conns[id]
		if !strings.HasPrefix(c.AccessToken, igFreshToken) || time.Until(c.ExpiresAt) < 59*24*time.Hour {
			t.Fatalf("connection %s not refreshed: token %q expires %v", id, c.AccessToken, c.ExpiresAt)
		}
	}
	if c := h.store.conns[bad]; c.AccessToken != "tok-bad" || !c.ExpiresAt.Equal(expires) {
		t.Fatalf("the refused token was changed: %+v", c)
	}
	if h.store.marked != 1 {
		t.Fatalf("expired connections were not marked (%d)", h.store.marked)
	}

	// The next run tries the failed one again.
	delete(h.meta.refreshFails, "tok-bad")
	h.ig.RefreshTokens(t.Context())
	if c := h.store.conns[bad]; !strings.HasPrefix(c.AccessToken, igFreshToken) {
		t.Fatalf("the failed token was not retried on the next run: %q", c.AccessToken)
	}
	if strings.Contains(h.logs.String(), "tok-bad") || strings.Contains(h.logs.String(), igFreshToken) {
		t.Fatalf("a token reached the logs:\n%s", h.logs.String())
	}
}

func TestInstagramRefreshDoesNothingWhenTheFeatureIsOff(t *testing.T) {
	h := newIGHarness(t, false)
	h.store.conns[uuid.New()] = &complexstore.InstagramConnection{AccessToken: "tok", ExpiresAt: time.Now().Add(time.Hour)}
	h.ig.RefreshTokens(t.Context())
	if h.meta.refreshes.Load() != 0 {
		t.Fatal("the refresh called Meta with the feature off")
	}
}

// Meta refusing a refresh means the owner revoked access: the connection says
// so at once instead of reporting connected until the token runs out. An
// outage is retried instead.
func TestInstagramRefreshMarksARevokedTokenAsNeedingReconnect(t *testing.T) {
	h := newIGHarness(t, true)
	expires := time.Now().Add(10 * 24 * time.Hour).Truncate(time.Second)
	revoked, down := uuid.New(), uuid.New()
	h.store.conns[revoked] = &complexstore.InstagramConnection{ComplexID: revoked, AccessToken: "tok-revoked", ExpiresAt: expires}
	h.store.conns[down] = &complexstore.InstagramConnection{ComplexID: down, AccessToken: "tok-down", ExpiresAt: expires}
	h.meta.refreshFails["tok-revoked"] = http.StatusBadRequest
	h.meta.refreshFails["tok-down"] = http.StatusInternalServerError

	out := h.ig.RefreshTokens(t.Context())
	if out.Revoked != 1 || out.Failed != 1 {
		t.Fatalf("outcome = %+v, want 1 revoked and 1 failed", out)
	}
	if !h.store.conns[revoked].NeedsReconnect {
		t.Error("a refused refresh must mark the connection as needing reconnect")
	}
	if h.store.conns[down].NeedsReconnect {
		t.Error("an outage must not mark the connection; it is retried")
	}
}

// A refresh that lost the race to a reconnect stored nothing, and is not
// counted as refreshed.
func TestInstagramRefreshCountsALostRaceAsSkipped(t *testing.T) {
	h := newIGHarness(t, true)
	expires := time.Now().Add(10 * 24 * time.Hour).Truncate(time.Second)
	id := uuid.New()
	h.store.conns[id] = &complexstore.InstagramConnection{ComplexID: id, AccessToken: "tok-1", ExpiresAt: expires}
	h.store.raceOnUpdate = true

	out := h.ig.RefreshTokens(t.Context())
	if out.Refreshed != 0 || out.Skipped != 1 {
		t.Fatalf("outcome = %+v, want 0 refreshed and 1 skipped", out)
	}
}

// The complex changed hands while its owner was on Instagram's consent page:
// the account is not attached to a complex the person no longer owns.
func TestInstagramCallbackStoresNothingWhenTheOwnerChanged(t *testing.T) {
	h := newIGHarness(t, true)
	state, cookie := h.connect(t)
	h.store.owners[h.complex.ID] = uuid.New()

	loc := h.callback(t, InstagramCallback{Code: "AQBx-code#_", State: state}, cookie)
	if got := outcome(loc); got == "connected" {
		t.Fatalf("callback outcome = %s; a former owner must not connect the complex", got)
	}
	if len(h.store.conns) != 0 {
		t.Fatalf("nothing must be stored; got %+v", h.store.conns)
	}
}

// While Meta has not approved publishing, only the owners on the allow list see
// the feature: everyone else gets enabled:false and a refused connect.
func TestInstagramIsOfferedOnlyToTheOwnersOnTheAllowList(t *testing.T) {
	h := newIGHarness(t, true)
	h.ig.cfg.AllowedEmails = []string{"tester@vibe.test"}

	as := func(email, method, target string) *http.Request {
		r := ownerRequest(t, method, target, h.owner, h.complex, nil, "")
		return httpx.ContextSetUser(r, &authstore.User{ID: h.owner, Role: "owner", Email: email})
	}
	enabled := func(email string) bool {
		w := httptest.NewRecorder()
		h.ig.Status(w, as(email, http.MethodGet, "/api/v1/complexes/x/instagram"))
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("status body: %v", err)
		}
		return body.Enabled
	}
	connectCode := func(email string) int {
		w := httptest.NewRecorder()
		h.ig.Connect(w, as(email, http.MethodPost, "/api/v1/complexes/x/instagram/connect"))
		return w.Code
	}

	if enabled("other@vibe.test") {
		t.Fatal("an owner off the list must see enabled:false")
	}
	if got := connectCode("other@vibe.test"); got != http.StatusNotImplemented {
		t.Fatalf("connect for an owner off the list = %d, want 501", got)
	}
	if !enabled("Tester@Vibe.test") {
		t.Fatal("an owner on the list must see enabled:true, whatever the case of the address")
	}
	if got := connectCode("Tester@Vibe.test"); got != http.StatusOK {
		t.Fatalf("connect for an owner on the list = %d, want 200", got)
	}
}

// S8.3: only Graph code 190 means the owner revoked access. Throttling and any
// other refusal is counted as failed and retried on the next run, never
// flagged for reconnect.
func TestInstagramRefreshFlagsReconnectOnlyForCode190(t *testing.T) {
	h := newIGHarness(t, true)
	expires := time.Now().Add(10 * 24 * time.Hour).Truncate(time.Second)
	cases := map[string]struct {
		status, code int
		flagged      bool
	}{
		"tok-190":       {http.StatusBadRequest, instagram.CodeInvalidToken, true},
		"tok-throttled": {http.StatusBadRequest, 4, false},
		"tok-other4xx":  {http.StatusBadRequest, 100, false},
		"tok-403":       {http.StatusForbidden, 10, false},
	}
	ids := map[string]uuid.UUID{}
	for tok, tc := range cases {
		id := uuid.New()
		ids[tok] = id
		h.store.conns[id] = &complexstore.InstagramConnection{ComplexID: id, AccessToken: tok, ExpiresAt: expires}
		h.meta.refreshFails[tok] = tc.status
		h.meta.refreshCodes[tok] = tc.code
	}

	out := h.ig.RefreshTokens(t.Context())

	if out.Revoked != 1 || out.Failed != 3 {
		t.Fatalf("outcome = %+v, want 1 revoked and 3 failed", out)
	}
	for tok, tc := range cases {
		if got := h.store.conns[ids[tok]].NeedsReconnect; got != tc.flagged {
			t.Errorf("%s: needs_reconnect = %v, want %v", tok, got, tc.flagged)
		}
	}

	// The next run retries the ones that were not flagged.
	for tok := range cases {
		delete(h.meta.refreshFails, tok)
	}
	h.ig.RefreshTokens(t.Context())
	for tok, tc := range cases {
		if !tc.flagged && !strings.HasPrefix(h.store.conns[ids[tok]].AccessToken, igFreshToken) {
			t.Errorf("%s was not retried on the next run", tok)
		}
	}
}
