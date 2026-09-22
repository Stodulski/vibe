# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make run                # Run API server (go run ./cmd/api)
make build              # Build binary (CGO_ENABLED=0 go build -o ./bin/api ./cmd/api)
make test               # Run unit tests (go test -race -shuffle=on -v -count=1 ./...)
make lint               # Run golangci-lint
make tidy               # go mod tidy + go fix
make audit              # go mod verify + govulncheck

# Run a single test
go test -race -v -run TestHandlerName ./cmd/api/

# OpenAPI
make generate/api       # Regenerate internal/openapi/gen after editing internal/openapi/openapi.yaml; CI re-runs this and fails the build on a diff
make lint/openapi       # Check the embedded OpenAPI document loads and validates

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

**Go 1.27 backend** — Multi-tenant SaaS for padel court management. Deployed on Railway.

### Request flow

```
HTTP → middleware chain (internal/middleware) → route table, spec-validated outside production (cmd/api/routes.go) → gen.HandlerWithOptions dispatch, wrapped by the route's guard chain (cmd/api/apiserver_guards.go) → domain Handler method (internal/<domain>) → domain store (internal/stores, internal/<domain>/store) → sqlc queries (internal/db/) → PostgreSQL
```

Every operation in `internal/openapi/openapi.yaml` is registered through the generated `gen.ServerInterface`; `cmd/api/apiserver.go` implements it and dispatches each operation, unchanged, to its domain's own handler (e.g. `AdminListAuditLog` calls `s.app.admin.ListAuditLogs`). There is no `httprouter` any more — see `docs/adr/0001-httprouter-and-the-move-to-net-http.md`.

Middleware chain order, outermost first (`Middleware.Wrap`, `internal/middleware/middleware.go`): `RequestID → LogRequests → RecoverPanic → SecurityHeaders → CORS → RateLimit → CSRFProtect → Authenticate → RateLimitUser → routes`. RequestID sits outside RecoverPanic on purpose: RecoverPanic reports through the Responder, which stamps the request id from the context onto the log line, so recovery has to run inside the id or the one log line that matters most would carry no id to join to the one the client was handed. The route table itself is wrapped in the OpenAPI spec validator (`cmd/api/routes.go`) before the middleware chain ever sees it, so a request that does not conform to the document is rejected before the per-route guards run (see Auth chain below) — the validator is `nil` in production and wherever `OPENAPI_VALIDATE_REQUESTS` is off.

### Code layout

