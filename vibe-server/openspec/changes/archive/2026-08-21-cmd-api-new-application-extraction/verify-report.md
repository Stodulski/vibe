# Verify Report: `cmd-api-new-application-extraction` (final)

Independent verification against `specs/application-composition-root/spec.md` (6
requirements, 12 scenarios), `design.md`, and `tasks.md`, run at commit
`7a10674` on `fix/money-path-blockers`. Reads the diffs of the five commits
that carry this change (`4004cf2`, `22c5a61`, `b76361d`, `8b88421`, `d910dc9`),
not only the current tree. Supersedes the interim slice-2 catalogue
`sdd-apply` wrote to this same path (its Findings 1–3 are preserved below,
under "Prior slice-2 catalogue, re-confirmed").

## Task completeness

25/25 tasks in `tasks.md` checked. No unchecked task.

`state.yaml` still reads `phases.apply.status: ready` with an `unblocked_note`
dated before any of the five commits landed — it was never updated after
apply finished. This is a bookkeeping gap only: `tasks.md` and the commit
history are unambiguous that apply is done. WARNING, not blocking; fix before
archive so the artifact trail is not misleading to a future reader.

## Gates, run independently (not trusting prior numbers)

| Gate | Result |
|---|---|
| `go build ./...` | clean, exit 0 |
| `make test` (`go test -race -v -count=1 ./...`) | 29/29 packages `ok`, 0 failures |
| `go test -race -count=1 ./...` | 29/29 packages `ok`, 0 failures |
| `golangci-lint run` | `0 issues.` — confirmed after a clean build, so this is a real 0, not a suppressed one |
| `make audit` (`go mod verify` + `govulncheck`) | modules verified; 0 vulnerabilities in reachable code (1 vulnerability exists in a required-but-uncalled module) |
| `make test/integration` (`-p 1`, real Postgres) | 29/29 `cmd/api` integration functions + full `./...` suite green, 0 failures, `cmd/api` in 237.6s |
| `make test/security` (`-run TestSecurity`, real Postgres) | 17/17 top-level security tests green (16 `TestSecurity_*` + `TestSecurityHeadersAreSet`), including the two `newApplication`-sensitive ones fixed in `8b88421` |
| `go test -run 'TestAPISurfaceMatchesTheInventory\|TestEveryRouteIsGuarded\|TestRouteAuthorizationMatrix\|TestPublicRouteAllowlistIsCurrent' ./cmd/api/...` | all pass |

All eight gates are green. No gate was skipped or assumed.

## Per-requirement verdict

