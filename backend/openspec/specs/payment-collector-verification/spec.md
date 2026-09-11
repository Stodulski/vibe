# Payment Collector Verification Specification

## Purpose

No branch of `processApprovedPayment` that writes a `data.Payment` row or
moves a booking's payment columns may do so for a payment whose collector
identity is unproven. `collectorMatchesComplex`
(`internal/payments/process.go:402-441`) already exists and is fail-closed;
today it is reached only from the normal pending-booking branch at
`process.go:92`. The already-cancelled branch returns at `process.go:61`
(`return h.refundCancelledBookingPayment(...)`), ahead of both the amount
check at `process.go:66-81` and the collector check at `:92`, and writes a
payment row from `mpPayment.TransactionAmount` /
`mpPayment.ExternalReference` — both attacker-controlled on the attacker's
own MercadoPago preference — with neither check applied. This does not lose
funds (MercadoPago's own collector check is expected to reject the
resulting wrong-collector refund), but it durably attaches a fictitious
payment row to a victim complex/booking, drives it to
`refund_status='pending'` permanently once the retry budget
exhausts, and fires `REFUND EXHAUSTED` / `MANUAL REFUND OWED` at a venue
that received nothing.

## Requirements

### Requirement: Collector identity is verified before any branch of `processApprovedPayment` writes a payment row or moves money

The system MUST verify that an approved MercadoPago payment was collected by
the seller account belonging to the booking's complex before any branch of
`processApprovedPayment` — including the already-cancelled branch — inserts
or updates a `data.Payment` row, or moves the booking's `collection_status`
or `refund_status` toward a refund. A collector mismatch or an unverifiable collector
identity MUST refuse the write, exactly as `collectorMatchesComplex` already
refuses it for the normal branch today.

#### Scenario: The already-cancelled branch refuses a payment from an unproven collector

- GIVEN a booking already has `status='cancelled'` for complex A, and an
  approved MercadoPago payment webhook names that booking as its
  `external_reference`, with a `collector_id` that does not match complex
  A's `mp_user_id`
- WHEN `processApprovedPayment` routes into the already-cancelled branch for
  that webhook
- THEN no `data.Payment` row is written for the booking, neither its
  `collection_status` nor its `refund_status` is moved toward a refund, and
  `RefundPayment` is never called

#### Scenario: The already-cancelled branch still refunds a payment from the correct collector

- GIVEN a cancelled booking whose approved payment's `collector_id` matches
  its complex's `mp_user_id`
- WHEN `processApprovedPayment` routes into the already-cancelled branch
- THEN the payment is recorded and `refundCancelledBookingPayment` proceeds
  exactly as it does today

#### Scenario: The normal pending-booking branch is unaffected

- GIVEN a booking still pending payment, whose approved webhook payment's
  `collector_id` matches its complex
- WHEN `processApprovedPayment` runs the normal confirmation branch
- THEN the booking confirms exactly as it does today

### Requirement: The amount-equality check is not extended to the already-cancelled branch

The already-cancelled branch's payment record MUST continue to be built
from MercadoPago's own `TransactionAmount`
(`internal/payments/process.go:276-283`, `recordPaymentOwedARefund`), and
MUST NOT be compared against the booking's current
`DepositAmount`-derived expected amount (the check at `process.go:72-81`).
Only the collector check moves ahead of this branch's write; the amount
check stays where it is.

#### Scenario: A legitimate refund whose deposit changed since checkout is not refused

- GIVEN a booking was cancelled after checkout, and its `DepositAmount` (or
  the complex's pricing) has since changed from what the client actually
  paid at checkout time
- WHEN an approved payment webhook from that booking's own collector
  reaches the already-cancelled branch
- THEN the payment is recorded using MercadoPago's `TransactionAmount`
  rather than a recomputed expected amount, and the refund is not refused
  for an amount mismatch

## Out of Scope

- Reconciling the refunded amount or fee against MercadoPago's response.
- Hoisting or duplicating the amount-equality check — explicitly rejected
  above.
- The recursive re-entry at `process.go:119-125` running the collector
  check twice — read-only and idempotent; left to design.
