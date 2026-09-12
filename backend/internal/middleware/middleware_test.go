package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/auth"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// TestMain silences go-redis's package-level logger before any test runs. It
// is a global, so setting it from inside a test races with the connection
// goroutines an earlier test left behind.
func TestMain(m *testing.M) {
	redis.SetLogger(quietRedisLogger{})
	os.Exit(m.Run())
}

// errUnexpectedStoreRead marks a store read that a test asserts must not
// happen — a cache hit that still went to the database, for instance.
var errUnexpectedStoreRead = errors.New("the store was read when it should not have been")

type stubUsers struct {
	user *authstore.User
	err  error
	// deadline records the budget the caller gave this read, so a test can
	// assert the chain does not run it on the bare request context.
	deadline time.Duration
}

func (s *stubUsers) GetByID(ctx context.Context, _ uuid.UUID) (*authstore.User, error) {
	s.deadline = budget(ctx)
	if s.err != nil {
		return nil, s.err
	}
	if s.user == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.user, nil
}

type stubComplexes struct {
	complex  *complexstore.Complex
	err      error
	deadline time.Duration
}

func (s *stubComplexes) GetByID(ctx context.Context, _ uuid.UUID) (*complexstore.Complex, error) {
	s.deadline = budget(ctx)
	if s.err != nil {
		return nil, s.err
	}
	if s.complex == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.complex, nil
}

type stubTokens struct {
	claims    *auth.Claims
	err       error
	csrfValid bool
}

func (s *stubTokens) ValidateAccessToken(string) (*auth.Claims, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.claims, nil
}

func (s *stubTokens) ValidateCSRFToken(string, string) bool { return s.csrfValid }

type stubBlacklist struct{ revoked bool }

func (s *stubBlacklist) IsBlacklisted(context.Context, string, uuid.UUID, time.Time) bool {
	return s.revoked
}

type fixture struct {
	mw        *Middleware
	users     *stubUsers
	complexes *stubComplexes
	tokens    *stubTokens
	blacklist *stubBlacklist
	logs      *bytes.Buffer
}

func newFixture(t *testing.T, cfg Config) *fixture {
	t.Helper()
	return newFixtureWith(t, cfg, nil)
}

// newFixtureWith builds the chain against a Redis client, so the production
// branch of the limiter and the whole user cache can be driven.
func newFixtureWith(t *testing.T, cfg Config, rdb *redis.Client) *fixture {
	t.Helper()

	f := &fixture{
		users: &stubUsers{}, complexes: &stubComplexes{},
		tokens: &stubTokens{csrfValid: true}, blacklist: &stubBlacklist{},
		logs: &bytes.Buffer{},
	}
	logger := slog.New(slog.NewTextHandler(f.logs, nil))
	f.mw = New(Dependencies{
		Users: f.users, Complexes: f.complexes, Tokens: f.tokens, Blacklist: f.blacklist,
		Redis:   rdb,
		Respond: httpx.NewResponder(logger), Logger: logger, Shutdown: make(chan struct{}),
	}, cfg)
	return f
}

// budget is how long a context has left, or zero when it has no deadline at
// all — which is the state the chain's database reads used to run in.
func budget(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	return time.Until(deadline)
}

// ok is a handler that records that it was reached.
func ok(reached *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	}
}

func TestRecoverPanicTurnsAPanicIntoAServerError(t *testing.T) {
	f := newFixture(t, Config{})

	handler := f.mw.RecoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("something went wrong")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500; got %d", w.Code)
	}
	// A panic message can carry anything, including a query or a token.
	if bytes.Contains(w.Body.Bytes(), []byte("something went wrong")) {
		t.Errorf("the panic value must not reach the client; got %s", w.Body.String())
	}
	// Connection: close tells the client this connection is not reusable after
	// a panic unwound mid-response.
	if w.Header().Get("Connection") != "close" {
		t.Errorf("want Connection: close; got %q", w.Header().Get("Connection"))
	}
}

func TestSecurityHeadersAreSet(t *testing.T) {
	f := newFixture(t, Config{})
	var reached bool

	w := httptest.NewRecorder()
	f.mw.SecurityHeaders(ok(&reached)).ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	for _, header := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if w.Header().Get(header) == "" {
			t.Errorf("%s was not set", header)
		}
	}
	if !reached {
		t.Error("the request must still reach the handler")
	}
}

