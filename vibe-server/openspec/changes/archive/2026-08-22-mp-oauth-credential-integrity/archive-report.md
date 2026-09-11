# Archive Report: mp-oauth-credential-integrity

**Date**: 2026-08-22  
**Status**: ARCHIVED  
**Verification Verdict**: PASS WITH WARNINGS (all blocking issues resolved)

---

## Change Summary

**Capability**: `seller-credential-integrity` (new) — how MercadoPago seller OAuth credentials are stored, keyed, rotated, and what every consumer must do when a credential cannot be produced intact.

**Scope**: Two chained slices, ordered so no intermediate commit is unsafe:
1. **Slice 1** (PR 1): Guard relocation — introduce the credential accessor and route all eight + one guard sites through it (plaintext round-trip, fully testable)
2. **Slice 2** (PR 2): AEAD encryption at rest behind the accessor, plus keyring and migration

**Final State Authority**: This archive reflects the state at close (2026-08-22, commit 6d39822, tree CLEAN). The verify-report dated the closure pending W0 resolution; that fix has landed.

---

## Archive Contents

| Artifact | Location | Status |
|---|---|---|
| proposal.md | archived | ✅ Complete — intent, scope, approach, rollback |
| spec.md | `openspec/specs/seller-credential-integrity/spec.md` | ✅ Complete — 10 requirements with W0 caveat resolved |
| design.md | archived | ✅ Complete — keyring, AEAD, guard landing, fallback discrimination |
| tasks.md | archived | ✅ Complete — 24 phases, all `[x]` checked |
| exploration.md | archived | ✅ Complete — materialized from Engram by orchestrator |

---

## Specs Synced to Main

**Action**: New capability — delta spec IS the full spec.

| Domain | Spec Path | Action | Details |
|---|---|---|---|
| seller-credential-integrity | `openspec/specs/seller-credential-integrity/spec.md` | Created | 10 requirements covering storage, rotation, accessor contract, and eight + one guard site consolidation |

**Verification**: Mechanical copy with diff readback — empty diff confirms byte identity.

---

## Must-Survive Artifacts (Verbatim)

### 1. Caveat Block on Requirement 2 — Migration vs. Operator Step

From `spec.md` (lines 59–77), the corrected wording that resolves verify-report W0:

```markdown
> **The conversion is an operator step, not a goose migration, and this wording
> was corrected at verify to say so.**
>
> It first read "a migration MUST convert every existing value in place", which
> is not what shipped. `db/migrations/009_mp_credentials_nonempty.sql` is
> prove-then-constrain CHECKs only; the plaintext-to-ciphertext conversion is
> `cmd/mpcredkey seal`, run by an operator with the keyring in hand.
>
> That split is deliberate. SQL cannot produce this envelope — AES-256-GCM with
> an AAD binding each ciphertext to its row and column — and the one SQL route
> that could, pgcrypto, would put the encryption key into the query text, and
> from there into the statement log and `pg_stat_statements`. A migration that
> leaks the key while encrypting the data is not an improvement.
>
> Recorded rather than quietly reworded because the requirement's own verb was
> the claim: an archived spec promising a migration nobody wrote is a claim
> somebody will rely on. The consequence today is nil — nothing is deployed and
> there are no rows to convert — which is exactly why it had to be caught now
> rather than by the first operator to trust it.
```

**Why it matters**: This documents the two-step deployment contract (schema migration 009 + required operator `cmd/mpcredkey seal`) so nobody ships the schema constraint expecting automatic plaintext conversion and discovers operators must run a separate tool.

### 2. Four-Fallback Discrimination in Design — Apply-Time Warning

From `design.md` (lines 99–150), the deliberate asymmetry documented:

