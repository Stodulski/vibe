package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/middleware"
)

// ---------------------------------------------------------------------------
// Why this file exists
// ---------------------------------------------------------------------------
//
// TestEveryRouteIsGuarded proves that a guard is present. It cannot prove the
// guard is the right one: it only ever sends an anonymous request, and
// RequireAuth, RequireComplexOwner and RequireSuperAdmin all reject an
// anonymous caller with the same 401. Three separate reviewers each replaced a
// stronger guard with RequireAuth — opening every /api/v1/admin route to any
// registered owner, removing tenant isolation from every complex-scoped route,
// and exposing the platform audit log — and the suite stayed green.
//
// The matrix below closes that gap by sending the same request as five
// different callers and asserting what each one gets back. A guard swapped for
// a weaker one changes at least one cell.

// ---------------------------------------------------------------------------
// Caller classes
// ---------------------------------------------------------------------------

// callerClass is one of the five kinds of caller every private route is tested
// against.
type callerClass int

const (
	// anonymous carries no credentials at all.
	anonymous callerClass = iota
	// authenticated is a logged-in account that owns no complex. It is the
	// caller a "RequireSuperAdmin -> RequireAuth" slip hands the admin API to.
	authenticated
	// foreignOwner owns a complex, but not the one named in the path. It is
	// the caller tenant isolation exists for.
	foreignOwner
	// resourceOwner owns the complex named in the path.
	resourceOwner
	// superadmin holds the platform role. It owns no complex, which is why it
	// is refused by the ownership guard as well — see the matrix.
	superadmin
)

func (c callerClass) String() string {
	switch c {
	case anonymous:
		return "anonymous"
	case authenticated:
		return "authenticated-non-owner"
	case foreignOwner:
		return "owner-of-another-complex"
	case resourceOwner:
		return "owner-of-this-complex"
	case superadmin:
		return "superadmin"
	}
	return "unknown"
}

// callerClasses is the full set, in matrix column order.
var callerClasses = []callerClass{anonymous, authenticated, foreignOwner, resourceOwner, superadmin}

// ---------------------------------------------------------------------------
// The outcomes a cell can hold
// ---------------------------------------------------------------------------

// allowed means the guards let the request reach its handler. What the handler
// then answers is that handler's own business and not asserted here — only
// that it was not turned away. See assertOutcome for how the two are told
// apart.
const allowed = 0

// tenantDenial is what a caller who does not own the complex named in the path
// gets back from RequireComplexOwner.
//
// It is 403, and the convention says 404. internal/clients/clients.go:6-8: a
// mismatch is reported "as 404 rather than 403 so the endpoint cannot be used to
// probe which client ids exist under another complex." So a 403 here does
// confirm the id exists.
//
// That divergence is already known and recorded: see the comment on
// TestRequireComplexOwnerRefusesAnotherOwnersComplex in
// internal/middleware/middleware_test.go, which pins the 403 and argues that
// complex ids are UUIDs, so the oracle is not enumerable and the two layers are
// left disagreeing on purpose. This matrix asserts the behaviour that decision
// produced rather than re-litigating it, and names it once instead of spelling
// 403 into forty rows — flipping this constant to http.StatusNotFound is the
// whole of the test-side change if the guard is ever brought in line.
//
// What is not covered by that decision, and is worth fixing: RequireComplexOwner
// still carries a doc comment saying it does the opposite of what it does — "A
// complex the caller does not own reads as missing rather than forbidden: a 403
// would confirm the id exists" — directly above the line that calls
// respond.NotPermitted. See internal/middleware/chain.go.
const tenantDenial = http.StatusForbidden

// ---------------------------------------------------------------------------
// The matrix
// ---------------------------------------------------------------------------

// policy is the access rule a route is registered with. Each one expands to a
// full row of the matrix through wantStatus below, so the table stays one line
// per route and the whole authorization surface fits on a screen.
type policy int

