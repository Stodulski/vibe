# Archive Report — refund-durability-and-collector-integrity

Archived 2026-08-23. Third and final SDD-routed change in the money-path
remediation program to carry a hard dependency edge.

Written by the orchestrator rather than the archive phase agent. Two reasons,
both recorded because a silent gap in an archive is worse than a stated one.

## Why there is no verify-report.md

The verify phase agent made **zero writes**. Its contract requires running
`gentle-ai sdd-verify-validate` before persisting a report, that tool was not
available in the session, and the contract forbids persisting without it. It
delivered its findings inline and said so, rather than writing an unvalidated
file that would read as validated.

That was the right call, and the verdict is recorded below instead.

## Why this report was rewritten

The archive phase agent's own report cited a regression test named
`TestALegitimateRefundWhoseDepositChangedSinceCheckoutIsNotRefused`. **No such
test exists.** The test it describes is real and is
`TestACancelledBookingIsRefundedEvenWhenItsDepositMoved`
(`internal/payments/payments_test.go`).

A wrong test name in an archive is not a typo. It is the exact failure this
program has been removing all week: somebody greps for the guarantee the record
promises, finds nothing, and concludes the coverage was never written. Corrected
rather than left, and the reason kept so the next reader knows the record was
checked rather than copied.

The report also cited `process.go:66` for the amount-check comment. Line numbers
in this file have moved three times across this change's own four commits. The
comment is in `processApprovedPayment`, above the amount-equality check, and
names `recordPaymentOwedARefund` as the reason.

## Verification outcome

**PASS, archive recommended. No CRITICAL findings.**

Every gate green, all run by the verifier rather than taken from the apply
reports: `go build` under both tags, `make test`, `-race` across 29 packages,
`golangci-lint` at 0 issues on a confirmed-clean build, `make audit`, a goose
down/up round-trip on migration 010, and `make test/integration` followed
separately by `make test/security` — never concurrently, because they share one
database and TRUNCATE between tests.

All requirements across the three specs PASS with runtime evidence.

Two properties were judged independently at the strictest reading and hold:

- **`owesRefund` is genuinely one expression** in each cancel handler —
  `actions.go` and `public.go`, each declared once and read twice with no
  restatement. Not two conditions that happen to agree.
- **`AutoRefundIfPaid` clears the marker through a single `defer`-guarded
  boolean** rather than four hand-placed calls. That is stronger than the spec's
  literal wording: a branch added later cannot silently skip the clear.

**One WARNING, closed before archive in `674ebe3`.** The spec's "a legitimate
refund whose deposit changed since checkout is not refused" scenario had no test
by that name. Coverage existed, but accidentally: an unrelated test caught the
same mutation only because its fixture leaves `DepositAmount` at zero while the
payment carries 1500 pesos — a mismatch nobody wrote on purpose, and one tidy-up
away from gone. The explicit test now states the case in its own setup and is
mutation-verified.

## What must survive, and did

**Why the sweep is safe — both properties, not one.** Its predicate names a
single column, so `status`, `payment_status`, `deposit_amount` and `payments`
cannot pull a row into it; and the marker is written from the same expression
that decides whether the refund runs. A future reader who adds a "defensive"
filter on those columns breaks it. That instinct is the thing this record exists
to forestall.

**Why the amount check does not move with the collector check.**
`recordPaymentOwedARefund` builds the cancelled branch's payment from
MercadoPago's own transaction amount on purpose. A deposit that changed since
checkout would fail the recomputed comparison and refuse a refund the client is
owed. An implementer reading "verify before the branch writes" would plausibly
move both; the code says why not, and a test now fails if they do.

**The modification's scope.** This supersedes the archived seller-credential
requirement **only** for `getSellerToken`'s three sites, where the claim commits
first so a refusal is queued rather than manual. Preference expiry in
`cancel.go` and `cronReleaseExpiredPayments` keeps loud-and-continue, and both
were verified untouched by all four commits via per-file git history.

## Carried forward

- **`cancel.go`'s credential-read failure has no Sentry alert.** It sets an empty
  token and continues. The archived requirement's "as loud as today" claim may
  never have been satisfied at that site. It predates this change and was
  confirmed untouched, so it is a follow-up rather than a regression.
- **The MercadoPago rejection premise is still unconfirmed.** Nobody in this
  chain has had network access to check whether a platform-token refund is
  actually rejected. The design chose per-arm refusal partly because it never
  sends that token, so the mechanism is correct either way — but the severity
  narrative depends on it. First task of any follow-up is confirming it against
  a sandbox credential, not building anything.
- **`mp.Caller` (`AsSeller`/`AsPlatform`)** — deferred from the previous change
  and still the shape that would make the platform-token confusion impossible
  rather than documented.

## Program position

Last of the six SDD-routed changes with a hard dependency edge.
`booking-link-credential-hardening` is unblocked by the owner's expiry decision
of 2026-08-21 but has not started. The direct-delivery changes continue.
