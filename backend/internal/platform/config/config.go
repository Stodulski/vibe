// Package config is where the API's configuration is defined and loaded, once,
// at boot: command-line flags first, environment variables over them.
//
// It is a platform package, so it depends on nothing in the domain. What it
// cannot parse it refuses to guess at: every value this package reads either
// ends up in the returned Config or ends up in the returned error, and no
// value ever silently falls back to its default because the operator typed it
// wrong. A knob that is quietly ignored is a knob that is off in production
// while its owner believes it is on.
package config

import (
	"strings"
	"time"
)

// Config is the whole of the API's configuration. It is loaded by Load and
// then read-only: nothing re-reads the environment after boot.
type Config struct {
	Port int
	Env  string
	HTTP HTTP
	DB   DB
	JWT  JWT
	MP   MP

	WhatsApp  WhatsApp
	Brevo     Brevo
	Turnstile Turnstile
	Leads     Leads
	SMTP      SMTP
	R2        R2

	FrontendURL string
	// PasswordHashCost is the bcrypt cost the auth module hashes at. Zero
	// means authstore.DefaultHashCost, the production value; the test harness
	// lowers it so a suite that registers hundreds of users does not spend a
	// quarter of a second on each one. It carries no flag and no env var:
	// nothing about a real deployment should ever set it.
	PasswordHashCost int
	BackendURL       string
	CookieDomain     string
	// TrustedProxiesSpec is the unparsed -trusted-proxies/TRUSTED_PROXIES
	// value: "false" (default), "true" for the private ranges, or a
	// comma-separated list of CIDR prefixes. It is parsed by the composition
	// root rather than here, because the parsed type lives in internal/httpx
	// and a platform package does not reach into the domain.
	TrustedProxiesSpec string

	Limiter Limiter
	// RequestLogSample thins the request log: 0 or 1 logs every request, N
	// logs one successful request in N. Failures and slow requests are never
	// sampled away, and the metrics behind the detailed health endpoint are
	// never sampled at all.
	RequestLogSample int

	Booking Booking
	Limits  Limits
	Redis   Redis
	Sentry  Sentry
	Google  Google

	PProf bool
	// Features are the product flags read from FEATURE_FLAGS: code can ship
	// dark and be switched on per deployment without a new build.
	Features Features
	// MigrateOnly is -migrate-only: apply the embedded migration chain, print
	// the status and exit without serving. It is what a Railway pre-deploy
	// command runs, and it has no env var on purpose — it is an invocation
	// mode, not a setting of the deployment.
	MigrateOnly bool
}

// HTTP is the server's timeout surface: the four bounds http.Server places on
// one connection's lifetime.
//
// They are configuration rather than constants because the right values depend
// on what sits in front of the process. A platform whose proxy already caps a
// request at 30s wants a write timeout under that, not over it; a deployment
// behind a slow uplink wants a longer read.
//
// Zero means "no limit" for every one of them, which is http.Server's own
// meaning. It is accepted, and it is never a default: an unbounded server is a
// choice an operator has to make on purpose.
type HTTP struct {
	// ReadHeaderTimeout bounds the request line and headers alone. Without it
	// a peer that opens a connection and dribbles one header byte per minute
	// holds a goroutine and a file descriptor for as long as it likes —
	// Slowloris — because ReadTimeout is only armed once the handler starts
	// reading the body.
	ReadHeaderTimeout time.Duration
	// ReadTimeout bounds reading the whole request, headers and body.
	ReadTimeout time.Duration
	// WriteTimeout bounds writing the response, measured from the end of the
	// request headers. It is the ceiling the spreadsheet export's own budget
	// has to sit under (see reporting.ExportBudget and validateBootConfig):
	// the timeout closes the connection but does not cancel the handler, so
	// work that outlives it is work nobody will ever read.
	WriteTimeout time.Duration
	// IdleTimeout bounds how long a keep-alive connection may sit unused
	// between requests.
	IdleTimeout time.Duration
}

// DB is the Postgres connection and pool configuration.
type DB struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  time.Duration
	// StatementTimeout is the server-side backstop. A Go context cancels a
	// query by sending PostgreSQL a cancel request over a second connection;
	// when the pool is exhausted or the network is the thing that is broken,
	// that request is exactly what cannot get through, and the query keeps a
	// pool connection for as long as it likes. With 25 connections, a handful
	// of those is the whole instance.
	StatementTimeout time.Duration
	// SlowQueryThreshold is the duration past which a single query earns a
	// warn line of its own. Without it a slow query is only visible
	// indirectly, as a request that crossed the HTTP logger's own threshold.
	SlowQueryThreshold time.Duration
	// AutoMigrate runs the embedded migration chain before the server starts
	// listening. Off by default: a process that changes the schema as a side
	// effect of booting is a surprise unless somebody asked for it. Railway
	// asks for it the other way, through -migrate-only as a pre-deploy
	// command, which is the same chain run once instead of once per replica.
	AutoMigrate bool
	// MigratorDSN is the connection migrations run as. Empty means "the same
	// one the server uses".
	MigratorDSN string
}

