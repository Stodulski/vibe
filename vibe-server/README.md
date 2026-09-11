# Vibe API

Backend API for Vibe, a booking platform for sports complexes: court availability and scheduling, deposits collected through MercadoPago, and client notifications by email and WhatsApp. Used by complex owners and staff to manage courts, bookings and clients, and by the public booking pages clients use to reserve a court.

## Stack

- Go 1.26, `net/http` + [httprouter](https://github.com/julienschmidt/httprouter)
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

- Go 1.26 (see `go.mod`)
- Docker and Docker Compose, to run PostgreSQL and Redis locally
- PostgreSQL 18.6 and Redis 8.10.1 (provided by `docker-compose.yml`, no separate install needed)
- [goose](https://github.com/pressly/goose) for running migrations with the CLI: `go install github.com/pressly/goose/v3/cmd/goose@latest`
- [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html), only if you edit `db/queries/*.sql`
- [golangci-lint](https://golangci-lint.run/welcome/install/) v2.12+, only for `make lint` (CI pins v2.12)

## Install and run locally

```bash
git clone https://github.com/Stodulski/vibe.git
cd vibe/vibe-server

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

Then apply migrations and run the server:

```bash
make migrate-up
make run
```

The API listens on port `8080` (`http://localhost:8080`). Health check: `GET /api/v1/healthcheck`.

## API reference

The full HTTP API is documented as an OpenAPI 3.1 document, served by the API itself:

- `GET /api/v1/docs` — an interactive reference (Scalar), the easiest place to start.
- `GET /api/v1/openapi.json` — the same document as JSON.
- `GET /api/v1/openapi.yaml` — the same document as raw YAML.

All three are public and answer at the general rate-limit tier.

The YAML committed at `internal/openapi/openapi.yaml` is the single source of truth. It is not
generated from the handlers, so `cmd/api/openapi_sync_test.go` (`TestOpenAPISyncWithRouter`)
fails whenever a route is registered without a matching entry there, or the document names a
route that no longer exists, so the two cannot drift apart silently.

## Environment variables

`.env.example` is versioned in this repository and lists every variable with a comment. `.env` is gitignored and never committed. This file's contents are not reproduced here; the tables below list names, purpose, and required/default status as read from `cmd/api/main.go` and `cmd/api/boot_config.go`.

### Core

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `ENV` | Runtime environment: `development`, `staging`, or `production`. Production enforces stricter checks on `JWT_SECRET` and `BACKEND_URL`. | Optional | `development` |
| `REDIS_URL` | Redis connection URL. | **Required**: the notification queue (email/WhatsApp) has no fallback; boot refuses to start without a reachable Redis. | none |

### Database

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `DATABASE_URL` | PostgreSQL DSN. | **Required**: boot fails to open/ping the pool without it. | none |
| `DB_AUTO_MIGRATE` | Apply pending migrations at startup, before serving. Prefer Railway's pre-deploy command over this when running more than one replica. | Optional | `false` |
| `DB_MIGRATOR_URL` | DSN migrations run as, when different from `DATABASE_URL` (the schema-owner role). | Optional | falls back to `DATABASE_URL` |
| `DB_STATEMENT_TIMEOUT` | Server-side `statement_timeout` (Go duration, e.g. `15s`). | Optional | `15s` |

### Auth

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `JWT_SECRET` | Secret used to sign JWTs. | **Required**: boot refuses to start if empty; in production must be at least 32 bytes and not look like a placeholder. | none |
| `MP_CREDENTIAL_KEYS` | AES-256 keyring encrypting stored MercadoPago credentials, format `kid:base64key[,kid:base64key...]` (each key decodes to 32 bytes). | **Required unconditionally**, even if MercadoPago is unused. | none |
| `COOKIE_DOMAIN` | Domain scope for auth cookies (e.g. `.example.com`). | Optional | `""` (host-only cookie) |
| `FRONTEND_URL` | Frontend origin, used for CORS and links in emails. | Optional | `http://localhost:5173` |
| `BACKEND_URL` | Public backend URL, used for MercadoPago OAuth callbacks and webhooks. | **Required when `MP_ACCESS_TOKEN` is set**, must be absolute (https in production). | `""` |
| `TRUSTED_PROXIES` | Peers allowed to set `X-Forwarded-For`: `false`, `true` (private ranges), or a comma-separated CIDR list. | Optional | `false` |
| `TURNSTILE_SECRET_KEY` | Cloudflare Turnstile secret key. Setting it enables Turnstile verification on register, login and forgot-password; pairs with the client's `VITE_TURNSTILE_SITE_KEY`. | Optional | `""` |
| `GOOGLE_OAUTH_CLIENT_ID` | Google OAuth client id. Setting it enables "Sign in with Google" (`POST /api/v1/auth/google`); the same value goes to the client as `VITE_GOOGLE_CLIENT_ID`. Create a **Web application** OAuth client in the Google Cloud console with this app's origin as an authorized JavaScript origin — no redirect URI is needed, Google Identity Services posts the ID token directly. | Optional | `""` |

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

### Observability

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `SENTRY_DSN` | Sentry DSN. | Optional: enables error tracking when set. | `""` |
| `SENTRY_RELEASE` | Sentry release tag. | Optional | `vibe@1.0.0` |
| `PPROF_ENABLED` | Enable `pprof` profiling endpoints. | Optional | `false` |
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

## Folder structure

```
cmd/api/           HTTP handlers, routes, middleware, boot and config
internal/data/     Store interfaces and implementations, tenant scoping
internal/db/       sqlc-generated code: do not edit manually
internal/          Cross-cutting services: mailer, notifier, storage, whatsapp, mp, circuitbreaker...
db/migrations/     Goose migrations (PostgreSQL)
db/queries/        SQL consumed by sqlc, one file per entity
docs/              Reference docs (e.g. WhatsApp templates)
scripts/           Operational scripts (backup, e2e runner)
tests/load/        Artillery load test scenarios
```

## Useful commands

```bash
make test               # unit tests (go test -race)
make test/cover          # unit tests with coverage report
make lint                # golangci-lint
make audit                # go mod verify + govulncheck
make build               # build ./bin/api
make sqlc                 # regenerate internal/db after editing db/queries/*.sql
make migrate-up           # apply migrations with goose
make migrate-down         # roll back one migration (asks for confirmation)
make e2e-db-up            # start an isolated Postgres/Redis for integration tests
make test/integration     # integration tests against that database
make test/security        # security-focused integration tests
make e2e                  # run the client's Playwright suite against an isolated stack
psql $DATABASE_URL -f db/truncate_all.sql   # empty all tables, keep schema/indexes/migrations
go run ./cmd/mpcredkey seal|rekey ...        # convert MercadoPago OAuth credentials between plaintext and the v1 envelope (dev/E2E databases only)
```

`make e2e` looks for the client at `../vibe-client` by default (override with `CLIENT_DIR`), which is where it lives in this repository.

## Deploy

The server deploys to [Railway](https://railway.app) from `Dockerfile` (Go 1.26 multi-stage build). `railway.toml` sets:

- `preDeployCommand = ["/app/api -migrate-only"]`: applies migrations once per deploy, before any replica serves traffic, using the same binary that runs the server.
- `startCommand = "/app/api"`
- `healthcheckPath = "/api/v1/healthcheck"`

CI runs on GitHub Actions (`.github/workflows/server.yml` at the repository root) with these jobs on every push and pull request to `main` that touches `vibe-server/`: `lint` (golangci-lint), `test` (`make test`), `build` (`make build`), `audit` (`make audit`), and `integration` (`make e2e-db-up && make test/integration`). The client's Playwright suite runs from `.github/workflows/e2e.yml` via `make e2e`, triggered by changes to either `vibe-server/` or `vibe-client/`.

## Architecture notes

- **Request flow**: `HTTP → middleware chain → httprouter → handler (cmd/api/) → store (internal/data/) → sqlc queries (internal/db/) → PostgreSQL`.
- **Tenant scoping**: every tenant-scoped table has row-level security (`FORCE`d, so it also applies to the schema owner). Two database roles exist: `vibe_migrator` (owns the schema, runs DDL) and `vibe_app` (`SELECT`/`INSERT`/`UPDATE`/`DELETE` only, no superuser, no RLS bypass). The tenant reaches SQL through the request context (`internal/data/tenant.go`); a route that never sets a tenant scope reads nothing under RLS, by design.
- **Durable notifications**: emails and WhatsApp messages go through a Redis-backed task queue (`internal/notifier`), not synchronously in the request path. This is why Redis has no fallback for that subsystem even though rate limiting, the token blacklist, and the SSE hub degrade gracefully without it.
- **Circuit breakers**: external services (MercadoPago, WhatsApp, mailer) are wrapped with a closed/open/half-open circuit breaker that is a no-op on a nil receiver, so it is safe when a service is not configured.

## Conventions

Commits follow [Conventional Commits](https://www.conventionalcommits.org/).

## Troubleshooting

- **Port `8080` already in use**: another process (often a previous `make run`) is bound to it. Stop it, or run with `go run ./cmd/api -port 8081`.
- **The client's E2E suite is hitting the dev API instead of an isolated one**: run `make e2e` from this repository rather than `pnpm test:e2e` directly from the client. The Playwright suite truncates its target database on setup, so it must point at the isolated stack (`:8081`/`:5433`/`:6380`), never at the dev API on `:8080`.
- **WhatsApp notifications never send, with no error**: `WHATSAPP_TOKEN` and `WHATSAPP_PHONE_NUMBER_ID` are both empty, so the feature is disabled at boot (logged once as `whatsapp notifications disabled`). Set both to enable it.
- **The client shows the Turnstile challenge but register/login/forgot-password answer 422 `turnstile_token: unavailable`**: the server could not confirm the token with Cloudflare — `TURNSTILE_SECRET_KEY` is wrong for the client's `VITE_TURNSTILE_SITE_KEY`, or Cloudflare's siteverify endpoint is unreachable from this deployment. It is not the same as `turnstile_token: invalid`, which means Cloudflare reached a verdict and rejected the token itself.
- **Address autocomplete answers 502, and the log shows `places upstream ... PERMISSION_DENIED`**: `GOOGLE_MAPS_API` is set to a key whose Cloud project does not have **Places API (New)** enabled (the legacy Places API being enabled instead is not enough). Enable Places API (New) for that project in the Google Cloud console.

## License

Vibe is free software: you can redistribute it and/or modify it under the terms of the GNU Affero General Public License as published by the Free Software Foundation, version 3. See [LICENSE](LICENSE). If you run a modified version as a network service, the AGPL requires you to offer its source to the users of that service.

Copyright (C) 2026 Vibe.
