# Archive Report: `cmd-api-new-application-extraction`

**Change**: `cmd-api-new-application-extraction`  
**Archived**: 2026-08-21  
**Status**: Complete — archived after pass-with-warnings verification  
**Location**: `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/`

## Executive Summary

The change has been fully planned (proposal, spec, design, tasks), implemented (7 commits, all 25 tasks checked), verified (pass-with-warnings with explicit archive recommendation), and archived. The new `application-composition-root` capability spec has been synced to the main specs directory at `openspec/specs/application-composition-root/spec.md`. The change folder was moved to the archive via mechanical `git mv` and verified with empty `diff -r`.

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| `application-composition-root` | Created | New 6-requirement capability spec (146 lines) with documented implementation gaps |

## Must-Survive Items Carried Forward

### 1. Caveat Block on "Incorrect Wiring Order Fails to Compile" (Requirement 4)

This caveat is preserved verbatim from the archived spec at `openspec/specs/application-composition-root/spec.md` (lines 82–108):

> **DELIVERED WITH A KNOWN LIMIT — recorded because the archived spec has to be true, and because this requirement's own worked example is the case that escapes it.**
>
> Locals-then-publish makes wrong *order* a build error: moving the bookings block above `paymentsHandler`'s yields `undefined: paymentsHandler` (mutation-verified at `cmd/api/app.go:306`). Scenario 1 holds.
>
> It does not make wrong *source* one. `app` exists as a zero-valued struct from the start of the constructor, so `Refunds: app.payments` — the exact assignment this requirement cites — stays syntactically valid at any point, compiles clean, passes the whole suite, and hands `bookings` a nil, because `paymentsHandler` is built at `:297` and `app.payments` is not published until `:378`. A nil `*payments.Handler` in an interface field reads as non-nil, so nothing downstream can detect it either.
>
> Closing it means each module's `NewHandler` validating its own `Dependencies` — the shape `validateDeps` already applies to the 15 `data.Models` stores, pushed down one level. Considered and declined in design.md's rejected alternatives, and named in `cmd/api/app_test.go`'s own comments so the next reader of the code finds it without reading this file.
>
> Accepted as a bounded, documented gap rather than closed here: it crosses every module, and burying that in a composition-root change would make the change unreviewable.

### 2. Slice-2 Evidence (from state.yaml, lines 59–77)

The critical observation that motivated this change — that `make test/security` had never executed before — is preserved in the archived state documentation:

> `make test/security` was run for the first time and every test in it panics before its first assertion: newIntegrationApp builds an *application literal that never sets `middleware`, so routes.go:31 -> middleware.Wrap -> ratelimit.go:117 dereferences nil. The whole cmd/api integration suite has therefore never executed, including all of security_test.go.
>
> This is the same defect class as the nil *notifications.Service that broke user registration: a struct field left unset because nothing forces it. It is exactly what slice 2 exists to remove, and it raises that slice from housekeeping to the reason the change is worth doing.

## Archive Contents

- ✅ `proposal.md` (77 lines) — Intent, scope, capabilities, approach, affected areas, risks, rollback, dependencies, success criteria
- ✅ `specs/application-composition-root/spec.md` (146 lines) — 6 requirements, 12 scenarios, purpose, out-of-scope
- ✅ `design.md` (108 lines) — Technical approach, architecture decisions, data flow, file changes, interfaces, testing strategy, threat matrix, migration/rollout
- ✅ `tasks.md` (74 lines) — 25 tasks across 8 phases (0–7) + Phase 8 (slice 2); all checked
- ✅ `verify-report.md` (91 lines) — Independent verification, 8 gates run and passed, per-requirement verdict, findings, prior slice-2 catalogue
- ✅ `state.yaml` — Complete session and phase state, slices definition, review workload guard, chain strategy resolved

## Verification Gate Status

All 8 gates re-run independently at verification time and passed:
- ✅ `go build ./...` clean
- ✅ `make test` (29/29 packages ok)
- ✅ `golangci-lint run` (0 issues)
- ✅ `make audit` (0 vulnerabilities in reachable code)
- ✅ `make test/integration` (full suite green, 237.6s)
- ✅ `make test/security` (17/17 tests green)
- ✅ Route authorization tests (zero-line diff across all 5 commits)
- ✅ Authorization matrix (`TestRouteAuthorizationMatrix` and friends pass unchanged)

**Verdict**: `pass-with-warnings` — all gates green; documented gaps accepted per design decisions.

## Task Completion

