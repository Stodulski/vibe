package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// conformanceServerHost matches the "http://localhost:8080" entry in
// internal/openapi/openapi.yaml's servers list. The router matches a
// request's route by first matching it against a declared server, and the
// httptest.Server this suite runs against listens on an ephemeral
// 127.0.0.1 port, which matches neither declared server. Route lookup is
// therefore done against a clone of the request with its URL rehosted here,
// while the exchange itself still runs against the real ephemeral listener.
const conformanceServerHost = "http://localhost:8080"

// conformanceRouter builds an openapi3filter router from the application's
// own parsed document, so a live HTTP exchange can be matched back to the
// operation the document declares for it.
func conformanceRouter(t *testing.T, app *application) routers.Router {
	t.Helper()

	router, err := legacy.NewRouter(app.openapi.Document())
	if err != nil {
		t.Fatalf("building an openapi3filter router from the embedded document: %v", err)
	}
	return router
}

// assertResponseConformsToSpec checks one HTTP exchange against the
// operation the OpenAPI document declares for its method and path: the
// response status must be documented, and the body and headers must satisfy
// that status's schema.
func assertResponseConformsToSpec(t *testing.T, router routers.Router, req *http.Request, resp *http.Response, body []byte) {
	t.Helper()

	matchReq, route, pathParams := matchRoute(t, router, req)

	reqInput := specInput(t, matchReq, pathParams, route)
	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 resp.StatusCode,
		Header:                 resp.Header,
	}
	respInput.SetBodyBytes(body)

	if err := openapi3filter.ValidateResponse(req.Context(), respInput); err != nil {
		t.Errorf("response for %s %s (status %d) does not conform to internal/openapi/openapi.yaml: %v\nbody: %s",
			req.Method, req.URL.Path, resp.StatusCode, err, body)
	}
}

// matchRoute rehosts a request onto a declared server and finds the operation
// the document declares for it.
func matchRoute(t *testing.T, router routers.Router, req *http.Request) (*http.Request, *routers.Route, map[string]string) {
	t.Helper()

	matchReq := req.Clone(req.Context())
	rehosted, err := url.Parse(conformanceServerHost + req.URL.Path)
	if err != nil {
		t.Fatalf("rehosting the request URL for route matching: %v", err)
	}
	rehosted.RawQuery = req.URL.RawQuery
	matchReq.URL = rehosted
	matchReq.Host = rehosted.Host

	route, pathParams, err := router.FindRoute(matchReq)
	if err != nil {
		t.Fatalf("the document has no route for %s %s: %v", req.Method, req.URL.Path, err)
	}
	return matchReq, route, pathParams
}

// specInput is the validation input both halves of the contract are checked
// against. Authentication is the application's own middleware chain, not the
// document's to enforce here.
func specInput(t *testing.T, matchReq *http.Request, pathParams map[string]string,
	route *routers.Route) *openapi3filter.RequestValidationInput {
	t.Helper()

	return &openapi3filter.RequestValidationInput{
		Request:    matchReq,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error { return nil },
		},
	}
}

// validateRequestAgainstSpec runs the request half of the contract (API-03).
//
// Until this existed the suite only checked what the API answers, so a document
// that described a body the handler would never accept — or accepted one it
// forbids — passed. The same validation runs as middleware outside production
// (internal/middleware/openapi.go); here it costs a test instead of a request.
//
// The body is refilled from GetBody, because the client already read it to put
// it on the wire. A request built without one (a GET) has nothing to refill.
func validateRequestAgainstSpec(t *testing.T, router routers.Router, req *http.Request) error {
	t.Helper()

	matchReq, route, pathParams := matchRoute(t, router, req)
	input := specInput(t, matchReq, pathParams, route)

	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			t.Fatalf("rewinding the request body to validate it: %v", err)
		}
		input.Request.Body = body
	}
	return openapi3filter.ValidateRequest(req.Context(), input)
}