const (
	// authOnly is guards.RequireAuth: any logged-in account, no further check.
	authOnly policy = iota
	// complexOwner is RequireAuth + RequireComplexOwner: only the owner of the
	// complex named in the path, whatever their role.
	complexOwner
	// superAdmin is RequireAuth + RequireSuperAdmin: only the platform role.
	superAdmin
)

func (p policy) String() string {
	switch p {
	case authOnly:
		return "authOnly"
	case complexOwner:
		return "complexOwner"
	case superAdmin:
		return "superAdmin"
	}
	return "unknown"
}

// wantStatus is the matrix itself: what each policy owes each caller class.
//
//	                anonymous  authed  foreign-owner  this-owner  superadmin
//	authOnly        401        allow   allow          allow       allow
//	complexOwner    401        403*    403*           allow       403*
//	superAdmin      401        403     403            403         allow
//
// (*) tenantDenial — 403 today where the convention asks for 404. See its
// comment.
//
// The superadmin cell on complexOwner is deliberate: RequireComplexOwner
// compares owner ids and knows nothing about roles, so the platform role does
// not inherit access to a tenant's data. Platform-wide reads live behind
// /api/v1/admin instead.
func wantStatus(p policy, c callerClass) int {
	if c == anonymous {
		return http.StatusUnauthorized
	}

	switch p {
	case authOnly:
		return allowed

	case complexOwner:
		if c == resourceOwner {
			return allowed
		}
		return tenantDenial

	case superAdmin:
		if c == superadmin {
			return allowed
		}
		return http.StatusForbidden
	}

	panic(fmt.Sprintf("unknown policy %d", p))
}

// routePolicies names the access rule of every route that is not in
// publicRoutes. It is checked against the live route table in both directions,
// so a new endpoint fails the test until its policy is written down here.
var routePolicies = map[string]policy{
	// Session-scoped: the caller's own account and their own list of complexes.
	"GET /api/v1/auth/me":        authOnly,
	"PUT /api/v1/auth/me":        authOnly,
	"DELETE /api/v1/auth/me":     authOnly,
	"GET /api/v1/slug-available": authOnly,
	"GET /api/v1/complexes":      authOnly,
	"POST /api/v1/complexes":     authOnly,

	// A billed third-party proxy. Any account may use it; none of it is
	// tenant data.
	"GET /api/v1/places/autocomplete": authOnly,
	"GET /api/v1/places/details":      authOnly,

	// Tenant data. Every one of these names a complex in the path and must be
	// readable only by that complex's owner.
	"GET /api/v1/complexes/:id":                                      complexOwner,
	"PUT /api/v1/complexes/:id":                                      complexOwner,
	"DELETE /api/v1/complexes/:id":                                   complexOwner,
	"PUT /api/v1/complexes/:id/schedules":                            complexOwner,
	"POST /api/v1/complexes/:id/uploads/presign":                     complexOwner,
	"DELETE /api/v1/complexes/:id/uploads":                           complexOwner,
	"POST /api/v1/complexes/:id/mp/connect":                          complexOwner,
	"DELETE /api/v1/complexes/:id/mp/connect":                        complexOwner,
	"GET /api/v1/complexes/:id/mp/status":                            complexOwner,
	"GET /api/v1/complexes/:id/courts":                               complexOwner,
	"POST /api/v1/complexes/:id/courts":                              complexOwner,
	"PUT /api/v1/complexes/:id/courts/:courtID":                      complexOwner,
	"DELETE /api/v1/complexes/:id/courts/:courtID":                   complexOwner,
	"PUT /api/v1/complexes/:id/courts/:courtID/prices":               complexOwner,
	"POST /api/v1/complexes/:id/courts/:courtID/block":               complexOwner,
	"GET /api/v1/complexes/:id/audit-log":                            complexOwner,
	"GET /api/v1/complexes/:id/blocked-slots":                        complexOwner,
	"DELETE /api/v1/complexes/:id/blocked-slots/:slotID":             complexOwner,
	"GET /api/v1/complexes/:id/bookings":                             complexOwner,
	"POST /api/v1/complexes/:id/bookings":                            complexOwner,
	"GET /api/v1/complexes/:id/bookings/:bookingID":                  complexOwner,
	"PUT /api/v1/complexes/:id/bookings/:bookingID":                  complexOwner,
	"POST /api/v1/complexes/:id/bookings/:bookingID/cancel":          complexOwner,
	"POST /api/v1/complexes/:id/bookings/:bookingID/confirm-payment": complexOwner,
	"POST /api/v1/complexes/:id/bookings/:bookingID/manual-refund":   complexOwner,
	"GET /api/v1/complexes/:id/clients":                              complexOwner,
	"GET /api/v1/complexes/:id/clients/:clientID":                    complexOwner,
	"PUT /api/v1/complexes/:id/clients/:clientID":                    complexOwner,
	"GET /api/v1/complexes/:id/events":                               complexOwner,
	"GET /api/v1/complexes/:id/stats":                                complexOwner,
	"GET /api/v1/complexes/:id/stats/revenue":                        complexOwner,
	"GET /api/v1/complexes/:id/stats/occupancy":                      complexOwner,
	"GET /api/v1/complexes/:id/stats/clients":                        complexOwner,
	"GET /api/v1/complexes/:id/reports/monthly":                      complexOwner,
	"GET /api/v1/complexes/:id/reports/export":                       complexOwner,

	// Platform-wide: every tenant's data, plus the audit log that records who
	// did what, from which address, to which entity.
	"GET /api/v1/admin/stats":                     superAdmin,
	"GET /api/v1/admin/users":                     superAdmin,
	"GET /api/v1/admin/users/:id":                 superAdmin,
	"PATCH /api/v1/admin/users/:id/toggle-active": superAdmin,
	"GET /api/v1/admin/complexes":                 superAdmin,
	"GET /api/v1/admin/complexes/:id":             superAdmin,
	"GET /api/v1/admin/audit-log":                 superAdmin,
	"GET /api/v1/admin/healthcheck":               superAdmin,
}

