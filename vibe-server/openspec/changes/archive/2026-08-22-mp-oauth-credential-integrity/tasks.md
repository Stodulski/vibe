# Tasks: MercadoPago Seller Credential Integrity

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~650 (slice 1) + ~600 (slice 2) |
| 800-line budget risk | Medium (each slice separately under budget; combined, well over) |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (slice 1, plaintext guard relocation) → PR 2 (slice 2, encryption at rest) |
| Delivery strategy | auto-chain (per state.yaml) |
| Chain strategy | pending |

Decision needed before apply: No — the slice boundary is fixed by design and by the
proposal's own rollback argument (slice 1 touches no data, slice 2 reverts by
running migration 009 down). Nothing here is a judgment call for sdd-apply.

### Suggested Work Units

| Unit | Goal | PR | Focused test | Rollback boundary |
|---|---|---|---|---|
| 1 | All nine guard sites relocated behind `SellerAccessToken`/`SellerRefreshToken`/`MPConnected`, fields unexported, `mp.go:352` fallback deleted — values still plaintext | PR 1 | `go test -race ./internal/data/... ./internal/bookings/... ./internal/payments/... ./internal/complexes/... ./internal/mp/... ./cmd/api/...` | pure code revert — no data touched |
| 2 | Keyring + AEAD envelope behind the same accessors; migration 009; `cmd/mpcredkey` | PR 2 | `go test -race ./internal/crypto/... ./internal/data/...` + `make test/integration` | migration 009 down + revert the encryption commit; PR 1's accessor keeps working because it never assumed ciphertext |

Slice 1 is the larger reviewable unit (nine individually-verified call sites plus
the largest mechanical diff in the whole change — four cross-package test files
broken by unexporting). If it grows past budget, the named cut point is before
Phase 13 (the four cross-package test fixups): slice 1 would then ship with all
nine guards relocated and proven by the same-package tests, but the mutation-
verified money-path test (Phase 14) needs Phase 13's stub-recording change first,
so that is not a clean cut — flag it to sdd-apply/orchestrator rather than
splitting unilaterally.

## Phase 0: Re-sync with concurrent edits
- [x] 0.1 Re-read `internal/data/complexes.go`, `internal/data/bookings.go`,
      `internal/data/models.go`, `internal/bookings/public.go`,
      `internal/bookings/cancel.go`, `internal/payments/refund.go`,
      `internal/complexes/handlers.go`, `cmd/api/cron.go`, `internal/mp/mp.go`,
      and the four cross-package test files (`internal/complexes/handlers_test.go`,
      `internal/bookings/handlers_test.go`, `internal/payments/payments_test.go`,
      `cmd/api/cron_test.go`) for drift since design.md was written. Confirm every
      file:line reference below still points at the code it names before editing
      anything. Confirm `db/migrations/` is still at `008_*` (009 is free) and
      `cmd/mpcredkey/` does not yet exist.

# SLICE 1 — Fail-closed access, still plaintext (PR 1)

Behaviour-preserving. No column changes, no crypto. Every step below keeps
`Complex.MPAccessToken`/`MPRefreshToken` and `CronBooking.MPAccessToken`/
`MPRefreshToken` **exported** until Phase 11, so the tree compiles after every
individual guard relocation — unexporting is the enforcement step, done last,
once nothing outside `internal/data` still needs direct field access.

## Phase 1: `internal/data/mpcred.go` — the accessor contract
- [x] 1.1 [RED] `internal/data/mpcred_test.go`: table tests for
      `(*Complex).SellerAccessToken()`, `(*Complex).SellerRefreshToken()`,
      `(*Complex).MPConnected()`, `(*CronBooking).SellerAccessToken()` — nil
      pointer, pointer-to-empty-string, and pointer-to-value cases. Nil/empty
      MUST return `ErrMPNotConnected`; a non-empty value MUST return the value
      unchanged (open is identity in this slice) and `MPConnected()` MUST report
      `false` for both nil and empty. Fails to compile — nothing exists yet.
- [x] 1.2 [GREEN] Create `internal/data/mpcred.go`: `var ErrMPNotConnected,
      ErrMPCredentialUnreadable, ErrMPCredentialEmpty error` (the second sentinel
      is declared now for signature stability but is unreachable until Slice 2
      adds a real `Keyring.Open`); the four accessor methods reading the
      still-exported `MPAccessToken`/`MPRefreshToken` fields directly. 1.1 green.
