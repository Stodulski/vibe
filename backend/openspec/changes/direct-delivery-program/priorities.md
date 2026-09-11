# Direct-delivery priorities

The six SDD-routed changes are archived. This orders what remains — the
exploration's changes 3 through 37 minus what has already landed — and it is
ordered by **what it costs to leave broken**, not by the exploration's numbering.

Two entries were re-verified before being ranked, and both turned out different
from their one-line description. That is the reason this file exists rather than
a re-sorted copy of the exploration's table: a priority list built on unverified
one-liners inherits their errors and adds authority to them.

## Already landed, not tracked as such by the umbrella

| # | Change | How |
|---|---|---|
| 13 | `authorization-route-test-coverage` | The route matrix plus `TestRoutePolicyTableIsComplete`, which fails closed on a new route |
| 15 | `client-ip-trust-boundary` | `TrustedProxies` as a CIDR type; a config that trusts every peer is refused at boot |
| 33 | `security-test-wiring` | `make test/security` runs for the first time — 16 tests that had never executed an assertion |
| 35 | `db-pool-boot-order` | Per-job random start-up jitter; twenty of twenty-five connections at boot became eleven short queries over thirty seconds |

## Partially landed — what is left is named

| # | Change | Done | Left |
|---|---|---|---|
| 3 | `circuit-breaker-correctness` | The `HalfOpenMaxReqs + 1` over-admission | A stale in-flight success closing the breaker; isolating the bulk refresh cron from the breaker that gates real payments |
| 5 | `refund-claim-atomicity` | claim/call/record, `exhausted` recoverable, the sweep drains its batch | Transitions that do not pass through `ClaimRefund` |
| 12 | `slot-lock-lifecycle` | The advisory lock became a lease row | `AcquireLock` respecting `expires_at`; release on every exit path |
| 17 | `rate-limiter-consistency-and-deadlines` | The data race; one `ceiling` for both backends | Explicit deadlines on the limiter's Redis call and the auth database read |

---

## P0 — a client loses money, or is silently not served

**1. The two-hour reminder never fires in the last two hours of the day.**
Re-verified, and it is not what finding 67 described. The predicate is

```sql
AND b.start_time <= (now::time + INTERVAL '2 hours')
AND b.start_time >  now::time
```

`'23:00'::time + INTERVAL '2 hours'` is `01:00`, and `01:00 > 23:00` is false —
confirmed against Postgres. So between 22:00 and midnight the predicate asks for
`start_time <= 01:00 AND start_time > 23:00`: the empty set. Finding 67 said it
"filters by five hours". It filters by **nothing**, in the window where a
forgotten booking costs a court its slot.

Six sites do this arithmetic across three editable files. Fix the helper, not
the six.

**2. Webhook idempotency (change 4).** A read failure returns 200, so
MercadoPago never redelivers and the payment is lost. Redelivery can orphan a
second payment row. `GetPaymentByMPID` is non-deterministic.

**3. Refund amount and fee truth (change 7).** The refund amount and the service
fee are recomputed locally instead of reconciled against MercadoPago's own
response. Any drift becomes a wrong refund.

