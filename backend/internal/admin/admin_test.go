package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	"github.com/stodulski/vibe-server/internal/audit"
	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

type stubStore struct {
	stats       *adminstore.PlatformStats
	users       []*adminstore.AdminUserRow
	userDetail  *adminstore.AdminUserDetail
	complexes   []*adminstore.AdminComplexRow
	cxDetail    *adminstore.AdminComplexDetail
	logs        []*auditstore.AuditLogRow
	err         error
	toggleErr   error
	toggledTo   *bool
	toggledUser uuid.UUID

	// lastSearch and lastRole record what the list handlers forwarded.
	lastSearch, lastRole, lastEntityType string
	lastComplexID                        *uuid.UUID
	lastFilters                          data.Filters
}

func (s *stubStore) GetPlatformStats(context.Context) (*adminstore.PlatformStats, error) {
	return s.stats, s.err
}

func (s *stubStore) ListUsers(_ context.Context, search, role string, f data.Filters) ([]*adminstore.AdminUserRow, data.Metadata, error) {
	s.lastSearch, s.lastRole, s.lastFilters = search, role, f
	return s.users, data.Metadata{}, s.err
}

func (s *stubStore) GetUserDetail(context.Context, uuid.UUID) (*adminstore.AdminUserDetail, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.userDetail, nil
}

func (s *stubStore) ListComplexes(_ context.Context, search string, f data.Filters) ([]*adminstore.AdminComplexRow, data.Metadata, error) {
	s.lastSearch, s.lastFilters = search, f
	return s.complexes, data.Metadata{}, s.err
}

func (s *stubStore) GetComplexDetail(context.Context, uuid.UUID) (*adminstore.AdminComplexDetail, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.cxDetail, nil
}

func (s *stubStore) ListAuditLogs(_ context.Context, complexID *uuid.UUID, entityType string, f data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error) {
	s.lastComplexID, s.lastEntityType, s.lastFilters = complexID, entityType, f
	return s.logs, data.Metadata{}, s.err
}

func (s *stubStore) ToggleUserActive(_ context.Context, userID uuid.UUID, isActive bool) error {
	if s.toggleErr != nil {
		return s.toggleErr
	}
	s.toggledUser, s.toggledTo = userID, &isActive
	return nil
}

type stubCache struct{ invalidated []uuid.UUID }

func (c *stubCache) InvalidateUser(_ context.Context, id uuid.UUID) {
	c.invalidated = append(c.invalidated, id)
}

type stubRecorder struct{ entries []audit.Entry }

func (r *stubRecorder) Record(e audit.Entry) { r.entries = append(r.entries, e) }

// newTestHandler builds the handler from one stub that plays both the admin
// store and the audit reader — they are two dependencies now, but one fake
// answers for both, which keeps every existing case unchanged.
func newTestHandler(store *stubStore) (*Handler, *stubCache, *stubRecorder) {
	cache, recorder := &stubCache{}, &stubRecorder{}
	responder := httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return NewHandler(NewService(store, store, cache, recorder), responder, false), cache, recorder
}

// operatorRequest builds a request from an authenticated superadmin, with the
// given path parameter bound.
func operatorRequest(t *testing.T, method, target string, operator uuid.UUID, pathID *uuid.UUID, body string) *http.Request {
	t.Helper()

	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	}

	r = httpx.ContextSetUser(r, &authstore.User{ID: operator, Role: "superadmin"})
	if pathID != nil {
		r.SetPathValue("id", pathID.String())
	}
	return r
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return body
}

