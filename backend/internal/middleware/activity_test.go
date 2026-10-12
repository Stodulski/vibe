package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
)

type recordingActivity struct {
	mu  sync.Mutex
	ids []uuid.UUID
}

func (r *recordingActivity) TouchOwner(id uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, id)
}

func (r *recordingActivity) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ids)
}

// An owner's authenticated request is reported, so the inactive nudge can tell
// who is still using the panel.
func TestAuthenticatedOwnerRequestIsReportedAsActivity(t *testing.T) {
	f := newFixture(t, Config{})
	activity := &recordingActivity{}
	f.mw.activity = activity

	userID := uuid.New()
	f.tokens.claims = validClaims(userID)
	f.users.user = &authstore.User{ID: userID, Role: "owner", IsActive: true}

	var reached bool
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	f.mw.Authenticate(ok(&reached)).ServeHTTP(httptest.NewRecorder(), r)

	if !reached {
		t.Fatal("the request did not reach the handler")
	}
	if activity.count() != 1 || activity.ids[0] != userID {
		t.Fatalf("activity = %v, want one entry for the owner", activity.ids)
	}
}

// The stamp is the owner's, so a superadmin's session does not count as an owner
// being active.
func TestNonOwnerRequestIsNotReportedAsActivity(t *testing.T) {
	f := newFixture(t, Config{})
	activity := &recordingActivity{}
	f.mw.activity = activity

	userID := uuid.New()
	f.tokens.claims = validClaims(userID)
	f.users.user = &authstore.User{ID: userID, Role: "superadmin", IsActive: true}

	var reached bool
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	f.mw.Authenticate(ok(&reached)).ServeHTTP(httptest.NewRecorder(), r)

	if activity.count() != 0 {
		t.Fatalf("a superadmin request was recorded as owner activity: %v", activity.ids)
	}
}

// An anonymous request has no owner, so nothing is recorded.
func TestAnonymousRequestIsNotReportedAsActivity(t *testing.T) {
	f := newFixture(t, Config{})
	activity := &recordingActivity{}
	f.mw.activity = activity

	var reached bool
	f.mw.Authenticate(ok(&reached)).ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/public/complexes/x", nil))

	if activity.count() != 0 {
		t.Fatal("an anonymous request was recorded as activity")
	}
}