// ---------------------------------------------------------------------------
// The test
// ---------------------------------------------------------------------------

// TestRouteAuthorizationMatrix sends every private route to the real router as
// each of the five caller classes and checks the answer against wantStatus.
func TestRouteAuthorizationMatrix(t *testing.T) {
	for _, rt := range recordRoutes(t, newTestApplication(t)) {
		key := rt.method + " " + rt.path
		if _, public := publicRoutes[key]; public {
			continue
		}
		if strings.HasPrefix(rt.path, "/debug/") {
			continue // registered only when pprof is explicitly enabled
		}

		p, known := routePolicies[key]
		if !known {
			// Checked here as well as in TestRoutePolicyTableIsComplete so a
			// new route can never be exercised against an implicit default.
			continue
		}

		for _, class := range callerClasses {
			t.Run(key+"/"+class.String(), func(t *testing.T) {
				// Every cell owns its fixture (below), so cells can run at once.
				t.Parallel()
				// A fixture per cell, not one for the whole run.
				//
				// These are real handlers against real stores, and some of them
				// write: DELETE /api/v1/auth/me removes the calling account and
				// revokes its token, so with a shared fixture every route
				// registered after auth answered 401 to every caller — a
				// convincing-looking column of denials that was really the
				// previous row's side effect. A matrix whose result depends on
				// route ordering is not evidence of anything.
				fx := newAuthzFixture(t)
				status := fx.call(t, rt, class)
				assertOutcome(t, key, p, class, status)
			})
		}
	}
}

