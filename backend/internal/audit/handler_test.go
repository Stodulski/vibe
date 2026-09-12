package audit

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
	"time"

	"github.com/google/uuid"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// stubReader answers with whatever the test queued and records how it was
// asked.
//
// It records the scope and filters rather than only the fact of being called:
// the whole point of this endpoint is that the tenant scope comes from the
// verified complex and not from the request, and a reader that discarded its
// arguments could not tell the two apart.
type stubReader struct {
	logs []*adminstore.AuditLogRow
	meta data.Metadata
	err  error

	called     int
	gotComplex *uuid.UUID
	gotEntity  string
	gotFilters data.Filters
}

func (s *stubReader) ListAuditLogs(_ context.Context, complexID *uuid.UUID, entityType string,
	filters data.Filters,
) ([]*adminstore.AuditLogRow, data.Metadata, error) {
	s.called++
	if complexID != nil {
		id := *complexID
		s.gotComplex = &id
	} else {
		s.gotComplex = nil
	}
	s.gotEntity = entityType
	s.gotFilters = filters
	return s.logs, s.meta, s.err
}

type trailFixture struct {
	handler *Handler
	reader  *stubReader
	store   *stubStore
	complex *complexstore.Complex
	caller  *authstore.User
}

func newTrailFixture(t *testing.T) *trailFixture {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	f := &trailFixture{
		reader:  &stubReader{},
		store:   &stubStore{},
		complex: &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo"},
		caller:  &authstore.User{ID: uuid.New(), Role: "owner"},
	}
	// The real Recorder, over a store that captures what was written: the
	// entry is asserted as it reaches persistence, encoding included.
	recorder := NewRecorder(f.store, logger, runInline)
	f.handler = NewHandler(f.reader, recorder, httpx.NewResponder(logger), false)
	return f
}

// request builds a call from the owner, with the complex the ownership guard
// verified already in the context.
func (f *trailFixture) request(t *testing.T, target string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	r.RemoteAddr = "203.0.113.7:41234"
	r = httpx.ContextSetUser(r, f.caller)
	return httpx.ContextSetComplex(r, f.complex)
}

func (f *trailFixture) row() *adminstore.AuditLogRow {
	return &adminstore.AuditLogRow{
		ID:         uuid.New(),
		UserID:     &f.caller.ID,
		ComplexID:  &f.complex.ID,
		Action:     "cancel",
		EntityType: "booking",
		CreatedAt:  time.Now(),
	}
}

