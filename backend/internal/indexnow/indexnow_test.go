package indexnow

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/jobs"
)

// fakeQueue records every EnqueueAt. It applies the dedup rule the real queue
// applies, so a test can count the submissions that would actually be run.
type fakeQueue struct {
	calls []enqueued
	seen  map[string]bool
}

type enqueued struct {
	taskType string
	payload  any
	runAt    time.Time
	key      string
}

func (q *fakeQueue) EnqueueAt(taskType string, payload any, runAt time.Time, dedupKey string) {
	q.calls = append(q.calls, enqueued{taskType: taskType, payload: payload, runAt: runAt, key: dedupKey})
	if q.seen == nil {
		q.seen = map[string]bool{}
	}
	q.seen[dedupKey] = true
}

func (q *fakeQueue) submissions() int { return len(q.seen) }

func visible(slug, city string) Snapshot { return Snapshot{Slug: slug, City: city, Visible: true} }
func hidden(slug, city string) Snapshot  { return Snapshot{Slug: slug, City: city} }

func TestTheKeyIsTheOneTheLandingServes(t *testing.T) {
	// This package runs in backend/internal/indexnow; the key file is the
	// landing's public file, which the IndexNow protocol reads to verify the site.
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "landing", "public", Key+".txt"))
	if err != nil {
		t.Fatalf("the landing must serve the key file: %v", err)
	}
	if strings.TrimSpace(string(raw)) != Key {
		t.Fatalf("landing/public/%s.txt holds %q, want the Key constant", Key, raw)
	}
}

func TestEnabledOnlyForTheProductionOrigin(t *testing.T) {
	tests := []struct {
		env, origin string
		want        bool
	}{
		{"production", "https://vibe.com.ar", true},
		{"production", "https://vibe.com.ar/", true},
		{"development", "https://vibe.com.ar", false},
		{"staging", "https://vibe.com.ar", false},
		{"production", "https://app.vibe.com.ar", false},
		{"production", "http://localhost:5173", false},
	}
	for _, tt := range tests {
		if got := Enabled(tt.env, tt.origin); got != tt.want {
			t.Errorf("Enabled(%q, %q) = %v, want %v", tt.env, tt.origin, got, tt.want)
		}
	}
}

func TestURLsForAChange(t *testing.T) {
	const origin = ProductionOrigin
	const hub = origin + "/canchas/vicente-lopez"
	const index = origin + "/canchas"

	tests := []struct {
		name   string
		change Change
		want   []string
	}{
		{
			name:   "a new visible complex: its page, its hub and the index",
			change: Change{After: visible("club-norte", "Vicente López")},
			want:   []string{origin + "/c/club-norte", hub, index},
		},
		{
			name:   "a new switched-off complex: nothing",
			change: Change{After: hidden("club-norte", "Vicente López")},
		},
		{
			name:   "an edit of a visible complex: its page and its hub",
			change: Change{Before: visible("club-norte", "Vicente López"), After: visible("club-norte", "Vicente López")},
			want:   []string{origin + "/c/club-norte", hub},
		},
		{
			name:   "a slug change of a visible complex: the old and the new page, and the hub",
			change: Change{Before: visible("club-viejo", "Vicente López"), After: visible("club-norte", "Vicente López")},
			want:   []string{origin + "/c/club-norte", origin + "/c/club-viejo", hub},
		},
		{
			name: "a move to another city: both hubs",
			change: Change{
				Before: visible("club-norte", "Rosario"),
				After:  visible("club-norte", "Vicente López"),
			},
			want: []string{origin + "/c/club-norte", origin + "/canchas/rosario", hub},
		},
		{
			name:   "switched off: the page it stops serving, its hub and the index",
			change: Change{Before: visible("club-norte", "Vicente López"), After: hidden("club-norte", "Vicente López")},
			want:   []string{origin + "/c/club-norte", hub, index},
		},
		{
			name:   "switched on: the page, its hub and the index",
			change: Change{Before: hidden("club-norte", "Vicente López"), After: visible("club-norte", "Vicente López")},
			want:   []string{origin + "/c/club-norte", hub, index},
		},
		{
			name:   "deleted while visible: the page that now answers gone, its hub and the index",
			change: Change{Before: visible("club-norte", "Vicente López")},
			want:   []string{origin + "/c/club-norte", hub, index},
		},
		{
			name:   "deleted while switched off: nothing",
			change: Change{Before: hidden("club-norte", "Vicente López")},
		},
		{
			name:   "an edit while switched off: nothing",
			change: Change{Before: hidden("club-viejo", "Vicente López"), After: hidden("club-norte", "Vicente López")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := URLsFor(origin, tt.change)
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("URLsFor() = %q, want %q", got, want)
			}
		})
	}
}

func TestURLsForIgnoresATrailingSlashOnTheOrigin(t *testing.T) {
	change := Change{After: visible("club-norte", "Rosario")}
	if got, want := URLsFor(ProductionOrigin+"/", change), URLsFor(ProductionOrigin, change); !slices.Equal(got, want) {
		t.Fatalf("trailing slash changed the URLs: %q, want %q", got, want)
	}
}

