# Verification Report: booking-link-credential-hardening

**Change**: booking-link-credential-hardening
**Mode**: Full artifacts (proposal, design, tasks, spec all present)
**Commits verified**: e1b60f7 (sqlc), bf4c059 (slice 1), 854d783 (slice 2)
**Tree**: clean at 854d783
**Verdict**: **PASS**

## Task completeness

All 50 tasks across 17 phases in `tasks.md` are checked `[x]`. Source inspection
confirms the checked state matches the code: slice 1's `booklink.QueryParam`
was `"booking_id"` at bf4c059 (verified via `git show bf4c059:internal/booklink/booklink.go`)
and flips to `"token"` at 854d783 — the slice boundary tasks.md declares as
load-bearing actually held.

## Spec compliance matrix (7 requirements, specs/booking-link-credential/spec.md)

| # | Requirement | Verdict | Evidence |
|---|---|---|---|
| 1 | Primary key authorizes nothing on public routes | PASS | `internal/bookings/bookings.go:237-240` registers all three public routes to handlers that call only `resolveLink` (`public.go:420,457,480,547`); no `booking_id` read anywhere in `public.go`. Test `TestABookingIDPresentedAsTokenAuthorizesNothing` — PASS (`handlers_test.go:580`) |
| 2 | Token opaque, expiring, hashed at rest, minted no later than insert commit | PASS | `internal/data/booking_link_tokens.go:42-45` SHA-256 at rest; `bookings.go:239-245` mints inside `InsertSafe`'s tx before `tx.Commit`, deferred rollback covers mint failure. Test `TestInsertSafeMintsATokenInTheSameTransaction` — PASS (integration). Staff path (`create.go:207-223`) also routes through `InsertSafe` unconditionally, so every production insert mints |
| 3 | Token expiry can never block a refund-eligible cancellation | PASS | `internal/pricing/refund.go:70-72`: `LinkLive = now.Before(expiresAt) \|\| CanRefund(...)`, a disjunction using the *same* `CanRefund` function the dispatch calls (`public.go:558`, `public.go:492`) — not a divergent copy. Only one call site of `LinkLive` exists (`public.go:437`, inside `resolveLink`); no second expiry check found anywhere between `resolveLink` and the cancel dispatch. Matrix test `TestLinkLiveNeverRejectsARefundEligibleCancellation` — 44/44 sub-tests PASS, including every `cancellationHours=0` row |
| 4 | Expired token → 410 distinguishable from unknown → 404 | PASS | `resolveLink` (`public.go:420-443`): `ErrRecordNotFound` → `h.respond.NotFound` (404); `!LinkLive` → `h.respond.Error(w,r,http.StatusGone,linkExpiredMessage)` (410) with non-empty recourse copy. Tests `TestAnExpiredTokenGetsA410`, `TestPublicStatusUnknownTokenIsNotFound`, `TestACancelledBookingsTokenStillAnswersStatus`, `TestAnExpiredTokenOnARefundEligibleBookingIsNot410` — all PASS |
| 5 | Public response bodies never return the primary key | PASS | All four handlers (`PublicBook:309-322`, `PublicStatus:462-467`, `PublicCancelInfo:502-516`, `PublicCancel:646-655`) construct explicit `httpx.Envelope` literals with no `id`/`booking.ID` field. `rg '"id":'` across `internal/bookings/*.go` (excl. tests) returns nothing. Test `TestPublicBookResponseCarriesTheTokenAndNeverTheBookingID` — PASS |
| 6 | Token query parameter named exactly `token` | PASS | `internal/booklink/booklink.go:26`: `const QueryParam = "token"`. Regression pair confirmed: `TestBookingLinkURLIsScrubbed` (`sentry_test.go:288`) builds the URL via `booklink.Cancel(...)` — not a literal — and asserts the plaintext is absent after scrubbing; `TestDifferentlyNamedTokenParameterIsNotScrubbed` (`:318`) proves the negative with a literal `?booking_token=` string. Both PASS |
| 7 | A resolved booking id never re-enters an error report | PASS | Audit confirms the two `sentry.CaptureMessage` calls in `internal/bookings` (`public.go:244`, `:671`) emit only `complex_id`. `PublicStatus`/`PublicCancelInfo`/`PublicCancel` reach Sentry through nothing — `ServerError` → `slog` only. Test `TestPublicRoutesCaptureNothing` (integration) — PASS, all three sub-tests |

