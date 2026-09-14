# Vibe API

Backend API for Vibe, a booking platform for sports complexes: court availability and scheduling, deposits collected through MercadoPago, and client notifications by email and WhatsApp. Used by complex owners and staff to manage courts, bookings and clients, and by the public booking pages clients use to reserve a court.

## Stack

- Go 1.27, `net/http`'s own `ServeMux` + [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) generating the server interface and models from the OpenAPI document (see [ADR 0001](docs/adr/0001-httprouter-and-the-move-to-net-http.md))
- PostgreSQL via [pgx/v5](https://github.com/jackc/pgx) and [sqlc](https://sqlc.dev)
- Redis (required at runtime, see below)
- [MercadoPago](https://www.mercadopago.com) for payments
- [WhatsApp Cloud API](https://developers.facebook.com/docs/whatsapp/cloud-api) for optional client notifications
- [Brevo](https://www.brevo.com) transactional email API, with SMTP as a fallback
- [Cloudflare R2](https://developers.cloudflare.com/r2/) for optional object storage (images)
- [Sentry](https://sentry.io) for optional error tracking
- [Google Places API](https://developers.google.com/maps/documentation/places/web-service) for optional address autocomplete
- Deployed on [Railway](https://railway.app)

## Prerequisites

- Go 1.27 (see `go.mod`)
- Docker and Docker Compose, to run PostgreSQL and Redis locally
- PostgreSQL 18.6 and Redis 8.10.1 (provided by `docker-compose.yml`, no separate install needed)
- [goose](https://github.com/pressly/goose) for running migrations with the CLI: `go install github.com/pressly/goose/v3/cmd/goose@latest`
- [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html) v1.31+, only if you edit `db/queries/*.sql` (`make sqlc`) or want to run `make vet/sqlc` locally
- [golangci-lint](https://golangci-lint.run/welcome/install/) v2.13+, only for `make lint` (CI pins v2.13)

## Install and run locally

```bash
git clone https://github.com/Stodulski/vibe.git
cd vibe/backend

# Start PostgreSQL (:5432) and Redis (:6379)
docker compose up -d

# Copy the versioned example and fill in the values from the table below.
# At minimum: DATABASE_URL, REDIS_URL, JWT_SECRET, MP_CREDENTIAL_KEYS.
cp .env.example .env
```

With the containers from `docker-compose.yml`, `DATABASE_URL` is:

```
postgres://vibe:vibe_dev@localhost:5432/vibe?sslmode=disable
```

and `REDIS_URL` is `redis://localhost:6379`.

Generate the two secrets `.env` needs and that have no default:

```bash
openssl rand -base64 32   # JWT_SECRET (at least 32 random bytes)
echo "k1:$(openssl rand -base64 32)"   # MP_CREDENTIAL_KEYS
```

`MP_CREDENTIAL_KEYS` is required unconditionally, even if MercadoPago is unused — it is the
keyring that encrypts stored MercadoPago credentials at rest. `cmd/mpcredkey` does not generate
this keyring (the command above does); it consumes one, to convert credentials already in the
database between plaintext and the encrypted envelope when the keyring changes — see
`go run ./cmd/mpcredkey` (dev/E2E databases only) and its package doc comment for `seal`/`rekey`.

Then apply migrations and run the server:

```bash
make migrate-up
make run
```

The API listens on port `8080` (`http://localhost:8080`). Health check: `GET /api/v1/healthcheck`.

## API reference

This API is spec-first: `internal/openapi/openapi.yaml` is the single source of truth for the
whole HTTP surface, not something scraped off the handlers. The full document is also served by
the API itself:

- `GET /api/v1/docs` — an interactive reference (Scalar), the easiest place to start.
- `GET /api/v1/openapi.json` — the same document as JSON.
- `GET /api/v1/openapi.yaml` — the same document as raw YAML.

All three are public and answer at the general rate-limit tier.

`cmd/api/openapi_sync_test.go` (`TestOpenAPISyncWithRouter`) fails whenever a route is registered
without a matching entry in the document, or the document names a route that no longer exists, so
the two cannot drift apart silently.

### Generated code

`make generate/api` runs [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) — the
version pinned by the `tool` directive in `go.mod`, never one from `$PATH` — over the document,
regenerating `internal/openapi/gen` (request/response models, `ServerInterface`,
`HandlerWithOptions`). `cmd/api/apiserver.go` implements `ServerInterface` and registers it through
`HandlerWithOptions`, so every route, parameter and body type in the generated package comes
straight from the document: a route the document declares with no implementation fails to
compile, and a handler with no matching operation cannot be registered.

Run `make generate/api` after any edit to `internal/openapi/openapi.yaml` and commit the result —
`internal/openapi/gen` is checked in. CI's lint job re-runs `make generate/api` and fails the
build on a diff, so the generated package and the document cannot drift apart.

Handlers decode requests into, and encode responses through, the generated types, mapping from
store/service types explicitly (no store or sqlc struct is ever serialized directly). A few
request bodies and responses keep a small local type instead, where the generated model's stricter
decoding (a UUID or date that fails to unmarshal instead of failing this API's own field
validation) or encoding (a schema-nullable field oapi-codegen still emits as `omitempty`, dropping
the key instead of sending JSON `null`) would silently change the wire — each is documented in
place as a comment where the local type is defined.

### Errors

Every 4xx/5xx response is an [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem detail,
`Content-Type: application/problem+json`:

```json
{
  "type": "https://vibe.com.ar/problems/validation",
  "title": "Validation Failed",
  "status": 422,
  "detail": "the request failed validation",
  "instance": "/api/v1/auth/register",
  "request_id": "8f14e45f-ceea-467e-bd42-ce8c74d5f870",
  "errors": [
    {"field": "email", "message": "must be provided"},
    {"field": "password", "message": "must be provided"}
  ]
}
```

`type` is a stable URI per failure kind (`https://vibe.com.ar/problems/<kind>` — `bad-request`,
`invalid-json`, `unsupported-media-type`, `validation`, `not-found`, `route-not-found`, `conflict`,
`duplicate-booking`, `slot-unavailable`, `stale-version`, `unauthorized`, `forbidden`,
`rate-limited`, `too-large`, `unavailable`, `gone`, `internal`, `method-not-allowed`) and is the
field a client should switch on; `title`/`detail` are for humans and may change wording between
releases. `errors` is present only on a validation problem. `bad-request` is the generic 400;
`invalid-json` is reserved for a request body that failed to decode as JSON (`httpx.ReadJSON`).
`duplicate-booking` and `slot-unavailable` are 409s for a booking write that lost a race — a
duplicate booking or a court slot somebody else now holds. `stale-version` is a 409 for a write
whose If-Match/version the row no longer carries. All three are kept distinct from the generic
`conflict` so the frontend can switch on them without parsing `detail`.
`internal/httpx/problem.go` is the one place that builds this body — see its `Kind` constants for
the full, exact list.

Every JSON request body must carry `Content-Type: application/json` (a `; charset=utf-8`
parameter is accepted; nothing else is). `httpx.ReadJSON` checks this before it ever reads the
body and answers `415 Unsupported Media Type` (kind `unsupported-media-type`) otherwise. This
exists because a handful of auth routes mint or spend a session cookie and are therefore exempt
from CSRF protection (see `csrfExemptRoutes` in `internal/middleware/chain.go`); without this
check, a cross-site `<form enctype="text/plain">` submission could drive one of those routes with
attacker-chosen form-encoded bytes as the body, since the browser attaches `SameSite=Lax` cookies
to a top-level form navigation and stores whatever `Set-Cookie` the response answers with.

### Sign in with Google

There are two ways in, and they end in the same place. Which one the client uses is the client's
choice; the server supports both while `GOOGLE_OAUTH_CLIENT_ID` is set.

**Popup mode** is one request: the browser gets an ID token from Google and posts it to
`POST /api/v1/auth/google`, which answers with the session cookies, or with `needs_profile` and a
profile token for `POST /api/v1/auth/google/complete` — Google never supplies a phone number, and
registration needs one.

**Redirect mode** exists because popup mode opens a blank Google page on a good share of mobile
browsers (storage partitioning, a lost opener, in-app webviews). The client asks Google for
`ux_mode: 'redirect'` with `login_uri: https://app.vibe.com.ar/auth/google/callback`; Vercel
proxies that path — POST, body and cookies intact — to `POST /api/v1/auth/google/redirect`. Three
steps:

1. Google form-POSTs `application/x-www-form-urlencoded` with `credential` (the ID token) and
   `g_csrf_token`, having set a `g_csrf_token` cookie on the app's origin. Both must be present
   and equal; they are compared in constant time. This is the double submit from
   [Google's own guide](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token),
   and it works because a cross-site forgery can write the form but cannot read the cookie.
2. The server verifies the credential and answers `303 See Other` to
   `<FRONTEND_URL>/auth/google/return?code=<code>`. The code is opaque, 256 bits of entropy,
   single-use (consumed with Redis `GETDEL`) and valid for 120 seconds; it is held at
   `vibe:<env>:gauth:<code>` and carries the verified claims plus `sha256(g_csrf_token)` — never
   the ID token, and never a value that would be enough to spend it. **No session cookie is ever
   set by this endpoint** — the request is a top-level cross-site form navigation, and a POST an
   attacker can cause must not end in a session.
3. The frontend spends the code from its own origin against `POST /api/v1/auth/google/exchange`
   (`{"code": "...", "g_csrf_token": "..."}`), which answers exactly what
   `POST /api/v1/auth/google` answers: the session cookies, or `needs_profile` with a profile
   token. An unknown code, an expired or already-spent one, and one presented with the wrong
   `g_csrf_token` all answer `422` on field `code` with "invalid or expired" — never told apart.

**The code is bound to the browser it was issued to.** Google sets `g_csrf_token` as a readable
cookie on the app's origin, so the return page reads it and sends it back **in the exchange's JSON
body** — the API is a different origin and never receives that cookie — and the server compares its
hash in constant time. The redirect endpoint writes no `Set-Cookie` of any kind, so the cookie the
code is bound to is never cleared or rotated underneath the return page. Without that binding the code would be an unbound bearer:
anybody holding a valid Google ID token could mint one with `curl` — it supplies both halves of
Google's double submit itself — and send a victim the return URL, whose browser would spend it and
be signed in as the attacker. The code is consumed before the comparison, so a wrong pairing burns
it and cannot be retried. A browser that blocks the cookie cannot produce the value and fails
closed at the exchange, which is the right way round: it fails rather than signing somebody in
wrongly.

Every failure of the redirect endpoint is a `303` too, because its caller is a browser
mid-navigation and a problem document would be a dead-end page. The frontend's login page reads
two values out of `?error=`:

| `Location` | When |
|---|---|
| `<FRONTEND_URL>/login?error=google_rejected` | The CSRF cookie or field is missing or they disagree; the content type is not `application/x-www-form-urlencoded`; the body is over 16 KiB; the credential is missing, or Google refused it. |
| `<FRONTEND_URL>/login?error=google_unavailable` | `GOOGLE_OAUTH_CLIENT_ID` is not configured, Google's JWKS is unreachable, Redis would not hold the code, or any other internal failure. |

The reason is logged (`reason=csrf_mismatch`, `credential_rejected`, …) and never shown: the
redirect target is a URL a person can read and share.

The one answer that is not a redirect is `501`, when `FRONTEND_URL` is empty — there is then
nowhere to send the browser. Set `FRONTEND_URL` on any deployment that offers redirect mode.
`POST /api/v1/auth/google/exchange` is unaffected by it and still answers `503` while
`GOOGLE_OAUTH_CLIENT_ID` is unset, like the other Google routes.

Redis is not optional here on more than one instance: the code is minted on whichever instance
Google's post reached and spent on whichever one the frontend's exchange reaches. Without
`REDIS_URL` the store falls back to this process's memory, which is correct for a single instance
and for local development and wrong for anything else.

## Environment variables

`.env.example` is versioned in this repository and lists every variable with a comment. `.env` is gitignored and never committed. This file's contents are not reproduced here; the tables below list names, purpose, and required/default status as read from `cmd/api/main.go` and `cmd/api/boot_config.go`.

### Core

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `ENV` | Runtime environment: `development`, `staging`, or `production`. Production enforces stricter checks on `JWT_SECRET` and `BACKEND_URL`. | Optional | `development` |
| `PORT` | HTTP port the API listens on. | Optional | `8080` |
| `REDIS_URL` | Redis connection URL. On Railway it must be the private-network host (`*.railway.internal`) — see the note under `DATABASE_URL`. | **Required**: the notification queue (email/WhatsApp) has no fallback; boot refuses to start without a reachable Redis. | none |

### HTTP server

The four bounds `http.Server` places on one connection. `0` disables any of them; none of them defaults to `0`.

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `HTTP_READ_HEADER_TIMEOUT` | Deadline for reading the request line and headers alone (Go duration). This is the Slowloris bound: `HTTP_READ_TIMEOUT` does not cover a peer that dribbles its headers a byte at a time. | Optional | `5s` |
| `HTTP_READ_TIMEOUT` | Deadline for reading the whole request, headers and body (Go duration). | Optional | `5s` |
| `HTTP_WRITE_TIMEOUT` | Deadline for writing the response (Go duration). The spreadsheet export's own time budget is derived from it as three quarters of its value. | Optional | `60s` |
| `HTTP_IDLE_TIMEOUT` | How long a keep-alive connection may sit idle between requests (Go duration). | Optional | `60s` |

### Database

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `DATABASE_URL` | PostgreSQL DSN. On Railway both must resolve over the **project's private network** — a host ending in `.railway.internal`. A public host (`*.railway.app`, or anything else) sends every query, every session token and every queued notification across the internet, is billed as egress, and leaves the datastore reachable from outside the project. Boot warns once, to the log and to Sentry, when `ENV=production` and either host is not on the private network; it names the host and never the URL, because both carry a password. It is a warning and not a refusal because a self-hosted deployment has no `.railway.internal` to point at. | **Required**: boot fails to open/ping the pool without it. | none |
| `DB_AUTO_MIGRATE` | Apply pending migrations at startup, before serving. Prefer Railway's pre-deploy command over this when running more than one replica. | Optional | `false` |
| `DB_MIGRATOR_URL` | DSN migrations run as, when different from `DATABASE_URL` (the schema-owner role). | Optional | falls back to `DATABASE_URL` |
| `DB_MAX_OPEN_CONNS` | Maximum open PostgreSQL connections in the pool. | Optional | `25` |
| `DB_MAX_IDLE_CONNS` | Maximum idle PostgreSQL connections kept in the pool. | Optional | `10` |
| `DB_MAX_IDLE_TIME` | Maximum time a pooled connection may sit idle before it is closed (Go duration, e.g. `15m`). | Optional | `15m` |
| `DB_STATEMENT_TIMEOUT` | Server-side `statement_timeout` (Go duration, e.g. `15s`). | Optional | `15s` |
| `DB_IDLE_IN_TX_TIMEOUT` | Server-side `idle_in_transaction_session_timeout` (Go duration). The case `statement_timeout` cannot see: a transaction that is open but running nothing holds its row locks, its pool connection and the vacuum horizon for as long as the client stays silent. `0` leaves the server's own setting alone. | Optional | `30s` |
| `DB_SLOW_QUERY_THRESHOLD` | Log a warn line for any single query slower than this (Go duration); `0` disables it. | Optional | `500ms` |

### Auth

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `JWT_SECRET` | Secret used to sign JWTs: the **active** key. Every token this API mints names its key in the `kid` header, and a token that names no key is refused. | **Required**: boot refuses to start if empty; in production must be at least 32 bytes and not look like a placeholder. | none |
| `JWT_KEY_ID` | Name the active key answers to in the `kid` header. Leave it empty and the name is a short digest of the secret, which is already unique per secret — set it only if you would rather read `k2` than a digest. | Optional | derived from `JWT_SECRET` |
| `JWT_SECRET_PREVIOUS` | The key that was active before the last rotation. It **verifies and never signs**, so replacing `JWT_SECRET` does not end every live session. Rotating is two deploys: first move the current secret here and put the new one in `JWT_SECRET`, then remove this one once the longest-lived token minted under it has expired (30 days, the refresh-token window). Leaving it set indefinitely keeps a retired — possibly leaked — secret valid, which is what the rotation was for. | Optional | `""` (no rotation in flight) |
| `JWT_KEY_ID_PREVIOUS` | Name the retired key answers to. Set it to whatever `JWT_KEY_ID` held while that key was active; leave it empty whenever `JWT_KEY_ID` was empty. Get this wrong and the tokens naming the old key stop verifying, which is exactly the outage `JWT_SECRET_PREVIOUS` exists to prevent. | Optional | derived from `JWT_SECRET_PREVIOUS` |
| `MP_CREDENTIAL_KEYS` | AES-256 keyring encrypting stored MercadoPago credentials, format `kid:base64key[,kid:base64key...]` (each key decodes to 32 bytes). | **Required unconditionally**, even if MercadoPago is unused. | none |
| `COOKIE_DOMAIN` | Domain scope for auth cookies (e.g. `.example.com`). | Optional | `""` (host-only cookie) |
| `FRONTEND_URL` | Frontend origin, used for CORS, links in emails, and every redirect `POST /api/v1/auth/google/redirect` answers with. Set to empty and that endpoint answers `501` — it has nowhere to send the browser; see [Sign in with Google](#sign-in-with-google). | Optional | `http://localhost:5173` |
| `BACKEND_URL` | Public backend URL, used for MercadoPago OAuth callbacks and webhooks. | **Required when `MP_ACCESS_TOKEN` is set**, must be absolute (https in production). | `""` |
| `TRUSTED_PROXIES` | Peers allowed to set `X-Forwarded-For`: `false`, `true` (private ranges), or a comma-separated CIDR list. | Optional | `false` |
| `TURNSTILE_SECRET_KEY` | Cloudflare Turnstile secret key. Setting it enables Turnstile verification on register, login and forgot-password; pairs with the client's `VITE_TURNSTILE_SITE_KEY`. | Optional | `""` |
| `GOOGLE_OAUTH_CLIENT_ID` | Google OAuth client id. Setting it enables "Sign in with Google" in both modes (`POST /api/v1/auth/google` and `POST /api/v1/auth/google/redirect`); the same value goes to the client as `VITE_GOOGLE_CLIENT_ID`. Create a **Web application** OAuth client in the Google Cloud console with this app's origin as an authorized JavaScript origin. Popup mode needs no redirect URI — Google Identity Services posts the ID token directly — but redirect mode does: add the client's `login_uri` (`https://app.vibe.com.ar/auth/google/callback`) as an authorized redirect URI. See [Sign in with Google](#sign-in-with-google). | Optional | `""` |

### MercadoPago

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `MP_ACCESS_TOKEN` | MercadoPago access token. Setting it enables the payments integration. | Optional | `""` |
| `MP_APP_ID` | MercadoPago application ID, for OAuth. | Required when `MP_ACCESS_TOKEN` is set | `""` |
| `MP_CLIENT_SECRET` | MercadoPago OAuth client secret. | Required when `MP_ACCESS_TOKEN` is set | `""` |
| `MP_WEBHOOK_SECRET` | Verifies MercadoPago webhook signatures. | Required when `MP_ACCESS_TOKEN` is set | `""` |

### WhatsApp

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `WHATSAPP_TOKEN` | WhatsApp Cloud API token. | Optional: together with `WHATSAPP_PHONE_NUMBER_ID`, enables WhatsApp notifications. | `""` |
| `WHATSAPP_PHONE_NUMBER_ID` | WhatsApp Cloud API phone number ID. | Optional, see above | `""` |
| `WHATSAPP_VERIFY_TOKEN` | Token Meta uses to verify the webhook subscription. | Optional | `""` |
| `WHATSAPP_APP_SECRET` | Verifies webhook payload signatures (HMAC). | Optional | `""` |

Message templates and their exact parameter order are documented in [`docs/whatsapp-templates.md`](docs/whatsapp-templates.md).

### Mail

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `BREVO_API_KEY` | Brevo transactional email API key. | Optional: SMTP is used as a fallback when empty. | `""` |
| `BREVO_SENDER` | From address. | Optional | `Vibe <no-reply@vibe.com.ar>` |
| `SMTP_HOST` | SMTP host, used only when `BREVO_API_KEY` is empty. | Optional | `""` |
| `SMTP_PORT` | SMTP port, used only when `BREVO_API_KEY` is empty. | Optional | `587` |
| `SMTP_USERNAME` | SMTP username. | Optional | `""` |
| `SMTP_PASSWORD` | SMTP password. | Optional | `""` |

### Storage

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `R2_ACCOUNT_ID` | Cloudflare R2 account ID. | Optional: together with the two keys below, enables object storage for images. | `""` |
| `R2_ACCESS_KEY` | Cloudflare R2 access key. | Optional, see above | `""` |
| `R2_SECRET_KEY` | Cloudflare R2 secret key. | Optional, see above | `""` |
| `R2_BUCKET_NAME` | R2 bucket name. | Optional | `vibe` |
| `R2_PUBLIC_URL` | Public base URL for serving stored images. | Optional | `""` |
| `R2_PRIVATE_BUCKET_NAME` | A second bucket, served by no public domain, holding the payments export workbooks. Empty: the export endpoints answer 501. Needs a 24h lifecycle rule on `exports/` — see [`docs/runbook-exports.md`](docs/runbook-exports.md). | Optional | `""` |

### Limiter

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `LIMITER_ENABLED` | Enable the HTTP rate limiter. | Optional | `true` |
| `LIMITER_RPS` | Rate limiter requests per second allowed. | Optional | `10` |
| `LIMITER_BURST` | Rate limiter maximum burst size. | Optional | `20` |
| `LIMITER_USER_RPS` | Requests per second allowed **per authenticated account**, counted on top of the per-address limits. It is what bounds one account driven from many addresses, which no address bucket can see; keep it looser than `LIMITER_RPS` so it does not become the binding limit for an ordinary signed-in user behind a NAT. | Optional | `20` |
| `LIMITER_USER_BURST` | Maximum burst per authenticated account. | Optional | `40` |

### Observability

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `SENTRY_DSN` | Sentry DSN. | Optional: enables error tracking when set. | `""` |
| `SENTRY_RELEASE` | Sentry release tag. | Optional | `""` (falls back to `vibe@<build version>` stamped by the Dockerfile) |
| `PPROF_ENABLED` | Enable `pprof` profiling endpoints. | Optional | `false` |
| `OPENAPI_VALIDATE_REQUESTS` | Validate every incoming request against the embedded OpenAPI document and refuse what it forbids with 400. **Ignored in production**: the check exists to fail a mismatch in front of the person who can fix it, and production's version of it is the conformance suite in CI, which costs nothing at runtime. Set it to `false` to turn it off in development or staging. | Optional | `true` outside production |
| `REQUEST_LOG_SAMPLE` | Log one successful request in N (failures and slow requests are never sampled away). | Optional | `1` |

### Other

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `GOOGLE_MAPS_API` | Google Places API key, for address autocomplete. Must have **Places API (New)** enabled — the legacy Places API is marked legacy and cannot be enabled on new Cloud projects. | Optional | `""` |
| `LEADS_ABANDONED_WEBHOOK_URL` | Google Apps Script webhook URL for abandoned-registration email capture. | Optional | `""` |
| `LEADS_ABANDONED_WEBHOOK_TOKEN` | Shared token the abandoned-registration webhook expects. | Optional | `""` |
| `LIMITS_MAX_COMPLEXES` | Maximum complexes per user account. | Optional | `4` |
| `BOOKING_GRACE_PERIOD` | Grace period for refund after booking creation. | Optional | `15m` |
| `BOOKING_PAYMENT_EXPIRY` | Time before an unpaid booking is auto-cancelled. | Optional | `15m` |
| `BOOKING_CANCELLATION_WINDOW` | Default cancellation window before game start. | Optional | `24h` |
| `BOOKING_SLOT_LOCK_TTL` | TTL for slot locks during payment. Must be `>= BOOKING_PAYMENT_EXPIRY`, checked at boot. | Optional | `15m` |
| `BOOKING_LINK_TOKEN_BUFFER` | Extra time past a booking's end during which its access link stays valid. | Optional | `24h` |
| `FEATURE_FLAGS` | Comma-separated product feature flags: a bare name turns it on, `name=false` turns it off explicitly. | Optional | `""` (all flags off) |

An environment variable that cannot be parsed — a number, a duration, a boolean — fails the boot, and every such error is reported together instead of one per restart.

## Folder structure

```
cmd/api/           HTTP handlers, routes, middleware, boot and config
internal/data/     Store interfaces and implementations, tenant scoping
internal/db/       sqlc-generated code: do not edit manually
internal/          Cross-cutting services: mailer, jobs, storage, whatsapp, mp, circuitbreaker...
db/migrations/     Goose migrations (PostgreSQL)
db/queries/        SQL consumed by sqlc, one file per entity
docs/              Reference docs (WhatsApp templates, backup, data-deletion and export runbooks, ADRs in docs/adr/)
scripts/           Operational scripts (backup, e2e runner)
tests/load/        Artillery load test scenarios
```

## Useful commands

```bash
make test               # unit tests (go test -race -shuffle=on)
make test/cover          # unit tests with coverage report (same flags, plus -coverprofile)
make lint                # golangci-lint
make audit                # go mod verify + govulncheck
make build               # build ./bin/api
make sqlc                 # regenerate internal/db after editing db/queries/*.sql
make vet/sqlc              # validate queries against a live, migrated schema (needs DATABASE_URL)
make migrate-up           # apply migrations with goose
make migrate-down         # roll back one migration (asks for confirmation)
make migrate-create name=<name>   # scaffold a new empty migration file
make e2e-db-up            # start an isolated Postgres/Redis for integration tests
make test/integration     # integration tests against that database
make test/security        # security-focused integration tests
make e2e                  # run the client's Playwright suite against an isolated stack
psql $DATABASE_URL -f db/truncate_all.sql   # empty all tables, keep schema/indexes/migrations
go run ./cmd/mpcredkey seal|rekey ...        # convert MercadoPago OAuth credentials between plaintext and the v1 envelope (dev/E2E databases only)
```

`make e2e` looks for the client at `../frontend` by default (override with `CLIENT_DIR`), which is where it lives in this repository.

## Testing

- **Flags**: `test`, `test/cover` and `test/integration` all run with `-race` (data-race detector) and `-shuffle=on` (each run reorders tests and subtests with a fresh seed, so a test that only passes for a particular execution order fails instead of hiding). `test/integration` also forces `-p 1` because every package shares the one E2E database. On a failure, `go test` prints the seed it used (`-shuffle=on -shuffle-seed=<n>`); reruns with that seed reproduce the same order for debugging an order-dependent failure.
- **Coverage**: `make test/cover` runs the same suite as `make test` (same flags) plus `-coverprofile=coverage.out -covermode=atomic`, then prints the per-function breakdown and a `total: (statements) NN.N%` line. CI runs this in the `test` job, appends that last line to the job's step summary, and uploads `coverage.out` as a build artifact (retained 14 days) — download it and run `go tool cover -html=coverage.out` locally to see line-by-line coverage in a browser. There is no minimum-coverage gate; this is visibility, not enforcement.
- **Integration Redis**: `test/integration` also exports `REDIS_URL` (`E2E_REDIS_URL` in the Makefile) pointing at `docker-compose.e2e.yml`'s `redis` service on `localhost:6380`, for any integration test that needs a real Redis instead of the nil client `newIntegrationApp` otherwise builds.

## Deploy

The server deploys to [Railway](https://railway.app) from `Dockerfile` (Go 1.27 multi-stage build, distroless final image — see the Dockerfile's own comments for why). `railway.toml` sets:

- `preDeployCommand = ["/app/api -migrate-only"]`: applies migrations once per deploy, before any replica serves traffic, using the same binary that runs the server.
- `startCommand = "/app/api"`
- `healthcheckPath = "/api/v1/healthcheck"`: **readiness** (pings Postgres and Redis). `/api/v1/livez` is liveness (answers 200 unconditionally) — not wired into `railway.toml` because Railway's own health check only supports one path per service, but there for a future separate liveness prober.

CI runs on GitHub Actions (`.github/workflows/backend.yml` at the repository root) on every push to `main` and every pull request that touches `backend/`, regardless of the PR's base branch (stacked PRs get CI too). Jobs:

| Job | What it runs |
|---|---|
| `lint` | golangci-lint (config in `.golangci.yml`): errcheck, gosec, errorlint, bodyclose, sqlclosecheck, rowserrcheck, noctx, contextcheck, nilerr, wastedassign, unconvert, unparam, copyloopvar, durationcheck, exhaustive, gocritic, funlen, revive and wrapcheck on top of golangci-lint's standard set |
| `format` | `gofmt -l .`, `goimports -l .`, `go vet ./...` |
| `test` | `make test/cover` (unit tests, race detector + `-shuffle=on`), then prints the total coverage line to the job summary and uploads `coverage.out` as a 14-day artifact |
| `build` | `make build` |
| `audit` | `make audit` (`go mod verify` + `govulncheck`) |
| `sqlc` | `make vet/sqlc` against a disposable, migrated Postgres — catches a query that no longer matches the schema |
| `container` | builds `Dockerfile` and scans the image with [Trivy](https://github.com/aquasecurity/trivy), failing on HIGH/CRITICAL findings with a known fix |
| `integration` | `make e2e-db-up && make test/integration` (also `-race -shuffle=on`) |

**wrapcheck**: an error returned from an external package (pgx, `database/sql`, stdlib, a third-party client) must cross into our code as `fmt.Errorf("<package>: <operation>: %w", err)`, so `errors.Is`/`errors.As` on sentinels still work while every store or service function names what it was doing when a dependency failed. Our own sentinels and typed errors are exempt from re-wrapping — `linters.settings.wrapcheck.ignore-package-globs` covers every `internal/*` package (`data.AssertTenant`, `DB.WithTx`/`RetryTx`, `Filters.ParseCursor`, store sentinels, internal interfaces) — and `extra-ignore-sigs` adds `(context.Context).Err(` and `(net/http.ResponseWriter).Write(` to wrapcheck's own default ignore list, since both are either an unwrappable sentinel pair or a write nothing downstream can act on. Tests, `internal/db` (sqlc-generated), `internal/openapi/gen` (oapi-codegen-generated), `internal/data/datatest` (test fixtures), and the two standalone CLIs `cmd/mpcredkey` and `cmd/benchmark` are excluded via `linters.exclusions.rules`.

**Required status checks**: all eight jobs above should be marked required for merging into `main` (GitHub → repository Settings → Branches → branch protection rule for `main`). Not configured by this PR — it is a repository setting, done by the owner outside the codebase.

**"Wait for CI"**: Railway's GitHub integration deploys on push to `main` by reading `railway.toml`, independent of whether GitHub Actions passed. Railway has a **"Wait for CI"** toggle (project → service → Settings → Source) that makes it hold the deploy until GitHub's checks for that commit are green. Not enabled by this PR — it is a Railway dashboard setting the owner flips.

The client's Playwright suite runs from `.github/workflows/e2e.yml` via `make e2e`, triggered by changes to either `backend/` or `frontend/`.

**Backups**: not a Railway feature for this plan — see [`docs/runbook-backups.md`](docs/runbook-backups.md) for the scheduled `pg_dump`-to-R2 workflow, retention, and the restore procedure.

**Dependency updates**: Dependabot (`.github/dependabot.yml`) opens weekly, grouped (minor/patch) PRs for `backend`'s Go modules, its Docker base images, and every workflow's GitHub Actions.

**Release tagging**: annotated, semantic-version tags per release — `git tag -a vX.Y.Z -m "..."`. The repository has no tags yet; `v1.0.0` should be the first, created by the owner (not by an automated PR) once cut.

## Operations

- **Backups and restore**: [`docs/runbook-backups.md`](docs/runbook-backups.md) — what runs, where backups land, retention, and the step-by-step restore procedure.
- **Somebody asks to be removed**: [`docs/runbook-data-deletion.md`](docs/runbook-data-deletion.md) — how to verify the request, what is anonymized versus kept and why, and `go run ./cmd/anonymize`. The venue owner can delete their own account (`DELETE /api/v1/auth/me`, which cascades); the final client who booked without one cannot, and this is their path.
- **Log retention**: this service does not manage its own log storage — stdout/stderr go to whatever Railway's plan retains and shows under the service's **Observability**/**Logs** tab. Check the current plan's retention window there (or in Railway's pricing page) rather than assuming a number; it can change with the plan. Sampling on top of that (independent of Railway's retention) is `REQUEST_LOG_SAMPLE`, implemented in `internal/middleware/logging.go` — it logs one successful request in N, never sampling away failures or slow requests.
- **R2 bucket policy** (`R2_PUBLIC_URL`, client-uploaded images): the bucket is public **by object key only** — anyone with a specific object's URL can read it, but the bucket does not expose listing, so an object's key has to already be known (it is not guessable: see `internal/storage`). Writes never go through the backend directly; the client uploads via a **presigned PUT** the backend issues, scoped to one object key. There is no presigned GET for images: reads are the plain public URL. This is a deliberate tradeoff (simplicity over per-read expiry), not an oversight — revisit if these images should ever need to stop being permanently public once linked. The payments exports do **not** live here, for exactly that reason: they are a month of a club's ledger and go to `R2_PRIVATE_BUCKET_NAME`, read only through a 15 minute presigned GET.
- **Payments exports**: [`docs/runbook-exports.md`](docs/runbook-exports.md) — the private bucket (`R2_PRIVATE_BUCKET_NAME`), the **24 hour lifecycle rule on `exports/` the operator has to create by hand**, and how to read the export queue. The application never deletes a workbook; it only refuses to sign a URL for one older than 24 hours, so without the rule the bucket keeps every club's ledger forever.
- **Timezone**: the process runs at `TZ=UTC`; the product's own wall-clock is a single hardcoded `America/Argentina/Buenos_Aires`. See [ADR 0005](docs/adr/0005-single-timezone.md).

## Architecture decisions

Short, one-page ADRs for decisions that aren't obvious from reading the code: [`docs/adr/`](docs/adr/).

- [0001 — httprouter today, planned move to `net/http` `ServeMux` + oapi-codegen](docs/adr/0001-httprouter-and-the-move-to-net-http.md)
- [0002 — sqlc + pgx/v5](docs/adr/0002-sqlc-and-pgx.md)
- [0003 — goose migrations, forward-only, direct edits while there are no production users](docs/adr/0003-goose-migrations.md)
- [0004 — Redis-backed notification queue today, planned unification onto a Postgres `jobs` table](docs/adr/0004-notification-queue-and-the-move-to-a-jobs-table.md)
- [0005 — single timezone by design](docs/adr/0005-single-timezone.md)

## Architecture notes

- **Request flow**: `HTTP → middleware chain → httprouter → handler (cmd/api/) → store (internal/data/) → sqlc queries (internal/db/) → PostgreSQL`.
- **Tenant scoping**: every tenant-scoped table has row-level security (`FORCE`d, so it also applies to the schema owner). Two database roles exist: `vibe_migrator` (owns the schema, runs DDL) and `vibe_app` (`SELECT`/`INSERT`/`UPDATE`/`DELETE` only, no superuser, no RLS bypass). The tenant reaches SQL through the request context (`internal/data/tenant.go`); a route that never sets a tenant scope reads nothing under RLS, by design.
- **Durable notifications**: emails and WhatsApp messages go through the `jobs` table (`internal/jobs`), not synchronously in the request path. Workers claim rows with `SELECT ... FOR UPDATE SKIP LOCKED`, so every instance drains the same queue without contending. Delivery is at-least-once, and every enqueue carries a deduplication key so a redelivered webhook cannot send the same confirmation twice. Redis is still required at boot, but for the subsystems whose fallbacks are per-instance and therefore wrong on more than one instance: the token blacklist, the user cache, distributed rate limiting, slot locking and the SSE relay.
- **Circuit breakers**: external services (MercadoPago, WhatsApp, mailer) are wrapped with a closed/open/half-open circuit breaker that is a no-op on a nil receiver, so it is safe when a service is not configured.

## Conventions

Commits follow [Conventional Commits](https://www.conventionalcommits.org/).

## Troubleshooting

- **Port `8080` already in use**: another process (often a previous `make run`) is bound to it. Stop it, or run with `go run ./cmd/api -port 8081`.
- **The client's E2E suite is hitting the dev API instead of an isolated one**: run `make e2e` from this repository rather than `pnpm test:e2e` directly from the client. The Playwright suite truncates its target database on setup, so it must point at the isolated stack (`:8081`/`:5433`/`:6380`), never at the dev API on `:8080`.
- **WhatsApp notifications never send, with no error**: `WHATSAPP_TOKEN` and `WHATSAPP_PHONE_NUMBER_ID` are both empty, so the feature is disabled at boot (logged once as `whatsapp notifications disabled`). Set both to enable it.
- **The client shows the Turnstile challenge but register/login/forgot-password answer 422 `turnstile_token: unavailable`**: the server could not confirm the token with Cloudflare — `TURNSTILE_SECRET_KEY` is wrong for the client's `VITE_TURNSTILE_SITE_KEY`, or Cloudflare's siteverify endpoint is unreachable from this deployment. It is not the same as `turnstile_token: invalid`, which means Cloudflare reached a verdict and rejected the token itself.
- **Google sign-in in redirect mode always lands on `/login?error=google_rejected`**: the `g_csrf_token` cookie Google sets on the app's origin is not reaching the API. The frontend's proxy has to forward the POST body **and** the request cookies to `POST /api/v1/auth/google/redirect`; a proxy that drops either makes the double-submit check fail on every attempt. The server log names which half was missing (`reason=csrf_cookie_missing`, `csrf_field_missing` or `csrf_mismatch`).
- **Address autocomplete answers 502, and the log shows `places upstream ... PERMISSION_DENIED`**: `GOOGLE_MAPS_API` is set to a key whose Cloud project does not have **Places API (New)** enabled (the legacy Places API being enabled instead is not enough). Enable Places API (New) for that project in the Google Cloud console.

## License

Vibe is free software: you can redistribute it and/or modify it under the terms of the GNU Affero General Public License as published by the Free Software Foundation, version 3. See [LICENSE](LICENSE). If you run a modified version as a network service, the AGPL requires you to offer its source to the users of that service.

Copyright (C) 2026 Vibe.
