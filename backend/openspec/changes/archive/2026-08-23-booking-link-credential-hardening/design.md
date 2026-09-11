# Design: Booking Link Credential Hardening

## Technical Approach

Two slices in the proposal's order, and the order is load-bearing for a reason the
proposal states but does not quite implement: slice 1 must be invisible to the
frontend. So slice 1 mints, stores and emits a token **while `booklink` still
builds `?booking_id=`**; slice 2 changes the parameter, the authorization, the
`410` and the response bodies in one cut. See *Slice boundary*, which is the one
place this design corrects the proposal's wording.

Nothing is invented. The table copies `email_verification_tokens`; the mint-and-
hash copies `auth/tokens.go:97-108`; the hand-written store statements copy
`refund_intents.go`; the "one expression, not two" liveness rule copies the
`owesRefund` property the archived refund-durability change established.

## Architecture Decisions

### Decision 1 — a 1:N `booking_link_tokens` table, minted per link, hash-at-rest

```sql
-- db/migrations/011_booking_link_tokens.sql (+goose Up)
CREATE TABLE booking_link_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_booking_link_tokens_booking ON booking_link_tokens (booking_id);
CREATE INDEX idx_booking_link_tokens_expires ON booking_link_tokens (expires_at);
-- Deliberately NOT constrained (006's convention: a constraint missing on purpose
-- and one missing by omission look identical in a dump): no "every booking has a
-- token" trigger. Production has exactly one insert path and it mints inside its
-- own transaction; a DEFERRABLE trigger would additionally fire on the test-only
-- BookingModel.Insert and on any future admin import, for no production gain.
```

**Minted**, not derived: 32 bytes from `crypto/rand`, `base64.RawURLEncoding`
(43 chars). Stored as `sha256.Sum256(plaintext)`, exactly `hashRefreshToken`.

| Option | Tradeoff | Verdict |
|---|---|---|
| `uuid.New().String()` — the literal house idiom (`tokens.go:100`) | 122 bits, and **UUID-shaped**: a link token would be indistinguishable from a booking id in any URL, log or paste — the precise confusion this change exists to end | rejected |
| 32 random bytes, base64url | 256 bits, visibly not a UUID, `crypto/rand` returns an error where `uuid.New()` panics | **chosen** |
| bcrypt/argon2 at rest | Stretching defends low-entropy secrets; 256 uniform bits are not brute-forceable, and a per-request KDF on a polled status route is pure cost | rejected |
| A column on `bookings` | Commits atomically for free, but welds a credential into the row every staff list, JOIN and sqlc model carries, and makes reissue a schema change | rejected |
| Signed stateless HMAC/JWT | Already rejected in the proposal (no revocation). Its variant `token = HMAC(secret, booking_id)` would let any process recompute the plaintext — see below — but one leaked secret forges every link | rejected |

**Why 1:N rather than one token per booking, which is not a free choice.** With
hash-at-rest the plaintext is unrecoverable, so *every* process that emits a link
must mint. `internal/payments/process.go:222` runs hours after the booking row was
inserted, in a different request; it cannot read back a token PublicBook minted.
Either the plaintext becomes reversible at rest (the `crypto.Keyring` idiom used
for `mp_access_token`) or the table is 1:N. 1:N is chosen, and it is the same
shape `refresh_tokens` already has for the same reason. Consequence, stated
plainly: a public booking ends up with **two** live tokens — one in the checkout
`back_urls`, one in the confirmation email — both resolving to the same booking
with the same expiry. Not deleting the checkout one at confirmation is
deliberate: MercadoPago redirects the browser to the success `back_url` at
roughly the moment the webhook fires, so revoking it would break the success page.

**Expiry: stored, and its input is immutable.** `expires_at = booking.EndsAt() +
buffer`, from a new `-booking-link-token-buffer` flag (default 24h) beside
`booking-payment-expiry` (`main.go:242`). The inputs are `date`, `end_time` and a
config constant. Rescheduling does not exist (`handlers.go:185-190`), so this
cannot go stale — **and, unlike a derived expiry, it does not depend on
`complexes.cancellation_hours`, which is mutable at any time
(`complexes/handlers.go:271`).** That mutability is the coupling that would have
made deriving wrong; the invariant below handles it instead. A future reschedule
route must recompute `expires_at` for the booking's live tokens in the same
transaction that moves the booking — that is the coupling to revisit.