- **Total tasks**: 25/25 checked across phases 0–8
- **Phases**: All 8 complete
- **Commits**: 7 commits landed (4004cf2, 22c5a61, b76361d, 8b88421, d910dc9, 926c894, 7a10674)
- **Current tree**: Clean at f170199

**Bookkeeping note**: `state.yaml` reported `phases.apply.status: ready` with an `unblocked_note` dated before the commits landed (a gap in state documentation, not in implementation). This gap was corrected — see note below.

## State Corrections Made Before Archive

- ✅ Updated `state.yaml` to record that apply is complete (committed)

## Post-Archive Follow-Ups (Carried Forward, Not Failures)

These items were discovered during this work and belong to follow-up decisions and changes, not to this change's scope:

### 1. Wrong-Source Wiring Still Compiles

**Owner**: Product/engineering  
**Description**: The implementation achieves "wrong ORDER fails to compile" but not "wrong SOURCE fails to compile." Closing this gap means adding validation to each module's `NewHandler`, which was deliberately declined here because it would cross every module and make this change unreviewable.

**Evidence**: Spec Requirement 4, lines 82–108 (the caveat block above), and `cmd/api/app_test.go` line 149–154.

**Proposed follow-up**: Small change to add per-`NewHandler` `Dependencies` validation across ~12 modules, plus a test that passes a `miniredis`-backed `deps.rdb` through `newApplication` to close Requirement 6 Scenario 2 (untested Redis composition path).

### 2. WhatsApp Reply Handling Descope or Regression

**Owner**: Product  
**Description**: `internal/bookings/whatsapp.go`'s `normalizeResponse` function is not called by any production code. During this session, it is unclear whether this was deliberately descoped or represents a regression in reply-path handling.

**Evidence**: Code exists; no call site in production or integration tests.

**Proposed follow-up**: Clarify intent with product; if active, restore/create call site; if descoped, mark as such or delete.

### 3. Booking Link Credential Hardening — Product Decision Pending

**Owner**: Product  
**Description**: The `booking-link-credential-hardening` change in the remediation program is blocked on a product decision: should booking links rotate or expire?

**Evidence**: State documented in `openspec/changes/money-path-remediation-program/state.yaml`.

**Proposed follow-up**: Product decision on rotation vs. expiration; then SDD or direct delivery.

## Program Umbrella Status

Updated `openspec/changes/money-path-remediation-program/state.yaml` to reflect:
- ✅ `cmd-api-new-application-extraction`: archived, 2026-08-21
- ℹ️ `operational-observability`: delivered directly (commit a141155)
- ℹ️ `notification-delivery-durability`: largely delivered directly
- ℹ️ `tenant-isolation-integrity`: largely delivered via migration 006
- ⏳ `mp-oauth-credential-integrity`: pending SDD
- 🔒 `booking-link-credential-hardening`: blocked on product decision
- ✅ 31 direct-delivery changes: grouped by phase, mutation-verified

## Mechanical Archive Verification

All copy and move operations used shell mechanicals (`cp -R`, `git mv`, `diff -r`) with empty `diff` output confirming byte-identity:

1. **Spec copy to main specs**:
   - Source: `openspec/changes/cmd-api-new-application-extraction/specs/application-composition-root/spec.md`
   - Destination: `openspec/specs/application-composition-root/spec.md`
   - Diff result: **empty** (exit 0)

2. **Change folder move to archive**:
   - Source snapshot created before move
   - Moved via `git mv` to: `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/`
   - Diff result: **empty** (exit 0)
   - Source directory confirmed absent after move

## Next Steps

- If the archived spec's documented gaps require follow-up, open new SDD changes for:
  - Per-`NewHandler` validation + miniredis test (small, ~50 lines)
  - Possibly as scoped follow-up, not urgent
- Product to resolve booking-link-credential-hardening blockers
- Remaining 5 SDD changes and 31 direct-delivery changes continue per the program's phases

## Artifacts Archived

- `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/proposal.md`
- `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/design.md`
- `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/specs/application-composition-root/spec.md`
- `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/tasks.md`
- `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/verify-report.md`
- `openspec/changes/archive/2026-08-21-cmd-api-new-application-extraction/state.yaml`
- `openspec/specs/application-composition-root/spec.md` (main specs, new)
- `openspec/changes/money-path-remediation-program/state.yaml` (updated)

---

**Archive created by**: sdd-archive executor  
**Mode**: openspec  
**Final state authority rank**: Verify-report (pass-with-warnings) + explicit final-state facts in launch prompt + archived artifacts  
**Change ready for deployment**: Yes, subject to the listed follow-up clarifications
