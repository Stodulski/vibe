# Archive Report: Booking Link Credential Hardening

**Change**: booking-link-credential-hardening  
**Artifact store**: openspec  
**Created**: 2026-08-23  
**Archived**: 2026-08-23  
**Tree**: clean at a1a13ed (both slices merged)  
**Archive location**: `openspec/changes/archive/2026-08-23-booking-link-credential-hardening/`

## Executive Summary

The booking link credential hardening change has been completed, verified (PASS, 7/7 requirements), and archived. Both slices implemented; all 50 tasks checked. The new capability spec `booking-link-credential` is now the source of truth under `openspec/specs/booking-link-credential/spec.md`.

## Verification Status

**Verdict**: PASS  
**Requirements met**: 7 of 7  
**Critical issues**: 0  
**Warnings**: 1 (documented, non-blocking)

Per `verify-report.md`, all seven spec requirements are satisfied with passing covering tests, both regression tests for the Sentry parameter-name requirement are structurally sound, the refund-invariant is a genuine tautology with no divergent expiry check anywhere in the cancel path, and all gates pass clean (build ×2, unit tests, race tests, lint, audit, integration and security suites, and goose migration round-trip).

The single WARNING concerns an undocumented breaking change in `PublicBook`'s response envelope: in addition to dropping the primary key (which the spec requires), it also drops ten other fields (`complex_id`, `court_id`, `client_id`, `duration_minutes`, `reminder_sent_2h`, `notes`, `created_by`, `created_at`, `updated_at`, `version`). This is in scope per the proposal and tasks, but not enumerated in the design doc's frontend coordination section. Flagged to owner/frontend team.

## Artifacts Archived

- ✅ proposal.md (proposal 7 sections, tradeoff analysis documented)
- ✅ specs/ (delta spec → new capability spec)
- ✅ design.md (22 sections: product decision, scope, slices, architecture, frontend contract)
- ✅ tasks.md (50 tasks across 17 phases, all checked)
- ✅ exploration.md (persistence note: agent had no write tool; orchestrator materialized)
- ✅ state.yaml (phase status, product decision record, core findings)
- ✅ verify-report.md (task completeness, spec compliance matrix, gate results)

## Spec Sync

**New capability spec created**: `openspec/specs/booking-link-credential/spec.md`  
**Action**: Full new spec (no merging required; no prior main spec existed)  
**Content**: 7 requirements × scenarios (14 scenarios total)

### Specification Contents

1. **Requirement 1**: The booking's primary key authorizes nothing on the public routes
2. **Requirement 2**: The access token is opaque, expiring, and hashed at rest
3. **Requirement 3**: Token expiry can never block a refund-eligible cancellation
4. **Requirement 4**: An expired token answers 410, distinguishably from unknown (404)
5. **Requirement 5**: Public response bodies never return the booking's primary key
6. **Requirement 6**: The token's query parameter is named exactly `token`
7. **Requirement 7**: A resolved booking id never re-enters an error report

## Three Must-Survive Items (Verbatim)

### 1. The Invariant and Why It Is a Tautology

**Invariant**: `LinkLive` returns `now.Before(expiresAt) || CanRefund(...)`, and its right-hand term is the same call the refund dispatch makes — verified, not a parallel copy.

**Why a tautology, not arithmetic coincidence**: The proposal claimed "both CanRefund deadlines fall strictly before booking start, so token lifetime till booking end plus buffer dominates them by construction." This claim is false for a legal configuration: `internal/pricing/refund.go:36` returns true unconditionally when `cancellationHours <= 0`, and migration 006 permits exactly 0. For such a complex there is no deadline at all.

**The replacement**: Stop comparing numbers. `LinkLive` instead returns the disjunction of two truths: `now.Before(expiresAt) || CanRefund(...)`. The right-hand term is the same call the refund dispatch makes at `public.go:558` and `public.go:492` — not a divergent copy, not a parallel deadline check. This makes the invariant a tautology rather than arithmetic coincidence: `A implies (A or B)` for every value of every number in that file, including `cancellationHours == 0`, a negative buffer, and any future edit.