// assertRequestConformsToSpec fails when the document forbids a request the
// suite considers valid.
func assertRequestConformsToSpec(t *testing.T, router routers.Router, req *http.Request) {
	t.Helper()

	if err := validateRequestAgainstSpec(t, router, req); err != nil {
		t.Errorf("request %s %s does not conform to internal/openapi/openapi.yaml: %v",
			req.Method, req.URL.Path, err)
	}
}

func TestOpenAPIConformance_Healthcheck(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/api/v1/healthcheck", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	assertRequestConformsToSpec(t, router, req)
	assertResponseConformsToSpec(t, router, req, resp, body)
}

func TestOpenAPIConformance_LoginInvalidBody(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/v1/auth/login",
		bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", resp.StatusCode, body)
	}
	assertResponseConformsToSpec(t, router, req, resp, body)
}

func TestOpenAPIConformance_LoginSuccess(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	users, ok := app.models.Users.(*mockUserStore)
	if !ok {
		t.Fatalf("user store is %T, not *mockUserStore", app.models.Users)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-battery"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users.seed(authstore.User{
		ID:            uuid.New(),
		Email:         "ana@example.com",
		FirstName:     "Ana",
		LastName:      "Perez",
		Phone:         "+5491112345678",
		Role:          "owner",
		PasswordHash:  hash,
		IsActive:      true,
		EmailVerified: true,
	})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/v1/auth/login",
		bytes.NewReader([]byte(`{"email":"ana@example.com","password":"correct-horse-battery"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	assertRequestConformsToSpec(t, router, req)
	assertResponseConformsToSpec(t, router, req, resp, body)
}

// TestOpenAPIConformance_AuthenticatedPaginatedList exercises one
// cookie-authenticated, owner-scoped, cursor-paginated route: the client
// roster. The seeded row is non-empty so the response carries a real Client
// object, not just an empty array.
func TestOpenAPIConformance_AuthenticatedPaginatedList(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	users, ok := app.models.Users.(*mockUserStore)
	if !ok {
		t.Fatalf("user store is %T, not *mockUserStore", app.models.Users)
	}
	complexes, ok := app.models.Complexes.(*mockComplexStore)
	if !ok {
		t.Fatalf("complex store is %T, not *mockComplexStore", app.models.Complexes)
	}
	clients, ok := app.models.Clients.(*mockClientStore)
	if !ok {
		t.Fatalf("client store is %T, not *mockClientStore", app.models.Clients)
	}

	owner := users.seed(authstore.User{
		ID: uuid.New(), Email: "owner@example.com", FirstName: "Owner", LastName: "Account",
		Phone: "+5491112345678", Role: "owner", IsActive: true, EmailVerified: true,
	})
	complex := complexes.seed(complexstore.Complex{
		ID: uuid.New(), OwnerID: owner.ID, Name: "Complejo", Slug: "complejo",
		IsActive: true, CountryCode: "AR", Currency: "ARS",
	})
	seededClient := &clientstore.Client{
		ID: uuid.New(), ComplexID: complex.ID, FirstName: "Juan", LastName: "Perez",
		Phone: "+5491100000000", TotalBookings: 3, NoShows: 0,
	}
	clients.GetByComplexFn = func(_ context.Context, _ uuid.UUID, _ string, _ data.Filters) ([]*clientstore.Client, data.Metadata, error) {
		return []*clientstore.Client{seededClient}, data.Metadata{HasMore: false}, nil
	}

	token, err := app.tokens.GenerateAccessToken(owner.ID, owner.Role)
	if err != nil {
		t.Fatalf("minting an access token: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		ts.URL+"/api/v1/complexes/"+complex.ID.String()+"/clients?limit=20", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	assertRequestConformsToSpec(t, router, req)
	assertResponseConformsToSpec(t, router, req, resp, body)
}

func TestOpenAPIConformance_NotFound(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		ts.URL+"/api/v1/public/complexes/does-not-exist", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", resp.StatusCode, body)
	}
	assertRequestConformsToSpec(t, router, req)
	assertResponseConformsToSpec(t, router, req, resp, body)
}

// TestOpenAPIConformance_RateLimited trips the fixed auth-prefix ceiling
// (1/6 rps, burst 10), which the test harness's general limiter override
// does not touch, by sending more requests than the burst allows in a tight
// loop.
// The document is only worth validating against if it actually refuses
// something. This is the other half of TestOpenAPIConformance_LoginInvalidBody:
// the handler answers 422 for an empty login body, and the document — which is
// what the middleware in internal/middleware/openapi.go enforces outside
// production — refuses the same body before a handler sees it.
func TestOpenAPIConformance_TheDocumentRefusesAnInvalidRequestBody(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/v1/auth/login",
		bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := validateRequestAgainstSpec(t, router, req); err == nil {
		t.Error("a login body with neither email nor password was accepted by the document")
	}
}

// TestOpenAPIConformance_OptionalFieldsMayBeOmitted is the regression guard
// for the openapi/code required-field audit (fix(openapi): describe optional
// request fields as optional so validation matches the code): every field
// below is optional in its handler (either a pointer never checked with "must
// be provided", or a value type whose zero value passes every validator.Check
// it is subject to), so a minimal body omitting it must still conform to
// internal/openapi/openapi.yaml. Before that fix each of these bodies made
// the document refuse a request the handler accepts, exactly as
// deposit_percentage did on POST /api/v1/complexes (the shape the E2E
// helper's createComplex sends, and the failure CI caught).
func TestOpenAPIConformance_OptionalFieldsMayBeOmitted(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create complex without deposit_percentage",
			method: http.MethodPost,
			path:   "/api/v1/complexes",
			body: `{"name":"Complejo E2E Test","slug":"complejo-e2e-test","address":"Av. Libertador 1234",` +
				`"city":"Buenos Aires","province":"Buenos Aires","phone":"+5491198765432","cancellation_hours":24}`,
		},
		{
			name:   "toggle user active without is_active",
			method: http.MethodPatch,
			path:   "/api/v1/admin/users/" + uuid.New().String() + "/toggle-active",
			body:   `{}`,
		},
		{
			name:   "update schedules without is_closed",
			method: http.MethodPut,
			path:   "/api/v1/complexes/" + uuid.New().String() + "/schedules",
			body: `{"schedules":[` +
				`{"day":"monday","open_time":"08:00","close_time":"23:00"},` +
				`{"day":"tuesday","open_time":"08:00","close_time":"23:00"},` +
				`{"day":"wednesday","open_time":"08:00","close_time":"23:00"},` +
				`{"day":"thursday","open_time":"08:00","close_time":"23:00"},` +
				`{"day":"friday","open_time":"08:00","close_time":"23:00"},` +
				`{"day":"saturday","open_time":"08:00","close_time":"23:00"},` +
				`{"day":"sunday","open_time":"08:00","close_time":"23:00"}]}`,
		},
		{
			name:   "resend verification without email",
			method: http.MethodPost,
			path:   "/api/v1/auth/resend-verification",
			body:   `{}`,
		},
		{
			name:   "forgot password without email",
			method: http.MethodPost,
			path:   "/api/v1/auth/forgot-password",
			body:   `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), tc.method, ts.URL+tc.path,
				bytes.NewReader([]byte(tc.body)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			assertRequestConformsToSpec(t, router, req)
		})
	}
}

func TestOpenAPIConformance_RateLimited(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)
	router := conformanceRouter(t, app)

	var last *http.Response
	var lastBody []byte
	var lastReq *http.Request

	for i := 0; i < 15; i++ {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/v1/auth/login",
			bytes.NewReader([]byte(`{}`)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			last, lastBody, lastReq = resp, body, req
			break
		}
	}

	if last == nil {
		t.Fatal("15 rapid requests to the auth-prefix ceiling never tripped a 429; " +
			"the fixed auth rate limit (1/6 rps, burst 10) should have")
	}
	assertResponseConformsToSpec(t, router, lastReq, last, lastBody)
}
