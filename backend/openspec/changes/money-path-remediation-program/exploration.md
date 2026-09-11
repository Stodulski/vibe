# Exploration — 108-finding remediation program

> Produced by `sdd-explore`. The phase agent had no filesystem write capability, so it
> persisted to Engram (`sdd/money-path-remediation-program/explore`, id 21) and the
> orchestrator materialized it here. Content is the phase's, unmodified in substance.

## Input

Two adversarial review rounds (13 reviewers) over `padel-server` produced **108 open
findings**, verified in source. This exploration decides how that remediation is cut into
deliverable SDD changes. It fixes nothing.

## Grounding

Read via `codegraph_explore` (verbatim source): `internal/payments/{refund,webhook,payments}.go`,
`internal/data/{refunds,failed_refunds,payments,models,bookings,slot_locks}.go`,
`internal/bookings/{bookings,public}.go`, `internal/middleware/{chain,middleware}.go`,
`internal/circuitbreaker/circuitbreaker.go`, `internal/complexes/complexes.go`,
`internal/mp/mp.go`, `internal/reporting/{reporting,monthly}.go`,
`internal/notifications/{tasks,notifications}.go`, `internal/notifier/notifier.go`,
`internal/mailer/mailer.go`, `internal/scheduler/scheduler.go`,
`cmd/api/{main,server,routes,adapters}.go`.

Confirmed in source, not assumed:

- `main()` is ~540 lines of flat wiring with no `newApplication()` extraction — finding 93 is
  real and gates every full-HTTP-route test.
- The `RequireAuth(RequireComplexOwner(...))` closure is duplicated verbatim across
  `bookings.go`, `complexes.go` and `reporting.go` — finding 30.
- `rateLimitRedis` and `rateLimitInMemory` duplicate the same `/api/v1/webhooks/` prefix
  exemption — findings 26 and 27.
- `AutoRefundIfPaid` already uses a real `ClaimRefund` plus `context.WithoutCancel`, so finding
  4's "no durability before the claim" refers to the **cancellation write**, not to
  `ClaimRefund` internals.

## Decomposition — 37 changes

Budget is the base line budget. **2x** marks changes that qualify for the doubling modifier:
they ship a test shown to go red when the fix is reverted.

### Phase 0 — prerequisite

| # | Change | Findings | Budget | 2x | Intent |
|---|---|---|---|---|---|
| 1 | `cmd-api-new-application-extraction` | 93 | 800 | no | Extract `newApplication(cfg) (*application, error)` out of `main()` so tests can construct a real app and router without running the binary. `main()` becomes flag-parse, call, `os.Exit`. |

### Phase 1 — money path (base 400)

| # | Change | Findings | Budget | 2x | Intent |
|---|---|---|---|---|---|
| 2 | `mp-oauth-credential-integrity` | 54, 55, 56, 98 | 400 | 800 | One validated write path for MercadoPago OAuth credentials: reject a literal `"0"` user id, surface refresh-persist errors, add the active-booking guard to Connect that Disconnect already has, encrypt tokens at rest. |
| 3 | `circuit-breaker-correctness` | 73, 84, 92 | 400 | 800 | Fix the half-open state machine (a stale in-flight success closing it, `HalfOpenMaxReqs + 1` admission) and isolate the bulk token-refresh cron so it cannot wedge the shared breaker that gates real payments. |
| 4 | `webhook-idempotency-and-durability` | 8, 10, 11, 103 | 400 | 800 | A read failure returns a retryable status, not 200; confirmation is idempotent against redelivery so no orphaned second payment row; `GetPaymentByMPID` becomes deterministic; the signature trust boundary is enforced consistently. |
| 5 | `refund-claim-atomicity` | 5, 6, 7, 13, 14, 43, 107 | 400 | 800 | Every refund state transition becomes an atomic claim with a status predicate, so `refunded` cannot regress to `refund_pending`, a forged claim cannot be recorded, `exhausted` gets a recoverable definition, and the reclaim sweep can drain its batch. |
| 6 | `refund-durability-and-collector-integrity` | 1, 3, 4, 9 | 400 | 800 | Durably record refund intent before the cancel write commits, and make `getSellerToken` fail closed instead of silently using the platform token. Depends on change 2. |
| 7 | `refund-amount-and-fee-truth` | 2, 86 | 400 | 800 | Stop recomputing the refund amount and the fee locally; reconcile against MercadoPago's own response as the source of truth in both the refund and the confirm flow. |
| 8 | `refund-window-rule-correctness` | 44, 53, 91 | 400 | 800 | Fix the swallowed preference-expiry branch, add retry and alerting on the request paths, and bring `WithinStandardWindow` from 0% to full table-driven coverage. |
| 9 | `refund-notification-consistency` | 46, 47, 48, 49 | 400 | 800 | One "on terminal refund outcome, notify exactly once and transition state" path shared by the webhook, the auto-refund and the retry sweep. |
| 10 | `cancel-communication-accuracy` | 45, 51 | 400 | no | Cancellation copy must reflect real state: no refund promised for cash payments, and the configured grace period instead of a hardcoded fifteen minutes. |
| 11 | `price-band-resolution` | 37, 58 | 400 | 800 | One resolution function shared by availability and booking, so the peak band stops losing to the base band and the price shown is the price charged. |
| 12 | `slot-lock-lifecycle` | 34, 35, 42 | 400 | 800 | `AcquireLock` respects `expires_at`; locks are released on every exit path; failure cleanup runs on a context that the client cannot cancel. |

