package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/platform/config"
)

// env builds a Lookup over a literal map, so a test names exactly the
// variables it is about and nothing else exists.
func env(pairs map[string]string) config.Lookup {
	return func(key string) (string, bool) {
		v, ok := pairs[key]
		return v, ok
	}
}

func TestHTTPTimeoutsAreConfigurable(t *testing.T) {
	cfg, err := config.Load(nil, env(map[string]string{
		"HTTP_READ_HEADER_TIMEOUT": "2s",
		"HTTP_READ_TIMEOUT":        "20s",
		"HTTP_WRITE_TIMEOUT":       "90s",
		"HTTP_IDLE_TIMEOUT":        "2m",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTP.ReadHeaderTimeout != 2*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want 2s", cfg.HTTP.ReadHeaderTimeout)
	}
	if cfg.HTTP.ReadTimeout != 20*time.Second {
		t.Errorf("ReadTimeout = %s, want 20s", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.WriteTimeout != 90*time.Second {
		t.Errorf("WriteTimeout = %s, want 90s", cfg.HTTP.WriteTimeout)
	}
	if cfg.HTTP.IdleTimeout != 2*time.Minute {
		t.Errorf("IdleTimeout = %s, want 2m", cfg.HTTP.IdleTimeout)
	}
}

func TestFlagsAreRead(t *testing.T) {
	cfg, err := config.Load([]string{
		"-port", "9090",
		"-env", "staging",
		"-db-dsn", "postgres://flag",
		"-migrate-only",
		"-limiter-enabled=false",
	}, env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 || cfg.Env != "staging" || cfg.DB.DSN != "postgres://flag" {
		t.Errorf("flags not applied: %+v", cfg)
	}
	if !cfg.MigrateOnly {
		t.Error("MigrateOnly: want true")
	}
	if cfg.Limiter.Enabled {
		t.Error("Limiter.Enabled: want false")
	}
}

// TestEnvironmentWinsOverFlags pins the precedence the deployment depends on:
// Railway sets variables, the start command sets flags, and the variable is
// the one an operator can change without a redeploy.
func TestEnvironmentWinsOverFlags(t *testing.T) {
	cfg, err := config.Load([]string{"-port", "9090", "-db-dsn", "postgres://flag"}, env(map[string]string{
		"PORT":         "7070",
		"DATABASE_URL": "postgres://env",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 7070 {
		t.Errorf("Port = %d, want 7070", cfg.Port)
	}
	if cfg.DB.DSN != "postgres://env" {
		t.Errorf("DSN = %q, want the environment's", cfg.DB.DSN)
	}
}

// TestEnvOverridesForEveryFormerlyFlagOnlyKnob covers the seven knobs that
// used to be settable only on the command line: on Railway the start command
// is the one thing an operator cannot change without a redeploy.
func TestEnvOverridesForEveryFormerlyFlagOnlyKnob(t *testing.T) {
	cfg, err := config.Load(nil, env(map[string]string{
		"PORT":               "7070",
		"DB_MAX_OPEN_CONNS":  "50",
		"DB_MAX_IDLE_CONNS":  "20",
		"DB_MAX_IDLE_TIME":   "30m",
		"LIMITER_ENABLED":    "false",
		"LIMITER_RPS":        "2.5",
		"LIMITER_BURST":      "5",
		"FEATURE_FLAGS":      "new-checkout, dark-mode=false ,waitlist=true",
		"SENTRY_RELEASE":     "vibe@abc123",
		"PPROF_ENABLED":      "true",
		"REQUEST_LOG_SAMPLE": "10",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 7070 {
		t.Errorf("port = %d", cfg.Port)
	}
	if cfg.DB.MaxOpenConns != 50 || cfg.DB.MaxIdleConns != 20 || cfg.DB.MaxIdleTime != 30*time.Minute {
		t.Errorf("pool = %d/%d/%s", cfg.DB.MaxOpenConns, cfg.DB.MaxIdleConns, cfg.DB.MaxIdleTime)
	}
	if cfg.Limiter.Enabled || cfg.Limiter.RPS != 2.5 || cfg.Limiter.Burst != 5 {
		t.Errorf("limiter = %v/%v/%v", cfg.Limiter.Enabled, cfg.Limiter.RPS, cfg.Limiter.Burst)
	}
	if !cfg.PProf || cfg.RequestLogSample != 10 || cfg.Sentry.Release != "vibe@abc123" {
		t.Errorf("misc = %v/%d/%q", cfg.PProf, cfg.RequestLogSample, cfg.Sentry.Release)
	}
	if !cfg.Features.Enabled("new-checkout") || !cfg.Features.Enabled("waitlist") {
		t.Errorf("features: %v", cfg.Features)
	}
	if cfg.Features.Enabled("dark-mode") {
		t.Error("dark-mode: an explicit =false is off")
	}
	if cfg.Features.Enabled("never-mentioned") {
		t.Error("a flag nobody named is off")
	}
}

// TestAnUnparseableValueIsAnErrorNotADefault is the whole point of the loader:
// eleven variables used to swallow the error and boot on the default, so a
// deployment that asked for a thirty-second statement timeout and typed it
// wrong ran on fifteen with nothing said.
func TestAnUnparseableValueIsAnErrorNotADefault(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"PORT", "eighty"},
		{"DB_MAX_OPEN_CONNS", "lots"},
		{"DB_MAX_IDLE_TIME", "15"},
		{"DB_STATEMENT_TIMEOUT", "30 seconds"},
		{"DB_SLOW_QUERY_THRESHOLD", "half a second"},
		{"DB_AUTO_MIGRATE", "yes please"},
		{"LIMITER_ENABLED", "sometimes"},
		{"LIMITER_RPS", "fast"},
		{"LIMITER_BURST", "many"},
		{"REQUEST_LOG_SAMPLE", "abc"},
		{"BOOKING_GRACE_PERIOD", "fifteen"},
		{"BOOKING_PAYMENT_EXPIRY", "soon"},
		{"BOOKING_CANCELLATION_WINDOW", "1 day"},
		{"BOOKING_SLOT_LOCK_TTL", "-5m"},
		{"BOOKING_LINK_TOKEN_BUFFER", "24"},
		{"PPROF_ENABLED", "enabled"},
		{"FEATURE_FLAGS", "checkout=maybe"},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			_, err := config.Load(nil, env(map[string]string{tc.key: tc.value}))
			if err == nil {
				t.Fatalf("Load(%s=%q): want an error, got none", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error %q does not name %s", err, tc.key)
			}
		})
	}
}

// TestZeroIsAcceptedWhereItWasBeforeTheRegression guards a CRITICAL fix:
// requiring these values to be strictly positive rejected a zero that each
// field's own consumer already treats as meaningful, not unparseable — see
// Config.RequestLogSample and middleware's sampler (0 logs everything, same
// as 1), Limiter.Burst (0 is a valid, if severe, ceiling), and
// stores.defaultPaymentExpiry / validateBootConfig for the two booking
// windows. Fail-fast belongs to values nobody could parse, not to values a
// downstream consumer already handles.
func TestZeroIsAcceptedWhereItWasBeforeTheRegression(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"REQUEST_LOG_SAMPLE", "0"},
		{"LIMITER_BURST", "0"},
		{"BOOKING_PAYMENT_EXPIRY", "0"},
		{"BOOKING_SLOT_LOCK_TTL", "0"},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			if _, err := config.Load(nil, env(map[string]string{tc.key: tc.value})); err != nil {
				t.Fatalf("Load(%s=%q): %v, want no error", tc.key, tc.value, err)
			}
		})
	}
}

// TestRequestLogSampleZeroLoadsAsZero pins the specific regression: Config.RequestLogSample
// documents that 0 and 1 both mean "log every request" (middleware's sampler treats <= 1 the
// same way), so 0 must come through as 0 rather than being rejected or coerced to the flag's
// default of 1.
func TestRequestLogSampleZeroLoadsAsZero(t *testing.T) {
	cfg, err := config.Load(nil, env(map[string]string{"REQUEST_LOG_SAMPLE": "0"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RequestLogSample != 0 {
		t.Errorf("RequestLogSample = %d, want 0 (same effective sampling as before)", cfg.RequestLogSample)
	}
}

// TestEveryBadValueIsReported means one restart tells an operator about all of
// it rather than one variable at a time.
func TestEveryBadValueIsReported(t *testing.T) {
	_, err := config.Load(nil, env(map[string]string{
		"PORT":              "eighty",
		"DB_MAX_OPEN_CONNS": "lots",
	}))
	if err == nil {
		t.Fatal("Load: want an error")
	}
	if !strings.Contains(err.Error(), "PORT") || !strings.Contains(err.Error(), "DB_MAX_OPEN_CONNS") {
		t.Errorf("error %q does not name both variables", err)
	}
}

// TestAnEmptyValueIsTreatedAsUnset keeps a variable declared but left blank in
// a deployment's environment from wiping out a flag's default.
func TestAnEmptyValueIsTreatedAsUnset(t *testing.T) {
	cfg, err := config.Load([]string{"-port", "9090"}, env(map[string]string{
		"PORT":         "",
		"FRONTEND_URL": "",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want the flag's 9090", cfg.Port)
	}
	if cfg.FrontendURL != "http://localhost:5173" {
		t.Errorf("FrontendURL = %q, want the default", cfg.FrontendURL)
	}
}

// TestPublicURLIsOptionalAndLoaded: PUBLIC_URL is the storefront origin CORS
// also accepts. It loads from the environment and from -public-url, and an
// empty variable leaves it unset without touching FRONTEND_URL.
func TestPublicURLIsOptionalAndLoaded(t *testing.T) {
	cfg, err := config.Load(nil, env(map[string]string{"PUBLIC_URL": "https://vibe.com.ar"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PublicURL != "https://vibe.com.ar" {
		t.Errorf("PublicURL = %q, want the environment value", cfg.PublicURL)
	}
	if cfg.FrontendURL != "http://localhost:5173" {
		t.Errorf("FrontendURL = %q, want its default untouched", cfg.FrontendURL)
	}

	cfg, err = config.Load([]string{"-public-url", "https://vibe.com.ar"}, env(nil))
	if err != nil {
		t.Fatalf("Load with -public-url: %v", err)
	}
	if cfg.PublicURL != "https://vibe.com.ar" {
		t.Errorf("PublicURL = %q from the flag, want https://vibe.com.ar", cfg.PublicURL)
	}

	cfg, err = config.Load(nil, env(map[string]string{"PUBLIC_URL": ""}))
	if err != nil {
		t.Fatalf("Load with an empty PUBLIC_URL: %v", err)
	}
	if cfg.PublicURL != "" {
		t.Errorf("PublicURL = %q for an empty variable, want unset", cfg.PublicURL)
	}
}

// TestPublicBaseURLIsPublicURLOrFrontendURL pins the origin every public
// player-facing link is built on: PUBLIC_URL when set, FRONTEND_URL while it
// is empty.
func TestPublicBaseURLIsPublicURLOrFrontendURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "PUBLIC_URL wins when set",
			cfg:  config.Config{FrontendURL: "http://localhost:5173", PublicURL: "https://vibe.com.ar"},
			want: "https://vibe.com.ar",
		},
		{
			name: "FRONTEND_URL is the fallback while PUBLIC_URL is empty",
			cfg:  config.Config{FrontendURL: "https://app.vibe.com.ar"},
			want: "https://app.vibe.com.ar",
		},
		{
			name: "nothing set yields nothing",
			cfg:  config.Config{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			if got := cfg.PublicBaseURL(); got != tt.want {
				t.Errorf("PublicBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestHelpIsNotAMisconfiguration keeps -h printing the flags and exiting zero,
// which is what the ~70 flags' own documentation is.
// TestGoogleOAuthClientSecretIsLoaded pins the one new variable the OIDC
// authorization-code flow needs: GoogleOAuthEnabled requires it alongside
// the client id, so a typo in its name would silently keep that flow
// disabled in production with no error anywhere.
func TestGoogleOAuthClientSecretIsLoaded(t *testing.T) {
	cfg, err := config.Load(nil, env(map[string]string{
		"GOOGLE_OAUTH_CLIENT_ID":     "client-id.apps.googleusercontent.com",
		"GOOGLE_OAUTH_CLIENT_SECRET": "shh-its-a-secret",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Google.OAuthClientID != "client-id.apps.googleusercontent.com" {
		t.Errorf("OAuthClientID = %q", cfg.Google.OAuthClientID)
	}
	if cfg.Google.OAuthClientSecret != "shh-its-a-secret" {
		t.Errorf("OAuthClientSecret = %q", cfg.Google.OAuthClientSecret)
	}
}

func TestHelpIsNotAMisconfiguration(t *testing.T) {
	_, err := config.Load([]string{"-h"}, env(nil))
	if !errors.Is(err, config.ErrHelp) {
		t.Fatalf("Load(-h) = %v, want ErrHelp", err)
	}
}

// TestTurnstileBookingEnforceIsOptIn pins the storefront bot check's switch:
// off unless the operator says so, so a deploy that never heard of it keeps
// taking bookings exactly as before.
func TestTurnstileBookingEnforceIsOptIn(t *testing.T) {
	cfg, err := config.Load(nil, env(map[string]string{"TURNSTILE_BOOKING_ENFORCE": "true"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Turnstile.BookingEnforce {
		t.Error("TURNSTILE_BOOKING_ENFORCE=true must turn BookingEnforce on")
	}

	_, err = config.Load(nil, env(map[string]string{"TURNSTILE_BOOKING_ENFORCE": "maybe"}))
	if err == nil || !strings.Contains(err.Error(), "TURNSTILE_BOOKING_ENFORCE") {
		t.Errorf("an unparseable TURNSTILE_BOOKING_ENFORCE must be an error naming it; got %v", err)
	}
}

// TestMPAPIBaseURLLoadsFromEnv covers the fake MercadoPago seam: unset is the
// real API (empty), and MP_API_BASE_URL reaches the config.
func TestMPAPIBaseURLLoadsFromEnv(t *testing.T) {
	cfg, err := config.Load(nil, env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MP.APIBaseURL != "" {
		t.Errorf("default APIBaseURL = %q, want empty", cfg.MP.APIBaseURL)
	}

	cfg, err = config.Load(nil, env(map[string]string{"MP_API_BASE_URL": "http://localhost:8090"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MP.APIBaseURL != "http://localhost:8090" {
		t.Errorf("APIBaseURL = %q, want http://localhost:8090", cfg.MP.APIBaseURL)
	}
}
