# Delta for Seller Credential Integrity

## MODIFIED Requirements

### Requirement: Call sites that read or modify an already-created obligation stay at least as loud on failure as they are today

At call sites that only read or modify a payment obligation that already
exists — a refund, a preference expiry — a credential-read failure MUST be
surfaced at least as loudly as the equivalent "not connected" case is
surfaced today, and MUST NOT be silently swallowed into a quieter path than
exists now. This requirement continues to govern the preference-expiry call
sites unchanged: `internal/bookings/cancel.go:104` and `cmd/api/cron.go:171`
(`cronReleaseExpiredPayments`) MUST keep routing a credential-read failure
into a path at least as loud as today's.

`getSellerToken` (`internal/payments/refund.go:441-461`) is no longer
governed by this requirement's loud-alert-**and-continue** trade. (Previously:
this requirement also routed `getSellerToken`'s credential-read failure into
that same alert-then-proceed path — logging, sending a Sentry message, and
letting the caller present the refund to MercadoPago with an empty seller
token — reasoning that "refusing outright would turn a recoverable refund
into a manual one".)

**Superseded here, for `getSellerToken` only.** That reasoning does not hold
at any of `getSellerToken`'s three call sites — `AutoRefundIfPaid`
(`refund.go:273`), `RetryFailedRefunds` (`refund.go:505`), and
`refundCancelledBookingPayment` (`process.go:313`) — because each of them
asks for the seller token only *after* `ClaimRefund` (or, for the retry job,
a pre-existing attempt row) has already committed a durable, retry-tracked
`failed_refunds` row. Refusing there does not turn a recoverable refund into
a manual one, because nothing about the claim's durability depends on
whether the seller token was readable — the row exists either way. The
per-arm refusal behavior that replaces the old continue-anyway trade for
these three sites is specified below, under "A seller-token failure that
cannot self-heal refuses before MercadoPago is called". The preference-expiry
call sites keep the original trade because they have no claim to refuse
into: refusing loudly, without a queued attempt behind it, is already the
loudest option available to them, exactly as today.

#### Scenario: A preference-expiry credential failure is at least as loud as today

- GIVEN a booking cancellation or `cronReleaseExpiredPayments` needs to
  expire a live MercadoPago preference for a complex whose stored
  credential cannot be decrypted
- WHEN `internal/bookings/cancel.go`'s expiry call, or the cron job, asks
  for that complex's seller token
- THEN the failure is surfaced at least as loudly as the "complex has no MP
  access token" case is today — unchanged by this delta

## ADDED Requirements

### Requirement: A seller-token failure that cannot self-heal refuses before MercadoPago is called, instead of spending the retry budget

`getSellerToken`'s three call sites — `AutoRefundIfPaid`,
`RetryFailedRefunds`, `refundCancelledBookingPayment` — MUST distinguish the
three outcomes `internal/data/mpcred.go` already returns as typed sentinels,
rather than collapsing them into an empty string:

- **UNAVAILABLE** (fetching the complex failed — a transient database read):
  MUST continue to be treated as an ordinary provider-call attempt whose
  eventual failure is classified by the existing `transientProviderFailure`
  rule (`internal/data/failed_refunds.go:104-112`) — a retryable outage that
  does not spend the escalating 1m/5m/15m/1h/4h backoff.
- **UNREADABLE** (a stored credential exists but fails to decrypt) and
  **MISSING** (never connected, or disconnected): MUST refuse before any
  request reaches MercadoPago, and MUST raise an operator-facing alert on
  this same attempt, naming which of the two arms occurred — the same
  severity of alert `recordRefundFailure` today only reaches once its retry
  budget is exhausted, reached here without first spending five queued
  attempts across roughly 5h20m of wall-clock time.

No second classification rule is introduced beside `transientProviderFailure`:
all three arms are routed through the existing `recordRefundFailure` seam.

> **Premise this requirement's severity narrative leans on, not verified in
> this environment.** MercadoPago is assumed to reject a refund presented on
> the platform's own (non-seller) access token, because the platform is not
> the collector of a seller-scoped payment — inferred from MercadoPago's
> marketplace authorization model (`internal/mp/mp.go:491-495`), not observed
> against a live or sandbox account. If the premise is false, the
> continue-anyway behavior this requirement replaces issued refunds from the
> platform's own account rather than being rejected; this requirement closes
> that path either way, because UNREADABLE/MISSING never reach MercadoPago
> under it — only how severe the prior behavior was changes with the
> premise, not the correctness of refusing here.

#### Scenario: UNREADABLE refuses on the first attempt with a credential-specific alert

- GIVEN a refund has been claimed (`ClaimRefund` has committed) for a
  booking whose complex's stored MercadoPago credential exists but fails to
  decrypt
- WHEN the refund attempt asks for that complex's seller token
- THEN no request reaches MercadoPago, and an operator-facing alert naming
  the failure as credential-unreadable is raised on this same attempt

#### Scenario: MISSING refuses on the first attempt with a credential-specific alert

- GIVEN a refund has been claimed for a booking whose complex has never
  connected MercadoPago, or has disconnected it
- WHEN the refund attempt asks for that complex's seller token
- THEN no request reaches MercadoPago, and an operator-facing alert naming
  the failure as credential-missing is raised on this same attempt

#### Scenario: UNAVAILABLE still queues and retries without spending a retry

- GIVEN a refund has been claimed for a booking whose complex cannot
  currently be fetched (a transient database read failure)
- WHEN the refund attempt asks for that complex's seller token
- THEN the failure is classified as an outage by the existing
  `transientProviderFailure` rule, and the attempt's retry count is not
  spent for it

#### Scenario: A seller-token refusal always has a committed attempt row behind it

- GIVEN any of the three call sites has already committed a `ClaimRefund`
  attempt row (or, for the retry job, an existing one) before asking for
  the seller token
- WHEN a seller-token failure — UNAVAILABLE, UNREADABLE, or MISSING — is
  recorded
- THEN the outcome is produced by `recordRefundFailure` against that
  already-committed row — queued for retry, or exhausted with a full retry
  history — never the untracked, no-attempt-row refusal `owedManually`
  produces for a claim that was never reserved
