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

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(nil, env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.DB.MaxOpenConns != 25 || cfg.DB.MaxIdleConns != 10 {
		t.Errorf("pool = %d/%d, want 25/10", cfg.DB.MaxOpenConns, cfg.DB.MaxIdleConns)
	}
	if cfg.DB.StatementTimeout != 15*time.Second {
		t.Errorf("StatementTimeout = %s, want 15s", cfg.DB.StatementTimeout)
	}
	if cfg.DB.SlowQueryThreshold != 500*time.Millisecond {
		t.Errorf("SlowQueryThreshold = %s, want 500ms", cfg.DB.SlowQueryThreshold)
	}
	if !cfg.Limiter.Enabled || cfg.Limiter.RPS != 10 || cfg.Limiter.Burst != 20 {
		t.Errorf("limiter = %v/%v/%v, want true/10/20", cfg.Limiter.Enabled, cfg.Limiter.RPS, cfg.Limiter.Burst)
	}
	if cfg.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want 587", cfg.SMTP.Port)
	}
	if cfg.Booking.SlotLockTTL != 15*time.Minute || cfg.Booking.LinkTokenBuffer != 24*time.Hour {
		t.Errorf("booking windows = %s/%s", cfg.Booking.SlotLockTTL, cfg.Booking.LinkTokenBuffer)
	}
	if cfg.R2.BucketName != "vibe" {
		t.Errorf("R2.BucketName = %q, want vibe", cfg.R2.BucketName)
	}
	if cfg.FrontendURL != "http://localhost:5173" {
		t.Errorf("FrontendURL = %q", cfg.FrontendURL)
	}
}

// The four server timeouts are the difference between a bounded connection and
// one a peer can hold open indefinitely, so their defaults are asserted rather
// than left to whoever last edited the flag set. The write timeout in
// particular has to stay above reporting.ExportBudget, which cmd/api checks at
// boot.
func TestHTTPTimeoutDefaults(t *testing.T) {
	cfg, err := config.Load(nil, env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTP.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want 5s", cfg.HTTP.ReadHeaderTimeout)
	}
	if cfg.HTTP.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %s, want 5s", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.WriteTimeout != 60*time.Second {
		t.Errorf("WriteTimeout = %s, want 60s", cfg.HTTP.WriteTimeout)
	}
	if cfg.HTTP.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %s, want 60s", cfg.HTTP.IdleTimeout)
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
		"SMTP_PORT":          "2525",
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

	if cfg.Port != 7070 || cfg.SMTP.Port != 2525 {
		t.Errorf("ports = %d/%d", cfg.Port, cfg.SMTP.Port)
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
		{"SMTP_PORT", "-1"},
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

func TestAnUnknownFlagIsAnError(t *testing.T) {
	if _, err := config.Load([]string{"-nonsense"}, env(nil)); err == nil {
		t.Fatal("Load: want an error for an unknown flag")
	}
}

func TestFeaturesZeroValueSaysNo(t *testing.T) {
	var f config.Features
	if f.Enabled("anything") {
		t.Error("the zero Features says yes")
	}
	if len(f.Names()) != 0 {
		t.Errorf("Names = %v, want none", f.Names())
	}
}

// TestHelpIsNotAMisconfiguration keeps -h printing the flags and exiting zero,
// which is what the ~70 flags' own documentation is.
func TestHelpIsNotAMisconfiguration(t *testing.T) {
	_, err := config.Load([]string{"-h"}, env(nil))
	if !errors.Is(err, config.ErrHelp) {
		t.Fatalf("Load(-h) = %v, want ErrHelp", err)
	}

	var out strings.Builder
	config.Usage(&out)
	for _, want := range []string{"-port", "PORT", "-mp-credential-keys"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("Usage does not mention %q", want)
		}
	}
}