### Decision 2 — the refund invariant is a tautology, not an arithmetic accident

**The proposal's stated ground is false, and this matters.** It claims both
`CanRefund` deadlines fall strictly before booking start. `refund.go:36` reads
`if cancellationHours <= 0 { return true }`, and migration 006 permits
`cancellation_hours = 0` (`:170-171`, range `0..168`, validated identically at
`complexes/handlers.go:79`). For such a complex `CanRefund` is **unconditionally
true forever** — no deadline exists to fall anywhere. Separately, the grace branch
(`:44`) is anchored to `CreatedAt`, not to start. So "end + buffer dominates both
deadlines" is a numeric coincidence that a legal configuration already breaks.

The fix is to stop comparing numbers:

```go
// internal/pricing/refund.go, immediately below CanRefund.
//
// The right-hand term is the same call the refund dispatch makes. "Expired"
// therefore cannot be true while a refund is owed — not because end+buffer
// happens to sit after both deadlines, but because A implies (A or B) for every
// value of every number in this file, including cancellationHours == 0 and a
// negative buffer.
func LinkLive(b *data.Booking, expiresAt time.Time, cancellationHours int,
    grace time.Duration, now time.Time) bool {
    return now.Before(expiresAt) || CanRefund(b, cancellationHours, grace)
}
```

`LinkLive` uses `CanRefund`, not `owesRefund` (`public.go:550`), so it is strictly
weaker than the condition that dispatches money: it can never be false when a
refund is owed. It lives in the same file, four lines below the deadlines it
depends on, so an edit to either is an edit a reader is looking at.

A `cancellation_hours = 0` complex gets non-expiring links. That is exactly what
that setting means — "cancel and refund at any time" — and is the venue's own
choice, not a hole.

**Rejected: bake the expiry into the lookup SQL**, as
`email_verification.sql:9` does (`AND expires_at > NOW()`). It makes `410`
unrepresentable: the row vanishes and "expired" collapses into "unknown", which
is exactly why `auth/handlers.go:266` can only answer a generic
`invalid or expired`. `ResolveBooking` therefore has **no expiry predicate**; Go
decides. **Rejected: enforce it at mint time by taking `max(end+buffer,
refundDeadline)`** — that couples the stored value to mutable
`cancellation_hours` and goes stale the moment an owner lowers it.

### Decision 3 — `internal/booklink`, four shapes, one parameter name

A new leaf package (imports `fmt` and `net/url`, nothing else). `internal/mp`
must not import `internal/bookings`, so URL construction moves **out** of `mp`:

```go
package booklink

// QueryParam is the name the three public routes read and every link writes.
// It must stay exactly "token": cmd/api/sentry.go's named-secret rule ends its
// alternation with a bare `token`, and the \b before it cannot match inside
// `booking_token` because an underscore is a word character (sentry.go:92-94).
// Pinned by TestBookingLinkURLIsScrubbed, not by this comment.
const QueryParam = "token"

func Cancel(frontendURL, complexSlug, credential string) string
func CancelPath(complexSlug, credential string) string   // relative, WhatsApp button
func Success(frontendURL, complexSlug, credential string) string
func SuccessPending(frontendURL, complexSlug, credential string) string
func Failure(frontendURL, complexSlug string) string      // no credential
```

`credential` is an opaque `string` on purpose: slice 1 passes
`booking.ID.String()`, slice 2 passes `booking.LinkToken`, and the signatures do
not move between slices.

`mp.CreatePreferenceInput` gains `BackURLs mp.BackURLs{Success, Failure, Pending
string}` and **loses** `FrontendURL` and `ComplexSlug` — `:304-306` was their only
use, so `internal/mp` builds no client-facing URL at all and gets smaller.
`BookingID` stays: `external_reference` and `metadata.booking_id` (`:322-324`) are
correlation for a signature-authenticated webhook, not a credential.

### Decision 4 — Sentry, pinned by two tests and an audit

**(a) The name.** `TestBookingLinkURLIsScrubbed` in `cmd/api/sentry_test.go`
(package `main`, where `scrubText`/`scrubEvent` are reachable) builds a real URL
with `booklink.Cancel(...)` and a known token, runs it through `scrubEvent` as
`Request.URL` and `Request.QueryString`, and asserts the plaintext is absent.
Renaming `QueryParam` to `booking_token` changes the built URL, the `\btoken\b`
rule stops matching, the plaintext survives, and the test fails. A comment cannot
do that; this does.

