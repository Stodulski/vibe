package complexes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/instagram"
	"github.com/stodulski/vibe-server/internal/timezone"
)

const storyToken = "IGQVJstory-publishing-secret-token"

// memStoryStore is an in-memory InstagramStoryStore. Which daily stories are
// due, the freed cap and the leases are SQL and are tested against Postgres
// (store/instagram_stories_integration_test.go); here the queue keeps what it
// is given, deduplicated by key.
type memStoryStore struct {
	mu       sync.Mutex
	settings map[uuid.UUID]complexstore.StorySettings
	stories  []*memStory
	accounts map[uuid.UUID]*complexstore.StoryAccount
	// failSaves makes that many SaveStoryState calls fail, storing nothing.
	failSaves int
}

type memStory struct {
	complexstore.NewStory
	ID    uuid.UUID
	State complexstore.StoryState
}

func newMemStoryStore() *memStoryStore {
	return &memStoryStore{settings: map[uuid.UUID]complexstore.StorySettings{}, accounts: map[uuid.UUID]*complexstore.StoryAccount{}}
}

func (s *memStoryStore) GetStorySettings(_ context.Context, id uuid.UUID) (complexstore.StorySettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.settings[id]; ok {
		return st, nil
	}
	return complexstore.DefaultStorySettings(), nil
}

func (s *memStoryStore) SaveStorySettings(_ context.Context, id uuid.UUID, in complexstore.StorySettings) (complexstore.StorySettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings[id] = in
	return in, nil
}

func (s *memStoryStore) ListRecentStories(_ context.Context, id uuid.UUID, limit int) ([]complexstore.RecentStory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []complexstore.RecentStory
	for i := len(s.stories) - 1; i >= 0 && len(out) < limit; i-- {
		st := s.stories[i]
		if st.ComplexID != id {
			continue
		}
		r := complexstore.RecentStory{Kind: st.Kind, Status: st.State.Status, ScheduledFor: st.ScheduledFor, PublishedAt: st.State.PublishedAt}
		if st.State.Reason != "" {
			reason := st.State.Reason
			r.Reason = &reason
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *memStoryStore) QueueFreedStory(ctx context.Context, story complexstore.NewStory) (bool, error) {
	settings, _ := s.GetStorySettings(ctx, story.ComplexID)
	if !settings.FreedEnabled {
		return false, nil
	}
	return s.QueueStory(ctx, story)
}

func (s *memStoryStore) QueueStory(_ context.Context, story complexstore.NewStory) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.stories {
		if st.ComplexID == story.ComplexID && st.DedupKey == story.DedupKey {
			return false, nil
		}
	}
	s.stories = append(s.stories, &memStory{NewStory: story, ID: uuid.New(), State: complexstore.StoryState{
		Status: complexstore.StoryPending, NextAttemptAt: story.ScheduledFor,
	}})
	return true, nil
}

func (s *memStoryStore) ListDailyStoriesDue(context.Context, time.Time, string, string, int) ([]complexstore.DailyStoryDue, error) {
	return nil, nil
}

func (s *memStoryStore) ClaimDueStories(_ context.Context, now, _ time.Time, limit int) ([]complexstore.QueuedStory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []complexstore.QueuedStory
	for _, st := range s.stories {
		if st.State.Status != complexstore.StoryPending || st.State.NextAttemptAt.After(now) || len(out) == limit {
			continue
		}
		out = append(out, complexstore.QueuedStory{
			ID: st.ID, ComplexID: st.ComplexID, Kind: st.Kind, ImageURL: st.ImageURL, QueuedDay: st.QueuedDay,
			SlotStartsAt: st.SlotStartsAt, ScheduledFor: st.ScheduledFor, Attempts: st.State.Attempts,
			ContainerID: st.State.ContainerID, ContainerCreatedAt: st.State.ContainerCreatedAt,
		})
	}
	return out, nil
}

func (s *memStoryStore) SaveStoryState(ctx context.Context, id uuid.UUID, st complexstore.StoryState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failSaves > 0 {
		s.failSaves--
		return errors.New("connection reset")
	}
	for _, m := range s.stories {
		if m.ID == id {
			m.State = st
		}
	}
	return nil
}

func (s *memStoryStore) SaveStoryContainer(ctx context.Context, id uuid.UUID, containerID string, createdAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.stories {
		if m.ID == id {
			m.State.ContainerID, m.State.ContainerCreatedAt = containerID, &createdAt
		}
	}
	return nil
}

func (s *memStoryStore) StoryAccount(_ context.Context, id uuid.UUID) (*complexstore.StoryAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return nil, data.ErrRecordNotFound
	}
	cp := *a
	return &cp, nil
}

