# Application Composition Root Specification

## Purpose

Defines the observable contract for `newApplication`, the constructor that
replaces `main()`'s inline wiring: what it accepts, what it must never do, who
owns cleanup, and the guarantee that a wrong assembly order fails `go build`,
not a test run or a runtime checklist. Behaviour preservation is the
acceptance bar — the route table and its authorization matrix stay unchanged.

## Requirements

### Requirement: Constructor accepts config and a substitution boundary

The system MUST expose `newApplication(cfg config, d deps) (*application, error)`.
`deps` MUST hold exactly the externally-injectable values: `logger`,
`models data.Models`, `db *pgxpool.Pool` (health probe only, MAY be nil),
`rdb *redis.Client` (MAY be nil — a nil value MUST fall back to in-memory
implementations), and `storage`. Every other `*application` field MUST derive
from `cfg` inside the constructor.

#### Scenario: Building with mocks and no Redis
- GIVEN `deps.models` holds mock stores and `deps.rdb` is nil
- WHEN `newApplication` is called
- THEN it returns a working `*application` on the in-memory fallback, with no panic or error

#### Scenario: Real dependencies reproduce the pre-extraction wiring
- GIVEN `deps` holds a live pool and a connected Redis client
- WHEN `newApplication` is called
- THEN the returned application is wired identically to what `main()` built before extraction

### Requirement: The constructor performs no process-level side effects

The constructor MUST NOT launch a goroutine, open a network or database
connection, or register a process-global (`expvar`, signal handlers,
`notifier.Start`, cron scheduling). These stay in `main()`.

#### Scenario: Called twice in one process
- GIVEN a test calls `newApplication` twice in the same binary with equivalent `deps`
- WHEN both calls run
- THEN neither panics and no `expvar` duplicate-registration or double-listen error occurs

#### Scenario: A returned error leaks nothing the constructor did not own
- GIVEN `deps.db` and `deps.rdb` are caller-supplied
- WHEN `newApplication` returns an error
- THEN it has opened no connection and started no goroutine needing extra cleanup

### Requirement: Resource ownership is unambiguous

Whoever opens `db` and `rdb` closes them — `main()` via `defer`, tests via
`t.Cleanup`. The constructor MUST NOT close or take ownership of a `db`/`rdb`
value it did not open.

#### Scenario: main() shuts down cleanly
- GIVEN `main()` opened `db` and `rdb` before calling `newApplication`
- WHEN the process receives a shutdown signal
- THEN `main()`'s own deferred close/shutdown calls run; `newApplication`'s output needs no extra cleanup step

#### Scenario: A mocks-only test needs no cleanup
- GIVEN a unit test supplies mock stores and a nil `rdb`
- WHEN the test ends without calling any close function
- THEN nothing `newApplication` opened is leaked, because it opened nothing

### Requirement: An incorrect wiring order fails to compile

Assembly MUST be structured so a handler consuming another handler's output
(for example `bookings.Dependencies.Refunds: app.payments`) cannot be assigned
before that value exists — as a compile-time property. A runtime checklist
over a subset of fields MUST NOT be the sole safeguard against this defect
class.

#### Scenario: Swapping two ordered constructor calls
- GIVEN the source moves the bookings-handler call before the payments-handler call it depends on
- WHEN `go build ./...` runs
- THEN the build fails, without needing `go test` or a running application to expose it

#### Scenario: Every captured field is compile-checked or checked before capture
- GIVEN the full set of fields `*application` composes
- WHEN `newApplication` assembles them
- THEN each field a later constructor captures is either unrepresentable in the wrong order, or covered by an explicit check that runs before any constructor that could capture it as nil

> **DELIVERED WITH A KNOWN LIMIT — recorded because the archived spec has to be
> true, and because this requirement's own worked example is the case that
> escapes it.**
>
> Locals-then-publish makes wrong *order* a build error: moving the bookings
> block above `paymentsHandler`'s yields `undefined: paymentsHandler`
> (mutation-verified at `cmd/api/app.go:306`). Scenario 1 holds.
>
> It does not make wrong *source* one. `app` exists as a zero-valued struct
> from the start of the constructor, so `Refunds: app.payments` — the exact
> assignment this requirement cites — stays syntactically valid at any point,
> compiles clean, passes the whole suite, and hands `bookings` a nil, because
> `paymentsHandler` is built at `:297` and `app.payments` is not published
> until `:378`. A nil `*payments.Handler` in an interface field reads as
> non-nil, so nothing downstream can detect it either.
>
> `TestNewApplicationWiresEveryField` does not close this: `app.bookings` comes
> back non-nil; the nil is a dependency *inside* the handler. Closing it means
> each module's `NewHandler` validating its own `Dependencies` — the shape
> `validateDeps` already applies to the 15 `data.Models` stores, pushed down
> one level. Considered and declined in design.md's rejected alternatives, and
> named in `cmd/api/app_test.go`'s own comments so the next reader of the code
> finds it without reading this file.
>
> Accepted as a bounded, documented gap rather than closed here: it crosses
> every module, and burying that in a composition-root change would make the
> change unreviewable.

### Requirement: Route table and authorization are unchanged

`newApplication` MUST produce a route table byte-identical to the
pre-extraction table, with every route's authorization guard unchanged.

#### Scenario: Slice 1 preserves order and behaviour exactly
- GIVEN the pre-extraction wiring order, including the now-fixed nil-`notify`-capture history
- WHEN `newTestApplication` is repointed onto `newApplication`
- THEN `TestAPISurfaceMatchesTheInventory` and `TestEveryRouteIsGuarded` pass unchanged

#### Scenario: Slice 2 fixes the ordering defect as a named change
- GIVEN `newIntegrationApp` is repointed onto `newApplication`
- WHEN the integration suite runs against a real router
- THEN it no longer panics on a nil `*Middleware`, and any behaviour change from the ordering fix is documented, not silently folded into "just a refactor"

### Requirement: Test substitution boundary

A test MUST be able to build a real router with mock stores and either the
in-memory `rdb` fallback or `miniredis`, with no live external dependency
beyond what the `integration`-tagged harness explicitly opts into.

#### Scenario: Unit test with no Redis
- GIVEN a test supplies `deps.rdb = nil`
- WHEN it builds an app and calls `.routes()`
- THEN the router is fully guarded and functional on in-memory rate limiting, blacklist and hub fallbacks

#### Scenario: Redis-path test uses miniredis, not a live server
- GIVEN a test constructs a `*redis.Client` pointed at a `miniredis` instance and passes it as `deps.rdb`
- WHEN `newApplication` wires it
- THEN the Redis-backed implementations run against it with no network dependency outside the process

## Out of Scope

- A broader dependency-injection framework or wider composition-root redesign.
- The authorization matrix test itself (already landed in `routes_authz_test.go`).
- Unrelated defects the newly-live integration suite reveals — catalogue, do not fix.