**(b) Audit result, and it is a clean one.** Every `sentry.Capture*` in
`internal/bookings` is in `PublicBook` (`public.go:243`, `:657`) and emits
`complex_id`, never `booking_id`. `PublicStatus`, `PublicCancelInfo` and
`PublicCancel` reach Sentry through nothing: `respond.ServerError` →
`Responder.LogError` → `slog` only, and `middleware.RecoverPanic` does not import
`sentry`. So no event from these three routes carries a `Request` today at all —
`scrubRequest` is defensive infrastructure for a capture path that does not yet
exist on them. Criterion (c) currently holds vacuously; the risk is future code.
Pinned by `TestPublicRoutesCaptureNothing` in `internal/bookings`, reusing the
existing `captureTransport` (`sentry_capture_test.go`): force a store failure on
each of the three routes and assert `count() == 0`. Adding a
`CaptureMessage(... booking_id=%s ...)` to any of them breaks it.

### Slice boundary — the one correction to the proposal

The proposal calls slice 1 "purely additive: the three routes still accept
`booking_id`, so nothing is broken between slices", while also placing the link
helper and its `?token=` output in slice 1. Both cannot hold: emitting `?token=`
against routes that read `booking_id` kills every link for the duration.

Resolved by keeping the *credential* swap whole in slice 2. Slice 1 collapses the
three construction sites into `booklink` **still emitting `booking_id`**, mints
and stores the token, and adds it to `PublicBook`'s body. Slice 1 is then
genuinely invisible to the frontend — no dual-accept branch is written and then
deleted, and the "exactly one function constructs a booking link" criterion is
already met when slice 2 changes one constant.

## Data Flow

```
staff create / public book
  InsertSafe(tx) ── booking INSERT ─→ token INSERT (same tx) ─→ b.LinkToken (json:"-")
        │                                    expires_at = EndsAt() + buffer
        └─→ booklink.Cancel/Success/… ─→ email · WhatsApp · mp.BackURLs

mp webhook confirms
  InsertAndConfirmBooking ─→ links.Mint(bookingID, EndsAt()+buffer) ─→ booklink.Cancel
        (mint fails → webhook errors → webhook_events redelivers; no email sent)

GET/POST /api/v1/book/{status,cancel-info,cancel}?token=…
  resolveLink ─→ ResolveBooking(sha256(token))   -- no expiry predicate
        ├─ no row ─────────────────────→ 404
        ├─ row ─→ complexes.GetByID ─→ pricing.LinkLive(...)
        │            ├─ false ────────→ 410 + recourse
        │            └─ true  ────────→ handler body (booking + complex in hand)
```

## File Changes

| File | Action | Slice | Description |
|---|---|---|---|
| `db/migrations/011_booking_link_tokens.sql` | Create | 1 | Table + two indexes; down drops the table |
| `internal/data/booking_link_tokens.go` | Create | 1 | `BookingLinkToken`, `BookingLinkTokenModel`, `Mint`, `ResolveBooking`, `DeleteExpiredTerminal` — hand-written SQL, `refund_intents.go` precedent |
| `internal/data/bookings.go` | Modify | 1 | `Booking.LinkToken string \`json:"-"\``; `EndsAt()`; `InsertSafe` mints inside its tx (`:174-204`); `Insert` (`:78`) gains a test-only-and-tokenless comment |
| `internal/data/models.go` | Modify | 1 | `Models.BookingLinkTokens`; `Config.LinkTokenBuffer` → `BookingModel` |
| `internal/booklink/booklink.go` | Create | 1 | `QueryParam` + five URL functions |
| `internal/bookings/create.go` | Modify | 1 | `:262-263` → `booklink` |
| `internal/bookings/public.go` | Modify | 1 | `:307-324` passes `mp.BackURLs`; `:304` response gains `token` |
| `internal/payments/process.go` | Modify | 1 | `:222-223` → mint + `booklink` |
| `internal/mp/mp.go` | Modify | 1 | `:303-307` consumes `BackURLs`; drops `FrontendURL`/`ComplexSlug`; `:322-324` unchanged |
| `cmd/api/main.go`, `app.go` | Modify | 1 | `-booking-link-token-buffer` (24h) wired to `data`, `bookings`, `payments` |
| `cmd/api/cron.go` | Modify | 1 | `job("sweep_booking_link_tokens", …)` — deletes only tokens whose booking is terminal *and* whose `expires_at` is past retention, so a sweep can never turn a live link into a `404` |
| `internal/pricing/refund.go` | Modify | 2 | `LinkLive` below `CanRefund` |
| `internal/booklink/booklink.go` | Modify | 2 | `QueryParam` → `"token"`; call sites pass `LinkToken` |
| `internal/bookings/public.go` | Modify | 2 | `resolveLink` helper; `:382-414`, `:418-482`, `:494-640` resolve by token; `410`; all four bodies stop emitting `booking.ID` |
| `internal/bookings/bookings.go` | Modify | 2 | `Store` gains the consumer-side link interface; `:210-212` doc comment |
| `cmd/api/sentry.go` | **Unmodified, pinned** | 2 | `sentry_test.go` proves the real URL redacts |
| `cmd/api/cron.go:185` | **Unmodified** | — | Deliberate; see proposal *Out of Scope* |

