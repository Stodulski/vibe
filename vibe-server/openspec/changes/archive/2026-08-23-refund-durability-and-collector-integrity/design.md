# Design: Refund Intent Durability and Collector Integrity

## Technical Approach

Three slices in the proposal's order — collector (~85), seller token (~140), marker
and sweep (~335). The order is kept and is load-bearing: slice 3's sweep calls
`AutoRefundIfPaid`, so landing it before slice 2 would drive the platform-token
refund at machine speed instead of once per cancellation.

Nothing new is invented. Each slice reuses the idiom the codebase already has for
that job: `collectorMatchesComplex` unchanged, only relocated; the credential
sentinels already in `internal/data/mpcred.go`; `MarkProcessing`'s conditional
UPDATE; migration 006's prove-then-constrain shape.

## Architecture Decisions

### Decision: the collector check moves to `process.go:44`, not above `:35`

**Choice**: `collectorMatchesComplex` is hoisted out of `:92` to sit **after** the
already-confirmed/completed/no_show skip (`:35-42`) and **before** the cancelled
branch (`:46`). The recursion at `:119-125` is replaced by a direct
`h.refundCancelledBookingPayment(...)` call, which is exactly what re-entering
reaches. Together those make the check run **once** per webhook.

**Rejected — hoisting above `:35`** (the proposal's parenthetical): `:35` writes
nothing and moves no money, so it needs no verification, and hoisting adds a
`complexes` read to the hottest duplicate-webhook path — where a database blip
would return an error and requeue an event that had nothing to do.

**Rejected — leaving the recursion**: the check would run twice. It is read-only
and idempotent, so this is cosmetic, but the direct call also removes a recursion
from a money function and one misleading duplicate FRAUD-ALERT log line.

`refundCancelledBookingPayment` has exactly two callers, `:61` and `:264`
(`refundBookingWhoseSlotIsGone`), both inside `processApprovedPayment` and both
below the new position, so one check dominates every writing branch.

> **The amount check (`:66-81`) does NOT move.** `process.go:276-283` records
> MercadoPago's `TransactionAmount` on purpose; a deposit that changed since
> checkout would fail the expected-amount comparison and a legitimate refund would
> be refused. Apply must add a comment at `:66` naming this, because an implementer
> reading "verify before the branch writes" will plausibly move both.

### Decision: `getSellerToken` returns `(string, error)`, carrying the arm unchanged

**Choice**: rename to `sellerCredential(ctx, complexID) (string, error)` and return
the arm the data layer already produced — a wrapped `GetByID` failure, or
`data.ErrMPNotConnected`, or `data.ErrMPCredentialUnreadable`. The provider is not
called when the error is non-nil. All three call sites become the same two lines,
each already holding a committed claim, so every refusal is queued and never manual:

```go
token, err := h.sellerCredential(ctx, booking.ComplexID)
if err != nil {
    return h.refuseForCredential(ctx, *claim, err) // refund.go:273
}
```

`refuseForCredential` alerts with a credential-specific Sentry message **on the
first attempt** and then hands a cause string to the existing `recordRefundFailure`.
Classification stays in the one classifier: `providerOutageMarkers`
(`failed_refunds.go:75-85`) gains **one** entry, `"seller credential unavailable"`,
which only the transient arm's cause embeds.

| Arm | Cause handed to `recordRefundFailure` | `transientProviderFailure` | Effect |
|---|---|---|---|
| `GetByID` failed | `seller credential unavailable: …` | true | no retry spent, flat 5-min probe — the arm that self-heals |
| `ErrMPCredentialUnreadable` | `seller credential unreadable for complex …` | false | spends the budget, alerts each attempt, ends in REFUND EXHAUSTED |
| `ErrMPNotConnected` | `seller credential missing for complex …` | false | same |

**Deliberate**: MISSING and UNREADABLE still spend the retry budget. Retrying is
only waste when it costs a provider call, and this path now refuses before the
provider — so a credential re-linked inside 5h20m finishes the refund by itself,
and the operator was told at t=0 either way. An immediate-exhaustion arm would need
a third classification the codebase does not have.

**Rejected**: a `sellerTokenResult` enum type (duplicates sentinels that exist);
reclassify-at-record-time (proposal: the only option whose safety depends on the
unconfirmed premise); an `mp.Caller` type (deferred by the archived
`mp-oauth-credential-integrity` design; still out of scope).

**Compatibility**: `mp.RefundPayment`'s own `""` fallback stays, so the archived
asymmetry test at `internal/mp` remains green. What changes is that
`internal/payments` never passes `""` again.

### Decision: `bookings.refund_intent_at TIMESTAMPTZ NULL` — marker and lease in one column

**Choice**: one nullable timestamp, written by the same statement that cancels.

| Who | Does what |
|---|---|
| `actions.go` `Cancel`, `public.go` `PublicCancel` | set it from `owesRefund`, **before** `h.store.Update` — one added column on the existing optimistic-concurrency UPDATE, zero extra round trips |
| `ClaimRefund` (`refunds.go`) | clears it inside the claim transaction — the `failed_refunds` row is now the durable record |
| `AutoRefundIfPaid` | clears it on every exit that commits **no** claim, after `owedManually` has alerted |
| the sweep | clears nothing; it calls `AutoRefundIfPaid`, so the lifecycle is identical whether the call came from a request or from a sweep |

The row, staff-cancelling a paid booking:

| Step | `status` | `payment_status` | `refund_intent_at` | `failed_refunds` |
|---|---|---|---|---|
| before | confirmed | deposit_paid | NULL | — |
| cancel UPDATE commits | cancelled | deposit_paid | `10:00:00Z` | — |
| **crash here → this is the orphan** | cancelled | deposit_paid | `10:00:00Z` | — |
| `ClaimRefund` commits | cancelled | refund_pending | NULL | pending |
| `RecordRefundSuccess` | cancelled | refunded | NULL | resolved |
| **out-of-window public cancel** | cancelled | deposit_paid | **NULL** | — |

The last two data rows differ in exactly one column, and it is the only column the
sweep reads.

**Not `payment_status='refund_pending'`** — `refundable()` (`refund.go:320-323`)
reads that as an existing claim and returns `RefundQueued`, so the marker would make
`AutoRefundIfPaid` decline the refund it exists to guarantee. **Not a second
`refund_intent_claimed_at` column** — the timestamp doubles as the lease (below).
**Not merging cancel + `ClaimRefund` into one transaction** — rejected in the
proposal on blast radius, and it does not remove the window anyway.

**Stale-write hazard, named**: `ClaimRefund` clears the column in the database but
cannot reach the handler's in-memory `*Booking`. After `AutoRefundIfPaid` returns,
both handlers set `booking.RefundIntentAt = nil`, in the same place and idiom they
already use for `booking.PaymentStatus = paymentStatusAfter(...)`.

### Decision: the sweep selects on the marker and nothing else

Three hand-written statements on `BookingModel`, in a new
`internal/data/refund_intents.go` (precedent: `refunds.go` holds `PaymentModel`'s
refund lifecycle apart from `payments.go`):

```sql
-- select: names one column. status, payment_status and payments are absent.
SELECT … FROM bookings
 WHERE refund_intent_at IS NOT NULL AND refund_intent_at < NOW() - $1::interval
 ORDER BY refund_intent_at ASC LIMIT $2;

-- claim: compare-and-swap on the exact value read; MarkProcessing's idiom.
UPDATE bookings SET refund_intent_at = NOW() WHERE id = $1 AND refund_intent_at = $2;
--   RowsAffected() != 1 -> ErrRecordNotFound: another instance took it.

-- clear
UPDATE bookings SET refund_intent_at = NULL WHERE id = $1;
```

**Why a declined refund is unrepresentable rather than unlikely.** The predicate
mentions one column. No value of `status`, `payment_status`, `deposit_amount` or any
`payments` row can bring a row into the result set, so the sweep cannot re-decide
policy from a row that does not record it. And the marker is written from the *same*
boolean that decides whether `AutoRefundIfPaid` is called — `owesRefund` — so the
`default:` branch that answers `RefundNotEligible` at `public.go:583-587` is
literally `!owesRefund` and cannot set the marker without the same edit also
refunding. One expression, not two.

The claim UPDATE bumps no `version` and does not fire migration 006's reversal
trigger (its `WHEN` names `status`/`payment_status` only), so the sweep cannot lose
a version race with a concurrent booking edit. Setting the column to `NOW()` re-leases
it for another grace period, exactly as `MarkProcessing` re-stamps `updated_at`.

Constants mirror the sibling queue: `refundIntentGrace = 5 * time.Minute` (must
outlast one honest in-request refund — the MercadoPago call is capped at 8s),
`refundIntentBatch = 12`, job every 2 minutes like `retry_refunds` and
`sweep_webhook_events`.

**The sweep bounds how long an orphan is invisible; it does not close the crash
window.** Atomicity needs the transaction merge the proposal rejected.

### Decision: slice order confirmed

1 and 2 are pure code and mutually independent. 3 carries migration 010 and the
sqlc regeneration, and depends on 2 for the reason above.

## Data Flow

```
Cancel / PublicCancel
   owesRefund ──┬── true  ─→ booking.RefundIntentAt = now
                └── false ─→ nil                (RefundNotEligible: never swept)
        │
   store.Update  ── one UPDATE: status='cancelled' + payment_status + marker
        │
        ├─ crash ────────────→ marker survives ─→ sweep (≥5 min) ─→ CAS claim ─┐
        │                                                                       │
   AutoRefundIfPaid ←──────────────────────────────────────────────────────────┘
        ├─ ClaimRefund commits ─→ marker cleared in the same tx ─→ RefundPayment
        └─ no claim (cash, no MP id, already refunded) ─→ alert, then clear marker

processApprovedPayment
   :35 skip (no write) → COLLECTOR CHECK → :46 cancelled branch → amount check (stays)
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/payments/process.go` | Modify | Slice 1: collector check to `:44`, removed from `:92`; recursion `:119-125` → direct `refundCancelledBookingPayment`; comment at `:66` pinning the amount check. Slice 2: `:313` refusal |
| `internal/payments/refund.go` | Modify | Slice 2: `:441-461` → `sellerCredential`; new `refuseForCredential`; `:273`, `:505` refusal paths |
| `internal/data/failed_refunds.go` | Modify | Slice 2: one entry in `providerOutageMarkers` + the exported cause prefix |
| `internal/bookings/actions.go`, `public.go` | Modify | Slice 3: `owesRefund` computed before the cancel `Update`; marker set; `RefundIntentAt = nil` after the outcome |
| `internal/data/refund_intents.go` | Create | Slice 3: `GetRefundIntentOrphans`, `ClaimRefundIntent`, `ClearRefundIntent` |
| `internal/data/bookings.go` | Modify | `Booking.RefundIntentAt *time.Time \`json:"-"\``; `bookingFromDB`; `Update` carries the column |
| `internal/data/refunds.go` | Modify | Clear the marker inside `ClaimRefund`'s tx; `cancelRefundedBooking` writes `locked.RefundIntentAt` back unchanged |
| `internal/data/payments.go` | Modify | `:199`, `:257` carry `b.RefundIntentAt` through `UpdateBookingParams` |
| `internal/data/convert.go` | Modify | `timePtrToPg` / `pgToTimePtr`, the `textToPg`/`pgToTextPtr` pair's shape |
| `internal/data/models.go` | Modify | New `BookingRefundIntentManager`, composed into `BookingStore` |
| `internal/payments/payments.go` | Modify | New `RefundIntentStore` (3 methods) + `SweepOrphanedRefundIntents`; **not** added to `BookingStore`, which stays 2 methods |
| `db/queries/bookings.sql` | Modify | `UpdateBooking` sets `refund_intent_at` |
| `db/migrations/010_refund_intent_marker.sql` | Create | Column, partial index, CHECK |
| `internal/db/` | Regenerate | `make sqlc`, isolated commit |
| `cmd/api/cron.go` | Modify | `job("sweep_refund_intents", 2*time.Minute, app.payments.SweepOrphanedRefundIntents)` |

## Interfaces / Contracts

```go
// internal/data — new sub-interface, composed into BookingStore (ISP: the queue
// methods do not belong in BookingUpdater or BookingLifecycleManager).
type BookingRefundIntentManager interface {
    GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*Booking, error)
    ClaimRefundIntent(ctx context.Context, id uuid.UUID, seen time.Time) error // ErrRecordNotFound = lost the race
    ClearRefundIntent(ctx context.Context, id uuid.UUID) error
}

// internal/payments — consumer-side, separate from BookingStore{GetByID,Update}.
type RefundIntentStore interface { /* the same three */ }

func (h *Handler) sellerCredential(ctx context.Context, complexID uuid.UUID) (string, error)
func (h *Handler) SweepOrphanedRefundIntents(ctx context.Context)
```

```sql
-- db/migrations/010_refund_intent_marker.sql (+goose Up)
ALTER TABLE bookings ADD COLUMN refund_intent_at TIMESTAMPTZ;
-- Every row is NULL except an unclaimed orphan, so the sweep must not scan the table.
CREATE INDEX idx_bookings_refund_intent ON bookings (refund_intent_at)
    WHERE refund_intent_at IS NOT NULL;
-- An intent marker on a live booking is not a state this product has; migration
-- 006's floor-not-policy convention. Terminal->live is already forbidden by the
-- 006 trigger, so no path can strand a marker on a resurrected row.
ALTER TABLE bookings ADD CONSTRAINT bookings_refund_intent_only_when_cancelled
    CHECK (refund_intent_at IS NULL OR status = 'cancelled');
```

## Testing Strategy

`rules.evidence: mutation-verified` — each row names the mutation that must break it.

| Layer | What | Mutation that must fail |
|---|---|---|
| Unit `bookings` | Out-of-window `PublicCancel` on a paid booking: the stub's `Update` receives `RefundIntentAt == nil` | set the marker unconditionally |
| Unit `bookings` | Staff `Cancel` out of window: marker **is** set (staff refund ignores the window) | apply the window to the staff path |
| Unit `payments` | UNREADABLE and MISSING: `RefundPayment` is never called; Sentry fires on attempt 1; cause reaches `RecordRefundFailure` | restore the `""` fallback |
| Unit `payments` | UNAVAILABLE cause satisfies `transientProviderFailure`; the other two do not | drop the marker entry |
| Unit `payments` | Cancelled booking + foreign `CollectorID`: no payment row written, no `ClaimRefund`, booking never `refund_pending` | move the check back below `:46` |
| Unit `payments` | The concurrent-cancellation path calls `refundCancelledBookingPayment` once, with one collector check | restore the recursion |
| Unit `payments` | Sweep: a cash booking with no MP id is answered `RefundManual` + alert, never `RefundPayment`, and its marker is cleared once | — |
| Integration | Cancel out-of-window, then run the orphan query: **zero rows**, while the row reads `cancelled` / `deposit_paid` with no `failed_refunds` row — the orphan's exact shape | add `OR (status='cancelled' AND payment_status IN ('deposit_paid','fully_paid'))` |
| Integration | Commit the cancel, skip `AutoRefundIfPaid`, run two sweepers concurrently: exactly one `ClaimRefundIntent` succeeds, exactly one refund is claimed | make the claim UPDATE unconditional |
| Integration | `ClaimRefund` leaves `refund_intent_at` NULL; the CHECK refuses a marker on a `confirmed` row | — |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file
classification or process-integration boundary. The one external trust boundary is
the MercadoPago webhook, and the collector requirement is exactly its treatment.

## Migration / Rollout

Nothing is deployed: **no backfill**, and every existing row takes NULL. Migration
010 is instantaneous on an empty table; after launch the CHECK would want
`NOT VALID` + `VALIDATE CONSTRAINT` (migration 006's cost note).

`make sqlc` is required because `UpdateBooking` is a sqlc query. Run it from a clean
tree in **its own commit** inside slice 3, containing only generated output — that
commit also carries the known stray `WebhookEvent` struct migration 006 documented,
which must be called out in the commit message so it is not mistaken for scope.
Slice 3's Go changes land in the next commit.

Rollback: slices 1 and 2 are pure code reverts. Slice 3 reverts by code revert plus
`migrate-down` on 010; the dropped column is read by nothing else.

## Open Questions

- [ ] **Premise, unconfirmed here.** That `RefundPayment` on the platform token is
      rejected by MercadoPago is inferred from its marketplace authorization model;
      nobody in this chain has had network access. **If it is false**, today's `""`
      fallback is not a doomed retry but a refund paid from the platform's own
      account — a live money-loss defect, and slice 2 becomes urgent rather than
      hygienic. **The design does not change either way**: per-arm refusal never
      sends the platform token, so it is correct under both branches; only the
      severity narrative moves. Confirmation needs a sandbox seller credential and
      is an operational task, not a blocker.
- [ ] Should a cash booking be marked at all? It is marked today, alerts once
      through `owedManually`, and its marker is then cleared. Not marking it would
      need a payment read inside the cancel handler; the alert is correct and the
      loop is bounded, so it is kept.