```markdown
### Decision: the platform-token fallback — fix one of four, not four of four

`token := c.accessToken; if x != "" { token = x }` appears **four** times, verified:
`mp.go:352` `CreatePreference`, `:395` `UpdatePreferenceExpired`, `:423` `GetPayment`,
`:467` `RefundPayment`. Removing it everywhere breaks webhook confirmation; leaving it
everywhere leaves the money hazard. The discriminator is not the idiom, it is **what
MercadoPago does with a platform token**:

| Site | MP's behaviour on `""` | Callers |
|---|---|---|
| `:352` `CreatePreference` | **Accepts and settles money to the platform.** Silent wrong payee | `public.go:624,635` only, always a seller |
| `:423` `GetPayment` | Accepts, *correctly* — the app owner may read any payment under its `app_id` | `webhook.go:334`, the sole non-test caller, always `""` |
| `:395` `UpdatePreferenceExpired` | Rejects — wrong account owns neither preference | `cron.go:174,179`, `cancel.go:107`, deliberately `""` |
| `:467` `RefundPayment` | Rejects | `refund.go:274,504`, `process.go:314` via `getSellerToken`, `""` argued at `:429-440` |

> **Apply-time warning — consistency across these four is the wrong instinct.**
> An implementer who sees `CreatePreference` refuse an empty seller token will
> reasonably want to make the client uniform and apply the same refusal to `:395`,
> `:423` and `:467`. **Do not.** `mp.go:423` `GetPayment` *must* keep the fallback:
> `internal/payments/webhook.go:334` calls `GetPayment(ctx, mpPaymentID, "")` with a
> deliberately empty seller token, because MercadoPago's marketplace API lets the app
> owner read any payment under its own `app_id`, and that is the only way this method
> is ever called. Refusing there breaks webhook payment confirmation — the step that
> marks a booking paid. It would look like tidying and it would cost real bookings.
> `:395` and `:467` keep it too: both act on obligations that already exist, MP
> rejects them against the wrong account, and `refund.go:429-440` argues that trade
> explicitly. The asymmetry is the design, not an unfinished edit.
```

**Why it matters**: This is the second change in this program where a spec's own wording could tempt somebody to "fix" it into a silent data defect. Recording it means the apply-time comment (`webhook.go:334` named at `mp.go:423`) will survive review.

### 3. Nine Guard Sites — Final Count

From `state.yaml` (lines 123–128), the final count after orchestrator verification:

```yaml
# The count kept growing every time somebody looked, which is the finding.
# Exploration implied one guard. Proposal found eight. Design found NINE, and
# the orchestrator verified the new one: internal/bookings/public.go:626 tests
# MPRefreshToken != nil and :628 dereferences it into RefreshOAuthToken, so
# under ciphertext it ships an envelope to MercadoPago as a refresh token.
guard_sites: 9
```

**Why it matters**: This count (1 → 8 → 9 across phases) was computed three times because every pass counted again rather than trusting the last. Keeping this record stops the same recount from happening in a follow-up.

---

## Verification State

**Prior Verdict**: PASS WITH WARNINGS (verify-report issued 2026-08-21 pending W0 resolution)

**Blocking Issue (W0)**: Spec requirement 2 text claimed a goose migration would convert every plaintext row to ciphertext; the implemented migration 009 is constraint-only; conversion is `cmd/mpcredkey seal` (operator step).

**Resolution**: Spec requirement 2 reworded with caveat block (above) explaining the two-step contract. Spec now honestly describes what shipped.

**Warnings W1 and W2 — CLOSED before archive, in commit `6d39822`.**

This section was drafted from the verify report, which predates that commit.
Corrected at archive rather than left standing: an archive report that lists
closed gaps as open is the same drift this program keeps finding in specs and
comments, and it matters more here, because the archive is what a future reader
trusts once the change is out of everyone's head.

- **W1 — CLOSED.** `TestAnUnreadableSellerTokenIsReportedAsUnreadableNotMissing`
  drives the arm through `AutoRefundIfPaid` and asserts both halves: the alert
  says UNREADABLE, and it names the complex. Mutation-verified by collapsing
  `reason` to a constant `"MISSING"`, with the mutated tree confirmed to compile
  before the failure was accepted as RED.
- **W2 — CLOSED.** `stubComplexes.UpdateMPCredentials` returned `nil`
  unconditionally, which is why that branch was unreachable — a stub that cannot
  fail deletes the coverage of every error path hanging off it. It takes an
  injectable error now, and `TestCheckoutRetryReportsAFailedCredentialPersist`
  asserts the alert fires and names the complex. Mutation-verified by deleting
  the `sentry.CaptureMessage` call and leaving only the log line.
