package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/realtime"
)

// The stream authorizer is the only thing standing between a revoked session
// and a live feed of a venue's bookings, and the way it can fail is quiet: it
// is handed the very request whose context already holds the user and the
// complex the connect-time chain approved. A re-check that reads those values
// back re-confirms the decision it was built to distrust.

type streamAuthzFixture struct {
	app       *application
	users     *mockUserStore
	complexes *mockComplexStore
	owner     *authstore.User
	complex   *data.Complex
	// req is the stream's original request: authenticated, owner-approved, and
	// carrying both of those in its context, exactly as Stream receives it.
	req *http.Request
}

func newStreamAuthzFixture(t *testing.T) *streamAuthzFixture {
	t.Helper()

	app := newTestApplication(t)

	users, ok := app.models.Users.(*mockUserStore)
	if !ok {
		t.Fatalf("user store is %T, not *mockUserStore", app.models.Users)
	}
	complexes, ok := app.models.Complexes.(*mockComplexStore)
	if !ok {
		t.Fatalf("complex store is %T, not *mockComplexStore", app.models.Complexes)
	}

	owner := users.seed(authstore.User{
		ID:            uuid.New(),
		Email:         "owner@example.com",
		FirstName:     "Owner",
		LastName:      "Account",
		Phone:         "+5491112345678",
		Role:          "owner",
		IsActive:      true,
		EmailVerified: true,
	})
	complex := complexes.seed(data.Complex{
		ID:          uuid.New(),
		OwnerID:     owner.ID,
		Name:        "Complejo",
		Slug:        "complejo",
		IsActive:    true,
		CountryCode: "AR",
		Currency:    "ARS",
	})

	token, err := app.tokens.GenerateAccessToken(owner.ID, owner.Role)
	if err != nil {
		t.Fatalf("minting an access token: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		fmt.Sprintf("/api/v1/complexes/%s/events", complex.ID), nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	// What the connect-time chain leaves behind, and what a lazy re-check
	// would read instead of asking again.
	req = httpx.ContextSetUser(req, owner)
	req = httpx.ContextSetComplex(req, complex)

	return &streamAuthzFixture{
		app: app, users: users, complexes: complexes,
		owner: owner, complex: complex, req: req,
	}
}

// authorize calls Authorize the way stream.go's own re-check loop does:
// stream.go derives the ctx argument from the stream's ctx, which is
// r.Context() read once at Stream()'s entry — so ctx is always a descendant
// of the connect-time request's context, carrying whatever RequireAuth and
// RequireComplexOwner put there when the stream was first authorized. A test
// that instead passed an unrelated, unauthenticated context here (t.Context())
// would never exercise H-11 at all: the leak is specifically that ctx and r
// still share that stale ancestry, and Authorize's job is to build a probe
// that does not.
func (f *streamAuthzFixture) authorize(t *testing.T, complexID uuid.UUID) error {
	t.Helper()
	return streamAuthorizer{mw: f.app.middleware}.Authorize(f.req.Context(), f.req, complexID)
}

func TestStreamAuthorizerAllowsTheOwnerWhoOpenedTheStream(t *testing.T) {
	f := newStreamAuthzFixture(t)

	if err := f.authorize(t, f.complex.ID); err != nil {
		t.Errorf("the owner of an open stream was refused: %v", err)
	}
}

// The account is deactivated after the stream opened. The request's context
// still says otherwise.
func TestStreamAuthorizerRefusesADeactivatedAccount(t *testing.T) {
	f := newStreamAuthzFixture(t)

	deactivated := *f.owner
	deactivated.IsActive = false
	f.users.seed(deactivated)

	err := f.authorize(t, f.complex.ID)
	if !errors.Is(err, realtime.ErrStreamUnauthorized) {
		t.Errorf("want the stream refused for a deactivated account; got %v", err)
	}
}

// The venue changed hands after the stream opened.
func TestStreamAuthorizerRefusesAnOwnerWhoNoLongerOwnsTheComplex(t *testing.T) {
	f := newStreamAuthzFixture(t)

	transferred := *f.complex
	transferred.OwnerID = uuid.New()
	f.complexes.seed(transferred)

	err := f.authorize(t, f.complex.ID)
	if !errors.Is(err, realtime.ErrStreamUnauthorized) {
		t.Errorf("want the stream refused after the complex changed hands; got %v", err)
	}
}

// The session was signed out: the token is blacklisted.
func TestStreamAuthorizerRefusesASignedOutSession(t *testing.T) {
	f := newStreamAuthzFixture(t)

	cookie, err := f.req.Cookie("access_token")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := f.app.tokens.ValidateAccessToken(cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if blErr := f.app.blacklist.BlacklistToken(t.Context(), cookie.Value, claims.ExpiresAt.Time); blErr != nil {
		t.Fatalf("blacklisting the token: %v", blErr)
	}

	if err := f.authorize(t, f.complex.ID); !errors.Is(err, realtime.ErrStreamUnauthorized) {
		t.Errorf("want the stream refused for a signed-out session; got %v", err)
	}
}

// The subject of the re-check is the stream's own complex, not whatever the
// request's URL says: the stream was opened for one venue and must be
// re-checked against that one.
func TestStreamAuthorizerChecksTheStreamsComplexNotTheRequestURL(t *testing.T) {
	f := newStreamAuthzFixture(t)

	foreign := f.complexes.seed(data.Complex{
		ID:       uuid.New(),
		OwnerID:  uuid.New(),
		Name:     "Ajeno",
		Slug:     "ajeno",
		IsActive: true,
	})

	if err := f.authorize(t, foreign.ID); !errors.Is(err, realtime.ErrStreamUnauthorized) {
		t.Errorf("want a complex this caller does not own refused; got %v", err)
	}
}

// A store that cannot answer is not a denial. Reporting it as one would
// disconnect every dashboard on a database blip.
func TestStreamAuthorizerReportsAnUnverifiableCheckSeparately(t *testing.T) {
	f := newStreamAuthzFixture(t)
	f.complexes.GetByIDFn = func(context.Context, uuid.UUID) (*data.Complex, error) {
		return nil, errors.New("connection refused")
	}

	err := f.authorize(t, f.complex.ID)
	if err == nil {
		t.Fatal("want an error when the check cannot be completed")
	}
	if errors.Is(err, realtime.ErrStreamUnauthorized) {
		t.Errorf("an unreachable store was reported as a revoked caller: %v", err)
	}
}
