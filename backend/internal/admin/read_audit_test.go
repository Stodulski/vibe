package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	"github.com/stodulski/vibe-server/internal/audit"
)

// Every route in this module reads across tenants, and none of them used to
// leave a trace. The trail recorded what an operator changed and nothing about
// what they looked at — which is the question an operator is most likely to be
// asked about, and the one the table could not answer.

// seededStore is a store with a row behind every read, so a handler reaches its
// success path.
func seededStore() *stubStore {
	return &stubStore{
		stats:      &adminstore.PlatformStats{TotalUsers: 10},
		users:      []*adminstore.AdminUserRow{{ID: uuid.New()}, {ID: uuid.New()}},
		userDetail: &adminstore.AdminUserDetail{},
		complexes:  []*adminstore.AdminComplexRow{{ID: uuid.New()}},
		cxDetail:   &adminstore.AdminComplexDetail{},
		logs:       []*adminstore.AuditLogRow{{ID: uuid.New()}},
	}
}

// onlyEntry returns the single audit entry the handler wrote, failing when
// there is not exactly one.
func onlyEntry(t *testing.T, rec *stubRecorder) audit.Entry {
	t.Helper()
	if len(rec.entries) != 1 {
		t.Fatalf("want exactly one audit entry; got %d (%+v)", len(rec.entries), rec.entries)
	}
	return rec.entries[0]
}

func TestEveryAdminReadIsRecorded(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		withPathID     bool
		call           func(h *Handler, w http.ResponseWriter, r *http.Request)
		wantEntityType string
		// wantEntityID says the entry must name the record that was read.
		wantEntityID bool
		// wantComplexID says the entry must also land in that tenant's trail.
		wantComplexID bool
	}{
		{
			name:           "platform stats",
			target:         "/api/v1/admin/stats",
			call:           (*Handler).Stats,
			wantEntityType: "platform_stats",
		},
		{
			name:           "user directory",
			target:         "/api/v1/admin/users",
			call:           (*Handler).ListUsers,
			wantEntityType: "user",
		},
		{
			name:           "one account",
			target:         "/api/v1/admin/users/x",
			withPathID:     true,
			call:           (*Handler).GetUser,
			wantEntityType: "user",
			wantEntityID:   true,
		},
		{
			name:           "complex directory",
			target:         "/api/v1/admin/complexes",
			call:           (*Handler).ListComplexes,
			wantEntityType: "complex",
		},
		{
			name:           "one complex",
			target:         "/api/v1/admin/complexes/x",
			withPathID:     true,
			call:           (*Handler).GetComplex,
			wantEntityType: "complex",
			wantEntityID:   true,
			wantComplexID:  true,
		},
		{
			name:           "the trail itself",
			target:         "/api/v1/admin/audit-log",
			call:           (*Handler).ListAuditLogs,
			wantEntityType: "audit_log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, rec := newTestHandler(seededStore())
			operator, target := uuid.New(), uuid.New()

			var pathID *uuid.UUID
			if tt.withPathID {
				pathID = &target
			}

			w := httptest.NewRecorder()
			tt.call(h, w, operatorRequest(t, http.MethodGet, tt.target, operator, pathID, ""))

			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
			}

			got := onlyEntry(t, rec)
			if got.Action != "read" {
				t.Errorf("action = %q; want read", got.Action)
			}
			if got.EntityType != tt.wantEntityType {
				t.Errorf("entity type = %q; want %q", got.EntityType, tt.wantEntityType)
			}
			if got.UserID == nil || *got.UserID != operator {
				t.Errorf("the entry does not name the operator; got %v", got.UserID)
			}
			// Without the address, the entry cannot distinguish an operator at
			// their desk from a stolen session.
			if got.IPAddress == "" {
				t.Error("the entry does not record where the read came from")
			}

			switch {
			case tt.wantEntityID && (got.EntityID == nil || *got.EntityID != target):
				t.Errorf("the entry does not name the record read; got %v, want %s", got.EntityID, target)
			case !tt.wantEntityID && got.EntityID != nil:
				t.Errorf("a list read named a single entity: %v", got.EntityID)
			}

			switch {
			case tt.wantComplexID && (got.ComplexID == nil || *got.ComplexID != target):
				t.Errorf("a read of one tenant must land in that tenant's trail; got %v", got.ComplexID)
			case !tt.wantComplexID && got.ComplexID != nil:
				t.Errorf("a platform-wide read was filed under one complex: %v", got.ComplexID)
			}
		})
	}
}

// The entry has to say what was asked for. "Somebody read the user directory"
// and "somebody searched the user directory for this person's email" are very
// different records, and the second is the one worth keeping.
func TestAUserSearchRecordsWhatWasSearchedFor(t *testing.T) {
	h, _, rec := newTestHandler(seededStore())

	w := httptest.NewRecorder()
	h.ListUsers(w, operatorRequest(t, http.MethodGet,
		"/api/v1/admin/users?search=victim@example.com&role=owner", uuid.New(), nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	scope, ok := onlyEntry(t, rec).NewValue.(map[string]any)
	if !ok {
		t.Fatalf("the entry carries no scope; got %#v", rec.entries[0].NewValue)
	}
	if scope["search"] != "victim@example.com" {
		t.Errorf("the search term was not recorded; got %v", scope["search"])
	}
	if scope["role"] != "owner" {
		t.Errorf("the role filter was not recorded; got %v", scope["role"])
	}
	if scope["returned"] != 2 {
		t.Errorf("how much came back was not recorded; got %v", scope["returned"])
	}
}

// A read that failed is not a read.
func TestAFailedAdminReadIsNotRecorded(t *testing.T) {
	store := seededStore()
	store.err = errors.New("connection refused")
	h, _, rec := newTestHandler(store)

	w := httptest.NewRecorder()
	h.ListUsers(w, operatorRequest(t, http.MethodGet, "/api/v1/admin/users", uuid.New(), nil, ""))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500; got %d (%s)", w.Code, w.Body.String())
	}
	if len(rec.entries) != 0 {
		t.Errorf("a failed read was recorded as one that happened; got %+v", rec.entries)
	}
}

// A rejected request never reached the data, so it is not a read either.
func TestARejectedAdminReadIsNotRecorded(t *testing.T) {
	h, _, rec := newTestHandler(seededStore())

	w := httptest.NewRecorder()
	h.ListUsers(w, operatorRequest(t, http.MethodGet, "/api/v1/admin/users?limit=100000", uuid.New(), nil, ""))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if len(rec.entries) != 0 {
		t.Errorf("a rejected request was recorded as a read; got %+v", rec.entries)
	}
}
