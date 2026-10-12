package social

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/storage"
)

type fakeAuth struct{ forbidden bool }

func (f fakeAuth) Verify(_ context.Context, raw string) error {
	if raw != "good" {
		return ErrUnauthenticated
	}
	if f.forbidden {
		return ErrForbidden
	}
	return nil
}

type fakeStore struct {
	posts     map[string]Post
	staged    map[string]map[string]Media
	frozen    map[string]bool
	cancelled []string
	upsertErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{posts: map[string]Post{}, staged: map[string]map[string]Media{}, frozen: map[string]bool{}}
}

func (s *fakeStore) Upsert(_ context.Context, p Post) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	if s.frozen[p.ID] {
		return ErrFrozen
	}
	s.posts[p.ID] = p
	return nil
}

func (s *fakeStore) StageMedia(_ context.Context, postID string, m Media) error {
	if s.staged[postID] == nil {
		s.staged[postID] = map[string]Media{}
	}
	s.staged[postID][m.Name] = m
	return nil
}

func (s *fakeStore) StagedMedia(_ context.Context, postID string, names []string) ([]Media, error) {
	var out []Media
	for _, n := range names {
		if m, ok := s.staged[postID][n]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *fakeStore) IsFrozen(_ context.Context, id string) (bool, error) {
	return s.frozen[id], nil
}

func (s *fakeStore) CancelUnpublished(_ context.Context, keep []string) ([]string, error) {
	var out []string
	for id, p := range s.posts {
		if p.Status == StatusScheduled && !slices.Contains(keep, id) {
			out = append(out, id)
			p.Status = StatusCancelled
			s.posts[id] = p
		}
	}
	s.cancelled = out
	return out, nil
}

type fakeMedia struct {
	base string
	puts []storage.Object
}

func (m *fakeMedia) PutObject(_ context.Context, obj storage.Object) error {
	m.puts = append(m.puts, obj)
	return nil
}

func (m *fakeMedia) PublicURL(key string) string {
	if m.base == "" {
		return ""
	}
	return m.base + "/" + key
}

type rig struct {
	h     *Handler
	store *fakeStore
	media *fakeMedia
}

func newRig(auth fakeAuth) rig {
	store := newFakeStore()
	media := &fakeMedia{base: "https://cdn.test"}
	logger := slog.New(slog.DiscardHandler)
	return rig{
		h:     NewHandler(auth, store, media, httpx.NewResponder(logger), logger),
		store: store,
		media: media,
	}
}

func authed(r *http.Request) *http.Request {
	r.Header.Set("Authorization", "Bearer good")
	return r
}

func postRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/social/posts/p1", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return authed(r)
}

const imagePost = `{"publish_at":"2026-10-20T19:00:00-03:00","networks":["instagram","facebook"],` +
	`"type":"image","caption":"Caja de turnos","alt_text":"","media":["1.jpg"],"draft":false}`

var jpegBody = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{1}, 32)...)

func (r rig) stage(t *testing.T, postID, name string) {
	t.Helper()
	if err := r.store.StageMedia(context.Background(), postID, Media{Name: name, URL: "u", ContentType: "image/jpeg", SHA256: "s", SizeBytes: 1}); err != nil {
		t.Fatalf("stage: %v", err)
	}
}

func TestPutPostRequiresAnAcceptedToken(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		auth   fakeAuth
		header string
		want   int
	}{
		"no token":       {auth: fakeAuth{}, header: "", want: http.StatusUnauthorized},
		"rejected token": {auth: fakeAuth{}, header: "Bearer bad", want: http.StatusUnauthorized},
		"other workflow": {auth: fakeAuth{forbidden: true}, header: "Bearer good", want: http.StatusForbidden},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rg := newRig(tc.auth)
			r := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/social/posts/p1", strings.NewReader(imagePost))
			r.Header.Set("Content-Type", "application/json")
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			rg.h.PutPost(w, r, "p1")
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestPutPostStoresThePostWithItsStagedMedia(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "p1", "1.jpg")

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, imagePost), "p1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", w.Code, w.Body.String())
	}
	stored, ok := rg.store.posts["p1"]
	if !ok || len(stored.Media) != 1 || stored.Media[0].SHA256 != "s" {
		t.Fatalf("stored post = %+v, want one media record copied from the upload", stored)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["status"] != "scheduled" {
		t.Fatalf("body = %s, want status scheduled", w.Body.String())
	}
}

func TestPutPostNamesTheMediaThatWasNeverUploaded(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, imagePost), "p1")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for unuploaded media", w.Code)
	}
	if !strings.Contains(w.Body.String(), "1.jpg") {
		t.Fatalf("body %q does not name the missing file", w.Body.String())
	}
}

func TestPutPostRefusesAPublishedPost(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "p1", "1.jpg")
	rg.store.frozen["p1"] = true

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, imagePost), "p1")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for a frozen post", w.Code)
	}
}

func TestPutPostRejectsInvalidContent(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.stage(t, "p1", "1.jpg")
	body := strings.Replace(imagePost, `"Caja de turnos"`, `"   "`, 1)

	w := httptest.NewRecorder()
	rg.h.PutPost(w, postRequest(t, body), "p1")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for an empty caption", w.Code)
	}
}