func TestStatsReturnsThePlatformFigures(t *testing.T) {
	store := &stubStore{stats: &adminstore.PlatformStats{TotalUsers: 10}}
	h, _, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.Stats(w, operatorRequest(t, http.MethodGet, "/", uuid.New(), nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	stats, _ := decode(t, w)["stats"].(map[string]any)
	if stats["total_users"] != float64(10) {
		t.Errorf("want total_users 10; got %v", stats["total_users"])
	}
}

func TestListUsersForwardsSearchRoleAndPaging(t *testing.T) {
	store := &stubStore{}
	h, _, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.ListUsers(w, operatorRequest(t, http.MethodGet, "/?search=ana&role=owner&limit=10&cursor=abc", uuid.New(), nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d", w.Code)
	}
	if store.lastSearch != "ana" || store.lastRole != "owner" {
		t.Errorf("filters were not forwarded; search=%q role=%q", store.lastSearch, store.lastRole)
	}
	if store.lastFilters.Limit != 10 || store.lastFilters.Cursor != "abc" {
		t.Errorf("paging was not forwarded; got %+v", store.lastFilters)
	}
}

func TestListUsersDefaultsThePageSize(t *testing.T) {
	store := &stubStore{}
	h, _, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.ListUsers(w, operatorRequest(t, http.MethodGet, "/", uuid.New(), nil, ""))

	if store.lastFilters.Limit != defaultPageLimit {
		t.Errorf("want the default limit %d; got %d", defaultPageLimit, store.lastFilters.Limit)
	}
}

func TestGetUserReportsMissingAsNotFound(t *testing.T) {
	store := &stubStore{err: data.ErrRecordNotFound}
	h, _, _ := newTestHandler(store)

	id := uuid.New()
	w := httptest.NewRecorder()
	h.GetUser(w, operatorRequest(t, http.MethodGet, "/", uuid.New(), &id, ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestGetComplexReportsMissingAsNotFound(t *testing.T) {
	store := &stubStore{err: data.ErrRecordNotFound}
	h, _, _ := newTestHandler(store)

	id := uuid.New()
	w := httptest.NewRecorder()
	h.GetComplex(w, operatorRequest(t, http.MethodGet, "/", uuid.New(), &id, ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestToggleUserActivePersistsAuditsAndInvalidates(t *testing.T) {
	store := &stubStore{}
	h, cache, recorder := newTestHandler(store)

	target, operator := uuid.New(), uuid.New()
	w := httptest.NewRecorder()
	h.ToggleUserActive(w, operatorRequest(t, http.MethodPatch, "/", operator, &target, `{"is_active":false}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if store.toggledTo == nil || *store.toggledTo {
		t.Error("the account was not deactivated")
	}
	if store.toggledUser != target {
		t.Errorf("the wrong account was changed; got %s", store.toggledUser)
	}

	// The cached user record carries is_active, so a stale copy would keep a
	// disabled account working until the entry expired.
	if len(cache.invalidated) != 1 || cache.invalidated[0] != target {
		t.Errorf("the target's cache entry was not invalidated; got %v", cache.invalidated)
	}

	if len(recorder.entries) != 1 {
		t.Fatalf("want one audit entry; got %d", len(recorder.entries))
	}
	entry := recorder.entries[0]
	if entry.Action != "toggle_active" || entry.EntityType != "user" {
		t.Errorf("the audit entry is wrong: %+v", entry)
	}
	if entry.UserID == nil || *entry.UserID != operator {
		t.Errorf("the audit entry must name the operator who acted; got %v", entry.UserID)
	}
	if entry.EntityID == nil || *entry.EntityID != target {
		t.Errorf("the audit entry must name the account changed; got %v", entry.EntityID)
	}
}

// An operator locking themselves out would need another superadmin to recover,
// so it is refused rather than merely discouraged.
func TestOperatorCannotDeactivateThemselves(t *testing.T) {
	store := &stubStore{}
	h, cache, recorder := newTestHandler(store)

	operator := uuid.New()
	w := httptest.NewRecorder()
	h.ToggleUserActive(w, operatorRequest(t, http.MethodPatch, "/", operator, &operator, `{"is_active":false}`))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if store.toggledTo != nil {
		t.Error("nothing must be persisted")
	}
	if len(cache.invalidated) != 0 || len(recorder.entries) != 0 {
		t.Error("a refused change must leave no audit entry and invalidate nothing")
	}
}

func TestToggleUserActiveRejectsAMalformedBody(t *testing.T) {
	store := &stubStore{}
	h, _, _ := newTestHandler(store)

	target := uuid.New()
	w := httptest.NewRecorder()
	h.ToggleUserActive(w, operatorRequest(t, http.MethodPatch, "/", uuid.New(), &target, `{"is_active":`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400; got %d", w.Code)
	}
	if store.toggledTo != nil {
		t.Error("nothing must be persisted when the body is unreadable")
	}
}

func TestStoreFailuresBecomeServerErrors(t *testing.T) {
	boom := errors.New("connection refused")
	h, _, _ := newTestHandler(&stubStore{err: boom})

	for name, call := range map[string]http.HandlerFunc{
		"stats":     h.Stats,
		"users":     h.ListUsers,
		"complexes": h.ListComplexes,
		"audit log": h.ListAuditLogs,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			call(w, operatorRequest(t, http.MethodGet, "/", uuid.New(), nil, ""))

			if w.Code != http.StatusInternalServerError {
				t.Errorf("want 500; got %d", w.Code)
			}
			if strings.Contains(w.Body.String(), "connection refused") {
				t.Error("the underlying cause must not reach the client")
			}
		})
	}
}