- [x] 1.3 Add the exported test constructor named in design's File Changes table
      (e.g. `func NewComplexForTest(id uuid.UUID, mpAccessToken,
      mpRefreshToken *string) *Complex`) to `mpcred.go`. Needed once Phase 11
      unexports the fields — packages `complexes`, `bookings`, `payments`, and
      `main` (`cmd/api`) cannot otherwise construct a `data.Complex` with a
      credential set. `go build ./internal/data/...` succeeds.

## Phase 2: Sites 1 & 2 — `internal/bookings/public.go:239,311` (the blocking constraint)
- [x] 2.1 Replace the `complex.MPAccessToken == nil || *complex.MPAccessToken == ""`
      check at `:239` with `tok, err := complex.SellerAccessToken()`; on any
      error, keep the existing 400 response, and additionally
      `sentry.CaptureMessage` when `errors.Is(err, data.ErrMPCredentialUnreadable)`
      (unreachable in this slice, but the branch must exist so Slice 2 needs no
      further edit here).
- [x] 2.2 Replace `SellerAccessToken: *complex.MPAccessToken` at `:311` with
      `SellerAccessToken: tok` from 2.1. The dereference is gone; nothing at this
      call site reads `complex.MPAccessToken` directly anymore.
      `go test -race ./internal/bookings/...` still green (behaviour-preserving:
      plaintext in, plaintext out).

## Phase 3: Site 9 — `internal/bookings/public.go:626,628,631-633` (refresh persist)
- [x] 3.1 In `createMPPreferenceWithRetry`, replace `complex.MPRefreshToken != nil`
      / `*complex.MPRefreshToken` at `:626,628` with
      `refreshTok, refreshErr := complex.SellerRefreshToken()`; only attempt
      `RefreshOAuthToken` when `refreshErr == nil`. This is not in the proposal's
      original eight — design's orchestrator-verified ninth site — flag it as
      such in the PR description, it is easy to miss on a second pass.
- [x] 3.2 Add `sentry.CaptureMessage` alongside the existing `logger.Error` at
      `:631-633` when persisting the refreshed credentials fails, matching
      `cmd/api/cron.go:345-351`'s alert class (proposal success criterion:
      Sentry on refresh-persist failure). `go test -race ./internal/bookings/...`
      green; this is the one behavioural change in slice 1 (a new alert, not a
      new refusal) — confirm no existing test asserts silence here.

## Phase 4: Site 3 — `internal/payments/refund.go:449`
- [x] 4.1 Replace `complex.MPAccessToken == nil || *complex.MPAccessToken == ""`
      / `*complex.MPAccessToken` in `getSellerToken` with
      `tok, err := complex.SellerAccessToken()`; on error keep the existing
      `slog.Error` + `sentry.CaptureMessage` + `return ""` trade from `:429-440`
      unchanged — this site stays loud-but-continue, it does not refuse. The
      message text may distinguish "missing" from "unreadable" using
      `errors.Is(err, data.ErrMPNotConnected)` vs
      `data.ErrMPCredentialUnreadable`, but the control flow does not change.
      `go test -race ./internal/payments/...` green.

## Phase 5: Site 4 — `internal/bookings/cancel.go:104`
- [x] 5.1 In `expireCheckoutPreference`, replace `if complex.MPAccessToken != nil {
      sellerToken = *complex.MPAccessToken }` with
      `if tok, err := complex.SellerAccessToken(); err == nil { sellerToken = tok }`.
      On error, `sellerToken` stays `""` — the platform token is safe for
      `UpdatePreferenceExpired` per design's fallback table (MP rejects it
      against the wrong account). No new logging required here; the existing
      best-effort shape is preserved. `go test -race ./internal/bookings/...` green.