| # | Requirement | Verdict | Evidence |
|---|---|---|---|
| 1 | Constructor accepts config and a substitution boundary | SATISFIED (Scenario 1), PARTIALLY TESTED (Scenario 2) | `deps` struct at `cmd/api/app.go:41-54` matches the 5-field boundary exactly. Scenario 1 ("mocks + no Redis") is covered end-to-end by `newTestApplication`/`newTestApplicationWithNotifications` (`testutils_test.go:15-113`), exercised by every `cmd/api` unit test. Scenario 2 ("real db + connected Redis reproduce pre-extraction wiring") has **no test that passes both a real pool and a connected/miniredis Redis client into `newApplication` together** — see "Gap" below. |
| 2 | The constructor performs no process-level side effects | SATISFIED | `realtime.NewHub` (`internal/realtime/hub.go:73-80`) launches nothing; `Start()` does, called only from `main()` (`cmd/api/main.go:568,570`), never from `newApplication`. I independently greped every `New*`/`NewHandler`/`NewRecorder` constructor reachable from `newApplication` (`audit.NewRecorder`, `scheduler.New`, `notifier.New`, `middleware.New`, `circuitbreaker.New`, `auth.NewTokenBlacklist`, `auth.NewTokenService`, all dozen `*.NewHandler`) for `go func`/`go h.x` — the only goroutine launch reachable through construction-adjacent code is `internal/middleware/ratelimit.go:122`'s `go buckets.evict(...)`, which fires inside `(*Middleware).RateLimit`, called from `Wrap` at `cmd/api/routes.go:31` — i.e. from `app.routes()`, never from `newApplication`. `TestNewApplicationCanBeCalledTwiceInOneProcess` (`app_test.go:101-116`) passes, though it only asserts the two returned pointers differ — see WARNING below, it does not itself detect a leaked goroutine or `expvar` panic; the guarantee rests on the source audit above, not on that test's assertions. |
| 3 | Resource ownership is unambiguous | SATISFIED | `main.go:467` `defer db.Close()`, `main.go:513` closes `rdb`; both open before calling `newApplication` and close after, unmodified by this change. Scenario 3.2 ("mocks-only test needs no cleanup") is literally true for `db`/`rdb` (both nil in the unit harness, nothing to close) — see the separate WARNING below about a goroutine `app.routes()` (not `newApplication`) leaks in the unit harness, which is outside this requirement's literal scope since it is not something `newApplication` opened. |
| 4 | An incorrect wiring order fails to compile | **PARTIALLY SATISFIED — spec overclaims relative to delivery, and the code says so itself** | Mutation-verified, independently: moving the bookings block above the payments block yields `undefined: paymentsHandler` at `cmd/api/app.go:306` — Scenario "Swapping two ordered constructor calls" holds. But the requirement's own worked example — `bookings.Dependencies.Refunds: app.payments` — is exactly what does **not** fail to compile: I independently confirmed that changing `app.go:329` from `Refunds: paymentsHandler` to `Refunds: app.payments` compiles clean (app already exists as a zero-valued struct at `app.go:138`, so `app.payments` is a valid, nil, reference at that point in phase 2), and reintroduces the original nil-capture defect the whole change exists to remove. This is not a hidden gap: `app_test.go:149-154`'s own comment says it plainly — *"Wrong SOURCE is not [caught]: changing `bookings.Dependencies.Refunds` from the local `paymentsHandler` back to `app.payments` still compiles... which is exactly the defect this change exists to remove."* — and commit `22c5a61`'s message calls it a "Known gap, recorded rather than papered over." Scenario "Every captured field is compile-checked or checked before capture" requires each field to be *either* unrepresentable in the wrong order *or* covered by an explicit pre-capture check — `app.payments`-as-written-in-`Refunds` is **neither**: it compiles in the wrong state, and no runtime check (`validateDeps` only covers `deps.models`, `TestNewApplicationWiresEveryField` only covers top-level `*application` fields, not values nested inside a handler's `Dependencies`) catches it. Verdict: the requirement's main clause and its second scenario, read literally against their own worked example, promise more than a Go local-variable discipline can deliver without per-`NewHandler` runtime validation — which design.md's own "Alternatives rejected" section explicitly declines to add ("closing that means each `NewHandler` validating its own Dependencies, which is a change across every module"). This is a genuine, documented, and honestly-labeled gap between spec text and implementation, not a silent one. |
| 5 | Route table and authorization are unchanged | SATISFIED | `routes_authz_test.go` has a **zero-line diff** across all five commits (confirmed via `git diff 8ffa34f..7a10674 -- cmd/api/routes_authz_test.go` and `git log --oneline -- cmd/api/routes_authz_test.go`, which shows only the pre-change commit `ed76618`). `TestAPISurfaceMatchesTheInventory`, `TestEveryRouteIsGuarded`, `TestRouteAuthorizationMatrix`, `TestPublicRouteAllowlistIsCurrent` all pass, independently re-run. Slice 2's behaviour change (config-mutation-after-construction no longer working) is documented in commit messages and the prior verify-report's Finding 1, not folded silently into "just a refactor." |
| 6 | Test substitution boundary | SATISFIED (Scenario 1), **UNTESTED at the composition-root level (Scenario 2)** | Scenario 1 ("no Redis") is proven by every `cmd/api` unit test running against `newApplication` with `deps.rdb == nil`. Scenario 2 ("Redis-path test uses `miniredis`... `newApplication` wires it") has no covering test: `rg -n miniredis` across the repo shows `miniredis` used only inside `internal/notifier`, `internal/realtime`, `internal/middleware`, `internal/auth` package tests — each exercises its own Redis-backed component directly, never through `newApplication`/`deps.rdb`. No test anywhere constructs a `*redis.Client` against `miniredis` and passes it as `deps.rdb` to `newApplication`. Per the Hard Rule that a scenario is compliant only when a covering test passed at runtime, this scenario is **not covered**, and it shares its root cause with Requirement 1 Scenario 2 above (nothing exercises `newApplication` with a live/simulated Redis client at all — the integration harness passes a real `db` but always leaves `rdb` nil). |

