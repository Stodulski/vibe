# Proposal: Booking Link Credential Hardening

## Intent

A UUIDv4 is unguessable. This was never a brute-force exposure, and nothing
below argues that it was.

The defect is that **one value is a primary key and a bearer credential at the
same time, and no single handling policy can be right for both.** Three public
routes take `booking_id` and nothing else
(`internal/bookings/bookings.go:222-224`), parse it, and call `GetByID`
(`internal/data/bookings.go:208-227`, `WHERE b.id = $1 LIMIT 1` — no expiry, no
status, no tenant predicate). That is the whole authorization. Every other use
of the same value in this codebase is an ordinary primary key.

`cmd/api/sentry.go` is the proof, and it is the reason this is worth doing
rather than worth noting. Its scrubbers cover JWTs (`:76`), MercadoPago
credentials (`:83`), bearer and basic values (`:88`), named secrets (`:98-99`),
emails (`:106`), phones (`:113`) and card-length digit runs (`:120`), and
`scrubRequest` runs `request.URL` and `request.QueryString` through all of them
(`:188-189`). **No rule matches a bare hyphenated UUID.** That list is evidence
somebody already reasoned carefully about what counts as a secret here. A
booking id was never on it, correctly — because everywhere else it is just a
primary key. The list is not careless; the value is doing two jobs.

The owner chose **expiry** on 2026-08-21 (`state.yaml`). This change implements
it, and does not reopen it.

### Severity, stated precisely

**An attacker gains nothing financially.** `AutoRefundIfPaid` resolves the
booking's own stored payment and returns money to whoever originally paid, by
the original method. A leaked link cannot redirect money.

The harm is denial, and it is asymmetric across the refund window
(`internal/pricing/refund.go:35-56`):

- **Inside the window** the client is refunded automatically but loses a slot
  they never gave up. An inconvenience.
- **Outside the window** they lose the slot **and** the deposit, with no
  recourse. Real money lost by a real client, for no gain to the attacker
  beyond griefing.

Damage is capped at one event, and the two guards that cap it are already
correct: `PublicCancel` refuses a second cancel on a terminal booking
(`internal/bookings/public.go:530-533`), and `ClaimRefund`'s atomic predicate —
hardened by the archived `refund-durability-and-collector-integrity` change —
prevents a double refund. Neither is re-proposed here.

Two other things verified as already correct and deliberately left alone: the
CSRF exemption on the `/api/v1/book` tree (`internal/middleware/chain.go:353`,
pinned by `chain_hardening_test.go:142-146`) is right, because that flow never
uses cookie auth and there is nothing to forge; and application request logs are
clean — `internal/middleware/logging.go:304-313` logs a sanitized path with no
query string, so `booking_id` never reaches structured logs.

`bookingCeiling` does not cover these three routes (`ratelimit.go:98` matches
`/api/v1/book` exactly, POST only). Confirmed, and close to irrelevant: it would
bound abuse of an already-leaked id, not discovery of one. Not addressed here.

## Scope

### In Scope

- **An opaque, expiring, hash-at-rest access token that replaces `booking_id` on
  the three public routes.** Resolving the booking *is* the token lookup.
- **Collapsing link construction into one helper.** There are **three** sites,
  not the two the exploration found — see Approach.
- **The expired-link response contract**: a distinguishable `410`, not the
  current generic `404`.
- **What Sentry sees on these routes afterwards**, as an acceptance criterion
  rather than an assumed side effect (`state.yaml`
  `acceptance_criterion_design_must_carry`). This includes the token's query
  parameter *name*, which is load-bearing — see Approach.
- **The public response bodies stop returning the booking primary key.**
  `PublicStatus` (`public.go:409`), `PublicCancelInfo` (`:468`), `PublicCancel`
  (`:633`) and `PublicBook` (`:304`, which returns the entire `booking` struct)
  all hand the id back to the browser today. Replacing the credential on the way
  in while continuing to emit it on the way out would be half a fix.

