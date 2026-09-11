# Proposal: Refund Intent Durability and Collector Integrity

## Intent

Three defects on the refund path, none of which loses money, all of which end
with a booking nothing will ever fix and an operator pointed at the wrong thing.

**1. `getSellerToken` cannot say why it failed.** It returns `""` for all three
distinct arms (`internal/payments/refund.go:441-461`) even though the data layer
already distinguishes them (`internal/data/mpcred.go:81-89`, sentinels at `:61`
and `:68`). All three call sites then present MercadoPago a refund on the
platform's token (`refund.go:273`, `refund.go:505`, `process.go:313`).

> **Premise, not fact.** MercadoPago rejects that call because the platform is
> not the collector of a seller-scoped payment. Nobody in this chain has had
> network access to MercadoPago; this is inferred from its marketplace
> authorization model. **Design MUST confirm it.** If it is false, the same
> sites are issuing refunds from the platform's own account — which the change
> below closes either way, so the direction is premise-independent; only the
> severity narrative changes.

The rejection reads as an ordinary provider error, so `transientProviderFailure`
(`internal/data/failed_refunds.go:104-112`) does not match it and each attempt
spends a retry: **~5h20m of guaranteed-doomed retries** before `recordRefundFailure`
alerts (`refund.go:409`). For UNREADABLE and MISSING, no retry can connect an
account or decrypt a key.

**2. A cancelled booking can owe a refund nothing knows about.** Both cancel
paths commit `status='cancelled'` (`internal/bookings/actions.go:77,86`;
`internal/bookings/public.go:546,556`) and only afterwards call
`AutoRefundIfPaid` (`:105`, `:581`), inside which `ClaimRefund` writes the first
durable record. A crash between the two leaves a paid, cancelled booking with no
attempt row. All thirteen cron jobs were read (`cmd/api/cron.go:42-60`); none
looks for it.

**3. The collector check is bypassed for cancelled bookings.**
`process.go:46-62` returns `refundCancelledBookingPayment` before the amount
check (`:66`) and before `collectorMatchesComplex` (`:92`). An attacker's own
preference, carrying a victim's cancelled booking id, writes a fictitious
payment row from attacker-controlled values, drives the booking to
`payment_status='refund_pending'` (`:354`), and — the mismatch being permanent
rather than transient — leaves it there forever, firing `REFUND EXHAUSTED` /
`MANUAL REFUND OWED` at a venue that received nothing. **Not theft; a durable
integrity defect with no recovery path.** Client cancellations and the expiry
cron supply the cancelled booking ids routinely.

## Scope

### In Scope

- **A typed seller-token accessor, per arm.** UNAVAILABLE (`GetByID` failed —
  transient, may self-heal) keeps the existing queue-and-retry treatment.
  UNREADABLE and MISSING refuse before the provider is called and alert
  immediately with a credential-specific message. All three route through the
  existing `recordRefundFailure` → `transientProviderFailure` seam; **no second
  classification rule is introduced.** All three sites refuse *after* their
  claim is committed (`refund.go:247`, `process.go:298`, and the retry loop's
  existing row), so a refusal is queued, never manual.
- **A durable refund-intent marker written in the cancel transaction**, plus a
  reconciliation sweep that works markers with no covering attempt. See Approach
  for why the marker, not the sweep, is the load-bearing half.
- **Collector verification before any branch of `processApprovedPayment` writes
  a payment row or moves money**, covering the cancelled branch.

### Out of Scope

- **Merging the cancel write and `ClaimRefund` into one transaction.**
  `ClaimRefund` owns its transaction (`internal/data/refunds.go:65-118`);
  merging inverts the store layer across `bookings` → `payments` → `data` and
  puts a money transaction in the HTTP request path. It also does not remove the
  window — the provider call still follows the commit. Rejected on blast radius.
- **Hoisting the amount check (`process.go:66-81`) with the collector check.**
  The cancelled branch records `mpPayment.TransactionAmount` deliberately
  (`process.go:276-283`): a deposit that moved since checkout would fail the
  expected-amount comparison and a legitimate refund would be refused. Only the
  collector check moves.
- Reconciling the refunded amount or fee against MercadoPago's response
  (`refund-amount-and-fee-truth`), refund-window rules (change 8), notification
  consolidation (change 9).
- Backfills. Nothing is deployed; there is no orphan data to reconcile.

## Capabilities

### New Capabilities
- `refund-intent-durability`: when a cancellation owes a refund, what durably
  records that fact, and what finds a refund whose claim never committed.
- `payment-collector-verification`: no webhook-driven branch may write a payment
  row or move money for a payment whose collector is unproven.

### Modified Capabilities
- `seller-credential-integrity`: the requirement at
  `openspec/specs/seller-credential-integrity/spec.md:164-187` ratified today's
  loud-alert-**and-continue** trade at `getSellerToken`, reasoning that
  "refusing outright would turn a recoverable refund into a manual one". That
  reasoning is wrong at all three sites, because each refuses after its claim is
  already committed. This change replaces continue-anyway with per-arm refusal.

## Approach