func (s *memStoryStore) MarkStoryAccountNeedsReconnect(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.accounts[id]; ok {
		a.NeedsReconnect = true
	}
	return nil
}

func (s *memStoryStore) only(t *testing.T) *memStory {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.stories) != 1 {
		t.Fatalf("want exactly one story queued; got %d", len(s.stories))
	}
	return s.stories[0]
}

// fakeStoryMeta is an httptest stand-in for graph.instagram.com's publishing
// endpoints and for the landing's story image route.
type fakeStoryMeta struct {
	srv *httptest.Server
	mu  sync.Mutex
	// calls lists "METHOD path" of every Instagram request, in order.
	calls []string
	// imageStatus is what the image route answers (200 by default).
	imageStatus int
	// quotaUsed is the publishing limit's usage, out of 100.
	quotaUsed int
	// containerStatus is what the container status answers.
	containerStatus string
	// createFail and publishFail, when set, are the status and Graph code the
	// container creation and the publish answer with.
	createFail, publishFail *graphFailure
	// sentImageURL is the image_url the container was created with.
	sentImageURL string
	// onPublish, when set, runs as media_publish arrives.
	onPublish func()
	// message, when set, is the error message failures carry.
	message string
}

type graphFailure struct {
	status, code int
	transient    bool
}

func newFakeStoryMeta(t *testing.T) *fakeStoryMeta {
	t.Helper()
	f := &fakeStoryMeta{imageStatus: http.StatusOK, containerStatus: instagram.ContainerFinished}
	mux := http.NewServeMux()
	fail := func(w http.ResponseWriter, g *graphFailure) {
		msg := "refused for " + storyToken
		if f.message != "" {
			msg = f.message
		}
		body, _ := json.Marshal(map[string]any{"error": map[string]any{"message": msg, "code": g.code, "is_transient": g.transient}})
		w.WriteHeader(g.status)
		_, _ = w.Write(body)
	}
	authorized := func(w http.ResponseWriter, r *http.Request) bool {
		_ = r.ParseForm()
		if r.Form.Get("access_token") != storyToken {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","code":190}}`))
			return false
		}
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		return true
	}
	mux.HandleFunc("GET /historias/c/{slug}/{piece}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(f.imageStatus)
	})
	mux.HandleFunc("GET /1784/content_publishing_limit", func(w http.ResponseWriter, r *http.Request) {
		if authorized(w, r) {
			_, _ = w.Write([]byte(`{"data":[{"quota_usage":` + strconv.Itoa(f.quotaUsed) + `,"config":{"quota_total":100}}]}`))
		}
	})
	mux.HandleFunc("POST /1784/media", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if f.createFail != nil {
			fail(w, f.createFail)
			return
		}
		if r.PostForm.Get("media_type") != "STORIES" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.sentImageURL = r.PostForm.Get("image_url")
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"id":"container-1"}`))
	})
	mux.HandleFunc("GET /container-1", func(w http.ResponseWriter, r *http.Request) {
		if authorized(w, r) {
			f.mu.Lock()
			status := f.containerStatus
			f.mu.Unlock()
			_, _ = w.Write([]byte(`{"status_code":"` + status + `","id":"container-1"}`))
		}
	})
	mux.HandleFunc("POST /1784/media_publish", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if f.publishFail != nil {
			fail(w, f.publishFail)
			return
		}
		if r.PostForm.Get("creation_id") != "container-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if f.onPublish != nil {
			f.onPublish()
		}
		// Meta reports a published container as PUBLISHED from now on.
		f.mu.Lock()
		f.containerStatus = instagram.ContainerPublished
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"id":"media-9"}`))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// count is how many Instagram requests were "METHOD path".
func (f *fakeStoryMeta) count(call string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == call {
			n++
		}
	}
	return n
}

