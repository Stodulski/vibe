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

	reqInput := &openapi3filter.RequestValidationInput{
		Request:    matchReq,
		PathParams: pathParams,
		Route:      route,
	}
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
	complex := complexes.seed(data.Complex{
		ID: uuid.New(), OwnerID: owner.ID, Name: "Complejo", Slug: "complejo",
		IsActive: true, CountryCode: "AR", Currency: "ARS",
	})
	seededClient := &data.Client{
		ID: uuid.New(), ComplexID: complex.ID, FirstName: "Juan", LastName: "Perez",
		Phone: "+5491100000000", TotalBookings: 3, NoShows: 0,
	}
	clients.GetByComplexFn = func(_ context.Context, _ uuid.UUID, _ string, _ data.Filters) ([]*data.Client, data.Metadata, error) {
		return []*data.Client{seededClient}, data.Metadata{HasMore: false}, nil
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
	assertResponseConformsToSpec(t, router, req, resp, body)
}

// TestOpenAPIConformance_RateLimited trips the fixed auth-prefix ceiling
// (1/6 rps, burst 10), which the test harness's general limiter override
// does not touch, by sending more requests than the burst allows in a tight
// loop.
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