// The trail of a venue was readable by the platform and not by the venue.
func TestTenantReadsItsOwnTrail(t *testing.T) {
	f := newTrailFixture(t)
	f.reader.logs = []*adminstore.AuditLogRow{f.row(), f.row()}

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		AuditLogs []struct {
			Action     string `json:"action"`
			EntityType string `json:"entity_type"`
		} `json:"audit_logs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	if len(body.AuditLogs) != 2 {
		t.Fatalf("want the two entries the store returned; got %d (%s)", len(body.AuditLogs), w.Body.String())
	}
	if body.AuditLogs[0].Action != "cancel" || body.AuditLogs[0].EntityType != "booking" {
		t.Errorf("the rows are not the ones the store returned; got %+v", body.AuditLogs[0])
	}
}

// The scope is the complex the ownership guard verified. A complex_id in the
// query string is a caller naming somebody else's history, and must not reach
// the query.
func TestTrailIsScopedToTheVerifiedComplexNotTheQueryString(t *testing.T) {
	f := newTrailFixture(t)
	other := uuid.New()

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log?complex_id="+other.String()))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.reader.called != 1 {
		t.Fatalf("want exactly one read; got %d", f.reader.called)
	}
	if f.reader.gotComplex == nil {
		t.Fatal("the trail was read unscoped — that is every tenant's history")
	}
	if *f.reader.gotComplex != f.complex.ID {
		t.Errorf("read scoped to %s; want the verified complex %s", *f.reader.gotComplex, f.complex.ID)
	}
}

// The filters the caller may set are the ones that shape their own page.
func TestTrailPassesTheCallersFiltersThrough(t *testing.T) {
	f := newTrailFixture(t)

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log?entity_type=court&limit=7&cursor=abc"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.reader.gotEntity != "court" {
		t.Errorf("entity_type reached the query as %q; want court", f.reader.gotEntity)
	}
	if f.reader.gotFilters.Limit != 7 {
		t.Errorf("limit reached the query as %d; want 7", f.reader.gotFilters.Limit)
	}
	if f.reader.gotFilters.Cursor != "abc" {
		t.Errorf("cursor reached the query as %q; want abc", f.reader.gotFilters.Cursor)
	}
}

// Without a limit the page has to be bounded by something, or one request
// walks the whole table.
func TestTrailAppliesADefaultPageSize(t *testing.T) {
	f := newTrailFixture(t)

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.reader.gotFilters.Limit != defaultPageSize {
		t.Errorf("default limit = %d; want %d", f.reader.gotFilters.Limit, defaultPageSize)
	}
}

// The person who can read the trail must leave a record of having read it.
func TestReadingTheTrailIsItselfRecorded(t *testing.T) {
	f := newTrailFixture(t)
	f.reader.logs = []*adminstore.AuditLogRow{f.row(), f.row(), f.row()}

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log?entity_type=booking"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.calls) != 1 {
		t.Fatalf("want exactly one audit entry for the read; got %d", len(f.store.calls))
	}

	got := f.store.calls[0]
	if got.action != "read" {
		t.Errorf("action = %q; want read", got.action)
	}
	if got.entityType != "audit_log" {
		t.Errorf("entity type = %q; want audit_log", got.entityType)
	}
	if got.userID == nil || *got.userID != f.caller.ID {
		t.Errorf("the entry does not name the reader; got %v", got.userID)
	}
	if got.complexID == nil || *got.complexID != f.complex.ID {
		t.Errorf("the entry does not name the complex read; got %v", got.complexID)
	}
	if got.ipAddr != "203.0.113.7" {
		t.Errorf("the entry does not record where the read came from; got %q", got.ipAddr)
	}
	// What was read, not merely that something was: an entry that records a
	// read of the whole trail identically to a read of one booking's history
	// is not much of a record.
	var scope map[string]any
	if err := json.Unmarshal(got.newJSON, &scope); err != nil {
		t.Fatalf("the entry's value is not valid JSON: %v (%q)", err, got.newJSON)
	}
	if scope["returned"] != float64(3) {
		t.Errorf("the entry does not say how much was read; got %v", scope["returned"])
	}
	if scope["entity_type"] != "booking" {
		t.Errorf("the entry does not record the filter that was applied; got %v", scope["entity_type"])
	}
}

// A read that failed is not a read: recording one would put an event in the
// trail that never happened.
func TestAFailedReadIsNotRecorded(t *testing.T) {
	f := newTrailFixture(t)
	f.reader.err = errors.New("connection refused")

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.calls) != 0 {
		t.Errorf("a failed read was recorded as one that happened; got %+v", f.store.calls)
	}
}

func TestTrailRejectsAnUnusableLimit(t *testing.T) {
	f := newTrailFixture(t)

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log?limit=100000"))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for a limit past the maximum; got %d (%s)", w.Code, w.Body.String())
	}
	if f.reader.called != 0 {
		t.Error("an invalid page size still reached the database")
	}
}

func TestTrailReportsABadCursorAsABadRequest(t *testing.T) {
	f := newTrailFixture(t)
	f.reader.err = data.ErrInvalidCursor

	w := httptest.NewRecorder()
	f.handler.List(w, f.request(t, "/api/v1/complexes/x/audit-log?cursor=nonsense"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for a malformed cursor; got %d (%s)", w.Code, w.Body.String())
	}
}

// The endpoint is complex-scoped, so it cannot run without one: reaching the
// store with a nil scope would serve every tenant's history.
func TestTrailRefusesWithoutAVerifiedComplex(t *testing.T) {
	f := newTrailFixture(t)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/complexes/x/audit-log", nil)
	r = httpx.ContextSetUser(r, f.caller)

	w := httptest.NewRecorder()
	f.handler.List(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 with no complex in context; got %d", w.Code)
	}
	if f.reader.called != 0 {
		t.Error("the trail was read with no verified complex to scope it to")
	}
}

// The route the module registers is the one the frontend calls, and it is
// behind the ownership guard rather than a bare session.
func TestTrailRegistersTheOwnerGuardedRoute(t *testing.T) {
	f := newTrailFixture(t)

	rec := &routeRecorder{}
	var applied []string
	mark := func(name string) httpx.Guard {
		return func(next http.HandlerFunc) http.HandlerFunc {
			applied = append(applied, name)
			return next
		}
	}

	f.handler.Routes(rec, httpx.Guards{
		RequireAuth:         mark("RequireAuth"),
		RequireComplexOwner: mark("RequireComplexOwner"),
		RequireSuperAdmin:   mark("RequireSuperAdmin"),
	})

	want := "GET /api/v1/complexes/:id/audit-log"
	if len(rec.routes) != 1 || rec.routes[0] != want {
		t.Fatalf("want %q registered; got %v", want, rec.routes)
	}
	joined := strings.Join(applied, ",")
	if !strings.Contains(joined, "RequireComplexOwner") || !strings.Contains(joined, "RequireAuth") {
		t.Errorf("the tenant trail must be behind auth and ownership; guards applied: %v", applied)
	}
	if strings.Contains(joined, "RequireSuperAdmin") {
		t.Errorf("the tenant trail must not be superadmin-only; guards applied: %v", applied)
	}
}

// routeRecorder is the httpx.Router a test passes to Routes to read back what
// was registered.
type routeRecorder struct{ routes []string }

func (r *routeRecorder) HandlerFunc(method, path string, _ http.HandlerFunc) {
	r.routes = append(r.routes, method+" "+path)
}

func (r *routeRecorder) Handler(method, path string, _ http.Handler) {
	r.routes = append(r.routes, method+" "+path)
}