// TestRoutePolicyTableIsComplete is the fail-closed half: the matrix must name
// every private route, and only routes that exist.
//
// Without it a new endpoint would simply not be tested, which is the failure
// mode this whole file was written to remove.
func TestRoutePolicyTableIsComplete(t *testing.T) {
	app := newTestApplication(t)

	registered := make(map[string]bool)
	for _, rt := range recordRoutes(t, app) {
		key := rt.method + " " + rt.path
		if strings.HasPrefix(rt.path, "/debug/") {
			continue
		}
		registered[key] = true

		if _, public := publicRoutes[key]; public {
			continue
		}
		if _, ok := routePolicies[key]; !ok {
			t.Errorf("route %q has no entry in routePolicies.\n"+
				"Every private route must declare which callers may reach it. Add it there, "+
				"or add it to publicRoutes with the reason it is open.", key)
		}
	}

	var stale []string
	for key := range routePolicies {
		if !registered[key] {
			stale = append(stale, key)
		}
		if reason, public := publicRoutes[key]; public {
			t.Errorf("route %q is in both routePolicies and publicRoutes (%q); it cannot be both", key, reason)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		t.Errorf("routePolicies names %q, which is not a registered route; remove it", key)
	}
}

// assertOutcome compares one observed status against the matrix cell.
func assertOutcome(t *testing.T, key string, p policy, class callerClass, got int) {
	t.Helper()

	want := wantStatus(p, class)

	if want != allowed {
		if got != want {
			t.Errorf("%s as %s: want %d, got %d\n"+
				"This route is registered %s. A weaker guard is the usual cause: "+
				"RequireComplexOwner or RequireSuperAdmin replaced by RequireAuth "+
				"lets this caller through and every anonymous-only test stays green.",
				key, class, want, got, p)
		}
		return
	}

	// An allowed caller must not be turned away. The handler is free to answer
	// 400 or 422 — it was given no request body — but 401, 403 and 404 are the
	// three statuses the guards themselves use, so none of them can be accepted
	// as evidence the request got through.
	//
	// A 404 in particular would be ambiguous: RequireComplexOwner answers 404
	// for a complex that does not exist, and a handler answers 404 for a
	// sub-resource it cannot find. Rather than guess which one happened, the
	// fixture seeds every record these routes look up, and an unexpected 404
	// here is reported as a missing fixture instead of being waved through.
	switch {
	case got == http.StatusUnauthorized || got == http.StatusForbidden:
		t.Errorf("%s as %s: want the request to reach its handler, got %d — "+
			"this caller is entitled to this route and a guard rejected them",
			key, class, got)
	case got == http.StatusNotFound:
		t.Errorf("%s as %s: got 404, which is ambiguous here — it is either the "+
			"ownership guard rejecting an entitled caller or a record the fixture "+
			"never seeded. Seed it in newAuthzFixture so this cell can distinguish "+
			"the two.", key, class)
	case got == http.StatusNotImplemented:
		// The uploads endpoints answer 501 on their first line when no object
		// store is configured, which the test application deliberately leaves
		// out. That is a considered response from inside the handler, so it is
		// proof the guards passed — unlike the 5xx below, which is a crash.
	case got >= 500:
		t.Errorf("%s as %s: got %d — the request reached the handler but the "+
			"fixture is not complete enough for it to run. Fill the gap in "+
			"newAuthzFixture rather than accepting the 5xx.", key, class, got)
	}
}

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

// authzFixture is an application whose stores hold the records these routes
// look up, plus one access token per caller class.
type authzFixture struct {
	app *application
	ts  *httptest.Server

	// complexID is the complex named by :id on every complex-scoped route.
	complexID uuid.UUID
	// adminUserID is the account named by :id under /api/v1/admin/users.
	adminUserID uuid.UUID
	// subResourceID stands in for :courtID, :bookingID, :clientID and :slotID.
	subResourceID uuid.UUID

	// tokens holds the bearer token for each class; anonymous has none.
	tokens map[callerClass]string
}

// newAuthzFixture builds the application, seeds the records and mints the
// tokens. Four accounts, two complexes, then the per-store records the
// complex-scoped handlers read: values that are only meaningful together, so
// they are read in one place.
func newAuthzFixture(t *testing.T) *authzFixture {
	t.Helper()

	app := newTestApplication(t)

	// The rate limiter is on in the shared harness, deliberately. Here it has
	// to come off: the auth limiter is a hardcoded 10 requests per 6 seconds
	// on /api/v1/auth/*, and this test issues twenty across the four
	// /auth/me routes. Leaving it on would answer 429 to requests this matrix
	// is trying to read as allowed or denied, which is both a false result and
	// a flaky one. Everything else in the chain — CSRF, Authenticate, the
	// guards — is the production wiring.
	app.middleware = middleware.New(middleware.Dependencies{
		Users:     app.models.Users,
		Complexes: app.models.Complexes,
		Tokens:    app.tokens,
		Blacklist: app.blacklist,
		Respond:   app.respond,
		Logger:    app.logger,
		Shutdown:  app.shutdown,
	}, middleware.Config{RateLimitEnabled: false})

	users, ok := app.models.Users.(*mockUserStore)
	if !ok {
		t.Fatalf("user store is %T, not *mockUserStore", app.models.Users)
	}
	complexes, ok := app.models.Complexes.(*mockComplexStore)
	if !ok {
		t.Fatalf("complex store is %T, not *mockComplexStore", app.models.Complexes)
	}

	account := func(email, role string) *data.User {
		return users.seed(data.User{
			ID:            uuid.New(),
			Email:         email,
			FirstName:     "Test",
			LastName:      "Account",
			Phone:         "+5491112345678",
			Role:          role,
			IsActive:      true,
			EmailVerified: true,
		})
	}

	owner := account("owner@example.com", "owner")
	stranger := account("stranger@example.com", "owner")
	other := account("other-owner@example.com", "owner")
	admin := account("super@example.com", "superadmin")

	complexOf := func(o *data.User, slug string) *data.Complex {
		return complexes.seed(data.Complex{
			ID:                uuid.New(),
			OwnerID:           o.ID,
			Name:              "Complejo " + slug,
			Slug:              slug,
			Address:           "Calle 1",
			City:              "La Plata",
			Province:          "Buenos Aires",
			CountryCode:       "AR",
			Currency:          "ARS",
			Phone:             "+5492211234567",
			DepositPercentage: 50,
			CancellationHours: 24,
			IsActive:          true,
		})
	}

	target := complexOf(owner, "complejo-propio")
	// The foreign owner has a complex of their own, so the class under test is
	// "an owner asking about somebody else's complex" rather than "an account
	// with no complexes at all" — which is what `authenticated` already covers.
	complexOf(other, "complejo-ajeno")

	fx := &authzFixture{
		app:           app,
		complexID:     target.ID,
		adminUserID:   stranger.ID,
		subResourceID: uuid.New(),
		tokens:        make(map[callerClass]string),
	}

	fx.seedSubResources(t, target)
	fx.ts = newTestServer(t, app)

	for class, user := range map[callerClass]*data.User{
		authenticated: stranger,
		foreignOwner:  other,
		resourceOwner: owner,
		superadmin:    admin,
	} {
		token, err := app.tokens.GenerateAccessToken(user.ID, user.Role)
		if err != nil {
			t.Fatalf("minting a token for %s: %v", class, err)
		}
		fx.tokens[class] = token
	}

	return fx
}

// seedSubResources gives the complex-scoped handlers the records they read
// after the guard has passed.
//
// Without them an entitled caller gets a 404 from the handler, which is
// indistinguishable from the ownership guard rejecting them — see
// assertOutcome. Every record here belongs to the target complex.
func (fx *authzFixture) seedSubResources(t *testing.T, target *data.Complex) {
	t.Helper()

	courts, ok := fx.app.models.Courts.(*mockCourtStore)
	if !ok {
		t.Fatalf("court store is %T, not *mockCourtStore", fx.app.models.Courts)
	}
	bookings, ok := fx.app.models.Bookings.(*mockBookingStore)
	if !ok {
		t.Fatalf("booking store is %T, not *mockBookingStore", fx.app.models.Bookings)
	}
	clients, ok := fx.app.models.Clients.(*mockClientStore)
	if !ok {
		t.Fatalf("client store is %T, not *mockClientStore", fx.app.models.Clients)
	}
	admin, ok := fx.app.models.Admin.(*mockAdminStore)
	if !ok {
		t.Fatalf("admin store is %T, not *mockAdminStore", fx.app.models.Admin)
	}

	id := fx.subResourceID
	tomorrow := time.Now().AddDate(0, 0, 1)

	court := &data.Court{
		ID:        id,
		ComplexID: target.ID,
		Name:      "Cancha 1",
		IsActive:  true,
	}
	courts.GetByIDFn = func(_ context.Context, _ uuid.UUID) (*data.Court, error) { return court, nil }
	courts.GetBlockedSlotByIDFn = func(_ context.Context, _ uuid.UUID) (*data.BlockedSlot, error) {
		return &data.BlockedSlot{ID: id, CourtID: court.ID, Date: tomorrow, StartTime: "10:00", EndTime: "11:00"}, nil
	}

	booking := &data.Booking{
		ID:              id,
		ComplexID:       target.ID,
		CourtID:         court.ID,
		Date:            tomorrow,
		StartTime:       "10:00",
		DurationMinutes: 60,
		Status:          "confirmed",
	}
	bookings.GetByIDFn = func(_ context.Context, _ uuid.UUID) (*data.Booking, error) { return booking, nil }

	client := &data.Client{
		ID:        id,
		ComplexID: target.ID,
		FirstName: "Cliente",
		LastName:  "Uno",
		Phone:     "+5492211234567",
	}
	clients.GetByIDFn = func(_ context.Context, _ uuid.UUID) (*data.Client, error) { return client, nil }

	// The admin detail routes read the platform store rather than the tenant
	// stores, so they need their own records.
	admin.GetUserDetailFn = func(_ context.Context, userID uuid.UUID) (*data.AdminUserDetail, error) {
		return &data.AdminUserDetail{User: &data.User{ID: userID, Email: "detail@example.com"}}, nil
	}
	admin.GetComplexDetailFn = func(_ context.Context, complexID uuid.UUID) (*data.AdminComplexDetail, error) {
		return &data.AdminComplexDetail{Complex: &data.Complex{ID: complexID}}, nil
	}
}

// call issues one request for a route as one caller class and returns the
// status.
func (fx *authzFixture) call(t *testing.T, rt recordedRoute, class callerClass) int {
	t.Helper()

	// The stream endpoint holds its connection open until the request context
	// is cancelled, so every request gets a cancellable context and the
	// deferred cancel below releases the handler.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, rt.method, fx.ts.URL+fx.concretePath(rt.path), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// The session is carried in the Authorization header rather than the
	// access_token cookie on purpose. CSRFProtect only challenges cookie-borne
	// sessions, so a bearer token takes CSRF out of the picture and leaves the
	// status reporting on authorization alone — which is what this matrix
	// measures. CSRF has its own coverage.
	if token := fx.tokens[class]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fx.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s as %s: %v", rt.method, rt.path, class, err)
	}
	// GET /complexes/:id/events is an open event stream; closing the body
	// cancels the request context, which is how its handler is told to stop.
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode
}

// concretePath fills the route parameters with ids the fixture has seeded, so a
// request reaches the guard and then the handler rather than failing earlier on
// an unparseable id or a missing record.
func (fx *authzFixture) concretePath(path string) string {
	segments := strings.Split(path, "/")
	adminUsers := strings.HasPrefix(path, "/api/v1/admin/users/")

	for i, s := range segments {
		if !strings.HasPrefix(s, ":") {
			continue
		}
		switch {
		case s == ":slug":
			segments[i] = "complejo-propio"
		case s == ":id" && adminUsers:
			segments[i] = fx.adminUserID.String()
		case s == ":id":
			segments[i] = fx.complexID.String()
		default:
			segments[i] = fx.subResourceID.String()
		}
	}
	return strings.Join(segments, "/")
}
