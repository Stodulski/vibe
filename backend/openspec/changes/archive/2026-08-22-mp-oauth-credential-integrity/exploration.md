# Exploration — mp-oauth-credential-integrity

Produced by the `sdd-explore` phase agent, which had no filesystem write tool in its
execution context (Read/Grep/Glob/codegraph/mem_* only). It persisted to Engram
(`sdd/mp-oauth-credential-integrity/explore`, id 34) and the orchestrator materialized
this file. Same gate outcome as the parent program's exploration, and for the same
reason — worth fixing in the phase agent's tooling rather than paying the round trip
each time.

## Orchestrator verification

Four load-bearing claims were re-checked against source before this artifact was
accepted. All four hold, and one is worse than the exploration framed it.

| Claim | Verdict |
|---|---|
| `mp_access_token` / `mp_refresh_token` are plaintext | Confirmed — bare `TEXT`, `db/migrations/001_initial_schema.sql:99-100`, no encryption anywhere in the write path |
| `mp_user_id` has no unique constraint | Confirmed — `001_initial_schema.sql:101`, and migration 006 added several tenant constraints but not this one |
| `CreatePreference` falls back to the platform token | Confirmed, and **understated**: see below |
| Checkout is fail-closed today | Confirmed — `internal/bookings/public.go:239` refuses on nil/empty before `:311` dereferences |

### The platform-token fallback is the default, not an error path

`internal/mp/mp.go:352-355`:

```go
token := c.accessToken            // the PLATFORM's token
if input.SellerAccessToken != "" {
    token = input.SellerAccessToken
}
```

The exploration describes this as a fallback. It is the initial value. Nothing marks
the empty-seller-token case as exceptional, nothing logs it, and the request proceeds
normally — so a preference created with an empty seller token settles real money into
the platform's own MercadoPago account instead of the venue's. That is a wrong-payee
bug with real reversal difficulty, not a 500 the client retries.

### The guard that prevents it today does not survive this change by default

This is the single most important constraint on the design, and it is not obvious.

`public.go:239` checks `complex.MPAccessToken` for nil/empty. Once credentials are
encrypted, that field holds ciphertext — non-nil and non-empty — so **the guard passes
on exactly the input it exists to reject**. If a decrypt failure then yields `""` and
that value flows on, `SellerAccessToken: ""` reaches `CreatePreference` and the
platform token is used silently.

So this change cannot treat encryption as a transparent storage detail. The decrypt
step has to collapse into the same refusal the nil/empty check produces today, at
every call site that creates a new payment obligation. Design must state where that
check moves to and prove the old guard is not left checking a value that can no longer
be empty.

---

## 1. What is stored, where, in what form

`complexes` table (`db/migrations/001_initial_schema.sql:99-101`):

```sql
mp_access_token    TEXT,
mp_refresh_token   TEXT,
mp_user_id         TEXT,
```

Plaintext. `internal/data/complexes.go:209-217` (`UpdateMPCredentials`) is a bare
parameterized `UPDATE` with no encryption call. No `mp_token_expiry` column exists in
any of the eight migrations: the OAuth response's `expires_in` (`internal/mp/mp.go:96`,
`OAuthTokens.ExpiresIn`) is decoded and then discarded.

Go side (`internal/data/complexes.go:36-38`):

```go
MPAccessToken     *string   `json:"-"`
MPRefreshToken    *string   `json:"-"`
MPUserID          *string   `json:"mp_user_id,omitempty"`
```

The `json:"-"` tag does more work than it appears to — see §5.

## 2. Every read and write path

**Writes** — all four go through `ComplexModel.UpdateMPCredentials`, an unconditional
overwrite with no optimistic lock and no prior-value check:

1. `internal/complexes/handlers.go:620-668` `ConnectMercadoPago` — OAuth code exchange.
2. `internal/complexes/handlers.go:671-697` `DisconnectMercadoPago` — clears to `NULL`.
3. `cmd/api/cron.go:315-360` `cronRefreshMPTokens` — every 12h, for every connected
   complex, regardless of expiry (none is tracked).
4. `internal/bookings/public.go:623-642` `createMPPreferenceWithRetry` — reactive
   refresh on a 401 from `CreatePreference`.

**Reads**:

1. `internal/complexes/handlers.go:701-715` `MercadoPagoStatus` — boolean plus
   `mp_user_id` to the owner, not the token.
2. `internal/complexes/handlers.go:414-435` `newPublicComplex` — only a boolean
   (`payments_enabled`) reaches the public unauthenticated endpoint. The raw
   `mp_user_id` used to leak here; `handlers.go:377-388` documents that fix.
3. `internal/bookings/public.go:239,311` — seller token for checkout preference creation.
4. `internal/bookings/cancel.go:104-105` — seller token to expire a preference.
5. `internal/payments/refund.go:429-456` `getSellerToken` — seller token for refunds.
6. `internal/payments/process.go:402-441` `collectorMatchesComplex` — reads `MPUserID`,
   not the tokens, to verify the payment's collector.
7. `internal/payments/webhook.go:334` — reads the payment with the **platform** token,
   which is correct for MercadoPago's marketplace API (the app owner can read any
   payment under its `app_id`). Not a gap; recorded so nobody re-flags it.
8. `cmd/api/cron.go:157-184` `cronReleaseExpiredPayments` — seller token to expire a
   preference when a payment window lapses.

## 3. Refresh and expiry

Both refresh paths call `RefreshOAuthToken` (`internal/mp/mp.go:148-188`) and persist
unconditionally. Neither records an expiry or a last-refreshed timestamp, so the cron
refreshes everything blind every 12h. MercadoPago tokens last months, so the margin is
generous — but there is no signal at all if that cron stops running.

