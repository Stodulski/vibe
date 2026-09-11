# Tasks: Booking Link Credential Hardening

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~230 (slice 1) + ~180 (slice 2) = ~410 total |
| 400-line budget risk | Medium as two slices; High as one unit |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (mint/store/emit, still `booking_id`) → PR 2 (flip the credential) |
| Delivery strategy | auto-chain (`state.yaml`) |
| Chain strategy | stacked-to-main — slice 1 is purely additive (rollback is a code revert plus `migrate-down` on 011); slice 2 reverts to accepting `booking_id`. Sequential merge-to-main carries no live-traffic risk since nothing is deployed |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

**Slice order is fixed and load-bearing, for a reason distinct from budget.**
Slice 1's `booklink` package must still emit `?booking_id=` — emitting `?token=`
against routes that still read `booking_id` kills every link for the slice's
duration (design, *Slice boundary*). Do not reorder, and do not let slice 1
flip `booklink.QueryParam` early.

### Suggested Work Units

| Unit | Goal | PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Migration 011, `booking_link_tokens`, mint-in-tx, `internal/booklink`, three sites collapsed — still emitting `booking_id` | PR 1 | `go test -race -v ./internal/data/... ./internal/booklink/... ./internal/bookings/... ./internal/payments/... ./internal/mp/...` | `make e2e-db-up && make test/integration && make e2e-db-down` — mint-atomicity and expiry-storage proofs need a real transaction | code revert + `make migrate-down` on 011; nothing else reads the new table |
| 2 | `LinkLive`, `QueryParam` flip, token-only resolution, `410`, id-free bodies, Sentry regression pair | PR 2, base=PR1 | `go test -race -v ./internal/pricing/... ./internal/bookings/... ./cmd/api/...` | `make e2e-db-up && make test/integration && make test/security && make e2e-db-down` — expiry-vs-refund proof and the unknown/expired distinction need real rows | pure code revert to accepting `booking_id`; no schema change in this slice |

## Phase 0: Re-sync with concurrent edits
- [x] 0.1 Re-read `internal/data/bookings.go` (`Insert`, `InsertSafe`, `GetByID`),
      `internal/data/models.go` (`Models`, `Config`, `NewModels`),
      `internal/data/errors.go` (`ErrRecordNotFound`), `internal/auth/tokens.go`
      (`generateRefreshToken`, `hashRefreshToken`), `internal/data/refund_intents.go`
      (hand-written-statement precedent), `internal/bookings/bookings.go` (`Store`
      interface, `Routes`), `internal/bookings/public.go` (`PublicBook`,
      `PublicStatus`, `PublicCancelInfo`, `PublicCancel`,
      `createMPPreferenceWithRetry`), `internal/bookings/create.go`,
      `internal/payments/process.go`, `internal/payments/payments.go`
      (`BookingStore`, `RefundIntentStore`, `Dependencies`), `internal/mp/mp.go`
      (`CreatePreferenceInput`, `CreatePreference`), `internal/pricing/refund.go`
      (`CanRefund`), `cmd/api/sentry.go` (the "named secret" rule),
      `cmd/api/main.go` (`cfg.booking` sub-struct, flag wiring),
      `cmd/api/app.go` (`newApplication`, `deps`), `cmd/api/cron.go`
      (`cronJobs`), `db/migrations/` for drift since design.md was written.
      Confirm every function cited below still has the shape this task file
      describes before editing anything. Confirm `db/migrations/` is still at
      `010_*` (011 is free).

---

# SLICE 1 — Mint, Store, Emit (PR 1, ~230 lines)