func TestRequestIDIsMintedAndEchoed(t *testing.T) {
	f := newFixture(t, Config{})

	var seen string
	handler := f.mw.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = httpx.ContextGetRequestID(r)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if seen == "" {
		t.Error("the request id must be in the context for error logs")
	}
	if got := w.Header().Get("X-Request-ID"); got != seen {
		t.Errorf("the echoed id must match the context one; got %q vs %q", got, seen)
	}
}

// Anonymous requests pass through: some routes are public. RequireAuth is what
// makes a route private.
func TestAuthenticateLetsAnonymousRequestsThrough(t *testing.T) {
	f := newFixture(t, Config{})
	var reached bool

	w := httptest.NewRecorder()
	f.mw.Authenticate(ok(&reached)).ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if !reached {
		t.Error("an anonymous request must reach the handler")
	}
	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d", w.Code)
	}
}

func TestAuthenticateSetsTheUserForAValidToken(t *testing.T) {
	f := newFixture(t, Config{})
	userID := uuid.New()
	f.tokens.claims = validClaims(userID)
	f.users.user = &authstore.User{ID: userID, Email: "ana@example.com", Role: "owner", IsActive: true}

	var got *authstore.User
	handler := f.mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = httpx.ContextGetAuthenticatedUser(r)
	}))

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer some-token")

	handler.ServeHTTP(httptest.NewRecorder(), r)

	if got == nil || got.ID != userID {
		t.Errorf("the authenticated user was not put in the context; got %v", got)
	}
}

// Authenticate never rejects. Every unusable credential — a bad signature, a
// revoked token, an unknown or deactivated account — falls through as
// anonymous, because the route may be public. RequireAuth is the layer that
// turns "no identity" into a 401, and the two together are what protects a
// route.
func TestAuthenticateNeverRejects(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fixture)
	}{
		{"invalid signature", func(f *fixture) { f.tokens.err = errors.New("bad signature") }},
		{"revoked token", func(f *fixture) {
			f.tokens.claims = validClaims(uuid.New())
			f.blacklist.revoked = true
		}},
		{"unknown account", func(f *fixture) {
			f.tokens.claims = validClaims(uuid.New())
		}},
		{"deactivated account", func(f *fixture) {
			id := uuid.New()
			f.tokens.claims = validClaims(id)
			f.users.user = &authstore.User{ID: id, IsActive: false}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, Config{})
			tt.setup(f)

			var user *authstore.User
			handler := f.mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, _ = httpx.ContextGetAuthenticatedUser(r)
				w.WriteHeader(http.StatusOK)
			}))

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			r.Header.Set("Authorization", "Bearer some-token")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("Authenticate must not reject; got %d (%s)", w.Code, w.Body.String())
			}
			if user != nil {
				t.Errorf("no user may be set for an unusable credential; got %v", user.ID)
			}
		})
	}
}

// The pair is what protects a route: identify permissively, then require.
func TestAuthenticateAndRequireAuthTogetherReject(t *testing.T) {
	f := newFixture(t, Config{})
	f.tokens.err = errors.New("bad signature")

	var reached bool
	handler := f.mw.Authenticate(f.mw.RequireAuth(ok(&reached)))

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer forged")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401; got %d", w.Code)
	}
	if reached {
		t.Error("the handler must not be reached")
	}
}

// A header that is not a Bearer token is treated as no credentials at all,
// rather than as an attempt: the route may be public, and RequireAuth is what
// decides.
func TestAuthenticateIgnoresANonBearerHeader(t *testing.T) {
	f := newFixture(t, Config{})
	var reached bool

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	w := httptest.NewRecorder()
	f.mw.Authenticate(ok(&reached)).ServeHTTP(w, r)

	if !reached {
		t.Errorf("want the request treated as anonymous; got %d", w.Code)
	}
}

// validClaims builds claims for a user id, as a verified token would carry.
func validClaims(userID uuid.UUID) *auth.Claims {
	return &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  userID.String(),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
		Role: "owner",
	}
}