### Out of Scope

- **Rate limiting these three routes.** A UUIDv4 and a random token are both
  unguessable; a ceiling bounds abuse of a leaked value, not its discovery.
  Independent of this change and not what expiry is for.
- **Adding a UUID rule to the Sentry scrubbers.** Rejected on its own merits, not
  merely as redundant: a `[0-9a-f]{8}-…` rule would redact *every* legitimate
  diagnostic booking id in every captured error string and breadcrumb, and
  locating a bug from a Sentry event is exactly what those ids are for. The
  correct fix is that the value crossing the boundary stops being a credential.
- **`cmd/api/cron.go:185`**, which sends `booking_id=%s` in a `CaptureMessage`.
  Deliberately unchanged. That line becomes harmless *because the id stops being
  a credential*, not because anything about it changes. It is an operator
  diagnostic on the preference-expiry path, which never sees a token.
- **The 400-vs-422 inconsistency on malformed input.** The GET routes answer
  `400` (`public.go:386,392`) and the POST route `422` (`:508,515`) for the same
  class of error. Pre-existing, cosmetic, unrelated to the credential defect, and
  changing a status code is its own contract change. The token swap preserves
  each route's existing shape.
- **Dual-accept windows, cutover campaigns, backfills.** Nothing is deployed;
  no links exist in the wild.
- **Re-proposing the idempotency and double-refund guards.** Verified correct
  above.

## Capabilities

### New Capabilities

- `booking-link-credential`: what authorizes an unauthenticated client to read
  or cancel their own booking, how long that authorization lives, and what a
  holder of an expired one is told.

### Modified Capabilities

- None. The three routes' behaviour on a *valid* credential is unchanged; only
  what counts as one, and how long it counts, changes.

## Approach

### Decision 1 — Replace, not accompany

**Agreed with the exploration, for its reason plus one it did not have.**

Accompany is more code, not less: it must additionally prove the token belongs to
the presented id, a check replace gets for free because resolving the booking
*is* the lookup. And accompany gates the double duty rather than ending it — the
primary key stays in every URL, `Referer` and Sentry payload exactly as today,
permanently paired with a secret that expires.

The reason the exploration did not have: **there is a third construction site.**
`internal/mp/mp.go:304` and `:306` build `?booking_id=` into the MercadoPago
preference's `back_urls` (success and pending). That URL is handed to a third
party and then to the client's browser, and it is what feeds `PublicStatus`
after checkout. Under accompany, all three sites keep emitting the primary key.
Under replace, none does.

Nothing is deployed, so replace costs nothing today and stops being free on the
first deploy.

**One precision the review should hold me to.** Replace does *not* mean the
booking id never leaves the process. `mp.go:322-324` sends it as
`external_reference` and `metadata.booking_id`, and that is correct and stays:
it is a correlation identifier for a **signature-authenticated** webhook, not a
credential anyone can present. What ends is the primary key's role as an
*authorization credential on public routes*. Overstating this as "the id never
crosses the boundary again" would be false.

**Rejected: a signed stateless token (HMAC or JWT).** No revocation, no
`used_at`, no way to invalidate one link, and no precedent in this repo — while
the stored-hash idiom has three (see below).

### Decision 2 — Lifetime tied to the booking, not a flat TTL

**The deciding fact is that the link is minted once and never re-sent.**
Verified: it is built at `internal/bookings/create.go:262-263` (staff booking
with immediate payment) and `internal/payments/process.go:222-223` (the webhook
confirming a public booking), and nowhere else. The two-hour reminder carries no
cancel link (`internal/notifications/tasks.go:66-73`), and neither does the
cancellation or refund notice (`:77-90`). So the token must survive from mint
until the client would plausibly use it, with no second chance to reissue.