func (f *fakeStoryMeta) instagramCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type storyHarness struct {
	stories *InstagramStories
	store   *memStoryStore
	meta    *fakeStoryMeta
	logs    *bytes.Buffer
	now     time.Time
	complex uuid.UUID
}

// venueTime is a wall-clock time at the venue on a fixed day.
func venueTime(hour, minute int) time.Time {
	return time.Date(2026, 11, 20, hour, minute, 0, 0, timezone.Argentina)
}

func newStoryHarness(t *testing.T) *storyHarness {
	t.Helper()
	meta := newFakeStoryMeta(t)
	logs := &bytes.Buffer{}
	h := &storyHarness{store: newMemStoryStore(), meta: meta, logs: logs, now: venueTime(12, 0), complex: uuid.New()}
	h.store.accounts[h.complex] = &complexstore.StoryAccount{InstagramUserID: "1784", AccessToken: storyToken, ExpiresAt: h.now.Add(30 * 24 * time.Hour)}
	h.store.settings[h.complex] = complexstore.StorySettings{DailyEnabled: true, DailyTime: "10:00", FreedEnabled: true, FreedDailyCap: 5}
	h.stories = NewInstagramStories(InstagramStoriesDeps{
		API:    instagram.New(instagram.Config{GraphBase: meta.srv.URL}),
		Store:  h.store,
		Logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}, InstagramStoriesConfig{Available: true, PublicURL: meta.srv.URL, Now: func() time.Time { return h.now }})
	return h
}

// cancel frees a 90-minute slot on court that starts at startsAt.
func (h *storyHarness) cancel(t *testing.T, court uuid.UUID, startsAt time.Time) {
	t.Helper()
	h.stories.SlotCancelled(t.Context(), h.complex, court, "club-padel", startsAt, startsAt.Add(90*time.Minute))
}

// queueFreed queues a freed story directly, as SlotCancelled does.
func (h *storyHarness) queueFreed(t *testing.T, startsAt time.Time) *memStory {
	t.Helper()
	h.cancel(t, uuid.New(), startsAt)
	return h.store.only(t)
}

func (h *storyHarness) assertNoToken(t *testing.T) {
	t.Helper()
	if strings.Contains(h.logs.String(), storyToken) {
		t.Fatalf("the token reached the logs:\n%s", h.logs.String())
	}
	for _, st := range h.store.stories {
		if strings.Contains(st.State.Reason, storyToken) {
			t.Fatalf("the token reached a stored reason: %q", st.State.Reason)
		}
	}
}

// --- Settings routes --------------------------------------------------------

func newStoryRoutesHarness(t *testing.T, available bool) (*igHarness, *memStoryStore) {
	t.Helper()
	h := newIGHarness(t, available)
	store := newMemStoryStore()
	h.ig.stories = store
	return h, store
}

func storyRequest(t *testing.T, h *igHarness, method, body, email string) *httptest.ResponseRecorder {
	t.Helper()
	r := ownerRequest(t, method, "/api/v1/complexes/x/instagram/stories", h.owner, h.complex, nil, body)
	r = httpx.ContextSetUser(r, &authstore.User{ID: h.owner, Role: "owner", Email: email})
	w := httptest.NewRecorder()
	if method == http.MethodGet {
		h.ig.StorySettings(w, r)
	} else {
		h.ig.UpdateStorySettings(w, r)
	}
	return w
}

