package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/platform/config"
)

// TestSentryReleaseFallsBackToTheBuildVersion pins what a deployment that
// never sets SENTRY_RELEASE reports. It used to be the literal "vibe@1.0.0"
// for every build ever shipped, which grouped four months of releases into one
// Sentry release and made "did this start with the last deploy?"
// unanswerable.
func TestSentryReleaseFallsBackToTheBuildVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	version = "abc1234"

	if got := sentryRelease(config.Config{}); got != "vibe@abc1234" {
		t.Errorf("sentryRelease = %q, want vibe@abc1234", got)
	}
}

func TestSentryReleasePrefersTheEnvironment(t *testing.T) {
	cfg := config.Config{Sentry: config.Sentry{Release: "vibe@deadbee"}}
	if got := sentryRelease(cfg); got != "vibe@deadbee" {
		t.Errorf("sentryRelease = %q, want the configured release", got)
	}
}

// TestEveryLogLineCarriesServiceEnvAndVersion is the other half: a line in a
// shared log stream that does not say which service, deployment and build
// wrote it is a line that cannot be correlated with anything.
func TestEveryLogLineCarriesServiceEnvAndVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	version = "abc1234"

	var out strings.Builder
	baseLogger(config.Config{Env: "production"}, &out).Info("hello")

	var line map[string]any
	if err := json.Unmarshal([]byte(out.String()), &line); err != nil {
		t.Fatalf("production logs are not JSON: %v (%q)", err, out.String())
	}
	for key, want := range map[string]string{"service": "vibe-api", "env": "production", "version": "abc1234"} {
		if line[key] != want {
			t.Errorf("%s = %v, want %q", key, line[key], want)
		}
	}
}

func TestOutsideProductionTheLoggerIsText(t *testing.T) {
	var out strings.Builder
	baseLogger(config.Config{Env: "development"}, &out).Info("hello")

	got := out.String()
	if !strings.Contains(got, "service=vibe-api") || !strings.Contains(got, "env=development") {
		t.Errorf("log line %q is missing its base attributes", got)
	}
}