## Phase 6: Sites 5 & 6 — `internal/complexes/handlers.go:433,708`
- [x] 6.1 Replace `c.MPAccessToken != nil && *c.MPAccessToken != ""` at `:433`
      (`newPublicComplex`'s `PaymentsEnabled`) with `c.MPConnected()`.
- [x] 6.2 Replace `complex.MPAccessToken != nil && *complex.MPAccessToken != ""`
      at `:708` (`MercadoPagoStatus`'s `connected`) with
      `complex.MPConnected()`. `go test -race ./internal/complexes/...` green.

## Phase 7: Sites 7 & 8 — `cmd/api/cron.go:171-172,328,332`
- [x] 7.1 Site 7: in `cronReleaseExpiredPayments`, replace
      `if b.MPAccessToken != nil { sellerToken = *b.MPAccessToken }` at
      `:171-172` with `if tok, err := b.SellerAccessToken(); err == nil {
      sellerToken = tok }`. Same best-effort contract as site 4 — log-and-keep-
      empty, not a refusal (`UpdatePreferenceExpired` tolerates the platform
      token here too).
- [x] 7.2 Site 8: in `cronRefreshMPTokens`, replace
      `if c.MPRefreshToken == nil || *c.MPRefreshToken == "" { continue }` /
      `*c.MPRefreshToken` at `:328,332` with
      `refreshTok, err := c.SellerRefreshToken(); if err != nil { if
      errors.Is(err, data.ErrMPCredentialUnreadable) { failed++;
      sentry.CaptureMessage(...) }; continue }`. Absent credential (`ErrMPNotConnected`)
      stays a silent skip, matching today; unreadable is new — it now counts
      toward `failed` and alerts, where today's `!= ''` SQL filter would have
      silently dropped the row from the result set entirely (design: "strictly
      better than the SQL filter").
- [x] 7.3 Update `TestCronRefreshMPTokens_SkipsEmptyRefreshToken`
      (`cmd/api/cron_test.go:97-114`): it constructs
      `&data.Complex{MPRefreshToken: &emptyToken}` directly — this still compiles
      in slice 1 (field still exported) so no change is strictly required yet,
      but rename the empty-string case to assert through `SellerRefreshToken()`
      returning `ErrMPNotConnected` so the test documents the accessor contract
      it is actually exercising, not the field it happens to still be able to
      set. `go test -race ./cmd/api/... -run TestCronRefreshMPTokens` green.

## Phase 8: SQL predicate — `internal/data/complexes.go:241-242`
- [x] 8.1 In `GetWithMPConnected`, delete the `mp_refresh_token != ''` line;
      keep only `mp_refresh_token IS NOT NULL`. Presence is the only thing SQL
      can still answer once the column is opaque (Slice 2); integrity now moves
      to the Go loop at cron.go:327 (Phase 7.2), which already rejects an
      unreadable row instead of the SQL filter silently excluding it.
      `go test -race ./internal/data/...` green (no unit test currently covers
      this raw-SQL method; note the gap for `make test/integration`'s Phase 22
      coverage rather than adding a unit test for a query that needs a real DB
      to verify).

## Phase 9: `internal/mp/mp.go:352` — delete the fallback, don't guard it
- [x] 9.1 [RED] `internal/mp/mp_test.go`: `CreatePreference` with
      `input.SellerAccessToken == ""` must return an error (not call
      `doRequest`, not fall back to `c.accessToken`). Fails against current
      `mp.go`.
- [x] 9.2 [GREEN] At `:352-355`, delete `token := c.accessToken; if
      input.SellerAccessToken != "" { token = input.SellerAccessToken }` and
      replace with `if input.SellerAccessToken == "" { return nil,
      ErrNoSellerToken }` (new sentinel) followed by `token :=
      input.SellerAccessToken`. `c.accessToken` is no longer referenced inside
      `CreatePreference` at all — no hazardous idiom left for a fifth method to
      copy. 9.1 passes.
- [x] 9.3 Leave `:395` (`UpdatePreferenceExpired`), `:423` (`GetPayment`), `:467`
      (`RefundPayment`) untouched. Add a comment at `:423` naming
      `internal/payments/webhook.go:334` as the caller that deliberately passes
      `""` (marketplace API lets the app owner read any payment under its own
      `app_id`) — this is the load-bearing exception the next phase's test
      guards.

## Phase 10: The fallback-asymmetry guard test
- [x] 10.1 `internal/mp/mp_test.go`, httptest server asserting the
      `Authorization` header: `GetPayment(ctx, id, "")` still sends
      `Bearer <platform token>` and succeeds; same assertion for
      `UpdatePreferenceExpired(ctx, id, "")` and
      `RefundPayment(ctx, id, amount, "")`. This test must fail the moment
      somebody "makes the four consistent" by adding the refusal from Phase 9
      to any of these three — that is the point of it. Name the three sites in
      the test's doc comment so a future editor sees the warning before editing,
      not after breaking webhook confirmation.
- [x] 10.2 `internal/payments/webhook_test.go` (or nearest existing webhook test
      file): confirm webhook payment confirmation still marks a booking paid
      with a stub `GetPayment` called with `""` — i.e. Phase 9 did not touch
      the one caller that depends on the fallback. `go test -race
      ./internal/mp/... ./internal/payments/...` green.

## Phase 11: Unexport `Complex` and `CronBooking` credential fields
- [x] 11.1 `internal/data/complexes.go:36-37`: rename `MPAccessToken *string`,
      `MPRefreshToken *string` (with their `json:"-"` tags) to unexported
      `mpAccessToken`, `mpRefreshToken`. Update `complexFromDB` at `:327-329` to
      populate the renamed fields. Update `internal/data/mpcred.go`'s four
      accessors to read the renamed fields (same package, mechanical).
- [x] 11.2 `internal/data/bookings.go:675-676`: rename `CronBooking.MPAccessToken`,
      `MPRefreshToken` the same way. Update `scanCronBookings` at `:757-766`
      (the `MPAccessToken: pgToTextPtr(...)`/`MPRefreshToken:
      pgToTextPtr(...)` lines) and `mpcred.go`'s `CronBooking` accessor.
      This closes the missing-`json:"-"` gap by construction, not by tag — the
      field can no longer be marshalled from outside the package regardless of
      whether anyone remembers to tag it.
- [x] 11.3 `go build ./...` — expect it to fail in exactly the six files Phase 12
      and Phase 13 fix (two same-package, four cross-package). Any other
      failure means a guard site was missed; cross-check against the nine-site
      table above before proceeding.

## Phase 12: Fix the two same-package `internal/data` tests
- [x] 12.1 `internal/data/complexes_test.go:42-43`
      (`TestComplex_StructFields`): rename the struct-literal fields
      `MPAccessToken:`/`MPRefreshToken:` to the lowercase names from 11.1. Same
      package (`data`), so this is a rename, not a migration to the accessor —
      no behaviour to change, only field visibility.
- [x] 12.2 `internal/data/bookings_test.go:160-161`
      (`TestCronBooking_StructFields`): same rename for the `CronBooking`
      literal fields from 11.2. `go test -race ./internal/data/...` green.

## Phase 13: Fix the four cross-package tests broken by unexporting
This is the largest mechanical diff in slice 1 — four files, four different
packages, each currently building a `data.Complex{MPAccessToken: &token}`
literal that no longer compiles once the field is unexported. Each uses
`data.NewComplexForTest(...)` from Phase 1.3 instead.
- [x] 13.1 `internal/complexes/handlers_test.go:446`
      (`TestGetPublicWithholdsTheOwnerAndCollectorIdentifiers`): replace the
      `MPAccessToken: &token` field in the composite literal with
      `data.NewComplexForTest(...)`, or set the token via the constructor and
      the remaining fields by struct-literal assignment on the result —
      whichever the constructor's final shape (1.3) supports. Confirm the test
      still asserts the owner/collector identifiers are withheld.
- [x] 13.2 `internal/bookings/handlers_test.go:235` (`preparePublicBooking`):
      same fix for the `MPAccessToken: &token` field feeding every public
      booking test that calls this helper.
- [x] 13.3 `internal/payments/payments_test.go:288`
      (`TestRefundsUseTheComplexOwnSellerToken`): same fix; this test directly
      exercises Phase 4's `getSellerToken`, so also confirm it still asserts
      the refund is issued against the complex's own token, not the platform's.
- [x] 13.4 `cmd/api/cron_test.go:107` (already touched behaviourally in 7.3):
      apply the same field-literal fix here now that the field is unexported.
      `go build ./...` succeeds; `go test -race ./...` (excluding integration
      tags) green across all four packages.

## Phase 14: The mutation-verified money-path test (spec requirement 6)
This is the single most important deliverable in this change: no code path may
be able to set `mp.CreatePreferenceInput.SellerAccessToken` to anything other
than a value the accessor produced, or leave it unset so the caller's refusal
applies.
- [x] 14.1 `internal/bookings/stubs_test.go`: add a `lastInput
      mp.CreatePreferenceInput` field to `stubCheckout`, recorded at the top of
      `CreatePreference` (`:209`), so a test can assert on what was actually
      passed — today's stub ignores its argument entirely.
- [x] 14.2 [RED→GREEN] New test in `internal/bookings`: build a fixture whose
      complex has no MercadoPago credential (nil/empty — `ErrMPNotConnected` is
      the reachable failure mode in this slice; `ErrMPCredentialUnreadable`'s
      arm of this same test is completed in Slice 2 Phase 18 once a real
      decrypt failure is constructible). Assert: the handler returns 400,
      `stubCheckout.created == 0` (never called), and
      `stubCheckout.lastInput.SellerAccessToken` was never populated from any
      raw stored value.
- [x] 14.3 **Mutation, run and recorded**: delete the guard at
      `internal/bookings/public.go:239` (Phase 2.1) locally, re-run 14.2 — it
      MUST fail (booking proceeds, `CreatePreference` gets called with an empty
      token). Restore the guard. Record this mutation step in the PR
      description so a reviewer does not have to reconstruct it.
      `go test -race ./internal/bookings/...` green with the guard restored.

## Phase 15: Slice 1 verification — PR 1 boundary
- [x] 15.1 `go build ./...` succeeds.
- [x] 15.2 `go vet ./...` clean.
- [x] 15.3 `golangci-lint run` clean on every changed file.
- [x] 15.4 `go test -race -v -count=1 ./...` (unit suite; `make test`) green.
- [x] 15.5 Grep confirms no remaining direct reads of `.MPAccessToken`/
      `.MPRefreshToken` outside `internal/data` (`rg
      '\.MPAccessToken|\.MPRefreshToken' --glob '!internal/data/**'` returns
      nothing) — the compiler already enforces this after Phase 11, this is a
      belt-and-suspenders check for the PR description, not a new gate.

---

# SLICE 2 — Encrypt at rest behind the accessors (PR 2)

Everything below assumes Slice 1 is merged: the accessors exist, every consumer
routes through them, and the fields are unexported. Slice 2 changes only what
`open`/write inside `internal/data` does with the bytes — no call site outside
`internal/data` changes again.

## Phase 16: `internal/crypto` — envelope and keyring
- [x] 16.1 [RED] `internal/crypto/envelope_test.go`,
      `internal/crypto/keyring_test.go`: table tests for seal/open round-trip;
      wrong kid; wrong `complexID` in AAD; flipped ciphertext/tag byte;
      truncated envelope; plaintext (non-envelope-shaped) input; and — task it
      explicitly — **a nil `*Keyring` errors on every `Seal` and every `Open`
      call**, never a plaintext passthrough. Fails — nothing exists yet.
- [x] 16.2 [GREEN] `internal/crypto/envelope.go`: `crypto/aes` +
      `crypto/cipher` AES-256-GCM seal/open against the exact byte-for-byte
      envelope in design.md ("v1" "." kid "." base64.RawURLEncoding(nonce ||
      sealed)), AAD = `"mpcred" 0x1F "v1" 0x1F complexID 0x1F column`.
      `internal/crypto/keyring.go`: `ParseKeyring(spec string) (*Keyring,
      error)` for `"kid:b64[,kid:b64...]"` (first entry writes, every entry
      opens; ≥1 entry, 32 raw bytes each, unique kids). 16.1 green.
      `go test -race ./internal/crypto/...` green.

## Phase 17: Wire the keyring behind the accessors
- [x] 17.1 `internal/data/models.go`: add `Keys *crypto.Keyring` to `Config`
      (alongside `PaymentExpiry`, `Logger` at `:396-404`); thread it into
      `ComplexModel` and `BookingModel` in `NewModels` (`:435-455`).
- [x] 17.2 `internal/data/complexes.go`: `complexFromDB` (`:306-333`) calls
      `Keyring.Open` with the row's AAD instead of the identity pass-through
      from Phase 1.2; store the decrypt outcome (value or error) so
      `SellerAccessToken`/`SellerRefreshToken` can return
      `ErrMPCredentialUnreadable` instead of a value. `UpdateMPCredentials`
      (`:208-217`) calls `Keyring.Seal` before the `UPDATE`, and refuses an
      empty input with `ErrMPCredentialEmpty` before sealing (design: "the
      CHECK is a floor, not the guard").
- [x] 17.3 `internal/data/bookings.go`: `scanCronBookings` (`:738-769`) opens
      both credential columns the same way, feeding `CronBooking`'s
      `SellerAccessToken()`.
- [x] 17.4 `internal/data/mpcred.go`: `MPConnected()` returns `false` for both
      `ErrMPNotConnected` and `ErrMPCredentialUnreadable` — a venue whose
      credential cannot be opened genuinely cannot take payments.
      `go test -race ./internal/data/...` green with a real (test) keyring
      wired into the model constructors used by existing tests.

## Phase 18: Complete the mutation-verified test — credential-unreadable arm
- [x] 18.1 Extend Phase 14's test (or add a sibling test) in
      `internal/bookings`: construct a complex whose stored `mp_access_token`
      was sealed under a key not present in the test keyring (or tamper one
      byte of the sealed envelope before decode). Assert `SellerAccessToken()`
      returns `ErrMPCredentialUnreadable`, the handler still returns 400,
      `stubCheckout.created == 0`, and — new in this slice — a Sentry message
      was captured (Phase 2.1's added branch). This is the scenario spec
      requirement 6 literally describes ("a stored credential's ciphertext...
      fails to decrypt"); Phase 14 covered the not-connected arm, this
      completes the unreadable arm now that a real decrypt failure is
      constructible.

## Phase 19: `db/migrations/009_mp_credentials_nonempty.sql`
- [x] 19.1 Write the decrypt-all/re-encrypt-all migration: assert `SELECT
      count(*) FROM complexes WHERE mp_access_token = '' OR mp_refresh_token =
      '' OR mp_user_id = ''` is 0 (NULL-out any stragglers first), then add
      `CHECK (mp_access_token <> '')` and the same on `mp_refresh_token` /
      `mp_user_id` — migration 006 SECTION 2's prove-then-constrain shape. Down:
      drop the CHECKs only; no data down-migration exists because nothing is
      deployed. No `db/queries/*.sql` change and no `make sqlc` regeneration —
      every touched query in this change is hand-written
      (`internal/data/complexes.go`'s `UpdateMPCredentials`/`GetWithMPConnected`
      use `m.DB.Exec`/`m.DB.Query` directly, not `m.Q.*`), which avoids
      migration 006's documented stray `WebhookEvent` diff.
- [x] 19.2 `make migrate-up` then `make migrate-down` against a local DB
      round-trips cleanly.

## Phase 20: `cmd/mpcredkey/main.go`
- [x] 20.1 New operator binary, precedent `cmd/benchmark/main.go`. `seal`
      subcommand: converts plaintext `mp_access_token`/`mp_refresh_token`
      values in a developer/E2E database to the v1 envelope under the active
      key. `rekey` subcommand: kid → kid, opens under the old key and seals
      under the new active key. No down mode — there is no production
      ciphertext to reverse.

## Phase 21: `cmd/api/main.go` — key material config
- [x] 21.1 `flag.StringVar(&cfg.mp.credentialKeys, "mp-credential-keys", "",
      ...)` beside the existing `mp-*` flags (`:206-209`); `MP_CREDENTIAL_KEYS`
      env override beside `MP_CLIENT_SECRET` (`:271-273`).
- [x] 21.2 Boot validation matching the `JWT_SECRET` pattern at `:434-443`:
      `crypto.ParseKeyring(cfg.mp.credentialKeys)` must succeed (≥1 entry, 32
      raw bytes each, unique kids) or the process calls `sentry.Flush` +
      `os.Exit(1)` before serving. Resolve the design's open question by
      requiring the keyring unconditionally (matching `JWT_SECRET`'s
      unconditional requirement), not gated on `cfg.env == "production"`.

## Phase 22: Integration coverage
- [x] 22.1 `//go:build integration` test: raw `SELECT mp_access_token FROM
      complexes WHERE id = $1` after a seed-and-connect returns a `"v1.…"`
      envelope, never a value usable as an MP bearer token.
- [x] 22.2 `//go:build integration` test: `GetWithMPConnected` returns rows
      that the Go loop (Phase 7.2) then correctly accepts/rejects — including
      at least one row sealed under a key not present in the test keyring, to
      prove the unreadable row is surfaced and alerted rather than silently
      dropped (this is what makes Phase 8's SQL predicate deletion provably
      safe end to end).

## Phase 23: Slice 2 verification — PR 2 boundary
- [x] 23.1 `go build ./...`, `go vet ./...`, `golangci-lint run` clean.
- [x] 23.2 `go test -race -v -count=1 ./...` green.
- [x] 23.3 `make e2e-db-up && make test/integration && make test/security &&
      make e2e-db-down` green, including Phase 22's new tests.
- [x] 23.4 Manually confirm every Success Criterion in proposal.md is now
      true: raw `SELECT` yields nothing usable; the mutation-verified test
      (Phases 14+18) proves both refusal arms; no consumer reads
      `MPAccessToken`/`MPRefreshToken` directly (Phase 15.5, re-run); rotating
      keys needs no downtime (exercised by `cmd/mpcredkey rekey` against a
      running local stack); `CronBooking` cannot deliver ciphertext as a
      bearer token (Phase 17.3 + 18.1's coverage extends to it structurally,
      since it shares the same accessor).