// A complex that never saved its settings has every story off, the daily time
// at 10:00 and the cap at 5: connecting publishes nothing by itself.
func TestStorySettingsDefaultToEverythingOff(t *testing.T) {
	h, _ := newStoryRoutesHarness(t, true)
	w := storyRequest(t, h, http.MethodGet, "", "owner@vibe.test")
	if w.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Settings map[string]any `json:"settings"`
		Recent   []any          `json:"recent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"daily_enabled": false, "daily_time": "10:00", "freed_enabled": false, "freed_daily_cap": float64(5)}
	for k, v := range want {
		if body.Settings[k] != v {
			t.Errorf("settings.%s = %v, want %v", k, body.Settings[k], v)
		}
	}
	if body.Recent == nil || len(body.Recent) != 0 {
		t.Errorf("recent = %v, want an empty list", body.Recent)
	}
}

// The daily time must be HH:MM from 08:00 to 22:00 and the cap 1..10; a
// refused body stores nothing, an accepted one is what GET answers next.
func TestStorySettingsValidateTheTimeAndTheCap(t *testing.T) {
	h, store := newStoryRoutesHarness(t, true)
	body := func(dailyTime string, limit int) string {
		return `{"daily_enabled":true,"daily_time":"` + dailyTime + `","freed_enabled":true,"freed_daily_cap":` + strconv.Itoa(limit) + `}`
	}
	for _, tc := range []struct {
		body string
		want int
	}{
		{body("07:59", 5), http.StatusUnprocessableEntity},
		{body("22:01", 5), http.StatusUnprocessableEntity},
		{body("9:00", 5), http.StatusUnprocessableEntity},
		{body("24:00", 5), http.StatusUnprocessableEntity},
		{body("10:00", 0), http.StatusUnprocessableEntity},
		{body("10:00", 11), http.StatusUnprocessableEntity},
		{`{"daily_enabled":true,"daily_time":"10:00","freed_daily_cap":3}`, http.StatusUnprocessableEntity},
	} {
		if w := storyRequest(t, h, http.MethodPut, tc.body, "owner@vibe.test"); w.Code != tc.want {
			t.Errorf("PUT %s = %d, want %d (%s)", tc.body, w.Code, tc.want, w.Body.String())
		}
	}
	if len(store.settings) != 0 {
		t.Fatalf("a refused body stored settings: %+v", store.settings)
	}
	for _, edge := range []struct {
		time  string
		limit int
	}{{"08:00", 1}, {"22:00", 10}} {
		if w := storyRequest(t, h, http.MethodPut, body(edge.time, edge.limit), "owner@vibe.test"); w.Code != http.StatusOK {
			t.Fatalf("PUT %s/%d = %d %s", edge.time, edge.limit, w.Code, w.Body.String())
		}
	}
	got := store.settings[h.complex.ID]
	if !got.DailyEnabled || got.DailyTime != "22:00" || !got.FreedEnabled || got.FreedDailyCap != 10 {
		t.Fatalf("stored settings = %+v", got)
	}
}

// Both routes follow connect's rule: off, or for an owner off the allow list,
// they answer 501 and store nothing.
func TestStoryRoutesAnswer501WhenInstagramIsNotOffered(t *testing.T) {
	h, store := newStoryRoutesHarness(t, false)
	put := `{"daily_enabled":true,"daily_time":"10:00","freed_enabled":true,"freed_daily_cap":3}`
	if w := storyRequest(t, h, http.MethodGet, "", "owner@vibe.test"); w.Code != http.StatusNotImplemented {
		t.Errorf("GET with the feature off = %d, want 501", w.Code)
	}
	if w := storyRequest(t, h, http.MethodPut, put, "owner@vibe.test"); w.Code != http.StatusNotImplemented {
		t.Errorf("PUT with the feature off = %d, want 501", w.Code)
	}

	h.ig.cfg.Available = true
	h.ig.cfg.AllowedEmails = []string{"tester@vibe.test"}
	if w := storyRequest(t, h, http.MethodPut, put, "other@vibe.test"); w.Code != http.StatusNotImplemented {
		t.Errorf("PUT for an owner off the list = %d, want 501", w.Code)
	}
	if len(store.settings) != 0 {
		t.Fatalf("a refused PUT stored settings: %+v", store.settings)
	}
	if w := storyRequest(t, h, http.MethodPut, put, "Tester@vibe.test"); w.Code != http.StatusOK {
		t.Errorf("PUT for an owner on the list = %d, want 200", w.Code)
	}
}

// --- Freed stories ----------------------------------------------------------

// A cancellation queues one story naming the slot, and the same slot freed a
// second time is not queued again.
func TestAFreedSlotQueuesOneStoryWithItsImage(t *testing.T) {
	h := newStoryHarness(t)
	court := uuid.New()
	starts := venueTime(18, 30)
	h.cancel(t, court, starts)
	h.cancel(t, court, starts)

	st := h.store.only(t)
	u, err := url.Parse(st.ImageURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Path != "/historias/c/club-padel/liberado.jpg" || q.Get("fecha") != "2026-11-20" || q.Get("hora") != "18:30" ||
		q.Get("cancha") != court.String() || q.Get("duracion") != "90" {
		t.Fatalf("image URL = %s", st.ImageURL)
	}
	if !st.ScheduledFor.Equal(h.now) {
		t.Fatalf("scheduled for %s, want now (%s)", st.ScheduledFor, h.now)
	}
}

// A freed story needs freed stories on and a usable connection, and a slot
// from an hour to 48 hours away.
func TestAFreedSlotIsQueuedOnlyWhenItMayBePublished(t *testing.T) {
	cases := []struct {
		name    string
		starts  time.Duration // after now
		prepare func(h *storyHarness)
		want    bool
	}{
		{"an hour away", time.Hour, nil, true},
		{"48 hours away", 48 * time.Hour, nil, true},
		{"59 minutes away", 59 * time.Minute, nil, false},
		{"in the past", -time.Hour, nil, false},
		{"more than 48 hours away", 48*time.Hour + time.Minute, nil, false},
		{"freed stories off", 3 * time.Hour, func(h *storyHarness) {
			s := h.store.settings[h.complex]
			s.FreedEnabled = false
			h.store.settings[h.complex] = s
		}, false},
		{"not connected", 3 * time.Hour, func(h *storyHarness) { delete(h.store.accounts, h.complex) }, false},
		{"needs reconnect", 3 * time.Hour, func(h *storyHarness) { h.store.accounts[h.complex].NeedsReconnect = true }, false},
		{"token expired", 3 * time.Hour, func(h *storyHarness) { h.store.accounts[h.complex].ExpiresAt = h.now.Add(-time.Minute) }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newStoryHarness(t)
			if tc.prepare != nil {
				tc.prepare(h)
			}
			h.cancel(t, uuid.New(), h.now.Add(tc.starts))
			if got := len(h.store.stories) == 1; got != tc.want {
				t.Fatalf("queued = %v, want %v", got, tc.want)
			}
		})
	}
}

// In quiet hours a freed story waits for 08:00, and a slot that would then be
// less than an hour away is not queued at all.
func TestAFreedSlotInQuietHoursWaitsForEightOClock(t *testing.T) {
	h := newStoryHarness(t)
	h.now = venueTime(23, 30)
	h.cancel(t, uuid.New(), venueTime(23, 30).Add(9*time.Hour)) // 08:30 next day
	if len(h.store.stories) != 0 {
		t.Fatal("a slot 30 minutes after quiet hours end was queued")
	}

	h.cancel(t, uuid.New(), venueTime(23, 30).Add(10*time.Hour+30*time.Minute)) // 10:00 next day
	st := h.store.only(t)
	if want := time.Date(2026, 11, 21, 8, 0, 0, 0, timezone.Argentina); !st.ScheduledFor.Equal(want) {
		t.Fatalf("scheduled for %s, want %s", st.ScheduledFor, want)
	}

	// The publisher holds it until 08:00 without spending an attempt.
	h.stories.PublishDue(t.Context())
	if h.meta.instagramCalls() != 0 {
		t.Fatal("Instagram was called during quiet hours")
	}
}

// A queued freed story whose slot is less than an hour away by the time it
// would be published is skipped, not published.
func TestAFreedStoryIsSkippedWhenItsSlotIsUnderAnHourAway(t *testing.T) {
	h := newStoryHarness(t)
	st := h.queueFreed(t, h.now.Add(2*time.Hour))
	h.now = h.now.Add(61 * time.Minute)
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StorySkipped {
		t.Fatalf("status = %s, want skipped", st.State.Status)
	}
	if h.meta.instagramCalls() != 0 {
		t.Fatal("Instagram was called for a slot under an hour away")
	}
}

// --- Publishing -------------------------------------------------------------

// The happy path: the image is there, the quota is not full, the container is
// created with the image URL and FINISHED, and it is published.
func TestAStoryIsPublished(t *testing.T) {
	h := newStoryHarness(t)
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	out := h.stories.PublishDue(t.Context())

	if st.State.Status != complexstore.StoryPublished || st.State.PublishedAt == nil || st.State.MediaID != "media-9" {
		t.Fatalf("state = %+v, want published with media-9", st.State)
	}
	if out.Published != 1 {
		t.Fatalf("outcome = %+v", out)
	}
	if h.meta.sentImageURL != st.ImageURL {
		t.Fatalf("container image_url = %q, want %q", h.meta.sentImageURL, st.ImageURL)
	}
	h.assertNoToken(t)
}

// A container still processing is kept and checked by a later run, which
// publishes it without creating another.
func TestAContainerStillProcessingIsPublishedByALaterRun(t *testing.T) {
	h := newStoryHarness(t)
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	h.meta.containerStatus = instagram.ContainerInProgress
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryPending || st.State.ContainerID != "container-1" || st.State.Attempts != 0 {
		t.Fatalf("after the first run: %+v, want pending with its container and no attempt spent", st.State)
	}

	h.meta.containerStatus = instagram.ContainerFinished
	h.now = h.now.Add(time.Minute)
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryPublished {
		t.Fatalf("after the second run: %+v, want published", st.State)
	}
	creates := 0
	for _, c := range h.meta.calls {
		if c == "POST /1784/media" {
			creates++
		}
	}
	if creates != 1 {
		t.Fatalf("containers created = %d, want 1", creates)
	}
}

// The image route's 404 means nothing true to show: skipped, and Instagram is
// never asked.
func TestAStoryWhoseImageIsGoneIsSkippedWithoutCallingInstagram(t *testing.T) {
	h := newStoryHarness(t)
	h.meta.imageStatus = http.StatusNotFound
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StorySkipped {
		t.Fatalf("status = %s, want skipped", st.State.Status)
	}
	if h.meta.instagramCalls() != 0 {
		t.Fatalf("Instagram received %v", h.meta.calls)
	}
}

// 9007 (media not ready) and 5xx are retried after 5m, 15m and 1h; the fourth
// failure fails the story.
func TestRetryableFailuresBackOffThenFail(t *testing.T) {
	for _, fail := range []graphFailure{{status: http.StatusBadRequest, code: instagram.CodeMediaNotReady}, {status: http.StatusInternalServerError, code: 1}} {
		h := newStoryHarness(t)
		h.meta.publishFail = &fail
		st := h.queueFreed(t, h.now.Add(40*time.Hour))
		for i, wait := range []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour} {
			h.stories.PublishDue(t.Context())
			if st.State.Status != complexstore.StoryPending || st.State.Attempts != i+1 || !st.State.NextAttemptAt.Equal(h.now.Add(wait)) {
				t.Fatalf("%+v attempt %d: %+v, want pending, next in %s", fail, i+1, st.State, wait)
			}
			h.now = st.State.NextAttemptAt
		}
		h.stories.PublishDue(t.Context())
		if st.State.Status != complexstore.StoryFailed || st.State.Attempts != 4 {
			t.Fatalf("%+v after the last attempt: %+v, want failed", fail, st.State)
		}
		h.assertNoToken(t)
	}
}

// Any other refusal fails the story at once.
func TestOtherRefusalsFailAtOnce(t *testing.T) {
	h := newStoryHarness(t)
	h.meta.createFail = &graphFailure{status: http.StatusBadRequest, code: 100}
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryFailed || st.State.Attempts != 1 || st.State.Reason == "" {
		t.Fatalf("state = %+v, want failed with a reason", st.State)
	}
	if h.store.accounts[h.complex].NeedsReconnect {
		t.Fatal("a refusal that is not about the token flagged the connection")
	}
	h.assertNoToken(t)
}

// An invalid token (code 190) fails the story and flags the connection, so the
// owner is asked to connect again and nothing else is attempted.
func TestAnInvalidTokenFailsAndFlagsTheConnection(t *testing.T) {
	h := newStoryHarness(t)
	h.meta.createFail = &graphFailure{status: http.StatusBadRequest, code: instagram.CodeInvalidToken}
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	other := h.queueSecondFreed(t, h.now.Add(4*time.Hour))
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryFailed {
		t.Fatalf("status = %s, want failed", st.State.Status)
	}
	if !h.store.accounts[h.complex].NeedsReconnect {
		t.Fatal("the connection was not flagged for reconnect")
	}
	if other.State.Status != complexstore.StorySkipped {
		t.Fatalf("the next story of the same complex = %s, want skipped once the connection is flagged", other.State.Status)
	}
	h.assertNoToken(t)
}

func (h *storyHarness) queueSecondFreed(t *testing.T, startsAt time.Time) *memStory {
	t.Helper()
	h.cancel(t, uuid.New(), startsAt)
	return h.store.stories[len(h.store.stories)-1]
}

// A full publishing quota waits an hour without spending an attempt or
// creating a container.
func TestAFullQuotaWaitsAnHourWithoutSpendingAnAttempt(t *testing.T) {
	h := newStoryHarness(t)
	h.meta.quotaUsed = 100
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryPending || st.State.Attempts != 0 || !st.State.NextAttemptAt.Equal(h.now.Add(time.Hour)) {
		t.Fatalf("state = %+v, want pending, no attempt spent, next in an hour", st.State)
	}
	for _, c := range h.meta.calls {
		if c == "POST /1784/media" {
			t.Fatal("a container was created with the quota full")
		}
	}
}

// --- Daily stories ----------------------------------------------------------

// A daily story is published only on its own day and while daily stories are
// on; nothing is queued in quiet hours.
func TestADailyStoryIsPublishedOnlyOnItsDay(t *testing.T) {
	h := newStoryHarness(t)
	queue := func() *memStory {
		_, _ = h.store.QueueStory(t.Context(), complexstore.NewStory{
			ComplexID: h.complex, Kind: complexstore.StoryKindDaily, DedupKey: "daily:2026-11-20",
			ImageURL:  h.meta.srv.URL + "/historias/c/club-padel/libres.jpg?fecha=2026-11-20",
			QueuedDay: timezone.Day(venueTime(0, 0)), ScheduledFor: venueTime(10, 0),
		})
		return h.store.only(t)
	}
	st := queue()
	h.now = venueTime(22, 59).AddDate(0, 0, 1) // the next day
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StorySkipped {
		t.Fatalf("a daily story published the next day: %+v", st.State)
	}

	h2 := newStoryHarness(t)
	h = h2
	st = queue()
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryPublished {
		t.Fatalf("today's daily story: %+v, want published", st.State)
	}

	h.now = venueTime(23, 0)
	if n := h.stories.QueueDailyStories(t.Context()); n != 0 {
		t.Fatalf("queued %d daily stories in quiet hours", n)
	}
}

// --- Never twice ------------------------------------------------------------

const (
	callCreate  = "POST /1784/media"
	callPublish = "POST /1784/media_publish"
)

// A retry whose container Meta already reports PUBLISHED is published: no
// second media_publish and no new container.
func TestARetryWhoseContainerIsAlreadyPublishedIsNotPublishedAgain(t *testing.T) {
	h := newStoryHarness(t)
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	created := h.now.Add(-10 * time.Minute)
	st.State.Attempts, st.State.ContainerID, st.State.ContainerCreatedAt = 1, "container-1", &created
	h.meta.containerStatus = instagram.ContainerPublished

	h.stories.PublishDue(t.Context())

	if st.State.Status != complexstore.StoryPublished || st.State.PublishedAt == nil {
		t.Fatalf("state = %+v, want published", st.State)
	}
	if n, c := h.meta.count(callPublish), h.meta.count(callCreate); n != 0 || c != 0 {
		t.Fatalf("media_publish sent %d times and %d containers created, want none", n, c)
	}
}

// The container id is stored before media_publish, so a save that fails after
// Meta published leaves the next run a container it finds PUBLISHED: exactly
// one media_publish ever.
func TestAFailedSaveAfterPublishingDoesNotPublishAgain(t *testing.T) {
	h := newStoryHarness(t)
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	h.meta.onPublish = func() {
		if st.State.ContainerID != "container-1" {
			t.Errorf("media_publish arrived before the container id was stored (stored %q)", st.State.ContainerID)
		}
	}
	h.store.failSaves = 1

	h.stories.PublishDue(t.Context())
	h.now = h.now.Add(time.Minute)
	h.stories.PublishDue(t.Context())

	if n := h.meta.count(callPublish); n != 1 {
		t.Fatalf("media_publish sent %d times, want 1", n)
	}
	if c := h.meta.count(callCreate); c != 1 {
		t.Fatalf("%d containers created, want 1", c)
	}
	if st.State.Status != complexstore.StoryPublished {
		t.Fatalf("state = %+v, want published", st.State)
	}
}

// A run cancelled while media_publish is in flight (SIGTERM, the job deadline)
// still stores the story's state with its container, so the next run finds it
// PUBLISHED instead of publishing it again.
func TestTheStateIsSavedWhenTheRunIsCancelled(t *testing.T) {
	h := newStoryHarness(t)
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	ctx, cancel := context.WithCancel(t.Context())
	h.meta.onPublish = cancel

	h.stories.PublishDue(ctx)
	if st.State.Attempts != 1 || st.State.ContainerID != "container-1" {
		t.Fatalf("after the cancelled run: %+v, want the attempt and the container stored", st.State)
	}

	h.meta.onPublish = nil
	h.now = st.State.NextAttemptAt
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryPublished || h.meta.count(callPublish) != 1 {
		t.Fatalf("state %+v after %d media_publish, want published by exactly one", st.State, h.meta.count(callPublish))
	}
}

// A run with less time left than one story's worst case starts no story.
func TestARunWithoutTimeForAWholeStoryStartsNone(t *testing.T) {
	h := newStoryHarness(t)
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	h.stories.PublishDue(ctx)

	if h.meta.instagramCalls() != 0 || st.State.Status != complexstore.StoryPending || st.State.Attempts != 0 {
		t.Fatalf("calls %v, state %+v: want nothing started and no attempt spent", h.meta.calls, st.State)
	}
}

// Throttling Meta answers as HTTP 400 (codes 4, 17, 32, 613, 80002, or any
// error marked is_transient) is retried with backoff, not failed at once.
func TestThrottlingAnsweredAs400IsRetried(t *testing.T) {
	for _, fail := range []graphFailure{
		{status: http.StatusBadRequest, code: 4}, {status: http.StatusBadRequest, code: 17},
		{status: http.StatusBadRequest, code: 32}, {status: http.StatusBadRequest, code: 613},
		{status: http.StatusBadRequest, code: 80002}, {status: http.StatusBadRequest, code: 1, transient: true},
	} {
		h := newStoryHarness(t)
		h.meta.createFail = &fail
		st := h.queueFreed(t, h.now.Add(3*time.Hour))
		h.stories.PublishDue(t.Context())
		if st.State.Status != complexstore.StoryPending || st.State.Attempts != 1 || !st.State.NextAttemptAt.Equal(h.now.Add(5*time.Minute)) {
			t.Errorf("%+v: state = %+v, want pending, retried in 5m", fail, st.State)
		}
		if h.store.accounts[h.complex].NeedsReconnect {
			t.Errorf("%+v flagged the connection", fail)
		}
	}
}

// A long reason made of multi-byte characters is cut on a character boundary:
// Postgres refuses invalid UTF-8, which would leave the story looping.
func TestALongMultiByteReasonIsStoredAsValidUTF8(t *testing.T) {
	h := newStoryHarness(t)
	h.meta.message = strings.Repeat("ñ€", 400)
	h.meta.createFail = &graphFailure{status: http.StatusBadRequest, code: 100}
	st := h.queueFreed(t, h.now.Add(3*time.Hour))
	h.stories.PublishDue(t.Context())
	if st.State.Status != complexstore.StoryFailed {
		t.Fatalf("state = %+v, want failed", st.State)
	}
	if !utf8.ValidString(st.State.Reason) || len(st.State.Reason) > maxStoryReason || len(st.State.Reason) < maxStoryReason-4 {
		t.Fatalf("reason of %d bytes, valid UTF-8 %v", len(st.State.Reason), utf8.ValidString(st.State.Reason))
	}
}