// JWT is the access-token signing configuration.
type JWT struct {
	Secret string
}

// MP is the MercadoPago platform account's configuration.
type MP struct {
	AccessToken    string
	WebhookSecret  string
	AppID          string
	ClientSecret   string
	CredentialKeys string
}

// WhatsApp is the Meta Cloud API configuration.
type WhatsApp struct {
	Token       string
	PhoneID     string
	VerifyToken string
	AppSecret   string
}

// Brevo is the transactional email provider's configuration.
type Brevo struct {
	APIKey string
	Sender string
}

// Turnstile is Cloudflare's bot check.
type Turnstile struct {
	// SecretKey enables verification on register, login and forgot-password
	// when non-empty. Optional: a self-hoster is not forced to use Turnstile.
	SecretKey string
}

// Leads is the abandoned-registration capture forwarding.
type Leads struct {
	// AbandonedWebhookURL is a Google Apps Script Web App /exec URL (the same
	// spreadsheet the landing page's mailing-list signup writes to). Blank
	// disables forwarding — the endpoint still accepts and validates
	// requests, it just drops them.
	AbandonedWebhookURL   string
	AbandonedWebhookToken string
}

// SMTP is the fallback mail transport, used when no Brevo API key is set.
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
}

// R2 is Cloudflare's object storage.
type R2 struct {
	AccountID  string
	AccessKey  string
	SecretKey  string
	BucketName string
	PublicURL  string
}

// Limiter is the HTTP rate limiter.
type Limiter struct {
	Enabled bool
	RPS     float64
	// Burst is the bucket size a ceiling is allowed to fill to. Zero is a
	// valid, if severe, setting: it rejects every request under that ceiling
	// instead of relaxing the limit.
	Burst int
}

// Booking holds the booking domain's time windows.
type Booking struct {
	GracePeriod time.Duration
	// PaymentExpiry of zero (or negative) is not "no hold at all": stores.Config
	// falls back to a 15-minute default (see internal/stores.defaultPaymentExpiry)
	// so a zero value never leaves an unpaid booking without a slot hold.
	PaymentExpiry      time.Duration
	CancellationWindow time.Duration
	// SlotLockTTL of zero is accepted here; a TTL shorter than PaymentExpiry
	// is rejected separately, as a boot invariant (see validateBootConfig),
	// not as a parsing error.
	SlotLockTTL time.Duration
	// LinkTokenBuffer is added to a booking's end time to compute a booking
	// link token's expires_at (specs/booking-link-credential).
	LinkTokenBuffer time.Duration
}

// Limits are the per-account product ceilings.
type Limits struct {
	MaxComplexes int
}

// Redis is the cache, queue and shared-state backend.
type Redis struct {
	URL string
}

// Sentry is the error reporter.
type Sentry struct {
	DSN string
	// Release is SENTRY_RELEASE. Empty means the build's own version is used
	// instead, so an unset variable still groups errors by build.
	Release string
}

// Google covers both Google integrations: Places and Sign in with Google.
type Google struct {
	PlacesAPIKey string
	// OAuthClientID enables Sign in with Google when non-empty: every
	// endpoint stays registered either way (POST /api/v1/auth/google and
	// .../google/complete answer 503 while it is empty), so the route table,
	// the OpenAPI document and the CSRF/rate-limit/tenant exemption tables
	// never depend on it.
	OAuthClientID string
}

// Features is the product flag set parsed from FEATURE_FLAGS.
//
// The point of it is deploying code without activating it: a flag named here
// is on, everything else is off, and neither state needs a new build. It is
// deliberately a flat set of names rather than typed settings — a flag that
// needs a value is a configuration knob and belongs in Config itself.
type Features map[string]bool

// Enabled reports whether the named feature is on. The zero Features (and a
// nil one) says no to everything, which is what an unset FEATURE_FLAGS means.
func (f Features) Enabled(name string) bool {
	return f[strings.TrimSpace(name)]
}

// Names returns the enabled feature names, for the one line the boot log
// writes about them.
func (f Features) Names() []string {
	names := make([]string, 0, len(f))
	for name, on := range f {
		if on {
			names = append(names, name)
		}
	}
	return names
}