### Phase 2 — authorization (base 400)

| # | Change | Findings | Budget | 2x | Intent |
|---|---|---|---|---|---|
| 13 | `authorization-route-test-coverage` | 15, 32, 75 | 400 | 800 | The route-level authorization matrix test that catches the eight surviving mutations. Depends on change 1. |
| 14 | `tenant-isolation-integrity` | 83, 101 | 400 | no | Schema-level constraints tying a booking's client and court to its complex, plus the by-id query audit. Blast radius likely underestimated — flagged for slicing. |
| 15 | `client-ip-trust-boundary` | 17, 89 | 400 | 800 | One `ClientIP()` trust policy used by both rate limiting and audit, so the header cannot bypass the limiter, lock out a shared IP, or forge the trail. |
| 16 | `exemption-list-exact-matching` | 26, 27 | 400 | no | Replace the duplicated prefix checks and the CSRF prefix exemption with one exact, route-attribute-driven list. |
| 17 | `rate-limiter-consistency-and-deadlines` | 19, 20, 21, 22 | 400 | 800 | Fix the data race (provable under `-race`), reconcile the two implementations' limit math, and give both the limiter's Redis call and the auth database read explicit deadlines. |
| 18 | `booking-link-credential-hardening` | 100 | 400 | no | **Needs a product decision before design**: should booking links rotate or expire, or is bearer-by-UUID accepted? Flagged, not pre-solved. |
| 19 | `audit-trail-completeness` | 52, 78 | 400 | no | Public cancel writes an audit entry; tenants can read their own trail; admin reads are themselves audited. |
| 20 | `sse-authorization-staleness` | 76 | 400 | no | Re-validate authorization on a long-lived stream, and give it a cap and a deadline. |

### Phase 3 — correctness-critical, self-contained (base 800)

| # | Change | Findings | Budget | 2x | Intent |
|---|---|---|---|---|---|
| 21 | `panic-recovery-hardening` | 18, 29 | 800 | 1600 | Preserve `request_id` on the recovered path; stop swallowing `ErrAbortHandler` and corrupting an already-written response. |
| 22 | `user-cache-integrity` | 16, 23, 25 | 800 | 1600 | One error policy for the Redis user cache: reads stop failing silently, invalidation stops being fire-and-forget, password change degrades instead of 500ing. |
| 23 | `overnight-schedule-math` | 36, 57, 79 | 800 | 1600 | Three reviewers hit the same midnight-crossing math in three call sites. One shared, tested helper replaces three divergent implementations. |
| 24 | `booking-status-filter-consistency` | 38, 81 | 800 | 1600 | One canonical "countable booking" predicate reused by availability, the heatmap and the dashboard, instead of three filters that disagree on `no_show`. |
| 25 | `booking-time-grid-validation` | 39, 41, 95 | 800 | no | Confirmation consults `blocked_slots`; creation rejects off-grid start times; the grid interval stops being hardcoded in three places. |
| 26 | `time-format-validation-fix` | 88 | 800 | 1600 | `ValidFormat` stops accepting non-`HH:MM` input and silently reinterpreting it. **Prerequisite for sweep item 61.** |
| 27 | `reminder-timezone-math-fix` | 67 | 800 | 1600 | The two-hour reminder filters by five hours. Needs a fixture-seeded integration test against real Postgres, not a reading. |
| 28 | `notification-delivery-durability` | 65, 66, 68, 69, 70, 71, 96 | 800 | 1600 | The pipeline drops messages in six ways. Apply the durable claim/retry idiom the codebase already uses for `webhook_events` and `failed_refunds`. |
| 29 | `presign-content-type-enforcement` | 62 | 800 | no | The presign validates content type and then discards it. |
| 30 | `places-proxy-status-and-key-leak` | 63 | 800 | no | Honour the upstream status; stop leaking the API key into logs. |
| 31 | `xlsx-export-robustness` | 82 | 800 | no | Bound and stream the export; fail before committing the 200. |
| 32 | `db-connection-hygiene` | 99, 105 | 800 | 1600 | Retry transient database errors, and stop holding a pooled connection across an external call. Same discipline, one pass. |
| 33 | `security-test-wiring` | 94 | 800 | no | `make test/security` has never run. Wire it and fix what it surfaces — unknown blast radius. Depends on change 1. |

