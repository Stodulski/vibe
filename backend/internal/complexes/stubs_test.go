package complexes

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
)

type stubStore struct {
	owned     []*data.Complex
	complex   *data.Complex
	schedules []*data.Schedule
	slugTaken bool
	slugErr   error
	getErr    error
	insertErr error

	// slugsWithPrefix are extra taken variants SlugsWithPrefix reports.
	slugsWithPrefix []string
	// slugsChecked is every slug SlugExists was asked about, in order.
	slugsChecked []string

	inserted          *data.Complex
	updated           *data.Complex
	updateErr         error
	softDeleted       *uuid.UUID
	courtsDeactivated int
	softDeleteErr     error
	upsertedSchedule  []*data.Schedule
	mpCredentials     *struct {
		access, refresh, user string
		expiresIn             int
	}
	mpCleared *uuid.UUID
}

func (s *stubStore) GetByOwner(context.Context, uuid.UUID) ([]*data.Complex, error) {
	return s.owned, s.getErr
}

func (s *stubStore) GetBySlug(context.Context, string) (*data.Complex, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.complex, nil
}

func (s *stubStore) GetSchedules(context.Context, uuid.UUID) ([]*data.Schedule, error) {
	return s.schedules, nil
}

func (s *stubStore) Insert(_ context.Context, c *data.Complex) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	c.ID = uuid.New()
	s.inserted = c
	return nil
}

func (s *stubStore) Update(_ context.Context, c *data.Complex) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = c
	return nil
}

func (s *stubStore) SoftDeleteCascade(_ context.Context, id uuid.UUID) (int, error) {
	s.softDeleted = &id
	return s.courtsDeactivated, s.softDeleteErr
}

// SlugExists records what it was asked. The stub used to discard the slug, so
// a test could not tell a check made against the normalised slug the handler
// is about to insert from one made against the raw input — or from no check at
// all that happened to return the right answer.
func (s *stubStore) SlugExists(_ context.Context, slug string) (bool, error) {
	s.slugsChecked = append(s.slugsChecked, slug)
	return s.slugTaken, s.slugErr
}

func (s *stubStore) SlugsWithPrefix(_ context.Context, base string) ([]string, error) {
	if s.slugTaken {
		return append([]string{base}, s.slugsWithPrefix...), s.slugErr
	}
	return s.slugsWithPrefix, s.slugErr
}

func (s *stubStore) UpsertSchedule(_ context.Context, sc *data.Schedule) error {
	s.upsertedSchedule = append(s.upsertedSchedule, sc)
	return nil
}

func (s *stubStore) UpdateMPCredentials(_ context.Context, _ uuid.UUID, access, refresh, user string, expiresIn int) error {
	s.mpCredentials = &struct {
		access, refresh, user string
		expiresIn             int
	}{access, refresh, user, expiresIn}
	return nil
}

func (s *stubStore) ClearMPCredentials(_ context.Context, id uuid.UUID) error {
	s.mpCleared = &id
	return nil
}

type stubCourts struct {
	courts []*data.Court
	prices []*data.CourtPrice
}

func (c *stubCourts) GetByComplex(context.Context, uuid.UUID) ([]*data.Court, error) {
	return c.courts, nil
}

func (c *stubCourts) GetPricesByCourtIDs(context.Context, []uuid.UUID) ([]*data.CourtPrice, error) {
	return c.prices, nil
}

type stubBookings struct {
	hasActive bool
	err       error
	cancelled *uuid.UUID
}

func (b *stubBookings) HasActiveBookings(context.Context, uuid.UUID) (bool, error) {
	return b.hasActive, b.err
}

func (b *stubBookings) CancelFutureByComplex(_ context.Context, id uuid.UUID) error {
	b.cancelled = &id
	return nil
}

type stubPayments struct {
	tokens *mp.OAuthTokens
	err    error
}

func (p *stubPayments) ExchangeOAuthCode(context.Context, string, string, string) (*mp.OAuthTokens, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.tokens, nil
}

type stubStorage struct {
	uploadURL, publicURL string
	err                  error
	deletedKeys          []string

	// signedKey and signedContentType record what the handler actually asked to
	// be signed. The stub used to discard every argument, so a test could not
	// tell a URL signed for the caller's declared type from one signed for a
	// hardcoded webp — and the handler hardcoded webp for both the key's
	// extension and the signed type while validating three.
	signedKey         string
	signedContentType string
}

func (s *stubStorage) GeneratePresignedPUT(_ context.Context, key, contentType string, _ int64, _ time.Duration) (string, string, error) {
	s.signedKey, s.signedContentType = key, contentType
	if s.err != nil {
		return "", "", s.err
	}
	return s.uploadURL, s.publicURL, nil
}

func (s *stubStorage) DeleteObject(_ context.Context, key string) error {
	s.deletedKeys = append(s.deletedKeys, key)
	return nil
}

func (s *stubStorage) KeyFromPublicURL(url string) (string, bool) {
	const prefix = "https://cdn.example/"
	if strings.HasPrefix(url, prefix) {
		return strings.TrimPrefix(url, prefix), true
	}
	return "", false
}

type stubRecorder struct{ entries []audit.Entry }

func (r *stubRecorder) Record(e audit.Entry) { r.entries = append(r.entries, e) }

type fixture struct {
	handler  *Handler
	store    *stubStore
	courts   *stubCourts
	bookings *stubBookings
	payments *stubPayments
	storage  *stubStorage
	audit    *stubRecorder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{
		store:    &stubStore{},
		courts:   &stubCourts{},
		bookings: &stubBookings{},
		payments: &stubPayments{},
		storage:  &stubStorage{uploadURL: "https://cdn.example/upload", publicURL: "https://cdn.example/logo.png"},
		audit:    &stubRecorder{},
	}
	responder := httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	f.handler = NewHandler(f.store, f.courts, f.bookings, f.payments, f.storage, f.audit, responder,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		func(fn func()) { fn() }, // run background work inline so tests can observe it
		Config{MaxComplexes: 4, FrontendURL: "https://vibe.test", MPAppID: "app-123"})
	return f
}

// ownerRequest builds a request from an authenticated owner. Pass a non-nil
// complexID to also put the owned complex in context, as the ownership
// middleware does.
func ownerRequest(t *testing.T, method, target string, ownerID uuid.UUID, complex *data.Complex, params map[string]string, body string) *http.Request {
	t.Helper()

	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	}

	r = httpx.ContextSetUser(r, &authstore.User{ID: ownerID, Role: "owner"})
	if complex != nil {
		r = httpx.ContextSetComplex(r, complex)
	}
	if len(params) > 0 {
		p := make(httprouter.Params, 0, len(params))
		for k, v := range params {
			p = append(p, httprouter.Param{Key: k, Value: v})
		}
		r = r.WithContext(context.WithValue(r.Context(), httprouter.ParamsKey, p))
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

// withSlug binds the :slug path parameter, as the router does for the public
// complex route.
func withSlug(r *http.Request, slug string) *http.Request {
	params := httprouter.Params{{Key: "slug", Value: slug}}
	return r.WithContext(context.WithValue(r.Context(), httprouter.ParamsKey, params))
}