func TestRequireAuthRejectsAnonymousRequests(t *testing.T) {
	f := newFixture(t, Config{})
	var reached bool

	w := httptest.NewRecorder()
	f.mw.RequireAuth(ok(&reached))(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401; got %d", w.Code)
	}
	if reached {
		t.Error("the handler must not be reached")
	}
}

func TestRequireRoleRejectsTheWrongRole(t *testing.T) {
	f := newFixture(t, Config{})
	var reached bool

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = httpx.ContextSetUser(r, &authstore.User{ID: uuid.New(), Role: "owner"})

	w := httptest.NewRecorder()
	f.mw.RequireRole("superadmin")(ok(&reached))(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403; got %d", w.Code)
	}
	if reached {
		t.Error("the handler must not be reached")
	}
}

// The ownership check answers 403 for a complex the caller does not own.
//
// Note the tension with the domain handlers, which answer 404 for a resource
// under another complex precisely so the response does not confirm the id
// exists. Here a 403 does confirm it. Complex ids are UUIDs, so enumeration is
// impractical and this is recorded rather than changed — but the two layers
// disagree, and that is worth knowing.
func TestRequireComplexOwnerRefusesAnotherOwnersComplex(t *testing.T) {
	f := newFixture(t, Config{})
	complexID := uuid.New()
	f.complexes.complex = &complexstore.Complex{ID: complexID, OwnerID: uuid.New()}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = httpx.ContextSetUser(r, &authstore.User{ID: uuid.New(), Role: "owner"})
	r = withComplexParam(r, complexID)

	var reached bool
	w := httptest.NewRecorder()
	f.mw.RequireComplexOwner(ok(&reached))(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403; got %d (%s)", w.Code, w.Body.String())
	}
	if reached {
		t.Error("the handler must not be reached")
	}
}

func TestRequireComplexOwnerPutsTheComplexInContext(t *testing.T) {
	f := newFixture(t, Config{})
	ownerID, complexID := uuid.New(), uuid.New()
	f.complexes.complex = &complexstore.Complex{ID: complexID, OwnerID: ownerID}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = httpx.ContextSetUser(r, &authstore.User{ID: ownerID, Role: "owner"})
	r = withComplexParam(r, complexID)

	var got *complexstore.Complex
	w := httptest.NewRecorder()
	f.mw.RequireComplexOwner(func(_ http.ResponseWriter, req *http.Request) {
		got, _ = httpx.ContextGetComplex(req)
	})(w, r)

	if got == nil || got.ID != complexID {
		t.Errorf("the owned complex must be in the context so the handler need not reload it; got %v", got)
	}
}

// Ownership is checked by owner id alone: the superadmin role does not bypass
// it. Operators reach cross-tenant data through the admin routes, which are
// behind RequireRole instead — so a superadmin cannot act inside a complex as
// if they owned it.
func TestRequireComplexOwnerDoesNotExemptASuperadmin(t *testing.T) {
	f := newFixture(t, Config{})
	complexID := uuid.New()
	f.complexes.complex = &complexstore.Complex{ID: complexID, OwnerID: uuid.New()}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = httpx.ContextSetUser(r, &authstore.User{ID: uuid.New(), Role: "superadmin"})
	r = withComplexParam(r, complexID)

	var reached bool
	w := httptest.NewRecorder()
	f.mw.RequireComplexOwner(ok(&reached))(w, r)

	if reached {
		t.Error("the superadmin role must not stand in for ownership")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403; got %d", w.Code)
	}
}

// Reads cannot change anything, so they carry no CSRF requirement.
func TestCSRFIgnoresSafeMethods(t *testing.T) {
	f := newFixture(t, Config{})
	f.tokens.csrfValid = false

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			var reached bool
			w := httptest.NewRecorder()
			f.mw.CSRFProtect(ok(&reached)).ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), method, "/", nil))

			if !reached {
				t.Errorf("%s must not require a CSRF token", method)
			}
		})
	}
}

// withComplexParam binds the {id} route parameter, as the router does.
func withComplexParam(r *http.Request, id uuid.UUID) *http.Request {
	r.SetPathValue("id", id.String())
	return r
}
