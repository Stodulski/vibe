# Proposal: MercadoPago Seller Credential Integrity

## Intent

`mp_access_token` and `mp_refresh_token` are bare `TEXT`
(`db/migrations/001_initial_schema.sql:99-100`), written by a plain `UPDATE`
(`internal/data/complexes.go:213-216`). A single database read is immediate,
indefinite authentication as every connected venue's MercadoPago seller account
— create preferences, read payments, issue refunds — and, via the refresh token,
permanently, because MP OAuth tokens are independent of any password the owner
can rotate.

**This change moves a guard, not just a column.** `internal/mp/mp.go:352`
initialises the outgoing token to the *platform's* own access token and
overrides it only when `SellerAccessToken != ""` (`:353-355`). An empty seller
token is therefore the default path, logs nothing, and settles real money into
the platform's account. What prevents that today is
`internal/bookings/public.go:239`, which refuses on nil-or-empty before `:311`
dereferences. Ciphertext is non-nil and non-empty — **that guard would pass on
exactly the input it exists to reject.** Cost of getting this wrong is not a
retryable 500; it is a payment settled to the wrong payee.

Eight predicates read "is it empty?" to mean "is it connected?" and all eight
stop being answerable from the stored field:
`bookings/public.go:239`, `payments/refund.go:449`, `bookings/cancel.go:104`,
`complexes/handlers.go:433`, `complexes/handlers.go:708`, `cmd/api/cron.go:171`,
`cmd/api/cron.go:328`, and the SQL predicate at
`internal/data/complexes.go:241-242`.

## Scope

### In Scope

- Authenticated encryption at rest for both token columns, with a versioned key
  identifier carried per ciphertext so rotation is a live operation.
- A single decrypt-all / re-encrypt-all migration (nothing is deployed).
- **The guard relocation.** A decrypt failure MUST collapse into the same
  refusal the nil/empty check produces today at every site that creates a new
  payment obligation (`public.go:239/311`), and the old predicate MUST NOT be
  left checking a value that can no longer be empty. Sites that only read or
  modify an existing obligation keep their documented loud-but-continue policy
  (`refund.go:429-440` argues that trade explicitly; MP rejects a refund
  presented against the wrong account).
- The second decode surface: `data.CronBooking`
  (`internal/data/bookings.go:669-678`) carries raw tokens selected at `:692`
  and `:722` and has **no** `json:"-"` tags. Ciphertext reaching `cron.go:172`
  as a bearer token must be impossible.
- Sentry on refresh-persist failure at `internal/bookings/public.go:631-633`,
  matching `cmd/api/cron.go:345-351`. MP rotates the refresh token on use, so a
  lost persist kills the stored one; today only the cron is loud, up to 12h
  later. Three lines, in a function this change already rewrites.

### Out of Scope

