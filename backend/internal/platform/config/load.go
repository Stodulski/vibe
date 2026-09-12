package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Lookup is the environment the loader reads. It is a parameter so a test can
// hand Load an environment without touching the process's own.
type Lookup func(key string) (string, bool)

// OSLookup reads the process environment. It is the Lookup every real boot
// passes, and it is defined here so that the process environment is read in
// exactly one package.
func OSLookup(key string) (string, bool) { return os.LookupEnv(key) }

// Load builds the configuration from command-line flags and then environment
// variables, which win over flags wherever both are set.
//
// Every value that has to be parsed — a number, a boolean, a duration — is an
// error when it cannot be, and every error is collected rather than the first
// one returned, so an operator fixing a misconfigured deployment is told about
// all of it at once instead of one variable per restart.
func Load(args []string, lookup Lookup) (Config, error) {
	var cfg Config
	fs := newFlagSet(&cfg)

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("config: parsing command-line flags: %w", err)
	}

	env := &reader{lookup: lookup}
	cfg.applyEnv(env)

	if err := errors.Join(env.errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

// ErrHelp is what Load returns, wrapped, for -h. The caller prints Usage and
// exits zero: asking what the flags are is not a misconfiguration.
var ErrHelp = flag.ErrHelp

// Usage writes every flag, its default and the environment variable that
// overrides it. It builds the same flag set Load does, so the two cannot
// describe different programs.
func Usage(w io.Writer) {
	var cfg Config
	fs := newFlagSet(&cfg)
	fs.SetOutput(w)
	fs.PrintDefaults()
}

// newFlagSet declares the whole configuration surface: every flag, its default,
// and in its usage text the environment variable that overrides it.
//
// splitting it hides which flag carries which default, which is the only thing
// it is for.
//
//nolint:funlen // one flat declaration of that surface;
func newFlagSet(cfg *Config) *flag.FlagSet {
	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.IntVar(&cfg.Port, "port", 8080, "API server port (PORT)")
	fs.StringVar(&cfg.Env, "env", "development", "Environment (development|staging|production) (ENV)")

	fs.DurationVar(&cfg.HTTP.ReadHeaderTimeout, "http-read-header-timeout", 5*time.Second,
		"Deadline for reading the request line and headers; 0 disables it (HTTP_READ_HEADER_TIMEOUT)")
	fs.DurationVar(&cfg.HTTP.ReadTimeout, "http-read-timeout", 5*time.Second,
		"Deadline for reading the whole request, headers and body; 0 disables it (HTTP_READ_TIMEOUT)")
	fs.DurationVar(&cfg.HTTP.WriteTimeout, "http-write-timeout", 60*time.Second,
		"Deadline for writing the response; must stay above the spreadsheet export budget; "+
			"0 disables it (HTTP_WRITE_TIMEOUT)")
	fs.DurationVar(&cfg.HTTP.IdleTimeout, "http-idle-timeout", 60*time.Second,
		"How long a keep-alive connection may sit idle between requests; 0 disables it (HTTP_IDLE_TIMEOUT)")

	fs.StringVar(&cfg.DB.DSN, "db-dsn", "", "PostgreSQL DSN (DATABASE_URL)")
	fs.IntVar(&cfg.DB.MaxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections (DB_MAX_OPEN_CONNS)")
	fs.IntVar(&cfg.DB.MaxIdleConns, "db-max-idle-conns", 10, "PostgreSQL max idle connections (DB_MAX_IDLE_CONNS)")
	fs.DurationVar(&cfg.DB.MaxIdleTime, "db-max-idle-time", 15*time.Minute,
		"PostgreSQL max connection idle time (DB_MAX_IDLE_TIME)")
	fs.DurationVar(&cfg.DB.StatementTimeout, "db-statement-timeout", 15*time.Second,
		"PostgreSQL statement_timeout: the server-side backstop for a query no context managed to cancel "+
			"(DB_STATEMENT_TIMEOUT)")
	fs.DurationVar(&cfg.DB.SlowQueryThreshold, "db-slow-query-threshold", 500*time.Millisecond,
		"Log a warn line for any single query slower than this; 0 disables it (DB_SLOW_QUERY_THRESHOLD)")
	fs.BoolVar(&cfg.DB.AutoMigrate, "db-auto-migrate", false,
		"Apply the embedded migration chain before serving (DB_AUTO_MIGRATE)")
	fs.StringVar(&cfg.DB.MigratorDSN, "db-migrator-dsn", "",
		"PostgreSQL DSN migrations run as; falls back to db-dsn/DATABASE_URL (DB_MIGRATOR_URL)")
	fs.BoolVar(&cfg.MigrateOnly, "migrate-only", false,
		"Apply the embedded migration chain, print the status, and exit without starting the server")

	fs.StringVar(&cfg.JWT.Secret, "jwt-secret", "", "JWT secret (JWT_SECRET)")

	fs.StringVar(&cfg.MP.AccessToken, "mp-access-token", "", "MercadoPago access token (MP_ACCESS_TOKEN)")
	fs.StringVar(&cfg.MP.WebhookSecret, "mp-webhook-secret", "", "MercadoPago webhook secret (MP_WEBHOOK_SECRET)")
	fs.StringVar(&cfg.MP.AppID, "mp-app-id", "", "MercadoPago application ID (for OAuth) (MP_APP_ID)")
	fs.StringVar(&cfg.MP.ClientSecret, "mp-client-secret", "", "MercadoPago client secret (for OAuth) (MP_CLIENT_SECRET)")
	fs.StringVar(&cfg.MP.CredentialKeys, "mp-credential-keys", "",
		"MercadoPago credential encryption keyring: kid:base64key[,kid:base64key...] "+
			"(first entry writes, every entry opens) (MP_CREDENTIAL_KEYS)")

	fs.StringVar(&cfg.WhatsApp.Token, "whatsapp-token", "", "WhatsApp API token (WHATSAPP_TOKEN)")
	fs.StringVar(&cfg.WhatsApp.PhoneID, "whatsapp-phone-id", "", "WhatsApp phone number ID (WHATSAPP_PHONE_NUMBER_ID)")
	fs.StringVar(&cfg.WhatsApp.VerifyToken, "whatsapp-verify-token", "", "WhatsApp verify token (WHATSAPP_VERIFY_TOKEN)")
	fs.StringVar(&cfg.WhatsApp.AppSecret, "whatsapp-app-secret", "",
		"WhatsApp app secret for webhook verification (WHATSAPP_APP_SECRET)")

	fs.StringVar(&cfg.Brevo.APIKey, "brevo-api-key", "", "Brevo API key (BREVO_API_KEY)")
	fs.StringVar(&cfg.Brevo.Sender, "brevo-sender", "Vibe <no-reply@vibe.com.ar>", "Email sender (BREVO_SENDER)")
	fs.StringVar(&cfg.SMTP.Host, "smtp-host", "", "SMTP host (fallback if no Brevo API key) (SMTP_HOST)")
	fs.IntVar(&cfg.SMTP.Port, "smtp-port", 587, "SMTP port (SMTP_PORT)")
	fs.StringVar(&cfg.SMTP.Username, "smtp-username", "", "SMTP username (SMTP_USERNAME)")
	fs.StringVar(&cfg.SMTP.Password, "smtp-password", "", "SMTP password (SMTP_PASSWORD)")

	fs.StringVar(&cfg.Turnstile.SecretKey, "turnstile-secret-key", "",
		"Cloudflare Turnstile secret key; enables verification on register, login and forgot-password "+
			"(TURNSTILE_SECRET_KEY)")

	fs.StringVar(&cfg.Leads.AbandonedWebhookURL, "leads-abandoned-webhook-url", "",
		"Google Apps Script webhook URL for abandoned-registration email capture (LEADS_ABANDONED_WEBHOOK_URL)")
	fs.StringVar(&cfg.Leads.AbandonedWebhookToken, "leads-abandoned-webhook-token", "",
		"Shared token the abandoned-registration webhook expects (LEADS_ABANDONED_WEBHOOK_TOKEN)")

	fs.StringVar(&cfg.R2.AccountID, "r2-account-id", "", "Cloudflare R2 account ID (R2_ACCOUNT_ID)")
	fs.StringVar(&cfg.R2.AccessKey, "r2-access-key", "", "Cloudflare R2 access key (R2_ACCESS_KEY)")
	fs.StringVar(&cfg.R2.SecretKey, "r2-secret-key", "", "Cloudflare R2 secret key (R2_SECRET_KEY)")
	fs.StringVar(&cfg.R2.BucketName, "r2-bucket-name", "vibe", "Cloudflare R2 bucket name (R2_BUCKET_NAME)")
	fs.StringVar(&cfg.R2.PublicURL, "r2-public-url", "", "R2 public base URL for serving images (R2_PUBLIC_URL)")

	fs.StringVar(&cfg.FrontendURL, "frontend-url", "http://localhost:5173", "Frontend URL (FRONTEND_URL)")
	fs.StringVar(&cfg.BackendURL, "backend-url", "", "Backend public URL (for webhooks) (BACKEND_URL)")
	fs.StringVar(&cfg.CookieDomain, "cookie-domain", "", "Cookie domain (e.g. .skymait.com) (COOKIE_DOMAIN)")

	fs.BoolVar(&cfg.Limiter.Enabled, "limiter-enabled", true, "Enable rate limiter (LIMITER_ENABLED)")
	fs.Float64Var(&cfg.Limiter.RPS, "limiter-rps", 10, "Rate limiter requests per second (LIMITER_RPS)")
	fs.IntVar(&cfg.Limiter.Burst, "limiter-burst", 20, "Rate limiter maximum burst (LIMITER_BURST)")

	fs.DurationVar(&cfg.Booking.GracePeriod, "booking-grace-period", 15*time.Minute,
		"Grace period for refund after booking creation (BOOKING_GRACE_PERIOD)")
	fs.DurationVar(&cfg.Booking.PaymentExpiry, "booking-payment-expiry", 15*time.Minute,
		"Time before unpaid booking is auto-cancelled (BOOKING_PAYMENT_EXPIRY)")
	fs.DurationVar(&cfg.Booking.CancellationWindow, "booking-cancellation-window", 24*time.Hour,
		"Default cancellation window before game start (BOOKING_CANCELLATION_WINDOW)")
	fs.DurationVar(&cfg.Booking.SlotLockTTL, "booking-slot-lock-ttl", 15*time.Minute,
		"TTL for slot locks during payment flow (BOOKING_SLOT_LOCK_TTL)")
	fs.DurationVar(&cfg.Booking.LinkTokenBuffer, "booking-link-token-buffer", 24*time.Hour,
		"How long past a booking's end its access token stays valid (BOOKING_LINK_TOKEN_BUFFER)")
	fs.IntVar(&cfg.Limits.MaxComplexes, "limits-max-complexes", 4,
		"Maximum complexes per user account (LIMITS_MAX_COMPLEXES)")

	fs.StringVar(&cfg.Google.PlacesAPIKey, "google-places-api-key", "", "Google Places API key (GOOGLE_MAPS_API)")
	fs.StringVar(&cfg.Google.OAuthClientID, "google-oauth-client-id", "",
		"Google OAuth client id; enables Sign in with Google (GOOGLE_OAUTH_CLIENT_ID)")
	fs.BoolVar(&cfg.OpenAPIValidateRequests, "openapi-validate-requests", true,
		"Validate every request against the embedded OpenAPI document; ignored in production "+
			"(OPENAPI_VALIDATE_REQUESTS)")
	fs.BoolVar(&cfg.PProf, "pprof", false, "Enable pprof profiling endpoints (PPROF_ENABLED)")
	fs.IntVar(&cfg.RequestLogSample, "request-log-sample", 1,
		"Log one successful request in N (1 logs every request; failures and slow requests are never "+
			"sampled away) (REQUEST_LOG_SAMPLE)")
	fs.StringVar(&cfg.TrustedProxiesSpec, "trusted-proxies", "",
		`Peers allowed to set X-Forwarded-For: "false" (default), "true" for the private ranges, `+
			`or a comma-separated list of CIDR prefixes (TRUSTED_PROXIES)`)

	return fs
}

// applyEnv lets the environment override what the flags left behind. The
// order matches the struct, and every parsed value goes through the reader so
// that a value nobody could parse becomes an error instead of a default.
//
//nolint:funlen // the flat environment surface, one variable per line.
func (cfg *Config) applyEnv(env *reader) {
	env.intVal("PORT", &cfg.Port, positive)
	env.strVal("ENV", &cfg.Env)

	env.durVal("HTTP_READ_HEADER_TIMEOUT", &cfg.HTTP.ReadHeaderTimeout, nonNegativeDur)
	env.durVal("HTTP_READ_TIMEOUT", &cfg.HTTP.ReadTimeout, nonNegativeDur)
	env.durVal("HTTP_WRITE_TIMEOUT", &cfg.HTTP.WriteTimeout, nonNegativeDur)
	env.durVal("HTTP_IDLE_TIMEOUT", &cfg.HTTP.IdleTimeout, nonNegativeDur)

	env.strVal("DATABASE_URL", &cfg.DB.DSN)
	env.strVal("DB_MIGRATOR_URL", &cfg.DB.MigratorDSN)
	env.intVal("DB_MAX_OPEN_CONNS", &cfg.DB.MaxOpenConns, positive)
	env.intVal("DB_MAX_IDLE_CONNS", &cfg.DB.MaxIdleConns, nonNegative)
	env.durVal("DB_MAX_IDLE_TIME", &cfg.DB.MaxIdleTime, nonNegativeDur)
	env.durVal("DB_STATEMENT_TIMEOUT", &cfg.DB.StatementTimeout, nonNegativeDur)
	env.durVal("DB_SLOW_QUERY_THRESHOLD", &cfg.DB.SlowQueryThreshold, nonNegativeDur)
	env.boolVal("DB_AUTO_MIGRATE", &cfg.DB.AutoMigrate)

	env.strVal("JWT_SECRET", &cfg.JWT.Secret)

	env.strVal("MP_ACCESS_TOKEN", &cfg.MP.AccessToken)
	env.strVal("MP_WEBHOOK_SECRET", &cfg.MP.WebhookSecret)
	env.strVal("MP_APP_ID", &cfg.MP.AppID)
	env.strVal("MP_CLIENT_SECRET", &cfg.MP.ClientSecret)
	env.strVal("MP_CREDENTIAL_KEYS", &cfg.MP.CredentialKeys)

	env.strVal("WHATSAPP_TOKEN", &cfg.WhatsApp.Token)
	env.strVal("WHATSAPP_PHONE_NUMBER_ID", &cfg.WhatsApp.PhoneID)
	env.strVal("WHATSAPP_VERIFY_TOKEN", &cfg.WhatsApp.VerifyToken)
	env.strVal("WHATSAPP_APP_SECRET", &cfg.WhatsApp.AppSecret)

	env.strVal("BREVO_API_KEY", &cfg.Brevo.APIKey)
	env.strVal("BREVO_SENDER", &cfg.Brevo.Sender)
	env.strVal("SMTP_HOST", &cfg.SMTP.Host)
	env.intVal("SMTP_PORT", &cfg.SMTP.Port, positive)
	env.strVal("SMTP_USERNAME", &cfg.SMTP.Username)
	env.strVal("SMTP_PASSWORD", &cfg.SMTP.Password)

	env.strVal("TURNSTILE_SECRET_KEY", &cfg.Turnstile.SecretKey)
	env.strVal("LEADS_ABANDONED_WEBHOOK_URL", &cfg.Leads.AbandonedWebhookURL)
	env.strVal("LEADS_ABANDONED_WEBHOOK_TOKEN", &cfg.Leads.AbandonedWebhookToken)

	env.strVal("R2_ACCOUNT_ID", &cfg.R2.AccountID)
	env.strVal("R2_ACCESS_KEY", &cfg.R2.AccessKey)
	env.strVal("R2_SECRET_KEY", &cfg.R2.SecretKey)
	env.strVal("R2_BUCKET_NAME", &cfg.R2.BucketName)
	env.strVal("R2_PUBLIC_URL", &cfg.R2.PublicURL)

	env.strVal("FRONTEND_URL", &cfg.FrontendURL)
	env.strVal("BACKEND_URL", &cfg.BackendURL)
	env.strVal("COOKIE_DOMAIN", &cfg.CookieDomain)
	env.strVal("TRUSTED_PROXIES", &cfg.TrustedProxiesSpec)

	env.boolVal("LIMITER_ENABLED", &cfg.Limiter.Enabled)
	env.floatVal("LIMITER_RPS", &cfg.Limiter.RPS)
	env.intVal("LIMITER_BURST", &cfg.Limiter.Burst, nonNegative)
	env.intVal("REQUEST_LOG_SAMPLE", &cfg.RequestLogSample, nonNegative)

	env.durVal("BOOKING_GRACE_PERIOD", &cfg.Booking.GracePeriod, nonNegativeDur)
	env.durVal("BOOKING_PAYMENT_EXPIRY", &cfg.Booking.PaymentExpiry, nonNegativeDur)
	env.durVal("BOOKING_CANCELLATION_WINDOW", &cfg.Booking.CancellationWindow, nonNegativeDur)
	env.durVal("BOOKING_SLOT_LOCK_TTL", &cfg.Booking.SlotLockTTL, nonNegativeDur)
	env.durVal("BOOKING_LINK_TOKEN_BUFFER", &cfg.Booking.LinkTokenBuffer, nonNegativeDur)
	env.intVal("LIMITS_MAX_COMPLEXES", &cfg.Limits.MaxComplexes, positive)

	env.strVal("REDIS_URL", &cfg.Redis.URL)
	env.strVal("SENTRY_DSN", &cfg.Sentry.DSN)
	env.strVal("SENTRY_RELEASE", &cfg.Sentry.Release)
	env.strVal("GOOGLE_MAPS_API", &cfg.Google.PlacesAPIKey)
	env.strVal("GOOGLE_OAUTH_CLIENT_ID", &cfg.Google.OAuthClientID)
	env.boolVal("OPENAPI_VALIDATE_REQUESTS", &cfg.OpenAPIValidateRequests)
	env.boolVal("PPROF_ENABLED", &cfg.PProf)

	cfg.Features = env.features("FEATURE_FLAGS")
}

// reader reads one environment variable at a time and keeps the failures.
//
// An empty value is treated as unset throughout, which is what the flag
// overrides it replaces already did: a variable declared but left blank in a
// deployment's environment must not wipe out a flag's default.
type reader struct {
	lookup Lookup
	errs   []error
}

func (r *reader) value(key string) (string, bool) {
	if r.lookup == nil {
		return "", false
	}
	v, ok := r.lookup(key)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

func (r *reader) fail(key, raw string, want string, err error) {
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s %q is not %s: %w", key, raw, want, err))
		return
	}
	r.errs = append(r.errs, fmt.Errorf("%s %q is not %s", key, raw, want))
}

func (r *reader) str(key string, set func(string)) {
	if v, ok := r.value(key); ok {
		set(v)
	}
}

func (r *reader) strVal(key string, target *string) {
	r.str(key, func(v string) { *target = v })
}

func (r *reader) intVal(key string, target *int, check func(int) error) {
	v, ok := r.value(key)
	if !ok {
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		r.fail(key, v, "a whole number", err)
		return
	}
	if err := check(n); err != nil {
		r.fail(key, v, err.Error(), nil)
		return
	}
	*target = n
}

func (r *reader) floatVal(key string, target *float64) {
	v, ok := r.value(key)
	if !ok {
		return
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		r.fail(key, v, "a number", err)
		return
	}
	if f <= 0 {
		r.fail(key, v, "greater than zero", nil)
		return
	}
	*target = f
}

func (r *reader) boolVal(key string, target *bool) {
	v, ok := r.value(key)
	if !ok {
		return
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.fail(key, v, "a boolean (true/false/1/0)", err)
		return
	}
	*target = b
}

func (r *reader) durVal(key string, target *time.Duration, check func(time.Duration) error) {
	v, ok := r.value(key)
	if !ok {
		return
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key, v, `a duration (e.g. "500ms", "15m")`, err)
		return
	}
	if err := check(d); err != nil {
		r.fail(key, v, err.Error(), nil)
		return
	}
	*target = d
}

// features parses the comma-separated flag list. A bare name is on; an
// explicit `name=false` is off and still recorded, so a deployment can say in
// one place that it has considered a flag and left it off.
func (r *reader) features(key string) Features {
	v, ok := r.value(key)
	if !ok {
		return nil
	}
	flags := Features{}
	for _, entry := range strings.Split(v, ",") {
		name, value, hasValue := strings.Cut(entry, "=")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		on := true
		if hasValue {
			b, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				r.errs = append(r.errs, fmt.Errorf(
					"%s: feature %q has value %q, which is not a boolean (true/false/1/0): %w",
					key, name, value, err))
				continue
			}
			on = b
		}
		flags[name] = on
	}
	return flags
}

func positive(n int) error {
	if n <= 0 {
		return errors.New("greater than zero")
	}
	return nil
}

func nonNegative(n int) error {
	if n < 0 {
		return errors.New("zero or greater")
	}
	return nil
}

func nonNegativeDur(d time.Duration) error {
	if d < 0 {
		return errors.New("a duration of zero or greater")
	}
	return nil
}
