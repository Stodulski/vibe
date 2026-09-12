package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// requestWithParams builds a request carrying the given router path
// parameters, the way net/http's ServeMux binds them on a matched route.
func requestWithParams(t *testing.T, params map[string]string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	for k, v := range params {
		r.SetPathValue(k, v)
	}
	return r
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	err := WriteJSON(w, http.StatusCreated, Envelope{"court": "centre"}, http.Header{"X-Trace": []string{"abc"}})
	if err != nil {
		t.Fatalf("WriteJSON returned an error: %v", err)
	}

	if w.Code != http.StatusCreated {
		t.Errorf("want status %d; got %d", http.StatusCreated, w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("want Content-Type application/json; got %q", got)
	}
	if got := w.Header().Get("X-Trace"); got != "abc" {
		t.Errorf("caller-supplied header was dropped; got %q", got)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["court"] != "centre" {
		t.Errorf(`want body key "court" to be "centre"; got %q`, body["court"])
	}
}

func TestWriteJSONUnencodableValue(t *testing.T) {
	w := httptest.NewRecorder()

	// A channel cannot be marshalled. The failure must surface as an error
	// before anything is written, so the caller can still send a 500.
	if err := WriteJSON(w, http.StatusOK, Envelope{"bad": make(chan int)}, nil); err == nil {
		t.Fatal("want an error for an unencodable value; got nil")
	}
	if w.Body.Len() != 0 {
		t.Errorf("a failed encode must not write a partial body; got %q", w.Body.String())
	}
}

func TestReadJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"valid", `{"name":"centre"}`, ""},
		{"empty body", ``, "body must not be empty"},
		{"malformed", `{"name":`, "body contains badly-formed JSON"},
		{"wrong type", `{"name":42}`, `body contains incorrect JSON type for field "name"`},
		{"unknown field", `{"nombre":"centre"}`, `body contains unknown key "nombre"`},
		{"two values", `{"name":"a"}{"name":"b"}`, "body must only contain a single JSON value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tt.body))

			var dst struct {
				Name string `json:"name"`
			}
			err := ReadJSON(w, r, &dst)

			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("want no error; got %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("want error %q; got nil", tt.wantErr)
			case tt.wantErr != "" && err.Error() != tt.wantErr:
				t.Errorf("want error %q; got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestReadJSONBodyTooLarge(t *testing.T) {
	w := httptest.NewRecorder()
	oversized := `{"name":"` + strings.Repeat("x", MaxJSONBody) + `"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(oversized))

	var dst struct {
		Name string `json:"name"`
	}
	err := ReadJSON(w, r, &dst)
	if err == nil {
		t.Fatal("want an error for an oversized body; got nil")
	}
	if !strings.Contains(err.Error(), "must not be larger than") {
		t.Errorf("want a size-limit error; got %q", err.Error())
	}

	var tooLarge *BodyTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("want a *BodyTooLargeError so the Responder answers 413; got %T", err)
	}
}

// TestBadRequestReportsOversizedBodyAs413 checks that the Responder — the
// helper every handler calls after a failed ReadJSON — turns the oversized
// body case into 413 while leaving every other 400 path unchanged, and that
// the message and envelope shape stay the same either way.
func TestBadRequestReportsOversizedBodyAs413(t *testing.T) {
	rs := NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	rs.BadRequest(w, r, &BodyTooLargeError{msg: "body must not be larger than 1048576 bytes"})

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("want %d; got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
	var body Problem
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Detail != "body must not be larger than 1048576 bytes" {
		t.Errorf("want the original message preserved; got %q", body.Detail)
	}

	w = httptest.NewRecorder()
	rs.BadRequest(w, r, errors.New("body contains badly-formed JSON"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("an ordinary decoding error must still report 400; got %d", w.Code)
	}
}

func TestReadString(t *testing.T) {
	qs := url.Values{"sport": []string{"padel"}, "blank": []string{""}}

	if got := ReadString(qs, "sport", "tennis"); got != "padel" {
		t.Errorf("want the supplied value; got %q", got)
	}
	if got := ReadString(qs, "blank", "tennis"); got != "tennis" {
		t.Errorf("an empty value must fall back to the default; got %q", got)
	}
	if got := ReadString(qs, "absent", "tennis"); got != "tennis" {
		t.Errorf("a missing key must fall back to the default; got %q", got)
	}
}

func TestReadInt(t *testing.T) {
	qs := url.Values{"limit": []string{"25"}, "junk": []string{"many"}}

	if got := ReadInt(qs, "limit", 10); got != 25 {
		t.Errorf("want 25; got %d", got)
	}
	if got := ReadInt(qs, "junk", 10); got != 10 {
		t.Errorf("an unparseable value must fall back to the default; got %d", got)
	}
	if got := ReadInt(qs, "absent", 10); got != 10 {
		t.Errorf("a missing key must fall back to the default; got %d", got)
	}
}

// H-05: ReadInt's fallback-to-default is right for limit/weeks-style
// parameters and wrong for a value like the monthly report's month/year,
// where a caller who typed nonsense must be told rather than answered as if
// they had asked for today. ReadIntStrict is the variant those call sites
// use instead.
func TestReadIntStrict(t *testing.T) {
	qs := url.Values{"month": []string{"9"}, "junk": []string{"abc"}}

	got, err := ReadIntStrict(qs, "month", 1)
	if err != nil || got != 9 {
		t.Errorf("want (9, nil); got (%d, %v)", got, err)
	}

	got, err = ReadIntStrict(qs, "absent", 4)
	if err != nil || got != 4 {
		t.Errorf("a missing key must fall back to the default with no error; got (%d, %v)", got, err)
	}

	// The defect this closes: ReadInt turned "month=abc" into the default
	// silently. ReadIntStrict must refuse it instead.
	got, err = ReadIntStrict(qs, "junk", 4)
	if err == nil {
		t.Errorf("an unparseable value must return an error; got (%d, nil)", got)
	}
}

func TestContextUserRoundTrip(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	if _, ok := ContextGetAuthenticatedUser(r); ok {
		t.Error("an untouched request must report no authenticated user")
	}

	user := &authstore.User{ID: uuid.New(), Email: "owner@example.com"}
	r = ContextSetUser(r, user)

	got, ok := ContextGetAuthenticatedUser(r)
	if !ok {
		t.Fatal("want the user back after setting it")
	}
	if got.ID != user.ID {
		t.Errorf("want user %s; got %s", user.ID, got.ID)
	}
}

func TestContextComplexRoundTrip(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	if _, ok := ContextGetComplex(r); ok {
		t.Error("an untouched request must report no complex")
	}

	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo"}
	r = ContextSetComplex(r, complex)

	got, ok := ContextGetComplex(r)
	if !ok {
		t.Fatal("want the complex back after setting it")
	}
	if got.ID != complex.ID {
		t.Errorf("want complex %s; got %s", complex.ID, got.ID)
	}
}

func TestContextRequestID(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	if got := ContextGetRequestID(r); got != "" {
		t.Errorf("a request that skipped the middleware must report no id; got %q", got)
	}

	r = ContextSetRequestID(r, "a1b2c3d4")
	if got := ContextGetRequestID(r); got != "a1b2c3d4" {
		t.Errorf("want a1b2c3d4; got %q", got)
	}
}

func TestResponderErrorShape(t *testing.T) {
	rs := NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	tests := []struct {
		name     string
		call     func(w http.ResponseWriter, r *http.Request)
		wantCode int
		wantKind Kind
	}{
		{"not found", rs.NotFound, http.StatusNotFound, KindNotFound},
		{"method not allowed", rs.MethodNotAllowed, http.StatusMethodNotAllowed, KindMethodNotAllowed},
		{"edit conflict", rs.EditConflict, http.StatusConflict, KindConflict},
		{"rate limited", rs.RateLimitExceeded, http.StatusTooManyRequests, KindRateLimited},
		{"invalid credentials", rs.InvalidCredentials, http.StatusUnauthorized, KindUnauthorized},
		{"not permitted", rs.NotPermitted, http.StatusForbidden, KindForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			r = ContextSetRequestID(r, "req-1")
			tt.call(w, r)

			if w.Code != tt.wantCode {
				t.Errorf("want status %d; got %d", tt.wantCode, w.Code)
			}
			if got := w.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Errorf("want Content-Type application/problem+json; got %q", got)
			}

			var body Problem
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not valid JSON: %v", err)
			}
			if body.Type != tt.wantKind.URI() {
				t.Errorf("want type %q; got %q", tt.wantKind.URI(), body.Type)
			}
			if body.Status != tt.wantCode {
				t.Errorf("want status field %d; got %d", tt.wantCode, body.Status)
			}
			if body.Title == "" {
				t.Error("want a non-empty title")
			}
			if body.Detail == "" {
				t.Error("want a non-empty detail")
			}
			if body.Instance != "/" {
				t.Errorf("want instance %q; got %q", "/", body.Instance)
			}
			if body.RequestID != "req-1" {
				t.Errorf("want request_id %q; got %q", "req-1", body.RequestID)
			}
		})
	}
}

func TestResponderServerErrorHidesCause(t *testing.T) {
	rs := NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	w := httptest.NewRecorder()

	rs.ServerError(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
		errUnexpected{"connection to 10.0.0.5:5432 refused"})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want status 500; got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "10.0.0.5") {
		t.Errorf("the underlying cause must never reach the client; got %s", w.Body.String())
	}
}

func TestResponderInvalidAuthenticationTokenSetsChallenge(t *testing.T) {
	rs := NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	w := httptest.NewRecorder()

	rs.InvalidAuthenticationToken(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("want a Bearer challenge; got %q", got)
	}
}

type errUnexpected struct{ msg string }

func (e errUnexpected) Error() string { return e.msg }

// ReadUUIDParam replaced six near-identical readers (id, courtID, slotID,
// bookingID, clientID) with one parameterised function, so its behaviour for
// each of those names is what these cases pin down.
func TestReadUUIDParam(t *testing.T) {
	id := uuid.New()

	for _, name := range []string{"id", "courtID", "slotID", "bookingID", "clientID"} {
		t.Run(name+" valid", func(t *testing.T) {
			r := requestWithParams(t, map[string]string{name: id.String()})

			got, err := ReadUUIDParam(r, name)
			if err != nil {
				t.Fatalf("want no error; got %v", err)
			}
			if got != id {
				t.Errorf("want %s; got %s", id, got)
			}
		})

		t.Run(name+" malformed", func(t *testing.T) {
			r := requestWithParams(t, map[string]string{name: "not-a-uuid"})

			got, err := ReadUUIDParam(r, name)
			if err == nil {
				t.Fatal("want an error for a malformed UUID; got nil")
			}
			// The message names the parameter, because handlers return it to
			// the client verbatim as a 400.
			if want := "invalid " + name + " parameter"; err.Error() != want {
				t.Errorf("want %q; got %q", want, err.Error())
			}
			if got != uuid.Nil {
				t.Errorf("want uuid.Nil on failure; got %s", got)
			}
		})
	}

	t.Run("absent parameter", func(t *testing.T) {
		r := requestWithParams(t, nil)

		if _, err := ReadUUIDParam(r, "id"); err == nil {
			t.Fatal("want an error when the route never bound the parameter; got nil")
		}
	})
}

func TestReadStringParam(t *testing.T) {
	r := requestWithParams(t, map[string]string{"slug": "vibe-palermo"})

	if got := ReadStringParam(r, "slug"); got != "vibe-palermo" {
		t.Errorf("want vibe-palermo; got %q", got)
	}
	if got := ReadStringParam(r, "absent"); got != "" {
		t.Errorf("an unbound parameter must read as empty; got %q", got)
	}
}

func TestGuardsValidate(t *testing.T) {
	noop := func(h http.HandlerFunc) http.HandlerFunc { return h }

	t.Run("complete set passes", func(t *testing.T) {
		g := Guards{
			RequireAuth:         noop,
			RequireComplexOwner: noop,
			RequireSuperAdmin:   noop,
			Idempotent:          func(string) Guard { return noop },
		}
		if err := g.Validate(); err != nil {
			t.Errorf("want no error for a fully wired set; got %v", err)
		}
	})

	t.Run("names every missing guard", func(t *testing.T) {
		err := Guards{RequireComplexOwner: noop}.Validate()
		if err == nil {
			t.Fatal("want an error when guards are missing; got nil")
		}
		for _, name := range []string{"RequireAuth", "RequireSuperAdmin"} {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("the error must name %s; got %q", name, err.Error())
			}
		}
		if strings.Contains(err.Error(), "RequireComplexOwner") {
			t.Errorf("a wired guard must not be reported missing; got %q", err.Error())
		}
	})
}

func TestResponderJSON(t *testing.T) {
	rs := NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	t.Run("writes the body", func(t *testing.T) {
		w := httptest.NewRecorder()
		rs.JSON(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
			http.StatusCreated, Envelope{"court": "centre"})

		if w.Code != http.StatusCreated {
			t.Errorf("want 201; got %d", w.Code)
		}
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not valid JSON: %v", err)
		}
		if body["court"] != "centre" {
			t.Errorf(`want "centre"; got %q`, body["court"])
		}
	})

	// A value that cannot be encoded must become a 500 rather than a partial
	// body with a success status already committed.
	t.Run("an unencodable value becomes a server error", func(t *testing.T) {
		w := httptest.NewRecorder()
		rs.JSON(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
			http.StatusOK, Envelope{"bad": make(chan int)})

		if w.Code != http.StatusInternalServerError {
			t.Errorf("want 500; got %d", w.Code)
		}
	})
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		headers      map[string]string
		trustProxies bool
		want         string
	}{
		{
			name:       "direct connection",
			remoteAddr: "203.0.113.7:54321",
			want:       "203.0.113.7",
		},
		{
			name:         "forwarded chain behind a trusted proxy",
			remoteAddr:   "10.0.0.1:443",
			headers:      map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.2, 10.0.0.1"},
			trustProxies: true,
			want:         "203.0.113.7",
		},
		{
			name:         "single forwarded value",
			remoteAddr:   "10.0.0.1:443",
			headers:      map[string]string{"X-Forwarded-For": " 203.0.113.7 "},
			trustProxies: true,
			want:         "203.0.113.7",
		},
		{
			name:         "X-Real-IP when there is no chain",
			remoteAddr:   "10.0.0.1:443",
			headers:      map[string]string{"X-Real-IP": "203.0.113.9"},
			trustProxies: true,
			want:         "203.0.113.9",
		},
		{
			// The decisive case: with no proxy in front, these headers are
			// attacker-supplied and must be ignored, or any client can forge
			// the address that rate limiting and the audit trail record.
			name:         "headers are ignored when proxies are not trusted",
			remoteAddr:   "203.0.113.7:54321",
			headers:      map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8"},
			trustProxies: false,
			want:         "203.0.113.7",
		},
		{
			name:       "unparseable RemoteAddr falls through",
			remoteAddr: "/tmp/app.sock",
			want:       "/tmp/app.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			if got := ClientIP(r, tt.trustProxies); got != tt.want {
				t.Errorf("ClientIP() = %q; want %q", got, tt.want)
			}
		})
	}
}

// TestContextSetComplexScopesTheTenant pins the one line that puts every
// owner-guarded route under the tenant policies.
//
// ContextSetComplex is the only production caller's only way to set a tenant:
// middleware.RequireComplexOwner calls it once ownership is verified, and
// nothing else in the request path stamps app.complex_id. If it ever goes back
// to storing only the complex, every owner-scoped session becomes unscoped —
// which under those policies means every query returns nothing, so the symptom
// would be an outage rather than a leak, but it would still be this line that
// caused it.
//
// It is also the assumption TestRowLevelSecurityIsTheSecondWallWithTheHandler-
// ComparisonRemoved in internal/data rests on: that test scopes its context
// with data.ContextWithTenant directly, because internal/data cannot import
// this package, and this is what says the two are the same thing.
func TestContextSetComplexScopesTheTenant(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	if _, ok := data.TenantFromContext(r.Context()); ok {
		t.Error("an untouched request must carry no tenant")
	}

	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo"}
	r = ContextSetComplex(r, complex)

	got, ok := data.TenantFromContext(r.Context())
	if !ok {
		t.Fatal("a request carrying a verified complex must be scoped to it for the database too")
	}
	if got != complex.ID {
		t.Errorf("scoped to tenant %s; want %s", got, complex.ID)
	}
	if data.TenantBypassed(r.Context()) {
		t.Error("a request scoped to one complex must not also carry the cross-tenant bypass")
	}
}