## Phase 1: `db/migrations/011_booking_link_tokens.sql`
- [x] 1.1 Write the migration: `booking_link_tokens` table —
      `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`, `booking_id UUID NOT NULL
      REFERENCES bookings(id) ON DELETE CASCADE`, `token_hash BYTEA NOT NULL
      UNIQUE`, `expires_at TIMESTAMPTZ NOT NULL`, `created_at TIMESTAMPTZ NOT
      NULL DEFAULT NOW()`; index on `booking_id`; index on `expires_at`. No
      "every booking has a token" trigger — deliberate, per design's Decision 1
      comment (mirrors migration 006's documented-omission convention). Down
      drops both indexes then the table.
- [x] 1.2 `make migrate-up` then `make migrate-down` against a local/E2E DB
      round-trips cleanly.

## Phase 2: `internal/data/booking_link_tokens.go` — hand-written store layer
- [x] 2.1 New file, precedent `internal/data/refund_intents.go`. Add
      `BookingLinkTokenModel` with three hand-written statements (no sqlc — the
      lookup needs `GetByID`'s enrichment JOIN):
      - `Mint(ctx, bookingID uuid.UUID, expiresAt time.Time) (plaintext string, err error)`
        — 32 random bytes via `crypto/rand`, `base64.RawURLEncoding`, hashed with
        `sha256.Sum256` (same shape as `generateRefreshToken`/`hashRefreshToken`
        in `internal/auth/tokens.go`, duplicated here rather than imported —
        `internal/data` must not depend on `internal/auth`), `INSERT INTO
        booking_link_tokens`.
      - `ResolveBooking(ctx, plaintext string) (*Booking, time.Time, error)` —
        hashes the plaintext, joins `booking_link_tokens` to the same enrichment
        SELECT `GetByID` uses, **no `expires_at` predicate** (design Decision 2
        — baking expiry into SQL makes `410` unrepresentable). `ErrRecordNotFound`
        means no row carries that hash, never that it expired.
      - `DeleteExpiredTerminal(ctx, retention time.Duration) error` — deletes
        only tokens whose booking is terminal (`status IN ('cancelled',
        'completed', 'no_show')`) and whose `expires_at` is past
        `retention` — a sweep must never turn a live link into a `404`.
- [x] 2.2 `internal/data/models.go`: add `BookingLinkTokenStore` interface (the
      three methods above) as a top-level `Models` field
      (`Models.BookingLinkTokens`), matching how `EmailVerification` and
      `PasswordReset` are wired — not composed into `BookingStore` (ISP: a
      credential's lifecycle, not a booking's). Add `LinkTokenBuffer
      time.Duration` to `data.Config`, threaded into `NewModels`.

## Phase 3: Mint inside the booking's own transaction
- [x] 3.1 `internal/data/bookings.go`: add `LinkToken string \`json:"-"\`` to
      `Booking` (in-memory only, never persisted on the row itself). Add
      `func (b *Booking) EndsAt() time.Time` (date + end_time, mirroring however
      `StartTime`/`EndTime` are already parsed elsewhere in this file).
- [x] 3.2 `InsertSafe`: after the booking INSERT succeeds and before the
      transaction commits, call the link-token model's `Mint` with
      `expiresAt = b.EndsAt() + linkTokenBuffer` inside the same `tx`, and set
      `b.LinkToken` to the returned plaintext. A failing mint must roll back the
      booking insert — no booking row commits without a token.
- [x] 3.3 Add a comment on `Insert` (the plain, non-`Safe` variant) noting it
      is test-only and mints no token — do not route a production path through
      it without adding a mint call.
- [x] 3.4 `internal/payments/process.go`: at the point that inserts and
      confirms a public booking (the webhook-confirmation path, presently the
      `fmt.Sprintf` pair building `cancelURL`/`cancelPath`), call the link-token
      store's `Mint` for the newly confirmed booking (`expiresAt =
      b.EndsAt() + linkTokenBuffer`) before building any link. A failed mint
      must make the webhook handler return an error so the delivery platform
      redelivers — no email is sent on a mint failure.

## Phase 4: `internal/booklink` — one package, four shapes, still `booking_id`
- [x] 4.1 New leaf package `internal/booklink/booklink.go`. Imports only `fmt`
      and `net/url` — must not import `internal/bookings`, must not be imported
      by `internal/mp`.
      ```go
      const QueryParam = "booking_id" // slice 2 flips this to "token"
      func Cancel(frontendURL, complexSlug, credential string) string
      func CancelPath(complexSlug, credential string) string   // relative, WhatsApp button
      func Success(frontendURL, complexSlug, credential string) string
      func SuccessPending(frontendURL, complexSlug, credential string) string
      func Failure(frontendURL, complexSlug string) string      // no credential
      ```
      `credential` is an opaque `string`: this slice always passes
      `booking.ID.String()`. Do not change these signatures in slice 2 — only
      `QueryParam` and what callers pass as `credential` change.
- [x] 4.2 `internal/bookings/create.go`: replace the duplicated `fmt.Sprintf`
      pair building `cancelURL`/`cancelPath` with `booklink.Cancel(...)` /
      `booklink.CancelPath(...)`, passing `booking.ID.String()`.
- [x] 4.3 `internal/payments/process.go`: replace its byte-identical
      `fmt.Sprintf` pair with the same two `booklink` calls.
- [x] 4.4 `internal/mp/mp.go`: `CreatePreferenceInput` gains `BackURLs
      BackURLs` where `BackURLs struct{ Success, Failure, Pending string }`, and
      **loses** `FrontendURL` and `ComplexSlug` — their only use was building the
      `back_urls` map. `CreatePreference` sets `"back_urls"` directly from
      `input.BackURLs.{Success,Failure,Pending}` — no URL construction left
      inside `internal/mp`. `BookingID` stays: `external_reference` and
      `metadata.booking_id` are correlation for a signature-authenticated
      webhook, not a credential, and are unchanged.
- [x] 4.5 `internal/bookings/public.go`: where `prefInput := mp.CreatePreferenceInput{...}`
      is assembled (`PublicBook`), build `mp.BackURLs{Success:
      booklink.Success(...), Failure: booklink.Failure(...), Pending:
      booklink.SuccessPending(...)}` using `booking.ID.String()` and pass it as
      `prefInput.BackURLs`; drop the `FrontendURL`/`ComplexSlug` fields from the
      literal.
- [x] 4.6 `PublicBook`'s response body: add the minted `token` (i.e.
      `booking.LinkToken`) as a new field. It is additive and ignorable by the
      current frontend — do not remove any existing field in this slice.

## Phase 5: Wiring — config, `Dependencies`, cron placeholder
- [x] 5.1 `cmd/api/main.go`: add `linkTokenBuffer time.Duration` to `cfg.booking`,
      a `-booking-link-token-buffer` flag (default `24 * time.Hour`) next to
      `-booking-cancellation-window`, and a `BOOKING_LINK_TOKEN_BUFFER` env
      override following the existing pattern. Thread it into
      `data.NewModels(db, data.Config{..., LinkTokenBuffer:
      cfg.booking.linkTokenBuffer})`.
- [x] 5.2 (partial, deliberate — see apply report) `internal/payments/payments.go`:
      widened `Dependencies`/`Config` with `LinkTokens LinkMinter` +
      `LinkTokenBuffer`, wired in `cmd/api/app.go`, used by Phase 3.4's mint
      call. `internal/bookings/bookings.go`'s `Dependencies` was **not**
      widened in this slice: `InsertSafe` already mints internally at the data
      layer (Phase 3.2), so `bookings.Handler` has no slice-1 caller for a
      `LinkTokens` field, and an assigned-but-unread struct field risks
      golangci-lint's `unused` check. Deferred to Phase 11.1, which adds
      `LinkResolver` for an actual slice-2 caller (`resolveLink`).
- [x] 5.3 `cmd/api/cron.go`: register `job("sweep_booking_link_tokens",
      2*time.Minute, app.<handler>.SweepExpiredBookingLinkTokens)` — or the
      equivalent method name chosen for `DeleteExpiredTerminal`'s caller —
      alongside `sweep_refund_intents`. The sweep must only ever delete tokens
      whose booking is terminal, per Phase 2.1's `DeleteExpiredTerminal`
      predicate.

## Phase 6: Slice 1 tests
- [x] 6.1 [Unit, mutation-verified] `internal/data`: `InsertSafe` on a stubbed
      failing token INSERT leaves **no** booking row committed. **Mutation, run
      and recorded**: move the mint call outside the transaction — re-run, must
      fail (booking row persists despite the mint failure).
- [x] 6.2 [Unit] `internal/data`: `b.LinkToken` from a successful `InsertSafe`
      is 43 characters and never equal to `b.ID.String()`.
- [x] 6.3 [Unit, mutation-verified] `internal/data`: two `Mint` calls for the
      same `bookingID` produce two different plaintexts and both resolve via
      `ResolveBooking`. **Mutation**: reuse a per-booking constant instead of
      fresh random bytes — re-run, must fail.
- [x] 6.4 [Unit, mutation-verified] `internal/booklink`: every one of the five
      functions emits `?booking_id=` (this slice's value of `QueryParam`) and no
      other identifying query parameter. **Mutation**: change `QueryParam` to a
      literal `"token"` inline in one function only — re-run, must fail (the URL
      shape becomes inconsistent across the package).
- [x] 6.5 [Integration, `//go:build integration`] `internal/data`:
      `ResolveBooking` on a token whose `expires_at` is in the past still
      returns the row and its past `expires_at`, not `ErrRecordNotFound`.
      **Mutation**: restore `AND expires_at > NOW()` in the SELECT — re-run,
      must fail.
- [x] 6.6 [Integration, `//go:build integration`] `internal/data`:
      `DeleteExpiredTerminal` leaves tokens of `confirmed`/`pending` bookings
      untouched however old, and removes only expired tokens of terminal
      bookings past retention. **Mutation**: drop the terminal-status predicate
      — re-run, must fail.
- [x] 6.7 [Unit] `internal/mp`: `CreatePreference` builds `back_urls` exactly
      from `input.BackURLs`; confirm `CreatePreferenceInput` no longer has
      `FrontendURL`/`ComplexSlug` fields (compile-level proof — the fields don't
      exist).
- [x] 6.8 [Unit] `internal/bookings`: `PublicBook`'s response includes a
      non-empty `token` field equal to the booking's minted `LinkToken`, and
      every other existing field (including `id`) is still present — this slice
      does not remove anything.

## Phase 7: Slice 1 verification — PR 1 boundary
- [x] 7.1 `go build ./...`, `go vet ./...`, `go vet -tags integration ./...`,
      `golangci-lint run` clean.
- [x] 7.2 `go test -race -v -count=1 ./...` green.
- [x] 7.3 `make e2e-db-up && make test/integration && make test/security &&
      make e2e-db-down` green.
- [x] 7.4 Confirm by inspection: the three public routes
      (`internal/bookings/bookings.go`'s route registration) still accept and
      authorize on `booking_id` only — nothing reads `LinkToken` for
      authorization yet. Slice 1 is invisible to the frontend.
- [x] 7.5 `rg 'FrontendURL|ComplexSlug' internal/mp/mp.go` returns nothing.

## Phase 8: sqlc regeneration — its own commit, generated-only
- [x] 8.1 From a clean tree, run `make sqlc`. Confirm the generated diff under
      `internal/db/` is isolated as its own commit-shaped step and contains
      **only** generated output — no hand-edit. Name all four expected stray
      structures in the commit message so none is mistaken for scope: the new
      `BookingLinkToken` model from migration 011, plus the three strays this
      program has previously found regenerating from a stale `internal/db/`
      snapshot — `WebhookEvent`, `JobLock`, and the `RefundAmount` type change.
- [x] 8.2 Confirm no hand-edit crept into `internal/db/`: `git status --short
      internal/db/` shows exactly the files `make sqlc` touched.
- [x] 8.3 `go build ./...` after regeneration — fix any collateral call-site
      break the same way the previous change did (mechanical type-conversion
      fixes only, no design-bearing change), if any stray forces one.

---

# SLICE 2 — Switch the Authorization (PR 2, base=PR1, ~180 lines)

Must land after slice 1 is merged: slice 2's `booklink.QueryParam` flip and
resolve-by-token logic depend on every booking already having a minted,
committed token from slice 1's `InsertSafe`/process.go mint calls.

## Phase 9: `internal/pricing/refund.go` — `LinkLive`, the tautology
- [x] 9.1 [RED] Write the invariant matrix test in `internal/pricing` before
      adding `LinkLive`: table-driven over `cancellationHours ∈ {0, 1, 24,
      168}` × `grace ∈ {0, 15m, 72h}` × `expiresAt ∈ {past, future}` × booking
      date `{past, future}`. Assert: whenever `CanRefund(...) == true`,
      `LinkLive(...) == true`, regardless of `expiresAt`. This must fail against
      current code (no `LinkLive` exists yet) — that failure is expected and is
      the RED step.
- [x] 9.2 [GREEN] Add, immediately below `CanRefund` in `internal/pricing/refund.go`:
      ```go
      func LinkLive(b *data.Booking, expiresAt time.Time, cancellationHours int,
          grace time.Duration, now time.Time) bool {
          return now.Before(expiresAt) || CanRefund(b, cancellationHours, grace)
      }
      ```
      Include the comment explaining this is a tautology (`A implies A or B`),
      not a numeric-dominance claim — see design Decision 2.
- [x] 9.3 [Mutation, run and recorded] Re-run 9.1's matrix against the GREEN
      implementation — all rows pass, including every `cancellationHours == 0`
      row (where `CanRefund` is unconditionally true and no deadline exists).
      **Mutation named explicitly**: replace the disjunct with
      `now.Before(expiresAt)` alone (drop the `|| CanRefund(...)` term) — re-run
      the same matrix, and it MUST fail on every `cancellationHours == 0` row
      and every past-`expiresAt` grace-period row. Record both runs (pre- and
      post-mutation) in the PR description. This is the single most important
      test in this change — it is what makes "expiry never blocks a
      refund-eligible cancellation" a tautology instead of a coincidence.
- [x] 9.4 [Unit] Regression: a cancellation eligible for refund (before its
      `CanRefund` deadline) is never rejected for token expiry, expressed as a
      direct call through `LinkLive`, not only the matrix.

## Phase 10: Flip `booklink.QueryParam` and the credential argument
- [x] 10.1 `internal/booklink/booklink.go`: change `QueryParam` from
      `"booking_id"` to `"token"`. Do not touch any of the five function
      signatures — only the constant and what callers pass as `credential`.
- [x] 10.2 `internal/bookings/create.go`, `internal/payments/process.go`,
      `internal/bookings/public.go` (the `BackURLs` assembly from Phase 4.5):
      change every `booklink.*` call's `credential` argument from
      `booking.ID.String()` to `booking.LinkToken`.
- [x] 10.3 [Unit, mutation-verified] `TestBookingLinkURLIsScrubbed`
      (`cmd/api/sentry_test.go`, package `main`): build a real URL with
      `booklink.Cancel(frontendURL, slug, knownToken)`, run it through
      `scrubEvent` as both `Request.URL` and `Request.QueryString`, assert the
      plaintext token is absent from the scrubbed result. **Mutation, run and
      recorded**: rename `QueryParam` to `"booking_token"` — this changes the
      URL `booklink.Cancel` actually builds (not a hardcoded string in the
      test), the `\btoken\b` rule stops matching (underscore is a word
      character), the plaintext survives scrubbing, and the test MUST fail.
      Restore `QueryParam = "token"` — re-run, passes. This is the
      load-bearing regression named in the brief: a test that only checks the
      working name would pass after a rename, because the rename would move a
      hardcoded string too — building the URL through `booklink.Cancel` is
      what prevents that.
- [x] 10.4 [Unit] Companion negative test in the same file,
      `TestDifferentlyNamedTokenParameterIsNotScrubbed`: build
      `.../book/status?booking_token=<64-char-value>` directly (a string
      literal, standing in for what `booklink.Cancel` would build if
      `QueryParam` were ever renamed), run through `scrubText`, assert the
      value after `booking_token=` is **not** redacted. This proves *why*
      10.3's mutation must fail — it is not enough that 10.3 alone exists.

## Phase 11: Resolve by token — `resolveLink`, `410`, id-free bodies
- [x] 11.1 `internal/data/booking_link_tokens.go`: confirm `ResolveBooking`
      (Phase 2.1) is reachable from `internal/bookings` via a consumer-declared
      interface — add `LinkResolver` to `internal/bookings/bookings.go`:
      ```go
      type LinkResolver interface {
          ResolveBooking(ctx context.Context, plaintext string) (*data.Booking, time.Time, error)
      }
      ```
      One method, matching the existing `Refunder` consumer-interface
      convention in this package.
- [x] 11.2 `internal/bookings/public.go`: add
      `func (h *Handler) resolveLink(w http.ResponseWriter, r *http.Request,
      token string) (*data.Booking, *data.Complex, bool)` — the whole
      authorization for the three routes. On `ErrRecordNotFound`: today's `404`
      (GET routes) shape, return `false`. On a resolved row: load the complex
      (needed for `LinkLive`'s `cancellationHours`), call `LinkLive`; if false,
      respond `410 Gone` via `h.responder.Error(...)` with a body naming the
      client's recourse (owner question 2's copy — non-empty, distinguishable
      from `404`), return `false`; if true, return the booking, the complex,
      and `true`. `resolveLink` returns the loaded complex so `PublicCancelInfo`
      and `PublicCancel` can drop their own separate `complexes.GetByID` call.
- [x] 11.3 `PublicStatus`, `PublicCancelInfo`, `PublicCancel`: replace
      `booking_id` parsing and `GetByID` with `resolveLink`, reading `token`
      from the query string (GET routes) or request body (`PublicCancel`).
      Each route keeps its own existing absent/malformed shape (`400` on the
      GETs, `422` on the POST) — `resolveLink` owns only `404` / `410` / `500`,
      per design's stated interface contract.
- [x] 11.4 All four response bodies (`PublicBook`, and the `"id": booking.ID`
      fields in `PublicStatus`, `PublicCancelInfo`, `PublicCancel`): remove the
      booking's primary key. `PublicBook` stops returning the whole `booking`
      struct if it currently does — return only the fields the frontend needs
      plus `token`.
- [x] 11.5 `internal/bookings/bookings.go`: update the `Store`/route-registration
      doc comment to state the three public routes resolve by token only.

## Phase 12: Sentry — audit and the capture-nothing pin
- [x] 12.1 Audit (no code change expected, confirm design's claim still
      holds): every `sentry.CaptureMessage` reachable from
      `internal/bookings/public.go` emits `complex_id`, never `booking_id`, and
      `PublicStatus`/`PublicCancelInfo`/`PublicCancel` reach Sentry through
      nothing (`respond.ServerError` → `slog` only). If `resolveLink` (Phase
      11.2) or its callers introduce any new capture path, confirm it carries
      neither the booking UUID nor the token plaintext.
- [x] 12.2 [Unit, mutation-verified] `TestPublicRoutesCaptureNothing`
      (`internal/bookings`, reusing the existing `captureTransport` from
      `sentry_capture_test.go`): force a store failure on each of the three
      routes, assert `count() == 0` captured events. **Mutation**: add
      `sentry.CaptureMessage(fmt.Sprintf("... booking_id=%s", booking.ID))` to
      any one of the three routes — re-run, must fail.

## Phase 13: Slice 2 remaining tests
- [x] 13.1 [Unit] Unknown token on any of the three routes → `404`/`400`/`422`
      per each route's existing shape, never `410`.
- [x] 13.2 [Unit, mutation-verified] Expired token on a real, previously-minted
      token → `410` with a non-empty, distinguishable body. **Mutation**: swap
      the `410` branch's body for an empty one or the same message as `404` —
      re-run, must fail the distinguishability assertion.
- [x] 13.3 [Unit] Expired token on a refund-eligible booking (before its
      `CanRefund` deadline) → not `410`; the refund still dispatches. This is
      `LinkLive`'s invariant exercised through the HTTP layer, not just the
      pricing matrix.
- [x] 13.4 [Unit] A booking's own `id` presented as `token` authorizes nothing
      — `resolveLink` returns `false` via the `ErrRecordNotFound` path — and
      none of the four response bodies contains the booking UUID anywhere in
      their JSON.
- [x] 13.5 [Unit] A cancelled booking's still-unexpired token still returns
      current status/payment status from `PublicStatus`, not `410`/`404`.
- [x] 13.6 [Unit] `PublicCancel` on a token resolving to an already-terminal
      booking still answers its existing `400` (unchanged from today) — token
      swap does not touch this guard.

## Phase 14: The two-live-tokens consequence — asserted, not assumed
- [x] 14.1 [Unit, mutation-verified] `internal/data` or `internal/payments`
      (whichever layer owns the mint calls): a public booking that completes
      checkout via the MercadoPago webhook ends up with **two** rows in
      `booking_link_tokens` for the same `booking_id` — the one minted at
      `InsertSafe` (checkout `back_url`) and the one minted by
      `internal/payments/process.go`'s confirmation path (confirmation email) —
      both resolving to the same booking with expiry derived from the same
      `EndsAt() + linkTokenBuffer`. Assert explicitly that the checkout token
      is **not** revoked or deleted when the confirmation token is minted.
      **Mutation**: add a revoke/delete call on the checkout token when the
      confirmation mint runs — re-run, must fail (checkout token no longer
      resolves, breaking the success-page redirect design deliberately
      protects).

## Phase 15: Frontend coordination — named, not silently assumed
- [x] 15.1 Document (PR 2 description, not code) the exact contract this slice
      changes for the separate frontend repository, per design's Migration /
      Rollout section: read `?token=` instead of `?booking_id=` on
      `/{slug}/book/cancel` and `/{slug}/book/success`; send `{"token": …}` to
      `POST /api/v1/book/cancel`; stop reading `booking.id` from all four
      response bodies and instead carry the `token` field from the
      `PublicBook` response for subsequent calls; render `410` distinctly from
      `404`. State plainly that this repository cannot verify the frontend
      change and that deploy order must be: slice 1 → (any time) → frontend
      update → slice 2. Do not merge slice 2 to a deployed environment before
      the frontend change ships.

## Phase 16: Slice 2 verification — PR 2 boundary
- [x] 16.1 `go build ./...`, `go vet ./...`, `go vet -tags integration ./...`,
      `golangci-lint run` clean.
- [x] 16.2 `go test -race -v -count=1 ./...` green.
- [x] 16.3 `make e2e-db-up && make test/integration && make test/security &&
      make e2e-db-down` green.
- [x] 16.4 `rg 'booking_id' internal/bookings/public.go` shows no remaining use
      for authorization or response-body purposes (diagnostic-only remnants, if
      any, are outside these three routes' request/response path).
- [x] 16.5 Confirm every scenario in `specs/booking-link-credential/spec.md`
      has a passing test: re-map each of the seven requirements' scenarios to
      the test added in Phases 9-14 and list the mapping in the PR description.

---

## Phase 17: Whole-change verification against `proposal.md`'s Success Criteria
- [x] 17.1 A booking id presented to any of the three routes authorizes
      nothing — re-run 13.4.
- [x] 17.2 A token past `booking end + buffer` answers `410` with a
      distinguishable, actionable message; an unknown token answers `404` —
      re-run 13.1, 13.2.
- [x] 17.3 A refund-eligible cancellation can never be blocked by token expiry
      — re-run 9.3's mutation-verified matrix and 13.3.
- [x] 17.4 An error on any of the three routes produces a Sentry event
      containing neither the booking's UUID nor the token's plaintext — re-run
      12.2 and 10.3/10.4.
- [x] 17.5 No public response body returns the booking's primary key — re-run
      13.4 and inspect all four bodies by hand.
- [x] 17.6 Exactly one function in the repository constructs a booking cancel
      link — `rg 'book/cancel\?|book/success\?' --glob '!internal/booklink/**'
      --glob '!**/*_test.go'` returns nothing outside `internal/booklink`.
- [x] 17.7 A booking cannot commit without its token committing in the same
      transaction — re-run 6.1.
- [x] 17.8 `make test`, `golangci-lint run`, and the `integration`-tagged suite
      pass on the final merged tree (both slices together, as this program's
      prior change re-ran at its own Phase 21.6).