**There is no maximum-advance-booking limit** anywhere in config — the only
booking-time flag is `booking-payment-expiry`, a 15-minute unpaid hold
(`cmd/api/main.go:242`). So creation-to-play is unbounded. A flat TTL therefore
has to be either long enough to cover the longest booking — at which point it is
not an expiry — or short enough to silently kill links for far-ahead bookings,
which is precisely the cohort with the most time to change their mind and the
strongest claim to a refund.

**Decision: the token expires a configured buffer after the booking's end time.**
After the *end*, not the start, so a late arrival or a no-show can still read
status. Design picks the number; the proposal's recommendation is a flag
defaulting to 24h.

**Why not expire at the cancellation deadline**, which looks tighter and safer:
because `PublicCancel` deliberately still cancels outside the refund window
(`public.go:486-487`, `:594-599`) so the venue can resell the slot and the client
need not show up. Expiring at the refund deadline would delete a supported
product path and convert it into a call to the venue.

**The invariant this buys, which is the answer to "what happens if a link
expires mid-cancellation":** both refund deadlines in `CanRefund`
(`pricing/refund.go:44` grace-from-creation, `:54` `cancellationHours` before
start) fall strictly before the booking's start, hence strictly before its end
plus any non-negative buffer. **Token expiry can never be the binding constraint
on a refund-eligible cancellation.** By construction, not by tuning. Design
should carry this as a stated property with a test, because it is the whole
product answer to the lifetime question.

**Rejected: single-use, invalidated on cancel.** `PublicStatus` and the
post-checkout success page reuse the same link, `PublicCancel` is already
idempotent, and a client who refreshes would get a dead link. It buys nothing
and breaks a normal interaction.

**A verified negative, recorded so nobody re-derives it.** A stored `expires_at`
would go stale if a booking were rescheduled. **Rescheduling does not exist**:
the staff update route accepts only `status`, `payment_status`, `notes` and
`version` (`internal/bookings/handlers.go:185-190`). So storing `expires_at`
cannot go stale today, and the choice between storing it and deriving it from
the booking at read time is free. Recommend storing it, matching the three
existing token tables. If a reschedule route is ever added, a stored expiry is
the coupling that will be wrong.

### Decision 3 — `410 Gone`, distinguishable, with a stated recourse

Today an unknown id gets a bare `404` (`public.go:400`, `:435`, `:523`). An
expired token hitting the same `404` is a dead end: it reads as a bug, and it
becomes a support call to the venue — which is a cost this change would *create*.

`404` and `410` answer different questions. `404` says "no such thing, check what
you typed", which the client cannot act on. `410` says "this was real and is
finished", which they can. The `410` body must name the recourse; exact copy and
its language are the spec's, following each route's existing convention, but it
must be distinguishable from the unknown-token `404` and must not be silent
about what to do next.

`httpx.Responder.Error(w, r, status, message)` already exists (`errors.go:52`)
and `PublicCancel` already uses it (`public.go:531`), so no new helper and no
inline error JSON is needed — the house rule holds.

**The reflexive objection, answered.** Distinguishing "expired" from "unknown" is
normally an existence oracle. Here it leaks nothing: a caller who can present a
*syntactically valid, previously-issued* token already held one. With 122+ bits
of entropy, the distinction is only ever observable by someone who already had
the answer.

Unknown, malformed and absent tokens keep today's `404`/`400`/`422` shapes.
A *cancelled* booking's token stays valid for `PublicStatus`, so the client can
see that the cancellation and refund happened; `PublicCancel` continues to refuse
with the existing `400` on a terminal booking.

### Decision 4 — The Sentry work is in scope

`state.yaml` records it as the acceptance criterion, and it is the difference
between a real fix and a cosmetic one. Scoped in. Three parts:

**(a) The parameter name is load-bearing, and it is nearly free.** The existing
"named secret" rule (`sentry.go:96-101`) ends its alternation with a bare
`token`, and `scrubRequest` applies it to `request.URL` and `request.QueryString`
(`:188-189`). A query string reading `?token=<value>` is therefore **already
redacted** by a rule that exists today. But only if the parameter is named
*exactly* `token`: `\btoken\b` cannot match inside `booking_token`, because an
underscore is a word character — the file's own comment says so (`:92-94`). The
repo already delivers `?token=` this way for email verification and password
reset (`internal/auth/handlers.go:123`, `:748`). This is exactly the kind of
requirement that gets silently violated by a well-meant rename, so it belongs in
the spec as a requirement with a regression test, not in a comment.

