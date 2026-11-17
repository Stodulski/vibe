package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/stores"
)

// The place waitlist join is reachable without a session or a CSRF token, and
// a repeated join answers the same empty 204, through the whole chain the
// storefront hits (spec validation, guards, tenant scoping).
func TestThePlaceWaitlistJoinIsPublicAndIdempotent(t *testing.T) {
	app, _ := newTestApplicationWithStores(t, func(s *stores.Stores) {
		s.Complexes = &mockComplexStore{GetBySlugFn: func(context.Context, string) (*complexstore.Complex, error) {
			return &complexstore.Complex{ID: uuid.New(), Name: "Vibe Padel", Slug: "vibe-padel", IsActive: true}, nil
		}}
	})
	ts := newTestServer(t, app)

	body := `{"email":"ana@example.com","kind":"open_play","event_id":"` + uuid.NewString() + `"}`
	for i := 0; i < 2; i++ {
		status, got := postJSON(t, ts, "/api/v1/public/complexes/vibe-padel/waitlist/places", body)
		if status != http.StatusNoContent || got != "" {
			t.Fatalf("join %d: want an empty 204; got %d (%s)", i+1, status, got)
		}
	}

	status, _ := postJSON(t, ts, "/api/v1/public/complexes/vibe-padel/waitlist/places", `{"email":"ana@example.com","kind":"nope"}`)
	if status != http.StatusUnprocessableEntity && status != http.StatusBadRequest {
		t.Fatalf("an unknown kind must be refused; got %d", status)
	}
}

// The notify sweep runs every minute under the cross-instance lock, and the
// cleanup daily: a job that is not registered never runs.
func TestThePlaceWaitlistJobsAreScheduled(t *testing.T) {
	app := newTestApplication(t)
	want := map[string]bool{"notify_place_waitlist": false, "clean_place_waitlist_entries": false}
	for _, j := range app.cronJobs() {
		if _, ok := want[j.Name]; ok {
			want[j.Name] = true
			if j.Local || j.Run == nil {
				t.Errorf("%s must take the cross-instance lock and run something", j.Name)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("%s is not scheduled", name)
		}
	}
}