## The four independently-verified checks

1. **`booklink.QueryParam == "token"`** — confirmed, `internal/booklink/booklink.go:26`.
2. **All three public routes resolve through `resolveLink`, no `booking_id` reads** — confirmed by direct read of `public.go` and route registration in `bookings.go:237-240`.
3. **No public response body returns the primary key** — confirmed; no `"id":` literal or raw-struct marshal in any of the four handlers.
4. **`LinkLive` is `now.Before(expiresAt) || CanRefund(...)`, a disjunction** — confirmed, `internal/pricing/refund.go:70-72`, and it is provably the *same* `CanRefund` call used by the refund dispatch in `PublicCancel` (`public.go:558`) and `PublicCancelInfo` (`public.go:492`) — not a divergent copy. No second expiry check exists between `resolveLink` and any cancel/refund path.

## Invariant at the strictest reading

`LinkLive`'s safety property holds structurally, not by tuning: `resolveLink` is
the sole call site of `LinkLive` (`public.go:437`), and the `CanRefund` term
inside it is textually the same function `PublicCancel`'s and
`PublicCancelInfo`'s dispatch logic call, with the same three arguments
(`booking`, `complex.CancellationHours`, `h.cfg.GracePeriod`). No alternate
expiry gate exists anywhere in the request path between token resolution and
either read-only or write (cancel) handling. Design's own correction (the
proposal's original "both deadlines fall before booking end" claim is false
for `cancellationHours == 0`) is accounted for by the tautology, not
worked around — confirmed by the mutation-verified matrix test covering the
`hours=0` rows explicitly and passing.

## Sentry regression pair

Both tests exist and both hold their distinct roles:
- `TestBookingLinkURLIsScrubbed` builds the exercised URL through
  `booklink.Cancel(...)`, so a rename of `QueryParam` changes the bytes the
  test actually scrubs — not a hardcoded string that would silently survive
  the same rename. This is the load-bearing property the brief asked to
  confirm, and it holds.
- `TestDifferentlyNamedTokenParameterIsNotScrubbed` is the negative, built
  from a literal (intentionally, since it stands in for a hypothetical rename
  and needs no `booklink` call to make its point).

## Two-live-tokens assertion

Confirmed as designed and tested. `internal/data/booking_link_tokens.go`'s
`Mint` (standalone, own transaction) is called a second time by
`internal/payments/process.go:234` on webhook confirmation, independently of
`InsertSafe`'s mint at checkout. The checkout token is not revoked —
`internal/payments/process.go:230-233`'s comment states why (MercadoPago's
success redirect fires at roughly webhook time) and the integration test
`TestCheckoutTokenSurvivesTheConfirmationMint` asserts both tokens resolve
independently after the confirmation mint. PASS.

## Beyond-spec implementation: PublicBook's response envelope

`PublicBook` (`public.go:309-322`) replaces the previous full-`booking`-struct
response with an explicit minimal field set (`status`, `payment_status`,
`date`, `start_time`, `end_time`, `court_name`, `complex_name`, `price`,
`deposit_amount`, plus top-level `token`). This is **in scope** — the
proposal's own Scope section and Affected Areas table name exactly this
outcome ("`PublicBook` returns the token, not the whole booking"), and
`tasks.md` 11.4 explicitly directs it. It is not unauthorized scope creep.