**(b) No error path on these three routes may put the resolved `booking_id` into
a captured event.** Once the handler has resolved the token, the primary key is
in hand; a `ServerError` or a breadcrumb that includes it re-opens the exact
channel this change closes. Design must audit the capture path from these three
handlers and pin the result.

**(c) The acceptance criterion is stated as an observation, not an
implementation**: after this change, an error raised on any of the three routes
produces a Sentry event containing neither the booking's UUID nor the token's
plaintext.

### Storage — reuse the idiom, and reuse the right one

The exploration pointed at `RefundIntentAt` as the durable time-bound precedent.
The **closer** precedent is the three hashed-token tables already in
`db/migrations/001_initial_schema.sql`: `refresh_tokens` (`:51-58`),
`email_verification_tokens` (`:61-67`) and `password_reset_tokens` (`:70-76`) —
each `token_hash BYTEA` (unique on the latter two) plus `expires_at`, with the
established lookup shape `WHERE token_hash = $1 AND expires_at > NOW()`
(`db/queries/email_verification.sql:9`) and an expiry sweep
(`:23`). `internal/auth/tokens.go:97-108` is the mint-and-hash helper.

What `RefundIntentAt` contributes is not its shape but its **property**: written
by the same commit as the row it describes. The token must be minted no later
than the booking insert commits, so a crash cannot leave a booking whose link has
no token and no way to reissue one. Migrations top out at `010`, so this is
`011`.

Table-vs-column, and where the mint call sits relative to `InsertSafe`, are
design's.

### Link construction — one helper, three sites

`create.go:262-263` and `process.go:222-223` are byte-identical `fmt.Sprintf`
shapes; `mp.go:304`/`:306` is the third, in a different package. A fourth copy
for token minting would be the worst outcome.

The constraint this puts on design: `CreatePreferenceInput` needs **both** — the
token, for `back_urls`, and `BookingID`, which `external_reference` and
`metadata` legitimately still require (`mp.go:322-324`). And `internal/mp` must
not import `internal/bookings`. The cleanest resolution is to move URL
construction *out* of `internal/mp` and have its caller — `public.go:307-324`,
which already assembles `prefInput` — pass the finished back-URLs in. Design's
call; the requirement is that exactly one place knows the shape of a booking
link.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `db/migrations/011_*.sql` | New | Token storage, hash-at-rest, `expires_at` |
| `db/queries/*.sql`, `internal/db/` | New / Regenerated | Lookup by hash with the expiry predicate; expiry sweep |
| `internal/data/bookings.go` | Modified | Mint on insert; resolve-by-token alongside `GetByID` (`:208-227`) |
| `internal/bookings/public.go` | Modified | `:382-414`, `:418-482`, `:494-640` — token in, `410` out, id out of the response bodies |
| `internal/bookings/public.go:304` | Modified | `PublicBook` returns the token, not the whole booking |
| `internal/bookings/create.go` | Modified | `:262-263` → the shared helper |
| `internal/payments/process.go` | Modified | `:222-223` → the shared helper |
| `internal/mp/mp.go` | Modified | `:304`, `:306` back-URLs; `:322-324` unchanged |
| `cmd/api/sentry.go` | Unmodified, pinned | A regression test proves `?token=` is redacted and no UUID is emitted from these routes |
| `cmd/api/cron.go` | Unmodified | `:185` deliberately untouched — see Out of Scope |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| The token parameter is named anything but `token`, silently losing the existing Sentry rule | High if untreated | Spec requirement plus a regression test asserting `scrubText` redacts the actual URL these routes are called with |
| The resolved `booking_id` re-enters Sentry via an error path after a successful lookup | Med | Acceptance criterion (c); design audits the capture path |
| The frontend still sends `booking_id` and every public route breaks | High | Frontend is a separate repo and is not in this change's blast radius. `PublicBook` and the back-URLs must ship the token before the routes stop accepting the id — hence the slice order below |
| `make sqlc` emits the known stray `WebhookEvent` diff | Med | Isolate regeneration in its own commit, as `mp-oauth-credential-integrity` did |
| A stored `expires_at` goes stale | Low today | Rescheduling does not exist (`handlers.go:185-190`); recorded as the coupling to revisit if it is added |
| The `410` is read as an enumeration oracle in review | Med | Answered in Decision 3; carry the reasoning into the spec so it is not re-litigated |