- **A third arm of the same shape, absent from the verify report, also CLOSED.**
  `refund.go:446`'s UNAVAILABLE alert on a `GetByID` failure looked covered and
  was not: `payments_test.go` sets the complex store's error, but that case
  drives collector verification in `processApprovedPayment` and never reaches
  `getSellerToken`. `TestAnUnfetchableComplexIsReportedAsUnavailable` covers it,
  mutation-verified.

  All three end with the refund settling against the platform's own MercadoPago
  account, and the alert is the only thing that says so — but they are three
  different incidents. UNAVAILABLE means the database would not answer and a
  retry may succeed. UNREADABLE means every refund for that venue keeps settling
  wrong until somebody rotates a key. MISSING means the venue never connected.
  Collapsing them costs the alert its only purpose.

**Remaining warning** (advisory, non-blocking):
- W3: `scanCronBookings`'s decrypt wiring has no committed test. Verification
  proved it correct with a throwaway test and then removed it, so the behaviour
  is known-good and what is missing is the regression guard, not the knowledge.

**Gate Verdicts** (all green):
- `go build ./...` — exit 0
- `make test` — all packages ok, zero FAIL
- `golangci-lint run` — 0 issues
- `make test/integration` + `make test/security` — exit 0, zero FAIL
- `goose down/up` migration 009 — round-trips cleanly

**Task Completion**: All 24 phases checked `[x]`. Spot-checked against diff — every checked task's claimed code change present and matches description.

---

## Carried-Forward Work (Not Failures — Deliberate Scope Deferrals)

These items were evaluated and explicitly deferred as follow-ups, recorded here so the **why** survives:

### 1. `mp.Caller` Type (`AsSeller`/`AsPlatform`)

**What**: The design named this as the better shape and deferred it with its cost: two consumer-owned interfaces, their stubs and mocks, eight call sites.

**Why deferred**: Two chained slices against an 800-line review budget. The fallback discrimination at `:352` alone is what makes the change safe; the type refactor is what makes it stay safe. Splitting them lets the blocking defect close without absorbing the broader architectural work.

**Why it matters**: Prevents the fallback from being copied into a fifth method, which the current set of four makes possible (`:352` accepts `""`). A Caller type makes it unrepresentable.

**Prerequisite**: None — this is pure codebase improvement, not a blocker for anything else.

### 2. MercadoPago-Side Token Revocation on Disconnect

**What**: `DisconnectMercadoPago` clears local columns but does not call any MP OAuth revocation endpoint.

**Why deferred**: Feasibility against MercadoPago's API is unverified in this execution context (no network access). Scoping a deliverable on an unverified endpoint imports unbounded unknown into a change whose blocking constraint is elsewhere.

**Why it matters**: Leaked tokens remain usable at MP's side until they expire naturally. With nothing deployed, this is incident response for a hypothetical leak, not a live hazard.

**Next step**: First task is checking live MercadoPago OAuth API docs for a revocation endpoint.

### 3. `UNIQUE` on `mp_user_id`

**What**: One owner may legitimately hold several venues sharing one MercadoPago seller account (`GetComplexesByOwner`), so a global constraint would refuse a legitimate case.

**Why deferred**: The genuine invariant is "unique within one owner", which is an undecided product decision. A global constraint guesses the wrong boundary.

**Why it matters**: Migration 006 SECTION 4 made a similar guess (global UNIQUE on `mp_user_id`); that mistake is the exemplar for why this one stays deferred — not guessed again.

### 4. Sentry Test Double Duplication

**What**: The Sentry capture test double now exists twice:
- `internal/bookings/sentry_capture_test.go`
- `internal/payments/refund_test.go`

**Why**: Deliberate, not an oversight. Go test helpers cannot cross package boundaries.

**Why it matters**: Record this so nobody "consolidates" it into a shared non-test package later, which would either expose testing code to production or weaken the tests themselves.

---

## Program Umbrella Update

**Updated**: `openspec/changes/money-path-remediation-program/state.yaml`