## Interfaces / Contracts

```go
// internal/data — top-level Models field, matching EmailVerification/PasswordReset
// rather than composing into BookingStore: these are a credential's lifecycle,
// not a booking's (ISP).
type BookingLinkTokenStore interface {
    Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (plaintext string, err error)
    // ResolveBooking returns the enriched booking and the token's expiry in one
    // JOIN, mirroring GetByID's hand-written SELECT. ErrRecordNotFound means no
    // row carries that hash — never that the row expired.
    ResolveBooking(ctx context.Context, plaintext string) (*Booking, time.Time, error)
    DeleteExpiredTerminal(ctx context.Context, retention time.Duration) error
}

// internal/bookings — consumer-declared, one method, like Refunder.
type LinkResolver interface {
    ResolveBooking(ctx context.Context, plaintext string) (*data.Booking, time.Time, error)
}

// internal/bookings/public.go — the whole authorization for the three routes.
// Each handler keeps its own absent/malformed shape (400 on the GETs, 422 on the
// POST) per the proposal's Out of Scope; this owns only 404 / 410 / 500.
func (h *Handler) resolveLink(w http.ResponseWriter, r *http.Request, token string) (*data.Booking, *data.Complex, bool)
```

`resolveLink` returns the complex it had to load for `LinkLive`, so
`PublicCancelInfo` (`:443`) and `PublicCancel` (`:536`) drop their own
`complexes.GetByID` — net query count unchanged for two routes, `+1` for
`PublicStatus`, and roughly 30 lines of triplicated parse-and-switch deleted.

## Testing Strategy

`rules.evidence: mutation-verified` — each row names the mutation that must break it.

| Layer | What | Mutation that must fail |
|---|---|---|
| Unit `pricing` | Matrix over `cancellationHours ∈ {0,1,24,168}` × `grace ∈ {0,15m,72h}` × `expiresAt ∈ {past, future}` × booking past/future: `CanRefund == true` ⇒ `LinkLive == true` | replace the disjunct with `now.Before(expiresAt)` — fails on every `cancellationHours == 0` row and every past-`expiresAt` grace row |
| Unit `data` | `InsertSafe` on a stubbed failing token INSERT leaves **no** booking row; `b.LinkToken` is 43 chars and never equals `b.ID.String()` | move the mint outside the tx |
| Unit `data` | Two mints on the same booking produce different plaintexts and both resolve | reuse a per-booking constant |
| Unit `booklink` | Every function emits `?token=` and nothing else identifying | rename `QueryParam` |
| Unit `cmd/api` | `TestBookingLinkURLIsScrubbed`: `scrubEvent` on `Request.URL = booklink.Cancel(...)` leaves no plaintext | rename `QueryParam` to `booking_token` |
| Unit `bookings` | `TestPublicRoutesCaptureNothing`: forced store error on each route ⇒ zero Sentry events | add `CaptureMessage(... booking_id=%s ...)` to any of the three |
| Unit `bookings` | Unknown token ⇒ 404; expired token ⇒ 410 with a non-empty distinguishable body; **expired token on a refund-eligible booking ⇒ not 410, and the refund dispatches** | check expiry before resolving, or drop the `CanRefund` disjunct |
| Unit `bookings` | A booking id presented as `token` authorizes nothing (404); none of the four bodies contains the UUID | keep `GetByID` as a fallback |
| Integration | `ResolveBooking` on an expired row returns the row plus its past `expires_at`, not `ErrRecordNotFound` | restore `AND expires_at > NOW()` |
| Integration | Sweep leaves tokens of `confirmed`/`pending` bookings untouched however old | drop the terminal-status predicate |

