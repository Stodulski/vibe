# Exploration — booking-link-credential-hardening

Program change 18, finding 100. Produced by the `sdd-explore` phase agent, which
again had no filesystem write tool; it persisted to Engram
(`sdd/booking-link-credential-hardening/explore`, id 76) and the orchestrator
materialized this file. **Fourth time in this program.** It is no longer worth
noting as an incident — it is a standing gap in the phase agent's tooling and
should be fixed there.

## The product decision was already made

The owner chose **expiry** on 2026-08-21, and the reasoning is recorded in the
program umbrella: expiry does not depend on anything firing. A link leaked into
a group chat, a forwarded email or a screenshot stops working on its own,
without anyone having to notice the leak. This exploration maps how that lands;
it does not reopen the choice.

## What exists

Three public routes take a `booking_id` UUID and nothing else
(`internal/bookings/bookings.go:221-224`). None is wrapped in `owner(...)`, the
guard every tenant-scoped route in the same file uses:

```go
router.HandlerFunc(http.MethodGet,  "/api/v1/book/status",      h.PublicStatus)
router.HandlerFunc(http.MethodGet,  "/api/v1/book/cancel-info", h.PublicCancelInfo)
router.HandlerFunc(http.MethodPost, "/api/v1/book/cancel",      h.PublicCancel)
```

Each parses the UUID and calls `h.store.GetByID`. That is the entire
authorization. `GetByID` (`internal/data/bookings.go:208-227`) is
`WHERE b.id = $1 LIMIT 1` — no expiry predicate, no status predicate, no tenant
scoping. Whoever holds the UUID is the booking's owner as far as the system is
concerned.

The three-route claim was corroborated exhaustively rather than by re-reading
one file: every `router.HandlerFunc` registration in the repo was checked. There
is no fourth unguarded booking-id route. Webhooks are signature-authenticated,
not id-authenticated.

## Worse than scoped: the identifier reaches Sentry unredacted

`cmd/api/sentry.go` carries a deliberate scrubber pipeline — a header allowlist
plus regex rules for provider bodies, JWTs, MercadoPago credentials, bearer and
basic values, named secrets, emails, phone numbers, and 13-to-19-digit runs. It
passes `request.URL` and `request.QueryString` through those rules
(`sentry.go:188-189`).

**No rule matches a bare hyphenated UUID.** Orchestrator-verified against the
pattern list. So any error captured on those three routes ships the
`booking_id` to a third party verbatim in the request URL, and
`cmd/api/cron.go:185` sends it explicitly in a captured message.

What makes this the sharp finding rather than an incidental one: that scrubber
list is evidence somebody already reasoned carefully about what counts as a
secret in this system. A booking id was never on it — correctly, because for
every other use in the codebase it is a primary key and nothing more. **The
defect is not that the list is careless. It is that one value is a primary key
and a bearer credential at the same time, and no single policy can be right for
both.**

## Milder than scoped: application logs are clean

`internal/middleware/logging.go:304-313` logs method, route, a sanitized path,
status, duration, bytes, request id and IP — **no query string**. The
`booking_id` does not reach structured request logs. Recorded because "log
lines" was named as a place to check and it turned out clean, and this program
has previously both overstated and understated a finding.

## Blast radius, stated precisely

Scoped to one booking. `GetByID` returns exactly one row for one UUID; there is
no cross-tenant or enumeration path.

- `PublicStatus` — `status` and `payment_status` for that booking.
- `PublicCancelInfo` — additionally date, start time, court and complex name,
  and the refund eligibility fields. No client PII: no name, phone or email. It
  confirms to a third party that a specific reservation exists.
- `PublicCancel` — cancels the booking and, when a refund is owed, dispatches
  `AutoRefundIfPaid`.

**An attacker gains nothing financially.** The refund resolves the booking's own
stored payment and returns money to whoever originally paid, through the
original method. A leaked link cannot redirect money.

The harm is denial, and it is asymmetric across the refund window:

- **Inside the window** the legitimate client is refunded automatically but
  loses a slot they never meant to give up. An inconvenience.
