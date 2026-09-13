package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/platform/config"
)

// validateBootConfig checks configuration invariants that nothing else in the
// startup sequence enforces, and that a wrong value for silently breaks a
// feature rather than failing loudly:
//
//   - MercadoPago: when an access token is configured (MP enabled) but the
//     app id, client secret, webhook secret, or backend URL is missing, the
//     integration degrades in ways that are hard to notice from the outside
//     — OAuth link-a-seller flows fail, and an unconfigured webhook secret
//     is refused explicitly elsewhere (mp.VerifyWebhookSignature), but only
//     once the first webhook arrives.
//   - The booking slot lock TTL must be at least as long as the payment
//     expiry: a lock that expires before the payment window closes lets a
//     second client claim a slot while the first client's payment can still
//     complete.
//
// In production both are fatal (the caller exits); everywhere else they are
// logged once and startup continues, matching how JWT_SECRET's weaker checks
// above are already handled — development intentionally runs without every
// MercadoPago secret set.
func validateBootConfig(cfg config.Config, logger *slog.Logger) error {
	var missing []string

	if cfg.MP.AccessToken != "" {
		missing = append(missing, missingMPConfig(cfg)...)
	}

	if cfg.Booking.SlotLockTTL < cfg.Booking.PaymentExpiry {
		missing = append(missing, fmt.Sprintf(
			"booking-slot-lock-ttl (%s) must be >= booking-payment-expiry (%s), "+
				"otherwise a slot lock can expire while its payment window is still open",
			cfg.Booking.SlotLockTTL, cfg.Booking.PaymentExpiry))
	}

	if len(missing) == 0 {
		return nil
	}

	if cfg.Env == "production" {
		return fmt.Errorf("invalid boot configuration: %s", strings.Join(missing, "; "))
	}
	logger.Error("invalid boot configuration (continuing outside production)",
		"issues", strings.Join(missing, "; "))
	return nil
}

// defaultExportBudget is exportBudgetFor's answer for an unbounded (0)
// HTTP_WRITE_TIMEOUT: the ceiling the export always had before it was derived.
const defaultExportBudget = 50 * time.Second

// minExportBudget is the floor under which a derived budget leaves the
// spreadsheet export no meaningful time to build and stream the workbook.
const minExportBudget = 5 * time.Second

// exportBudgetFor derives the spreadsheet export's time budget from
// HTTP_WRITE_TIMEOUT: three quarters of it, leaving the last quarter to flush
// the response before WriteTimeout closes the connection out from under the
// still-running handler. Below minExportBudget that quarter is too little to
// matter — a full-cap export takes about a second — so the floor takes over.
func exportBudgetFor(writeTimeout time.Duration) time.Duration {
	if writeTimeout <= 0 {
		return defaultExportBudget
	}
	if budget := writeTimeout * 3 / 4; budget >= minExportBudget {
		return budget
	}
	return minExportBudget
}

// exportUploadAllowance is added on top of exportBudgetFor's answer to build
// the export worker's own attempt timeout (see the jobs.Config.Timeouts entry
// in newApplication). exportBudgetFor bounds the synchronous route, which
// streams the workbook straight to the response; the worker does the same
// build and then also uploads the result to R2 (storage.PutObject), a step
// the synchronous route never pays for. Fifteen seconds is comfortably above
// what an upload of a few-megabyte workbook takes.
const exportUploadAllowance = 15 * time.Second

// missingMPConfig reports what a MercadoPago-enabled deployment (a non-empty
// access token) is missing to work end to end.
func missingMPConfig(cfg config.Config) []string {
	var missing []string

	if cfg.MP.AppID == "" {
		missing = append(missing, "mp-app-id/MP_APP_ID is required when mp-access-token/MP_ACCESS_TOKEN is set")
	}
	if cfg.MP.ClientSecret == "" {
		missing = append(missing, "mp-client-secret/MP_CLIENT_SECRET is required when mp-access-token/MP_ACCESS_TOKEN is set")
	}
	if cfg.MP.WebhookSecret == "" {
		missing = append(missing, "mp-webhook-secret/MP_WEBHOOK_SECRET is required when mp-access-token/MP_ACCESS_TOKEN is set")
	}
	missing = append(missing, validateBackendURL(cfg)...)

	return missing
}

// validateBackendURL checks that backend-url/BACKEND_URL is set, absolute,
// and — in production — https. MercadoPago redirects OAuth callbacks and
// posts webhooks to it; a relative or non-https value fails those silently
// rather than at boot.
func validateBackendURL(cfg config.Config) []string {
	if cfg.BackendURL == "" {
		return []string{"backend-url/BACKEND_URL is required when mp-access-token/MP_ACCESS_TOKEN is set"}
	}
	u, err := url.Parse(cfg.BackendURL)
	if err != nil || !u.IsAbs() {
		return []string{fmt.Sprintf("backend-url/BACKEND_URL %q must be an absolute URL", cfg.BackendURL)}
	}
	if cfg.Env == "production" && u.Scheme != "https" {
		return []string{fmt.Sprintf("backend-url/BACKEND_URL %q must be https in production", cfg.BackendURL)}
	}
	return nil
}

// privateNetworkSuffix is the host suffix Railway gives a service on the
// project's private network. A DATABASE_URL or REDIS_URL pointing anywhere
// else in production is reaching the datastore over the public internet.
const privateNetworkSuffix = ".railway.internal"

// publicNetworkWarnings reports the datastore URLs that do not resolve over
// Railway's private network.
//
// It is a warning rather than a boot failure on purpose: this repository is not
// Railway-only, and a self-hoster with PostgreSQL on the same box has a
// perfectly good DATABASE_URL that will never end in .railway.internal.
// Refusing to boot would make the deployment shape a hard requirement of the
// code. Saying it out loud, once, is what turns "we meant to use the private
// network" into something visible from the logs.
//
// What is at stake when it fires: a public host means every query and every
// session token crosses the internet, it is billed as egress, and the database
// is reachable from outside the project at all — which is a different security
// posture from the one the audit assumed.
//
// Only the host is reported, never the URL. Both values carry a password.
func publicNetworkWarnings(cfg config.Config) []string {
	var warnings []string
	for _, dsn := range []struct{ name, value string }{
		{"DATABASE_URL", cfg.DB.DSN},
		{"REDIS_URL", cfg.Redis.URL},
	} {
		if dsn.value == "" {
			continue
		}
		host, ok := dsnHost(dsn.value)
		if !ok {
			warnings = append(warnings, fmt.Sprintf(
				"%s could not be parsed as a URL, so its host cannot be checked against Railway's "+
					"private network", dsn.name))
			continue
		}
		if strings.HasSuffix(host, privateNetworkSuffix) {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s points at %q, which is not on Railway's private network (*%s): in production the "+
				"connection crosses the public internet, is billed as egress, and leaves the datastore "+
				"reachable from outside the project",
			dsn.name, host, privateNetworkSuffix))
	}
	return warnings
}

// dsnHost returns the host of a datastore URL, without the port and without
// anything that could carry a credential.
func dsnHost(dsn string) (string, bool) {
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() == "" {
		return "", false
	}
	return u.Hostname(), true
}