func TestNothingIsQueuedOutsideProduction(t *testing.T) {
	tests := []struct {
		name   string
		env    string
		origin string
		queue  Queue
	}{
		{name: "development", env: "development", origin: ProductionOrigin, queue: &fakeQueue{}},
		{name: "a local origin in production", env: "production", origin: "http://localhost:5173", queue: &fakeQueue{}},
		{name: "no durable queue", env: "production", origin: ProductionOrigin, queue: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, _ := tt.queue.(*fakeQueue)
			svc := NewService(tt.env, tt.origin, tt.queue, NewClient(tt.origin, nil), discard())
			svc.ComplexChanged(Change{ComplexID: uuid.New(), After: visible("club-norte", "Rosario")})
			if q != nil && len(q.calls) != 0 {
				t.Fatalf("queued %d submissions, want none", len(q.calls))
			}
		})
	}
}

func TestAChangeIsDueWhenItsDebounceWindowCloses(t *testing.T) {
	q := &fakeQueue{}
	svc := NewService("production", ProductionOrigin, q, NewClient(ProductionOrigin, nil), discard())
	svc.now = fixedClock(time.Date(2026, 10, 11, 12, 3, 20, 0, time.UTC))

	svc.ComplexChanged(Change{ComplexID: uuid.New(), After: visible("club-norte", "Rosario")})

	if len(q.calls) != 1 {
		t.Fatalf("queued %d submissions, want 1", len(q.calls))
	}
	want := time.Date(2026, 10, 11, 12, 10, 0, 0, time.UTC)
	if !q.calls[0].runAt.Equal(want) {
		t.Fatalf("runAt = %s, want the end of the window %s", q.calls[0].runAt, want)
	}
	if q.calls[0].taskType != TaskSubmit {
		t.Fatalf("task type = %q, want %q", q.calls[0].taskType, TaskSubmit)
	}
}

func TestABurstOfEditsInOneWindowIsOneSubmission(t *testing.T) {
	q := &fakeQueue{}
	svc := NewService("production", ProductionOrigin, q, NewClient(ProductionOrigin, nil), discard())
	id := uuid.New()
	clock := time.Date(2026, 10, 11, 12, 1, 0, 0, time.UTC)
	svc.now = func() time.Time { return clock }

	// Owners save settings repeatedly: the same page, the same hub, the same window.
	svc.ComplexChanged(Change{ComplexID: id, Before: visible("club-norte", "Rosario"), After: visible("club-norte", "Rosario")})
	clock = clock.Add(6*time.Minute + 30*time.Second)
	svc.ComplexChanged(Change{ComplexID: id, Before: visible("club-norte", "Rosario"), After: visible("club-norte", "Rosario")})

	if got := q.submissions(); got != 1 {
		t.Fatalf("a burst of edits made %d submissions, want 1", got)
	}
	if q.calls[0].key != q.calls[1].key {
		t.Fatal("two saves in one window must share a dedup key")
	}
}

func TestAChangeOfAnotherPageIsNotDeduplicatedAway(t *testing.T) {
	q := &fakeQueue{}
	svc := NewService("production", ProductionOrigin, q, NewClient(ProductionOrigin, nil), discard())
	id := uuid.New()
	clock := time.Date(2026, 10, 11, 12, 1, 0, 0, time.UTC)
	svc.now = func() time.Time { return clock }

	svc.ComplexChanged(Change{ComplexID: id, Before: visible("club-norte", "Rosario"), After: visible("club-norte", "Rosario")})
	clock = clock.Add(2 * time.Minute)
	// A rename in the same window changes which pages are affected: it must not
	// be swallowed by the submission already queued for the edit.
	svc.ComplexChanged(Change{ComplexID: id, Before: visible("club-norte", "Rosario"), After: visible("club-nuevo", "Rosario")})
	clock = clock.Add(20 * time.Minute)
	svc.ComplexChanged(Change{ComplexID: id, Before: visible("club-norte", "Rosario"), After: visible("club-norte", "Rosario")})

	if got := q.submissions(); got != 3 {
		t.Fatalf("made %d submissions, want 3: the rename, the edit before it, and the edit in a later window", got)
	}
}

func TestTheJobPostsTheRecordedURLs(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusAccepted)
	})
	svc := NewService("production", ProductionOrigin, nil, c, discard())

	payload, err := json.Marshal(submission{URLs: []string{ProductionOrigin + "/c/club-norte"}, RequestID: "req-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Submit(t.Context(), payload); err != nil {
		t.Fatalf("Submit() = %v, want nil", err)
	}
	if got["urlList"] == nil {
		t.Fatalf("the endpoint received no urlList: %v", got)
	}
}

func TestARefusedSubmissionIsDeadLetteredAndLogged(t *testing.T) {
	logs := &strings.Builder{}
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	svc := NewService("production", ProductionOrigin, nil, c, slog.New(slog.NewTextHandler(logs, nil)))

	payload, _ := json.Marshal(submission{URLs: []string{ProductionOrigin + "/c/club-norte"}, RequestID: "req-2"})
	err := svc.Submit(t.Context(), payload)
	if jobs.Classify(err) != jobs.OutcomePermanent {
		t.Fatalf("a refused submission must be dead-lettered, got outcome %v", jobs.Classify(err))
	}
	if !strings.Contains(logs.String(), "refused") || !strings.Contains(logs.String(), "req-2") {
		t.Fatalf("the refusal must be logged as a warning with its request id, got %q", logs.String())
	}
}

// newTestClient returns a Client that posts to a test server instead of IndexNow.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(ProductionOrigin, circuitbreaker.New(circuitbreaker.Config{Name: "indexnow-test", MaxFailures: 3, ResetTimeout: time.Minute}))
	c.endpoint = srv.URL
	return c
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