func uploadRequest(name, contentType string, body []byte) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/social/posts/p1/media/"+name, bytes.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	return authed(r)
}

func TestPutMediaStoresTheBytesUnderTheirHash(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})

	w := httptest.NewRecorder()
	rg.h.PutMedia(w, uploadRequest("1.jpg", "image/jpeg", jpegBody), "p1", "1.jpg")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s; want 200", w.Code, w.Body.String())
	}
	if len(rg.media.puts) != 1 || !strings.HasPrefix(rg.media.puts[0].Key, "social/") ||
		!strings.HasSuffix(rg.media.puts[0].Key, ".jpg") || rg.media.puts[0].ContentType != "image/jpeg" {
		t.Fatalf("stored objects = %+v, want one social/<sha>.jpg image/jpeg object", rg.media.puts)
	}
	if got := rg.store.staged["p1"]["1.jpg"]; got.URL == "" || got.SizeBytes != int64(len(jpegBody)) {
		t.Fatalf("staged media = %+v, want the URL and size", got)
	}
}

func TestPutMediaRefusesWrongBytesAndWrongTypes(t *testing.T) {
	t.Parallel()
	mp4 := append([]byte{0, 0, 0, 0x18}, []byte("ftypisom0000")...)
	cases := map[string]struct {
		name, contentType string
		body              []byte
		want              int
	}{
		"mp4 bytes in a jpg": {name: "1.jpg", contentType: "image/jpeg", body: mp4, want: http.StatusUnsupportedMediaType},
		"png content type":   {name: "1.png", contentType: "image/png", body: jpegBody, want: http.StatusUnsupportedMediaType},
		"unsafe name":        {name: "..%2F1.jpg", contentType: "image/jpeg", body: jpegBody, want: http.StatusUnprocessableEntity},
		"empty body":         {name: "1.jpg", contentType: "image/jpeg", body: nil, want: http.StatusUnprocessableEntity},
		"image over the cap": {name: "1.jpg", contentType: "image/jpeg", body: append(append([]byte{}, jpegBody...), bytes.Repeat([]byte{0}, int(MaxImageBytes))...), want: http.StatusRequestEntityTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rg := newRig(fakeAuth{})
			w := httptest.NewRecorder()
			rg.h.PutMedia(w, uploadRequest(tc.name, tc.contentType, tc.body), "p1", tc.name)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if len(rg.media.puts) != 0 {
				t.Fatal("a refused upload must not reach storage")
			}
		})
	}
}

func TestPutMediaNeedsStorageAndAPublicURL(t *testing.T) {
	t.Parallel()
	noStorage := newRig(fakeAuth{})
	noStorage.h.media = nil
	w := httptest.NewRecorder()
	noStorage.h.PutMedia(w, uploadRequest("1.jpg", "image/jpeg", jpegBody), "p1", "1.jpg")
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("no storage: status = %d, want 501", w.Code)
	}

	noURL := newRig(fakeAuth{})
	noURL.media.base = ""
	w = httptest.NewRecorder()
	noURL.h.PutMedia(w, uploadRequest("1.jpg", "image/jpeg", jpegBody), "p1", "1.jpg")
	if w.Code != http.StatusServiceUnavailable || len(noURL.media.puts) != 0 {
		t.Fatalf("no public URL: status = %d, puts = %d; want 503 and no upload", w.Code, len(noURL.media.puts))
	}
}

func TestPutMediaRefusesAPublishedPost(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.store.frozen["p1"] = true
	w := httptest.NewRecorder()
	rg.h.PutMedia(w, uploadRequest("1.jpg", "image/jpeg", jpegBody), "p1", "1.jpg")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for a frozen post", w.Code)
	}
}

func reconcileRequest(body string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/social/reconcile", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return authed(r)
}

func TestReconcileCancelsScheduledPostsThatLeftTheRepository(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.store.posts["keep"] = Post{ID: "keep", Status: StatusScheduled}
	rg.store.posts["gone"] = Post{ID: "gone", Status: StatusScheduled}
	rg.store.posts["live"] = Post{ID: "live", Status: StatusPublished}

	w := httptest.NewRecorder()
	rg.h.Reconcile(w, reconcileRequest(`{"ids":["keep"]}`))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !slices.Equal(rg.store.cancelled, []string{"gone"}) {
		t.Fatalf("cancelled = %v, want [gone]", rg.store.cancelled)
	}
	if rg.store.posts["live"].Status != StatusPublished {
		t.Fatal("reconcile must never touch a published post")
	}
}

func TestReconcileRequiresAnIDList(t *testing.T) {
	t.Parallel()
	rg := newRig(fakeAuth{})
	rg.store.posts["p"] = Post{ID: "p", Status: StatusScheduled}

	w := httptest.NewRecorder()
	rg.h.Reconcile(w, reconcileRequest(`{}`))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for a missing ids list", w.Code)
	}
	if rg.store.posts["p"].Status != StatusScheduled {
		t.Fatal("a body without ids must cancel nothing")
	}
}

var _ = errors.New