- **`cmd/api/`** — The composition root, not the handlers. `main.go` builds `*application` (every injected dependency, including one `*<domain>.Handler` field per domain — see `apiServer.PublicsiteSitemapMoved` etc. calling `s.app.publicsite.SitemapMoved`); `apiserver.go` implements the generated `gen.ServerInterface`, one method per OpenAPI operation, each just calling its domain handler; `apiserver_guards.go` carries the `routeGuards` table (the auth/ownership/role/idempotency chain each route needs) and the OpenAPI parameter-binding error translation; `routes.go` builds the route table and middleware chain. There is no per-domain handler file here any more (no `auth.go`, `bookings.go`, `courts.go`) — HTTP handlers live in `internal/<domain>/`.
- **`internal/<domain>/`** (`admin`, `audit`, `auth`, `bookings`, `clients`, `complexes`, `courts`, `health`, `leads`, `openapi`, `payments`, `places`, `publicsite`, `realtime`, `reporting`, …) — each domain owns its HTTP handlers as methods on its own `*Handler` type, its business logic, and (mostly under `<domain>/store/`) its sqlc-backed store. A domain's `NewHandler` takes an `*httpx.Responder` (or a `*httpx.Refuser` wrapping it with a domain-specific error table) rather than `*application`, which is what keeps `cmd/api` the only package that has to know every domain exists.
- **`internal/stores/`** — `stores.Stores` (`stores.go`) is the struct that aggregates one interface per domain store (`UserStore`, `BookingStore`, …), still segregated by responsibility (ISP): e.g. `BookingStore` composes `BookingCreator`, `BookingReader`, `BookingUpdater`, `BookingStatsQuerier`, `BookingReminderManager`, `BookingLifecycleManager`. `stores.New(pool, cfg)` wires the real sqlc-backed implementations; `cmd/api/testutils_test.go` builds a `stores.Stores` of mocks field-by-field instead.
- **`internal/data/`** — no longer holds stores or `Models`. What is left is the plumbing every store package imports: type conversions (`convert.go`, exported — `UUIDToPg`, `PgToUUID`, `DateToPg`, `PgToDate`, `TimeToPg`, `PgToTime`, `TextToPg`, `PgToTextPtr`, …), domain error sentinels (`errors.go`), cursor pagination (`filters.go`), query/tx timeouts (`timeout.go`, see `QueryContext`/`TxContext`), advisory locks and retry helpers, sqlstate translation, and the tenant-scoping mechanism (`tenant.go` — see Database below).
- **`internal/openapi/`** — `openapi.yaml` is the source of truth for the whole HTTP surface. `internal/openapi/gen` is its oapi-codegen output (`gen.ServerInterface` and the per-operation parameter types); it is committed and regenerated with `make generate/api`, never edited by hand.
- **`internal/platform/`** — the infrastructure the composition root wires and the domain never names: `config/` (the whole flag and environment surface, loaded once by `config.Load`; nothing else in the tree reads the environment), `db/` (the pgx pool and its slow-query tracer) and `redis/` (the client). A platform package never imports a domain package.
- **`internal/db/`** — **sqlc-generated code — do not edit manually.** Edit SQL in `db/queries/*.sql`, then run `make sqlc`.
- **`db/migrations/`** — Goose migrations (PostgreSQL). `001_init.sql` is the pre-launch chain squashed twice (001–042, then 002–005 on 2026-09-14), and its header says why; it was immutable-while-no-production-users until 2026-09-22, when Vibe's first production deploy ended that condition. From that date every change is a new numbered file on top of it — `002_counter_payment_methods.sql` is the first — and every applied file is immutable. See `docs/adr/0003-goose-migrations.md`.
- **`db/queries/`** — SQL queries consumed by sqlc. One `.sql` file per entity.
- **`internal/`** subpackages — Cross-cutting services: `circuitbreaker/`, `mailer/` (Brevo API + SMTP fallback), `jobs/` (the durable Postgres work queue), `storage/` (Cloudflare R2), `validator/`, `whatsapp/`, `mp/` (MercadoPago).

### Key patterns

- **Dependency injection**: All services/stores injected via `*application` struct in `main.go`. Store interfaces allow mock implementations for testing.
- **Context passing**: User and Complex objects flow through `context.Context` (see `httpx.ContextSetUser`/`httpx.ContextGetAuthenticatedUser` and `httpx.ContextSetComplex`, `internal/httpx/context.go`). Request ID attached per request.
- **Auth chain**: routes are not guarded inline any more. `cmd/api/apiserver_guards.go`'s `routeGuards` table maps every `"METHOD /path"` to a `guardKind` (`guardPublic`, `guardAuth`, `guardOwner`, `guardSuperAdmin`), and `guard()` applies the real implementations from `httpx.Guards` (built by `Middleware.Guards()`, `internal/middleware/middleware.go`): `guards.RequireAuth` → `guards.RequireAuth(guards.RequireComplexOwner(...))` → `guards.RequireAuth(guards.RequireSuperAdmin(...))`, where `RequireSuperAdmin` is `m.RequireRole("superadmin")` underneath. `TestRouteGuardsMatchesInventory` (`cmd/api/apiserver_test.go`) proves the table's key set is exactly the registered route surface.
- **Cursor-based pagination**: `Filters` struct with cursor + limit. Generic `TrimPage` helper. Composite cursor (date+ID) for bookings.
- **Type conversions**: `internal/data/convert.go` has helpers between Go types and pgx types (uuid, date, time, text). Always use these helpers — never convert pgx types manually.
- **Circuit breaker**: External services (MercadoPago, WhatsApp, mailer) wrapped with circuit breaker (closed→open→half-open). Safe on nil receiver (no-op when not configured).
- **Dual rate limiting**: address-keyed general/auth/booking ceilings (`Middleware.RateLimit`, before authentication) plus a per-account ceiling keyed on user ID (`Middleware.RateLimitUser`, after `Authenticate`, so it catches one account arriving from many addresses). Redis-backed distributed buckets when configured, in-memory fallback otherwise — see below for why production never actually runs the fallback.
- **Redis is required in production, not optional**: rate limiting, the token blacklist, the user cache, the SSE hub and slot locking all have in-memory fallbacks, but every fallback is per-instance — correct on a single instance, silently wrong the moment a second one is running during a rolling deploy (a session revoked on one instance stays live on the other, a rate limit gets enforced twice over, an SSE event is missed). `main.go` refuses to boot without a reachable `REDIS_URL` for exactly that reason; it is not a "degrades gracefully" feature flag. The in-memory fallback still exists for the test harness, which builds an `*application` with `rdb: nil` (`newTestApplication`, `cmd/api/testutils_test.go`).

