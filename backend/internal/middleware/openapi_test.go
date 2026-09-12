package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// specFixture is a two-route document: one that constrains a body, and one
// standing in for a provider's webhook. It is written here rather than loaded
// from internal/openapi so that a change to the real document cannot silently
// change what these tests are about.
const specFixture = `
openapi: 3.1.0
info: {title: fixture, version: "1"}
paths:
  /api/v1/things:
    post:
      operationId: createThing
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name, size]
              properties:
                name: {type: string}
                size: {type: integer}
      responses:
        "201": {description: created}
  /api/v1/webhooks/mercadopago:
    post:
      operationId: mpWebhook
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [declared_by_us_not_by_them]
              properties:
                declared_by_us_not_by_them: {type: string}
      responses:
        "200": {description: ok}
`

// echoBody is the handler behind the validator: it proves the body survived
// validation intact, which is the failure mode a middleware that reads a
// request body has.
func echoBody(t *testing.T) (http.Handler, *string) {
	t.Helper()

	seen := new(string)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("the handler could not read the body the validator left behind: %v", err)
		}
		*seen = string(body)
		w.WriteHeader(http.StatusCreated)
	}), seen
}

func newTestValidator(t *testing.T) *SpecValidator {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromData([]byte(specFixture))
	if err != nil {
		t.Fatalf("loading the fixture document: %v", err)
	}
	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	v, err := NewSpecValidator(doc, respond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSpecValidator: %v", err)
	}
	return v
}

func post(t *testing.T, path, body string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestAConformingRequestReachesTheHandlerWithItsBodyIntact(t *testing.T) {
	next, seen := echoBody(t)
	w := httptest.NewRecorder()

	newTestValidator(t).ValidateRequests(next).
		ServeHTTP(w, post(t, "/api/v1/things", `{"name":"court","size":2}`))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	if *seen != `{"name":"court","size":2}` {
		t.Errorf("the handler saw %q; the validator did not put the body back", *seen)
	}
}

func TestARequestTheDocumentForbidsIsRefusedBeforeTheHandler(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })

	tests := map[string]string{
		"a missing required field": `{"name":"court"}`,
		"the wrong type":           `{"name":"court","size":"two"}`,
		"not JSON at all":          `nope`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			reached = false
			w := httptest.NewRecorder()

			newTestValidator(t).ValidateRequests(next).ServeHTTP(w, post(t, "/api/v1/things", body))

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", w.Code, w.Body.String())
			}
			if reached {
				t.Error("the handler ran on a request the document forbids")
			}
			if !strings.Contains(w.Body.String(), `"error"`) {
				t.Errorf("the refusal is not in the API's error envelope: %s", w.Body.String())
			}
		})
	}
}

// A webhook body is a third party's, and its signature is computed over the
// exact bytes that arrived. The document's description of it is our reading of
// their format, not a contract they agreed to, so refusing one would drop a
// real notification over a field the provider added on a Tuesday.
func TestAWebhookBodyIsNeverRefused(t *testing.T) {
	next, seen := echoBody(t)
	w := httptest.NewRecorder()

	body := `{"action":"payment.created","data":{"id":"123"}}`
	newTestValidator(t).ValidateRequests(next).
		ServeHTTP(w, post(t, "/api/v1/webhooks/mercadopago", body))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want the handler's 201: %s", w.Code, w.Body.String())
	}
	if *seen != body {
		t.Errorf("the handler saw %q, want the exact bytes that arrived", *seen)
	}
}

// /debug/pprof and /debug/vars are registered in development and deliberately
// absent from the document. A route the document does not describe passes
// through: openapi_sync_test.go is what stops a real route from being
// undocumented, so this branch cannot hide one.
func TestAnUndocumentedRoutePassesThrough(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	w := httptest.NewRecorder()

	newTestValidator(t).ValidateRequests(next).ServeHTTP(w, post(t, "/debug/vars", `whatever`))

	if !reached || w.Code != http.StatusOK {
		t.Errorf("an undocumented route was not passed through: reached=%v status=%d", reached, w.Code)
	}
}

func TestNewSpecValidatorRefusesANilDocument(t *testing.T) {
	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := NewSpecValidator(nil, respond, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Error("a nil document was accepted")
	}
}