### Phase 4 — lower consequence (base 1500)

| # | Change | Findings | Budget | 2x | Intent |
|---|---|---|---|---|---|
| 34 | `slug-consistency` | 59 | 1500 | no | A soft-deleted slug reports free then 500s. |
| 35 | `db-pool-boot-order` | 72 | 1500 | no | Twenty of twenty-five pool connections are consumed before the listener opens. |
| 36 | `operational-observability` | 28, 85, 108 | 1500 | no | Request logging and metrics beyond dev-only expvar, a healthcheck that sees the payment stack, and aggregate queue visibility. |

### Phase 5 — the sweep

| # | Change | Findings | Budget | 2x |
|---|---|---|---|---|
| 37 | `trivially-verifiable-sweep` | 12, 24, 30, 31, 33, 40, 50, 60, 61, 64, 74, 77, 80, 87, 90, 97, 102, 104, 106 | 2000 | no, by definition |

One deliberately boring change. Membership test: **a reviewer can judge the line correct or
incorrect at a glance, with no context.**

Deliberately excluded even where individually small: anything touching a payment transaction or
an authorization decision. Money and authorization get dedicated review attention rather than
being buried in a nineteen-item batch.

## Root-cause collapse

**108 findings → 37 changes (≈2.9:1).** Within the collapsed set, **78 findings become 25
multi-member changes (≈3.1:1)** because they share an actual mechanism — the same function,
table or code path — not a theme. The remaining 30 are genuinely atomic: 11 stand alone and 19
land in the sweep.

Largest collapses:

| Site | Findings → change |
|---|---|
| `internal/data/{refunds,failed_refunds}.go` claim state machine | 7 → 1 |
| `internal/notifier` + `internal/notifications` + reminder marking | 7 → 1 |
| `internal/middleware/chain.go` limiter (both impls) + auth read | 4 → 1 |
| `internal/complexes` OAuth connect/disconnect/refresh + storage | 4 → 1 |
| `internal/payments/webhook.go` + payment lookup | 4 → 1 |
| Midnight math across three call sites | 3 → 1 |
| Circuit breaker state machine + blast radius | 3 → 1 |

## Dependency order

1. **`cmd-api-new-application-extraction` (93) gates only two changes** — 13 and 33 — which need
   a real constructed router. Every other change's evidence is package-level and does not need
   it. This is narrower than assumed going in.
2. **`time-format-validation-fix` (26) must land before sweep item 61.** 61 wires the validator
   into `UpdatePrices`; doing that first would wire in a known-broken check.
3. **`mp-oauth-credential-integrity` (2) before `refund-durability-and-collector-integrity` (6).**
   A fail-closed `getSellerToken` only means something once token persistence is trustworthy.
4. **`circuit-breaker-correctness` (3) early in Phase 1** — the breaker gates preference
   creation, payment lookup, refunds and OAuth refresh.

No other hard edges. Remaining order is priority, not dependency.

## Non-goals

- No message-broker rewrite for notifications. Extend the existing durable-claim idiom.
- No KMS or secrets-manager integration. Application-level encryption closes finding 98.
- No audit-log UI or export feature.
- No general performance pass beyond the measured regressions.
- No rate-limiter redesign — reconcile the two implementations, do not replace them.
- No composition-root refactor beyond the specific extraction finding 93 asks for.
- No coverage-percentage target.
- Finding 100's product question is flagged, not decided.

## Risks in this decomposition

- **Three clusters likely exceed their budget as single PRs**: `refund-claim-atomicity` (7
  findings), `notification-delivery-durability` (7), `tenant-isolation-integrity` (unknown
  surface). `sdd-tasks` must slice inside the change rather than treat one change as one PR.
- **The extraction is a large mechanical diff** with real behaviour-change risk hiding inside a
  "just a refactor" description.
- **Sweep item 61 cannot ship before change 26.** If the sweep is wanted first for momentum, 61
  must be pulled out.
- **Some groupings are inferences** from finding text plus structure, not confirmed against each
  reviewer's exact annotated line — notably 1+9 and 45+51. `sdd-propose`/`sdd-spec` must
  re-verify before locking scope.
- **`tenant-isolation-integrity`'s blast radius is probably underestimated** at a 400-line budget.
- **Finding 97 touches sqlc-generated code**, so its evidence must include a `sqlc generate` run.
- **"Not in production" is load-bearing** for the cheap schema changes. If traffic starts
  mid-program, those items need re-scoping as online migrations.