### Testing

- **Unit tests** live in both `cmd/api/` and each domain's own `internal/<domain>/` package now that handlers moved there — a booking-handler test belongs next to `internal/bookings/bookings.go`, not in `cmd/api`. `cmd/api/*_test.go` covers wiring, route inventory and cross-cutting request-level behaviour (guard matrices, CORS, CSRF, the OpenAPI conformance check). Mock stores implement the segregated interfaces (e.g., `mockBookingStore`, `cmd/api/mock_stores_test.go`) and are assembled into a `stores.Stores`.
- `newTestApplication(t)` (`cmd/api/testutils_test.go`) builds an `*application` with every store mocked; `newTestApplicationWithStores(t, customize)` lets a test swap in one store's double before the app (and the domain services that captured it at construction) are built. `newTestServer(t, app)` wraps `app.routes()` in an `httptest.Server`.
- There is no `testGet`/`testPost`/`generateTestToken` helper — `cmd/api` tests issue requests with `http.NewRequestWithContext` (or the local `postJSON` helper in `wiring_test.go`) against the `httptest.Server`'s URL, and mint a token directly from the harness's token service (`app.tokens.GenerateAccessToken(userID, role)`) rather than through a wrapper. Domain packages (e.g. `internal/auth`) build their own fixtures (`stubs_test.go`'s `fixture`) instead of reusing `cmd/api`'s harness.
- **Integration tests** (`*_integration_test.go`): Build tag `//go:build integration`. Require `DATABASE_URL` env var pointing to real Postgres. `setupTestDB()` in `cmd/api/testutils_integration_test.go`. They are not limited to `cmd/api` — `internal/data` and most `internal/<domain>/store` packages carry their own; `make test/integration` discovers every package with an integration file and runs them serially (`-p 1`, one shared E2E database).

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

### Handler conventions

Handlers are methods on each domain's own `*Handler` type (`internal/<domain>/`), not on `*application`. The response-writing conventions below are unchanged in spirit from before the move to per-domain packages; only where they live changed: `app.writeJSON`/`envelope`/`app.readJSON`/`serverErrorResponse` and friends are gone, replaced by the shared `internal/httpx` package every domain imports.

- Handlers are methods on a domain's `*Handler`: `func (h *Handler) NameHandler(w http.ResponseWriter, r *http.Request)`. `NewHandler` takes an `*httpx.Responder` (or a `*httpx.Refuser`, see below) and stores it on the type, e.g. `respond *httpx.Refuser`.
- All JSON responses go through the `Responder`, not a package-level `writeJSON`: `h.respond.JSON(w, r, status, httpx.Envelope{"key": data})`. `httpx.Envelope` (`internal/httpx/json.go`) is the `map[string]any` wrapper every response body is written in — the same role the old `envelope` type played, just exported from `httpx` so every domain package can use it.
- Request payloads use **inline anonymous structs** with `json` tags — never standalone input types — decoded with the package-level `httpx.ReadJSON`, not a method on the handler:
  ```go
  var input struct {
      Email string `json:"email"`
      Name  string `json:"name"`
  }
  err := httpx.ReadJSON(w, r, &input)
  ```
- Error responses always go through the `Responder`'s methods: `h.respond.ServerError`, `h.respond.BadRequest`, `h.respond.NotFound`, `h.respond.FailedValidation`, etc. (`internal/httpx/errors.go`). Never write error JSON manually. A domain with its own error table wraps the shared `*httpx.Responder` in a `*httpx.Refuser` (`respond.WithRefusals(table)`) so `h.respond.Refuse(...)`/`h.respond.DomainError(...)` can answer that domain's sentinels before falling back to the shared ones — the type keeps every generic `Responder` method too, since `Refuser` embeds it.
- Validation follows the fluent pattern — create `validator.New()`, chain `v.Check()` calls, then check `v.Valid()` — unchanged:
  ```go
  v := validator.New()
  v.Check(input.Email != "", "email", "must be provided")
  v.Check(validator.Matches(input.Email, validator.EmailRX), "email", "must be valid")
  if !v.Valid() {
      h.respond.FailedValidation(w, r, v.Errors)
      return
  }
  ```
- Domain errors translate to HTTP responses via `errors.Is()` switch, same shape as before, just called on the handler's `respond` field:
  ```go
  case errors.Is(err, data.ErrRecordNotFound):
      h.respond.NotFound(w, r)
  ```

### SOLID principles

- **Single Responsibility**: One handler file per domain. One store file per entity. Each internal/ package does one thing.
- **Open/Closed**: New functionality = new handlers/methods. Don't modify existing handlers to add unrelated behavior.
- **Liskov Substitution**: Mock stores must satisfy the same interfaces as real stores. All store implementations are interchangeable through interfaces.
- **Interface Segregation (ISP)**: Store interfaces are composed from small, focused interfaces. Example: `BookingStore` = `BookingCreator` + `BookingReader` + `BookingUpdater` + `BookingStatsQuerier` + `BookingReminderManager` + `BookingLifecycleManager`. When adding new store methods, add them to the correct sub-interface (or create a new one). Never bloat an existing interface with unrelated methods.
- **Dependency Inversion**: Handlers depend on store interfaces (defined in `internal/stores/stores.go`), never on concrete implementations. `stores.Stores` aggregates every domain's store interface; `stores.New(pool, cfg)` (`internal/stores/stores.go`) wires the real, sqlc-backed implementations in production, and tests substitute mocks field-by-field instead.

### DRY rules

- **Type conversions**: Always use the exported helpers from `internal/data/convert.go` (`data.UUIDToPg`, `data.PgToUUID`, `data.DateToPg`, `data.PgToDate`, `data.TimeToPg`, `data.PgToTime`, `data.TextToPg`, `data.PgToTextPtr`, etc. — exported because every `internal/<domain>/store` package calls them from outside `internal/data`). Never manually construct `pgtype.UUID{}`, `pgtype.Timestamptz{}`, etc.
- **Pagination**: Use the generic `TrimPage[T]` helper with `CursorKeyer` interface. Don't write per-entity pagination trimming.
- **Error responses**: Use the `Responder`'s methods — `h.respond.ServerError`, `h.respond.BadRequest`, `h.respond.NotFound`, `h.respond.FailedValidation`, etc. (`internal/httpx/errors.go`). Never write `httpx.WriteJSON(w, 500, ...)` inline.
- **Buffer pool**: JSON encoding uses `bufPool` (`sync.Pool`) inside `httpx.WriteJSON`/`httpx.WriteProblemJSON` (`internal/httpx/json.go`). Don't allocate new buffers for JSON.
- **Context timeouts**: Use `data.QueryContext(ctx)` (3s) for single queries, `data.TxContext(ctx)` (5s) for transactions — called by store methods as `data.QueryContext`/`data.TxContext`, not as bare local functions. Don't hardcode timeouts in store methods.
- **Query parameters**: there is no `readString`/`readInt` helper any more. A route's query and path parameters are bound by the generated `gen.ServerInterface` method signature (typed `Params` structs and named path arguments, from `internal/openapi/openapi.yaml`) before the handler ever runs; a handler reads what oapi-codegen already parsed rather than parsing `r.URL.Query()` itself. A binding failure (missing required parameter, wrong format) is answered by `apiServerParamError` (`cmd/api/apiserver_guards.go`) as a 422 naming the field, before any guard or handler runs.
- **Background tasks**: Use `app.background(fn)` for goroutines — it handles WaitGroup tracking and panic recovery. Never launch bare `go func()`.
- **Circuit breaker integration**: External HTTP clients (MP, WhatsApp, mailer) call `cb.AllowRequest()` before requests, then `cb.RecordSuccess()`/`cb.RecordFailure()` based on result. Follow this pattern for any new external service.

### Architecture rules

- **Never edit `internal/db/`** — it's sqlc-generated. Edit `db/queries/*.sql` and run `make sqlc`.
- **Never edit `internal/openapi/gen/`** — it's oapi-codegen output. Edit `internal/openapi/openapi.yaml` and run `make generate/api`; CI re-runs the same target and fails the build on a diff.
- **Config loading**: Flags first, env vars override. Nest related config in sub-structs of `config`. Some features are still conditional on config presence (e.g. WhatsApp is wired only when its token is configured) — Redis is the one exception: it used to be conditional and is not any more, see the Redis bullet under Key patterns.
- **Middleware composition**: Middlewares are nested wrappers returning `http.Handler`, composed once in `Middleware.Wrap` (`internal/middleware/middleware.go`) — see Request flow above for the exact chain. Per-route guards are a separate layer, applied around the generated dispatch function by `cmd/api/apiserver_guards.go`'s `guard()`, not inline in each handler: `Authenticate` (in the chain, sets context if a valid session is present) → `guards.RequireAuth` (enforces login) → `guards.RequireComplexOwner` (verifies ownership, sets the complex in context) → `guards.RequireSuperAdmin` (checks the superadmin role).
- **Context keys**: Use unexported empty-struct keys (`type userContextKey struct{}`, `internal/httpx/context.go`) — not a `contextKey string` — so no other package can construct one and a typo can't collide with another package's key. Set and read only through the exported `httpx.ContextSet*`/`httpx.ContextGet*` functions.
- **Store implementations**: Wrap sqlc queries in store methods, one store package per domain under `internal/<domain>/store/`. Map between domain structs and sqlc params/results. Translate PostgreSQL errors to domain errors (e.g., unique constraint `23505` → `ErrDuplicateEmail`).
- **External service clients**: Constructor takes config + optional circuit breaker. HTTP client with explicit timeout. Methods follow `AllowRequest → Do → RecordSuccess/Failure` pattern.
- **Async work**: Use the durable queue (`internal/jobs`, reached through `notifications.Service`) for async tasks that must not be lost — emails, WhatsApp. Every enqueue carries a deduplication key, because the queue is at-least-once and a redelivery would otherwise send the same message twice. Use `app.background(fn)` for fire-and-forget work (audit logs). Never do I/O synchronously in the request path if it can be async.
- **Adding an HTTP endpoint** touches more than one file, by design — each one is a deliberate check, not boilerplate: add the operation to `internal/openapi/openapi.yaml`, run `make generate/api` to regenerate `internal/openapi/gen`, implement the new `gen.ServerInterface` method in `cmd/api/apiserver.go` (calling into a domain handler), add its `"METHOD /path"` entry to `routeGuards` in `cmd/api/apiserver_guards.go` (`TestRouteGuardsMatchesInventory` fails otherwise), and add it to `apiSurface` in `cmd/api/routes_surface_test.go` (the deliberate inventory `TestAPISurfaceMatchesTheInventory` checks against, so a route rename or removal is visible in review). A route reachable without a session also needs an entry in `publicRoutes` (`cmd/api/routes_audit_test.go`) with the reason it is public; a cross-tenant route needs an entry in `middleware.CrossTenantRoutes` with its reason (see Database below); a public state-changing route needs an entry in `csrfExemptRoutes` (`internal/middleware/chain.go`) with its reason. Each table exists so that adding, widening, or exempting a route is a visible diff instead of a silent behavior change.
- **Security**: Timing-safe password comparison (always run bcrypt, even for non-existent users). PII masking in logs (`maskPhone`). CSRF validation derives token from access_token cookie.
