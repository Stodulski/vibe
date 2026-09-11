# Verify Report: mp-oauth-credential-integrity

**Verdict: PASS WITH WARNINGS**

Both slices land cleanly, every gate is green, and the money-path defect
(`mp.go:352`'s platform-token fallback) is genuinely closed and mutation-tested.
The gaps below are coverage gaps on the loud-but-continue side paths and one
real spec/implementation mismatch on the "migrate existing rows" requirement —
none of them reopen the wrong-payee hazard the change exists to close, and none
touch the compile-time guarantee.

## Requirement Compliance Matrix

| # | Requirement | Verdict | Evidence |
|---|---|---|---|
| 1 | Credentials at rest are unreadable without key material | **SATISFIED** | `internal/crypto/envelope.go` AES-256-GCM; `TestIntegration_RawMPAccessTokenColumnIsNeverAUsableToken` (raw SELECT after `UpdateMPCredentials` returns `v1.k1....`, PASS at runtime against the E2E DB); `TestOpen_FlippedCiphertextByte`, `TestOpen_TruncatedEnvelope`, `TestOpen_WrongAAD`, `TestOpen_WrongKeyID` all PASS |
| 2 | Existing rows are migrated to ciphertext with no plaintext left behind | **NOT SATISFIED AS WRITTEN — spec overclaims** | `db/migrations/009_mp_credentials_nonempty.sql` is CHECK-constraint-only (NULL-out-then-CHECK), **not** a re-encrypt migration. The spec's own text says "A migration MUST convert every existing non-null value to ciphertext in place"; migration 009 does not do this — see "Spec Overclaim" section below |
| 3 | A key can be rotated with no maintenance window and no bulk data migration | **SATISFIED** | `crypto.Keyring` ordered multi-key, first-writes/every-opens; `TestParseKeyring_FirstEntryWritesEveryEntryOpens` PASS; independently reproduced live against the E2E DB with `cmd/mpcredkey rekey` — see "Independent Checks" §4 |
| 4 | A credential-read failure is a distinct outcome, never a value that passes an emptiness or nil check | **SATISFIED** | `ErrMPNotConnected`/`ErrMPCredentialUnreadable` sentinels, never a bare string; compile-time mutation reproduced independently (see §1 below) |
| 5 | Call sites that create a new payment obligation refuse on credential-read failure | **SATISFIED** | `public.go:239-246` refuses on both error arms before `:311`; `mp.go:352` (`CreatePreference`) independently refuses empty `SellerAccessToken` with `ErrNoSellerToken` — defense in depth; `TestPublicBookRefusesWhenTheComplexHasNoSellerCredential`, `TestPublicBookRefusesWhenTheComplexCredentialIsUnreadable`, `TestCreatePreference_RefusesEmptySellerAccessToken` all PASS |
| 6 | Ciphertext, or a value from an unproven decrypt, can never reach `CreatePreferenceInput.SellerAccessToken` | **SATISFIED** | Compiler-enforced (fields unexported, only accessor-produced values compile); mutation-verified both arms (Phase 14 not-connected, Phase 18 unreadable) — `stubCheckout.lastInput.SellerAccessToken` asserted empty in both tests, PASS |
| 7 | Call sites that read/modify an existing obligation stay at least as loud as today | **PARTIALLY TESTED** | `refund.go`'s "MISSING" arm well covered by many existing tests (`&data.Complex{ID: complexID}` with no token); the "UNREADABLE" arm (distinct reason string) has **zero runtime test coverage** anywhere in the suite — see WARNING W1 |
| 8 | "Is MercadoPago connected?" is never answered by testing the stored value for emptiness | **SATISFIED** | `complexes/handlers.go:433,708` now call `MPConnected()`; SQL predicate `mp_refresh_token != ''` deleted (`complexes.go:262` — only `IS NOT NULL` remains); `MPConnected()` itself is directly unit-tested for the unreadable arm (`TestComplexFromDB_UnreadableCredentialSurfacesAsUnreadable`) and end-to-end via `TestIntegration_GetWithMPConnectedSurfacesUnreadableRows`, PASS |
| 9 | `data.CronBooking` cannot deliver ciphertext to MercadoPago as a bearer token | **SATISFIED, weakly tested** | `scanCronBookings` routes through the same `openMPCredential` helper as `complexFromDB`; **no committed test exercises `scanCronBookings`'s Slice-2 decrypt wiring directly** — independently verified correct via a throwaway test (see "Independent Checks" §5), then removed; tree confirmed clean afterward. See WARNING W3 |
| 10 | Both refresh-persist paths are equally loud on failure | **NOT INDEPENDENTLY TESTED** | Cron side (`cron.go:345-351`) already tested; inline checkout-retry side (`public.go:631-641`, the new `sentry.CaptureMessage`) has **zero test coverage** — `stubComplexes.UpdateMPCredentials` in `internal/bookings/stubs_test.go:123-125` always returns `nil`, so the `credErr != nil` branch is unreachable from any test in the suite. See WARNING W2 |

## Spec Overclaim (requested finding)

**Requirement 2 as written in `specs/seller-credential-integrity/spec.md` is not what shipped.**

The spec text: *"A migration MUST convert every existing non-null `mp_access_token`
and `mp_refresh_token` value to ciphertext in place, and after it runs a raw
`SELECT` against either column MUST NOT yield a usable MercadoPago token for any
row that held one before the migration."* This was inherited near-verbatim from
the proposal's in-scope bullet: *"A single decrypt-all / re-encrypt-all migration
(nothing is deployed)."*

What actually shipped in `db/migrations/009_mp_credentials_nonempty.sql` is a
**CHECK-constraint migration only** (NULL-out-then-CHECK, migration 006's shape).
The actual plaintext-to-ciphertext conversion is `cmd/mpcredkey seal` — a
**separate, manually-invoked operator binary that is not part of the goose
migration chain and does not run automatically**. The migration file's own
header comment is explicit about this: *"This is not a re-encrypt migration...
Converting stored plaintext to the v1 envelope is `cmd/mpcredkey`'s job... this
migration only proves and then forbids a state... no code path is meant to
produce."*

The engineering reasoning for this substitution is sound (AES-GCM cannot run in
SQL without the key traveling through the query log), and it is currently
**harmless in practice** because `product_context.never_deployed: true` — there
are no existing rows to migrate, so the requirement's precondition ("GIVEN a
complex connected MercadoPago before the migration ran") is currently vacuous.
But the spec's literal MUST was not honestly walked back: if this were run today
against a database that already held plaintext credentials (which is exactly
what migration 009's own comment argues is impossible *right now*, not
impossible *in general*), `goose up` alone would leave those rows in plaintext,
directly contradicting Requirement 2's scenario ("Post-migration raw read yields
no plaintext token"). Nothing in the migration chain forces `cmd/mpcredkey seal`
to run, and nothing tests "migration 009 alone, given preexisting plaintext
rows, converts them" — because it provably does not.

**Recommendation**: before this ever reaches a database that could hold
pre-existing plaintext, either (a) amend `spec.md`'s Requirement 2 to describe
the actual two-step process (schema migration + manual operator conversion,
with the manual step as a hard deploy-order dependency), or (b) wire
`cmd/mpcredkey seal` into the deploy/migration runbook as a required step before
`009` is allowed to run against a database with existing rows. This does not
block archive given the current never-deployed context, but it should not be
silently carried forward as "done" against the spec's literal words.

## Independent Checks Against the User's Four Claims

1. **Compile-time guarantee** — CONFIRMED. Wrote a throwaway test in
   `internal/bookings` writing `c.MPAccessToken != nil && *c.MPAccessToken != ""`
   against `*data.Complex{}`; `go vet ./internal/bookings/...` failed with
   `c.MPAccessToken undefined (type *data.Complex has no field or method
   MPAccessToken)`. File removed, tree confirmed clean after (`git status
   --short` empty).
2. **Ciphertext at rest** — CONFIRMED independently, two ways: (a) the committed
   integration test `TestIntegration_RawMPAccessTokenColumnIsNeverAUsableToken`
   passed at runtime against the E2E DB; (b) manually seeded a plaintext row
   (`APP_USR-1234567890abcdef-081512-realsellertoken` /
   `TG-refreshtoken-abcdef`), ran `cmd/mpcredkey seal`, confirmed via `psql` the
   column became `v1.k1.OwDrR4JLjdx3UM_Mmn4mrFD51UDLO5gQSuP403LcU9MDwsdmorOMi37Ps-...`.
   Row deleted afterward.
3. **AAD binding under attack** — CONFIRMED via source + passing tests:
   `MPCredAAD(complexID, column)` binds `"mpcred" 0x1F "v1" 0x1F complexID 0x1F
   column`; `TestOpen_WrongAAD` and the integration test
   `TestIntegration_GetWithMPConnectedSurfacesUnreadableRows` (which seals a
   credential under a foreign key and confirms the SQL-surfaced row correctly
   reports `ErrMPCredentialUnreadable` rather than silently opening) both PASS.
   Did not additionally reproduce the manual cross-copy repro described in the
   prompt — the committed AAD-mismatch tests exercise the identical failure
   mode and are stronger evidence (committed, reproducible) than a one-off
   manual repro would add.
4. **`mp.go` fallback sites** — CONFIRMED, with a line-number note: exactly
   three `token := c.accessToken` sites remain, at `mp.go:409` (`UpdatePreferenceExpired`),
   `:447` (`GetPayment`), `:491` (`RefundPayment`) — not `:395/:423/:467` as
   stated in the prompt. The line numbers shifted because Slice 1 added
   doc-comments naming the caller dependency at each site (`GetPayment`'s in
   particular, naming `webhook.go:334`); the three sites themselves and their
   behavior are unchanged. `CreatePreference` (`mp.go:269`) returns
   `ErrNoSellerToken` instead of falling back, confirmed.

## The Nil-Keyring Guarantee

Confirmed by source and test: `Keyring.Seal`/`Keyring.Open` both check `if k ==
nil { return "", ErrNilKeyring }` as their first line (method calls on a nil
pointer receiver are valid in Go and this branch runs correctly).
`openMPCredential` calls `keys.Open(...)` unconditionally — a nil `*Keyring`
produces `ErrMPCredentialUnreadable` (wrapped), never a plaintext
passthrough — proven by `TestComplexFromDB_NilKeyringNeverPassesStoredValueThrough`
(PASS). `UpdateMPCredentials` calls `m.Keys.Seal(...)` the same way — a nil
`m.Keys` fails loudly rather than silently storing plaintext. At boot,
`cmd/api/main.go:471-476` calls `crypto.ParseKeyring` and `os.Exit(1)`s on any
error before `NewModels` is ever called, so a misconfigured or nil keyring can
never reach a running server — the only path to `NewModels` with a valid `Keys`
is a successfully parsed keyring. No path in the codebase constructs
`data.Config{}` (zero-value `Keys`) outside of tests, and every test that
touches real credentials constructs a real test keyring
(`testCredentialKeyring`, `testKeyring32`).

## `cmd/mpcredkey` Idempotency

Both subcommands independently re-verified live against the E2E database
(rows created and cleaned up afterward, tree unaffected — this tool talks to
Postgres directly, not the repo):

- `seal` run twice: first run converted 1 complex, second run converted 0 —
  confirmed idempotent (the `isEnvelope` check in `convertColumn` correctly
  skips already-sealed values).
- `rekey` run twice under a two-key ring (`k2` new/active, `k1` retired): first
  run converted 1 complex (re-sealed `v1.k1...` → `v1.k2...`), second run
  converted 0 — confirmed the reported bug fix holds. `ActiveKeyID()` lets
  `alreadyUnderActiveKey` skip a value already sealed under the writing key,
  rather than re-sealing on every run (which the fresh-nonce-per-`Seal` issue
  would otherwise cause).

## Deviations From Design — Judged

Three deviations the apply agent flagged, judged against whether they widen
the attack surface:

- **`Keyring.ActiveKeyID()`** — exported to let `cmd/mpcredkey rekey` be
  idempotent (see above). Read-only, returns only a key id string (never key
  material), degrades safely on a nil receiver. **No material surface
  widening** — this is exactly the kind of narrow, purpose-built export the
  idempotency bug required.
- **`data.MPCredAAD`, `MPAccessTokenColumn`, `MPRefreshTokenColumn` exported**
  — required so `cmd/mpcredkey` (outside `internal/data`) can reproduce the
  exact AAD byte sequence; without this export the operator tool could not
  exist at all, since AAD mismatch is a hard decrypt failure by design. The
  alternative (duplicating the AAD construction logic inside `cmd/mpcredkey`)
  would be strictly worse — two independent implementations of a
  security-critical byte sequence that must never drift. **Justified.**
- None of the three exports create a plaintext-reachable path from outside
  `internal/data`; the credential fields themselves remain unexported and the
  accessor discipline (spec requirement 6) is unaffected.

## Gate Results (verbatim)

| Gate | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go build -tags integration ./...` | exit 0 |
| `make test` (`go test -race -v -count=1 ./...`) | all packages `ok`, zero FAIL |
| `go test -race -count=1 ./...` | covered by `make test` above, zero FAIL |
| `golangci-lint run` | `0 issues.` — confirmed meaningful: build was independently verified clean (including the `integration` tag) before trusting this count, per the prompt's warning |
| `make audit` | `go mod verify`: all modules verified; `govulncheck`: 0 vulnerabilities in code or imports (1 vulnerability in a required-but-uncalled module, not reachable) |
| `make test/integration` (`-tags integration -race -v -count=1 -p 1 ./...`) | exit 0, every package `ok`, zero FAIL, including both new mp-credential integration tests (`TestIntegration_RawMPAccessTokenColumnIsNeverAUsableToken`, `TestIntegration_GetWithMPConnectedSurfacesUnreadableRows`) |
| `make test/security` (run separately, after integration completed) | exit 0, every `TestSecurity_*` PASS, zero FAIL |
| `goose down` then `up` on migration 009 | round-trips cleanly: `down` drops all three CHECK constraints, `up` restores them (`goose: successfully migrated database to version: 9`), verified via `psql \d complexes` before/after |

## Issues

### CRITICAL
None.

### WARNING

- **W0 — Spec overclaim on Requirement 2.** See "Spec Overclaim" section above.
  Not currently exploitable (never deployed), but the spec's literal words and
  the shipped migration diverge, and nothing in the deploy path enforces the
  manual `cmd/mpcredkey seal` step before real data could exist.
- **W1 — `refund.go:449`'s `ErrMPCredentialUnreadable` arm is untested.**
  `getSellerToken`'s "MISSING" reason is exercised by many existing tests; the
  "UNREADABLE" reason (new in Slice 2) has no covering test anywhere. The
  control-flow shape (log + Sentry + return `""`) is identical between the two
  arms and is proven correct for one of them, so this is a coverage gap, not a
  behavioral unknown — but per spec requirement 7's own scenario text ("a
  refund is being processed for a complex whose stored... credential cannot be
  decrypted"), this exact scenario has zero passing covering test.
- **W2 — `public.go:631-641`'s refresh-persist-failure Sentry alert (spec
  requirement 10) is unreachable from any test.** `stubComplexes.UpdateMPCredentials`
  in `internal/bookings/stubs_test.go` unconditionally returns `nil`; there is
  no test double that can inject a persist failure, so the new
  `sentry.CaptureMessage` call this change added can never fire in the test
  suite. Requirement 10's own scenario ("the inline checkout retry alerts
  exactly as the cron does") is untested.
- **W3 — `scanCronBookings`'s Slice-2 decrypt wiring (spec requirement 9) has
  no committed test.** `complexFromDB` got four dedicated Slice-2 unit tests
  (`TestComplexFromDB_*`); `scanCronBookings`, which routes through the same
  `openMPCredential` helper, got none. `state.yaml`'s note that "Phase 18.1's
  coverage extends to it structurally, since it shares the same accessor" is a
  code-review argument, not a test. Independently verified correct with a
  throwaway test during this verification (round-trip + wrong-key arm both
  behaved correctly); the throwaway test was removed afterward and is not
  part of the codebase — a committed equivalent is recommended before this is
  fully spec-compliant on requirement 9's own terms.

### SUGGESTION
- Consider whether W1–W3 are worth one small follow-up commit (three small
  test additions, no production code changes) before archive, given how far
  ahead of this bar the rest of the change already is — Phase 14/18's
  mutation-verified tests are the strongest evidence in the whole change, and
  these three gaps are the only place that rigor did not fully carry through
  to the loud-but-continue side paths.

## Task Completion

All 24 phases in `tasks.md` are checked `[x]`. Spot-checked against the actual
diff and current tree for phases 2, 3, 4, 6, 7, 8, 9, 11, 14, 17, 18, 19, 20,
21, 22 — every checked task's claimed code change is present and matches its
description. No task claims something the code does not deliver, **except**
that task 19's own text ("A migration MUST convert... to ciphertext in place")
inherits the spec's Requirement 2 overclaim discussed above; task 19.1 as
executed is honest about what it actually built (its own migration comment
explicitly disclaims being a re-encrypt migration), so the gap is in
`spec.md` not having been walked back to match, not in the task execution
itself.

## Recommendation

**Do not archive yet — resolve W0 first; W1–W3 are advisory.**

The compile-time guarantee, the ciphertext-at-rest guarantee, the AAD-under-attack
guarantee, and the nil-keyring guarantee are all real, all independently
reproduced, and all exactly as strong as claimed. This is a genuinely careful
piece of work. The blocking issue is narrower than a CRITICAL code defect: it
is that `specs/seller-credential-integrity/spec.md` still states a literal MUST
("A migration MUST convert every existing non-null value to ciphertext in
place") that migration 009 does not implement and that nothing in the deploy
path enforces. Before archiving:

1. Amend `spec.md`'s Requirement 2 to describe what actually shipped (a schema
   migration plus a required manual operator step), or explicitly scope the
   full automatic conversion as a follow-up gated on first deploy — whichever
   the team decides is the honest contract going forward.
2. W1–W3 do not need to block archive — they are coverage gaps in
   already-correct, already-loud code paths, not open defects — but should be
   tracked (e.g., as tasks in the next change) rather than silently dropped.