**The two paths differ in loudness on persist failure, and that asymmetry matters**
because MercadoPago rotates the refresh token on use:

- `cron.go:345-353` — persist failure gets `slog.Error` *and* `sentry.CaptureMessage`.
- `public.go:631-633` — the inline retry during a live checkout gets only
  `logger.Error`, no Sentry. If MP has already invalidated the previous refresh token
  and this persist fails, the stored refresh token is dead with no loud signal. The
  next cron run alerts, up to 12h later.

**Expiry mid-payment today**: the nil/empty case is already refused at
`public.go:239`. An expired-but-present token surfaces as a 401, caught by
`createMPPreferenceWithRetry`, retried once after a refresh; a second failure
propagates as an ordinary `ServerError` with no distinct "seller temporarily
unreachable" signal and no per-tenant isolation. That isolation belongs to
`circuit-breaker-correctness`, not here.

## 4. Blast radius if the database leaked tomorrow

For every connected complex, DB read access is immediate authentication as that
venue's MercadoPago seller: create and modify preferences, read and refund any payment
on that account, and — via the refresh token — keep doing so indefinitely, outliving
any password the venue owner rotates, since MP OAuth tokens are independent of this
platform's credentials.

Per tenant the compromise is total and permanent. Across tenants there is no shared
secret, so one row does not expose another.

`DisconnectMercadoPago` clears the local columns only; it never calls MercadoPago's own
revocation endpoint. A token captured in an earlier leak therefore stays usable at MP's
side after a venue disconnects here. Design should confirm whether MP exposes a revoke
endpoint and call it.

## 5. What already protects them — verified, not assumed

Listed so no one proposes work that is already done.

- **`json:"-"` protects two surfaces with one mechanism.** It blocks API-response
  leakage, and it blocks the audit trail: the generic complex `Update` handler passes
  the whole `*data.Complex` as `newVal` (`handlers.go:296`), and `audit.Recorder.encode`
  (`internal/audit/audit.go:125-140`) marshals with `json.Marshal`, which honours the
  tag. Confirmed by reading the encode path. The `mp_connect`/`mp_disconnect` records
  pass `nil, nil` anyway.
- **Public projection** already reduces `mp_user_id` to a boolean.
- **Sentry** (`cmd/api/sentry.go:79-101`) has a live regex for `APP_USR-`/`TEST-`/`TG-`
  tokens plus a generic named-secret scrubber that catches `access_token` and
  `refresh_token` in JSON-shaped strings. Every `CaptureMessage` touching MP
  credentials interpolates only `complex_id`, an error or a count — never a token — so
  the scrubber is defence in depth here rather than the only thing standing between a
  token and Sentry.

## 6. Adjacent gap, in or out of scope

`mp_user_id` has no unique constraint. The same MercadoPago seller account can be
linked to two different complexes at once. It does not break the per-booking collector
check, which is scoped to one complex's row, but it is a real integrity gap in the
table this change is already touching, and migration 006 established this project's
"prove it is zero rows, then constrain" convention. `sdd-propose` decides in or out.

## Design questions — framed, not decided

**Keying.** The parent exploration's non-goals already exclude KMS integration, and
Railway has no first-party KMS, so the fork is narrower than it first looks:

- **A. One static key from an env var**, following the existing pattern in
  `cmd/api/main.go:204-271` (flag first, env override). Same trust tier as `JWT_SECRET`,
  `MP_CLIENT_SECRET` and `R2_SECRET_KEY`, already in that store — no new class of
  exposure. Rotation means decrypt-all and re-encrypt-all in one migration: free today,
  not free after launch.
- **B. Same, plus a key id stored with each ciphertext** and a keyring of current and
  previous keys. Rotation becomes a live operation with no maintenance window.

The recommendation is B, and the reason is the constraint at the top of this document
turned around: "cheap because nothing is live" is the argument *for* building the
rotation path now, while it is still a pure code decision and not also a data migration.

**Algorithm.** `go.mod` carries no symmetric-crypto dependency; `golang.org/x/crypto`
is present only for bcrypt. Stdlib `crypto/aes` + `crypto/cipher` (AES-256-GCM) covers
this with zero new dependencies and matches a lean `go.mod`. AES-GCM's authenticated
tag also means tampering and corruption are detected at decrypt rather than producing
garbage that gets used — which is what makes the failure-mode question answerable.
`chacha20poly1305` is an equivalent zero-new-dependency alternative; no evidence either
way on AES-NI availability, so it is a coin flip for design, not a real fork.

**Failure mode.** The sharpest question, and §Orchestrator verification above is why.
Recommended shape for design to accept or reject:

> A decrypt failure is treated identically to "never connected" at every call site that
> creates a new payment obligation — refuse, do not fall back. At call sites that only
> read or modify an existing obligation, alert loudly and refuse; MercadoPago itself
> rejects a refund presented against the wrong account, so those fail loud-but-safe at
> the provider.

The asymmetry is deliberate: refusing a checkout costs one booking and a retry;
falling back on a checkout settles a real payment into the wrong account.

## Dependency note

This change is a prerequisite for `refund-durability-and-collector-integrity`. That
ordering is confirmed: making `getSellerToken` hard-fail-closed before tokens are
verifiably intact would convert "silently uses a possibly-wrong token" into "silently
refuses a possibly-fine token", which is worse without this underneath it.

## Known noise

`sqlc generate` on this tree currently produces an unrelated stray `WebhookEvent` diff
(migration 006 documents this). If this change needs a `db/queries/*.sql` edit, expect it.
