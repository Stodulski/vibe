package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
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
func validateBootConfig(cfg config, logger *slog.Logger) error {
	var missing []string

	if cfg.mp.accessToken != "" {
		missing = append(missing, missingMPConfig(cfg)...)
	}

	if cfg.booking.slotLockTTL < cfg.booking.paymentExpiry {
		missing = append(missing, fmt.Sprintf(
			"booking-slot-lock-ttl (%s) must be >= booking-payment-expiry (%s), "+
				"otherwise a slot lock can expire while its payment window is still open",
			cfg.booking.slotLockTTL, cfg.booking.paymentExpiry))
	}

	if len(missing) == 0 {
		return nil
	}

	if cfg.env == "production" {
		return fmt.Errorf("invalid boot configuration: %s", strings.Join(missing, "; "))
	}
	logger.Error("invalid boot configuration (continuing outside production)",
		"issues", strings.Join(missing, "; "))
	return nil
}

// missingMPConfig reports what a MercadoPago-enabled deployment (a non-empty
// access token) is missing to work end to end.
func missingMPConfig(cfg config) []string {
	var missing []string

	if cfg.mp.appID == "" {
		missing = append(missing, "mp-app-id/MP_APP_ID is required when mp-access-token/MP_ACCESS_TOKEN is set")
	}
	if cfg.mp.clientSecret == "" {
		missing = append(missing, "mp-client-secret/MP_CLIENT_SECRET is required when mp-access-token/MP_ACCESS_TOKEN is set")
	}
	if cfg.mp.webhookSecret == "" {
		missing = append(missing, "mp-webhook-secret/MP_WEBHOOK_SECRET is required when mp-access-token/MP_ACCESS_TOKEN is set")
	}
	missing = append(missing, validateBackendURL(cfg)...)

	return missing
}

// validateBackendURL checks that backend-url/BACKEND_URL is set, absolute,
// and — in production — https. MercadoPago redirects OAuth callbacks and
// posts webhooks to it; a relative or non-https value fails those silently
// rather than at boot.
func validateBackendURL(cfg config) []string {
	if cfg.backendURL == "" {
		return []string{"backend-url/BACKEND_URL is required when mp-access-token/MP_ACCESS_TOKEN is set"}
	}
	u, err := url.Parse(cfg.backendURL)
	if err != nil || !u.IsAbs() {
		return []string{fmt.Sprintf("backend-url/BACKEND_URL %q must be an absolute URL", cfg.backendURL)}
	}
	if cfg.env == "production" && u.Scheme != "https" {
		return []string{fmt.Sprintf("backend-url/BACKEND_URL %q must be https in production", cfg.backendURL)}
	}
	return nil
}
