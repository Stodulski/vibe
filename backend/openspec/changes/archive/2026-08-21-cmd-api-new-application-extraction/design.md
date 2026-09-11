# Design: Extract `newApplication` from `main()`

## Technical Approach

`newApplication(cfg config, d deps) (*application, error)` lands in a new `cmd/api/app.go`. Assembly runs in three phases under one rule: **phase 1 validates, phase 2 constructs into local variables, phase 3 publishes those locals into the struct.** Phase 2 never reads a field of `app`; phase 3 performs writes only. Every intra-constructor dependency travels through a local, which is what turns a wrong order into a build failure. `unwiredDependencies()` is deleted — it has no job left.

## Architecture Decisions

### Decision: locals-then-publish, not a checklist

**Choice**: each component becomes `x := pkg.New(...)`; its dependents take `x`, never `app.x`. A final write-only block assigns the locals.

**How it kills the named mistake**: `Refunds: app.payments` becomes `Refunds: paymentsHandler`. Move the bookings block above the payments block and `go build ./...` reports `undefined: paymentsHandler`. That is spec Requirement 4's scenario literally, and it generalises: ~30 of the 33 fields become locals, so none can be captured before it exists. Phase 3's order is semantically irrelevant because it reads nothing.

**Alternatives rejected**: *staged builder types* catch inter-stage order only — payments and bookings sit in one stage, so catching this pair needs one type per handler (12 types, 12 methods) and the data flow stops being readable. *Grouped sub-structs built bottom-up* have the same intra-group blind spot; kept only as a file-splitting tactic if `funlen` complains. Both relocate the checklist instead of removing it.

**What locals cannot cover**: `d.models`' fifteen stores are inputs. `validateDeps(d) error` runs first, before any constructor — spec Requirement 4's "checked before any constructor that could capture it" branch. Today's guard runs mid-sequence with eight handlers already built; this one cannot. Verified gap it closes: the unit harness supplies 13 of 15 stores — `PasswordReset` (captured by `auth`) and `Locks` (captured by `payments`) are nil while `unwiredDependencies()` reports everything wired.

Three back-references to `app` block pure locals; all three resolve:

| Back-reference | Resolution |
|---|---|
| `userCache{app: app}` → `app.middleware` | build `middleware` before `admin`/`auth` and pass the value; late binding removed |
| `app.healthProbes()` | free function `healthProbes(db, rdb)` |
| `Run: app.background` | kept — method value on the phase-0 shell whose `wg`/`logger` are already set, so it is order-independent by construction |

Consequence, stated plainly: this **reorders** wiring (`tokens`/`middleware` move ahead of `admin`). Slice 1 is therefore not a verbatim line move. Behaviour identity is proved by `TestAPISurfaceMatchesTheInventory`, `TestEveryRouteIsGuarded`, `TestRouteAuthorizationMatrix` and the unit suite — not by diff shape. This is safe because the proposal's ordering trap **has already been fixed in-tree**: `app.notify` is now built at `main.go:669`, before `auth`/`payments`/`bookings` (682/700/719), and `wiring_test.go` covers it. There is no ordering defect left for slice 2 to fix.

### Decision: no process-level side effects, with one API split

`expvar`, Sentry, `openDB`, Redis dial, R2, `notifier.Start`, `RegisterWorkers`, `startCronJobs`, `serve`, `os.Exit` stay in `main()`. One blocker: `realtime.NewHub` launches `go h.consume()` when `rdb != nil`. Split it — add idempotent `(*Hub).Start()`, call it from `main()` beside `notifier.Start()`; `Shutdown()` stays safe when `Start()` never ran. `middleware.New`, `notifier.New`, `scheduler.New`, `audit.NewRecorder` are already pure (the rate-limiter eviction goroutine starts in `Wrap`, not `New`). Result: the constructor allocates no goroutine and opens no connection, so calling it twice cannot panic and an error return leaks nothing.

### Decision: nil `rdb` gets a real in-memory queue

`notifier.Enqueue` is not nil-safe, so `rdb == nil` builds `memoryQueue` (a recording `notifications.Queue` in `adapters.go`) instead, exposed as `app.queue`. `notificationRecorder` in the harness is replaced by it, so unit tests assert against the same fallback production would use. Unreachable in production: `main()` still refuses to boot without Redis, before calling `newApplication`.

### Decision: ownership unchanged