**Mutation verification**: The matrix test `TestLinkLiveNeverRejectsARefundEligibleCancellation` runs 44 sub-tests covering `cancellationHours ∈ {0, 1, 24, 168}` × `grace ∈ {0, 15m, 72h}` × `expiresAt ∈ {past, future}` × booking date `{past, future}`. All 44 pass, including every `cancellationHours == 0` row (where `CanRefund` is unconditionally true and no deadline exists). When the disjunct is replaced with `now.Before(expiresAt)` alone, the matrix fails on every `cancellationHours == 0` row, confirming the replacement is load-bearing.

### 2. Why the Query Parameter Is Named `token` and Must Stay Named That

**The naming choice does the scrubber work for free**: `cmd/api/sentry.go:96-99` implements a "named secret" rule that redacts `?token=` and only that. The rule uses a word boundary regex (`\b`) that cannot match `token` inside `booking_token`, because an underscore is a word character. The code contains its own comment explaining this property.

**Why the name belongs in the spec with a regression test**: A well-meant rename to `booking_token` silently undoes the scrubbing with no visible signal. The two regression tests ensure this does not happen:

1. **`TestBookingLinkURLIsScrubbed`** (`cmd/api/sentry_test.go:288`): Builds a real URL with `booklink.Cancel(frontendURL, slug, knownToken)`, runs it through `scrubEvent` as both `Request.URL` and `Request.QueryString`, asserts the plaintext token is absent from the scrubbed result. When `QueryParam` is renamed to `"booking_token"` in the constant, this changes the URL that `booklink.Cancel` actually builds — not a hardcoded string in the test. The `\btoken\b` rule stops matching, the plaintext survives scrubbing, and the test fails. This is the load-bearing regression the brief asked to confirm.

2. **`TestDifferentlyNamedTokenParameterIsNotScrubbed`** (`cmd/api/sentry_test.go:318`): Proves the negative with a literal `?booking_token=` string, demonstrating *why* the first test must fail — the underscore blocks the word boundary match.

**Why this matters for the fix**: The proposal explicitly rejected adding a UUID scrubber rule on its own merits: it would redact every legitimate diagnostic booking id elsewhere in captured error strings. Naming the parameter correctly is what prevents the regression.

### 3. The Exact Frontend Contract (Ten Dropped Fields Plus Structure Change)

**`PublicBook` response before this change**: Returned the entire `booking` struct.

**`PublicBook` response after this change**: Returns an explicit minimal field set only:
- `status` (booking status)
- `payment_status` (payment status)
- `date` (booking date)
- `start_time` (start time)
- `end_time` (end time)
- `court_name` (court name)
- `complex_name` (complex name)
- `price` (price)
- `deposit_amount` (deposit amount)
- `token` (new, minted credential)

