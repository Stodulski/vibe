package clients

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

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// The doubles below implement Store and BookingReader — four methods between
// them. The shared mocks in cmd/api carry 32 methods across the same two
// entities, because they satisfy the full stores.ClientStore and
// stores.BookingStore whether a test needs them or not. This is what declaring
// the interface at the consumer buys.

type stubStore struct {
	client   *clientstore.Client
	list     []*clientstore.Client
	metadata data.Metadata

	getErr    error
	listErr   error
	updateErr error

	updated *clientstore.Client
	// lastSearch records what List forwarded to the store.
	lastSearch  string
	lastFilters data.Filters
}

func (s *stubStore) GetByID(context.Context, uuid.UUID) (*clientstore.Client, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.client, nil
}

func (s *stubStore) GetByComplex(_ context.Context, _ uuid.UUID, search string, filters data.Filters) ([]*clientstore.Client, data.Metadata, error) {
	s.lastSearch, s.lastFilters = search, filters
	if s.listErr != nil {
		return nil, data.Metadata{}, s.listErr
	}
	return s.list, s.metadata, nil
}

func (s *stubStore) Update(_ context.Context, c *clientstore.Client) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = c
	return nil
}

type stubBookings struct {
	bookings []*bookingstore.Booking
	err      error
	// lastLimit records the limit the handler asked for.
	lastLimit int
}

func (s *stubBookings) GetByClient(_ context.Context, _, _ uuid.UUID, limit int) ([]*bookingstore.Booking, error) {
	s.lastLimit = limit
	if s.err != nil {
		return nil, s.err
	}
	return s.bookings, nil
}

func testResponder() *httpx.Responder {
	return httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
}

// requestFor builds a request already carrying the owned complex and the
// clientID path parameter, as the middleware and router would supply them.
func requestFor(t *testing.T, method, target string, complexID, clientID uuid.UUID, body string) *http.Request {
	t.Helper()

	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	}

	r = httpx.ContextSetComplex(r, &complexstore.Complex{ID: complexID})
	if clientID != uuid.Nil {
		r = withParam(r, "clientID", clientID.String())
	}
	return r
}

