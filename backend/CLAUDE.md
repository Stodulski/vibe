# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make run                # Run API server (go run ./cmd/api)
make build              # Build binary (CGO_ENABLED=0 go build -o ./bin/api ./cmd/api)
make test               # Run unit tests (go test -race -v -count=1 ./...)
make lint               # Run golangci-lint
make tidy               # go mod tidy + go fix
make audit              # go mod verify + govulncheck

# Run a single test
go test -race -v -run TestHandlerName ./cmd/api/

# Database
make migrate-up         # Apply goose migrations
make migrate-down       # Rollback (requires confirmation)
make migrate-embedded   # Apply migrations with the API binary (the deploy path)
make sqlc               # Regenerate sqlc code after editing db/queries/*.sql

# Integration tests (require running Postgres)
make e2e-db-up          # Start E2E database + apply migrations
make test/integration   # Run integration tests against real DB
make test/security      # Run security-specific integration tests
make e2e-db-down        # Tear down E2E database

# Coverage
make test/cover         # Run tests with coverage report
make test/cover/html    # Generate HTML coverage report
```

## Architecture

**Go 1.26 backend** — Multi-tenant SaaS for padel court management. Deployed on Railway.

### Request flow

```
HTTP → middleware chain → httprouter → handler (cmd/api/) → store (internal/data/) → sqlc queries (internal/db/) → PostgreSQL
```

Middleware chain order: `recoverPanic → requestID → securityHeaders → CORS → rateLimit → csrfProtect → authenticate → routes`

### Code layout

- **`cmd/api/`** — HTTP handlers, routes, middleware, helpers. Handlers are methods on `*application` which holds all injected dependencies. One file per domain (auth.go, bookings.go, courts.go, etc.).
- **`internal/data/`** — Store interfaces and implementations. Interfaces are segregated by responsibility (ISP): e.g., `BookingStore` composes `BookingCreator`, `BookingReader`, `BookingUpdater`, `BookingStatsQuerier`, `BookingReminderManager`, `BookingLifecycleManager`. The `Models` struct in `models.go` aggregates all stores and is injected into the app.
- **`internal/platform/`** — the infrastructure the composition root wires and the domain never names: `config/` (the whole flag and environment surface, loaded once by `config.Load`; nothing else in the tree reads the environment), `db/` (the pgx pool and its slow-query tracer) and `redis/` (the client). A platform package never imports a domain package.
- **`internal/db/`** — **sqlc-generated code — do not edit manually.** Edit SQL in `db/queries/*.sql`, then run `make sqlc`.
- **`db/migrations/`** — Goose migrations (PostgreSQL). `001_init.sql` is the whole schema and the only file: the pre-launch chain was squashed into it twice (001–042, then 002–005 on 2026-09-14), and its header says why. While there are no production users a new change may be folded into it with a proof of schema equivalence; from the first production deploy onward every change is a new numbered file and applied files are immutable. See `docs/adr/0003-goose-migrations.md`.
- **`db/queries/`** — SQL queries consumed by sqlc. One `.sql` file per entity.
- **`internal/`** subpackages — Cross-cutting services: `circuitbreaker/`, `mailer/` (Brevo API + SMTP fallback), `jobs/` (the durable Postgres work queue), `storage/` (Cloudflare R2), `validator/`, `whatsapp/`, `mp/` (MercadoPago).

### Key patterns

- **Dependency injection**: All services/stores injected via `*application` struct in `main.go`. Store interfaces allow mock implementations for testing.
- **Context passing**: User and Complex objects flow through `context.Context` (see `contextSetUser`, `contextGetAuthenticatedUser`). Request ID attached per request.
- **Auth chain**: `app.requireAuth(handler)` → `app.requireComplexOwner(handler)` → `app.requireRole("superadmin")`.
- **Cursor-based pagination**: `Filters` struct with cursor + limit. Generic `TrimPage` helper. Composite cursor (date+ID) for bookings.
- **Type conversions**: `internal/data/convert.go` has helpers between Go types and pgx types (uuid, date, time, text). Always use these helpers — never convert pgx types manually.
- **Circuit breaker**: External services (MercadoPago, WhatsApp, mailer) wrapped with circuit breaker (closed→open→half-open). Safe on nil receiver (no-op when not configured).
- **Dual rate limiting**: Redis-backed (production) with in-memory fallback. Different limits for general/auth/booking endpoints.
- **Conditional features**: Redis enables distributed rate limiting, token blacklist caching, SSE hub, slot locking. Services degrade gracefully without it.

### Testing

- **Unit tests** (`*_test.go` in `cmd/api/`): Use mock stores. `newTestApplication()` in `testutils_test.go` creates app with mocks.
- **Integration tests** (`*_integration_test.go`): Build tag `//go:build integration`. Require `DATABASE_URL` env var pointing to real Postgres. `setupTestDB()` in `testutils_integration_test.go`.
- Mock stores implement the segregated interfaces (e.g., `mockBookingStore`).
- Test helpers: `newTestServer()`, `testGet()`, `testPost()`, `testPostWithCookies()`, `generateTestToken()`.

### Database

- **sqlc** generates type-safe Go from SQL queries. Config in `sqlc.yaml`. Uses pgx/v5 driver.
- **Migrations ship inside the binary.** `db/migrations.go` embeds `db/migrations/*.sql`; `internal/migrate` applies them with goose as a library, at the same version as the CLI and into the same `goose_db_version` table, so a database migrated by either can be finished by the other. Nothing changes about writing a migration — add the file to `db/migrations/` as before.
  - `./api -migrate-only` — apply, print the status, exit 0. This is `railway.toml`'s `preDeployCommand`: once per deploy, before any container serves, and a non-zero exit stops the deploy. It runs before `JWT_SECRET` and the rest of the boot checks, so a pre-deploy step never fails on a secret it does not use.
  - `DB_AUTO_MIGRATE=true` — apply at startup, before the server listens; a failure aborts the boot with a non-zero exit. Off by default. Prefer the pre-deploy command wherever there is more than one replica: the migrator takes a session advisory lock so racing replicas are safe, but only one of them has anything to do.
  - `DB_MIGRATOR_URL` — optional DSN migrations run as; falls back to `DATABASE_URL`. It exists for the split that is coming: DDL belongs to the role that owns the schema, while the server wants a DML-only role so that a handler defect cannot drop a table and so that row-level security binds (PostgreSQL exempts a table's owner from its own policies unless the table is `FORCE`d).
  - `db/migrations.go` is not sqlc-generated and is not a migration; it is the embed carrier, and it has to live in `db/` because `go:embed` cannot name a path outside its own package directory.
  - Both are listed in `.env.example`, which is regenerated from the environment surface `internal/platform/config` reads.
- **Two database roles, and the application is neither a superuser nor the owner.** `001_init.sql`'s ACCESS section creates `vibe_migrator` (owns schema `public`, runs DDL) and `vibe_app` (SELECT/INSERT/UPDATE/DELETE only, `NOSUPERUSER`, `NOBYPASSRLS`), then puts `ENABLE`/`FORCE ROW LEVEL SECURITY` and a `tenant_isolation` policy on every tenant-scoped table. It comes last in the file on purpose: the ownership walk and the blanket grants cover every object created above them, so anything added after that section would be owned by the wrong role and ungranted.
  - Both roles are created `NOLOGIN` with no password — roles are cluster-level and a password in a migration is a password in git. The deploy runs `ALTER ROLE vibe_migrator LOGIN PASSWORD '...'` and the same for `vibe_app`, once per cluster, then points `DB_MIGRATOR_URL` at the first and `DATABASE_URL` at the second. `make e2e-db-roles` does it for the test database with throwaway passwords.
  - **The tenant reaches SQL through the request context.** `internal/data/tenant.go` is the whole mechanism and the place to read first: `data.ContextWithTenant` / `data.ContextWithTenantBypass` put the scope on the context, the pool's `PrepareConn` hook (`data.StampTenantScope`, wired in `cmd/api/main.go`) stamps it on every checkout, and `DB.Begin` repeats it as `SET LOCAL` inside every transaction.
  - `httpx.ContextSetComplex` — called only by `RequireComplexOwner` — is what scopes an owner-guarded route. Every other route is either declared in `middleware.CrossTenantRoutes` with a reason or has no scope at all, which under 042 means it reads nothing. **That is the intended failure**: a new public endpoint that reads a tenant table breaks in its first test instead of shipping a leak.
  - **A data migration (a backfill, a repair `UPDATE`) must open with `SET LOCAL app.bypass_tenant = 'on';`** or it will silently touch zero rows: row-level security is `FORCE`d and applies to the schema owner too.
  - Integration tests keep using `DATABASE_URL`. The row-level-security tests take `DATABASE_APP_URL` (the `vibe_app` DSN, and they refuse to run as a superuser) and `DATABASE_ADMIN_URL` (fixtures and DDL, falls back to `DATABASE_URL`).
- Schema uses UUIDs, CITEXT for emails, PostgreSQL enums, generated range columns with EXCLUDE constraints for overlap, row-level security keyed on `app.complex_id`, auto-updated `updated_at` triggers.
- Domain errors as sentinels in `internal/data/errors.go` (e.g., `ErrRecordNotFound`, `ErrSlotLocked`, `ErrDuplicateEmail`).

## Coding Rules

These rules are **mandatory**. Follow them when writing or modifying any code in this repository.

### Handler conventions (Let's Go Further)

- All handlers are methods on `*application`: `func (app *application) nameHandler(w http.ResponseWriter, r *http.Request)`
- All JSON responses use the `envelope` wrapper: `app.writeJSON(w, status, envelope{"key": data}, nil)`
- Request payloads use **inline anonymous structs** with `json` tags — never standalone input types:
  ```go
  var input struct {
      Email string `json:"email"`
      Name  string `json:"name"`
  }
  err := app.readJSON(w, r, &input)
  ```
- Error responses always go through the error response helpers: `serverErrorResponse`, `badRequestResponse`, `notFoundResponse`, `failedValidationResponse`, etc. Never write error JSON manually.
- Validation follows the fluent pattern — create `validator.New()`, chain `v.Check()` calls, then check `v.Valid()`:
  ```go
  v := validator.New()
  v.Check(input.Email != "", "email", "must be provided")
  v.Check(validator.Matches(input.Email, validator.EmailRX), "email", "must be valid")
  if !v.Valid() {
      app.failedValidationResponse(w, r, v.Errors)
      return
  }
  ```
- Domain errors translate to HTTP responses via `errors.Is()` switch:
  ```go
  case errors.Is(err, data.ErrRecordNotFound):
      app.notFoundResponse(w, r)
  ```

### SOLID principles

- **Single Responsibility**: One handler file per domain. One store file per entity. Each internal/ package does one thing.
- **Open/Closed**: New functionality = new handlers/methods. Don't modify existing handlers to add unrelated behavior.
- **Liskov Substitution**: Mock stores must satisfy the same interfaces as real stores. All store implementations are interchangeable through interfaces.
- **Interface Segregation (ISP)**: Store interfaces are composed from small, focused interfaces. Example: `BookingStore` = `BookingCreator` + `BookingReader` + `BookingUpdater` + `BookingStatsQuerier` + `BookingReminderManager` + `BookingLifecycleManager`. When adding new store methods, add them to the correct sub-interface (or create a new one). Never bloat an existing interface with unrelated methods.
- **Dependency Inversion**: Handlers depend on store interfaces (defined in `internal/data/models.go`), never on concrete implementations. The `Models` struct wires everything together in `NewModels()`.

### DRY rules

- **Type conversions**: Always use helpers from `internal/data/convert.go` (`uuidToPg`, `pgToUUID`, `dateToPg`, `pgToDate`, `timeToPg`, `pgToTime`, `textToPg`, `pgToTextPtr`, etc.). Never manually construct `pgtype.UUID{}`, `pgtype.Timestamptz{}`, etc.
- **Pagination**: Use the generic `TrimPage[T]` helper with `CursorKeyer` interface. Don't write per-entity pagination trimming.
- **Error responses**: Use `app.serverErrorResponse`, `app.badRequestResponse`, `app.notFoundResponse`, `app.failedValidationResponse`, etc. Never write `app.writeJSON(w, 500, ...)` inline.
- **Buffer pool**: JSON encoding uses `bufPool` (sync.Pool) in `writeJSON`. Don't allocate new buffers for JSON.
- **Context timeouts**: Use `queryContext(ctx)` (3s) for single queries, `txContext(ctx)` (5s) for transactions. Don't hardcode timeouts in model methods.
- **Query string reading**: Use `readString(qs, key, default)` and `readInt(qs, key, default)` helpers. Don't parse query params manually.
- **Background tasks**: Use `app.background(fn)` for goroutines — it handles WaitGroup tracking and panic recovery. Never launch bare `go func()`.
- **Circuit breaker integration**: External HTTP clients (MP, WhatsApp, mailer) call `cb.AllowRequest()` before requests, then `cb.RecordSuccess()`/`cb.RecordFailure()` based on result. Follow this pattern for any new external service.

### Architecture rules

- **Never edit `internal/db/`** — it's sqlc-generated. Edit `db/queries/*.sql` and run `make sqlc`.
- **Config loading**: Flags first, env vars override. Nest related config in sub-structs of `config`. Feature enablement is conditional on config presence (e.g., Redis, WhatsApp).
- **Middleware composition**: Middlewares are nested wrappers returning `http.Handler`. Auth chain is layered: `authenticate` (sets context) → `requireAuth` (enforces login) → `requireComplexOwner` (verifies ownership, sets complex in context) → `requireRole` (checks role).
- **Context keys**: Use unexported typed keys (`type contextKey string`). Set in middleware, retrieve in handlers via `contextGet*` methods.
- **Store implementations**: Wrap sqlc queries in model methods. Map between domain structs and sqlc params/results. Translate PostgreSQL errors to domain errors (e.g., unique constraint `23505` → `ErrDuplicateEmail`).
- **External service clients**: Constructor takes config + optional circuit breaker. HTTP client with explicit timeout. Methods follow `AllowRequest → Do → RecordSuccess/Failure` pattern.
- **Async work**: Use the durable queue (`internal/jobs`, reached through `notifications.Service`) for async tasks that must not be lost — emails, WhatsApp. Every enqueue carries a deduplication key, because the queue is at-least-once and a redelivery would otherwise send the same message twice. Use `app.background(fn)` for fire-and-forget work (audit logs). Never do I/O synchronously in the request path if it can be async.
- **Security**: Timing-safe password comparison (always run bcrypt, even for non-existent users). PII masking in logs (`maskPhone`). CSRF validation derives token from access_token cookie.
