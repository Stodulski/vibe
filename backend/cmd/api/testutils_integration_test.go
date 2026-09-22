//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/platform/config"
	platformdb "github.com/stodulski/vibe-server/internal/platform/db"
	"github.com/stodulski/vibe-server/internal/stores"
	"golang.org/x/crypto/bcrypt"
)

const integrationJWTSecret = "e2e-test-secret-key-32-bytes-long!"

func setupTestDB(t *testing.T) *platformdb.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// The same opener production uses, so this suite runs against a pool
	// tuned the way the real one is — exec mode included, which is where a
	// []byte bound to a jsonb column turns into bytea.
	pool, err := platformdb.Open(ctx, platformdb.Config{
		DSN:          dsn,
		MaxOpenConns: 5,
		MaxIdleConns: 1,
		MaxIdleTime:  time.Minute,
	})
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func cleanupDB(t *testing.T, pool *platformdb.Pool) {
	t.Helper()

	tables := []string{
		"audit_log", "failed_refunds", "payments", "bookings",
		"blocked_slots", "slot_locks", "court_prices", "courts",
		"complex_schedules", "complexes", "email_verification_tokens",
		"password_reset_tokens", "refresh_tokens", "users",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := fmt.Sprintf("TRUNCATE %s CASCADE", strings.Join(tables, ", "))
	if _, err := pool.Exec(ctx, query); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
}

// newIntegrationApp builds a real *application via newApplication, the same
// constructor the unit harness (testutils_test.go) and main() both use — the
// only substitution boundary is deps: a real *platformdb.Pool instead of mocks,
// so deps.models comes from data.stores.New(pool, ...) and deps.db is the pool
// itself. deps.rdb stays nil, which routes newApplication to the same
// in-memory blacklist, hub, rate limiter and memoryQueue fallback the old
// hand-built struct literal wired directly.
//
// Before this, the struct literal below set only 9 of the application's ~30
// fields — middleware (among many others) was left nil, so every request
// panicked in middleware.Wrap before any handler ran and this whole
// integration suite never executed a single assertion.
func newIntegrationApp(t *testing.T, pool *platformdb.Pool, opts ...func(*config.Config)) *application {
	t.Helper()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = integrationJWTSecret
	}

	cfg := config.Config{
		Env: "development",
		JWT: config.JWT{Secret: jwtSecret},
		// bcrypt.MinCost, for the reason the unit harness gives: this suite
		// registers and signs in real users against a real database.
		PasswordHashCost: bcrypt.MinCost,
		// Placeholder credentials, matching what the previous hand-built
		// application literal passed directly to mp.NewMPClient /
		// whatsapp.NewWAClient. No test talks to the real MercadoPago or
		// WhatsApp APIs; these only need to be non-empty so client
		// construction mirrors production shape.
		MP: config.MP{
			AccessToken:   "test",
			WebhookSecret: "test",
			AppID:         "test",
			ClientSecret:  "test",
			// Unused here: newApplication never re-derives a keyring from
			// cfg.MP.CredentialKeys — that parsing only happens once, in
			// main()'s boot validation. deps.models below carries its own
			// independent Config with no Keys, matching production shape
			// where nothing in this suite ever seals or opens a real
			// credential (see integration_test.go's "fake MP connection").
			CredentialKeys: "test",
		},
		WhatsApp: config.WhatsApp{
			Token:       "test",
			PhoneID:     "test",
			VerifyToken: "test",
			AppSecret:   "test",
		},
		Limiter:     config.Limiter{Enabled: false},
		FrontendURL: "http://localhost:5173",
	}

	// Applied before the constructor runs, and that is the whole reason this
	// hook exists. newApplication bakes cfg into middleware.Config, auth.Config
	// and the rest at build time, so a test that reaches for app.config
	// afterwards writes a field nothing reads — silently, since assigning to a
	// live struct field is not an error. Two security tests did exactly that
	// and had never run to find out.
	for _, opt := range opts {
		opt(&cfg)
	}

	app, err := newApplication(cfg, deps{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		models: stores.New(pool, stores.Config{PaymentExpiry: 15 * time.Minute}),
		db:     pool,
		// rdb, storage stay nil: in-memory blacklist/hub/limiter/queue, and
		// no R2 — the same fallback production takes without Redis/R2
		// configured.
	})
	if err != nil {
		t.Fatalf("newIntegrationApp: %v", err)
	}

	// Reaps the eviction goroutine middleware.Wrap starts, same as the unit
	// harness is documented to need (design.md).
	t.Cleanup(func() {
		close(app.shutdown)
	})

	return app
}

