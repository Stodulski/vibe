# Refund Intent Durability Specification

## Purpose

Both cancel paths (`internal/bookings/actions.go` `Cancel`,
`internal/bookings/public.go`'s client cancel) commit `status='cancelled'`
before calling `AutoRefundIfPaid`, and only inside `AutoRefundIfPaid` does
`ClaimRefund` (`internal/data/refunds.go:65`) write the first durable
record that a refund is owed. A crash between the two commits leaves a
booking durably `status='cancelled'` with `payment_status` still
`'deposit_paid'`/`'fully_paid'` and no `failed_refunds` row — nothing in
the current cron registry (thirteen jobs, `cmd/api/cron.go:42-60`) looks
for this shape.

This specification pins what must durably exist after a cancel commits, and
what a reconciliation sweep may act on. It does not choose the marker's
storage shape, the sweep's query, or its schedule — those are design's to
decide.

**The sweep is not the fix by itself, and this is the load-bearing finding
of this capability.** An orphaned refund (the crash case above) and a
deliberately declined one produce the *identical* row: `status='cancelled'`,
`payment_status` still paid, no attempt row. A client cancelling outside the
complex's refund window through the public path
(`internal/bookings/public.go:582-587`, `RefundNotEligible`) produces this
exact shape on purpose, while a staff cancellation
(`internal/bookings/actions.go:96-99`) refunds regardless of window by
product decision. A sweep that inferred "orphan" from the row shape alone
would refund every out-of-window client cancellation — money the venue is
entitled to keep.

## Requirements

### Requirement: A refund-owed outcome from cancellation is durably recorded no later than the cancel write commits

When a cancellation of a currently-paid booking (`payment_status` is
`'deposit_paid'` or `'fully_paid'`) is a case where a refund would be owed,
a durable record of that fact MUST exist by the time the cancel write's
transaction commits — in that same commit, not a later, separate one — so
that a process crash immediately afterward, before `ClaimRefund` runs,
still leaves a discoverable trail. This record MUST NOT be expressed as
`payment_status='refund_pending'`: `refundable()`
(`internal/payments/refund.go:320-323`) reads that value as "a claim
already exists" and would decline the very refund the record exists to
guarantee.

#### Scenario: The record survives a crash between the cancel commit and the claim

- GIVEN a paid booking (`payment_status='deposit_paid'` or `'fully_paid'`)
  is cancelled through either the staff or the public cancel path, in a
  case where a refund is owed
- WHEN the process crashes immediately after the cancel write commits and
  before `AutoRefundIfPaid`'s `ClaimRefund` runs
- THEN a durable record, written by the cancel commit itself, already
  states that this booking's cancellation owes a refund attempt

#### Scenario: The record does not make the ordinary refund path decline itself

- GIVEN a booking is cancelled and its refund-owed record is written by the
  same commit
- WHEN `AutoRefundIfPaid` runs immediately afterward, in the ordinary
  non-crashed case
- THEN `refundable()` does not read the record as "a claim already exists"
  and does not decline the refund on that basis

### Requirement: The reconciliation sweep never refunds a deliberately declined refund

The sweep MUST select only bookings carrying the durable refund-owed record
above. It MUST NOT select or act on a booking based on the row shape alone
(`status='cancelled'`, `payment_status` still paid, no attempt row), because
that shape is also produced by a booking whose refund was deliberately
declined. A booking for which the cancel path decided no refund is owed
(including an out-of-window public cancellation) MUST NOT carry the record,
and the sweep MUST NOT claim or refund a booking that lacks it.

#### Scenario: An out-of-window public cancellation is never swept

- GIVEN a client cancels a paid booking (`payment_status='deposit_paid'`)
  outside the complex's refund window, through the public cancellation path
  (the `RefundNotEligible` branch at `internal/bookings/public.go:582-587`)
- WHEN the reconciliation sweep runs afterward
- THEN it does not find, claim, or refund that booking — no refund-owed
  record was written for it, because the cancel path decided the venue
  keeps the money

#### Scenario: An orphan produced by a genuine crash is found and refunded

- GIVEN a staff cancellation, or an in-window public cancellation, commits
  the cancel write with its refund-owed record set, and the process then
  crashes before `ClaimRefund` runs
- WHEN the reconciliation sweep runs
- THEN it finds the record, claims it, and drives the same
  claim-call-record path `AutoRefundIfPaid` uses, ending with the refund
  issued or durably queued

#### Scenario: The sweep never calls MercadoPago for a payment with no MercadoPago id

- GIVEN a booking was paid in cash or by transfer (no MercadoPago payment
  id) and cancelled in a case where a refund would be owed
- WHEN the reconciliation sweep runs
- THEN it never calls MercadoPago to refund that booking — a cash/no-id
  refund stays routed to the existing `owedManually` alert, not to an
  automatic provider call

### Requirement: Two concurrent sweep runs cannot both claim the same orphan

When two sweep runs race to act on the same refund-owed record, at most one
of them may claim and process it, using an idempotency guard in the spirit
of `MarkProcessing`'s conditional UPDATE
(`internal/data/failed_refunds.go:205-223`).

#### Scenario: Only one of two concurrent sweepers claims an orphan

- GIVEN a refund-owed record exists with no covering `failed_refunds`
  attempt
- WHEN two sweep runs, in two different processes, attempt to claim it at
  the same time
- THEN exactly one of them claims and processes it; the other observes it
  is already taken and does not duplicate the claim or the refund attempt

## Out of Scope

- Merging the cancel write and `ClaimRefund` into one transaction — the
  crash window is bounded by the sweep, not eliminated by it.
- Reconciling refunded amount/fee against MercadoPago's response, and
  refund-window policy itself (owned elsewhere).
- Backfilling any pre-existing orphan data — nothing is deployed yet.