**The sweep alone is unsound, and this is the change's real finding.** An
orphaned refund and a deliberately-declined one are the same row: `status='cancelled'`,
`payment_status='deposit_paid'`, no attempt row. `public.go:583-587` produces the
second on every out-of-window client cancellation, while `actions.go:96-99`
refunds staff cancellations regardless of window — so no sweep can reconstruct
which decision was made without re-deciding policy from a row that does not
record it. A sweep that guessed would refund every late cancellation the venue is
entitled to keep.

So the marker is primary and the sweep is its consumer. The marker MUST commit in
the same statement as the cancel: `BookingModel.Update` already writes
`payment_status` (`internal/data/bookings.go:331-343`), so one added column is
carried by the existing optimistic-concurrency UPDATE at zero extra round trips.
It MUST NOT be expressed as `payment_status='refund_pending'` — `refundable()`
(`refund.go:320-323`) reads that as "a claim already exists" and would decline the
very refund the marker exists to guarantee.

The sweep follows the two jobs already in the registry and needs the conditional-UPDATE
idempotency guard from `MarkProcessing` (`failed_refunds.go:205-223`) so two
instances cannot claim one orphan. **State it plainly: the sweep bounds how long
an orphan is invisible; it does not close the crash window.** Atomicity would
require the transaction merge rejected above.

Migrations top out at `009` and nothing is deployed, so the column is free now
and stops being free on first deploy.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/payments/refund.go` | Modified | `:441-461` typed accessor; `:273`, `:505` refusal paths; `:382-421` per-arm classification |
| `internal/data/failed_refunds.go` | Modified | `:75-112` extend the existing classifier; no new rule |
| `internal/payments/process.go` | Modified | `:33-62` collector verification ahead of every writing branch; `:313` refusal path |
| `internal/bookings/{actions,public}.go` | Modified | `:77-86` / `:546-556` set the intent marker in the cancel write |
| `internal/data/bookings.go`, `db/queries/bookings.sql` | Modified | `Update` carries the marker; new orphan-selection query |
| `db/migrations/010_*.sql` | New | Intent-marker column |
| `internal/payments/`, `cmd/api/cron.go` | New / Modified | Sweep job and its registration |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| MercadoPago accepts the platform-token refund (premise false) | Med | Design's first task is confirming it. Fail-closed is correct under both branches; only the framing changes |
| The sweep refunds a correctly-declined late cancellation | High if untreated | Sweep selects on the marker only, never on inferred state; a mutation-verified test must prove an out-of-window public cancellation is never swept |
| Two instances claim one orphan | Med | `MarkProcessing`'s conditional-UPDATE idiom, reused not reinvented |
| Refusing UNAVAILABLE regresses a self-healing case | Med | Per-arm, not uniform: UNAVAILABLE stays transient under the existing classifier |
| Collector check runs twice via the recursion at `process.go:119-125` | Low | Read-only and idempotent; design may hoist above `:35` so it runs once |
| `make sqlc` emits the known stray `WebhookEvent` diff | Med | Isolate the regeneration in its own commit, as `mp-oauth-credential-integrity` did |
| Three unrelated concerns in one review | High | Three chained slices; see Review Workload |

## Rollback Plan

Each slice reverts independently, in any order. Slice 1 (collector) and slice 2
(seller token) are pure code reverts touching no data. Slice 3 reverts by
reverting the code and running migration `010` down; the dropped column carries
no data any other path reads. Nothing is deployed, so no rollback has a
live-traffic window.

## Dependencies

- `mp-oauth-credential-integrity` (archived) — supplies the typed credential
  arms this change routes on. Satisfied.
- No network access to MercadoPago in this environment. The rejection premise
  must be confirmed by someone who has it, or by a sandbox credential.

## Review Workload

Base 400, doubled to 800 by the 2x modifier (mutation-verified tests). Estimated
~560 authored lines across three slices, each independently deliverable:

1. **Collector verification** (~85) — smallest, highest severity, no schema.
2. **Per-arm seller token** (~140) — depends on nothing; reverses one archived
   requirement.
3. **Intent marker and sweep** (~335) — carries the migration and the sqlc
   regeneration.

`Decision needed before apply: No` · `Chained PRs recommended: Yes` ·
`400-line budget risk: High` (against the doubled 800: Medium)

## Success Criteria

- [ ] A UNREADABLE or MISSING credential alerts on the first attempt, with a
      credential-specific message, and never reaches MercadoPago.
- [ ] A UNAVAILABLE credential still queues and retries, and does not spend a
      retry — proven against `transientProviderFailure`'s existing rule.
- [ ] A crash between the cancel commit and `ClaimRefund` leaves an orphan the
      sweep finds and claims exactly once, proven with two concurrent sweepers.
- [ ] An out-of-window public cancellation is never swept, and neither is a cash
      booking with no MercadoPago id.
- [ ] A webhook payment collected by a foreign account writes no payment row and
      moves no booking to `refund_pending`, for a cancelled booking as well as a
      pending one.
- [ ] `make test`, `golangci-lint run`, and the `integration`-tagged suite pass.