func newIntegrationServer(t *testing.T, app *application) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(app.routes())
	t.Cleanup(ts.Close)
	return ts
}

// integrationDo sends one request to the test server and decodes the JSON
// response. Every helper below funnels through it.
//
// It exists because integrationPost, integrationGet and integrationPut were
// three copies of the same twenty lines, and each copy dropped the
// json.Unmarshal error on the floor. A helper that swallows that error hands
// its caller a nil map, and the caller's `result["booking"]` then reads a zero
// value out of a response that never parsed — a malformed body scores a
// confident PASS. Decoding failures are now fatal, once, in one place.
func integrationDo(t *testing.T, ts *httptest.Server, method, path, body string, cookies []*http.Cookie, csrfToken string) (int, http.Header, map[string]any) {
	t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, ts.URL+path, bodyReader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, c := range cookies {
		req.AddCookie(c)
	}
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	rs, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rs.Body.Close() }()

	respBody, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			t.Fatalf("%s %s: decode response body (status %d): %v\nbody: %s", method, path, rs.StatusCode, err, respBody)
		}
	}

	return rs.StatusCode, rs.Header, result
}

// integrationPost sends a POST with JSON body. Returns status, headers, response body.
func integrationPost(t *testing.T, ts *httptest.Server, path string, body string, cookies []*http.Cookie, csrfToken string) (int, http.Header, map[string]any) {
	t.Helper()
	return integrationDo(t, ts, http.MethodPost, path, body, cookies, csrfToken)
}

// integrationGet sends a GET request. Returns status, headers, response body.
func integrationGet(t *testing.T, ts *httptest.Server, path string, cookies []*http.Cookie, csrfToken string) (int, http.Header, map[string]any) {
	t.Helper()
	return integrationDo(t, ts, http.MethodGet, path, "", cookies, csrfToken)
}

// integrationPut sends a PUT request with JSON body.
func integrationPut(t *testing.T, ts *httptest.Server, path string, body string, cookies []*http.Cookie, csrfToken string) (int, http.Header, map[string]any) {
	t.Helper()
	return integrationDo(t, ts, http.MethodPut, path, body, cookies, csrfToken)
}

// registerAndLogin creates a user and logs in, returning the auth cookies and CSRF token.
func registerAndLogin(t *testing.T, ts *httptest.Server, email, password string) ([]*http.Cookie, string) {
	t.Helper()

	// Register
	registerBody := fmt.Sprintf(`{"email":"%s","password":"%s","first_name":"Test","last_name":"User","phone":"+5491100000001"}`, email, password)
	status, _, _ := integrationPost(t, ts, "/api/v1/auth/register", registerBody, nil, "")
	if status != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", status)
	}

	// Login
	loginBody := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	status, headers, result := integrationPost(t, ts, "/api/v1/auth/login", loginBody, nil, "")
	if status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", status)
	}

	// Extract cookies
	var cookies []*http.Cookie
	for _, line := range headers.Values("Set-Cookie") {
		if c, err := http.ParseSetCookie(line); err == nil {
			cookies = append(cookies, c)
		}
	}

	// Extract CSRF token
	csrfToken, _ := result["csrf_token"].(string)

	return cookies, csrfToken
}