## Additional findings the spec does not cover

- **WARNING — unit harness leaks the rate-limiter eviction goroutine.** `design.md`'s Testing Strategy states "Test harnesses add `t.Cleanup` closing `app.shutdown` once, to reap the eviction goroutine `Wrap` starts" for both harnesses. `testutils_integration_test.go:168-170` does this (`t.Cleanup(func() { close(app.shutdown) })`). `testutils_test.go` (the unit harness) does **not** — I greped for `shutdown`/`t.Cleanup` in that file and found neither. The unit harness's `config.limiter.enabled = true` (deliberately, per its own comment, to exercise the real rate-limit path), so every unit test that calls `.routes()` starts `internal/middleware/ratelimit.go:122`'s `go buckets.evict(m.shutdown)` and never stops it. This does not fail any test — there is no `goleak`/`TestMain` leak detector in `cmd/api` — and it is not a violation of Requirement 2 or 3 as literally written (the goroutine is opened by `app.routes()`, not by `newApplication`), but it is a design-documented behaviour ("both harnesses get `t.Cleanup`") that was only half-delivered. Design deviation, not a spec break — WARNING per the decision gate.
- **Everything the implementation does beyond the spec's letter**: the `TestNewApplicationWiresEveryField` reflection test (`app_test.go:126-194`) and `optionalApplicationFields` documentation map are not required by any of the 6 requirements/12 scenarios — they are a stronger, self-imposed safeguard than the spec asks for, closing (partially — see Requirement 4 above) the gap `unwiredDependencies()` left. This is a positive over-delivery, not a risk.

## Prior slice-2 catalogue, re-confirmed

The three findings `sdd-apply` recorded in the previous `verify-report.md` (now
superseded by this file) still hold and are corroborated by my own gate runs:

1. **HIGH** (now fixed, per `8b88421`, and re-confirmed green in this run):
   `TestSecurity_RateLimit_AuthEndpoints` and `TestSecurity_Cookies_SecureAttributes`
   originally mutated `app.config` after construction and could never have worked
   against the locals-then-publish design; both now use `newIntegrationApp`'s
   `opts ...func(*config)` hook applied before construction, and both pass.
2. **MEDIUM** (fixed, per `8b88421`): `TestIntegration_BlockedSlots`'s hardcoded
   date is now computed from the current month; passes.
3. **LOW/unrelated**: the flaky `internal/data.TestOnlyOneWorkerCanTakeARefundAttempt`
   under full concurrent `./...` — not reproduced in either of my two full
   `-race -count=1 ./...` runs (`make test` and the standalone race run) or in
   `make test/integration`. Not re-flagged; still worth a separate ticket if it
   recurs, out of this change's scope either way.

The audit data-race fix (`d910dc9`) is out of this spec's scope (it is not an
`application-composition-root` requirement) but is real and verified: `-race`
is clean on `internal/audit`, `internal/courts`, and `internal/data` in both
full-suite runs above, and the fix is structural (encode-before-goroutine), not
a race-window narrowing.

## Recommendation

**Archive**, with the `state.yaml` `apply.status` staleness corrected first
(trivial, non-blocking edit) and Requirement 4's gap and Requirement 1/6's
untested Redis-through-`newApplication` scenario carried forward — either as a
named, accepted spec/implementation gap in the archived spec, or as a small
follow-up change (per-`NewHandler` `Dependencies` validation; one test that
passes a `miniredis`-backed `deps.rdb` through `newApplication`). Nothing found
is a functional regression: every gate is green, the route table and its
authorization are byte-for-byte unchanged, and the harnesses provably cannot
drift from `main()` anymore for the class of defect (`undefined: x`) the
locals-then-publish design targets. What is not delivered is a documented,
named subset of "wrong order" (wrong *source*, i.e. referencing `app.x`
instead of the local) and one untested composition path (Redis through
`newApplication`) — both narrower than the CRITICAL bar, both already
acknowledged in the codebase's own comments rather than hidden.