`main()` opens and `defer`s `db`/`rdb`; tests use `t.Cleanup`. `gracefulShutdown` is already correct (channel closed before `srv.Shutdown`, cleanup on the timeout path) and is not touched. Test harnesses add `t.Cleanup` closing `app.shutdown` once, to reap the eviction goroutine `Wrap` starts.

## Data Flow

    main(): flags → logger → sentry → openDB → redis dial → R2
              │
              └─→ deps{logger, models, db, rdb, storage}
                        │
        newApplication ─┤ 1. validateDeps      (fail fast, nothing built)
                        │ 2. respond → blackl/hub/mp/wa/mailer/notifier|memQueue
                        │    → auditor → tokens → middleware → userCache
                        │    → places…health → notify → auth → payments
                        │    → bookings(Refunds: payments) → complexes → scheduler
                        │ 3. publish locals into *application   (writes only)
                        ↓
    main(): hub.Start → RegisterWorkers → notifier.Start → startCronJobs → serve

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `cmd/api/app.go` | Create | `deps`, `validateDeps`, `newApplication` |
| `cmd/api/main.go` | Modify | wiring removed; `unwiredDependencies` deleted; `hub.Start()` added |
| `cmd/api/adapters.go` | Modify | `healthProbes` → free function; `userCache` holds `*middleware.Middleware`; add `memoryQueue` |
| `cmd/api/testutils_test.go` | Modify | ~150 wiring lines deleted; calls `newApplication` |
| `cmd/api/mock_stores_test.go` | Modify | add `mockPasswordResetStore`, `mockLockStore` |
| `cmd/api/testutils_integration_test.go` | Modify | `newIntegrationApp` calls `newApplication` with `db: pool` |
| `internal/realtime/hub.go` | Modify | `NewHub` no longer starts `consume`; new `Start()` |

## Interfaces / Contracts

```go
type deps struct {
	logger  *slog.Logger
	models  data.Models           // all 15 stores required
	db      *pgxpool.Pool         // health probe only; may be nil
	rdb     *redis.Client         // nil ⇒ in-memory blacklist, hub, limiter, queue
	storage storage.ObjectStorage // may be nil (no R2)
}

func newApplication(cfg config, d deps) (*application, error)
```

## Testing Strategy

| Layer | What to test | Approach |
|-------|--------------|----------|
| Unit | route table + authz identity | existing `routes_surface`/`routes_audit`/`routes_authz` tests, now driven by `newApplication` |
| Unit | build with mocks, `rdb == nil` | `newTestApplication` → `.routes()` returns a guarded router |
| Unit | called twice in one process | new test: two `newApplication` calls, no panic, no `expvar` re-registration |
| Unit | missing store rejected | new test: `deps` minus one store returns an error naming it |
| Compile | wrong order fails | documented negative case in `app.go`; asserted by review, not by a test binary |
| Integration | suite executes for the first time | `make e2e-db-up && make test/integration && make test/security` |

## Threat Matrix

N/A — no shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary; the matrix's five rows are all git/shell/PR forms. HTTP route identity is the security-relevant boundary here and is covered by `TestEveryRouteIsGuarded`, `TestRouteAuthorizationMatrix` and `TestPublicRouteAllowlistIsCurrent`, which must pass unchanged.

## Migration / Rollout

No data migration. Two slices, `auto-chain`, 800-line budget.

- **Slice 1** (~775 changed lines, est.): `app.go`, repoint `main()`, repoint `newTestApplication`, add the two missing mocks, hub `Start` split, delete `unwiredDependencies`. Leaves: binary boots, unit suite green, route/authz tests now proving the shared constructor. Fallback if measured diff exceeds 800: split at "repoint `newTestApplication`" — but then slice 1 ships with the harness still holding a divergent copy and no route test covering `newApplication`.
- **Slice 2** (~150 lines): repoint `newIntegrationApp` with `db: pool`, add `t.Cleanup`, run the integration and security suites, catalogue every failure in the verify report. Fix only what blocks the suite from executing.

## Open Questions

- [ ] Does `TestSecurity_RateLimit_AuthEndpoints` need `deps.rdb` on `miniredis`, or is the in-memory limiter the intended path? Resolve in slice 2 from the first run.
- [ ] Concurrent edits are in flight in `main.go`, `cron.go`, `internal/health`, `internal/scheduler`. Apply MUST re-read every file before editing.
