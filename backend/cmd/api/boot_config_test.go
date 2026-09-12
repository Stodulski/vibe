package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/platform/config"
)

// testConfig builds a config that passes validateBootConfig, so each test
// case only has to describe the one thing it breaks.
func testConfig() config.Config {
	var cfg config.Config
	cfg.Env = "development"
	cfg.MP.AccessToken = "APP_USR-token"
	cfg.MP.AppID = "app-id"
	cfg.MP.ClientSecret = "client-secret"
	cfg.MP.WebhookSecret = "webhook-secret"
	cfg.BackendURL = "https://api.example.com"
	cfg.Booking.PaymentExpiry = 15 * time.Minute
	cfg.Booking.SlotLockTTL = 15 * time.Minute
	cfg.HTTP.WriteTimeout = 60 * time.Second
	return cfg
}

func TestValidateBootConfig(t *testing.T) {
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name      string
		mutate    func(*config.Config)
		wantErr   bool
		errSubstr string
	}{
		{
			name:   "everything set, passes in every environment",
			mutate: func(c *config.Config) {},
		},
		{
			name: "MP disabled: an empty access token skips every MP check",
			mutate: func(c *config.Config) {
				c.MP.AccessToken = ""
				c.MP.AppID = ""
				c.MP.ClientSecret = ""
				c.MP.WebhookSecret = ""
				c.BackendURL = ""
			},
		},
		{
			name: "MP enabled, missing app id, development: logged, not fatal",
			mutate: func(c *config.Config) {
				c.MP.AppID = ""
			},
		},
		{
			name: "MP enabled, missing app id, production: fatal",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.MP.AppID = ""
			},
			wantErr:   true,
			errSubstr: "mp-app-id",
		},
		{
			name: "MP enabled, missing client secret, production: fatal",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.MP.ClientSecret = ""
			},
			wantErr:   true,
			errSubstr: "mp-client-secret",
		},
		{
			name: "MP enabled, missing webhook secret, production: fatal",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.MP.WebhookSecret = ""
			},
			wantErr:   true,
			errSubstr: "mp-webhook-secret",
		},
		{
			name: "MP enabled, missing backend URL, production: fatal",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.BackendURL = ""
			},
			wantErr:   true,
			errSubstr: "backend-url",
		},
		{
			name: "MP enabled, relative backend URL, production: fatal",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.BackendURL = "/webhooks/mp"
			},
			wantErr:   true,
			errSubstr: "must be an absolute URL",
		},
		{
			name: "MP enabled, http backend URL, production: fatal",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.BackendURL = "http://api.example.com"
			},
			wantErr:   true,
			errSubstr: "must be https in production",
		},
		{
			name: "MP enabled, http backend URL, development: allowed",
			mutate: func(c *config.Config) {
				c.BackendURL = "http://localhost:8080"
			},
		},
		{
			name: "everything missing at once, production: every issue reported",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.MP.AppID = ""
				c.MP.ClientSecret = ""
				c.MP.WebhookSecret = ""
				c.BackendURL = ""
			},
			wantErr:   true,
			errSubstr: "mp-app-id",
		},
		{
			name: "slot lock TTL shorter than payment expiry, production: fatal",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.Booking.SlotLockTTL = 5 * time.Minute
				c.Booking.PaymentExpiry = 15 * time.Minute
			},
			wantErr:   true,
			errSubstr: "booking-slot-lock-ttl",
		},
		{
			name: "slot lock TTL shorter than payment expiry, development: logged, not fatal",
			mutate: func(c *config.Config) {
				c.Booking.SlotLockTTL = 5 * time.Minute
				c.Booking.PaymentExpiry = 15 * time.Minute
			},
		},
		{
			name: "slot lock TTL exactly equal to payment expiry: allowed (>=, not >)",
			mutate: func(c *config.Config) {
				c.Env = "production"
				c.Booking.SlotLockTTL = 15 * time.Minute
				c.Booking.PaymentExpiry = 15 * time.Minute
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

func TestExportBudgetFor(t *testing.T) {
	tests := []struct {
		name         string
		writeTimeout time.Duration
		want         time.Duration
	}{
		{"60s write timeout leaves three quarters for the export", 60 * time.Second, 45 * time.Second},
		{"30s write timeout leaves three quarters for the export", 30 * time.Second, 22*time.Second + 500*time.Millisecond},
		{"unbounded write timeout falls back to the default budget", 0, 50 * time.Second},
		{"a tiny write timeout is floored rather than starved further", 4 * time.Second, 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exportBudgetFor(tt.writeTimeout); got != tt.want {
				t.Errorf("exportBudgetFor(%s) = %s, want %s", tt.writeTimeout, got, tt.want)
			}
		})
	}
}

func TestPublicNetworkWarnings(t *testing.T) {
	tests := []struct {
		name  string
		db    string
		redis string
		want  []string // substrings each warning must contain, in order
	}{
		{
			name:  "both on the private network",
			db:    "postgres://vibe:secret@postgres.railway.internal:5432/railway",
			redis: "redis://default:secret@redis.railway.internal:6379",
		},
		{
			name:  "the database is public",
			db:    "postgres://vibe:secret@containers-us-west-1.railway.app:7432/railway",
			redis: "redis://default:secret@redis.railway.internal:6379",
			want:  []string{"DATABASE_URL"},
		},
		{
			name:  "both are public",
			db:    "postgres://vibe:secret@db.example.com:5432/vibe",
			redis: "redis://cache.example.com:6379",
			want:  []string{"DATABASE_URL", "REDIS_URL"},
		},
		{
			name: "an unset URL is not a warning",
			db:   "postgres://vibe:secret@postgres.railway.internal:5432/railway",
		},
		{
			name: "an unparseable URL says so rather than passing",
			db:   "://nonsense",
			want: []string{"could not be parsed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.DB.DSN = tt.db
			cfg.Redis.URL = tt.redis

			got := publicNetworkWarnings(cfg)
			if len(got) != len(tt.want) {
				t.Fatalf("want %d warnings %v; got %d: %v", len(tt.want), tt.want, len(got), got)
			}
			for i, substr := range tt.want {
				if !strings.Contains(got[i], substr) {
					t.Errorf("warning %d should mention %q; got %q", i, substr, got[i])
				}
			}
		})
	}
}

// The two URLs carry a password each. A warning that quoted either of them
// would put a production credential in the deploy log and in Sentry, which is
// a worse outcome than the misconfiguration it is reporting.
func TestPublicNetworkWarningsNeverQuoteTheURL(t *testing.T) {
	cfg := testConfig()
	cfg.DB.DSN = "postgres://vibe:hunter2@db.example.com:5432/vibe"
	cfg.Redis.URL = "redis://default:swordfish@cache.example.com:6379"

	warnings := publicNetworkWarnings(cfg)
	if len(warnings) != 2 {
		t.Fatalf("want 2 warnings; got %v", warnings)
	}
	for _, w := range warnings {
		for _, leak := range []string{"hunter2", "swordfish", cfg.DB.DSN, cfg.Redis.URL} {
			if strings.Contains(w, leak) {
				t.Errorf("warning leaks %q: %s", leak, w)
			}
		}
	}
}
