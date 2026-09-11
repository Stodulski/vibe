# Tasks: Extract `newApplication` from `main()`

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~775 (slice 1) + ~150 (slice 2) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (slice 1) → PR 2 (slice 2) |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

Slice 1 is ~775 of the 800-line budget — no headroom. If it grows, the named cut
point is before repointing `testutils_test.go` (Phase 6): slice 1 then ships
with the harness still on a divergent copy and no route test proving
`newApplication` yet, per the design's own stated fallback. This is a boundary
report only — sdd-apply/orchestrator picks the chain strategy.

### Suggested Work Units

| Unit | Goal | PR | Focused test | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `newApplication` replaces inline wiring; unit suite proves route/authz identity | PR 1 | `go test ./cmd/api/... ./internal/realtime/... -race` | `newTestApplication` → `.routes()` | revert `app.go` + repointed callers, `main.go` keeps inline wiring |
| 2 | Integration harness repointed; newly-live suite catalogued | PR 2 | `make e2e-db-up && make test/integration && make test/security` | real Postgres via `newIntegrationApp` | revert `testutils_integration_test.go` only |

## Phase 0: Re-sync with concurrent edits
- [x] 0.1 Re-read `cmd/api/main.go`, `cmd/api/cron.go`, `internal/health/`, `internal/scheduler/` for drift since design; confirm the file list below still applies before editing any of them.

## Phase 1: `internal/realtime/hub.go` — remove constructor side effect
- [x] 1.1 [RED] `internal/realtime/hub_test.go`: assert `NewHub(rdb, logger)` starts no subscriber; `Start()` starts exactly one; a second `Start()` call stays at one. Fails against current `hub.go`.
- [x] 1.2 [GREEN] Drop `go h.consume()` from `NewHub`; add idempotent `(*Hub) Start()` that launches `consume` only when `rdb != nil`. 1.1 passes; `go test ./internal/realtime/... -race` green.

## Phase 2: `cmd/api/app.go` (new file)
- [x] 2.1 `type deps struct{ logger, models data.Models, db *pgxpool.Pool, rdb *redis.Client, storage storage.ObjectStorage }`.
- [x] 2.2 `validateDeps(d deps) error`: all 15 `data.Models` stores non-nil, names the missing one.
- [x] 2.3 [RED] `app_test.go`: `newApplication` with one store nil in `deps.models` returns an error naming that store. Fails before 2.2.
- [x] 2.4 `newApplication` phase 2 (locals, dependency order): `respond` → blacklist/hub/mp/wa/mailer/notifier-or-`memoryQueue` → `auditor` → `tokens` → `middleware` → `userCache` → places…`health` → `notify` → `auth` → `payments` → `bookings` (`Refunds: paymentsHandler` local) → `complexes` → `scheduler`.
- [x] 2.5 `newApplication` phase 3: one write-only block publishing every local into `*application`; return it.
- [x] 2.6 `memoryQueue` implementing `notifications.Queue` for `rdb == nil` (recording, not Redis-backed).
- [x] 2.7 [RED→GREEN] `app_test.go`: two `newApplication` calls in one process with equivalent `deps` — no panic.
- [x] 2.8 Verify Phase 2: `go build ./cmd/api/...` succeeds; 2.3 and 2.7 pass.

## Phase 3: `cmd/api/adapters.go`
- [x] 3.1 `healthProbes(db *pgxpool.Pool, rdb *redis.Client) (...)` becomes a free function, no `*application` receiver.
- [x] 3.2 `userCache` holds `*middleware.Middleware` directly (not `*application`); update `InvalidateUser`. `go build ./cmd/api/...` succeeds.

## Phase 4: `cmd/api/main.go` — repoint
- [x] 4.1 Replace the inline `app := &application{...}` + wiring block with `deps{...}` + `newApplication(cfg, deps)`; error path calls `os.Exit(1)`.
- [x] 4.2 Delete `unwiredDependencies()` and its call site.
- [x] 4.3 Add `app.events.Start()` beside `app.notifier.Start()`.
- [x] 4.4 Verify: `go build ./...` succeeds.

## Phase 5: `cmd/api/mock_stores_test.go`
- [x] 5.1 Add `mockPasswordResetStore` (`InsertWithCooldown`, `GetByHash`, `DeleteByUser`, `DeleteExpired`) and `mockLockStore` (`TryAdvisory`), satisfying `data.PasswordResetStore`/`data.LockStore`. `go vet ./cmd/api/...` clean.

## Phase 6: `cmd/api/testutils_test.go` — repoint
- [x] 6.1 Replace inline wiring with `deps{...}` + `newApplication`; add `PasswordReset`, `Locks` to the mock `data.Models` literal (13 → 15 stores); delete ~150 wiring lines.
- [x] 6.2 Verify: `go test ./cmd/api/... -race` green.

## Phase 7: Safety-net proof — route table and authz unchanged (ends PR 1)
- [x] 7.1 Run `TestAPISurfaceMatchesTheInventory`, `TestEveryRouteIsGuarded`, `TestRouteAuthorizationMatrix`, `TestPublicRouteAllowlistIsCurrent` against the repointed harness — all pass unchanged: `go test -run 'TestAPISurfaceMatchesTheInventory|TestEveryRouteIsGuarded|TestRouteAuthorizationMatrix|TestPublicRouteAllowlistIsCurrent' ./cmd/api/...`.
- [x] 7.2 `golangci-lint run` clean on every changed file.

## Phase 8 (Slice 2, PR 2): integration harness — repoint, catalogue, don't fix
- [x] 8.1 Repoint `newIntegrationApp` in `testutils_integration_test.go` onto `newApplication` with `deps{db: pool, models: data.NewModels(pool, ...), ...}`; add `t.Cleanup` closing `app.shutdown`.
- [x] 8.2 Run `make e2e-db-up && make test/integration && make test/security`; record every failure/panic the newly-live suite reveals in `verify-report.md` — fix only what blocks the suite from executing, nothing else.
- [x] 8.3 Record, from that first run, whether `TestSecurity_RateLimit_AuthEndpoints` needs `deps.rdb` on `miniredis` or the in-memory limiter is the intended path (open question from design.md). Resolved: neither — `newApplication`'s locals-then-publish design bakes `cfg.limiter`/`cfg.env` into `middleware.Config`/`auth.Config` at construction time, so this test's post-construction `app.config.limiter.enabled = true` mutation (and `TestSecurity_Cookies_SecureAttributes`'s `app.config.env = "production"`) has no effect on the already-built handlers, regardless of backend. See `verify-report.md` Finding 1.