## Rollback Plan

Slice 1 is additive — a new table, a minted token nothing yet requires, and a
helper — and reverts as a code revert plus migration `011` down; no other path
reads the dropped data. Slice 2 reverts to accepting `booking_id`. Nothing is
deployed, so no rollback has a live-traffic window and no link in the wild
breaks.

## Dependencies

- `refund-durability-and-collector-integrity` (archived) — supplies the
  already-correct double-refund guard this change relies on and does not
  re-propose. Satisfied.
- The frontend consumer of `?booking_id=` lives outside this repository. This
  change alters a public response contract (`PublicBook`, the three routes'
  bodies) and the back-URL shape. Coordination is required before the id stops
  being accepted; it is not required to land slice 1.

## Review Workload

Base 400, no 2x modifier (`state.yaml`). Estimated **~410 authored lines**, which
is over budget as one unit. Two chained slices, in this order:

1. **Mint, store and emit the token** (~230) — migration `011`, store methods,
   the single link helper across `create.go`, `process.go` and `mp.go`,
   `PublicBook` returning the token. Purely additive: the three routes still
   accept `booking_id`, so nothing is broken between slices.
2. **Switch the authorization** (~180) — the three routes resolve by token only,
   the `410` contract, the response bodies stop returning the id, and the Sentry
   acceptance tests.

`Decision needed before apply: No` · `Chained PRs recommended: Yes` ·
`400-line budget risk: Medium as two slices; High as one`

## Success Criteria

- [ ] A booking id presented to `/api/v1/book/{status,cancel-info,cancel}`
      authorizes nothing.
- [ ] A token whose booking has passed its end time plus the configured buffer
      answers `410` with a distinguishable, actionable message; an unknown token
      answers `404`.
- [ ] A refund-eligible cancellation can never be blocked by token expiry —
      proven against both `CanRefund` deadlines, including the grace-from-
      creation branch.
- [ ] An error raised on any of the three routes produces a Sentry event
      containing neither the booking's UUID nor the token's plaintext, proven by
      running the real request URL through `scrubEvent`.
- [ ] No public response body returns the booking's primary key.
- [ ] Exactly one function in the repository constructs a booking cancel link.
- [ ] A booking cannot commit without its token committing in the same
      transaction.
- [ ] `make test`, `golangci-lint run`, and the `integration`-tagged suite pass.

## Proposal question round

The four decisions above are made, not deferred. Three carry product judgement
the owner may want to correct before the spec pins them. Design should proceed on
these assumptions unless told otherwise:

1. **Lifetime buffer.** Assumed: the link dies 24h after the booking's *end*, so
   a no-show or a late arrival can still read status. Is 24h right, or should it
   be tighter (the link's remaining purpose after the game is only informational)
   or looser?
2. **Expired-link recourse.** Assumed: the `410` tells the client to contact the
   venue. Is there something better to offer — a way to reach the booking through
   the public site, or a self-service reissue? A reissue path would be a new
   capability and is out of scope as proposed.
3. **The public routes stop returning the booking id.** Assumed: nothing outside
   this repo needs it. If the frontend uses it for anything beyond passing it
   back to these routes, that use has to be named before slice 2.
