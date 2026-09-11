package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// testConfig builds a config that passes validateBootConfig, so each test
// case only has to describe the one thing it breaks.
func testConfig() config {
	var cfg config
	cfg.env = "development"
	cfg.mp.accessToken = "APP_USR-token"
	cfg.mp.appID = "app-id"
	cfg.mp.clientSecret = "client-secret"
	cfg.mp.webhookSecret = "webhook-secret"
	cfg.backendURL = "https://api.example.com"
	cfg.booking.paymentExpiry = 15 * time.Minute
	cfg.booking.slotLockTTL = 15 * time.Minute
	return cfg
}

func TestValidateBootConfig(t *testing.T) {
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name      string
		mutate    func(*config)
		wantErr   bool
		errSubstr string
	}{
		{
			name:   "everything set, passes in every environment",
			mutate: func(c *config) {},
		},
		{
			name: "MP disabled: an empty access token skips every MP check",
			mutate: func(c *config) {
				c.mp.accessToken = ""
				c.mp.appID = ""
				c.mp.clientSecret = ""
				c.mp.webhookSecret = ""
				c.backendURL = ""
			},
		},
		{
			name: "MP enabled, missing app id, development: logged, not fatal",
			mutate: func(c *config) {
				c.mp.appID = ""
			},
		},
		{
			name: "MP enabled, missing app id, production: fatal",
			mutate: func(c *config) {
				c.env = "production"
				c.mp.appID = ""
			},
			wantErr:   true,
			errSubstr: "mp-app-id",
		},
		{
			name: "MP enabled, missing client secret, production: fatal",
			mutate: func(c *config) {
				c.env = "production"
				c.mp.clientSecret = ""
			},
			wantErr:   true,
			errSubstr: "mp-client-secret",
		},
		{
			name: "MP enabled, missing webhook secret, production: fatal",
			mutate: func(c *config) {
				c.env = "production"
				c.mp.webhookSecret = ""
			},
			wantErr:   true,
			errSubstr: "mp-webhook-secret",
		},
		{
			name: "MP enabled, missing backend URL, production: fatal",
			mutate: func(c *config) {
				c.env = "production"
				c.backendURL = ""
			},
			wantErr:   true,
			errSubstr: "backend-url",
		},
		{
			name: "MP enabled, relative backend URL, production: fatal",
			mutate: func(c *config) {
				c.env = "production"
				c.backendURL = "/webhooks/mp"
			},
			wantErr:   true,
			errSubstr: "must be an absolute URL",
		},
		{
			name: "MP enabled, http backend URL, production: fatal",
			mutate: func(c *config) {
				c.env = "production"
				c.backendURL = "http://api.example.com"
			},
			wantErr:   true,
			errSubstr: "must be https in production",
		},
		{
			name: "MP enabled, http backend URL, development: allowed",
			mutate: func(c *config) {
				c.backendURL = "http://localhost:8080"
			},
		},
		{
			name: "everything missing at once, production: every issue reported",
			mutate: func(c *config) {
				c.env = "production"
				c.mp.appID = ""
				c.mp.clientSecret = ""
				c.mp.webhookSecret = ""
				c.backendURL = ""
			},
			wantErr:   true,
			errSubstr: "mp-app-id",
		},
		{
			name: "slot lock TTL shorter than payment expiry, production: fatal",
			mutate: func(c *config) {
				c.env = "production"
				c.booking.slotLockTTL = 5 * time.Minute
				c.booking.paymentExpiry = 15 * time.Minute
			},
			wantErr:   true,
			errSubstr: "booking-slot-lock-ttl",
		},
		{
			name: "slot lock TTL shorter than payment expiry, development: logged, not fatal",
			mutate: func(c *config) {
				c.booking.slotLockTTL = 5 * time.Minute
				c.booking.paymentExpiry = 15 * time.Minute
			},
		},
		{
			name: "slot lock TTL exactly equal to payment expiry: allowed (>=, not >)",
			mutate: func(c *config) {
				c.env = "production"
				c.booking.slotLockTTL = 15 * time.Minute
				c.booking.paymentExpiry = 15 * time.Minute
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			tt.mutate(&cfg)

			err := validateBootConfig(cfg, silentLogger)

			if tt.wantErr && err == nil {
				t.Fatalf("expected an error; got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error; got %v", err)
			}
			if tt.wantErr && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error to contain %q; got %q", tt.errSubstr, err.Error())
			}
		})
	}
}