| Field | Change | Reason |
|---|---|---|
| `mp-oauth-credential-integrity.status` | `pending` → `archived` | Change complete and archived |
| `mp-oauth-credential-integrity.date` | (added) `2026-08-22` | Archive date |
| `mp-oauth-credential-integrity.note` | (updated) | Records W0 resolution and prerequisite for `refund-durability-and-collector-integrity` now met |

**Downstream Impact**: `refund-durability-and-collector-integrity` listed this change as a prerequisite (cannot hard-fail `getSellerToken` until credentials are verifiably intact). Prerequisite now met; that change is unblocked but not started.

---

## Archive Checklist

- [x] All implementation tasks complete (`tasks.md` all `[x]`)
- [x] Verification passed (PASS WITH WARNINGS; W0 resolved by spec update)
- [x] No CRITICAL issues remaining
- [x] Delta spec synced to main specs (`openspec/specs/seller-credential-integrity/spec.md`)
- [x] Change folder moved to archive (`openspec/changes/archive/2026-08-22-mp-oauth-credential-integrity/`)
- [x] Archive readback verified (empty `diff -r` output)
- [x] Must-survive artifacts recorded in archive report (caveat block, fallback discrimination, guard count)
- [x] Program umbrella updated
- [x] Carried-forward work enumerated with rationale

---

## Risks & Recommendations

### No Blocking Risks
- The money-path defect is genuinely closed (mutation-verified)
- The compile-time guarantee (unexported fields) is real
- The ciphertext-at-rest guarantee is real
- The nil-keyring guarantee is real
- Key rotation needs no downtime or data migration

### Recommendations for Deployment
1. **Before deploying with existing production data**: Either (a) wire `cmd/mpcredkey seal` into the deploy runbook as a required step before migration 009 runs, or (b) add a pre-deploy checker that confirms zero plaintext rows exist after 009 runs. Migration 009 alone does not convert existing plaintext.
2. **Before enabling `refund-durability-and-collector-integrity`**: Confirm `getSellerToken` being hard-fail-safe (not silent) meets its requirements.

---

## Affected Source Files (Summary)

| File | Action | Impact |
|---|---|---|
| `internal/crypto/envelope.go`, `keyring.go` | Create | AEAD seal/open, kid parsing, ordered keyring |
| `internal/data/mpcred.go` | Create | Credential accessor methods, sentinels |
| `internal/data/complexes.go`, `bookings.go` | Modify | Unexport fields, decrypt/encrypt operations |
| `internal/bookings/public.go` | Modify | Guard relocation (sites 1, 2, 9) + Sentry on refresh |
| `internal/bookings/cancel.go`, `payments/refund.go`, `complexes/handlers.go`, `cmd/api/cron.go` | Modify | Guard relocation (sites 3–8) |
| `internal/mp/mp.go` | Modify | Delete fallback at `:352` only; others unchanged + comments |
| `db/migrations/009_mp_credentials_nonempty.sql` | Create | NULL-out-then-CHECK, schema constraint only |
| `cmd/mpcredkey/main.go` | Create | Operator binary for `seal` and `rekey` subcommands |
| `cmd/api/main.go` | Modify | Keyring config, flag→env, boot validation |
| Test files (6 files, 4 cross-package) | Modify | Unexport field handling, accessor coverage |

---

## Session Context

- **Execution Mode**: auto (SDD 6-phase cycle)
- **Artifact Store**: openspec (filesystem)
- **Delivery Strategy**: auto-chain (two chained PRs, stacked to main)
- **Review Budget**: 800 lines per PR (each slice under budget separately; combined well over 800)
- **Product Context**: never_deployed=true (no live data to migrate)

---

## Traceability

**Archive Folder**: `openspec/changes/archive/2026-08-22-mp-oauth-credential-integrity/`

**Main Spec**: `openspec/specs/seller-credential-integrity/spec.md`

**Change IDs**:
- mp-oauth-credential-integrity (SDD change)
- money-path-remediation-program (program umbrella, updated)

**Lineage**: This change is item #3 in the `money-path-remediation-program`, which was generated from the 108-finding audit of the padel-server's money path.