**Fields dropped from `PublicBook` (ten total)**:
1. `id` (the booking's primary key — required by spec Requirement 5)
2. `complex_id` (complex identifier)
3. `court_id` (court identifier)
4. `client_id` (client identifier)
5. `duration_minutes` (booking duration)
6. `reminder_sent_2h` (reminder flag)
7. `notes` (booking notes)
8. `created_by` (creator identifier)
9. `created_at` (creation timestamp)
10. `updated_at` (update timestamp)

**Also dropped**: `version` (version field)

**Status**: This is in scope per the proposal and tasks (`tasks.md` 11.4 explicitly directs it), but the design doc's "Frontend, separate repository" section names only the `token` addition and the `id` removal. It does not enumerate that these other fields also disappeared. If the frontend consumes any of them today, this is an undocumented breaking change. Frontend team must confirm they do not read any of these ten fields before slice 2 lands live.

**Other three public routes** (`PublicStatus`, `PublicCancelInfo`, `PublicCancel`): Each keeps its own structure; the primary key removal is the only change in those response bodies.

## Post-Archive Follow-Ups

The following items are operational findings recorded for future work or maintenance. They are not tasks blocked by this change, but knowledge that will be valuable when related work is undertaken.

### 1. The Deploy Order Is Load-Bearing and This Repository Cannot Enforce It

**Deploy order**: Slice 1 → frontend update → Slice 2

**Why it matters**: Slice 1 is purely additive (still emitting `?booking_id=` while routes read `booking_id`). Slice 2 flips the query parameter and authorization model. Shipping slice 2 first breaks every outstanding `?booking_id=` link the moment it lands. The frontend must be updated between slice 1 and slice 2; this repository can only document the requirement in the PR description, not enforce it.

### 2. A Complex with `cancellation_hours = 0` Gets Non-Expiring Links

**Configuration**: Migration 006 constrains `cancellation_hours` to `0..168`; the settings screen permits `0`.

**Behavior**: When `cancellation_hours == 0`, `CanRefund()` returns true unconditionally (no deadline). The `LinkLive` invariant then reduces to `now.Before(expiresAt) || true`, which simplifies to `true`. This means tokens for such bookings never expire (or expire only when the booking becomes terminal and the sweep runs).

**Why**: This follows from the invariant and is already what the setting means for refunds. No action needed; recorded for context.

### 3. `httpx` Error Logger Writes RequestURI to slog (Token Appears in Plaintext in Logs)

**Location**: `internal/httpx/errors.go:37` logs `r.URL.RequestURI()` to slog on any error response.

**Impact**: The token appears in plaintext in server logs on any error response from the three public routes.

**Scope**: Out of this change's scope. The token is opaque and unguessable; the privacy concern is the same as for any error that leaks request context. Documented here rather than left to be discovered.

### 4. Two Live Tokens Per Public Booking (By Design)

**Why**: Hashing at rest makes the plaintext unrecoverable, so a later request cannot read back what a previous request minted. Two emission sites exist:
- Checkout path: `internal/bookings/create.go` → `booklink.Success` (minted early)
- Confirmation path: `internal/payments/process.go` → webhook handler (minted at confirmation)

**Consequence**: A public booking ends with two live tokens: the checkout `back_url` and the confirmation email. The checkout token is deliberately not revoked at confirmation, because MercadoPago redirects the browser to the success URL at roughly the moment the webhook fires (race condition protection).

**Test coverage**: `TestCheckoutTokenSurvivesTheConfirmationMint` asserts both tokens resolve independently after the confirmation mint.

**Alternative if owner wants different behavior**: Reversible at rest via the existing `crypto.Keyring`.

### 5. `mp.Caller` (`AsSeller`/`AsPlatform`) — Deferred

This change does not touch the `mp.Caller` abstraction. It remains deferred from two changes back (mp-oauth-credential-integrity and refund-durability-and-collector-integrity).

### 6. MercadoPago-Side Token Revocation — Unverified

This change does not add token revocation on MercadoPago's side. Whether the endpoint exists and is working remains unverified; nobody in this chain has had network access to check. Documented for when that verification becomes relevant.

## Task Completion

All 50 implementation tasks across 17 phases are checked `[x]` in `tasks.md`. Source inspection confirms the checked state matches the code:

- Slice 1 boundary: `booklink.QueryParam` was `"booking_id"` at commit bf4c059 (verified via `git show bf4c059:internal/booklink/booklink.go`) and flips to `"token"` at commit 854d783.
- All 7 spec requirements covered by at least one test; spec compliance matrix verified in `verify-report.md`.
- No unchecked implementation tasks remain.

## Scope Notes

This change does not include:

- MercadoPago-side token revocation (unverified if endpoint exists)
- The `mp.Caller` abstraction refinement (deferred from earlier changes)
- Rate limiting enhancements beyond current per-IP ceilings
- Dual-accept windows or backfill (nothing deployed)
- A self-service token reissue path (out of scope)

## SDD Cycle Complete

The change has been fully:
- ✅ Explored (exploration.md, 9 sections)
- ✅ Proposed (proposal.md, tradeoff decisions documented)
- ✅ Specified (spec.md, 7 requirements with scenarios)
- ✅ Designed (design.md, with falsified-invariant correction and two-live-tokens consequence)
- ✅ Tasked (tasks.md, 50 tasks across 17 phases, all checked)
- ✅ Implemented (two slices, bf4c059 + 854d783)
- ✅ Verified (PASS, all 7 requirements met, one WARNING documented)
- ✅ Archived

The source of truth has been updated. The booking link credential is now governed by a formal specification under `openspec/specs/booking-link-credential/spec.md`.