func TestGetReturnsClientWithRecentBookings(t *testing.T) {
	complexID, clientID := uuid.New(), uuid.New()
	store := &stubStore{client: &clientstore.Client{ID: clientID, ComplexID: complexID, FirstName: "Ana"}}
	bookings := &stubBookings{bookings: []*bookingstore.Booking{{ID: uuid.New()}}}

	h := NewHandler(store, bookings, testResponder())
	w := httptest.NewRecorder()
	h.Get(w, requestFor(t, http.MethodGet, "/", complexID, clientID, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Client         map[string]any   `json:"client"`
		RecentBookings []map[string]any `json:"recent_bookings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body.Client["first_name"] != "Ana" {
		t.Errorf("client was not returned; got %v", body.Client)
	}
	if len(body.RecentBookings) != 1 {
		t.Errorf("want 1 recent booking; got %d", len(body.RecentBookings))
	}
	if bookings.lastLimit != recentBookingLimit {
		t.Errorf("want the recent-booking limit %d; got %d", recentBookingLimit, bookings.lastLimit)
	}
}

// A client belonging to another complex must read as missing, not forbidden:
// a 403 would confirm to the caller that the id exists.
func TestGetHidesClientsOfOtherComplexes(t *testing.T) {
	clientID := uuid.New()
	store := &stubStore{client: &clientstore.Client{ID: clientID, ComplexID: uuid.New()}}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.Get(w, requestFor(t, http.MethodGet, "/", uuid.New(), clientID, ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for a client under another complex; got %d", w.Code)
	}
}

func TestGetReportsMissingClientAsNotFound(t *testing.T) {
	store := &stubStore{getErr: data.ErrRecordNotFound}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.Get(w, requestFor(t, http.MethodGet, "/", uuid.New(), uuid.New(), ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestGetRejectsMalformedClientID(t *testing.T) {
	h := NewHandler(&stubStore{}, &stubBookings{}, testResponder())

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = httpx.ContextSetComplex(r, &complexstore.Complex{ID: uuid.New()})
	r = withParam(r, "clientID", "not-a-uuid")

	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for an unparseable id; got %d", w.Code)
	}
}

// Without the ownership middleware there is no complex to scope the query to,
// so the handler must fail loudly rather than read across complexes.
func TestHandlersRequireTheComplexInContext(t *testing.T) {
	h := NewHandler(&stubStore{}, &stubBookings{}, testResponder())

	for name, call := range map[string]http.HandlerFunc{"Get": h.Get, "Update": h.Update, "List": h.List} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			call(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

			if w.Code != http.StatusInternalServerError {
				t.Errorf("want 500 with no complex in context; got %d", w.Code)
			}
		})
	}
}

func TestUpdateAppliesOnlyTheFieldsSent(t *testing.T) {
	complexID, clientID := uuid.New(), uuid.New()
	existingNotes := "pays cash"
	store := &stubStore{client: &clientstore.Client{
		ID: clientID, ComplexID: complexID,
		Notes: &existingNotes, IsBlocked: false,
	}}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.Update(w, requestFor(t, http.MethodPut, "/", complexID, clientID, `{"is_blocked":true}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if store.updated == nil {
		t.Fatal("the client was never persisted")
	}
	if !store.updated.IsBlocked {
		t.Error("is_blocked was not applied")
	}
	// notes was omitted from the payload, so it must survive untouched.
	if store.updated.Notes == nil || *store.updated.Notes != existingNotes {
		t.Errorf("an omitted field was overwritten; notes = %v", store.updated.Notes)
	}
}

func TestUpdateRejectsMalformedBody(t *testing.T) {
	complexID, clientID := uuid.New(), uuid.New()
	store := &stubStore{client: &clientstore.Client{ID: clientID, ComplexID: complexID}}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.Update(w, requestFor(t, http.MethodPut, "/", complexID, clientID, `{"is_blocked":`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400; got %d", w.Code)
	}
	if store.updated != nil {
		t.Error("nothing must be persisted when the body is unreadable")
	}
}

func TestUpdateReportsALostRaceAsConflict(t *testing.T) {
	complexID, clientID := uuid.New(), uuid.New()
	store := &stubStore{
		client:    &clientstore.Client{ID: clientID, ComplexID: complexID},
		updateErr: data.ErrRecordNotFound,
	}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.Update(w, requestFor(t, http.MethodPut, "/", complexID, clientID, `{"is_blocked":true}`))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409 when the row vanished mid-update; got %d", w.Code)
	}
}

func TestListForwardsSearchAndPaging(t *testing.T) {
	complexID := uuid.New()
	store := &stubStore{list: []*clientstore.Client{{ID: uuid.New(), FirstName: "Ana"}}}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.List(w, requestFor(t, http.MethodGet, "/?search=ana&limit=10&cursor=abc", complexID, uuid.Nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if store.lastSearch != "ana" {
		t.Errorf("search was not forwarded; got %q", store.lastSearch)
	}
	if store.lastFilters.Limit != 10 || store.lastFilters.Cursor != "abc" {
		t.Errorf("paging was not forwarded; got %+v", store.lastFilters)
	}
}

func TestListDefaultsThePageSize(t *testing.T) {
	store := &stubStore{}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.List(w, requestFor(t, http.MethodGet, "/", uuid.New(), uuid.Nil, ""))

	if store.lastFilters.Limit != defaultPageLimit {
		t.Errorf("want the default limit %d; got %d", defaultPageLimit, store.lastFilters.Limit)
	}
}

func TestListRejectsAnInvalidCursor(t *testing.T) {
	store := &stubStore{listErr: data.ErrInvalidCursor}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.List(w, requestFor(t, http.MethodGet, "/?cursor=garbage", uuid.New(), uuid.Nil, ""))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for an unusable cursor; got %d", w.Code)
	}
}

func TestListRejectsAnOutOfRangeLimit(t *testing.T) {
	store := &stubStore{}

	h := NewHandler(store, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.List(w, requestFor(t, http.MethodGet, "/?limit=100000", uuid.New(), uuid.Nil, ""))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for a limit outside the allowed range; got %d", w.Code)
	}
}

func TestStoreFailuresBecomeServerErrors(t *testing.T) {
	boom := errors.New("connection refused")
	complexID, clientID := uuid.New(), uuid.New()

	t.Run("on read", func(t *testing.T) {
		h := NewHandler(&stubStore{getErr: boom}, &stubBookings{}, testResponder())
		w := httptest.NewRecorder()
		h.Get(w, requestFor(t, http.MethodGet, "/", complexID, clientID, ""))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("want 500; got %d", w.Code)
		}
		if strings.Contains(w.Body.String(), "connection refused") {
			t.Error("the underlying cause must not reach the client")
		}
	})

	t.Run("on the booking lookup", func(t *testing.T) {
		store := &stubStore{client: &clientstore.Client{ID: clientID, ComplexID: complexID}}
		h := NewHandler(store, &stubBookings{err: boom}, testResponder())
		w := httptest.NewRecorder()
		h.Get(w, requestFor(t, http.MethodGet, "/", complexID, clientID, ""))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("want 500; got %d", w.Code)
		}
	})

	t.Run("on list", func(t *testing.T) {
		h := NewHandler(&stubStore{listErr: boom}, &stubBookings{}, testResponder())
		w := httptest.NewRecorder()
		h.List(w, requestFor(t, http.MethodGet, "/", complexID, uuid.Nil, ""))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("want 500; got %d", w.Code)
		}
	})
}

// A complex with no clients returns 200 and an empty collection, not 404 —
// the collection exists, it is simply empty.
func TestListReturnsAnEmptyCollection(t *testing.T) {
	h := NewHandler(&stubStore{}, &stubBookings{}, testResponder())
	w := httptest.NewRecorder()
	h.List(w, requestFor(t, http.MethodGet, "/", uuid.New(), uuid.Nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d", w.Code)
	}

	var body struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if len(body.Clients) != 0 {
		t.Errorf("want no clients; got %d", len(body.Clients))
	}
}