## Threat Matrix

N/A — no shell, subprocess, VCS/PR automation, executable-file classification or
process integration. Routing changes are confined to the query parameter three
existing routes read; no route is added, removed or re-guarded.

## Migration / Rollout

No backfill: nothing is deployed, no link exists in the wild, no dual-accept
window. Migration `011` is a new empty table — instantaneous, and the only
post-launch cost note is that `token_hash UNIQUE` would want
`CREATE UNIQUE INDEX CONCURRENTLY` (006's note), which does not apply here.

**sqlc.** This design adds no `db/queries/*.sql` entry — `ResolveBooking` needs
`GetByID`'s enrichment JOIN, which is already hand-written raw SQL
(`bookings.go:215`), so all three statements follow `refund_intents.go` and stay
hand-written. But `make sqlc` reads `db/migrations/`, so the next person who runs
it for any reason will emit a `BookingLinkToken` model struct. Run it **once, in
its own commit containing only generated output**, at the end of slice 1: that
commit also carries the known stray diffs — the `WebhookEvent` struct migration
006 documented, plus the `JobLock` struct and the `RefundAmount` type change the
previous change found were also in it. Name all four in the commit message so
none is mistaken for scope.

**Frontend, separate repository, and this repo cannot verify any of it.** Slice 1
is invisible to it (same URLs; one new ignorable `token` field in `POST
/api/v1/book`). Slice 2 is the cutover and requires, before it merges: read
`?token=` instead of `?booking_id=` on `/{slug}/book/cancel` and
`/{slug}/book/success`; send `{"token": …}` to `POST /api/v1/book/cancel`; stop
reading `booking.id` from all four bodies and carry the `token` from the
`PublicBook` response instead; render the `410` distinctly from the `404`. The
only in-repo evidence is a passing slice-2 suite; deploy order is slice 1 → any
time → frontend → slice 2.

**Rollback.** Slice 2 is a pure code revert to accepting `booking_id`. Slice 1
reverts as a code revert plus `migrate-down` on `011`; nothing else reads the
dropped table.

## Open Questions

- [ ] **Owner question 1 (buffer).** Designed as a flag defaulting to 24h after
      booking end. Changing it is a one-word config change and, because of
      `LinkLive`, cannot endanger a refund at any value — including zero or
      negative.
- [ ] **Owner question 2 (recourse).** The `410` body's exact copy is the spec's.
      Design requires only that it be non-empty, distinguishable from the `404`,
      and name a next step; no reissue path exists and none is added.
- [ ] **Owner question 3 (bodies stop returning the id).** Assumed nothing
      outside this repo needs it. Must be confirmed before slice 2, not before
      slice 1.
- [ ] **`cancellation_hours = 0` means links never expire for that complex.**
      A consequence of the invariant, not a bug, but somebody should know it is
      reachable through the settings screen today.

---

## Frontend contract — the exact field set, added at verify

Verification found this document's coordination note named only the `token`/`id`
swap, while the implementation replaced `POST /api/v1/book`'s whole booking
struct with an explicit envelope. That drops more than the primary key, and a
frontend reading any of the removed fields breaks on a deploy that this
repository cannot fail.

Naming them is the point. "The response envelope changed" is not something a
frontend developer can act on; a field list is.

**What `POST /api/v1/book` returns now:**

```json
{
  "booking": {
    "status": "...", "payment_status": "...",
    "date": "2026-08-23", "start_time": "18:00", "end_time": "19:30",
    "court_name": "...", "complex_name": "...",
    "price": 0, "deposit_amount": 0
  },
  "token": "..."
}
```

**Gone, beyond `id`:** `complex_id`, `court_id`, `client_id`,
`duration_minutes`, `reminder_sent_2h`, `notes`, `created_by`, `created_at`,
`updated_at`, `version`.

The narrow envelope is deliberate rather than incidental. A public endpoint that
marshals a domain struct leaks every field anybody adds to that struct later,
without a decision being made — an explicit envelope forces one each time. But
that property is only worth having if the removals are stated, so any field the
frontend genuinely needs gets added back on purpose.

**Before slice 2 reaches a live environment**, the frontend team confirms it
reads none of the ten. If it needs one, add it to the envelope deliberately;
do not restore the struct.