**4. What still bypasses `ClaimRefund` (change 5's remainder).** Every refund
transition should be an atomic claim with a status predicate.

## P1 — the product tells the client something false

**5. Price band resolution (change 11).** The peak band loses to the base band,
so the price shown is not the price charged. Availability and booking use
different resolution.

**6. Cancellation copy (change 10).** A refund is promised for cash payments
that will never be refunded through MercadoPago, and a hardcoded fifteen minutes
stands in for the configured grace period.

**7. The swallowed preference-expiry branch (change 8).** Plus retry and
alerting on the request paths, and `WithinStandardWindow` from zero coverage.

**8. Notify exactly once on a terminal refund outcome (change 9).** One path
shared by the webhook, the auto-refund and the retry sweep, instead of three.

## P2 — correctness that hides until it does not

**9. Midnight-crossing math (change 23).** Three reviewers hit the same
calculation at three call sites. The reminder bug above is this family. When
three people find the same thing by separate routes, the defect is that there
are three implementations.

**10. `ValidFormat` accepts non-`HH:MM` and reinterprets it (change 26).**
Prerequisite for sweep item 61.

**11. Off-grid start times and unconsulted `blocked_slots` (change 25).**

**12. Three filters disagreeing on `no_show` (change 24).**

**13–15.** The remainders of changes 3, 12 and 17 above.

## P3 — infrastructure and robustness

16. `panic-recovery-hardening` (21) — `request_id` lost on the recovered path;
    `ErrAbortHandler` swallowed and an already-written response corrupted.
17. `user-cache-integrity` (22) — one error policy for the Redis user cache.
18. `db-connection-hygiene` (32) — retry transient errors; stop holding a pooled
    connection across an external call.
19. `sse-authorization-staleness` (20) — re-validate on a long-lived stream.
20. `audit-trail-completeness` (19) — public cancel writes an entry; tenants can
    read their own trail; admin reads are audited.
21. `exemption-list-exact-matching` (16).
22. `presign-content-type-enforcement` (29) — validated and then discarded.
23. `places-proxy-status-and-key-leak` (30) — **re-verify before scheduling.**
    The API key goes into a query parameter (`params.Set("key", ...)`), but no
    log or error path was found that emits the built URL. The leak may already be
    absent. Honouring the upstream status is the part that stands.
24. `xlsx-export-robustness` (31) — bound and stream; fail before the 200.
25. `slug-consistency` (34) — a soft-deleted slug reports free, then 500s.

## P4 — the sweep

26. `trivially-verifiable-sweep` (37) — nineteen findings in one pass, after
    change 26 lands, because item 61 depends on it.

---

## Ordering principle

Money and silence first, because both are invisible until somebody counts.
Then things the product asserts and gets wrong, because a client who is told the
wrong price does not file a bug — they leave. Then correctness that hides.
Then robustness. The sweep last, because it is the only group where the cost of
being wrong is a second pass rather than a refund.

---

# Closed — 2026-08-24

All twenty-six items. What follows is not a checklist; it is what the work
turned out to be, because in most cases the finding and the defect were not the
same thing.

## The one-liners were wrong about half the time

Kept as the record's most useful fact. Of the findings this program inherited:

- **Right symptom, wrong location** — the reminder's "filters by five hours" was
  real, but it lived in the `created_at` floor, not the window it was attached
  to; the window filtered by *nothing* between 22:00 and midnight. The slot-lock
  lifecycle findings were closed in `locks.go` and still live on the slot locks
  two packages away.
- **Wrong in the reassuring direction** — "the presign lets a client upload
  anything" was false, because the content type is inside the signature. And
  wrong in the alarming one, because a non-positive size drops the signature to
  the host alone.
- **Called absent when present** — I said the Places key probably no longer
  leaked, having searched for a line that logs the URL. Nobody logs the URL:
  `net/http` puts it in every transport error, and the error is logged.
- **Already closed** — three of the four webhook findings, three of the five
  panic and cache findings, the `AcquireLock` expiry, both rate-limiter
  deadlines.

Every one cost a round trip, and every one was caught by verifying before
fixing. That rule earned its place.

## Where the premise was wrong and the code said so

- **`GetPaymentSummary` must not be aligned.** I told the agent every revenue
  figure comes from `payments.status`, so alignment cost nothing. False for that
  one query, which sums deposits off the bookings table. The mutation that
  "helpfully" aligned it cost a venue's day 150,000 — the forfeited deposit is
  money really held. *Hours cannot be sold twice; pesos can be collected twice.*
- **The obvious circuit-breaker fix was worse than the bug.** Declining to
  credit a stale success leaves the half-open window spent with no verdict, and
  only a report can leave half-open — so it converts an intermittent false
  recovery into a permanent checkout outage.
- **A sweep on row shape would have paid out money the venue keeps.** An
  orphaned refund and a deliberately declined one are the same row; only a
  marker separates them.

## What kept shipping green

Seven defects survived their tests because a double could not observe: it
returned an empty value, discarded its arguments, ignored its context, could not
fail, held a live pointer the code mutated afterwards, or returned a flat value
where the real store accumulates.

And three tests passed on assertions weak enough that the bug satisfied them —
`!= 200`, "an alert fired", "two URLs came back". All true. None the thing the
test was named for.

## Four ways a mutation lies

All four turned up here:

1. **It does not compile.** The commonest, and the easiest to mistake for a
   result.
2. **It does not run** — a broken query, an unused bind parameter.
3. **It never applied** — a pattern that matched nothing, a helper invoked
   without its arguments. The suite prints `ok` and reads exactly like a
   mutation that did not matter.
4. **It stays green because another gate holds, or the fixture lacks the case.**
   The right response is to find out which; twice it was a real coverage hole.

## The final item could not be executed as written

`trivially-verifiable-sweep` carried nineteen finding numbers and no
descriptions — the text was lost. Reconstructing defects from numbers would have
meant inventing them, so a fresh review pass ran over the least-worked packages
instead. It found three real defects, including a revocation discarded the
moment Redis recovered, and reported four packages clean with what was checked
in each.

A clean package is a result. Without it, a short report is indistinguishable
from a lazy one.

## Carried forward

- The frontend lives in a separate repository and consumes contracts this work
  changed. Deploy order for the booking-link change is slice 1, then the
  frontend, then slice 2.
- ~~`mp.Caller` (`AsSeller`/`AsPlatform`).~~ Closed 2026-08-24. Calling as the
  platform is now something a call site writes down, and the empty string no
  longer means anything. Note what the work surfaced: the platform fallback
  was load-bearing in **three** places, not the one that was known —
  `internal/payments/webhook.go` (reading a payment before the seller is
  known), plus `internal/bookings/cancel.go` and `cmd/api/cron.go`, which both
  fall through to the platform to expire a checkout preference because a
  rejected call costs less than a payable link on a cancelled booking.
  Deleting the fallback to "make the four consistent" would have left those
  links live.
- `cmd/api/cron.go` and `internal/bookings/cancel.go` disagree about an
  unreadable seller credential. cancel.go refuses and alerts; cron.go treats it
  exactly like not-connected and calls as the platform. Both behaviors were
  preserved verbatim through the `mp.Caller` migration rather than quietly
  reconciled. cron.go's is the suspect one, and deciding it is a behavior
  change, not a refactor.
- ~~MercadoPago-side token revocation on disconnect.~~ Answered 2026-08-24:
  no such endpoint exists. See the section below. The `mp-connect` webhook is
  the useful inverse and is scoped, not built.
- The owner dashboard now refuses bookings outside opening hours, which it never
  did. If owners are meant to book off-hours specially, that is the line.
- ~~A complex with `cancellation_hours = 0` gets non-expiring booking links.~~
  Closed 2026-08-24: floor of 1 at the handler and at the column
  (`db/migrations/013`). **The frontend repository must send
  `cancellation_hours` on `POST /complexes`** — omitting it is now a 422,
  because the field is a value and not a pointer, so an omitted field was
  submitting zero. That is exactly how the two zero rows in the development
  database were written.
- ~~`httpx` logs the full RequestURI, so a booking link token reaches server
  logs in plaintext on any error.~~ Closed 2026-08-24. The error log now writes
  the path and the query's KEYS, never a value. Values are dropped wholesale
  rather than by a list of the ones known to be sensitive — a list is a
  snapshot of what somebody remembered, and the next credential to travel in a
  query string would be logged until they remembered again.

---

# MercadoPago-side revocation — answered 2026-08-24

Carried as unverified through three changes, because no agent in the chain had
network access. Checked against MercadoPago's own documentation, and the answer
inverts the finding.

**There is no endpoint an integrating application can call to revoke a seller's
tokens.** Revocation is always seller-initiated:

> *"Authorization revocation: revoking an authorization between the seller and
> the application triggers the deletion of all tokens and temporary grants
> associated with them."*
>
> *"User password change: there are password change flows where the seller can
> revoke all your credentials, including associated tokens and temporary
> grants."*

Both are things the seller does. A third path exists — MercadoPago's own fraud
team invalidating a user's credentials, and application deletion — and neither is
ours to call either.

So `DisconnectMercadoPago` clearing only the local columns is not a gap to close.
It is the whole of what this side can do. **The follow-up as scoped cannot be
built, and the record should stop implying it can.**

## What can be built, and it is the more useful half

MercadoPago notifies the application when a seller authorizes or deauthorizes
it. The topic is **`mp-connect`** — *"linking and unlinking of accounts connected
via OAuth"*.

Today this service learns that a seller disconnected only when a payment fails.
Until then it holds credentials that are already dead, keeps refreshing them on
the twelve-hour cron, and answers `payments_enabled: true` on the public page for
a venue that cannot take payments. A client reaches checkout and it breaks there.

Subscribing to `mp-connect` turns that into: the seller disconnects, the webhook
arrives, the credentials are cleared, and the storefront stops offering payment
for that venue in the same minute.

**Not built here**, because it needs two things this session cannot supply: the
payload shape, which the docs do not publish and which has to be observed against
a sandbox account, and a decision about what a venue's page should say between
disconnection and reconnection. It is scoped rather than guessed.

Sources: MercadoPago OAuth token management, and the webhook topics reference.

---

# The audit trail was blocking account deletion — found 2026-08-24

Found while wiring the auth audit trail, and worse than the gap that was being
closed.

`audit_log.user_id` was declared in migration 001 as `user_id UUID REFERENCES
users(id)` with no `ON DELETE` clause. In PostgreSQL that is `NO ACTION`: the
users row **cannot be deleted** while any entry points at it. Not cascaded, not
nulled — refused.

`UserModel.Delete` is a plain `DELETE FROM users`, so `DELETE /api/v1/auth/me`
already answered 500 for any owner who had ever been named in the trail — one
who edited a court, or cancelled a booking. The unit suite could not see it:
its stores are stubs, and stubs have no foreign keys. Proven directly against
the running e2e database, in a rolled-back transaction:

```
ERROR:  update or delete on table "users" violates foreign key constraint
        "audit_log_user_id_fkey" on table "audit_log"
```

**The auth trail widened it from some owners to everyone**, because sign-out and
password change write a non-nil `user_id`, and every account signs out.

Fixed in `db/migrations/014` with `ON DELETE SET NULL`, not `CASCADE`. Cascade
would also make the delete succeed — by destroying the record of everything the
account ever did, which is the opposite of what the table is for. Every writer
already names the thing acted on in `entity_id`, which carries no foreign key,
so a NULL `user_id` still reads as "an act by an account that no longer exists",
with the account identified beside it.

The regression test asserts both halves. A test that only checked that the
delete succeeds would pass against exactly the wrong fix.

`complex_id` has the same `NO ACTION` default and was left alone: complexes are
soft-deleted in production, and only test cleanup hard-deletes one.

## The second finding, not fixed

`audit_log.old_value` and `new_value` are written by `AdminModel.InsertAuditLog`
and **selected by no query in this codebase**. Neither `listAuditLogsSQL` nor
`data.AuditLogRow` mentions them. Every audit value written since this table
existed is write-only through the API — the data is correct to keep, because a
reader will exist and the entries have to be there when it does, but today
nobody can read one.