**However**, it is a wider contract change than the spec's letter strictly
requires. Spec Requirement 5 only forbids the primary key; the actual diff
also drops `complex_id`, `court_id`, `client_id`, `duration_minutes`,
`reminder_sent_2h`, `notes`, `created_by`, `created_at`, `updated_at`, and
`version` from `PublicBook`'s response — none of which is the booking's UUID.
`design.md`'s "Frontend, separate repository" section names only the `token`
addition and the `id` removal as what the frontend must adapt to; it does not
enumerate that these other fields also disappeared. If the frontend consumes
any of them today (e.g. `notes` or `duration_minutes`), this is an
undocumented breaking change riding on the credential fix. **WARNING**, not
CRITICAL — nothing in this repository can verify frontend usage, and the
dependency section already states coordination is required before slice 2
lands anywhere live. Flag explicitly to the owner/frontend team before
deploying slice 2.

## Frontend's exact required changes (separate repository — this tree cannot verify any of it)

1. Read `?token=` instead of `?booking_id=` on `/{slug}/book/cancel` and
   `/{slug}/book/success`.
2. Send `{"token": …}` (not `{"booking_id": …}`) to `POST /api/v1/book/cancel`.
3. Stop reading `booking.id`/`booking_id` from all four public response
   bodies; carry the `token` field from `PublicBook`'s response for
   subsequent calls instead.
4. Render `410 Gone` distinctly from `404` (with the `410`'s body message as
   the recourse copy).
5. **Not named in design.md but true of the diff**: `PublicBook`'s `booking`
   object no longer carries `complex_id`, `court_id`, `client_id`,
   `duration_minutes`, `reminder_sent_2h`, `notes`, `created_by`,
   `created_at`, `updated_at`, or `version` — only the fields enumerated
   above. Confirm the frontend does not read any of these before slice 2
   reaches a live environment.
6. Deploy order remains: slice 1 → (any time) → frontend update → slice 2.
   Nothing in this repository verifies the frontend change; it is out of this
   change's blast radius by design.

## Gate results (verbatim summary; full logs captured during this session)

| Gate | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | clean, no output |
| `go build -tags integration ./...` | 0 | clean, no output |
| `make test` (`go test -race -v -count=1 ./...`) | 0 | 0 `FAIL`, 31 `ok` packages |
| `go test -race -count=1 ./...` | 0 | same run as above, confirmed 0 FAIL |
| `golangci-lint run` | 0 | `0 issues.` (build confirmed clean first) |
| `make audit` | 0 | `go mod verify`: all modules verified; `govulncheck`: 0 vulnerabilities affecting code |
| `make e2e-db-up` | 0 | E2E Postgres up, migrations 001-011 applied cleanly |
| goose `011` down → status → up | 0 | down removed exactly the table + 2 indexes; status showed 010 as head with 011 pending; up reapplied cleanly |
| `make test/integration` | 0 | 0 `FAIL`, 31 `ok` packages, including all named booking-link-credential tests |
| `make test/security` (run separately, after integration completed) | 0 | 0 `FAIL`, 17 `--- PASS`, `cmd/api` security suite `ok` |
| `make e2e-db-down` | 0 | E2E containers/volumes torn down |

## Issues

**CRITICAL**: none.

**WARNING**:
1. `design.md`'s frontend-coordination note under-names the `PublicBook`
   response contract change — it lists the `token` addition and `id`
   removal but not the ten other fields the new minimal envelope also drops.
   Confirm with the frontend team before slice 2 reaches a live environment.

**SUGGESTION**: none beyond the above.

## Recommendation

**Archive.** All seven spec requirements are met with passing covering tests,
both regression tests for the Sentry parameter-name requirement are present
and structurally sound (one built through `booklink.Cancel`, one a literal
negative), the refund-invariant is a genuine tautology with no divergent
expiry check anywhere in the cancel path, and all requested gates pass clean
(build ×2, `make test`, `go test -race`, lint, audit, both integration and
security suites run separately, and the goose 011 round-trip). The one
WARNING (frontend response-contract gap in the design doc) does not block
archive — it is a documentation completeness note for the dependency this
repository cannot verify, not a defect in this repository's own contract.
