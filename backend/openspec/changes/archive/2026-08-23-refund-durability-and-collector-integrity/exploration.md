# Exploration — refund-durability-and-collector-integrity

Program change 6 (findings 1, 3, 4, 9). Produced by the `sdd-explore` phase agent,
which again had no filesystem write tool; it persisted to Engram
(`sdd/refund-durability-and-collector-integrity/explore`, id 69) and the
orchestrator materialized this file. That is the third time in this program, and
it is worth fixing in the phase agent's tooling rather than paying the round trip.

## It corrected the orchestrator's framing, and that correction leads

The launch brief for this exploration said, in the orchestrator's own words, that
falling back to the platform token means "the platform pays out of its own
MercadoPago account and someone reconciles it later."

**That is wrong for the refund call sites.** `mp.go:467 RefundPayment` presented
with an empty seller token uses the platform's token, and MercadoPago rejects it,
because the platform is not the collector of a seller-scoped payment. The
wrong-payee hazard lived at `mp.go:352 CreatePreference` — where an empty token
was silently *accepted* and money settled to the platform — and that site was
closed by the prerequisite change.

One honest qualification the exploration did not make and this file does: nobody
in this chain has had network access to MercadoPago. The rejection is inferred
from MercadoPago's marketplace authorization model and from the fact that the
same model is what makes `GetPayment(ctx, id, "")` legitimately work for the app
owner. It is a strong inference, not an observation. Design should treat it as a
premise to confirm, not a measured fact — and if it turns out to be wrong, the
severity of everything below changes.

**What fail-closed actually buys, given that.** Not fund protection. A
credential-unavailable refusal is not classified by `transientProviderFailure`
(`internal/data/refunds.go:104-112`), so it spends the full retry budget —
1m/5m/15m/1h/4h, roughly 5h20m across five attempts — before
`recordRefundFailure` marks the attempt `exhausted` and alerts. For MISSING and
UNREADABLE those five hours are guaranteed-doomed: retrying cannot connect an
account or decrypt a bad key. Fail-closed collapses that into an immediate,
correctly labelled alert.

## The three arms are not one case

- **UNAVAILABLE** — `h.complexes.GetByID` failed (`refund.go:442`). A transient
  database read. Retrying plausibly resolves it, and the existing
  `failed_refunds` machinery is the right home.
- **UNREADABLE** — the credential exists and will not decrypt. Every refund for
  that venue keeps failing until somebody rotates a key. Retrying is waste.
- **MISSING** — the venue never connected MercadoPago. Retrying is waste
  forever.

The codebase already draws exactly this outage-versus-rejection line in
`transientProviderFailure`. A uniform fail-closed policy would introduce a second,
inconsistent rule and would regress the one arm that can self-heal.

## The durability window — confirmed, and nothing catches it

Both cancel paths have the same shape:

- `internal/bookings/actions.go` `Cancel`: `booking.Status = "cancelled"` at `:77`,
  committed by `h.store.Update` at `:86` — a write that does **not** touch
  `payment_status`. `AutoRefundIfPaid` runs afterwards at `:105`, and only inside
  it does `ClaimRefund` (`internal/data/refunds.go:65`) commit a durable
  `failed_refunds` row.
- `internal/bookings/public.go` client cancel: identical, `:556` and `:581`.

If the process dies between those two commits, the booking is durably
`status='cancelled'` with `payment_status` still `'deposit_paid'` or
`'fully_paid'`, no `failed_refunds` row exists, and **nothing records that money
is owed**.

Verified that nothing catches it. The full cron registry was read
(`cmd/api/cron.go:42-59`): thirteen jobs, none scanning for a cancelled booking
whose payment status still says paid with no covering claim. `RetryFailedRefunds`
only works rows that already exist, and a crash in this window produces none.
Migration 006's triggers forbid *reversal* — terminal back to live — and do not
detect a stuck-but-individually-valid combination.

## The collector bypass — still live, and worse than the program scoped it

`collectorMatchesComplex` (`process.go:402-441`) is reached only in the normal
confirmation branch at `:92`. The already-cancelled branch at `:46-62` returns
first, orchestrator-verified: `return h.refundCancelledBookingPayment(...)` at
`:62` precedes the amount check at `:66` and the collector check at `:92`.

That branch writes a `data.Payment` row from `mpPayment.TransactionAmount` and
`mpPayment.ExternalReference` — both attacker-controlled on the attacker's own
MercadoPago preference — with neither check applied.

What it costs, stated precisely rather than dramatically:

- **Not theft.** `RefundPayment` rejects the wrong-collector refund, subject to
  the premise above.
- **A durable integrity defect.** A fictitious payment row is attached to a
  victim complex and booking, the booking goes to `payment_status='refund_pending'`
  (`process.go:354`), and because the mismatch is permanent rather than transient,
  it stays there after the retry budget exhausts. There is no automatic recovery.
- **Alert misdirection.** The exhausted attempt fires the same
  `REFUND EXHAUSTED` / `MANUAL REFUND OWED` messages used for genuine cases
  (`refund.go:376`, `:409`), pointing an operator at a venue that received nothing.
- **A low bar to reach.** It needs a booking ID already cancelled at the target
  complex, and client cancellations and the fifteen-minute expiry cron produce
  those routinely.

## What already works — verified, so nobody proposes it again

- `ClaimRefund` / `RecordRefundSuccess` / `RecordRefundFailure`
  (`internal/data/refunds.go`) are durable and atomic once reached.
- `transientProviderFailure` already stops a provider outage from spending the
  retry budget, and both queues revisit `exhausted` once the backoff elapses.
- Migration 006's monotonicity triggers and cross-tenant FKs hold; they simply do
  not cover either gap here.
- Seller credentials are encrypted at rest with a keyring, which is what makes
  this change's precondition — trustworthy token persistence — true.

## Framed for design, not decided

**getSellerToken policy.** One uniform refusal, per-arm handling, or leave the
call and only change how `recordRefundFailure` classifies the failure. The
per-arm shape needs a typed result rather than today's `""`, and that ripples to
three call sites (`refund.go:273`, `refund.go:505`, `process.go:313`) — a small
blast radius, but all three are in the money path.

**Durability.** A reconciliation sweep matching the existing sweep idiom; merging
the cancel write and the claim into one transaction; or writing a durable intent
marker from the cancel handler ahead of the refund. The sweep bounds how long an
orphan stays invisible without closing the window; the transaction merge closes
it but crosses three package boundaries in a change this program has repeatedly
warned about scope creep in.

**Collector.** Applying the existing check before the cancelled-booking branch
writes anything. The exploration found no tradeoff against it.

## Risks

- If design inherits the orchestrator's original wrong-payee framing rather than
  the correction above, it will overstate the money-loss risk and underweight the
  permanently-stuck booking, which is the real defect.
- A reconciliation sweep needs its own idempotency guard, in the spirit of
  `MarkProcessing`'s conditional UPDATE (`failed_refunds.go:205-223`), or two
  instances will both claim the same orphan.
- The MercadoPago rejection premise is inferred, not measured. Confirm it.