- **Outside the window** they lose the slot **and** the deposit, with no
  recourse. Real financial harm to the victim, for no gain to the attacker
  beyond griefing.

Damage is capped at one event: `PublicCancel` already refuses a second cancel on
a cancelled, completed or no-show booking (`public.go:530-533`), and
`ClaimRefund`'s atomic predicate — hardened by the archived
`refund-durability-and-collector-integrity` change — prevents a double refund. A
leaked link cannot be replayed.

## What already constrains it

- **Rate limiting is narrower than its name suggests.** `bookingCeiling`
  (1 request per 20s, burst 3) matches `r.URL.Path == "/api/v1/book"` exactly,
  and only for POST (`ratelimit.go:98`). The three routes here fall through to
  the general per-IP ceiling. Orchestrator-verified. It is close to irrelevant
  to the threat model — a UUIDv4 is unguessable, so this bounds abuse of an
  already-leaked id, not discovery of one.
- **The CSRF exemption for the `/api/v1/book` tree is correct**, not a gap: this
  flow never uses cookie auth, so there is nothing to forge.
- Migration 006's invariants do not touch this surface.

## Where the link is built

Two sites construct it independently, with a duplicated `fmt.Sprintf` shape:

- `internal/bookings/create.go:262-263` — staff booking with immediate payment
- `internal/payments/process.go:222-223` — the MercadoPago webhook confirming a
  public booking

Both put the id in a **query string**, on the email link and on the WhatsApp
template button. That matters beyond style: query strings ride into `Referer`
headers on any outbound request the cancel page makes, and into ordinary web
server and CDN access logs at the frontend. A path segment does not carry the
same way.

## The mechanism question — framed, not decided

**Replace** — the routes stop accepting `booking_id` and parse only an opaque
expiring token, resolving the booking from it.

**Accompany** — the routes keep `booking_id` and additionally require a valid
unexpired token bound to that id.

Cost is near-identical, and accompany is arguably *more* code: it must verify
the token belongs to the presented id, a check replace gets for free because
resolving the booking **is** the token lookup.

They differ in what keeps appearing in URLs, `Referer` headers and Sentry after
the fix. Replace means the primary key never crosses the public trust boundary
again; what leaks is a value designed to be disposable with a defined lifetime.
Accompany leaves the primary key in every one of those places exactly as today,
permanently paired with a secret that expires — the double-duty problem survives,
merely gated.

**Nothing is deployed**, so there are no links in the wild. Replace is free
today in a way it will never be again: no dual-accept window, no outstanding
links to honour, no cutover. Once real links exist, replace either breaks every
one of them on deploy day or requires temporarily accepting both forms.

The exploration recommends replace, and the reason is that it is the only option
that stops a primary key from being a bearer credential at all — which is the
property the owner's own reasoning depends on.

Storage should follow the existing durable time-bound marker idiom rather than
invent one: `RefundIntentAt`, established by the archived
`refund-durability-and-collector-integrity` change, is the precedent.

## Open questions for the proposal

- **Token lifetime policy.** Tied to the booking's date with a buffer, or a flat
  TTL from creation? It changes what "expired while a legitimate client was
  cancelling" looks like, not just what an attacker sees.
- **What a holder of an expired link sees.** Today a bad id gets a bare 404 or
  400. An expired token hitting the same generic 404 is a dead end with no
  stated recourse — "the link doesn't work" reads as a bug and becomes a support
  call to the venue. Product-adjacent; surfaced, not decided.

## Risks

- **The Sentry gap is not closed by either mechanism on its own.** If the new
  token is not added to the scrubber rules, or if an error path logs the
  resolved `booking_id` instead of the token, the fix is cosmetic. Design should
  treat "what Sentry sees on these three routes afterwards" as an explicit
  acceptance criterion rather than an assumed side effect.
- Link construction is duplicated across two call sites; collapse it into one
  helper while touching both, so token minting does not become a third copy.
- The expiry failure mode — 410 versus 404 versus a softer state — changes the
  response contract and should be settled before the spec is written.