- **`UNIQUE` on `mp_user_id` — argued down, not overlooked.** `GetComplexesByOwner`
  (`db/queries/complexes.sql:22-26`) means one owner may hold several venues, and
  sharing one MP seller account across them is ordinary. `complexes` is
  soft-deleted (`001:102`), so any constraint would have to be partial.
  `collectorMatchesComplex` (`internal/payments/process.go:415-436`) is scoped to
  one complex's row and does not need it. A global `UNIQUE` would refuse a
  legitimate multi-venue owner: that is migration 006 SECTION 4's mistake —
  encoding a product guess as a schema invariant. The real invariant ("only
  within one owner") is a product decision; filed as a follow-up, not guessed here.
- **Token revocation on Disconnect.** `DisconnectMercadoPago` clears local
  columns only (`internal/complexes/handlers.go:688` →
  `internal/data/complexes.go:220-226`), so a previously-captured token stays
  usable at MP's side. **I could not check MercadoPago's OAuth documentation:
  this execution context has no network access, and the repository contains no
  revocation call or reference** (`internal/mp/mp.go` exposes only
  `ExchangeOAuthCode:101` and `RefreshOAuthToken:148`). Scoping a deliverable
  whose feasibility is unverified would import an unbounded unknown into a change
  whose blocking constraint is elsewhere. It is also incident response for a
  leak that already happened, and with nothing deployed there is no captured
  token to revoke — unlike the rotation path, which is only cheap *before* first
  deploy. Follow-up; its first task is verifying the endpoint against live docs.
- **Recording `expires_in`** (decoded at `internal/mp/mp.go:96`, discarded).
  A 12h refresh against a ~6-month lifetime (`cmd/api/cron.go:315-316`) is a
  ~360× margin; the real signal is "did the cron run", which belongs to
  `operational-observability` (already delivered). Cost here is a migration plus
  a `sqlc` regeneration carrying the known stray `WebhookEvent` diff
  (migration 006:122-132) inside a change whose reviewable core is the guard move.
- KMS integration, per-tenant keys, staged rollout, dual-write windows.
- Per-tenant circuit-breaker isolation for an unreachable seller
  (`circuit-breaker-correctness`).

## Capabilities

### New Capabilities
- `seller-credential-integrity`: how MercadoPago seller credentials are stored,
  keyed, rotated, and — the load-bearing half — what every consumer must do when
  a credential cannot be produced intact.

### Modified Capabilities
- None. `application-composition-root` is unaffected: the cipher derives from
  `cfg`, and `data.Models` is assembled by the caller before `newApplication`,
  which its existing requirements already permit.

## Approach

Encryption sits behind a credential accessor, not inside a column. Every
consumer asks for a usable seller token and receives either one or a typed
refusal; no consumer inspects the stored field. That is what makes the eight
emptiness predicates disappear rather than silently invert.

Delivery is two chained slices, ordered so no intermediate commit is unsafe:

1. **Fail-closed access, still plaintext.** Introduce the accessor and route all
   eight sites through it. Behaviour-preserving, fully testable, and it puts the
   guard in its final position *before* any value becomes opaque.
2. **Encrypt at rest behind that accessor**, plus keyring and migration.

The reverse order would ship ciphertext under a guard that no longer works —
precisely the defect. Design owns the keyring shape, the algorithm
(stdlib AES-256-GCM and `chacha20poly1305` are both zero-new-dependency), and
the accessor's exact placement.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/crypto/` (new) | New | Authenticated envelope encrypt/decrypt, versioned keyring |
| `internal/data/complexes.go` | Modified | `:209-226` write paths; `:238-242` connected-predicate; `:327-329` decode |
| `internal/data/bookings.go` | Modified | `:669-678`, `:692`, `:722` — the `CronBooking` surface |
| `internal/bookings/public.go` | Modified | `:239`, `:311`, `:626-639` — the money-critical guard and refresh persist |
| `internal/payments/refund.go` | Modified | `:441-456` — decrypt failure enters the existing loud path |
| `internal/bookings/cancel.go`, `cmd/api/cron.go`, `internal/complexes/handlers.go` | Modified | Remaining emptiness predicates |
| `db/migrations/009_*.sql` | New | Re-encrypt existing rows |
| `cmd/api/main.go` | Modified | Key material config, flag-then-env per `:204-271` |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| A guard is left checking a value that can no longer be empty | High if untreated | Slice 1 relocates all eight before anything is encrypted; a test must prove ciphertext cannot reach `CreatePreferenceInput.SellerAccessToken` |
| Decrypt failure silently yields `""` and the platform token is used | Med | Typed refusal, never a bare string; the accessor cannot return a usable-looking empty value |
| Key lost or misconfigured → every venue's payments stop | Med | Fail-closed and loud by design; startup validates key material before serving |
| Diff exceeds the review budget | High | Two chained slices; see Review Workload |
| `make sqlc` emits the unrelated `WebhookEvent` diff | Med | Avoid regeneration; hand-written SQL already covers these paths |

## Rollback Plan

Slice 2 reverts by running migration 009 down (decrypt-all back to plaintext)
and reverting the encryption commit; the accessor from slice 1 keeps working
because it never assumed ciphertext. Slice 1 reverts as a pure code revert — it
touches no data. Nothing is deployed, so neither rollback has a live-traffic
window.

## Dependencies

- Key material available as an environment variable on Railway (same trust tier
  as `JWT_SECRET`, `MP_CLIENT_SECRET`, `R2_SECRET_KEY`).
- Blocks `refund-durability-and-collector-integrity`, which must not hard-fail
  `getSellerToken` until credentials are verifiably intact.

## Success Criteria

- [x] Both token columns hold ciphertext; a raw `SELECT` yields nothing usable at MP.
- [x] A mutation-verified test proves an undecryptable credential refuses checkout
      instead of falling through to the platform token at `mp.go:352`.
- [x] No consumer reads `MPAccessToken`/`MPRefreshToken` directly; all eight
      former predicates are gone or expressed through the accessor.
- [x] Rotating to a new key needs no downtime and no data migration.
- [x] `data.CronBooking` cannot deliver ciphertext to MercadoPago as a bearer token.
- [x] `make test`, `golangci-lint run`, and the `integration`-tagged suite pass.
