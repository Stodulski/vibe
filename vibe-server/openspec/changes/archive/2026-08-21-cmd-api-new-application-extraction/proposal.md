# Proposal: Extract `newApplication` from `main()`

## Intent

`main()` wires ~270 lines of dependencies inline, so an `*application` exists only by running the binary. `newIntegrationApp` hand-builds a literal with no `middleware`, `respond`, `tokens`, `auditor` or handlers, so `app.routes()` panics at `routes.go:31`. The integration suite and `make test/security` have never executed. Finding 93; gates `authorization-route-test-coverage` (eight surviving authorization mutations) and `security-test-wiring`.

## Scope

### In Scope
- `newApplication(cfg config, d deps) (*application, error)`.
- Repoint `newTestApplication` and `newIntegrationApp` onto it, deleting two duplicate wiring copies.
- The minimum fixes required for the now-live integration suite to pass.

### Out of Scope
- Any DI framework or wider composition-root redesign.
- The authorization matrix test itself.
- Unrelated defects the live suite reveals — catalogue, do not fix.

## Capabilities

### New Capabilities
- `application-composition-root`: what the constructor owns, what callers inject, and the wiring-order invariant.

### Modified Capabilities
- None (no specs exist yet).

## Approach

**Stays in `main()`** — process lifecycle, global registries, or I/O: flag/env parsing, logger, Sentry init and flush, `openDB`, Redis dial and fallback, R2 client, `expvar` publishing, `notifier.Start`, `RegisterWorkers`, `startCronJobs`, `serve`, `os.Exit`. `expvar.NewString("version")` panics on re-registration, so it must never enter a function tests call twice.

**Moves in**: breakers, mp/wa/mailer, blacklist, hub, notifier, notifications service, auditor, tokens, middleware, the twelve handlers, health probes, scheduler. JWT validation returns an error instead of exiting.

**Injectable `deps`** — the substitution boundary: `logger`; `models data.Models` (real pool or mocks); `db *pgxpool.Pool` (health probe only, nil-safe); `rdb *redis.Client` (nil ⇒ in-memory fallback, `miniredis` when the Redis path is under test); `storage`. Everything else derives from `cfg`.

**Ownership**: the constructor allocates no goroutine, opens no connection and closes nothing. Whoever opens `db`/`rdb` closes them — `main()` by `defer`, tests by `t.Cleanup`. No partial-cleanup path is needed because nothing it builds holds a socket.

**Ordering trap (verified)**: `app.notify` is assigned at `main.go:636`, after `auth`, `payments` and `bookings` capture it at 569/589/608. They hold a nil `*notifications.Service`, whose methods dereference `s.queue`. `POST /api/v1/auth/register` panics before its 201 and returns 500 today; `newTestApplication` copies the same order, so unit tests never see it. A natural constructor order silently fixes this. Slice 1 therefore preserves the order exactly; slice 2 fixes it as a named behaviour change.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/api/main.go` | Modified | Wiring moves out; flags, infra and exit remain |
| `cmd/api/app.go` | New | `newApplication`, `deps` |
| `cmd/api/testutils_test.go` | Modified | ~150 duplicate wiring lines deleted |
| `cmd/api/testutils_integration_test.go` | Modified | `newIntegrationApp` builds a real app |
| `internal/auth`, `internal/payments`, `internal/bookings` | Unchanged | Consumers of the ordering fix only |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Behaviour change hidden in a mechanical diff | High | Slice 1 moves lines without editing them; route inventory must stay byte-identical |
| `expvar` double registration panics across tests | Med | Excluded from the constructor by decision |
| Live suite surfaces unknown failures | High | Catalogue in the verify report; fix only what blocks the suite |
| Slice 1 exceeds the 800-line budget | Med | Fallback split: extract first, repoint `newTestApplication` second |

## Rollback Plan

Each slice is one revert. Slice 1 restores inline wiring; slice 2 restores the dead harness. No schema, config or data migration is involved, and the app is not in production.

## Dependencies

- None. This is Phase 0 of the remediation program.

## Success Criteria

- [ ] `main()` is flag parsing, infrastructure, one `newApplication` call, `serve()`, `os.Exit`.
- [ ] `TestAPISurfaceMatchesTheInventory` and `TestEveryRouteIsGuarded` pass unchanged, driven by `newApplication`.
- [ ] `make test`, `go build ./...` and `golangci-lint run` stay green.
- [ ] `make e2e-db-up && make test/integration` executes and passes for the first time.
- [ ] A test can build a real router with mock stores and no Redis.

## Delivery

`auto-chain`, budget 800. Two slices: **1** extract and repoint the unit harness, behaviour-identical; **2** repoint the integration harness, fix the ordering defect it exposes, catalogue the rest.
