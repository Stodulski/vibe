# Seller Credential Integrity Specification

## Purpose

Defines the observable contract for how a connected venue's MercadoPago OAuth
credentials (`mp_access_token`, `mp_refresh_token`) are stored, read, rotated,
and — the load-bearing half — what every consumer must do when a credential
cannot be produced intact. Today both columns are plaintext `TEXT`
(`db/migrations/001_initial_schema.sql:99-100`), and eight call sites decide
"is MercadoPago connected for this complex" by testing that stored value for
nil or emptiness. Once the column holds ciphertext, every one of those eight
predicates is non-nil and non-empty by construction and would pass on exactly
the input each exists to reject — most dangerously at
`internal/mp/mp.go:352-355`, which initializes an outgoing request to the
*platform's own* access token and only overrides it when the seller token is
non-empty, so a value that decrypts to nothing settles real money into the
platform's MercadoPago account instead of the venue's. This specification
pins the behaviours that keep that guard working once the column is opaque;
it does not choose the cipher, the key-material shape, or the accessor's
exact signature — those are design's to decide.

## Requirements

### Requirement: Credentials at rest are unreadable without key material

The system MUST store `mp_access_token` and `mp_refresh_token` as
authenticated ciphertext. A value read directly from the `complexes` table —
a raw `SELECT`, a database dump, a backup, a replica — MUST NOT be usable as
a MercadoPago bearer token. The encryption MUST be authenticated (tamper- and
corruption-detecting): a stored value that has been altered, truncated, or
encrypted under a key the system no longer holds MUST fail to decrypt rather
than decrypt into a different, unintended plaintext.

#### Scenario: A raw column read yields nothing usable at MercadoPago
- GIVEN a complex has connected MercadoPago and its `mp_access_token` column
  holds the stored value
- WHEN that raw column value, read outside the credential accessor, is used
  as an HTTP `Authorization: Bearer` value against MercadoPago's API
- THEN MercadoPago rejects it, because it is ciphertext, not the seller's
  OAuth access token

#### Scenario: A single altered byte is detected, not silently decrypted
- GIVEN a stored credential's ciphertext has one byte flipped after write
  (corruption, or a byte-level storage error)
- WHEN the system reads that credential
- THEN the read fails with a decryption/authentication error; no plaintext
  string is produced from the tampered bytes

### Requirement: Existing rows are converted to ciphertext with no plaintext left behind

Every existing non-null `mp_access_token` and `mp_refresh_token` value MUST be
converted to ciphertext in place, and afterwards a raw `SELECT` against either
column MUST NOT yield a usable MercadoPago token for any row that held one
before.

The conversion MUST be idempotent: running it again over already-converted rows
MUST convert nothing and MUST NOT re-seal what is already current.

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

#### Scenario: Post-conversion raw read yields no plaintext token
- GIVEN a complex connected MercadoPago before the conversion ran
- WHEN `cmd/mpcredkey seal` completes and the `mp_access_token` column is read
  directly, bypassing the credential accessor
- THEN the value returned is ciphertext, not the original plaintext token

#### Scenario: Running the conversion twice changes nothing the second time
- GIVEN every row's credentials are already sealed under the active key
- WHEN `cmd/mpcredkey seal` runs again
- THEN it reports zero rows converted and no ciphertext is rewritten

### Requirement: A key can be rotated with no maintenance window and no bulk data migration

Rotating the active key material MUST NOT require re-encrypting every
existing row before service resumes, and MUST NOT require stopping traffic.
A credential encrypted under a previously active key MUST remain readable
after rotation, for as long as that key remains available to the system.

#### Scenario: Rotating in a new key does not break existing credentials
- GIVEN credentials already stored under the currently active key material
- WHEN new key material is introduced and becomes the active key for new
  writes
- THEN previously stored credentials continue to decrypt successfully
  without first being re-encrypted, and the service serves payment traffic
  throughout the rotation with no downtime

### Requirement: A credential-read failure is a distinct outcome, never a value that passes an emptiness or nil check

When a stored credential cannot be turned back into its original plaintext —
wrong or retired key, corruption, tampering — the system MUST surface that as
an outcome distinguishable from "connected with a usable value" at every
consumption point. It MUST NOT be representable as a value that any
nil-check or empty-string check (the shape every one of today's eight
predicates uses) would treat as "present and usable".

#### Scenario: A decrypt failure cannot pass the historical nil/empty guard
- GIVEN a complex's stored credential ciphertext cannot be decrypted
- WHEN code that used to gate on `complex.MPAccessToken == nil ||
  *complex.MPAccessToken == ""` (`internal/bookings/public.go:239`) asks for
  that complex's seller access token
- THEN the outcome it receives is treated as "not usable" by that check or
  its replacement — never a value that satisfies "is present and non-empty"

### Requirement: Call sites that create a new payment obligation refuse on credential-read failure

At every call site that would create a *new* MercadoPago payment obligation,
a credential-read failure MUST produce the same refusal the nil/empty check
produces today, before any MercadoPago API call is made. The platform's own
access token MUST NOT be substituted in as a result of that failure. This
applies at minimum to public checkout preference creation
(`internal/bookings/public.go:239` refusing before `:311` dereferences the
token into `mp.CreatePreferenceInput.SellerAccessToken`).

Refusing here costs one booking and a client retry. Falling back would settle
a real payment into the platform's own MercadoPago account instead of the
venue's — a wrong-payee error, not a retryable failure.

#### Scenario: Checkout refuses instead of falling back to the platform account
- GIVEN a public booking checkout for a complex whose stored MercadoPago
  credential cannot be decrypted (never connected, or decrypt failure)
- WHEN the booking handler attempts to create the MercadoPago payment
  preference for that booking
- THEN it refuses before any request reaches MercadoPago, the booking is not
  left pending against a preference created under the platform's account,
  and the client sees the same "MercadoPago not connected" outcome the
  nil/empty check produces today

### Requirement: Ciphertext, or a value from an unproven decrypt, can never reach `CreatePreferenceInput.SellerAccessToken`

No code path MUST be able to set `mp.CreatePreferenceInput.SellerAccessToken`
to anything other than a value that has itself been produced by a
successful, verified decryption, or to leave it unset so the caller's
refusal path (above) applies. It MUST NOT be possible to assign the raw
stored column value, or a decrypt-failure sentinel, into that field.

#### Scenario: A test can assert this directly
- GIVEN a test harness that makes the credential accessor return a
  decrypt-failure outcome for a given complex, instead of a plaintext token
- WHEN the code path that builds `mp.CreatePreferenceInput` for that complex
  runs
- THEN the resulting `CreatePreferenceInput.SellerAccessToken` is never set
  to the accessor's raw internal representation of that failure, and
  `mp.CreatePreference` is never invoked with it — the surrounding call site
  refuses per the requirement above instead

### Requirement: Call sites that read or modify an already-created obligation stay at least as loud on failure as they are today

At call sites that only read or modify a payment obligation that already
exists — a refund, a preference expiry — a credential-read failure MUST be
surfaced at least as loudly as the equivalent "not connected" case is
surfaced today, and MUST NOT be silently swallowed into a quieter path than
exists now. Where today's behaviour already documents an explicit
loud-alert-and-continue trade (`internal/payments/refund.go:429-456`'s
`getSellerToken`, which logs and sends a Sentry message before the caller
proceeds — MercadoPago itself then rejects a refund presented against the
wrong account), a credential-read failure MUST be routed into that same
loud path, not a new silent one. This applies at minimum to
`internal/payments/refund.go:449`, `internal/bookings/cancel.go:104`, and
`cmd/api/cron.go:171` (`cronReleaseExpiredPayments`).

#### Scenario: A refund's credential failure is at least as loud as today
- GIVEN a refund is being processed for a complex whose stored MercadoPago
  credential cannot be decrypted
- WHEN `getSellerToken` (or its equivalent post-change) is asked for that
  complex's seller token
- THEN the failure is logged and a Sentry message is sent, exactly as the
  "complex has no MP access token" case is today, and the refund attempt is
  not left with no signal that a seller-scoped credential was unavailable

### Requirement: "Is MercadoPago connected?" is never answered by testing the stored value for emptiness

Determining whether a complex currently has a usable MercadoPago connection
MUST NOT be implemented as a check that the stored credential column is
non-null and non-empty, in application code or in SQL. Presence of a
non-null value in `mp_access_token` or `mp_refresh_token` is not the same
question as "does this value decrypt to something usable", and a check that
conflates them MUST NOT be relied on to gate a payment-affecting decision.
This applies at minimum to the connectivity booleans at
`internal/complexes/handlers.go:433` (`newPublicComplex`'s
`PaymentsEnabled`) and `:708` (`MercadoPagoStatus`'s `connected`), the
refresh-skip check at `cmd/api/cron.go:328`, and the SQL predicate at
`internal/data/complexes.go:241-242` (`mp_refresh_token != ''` inside
`GetWithMPConnected`).

A complex that has never connected MercadoPago, or that explicitly
disconnected (`ClearMPCredentials` sets both columns `NULL`,
`internal/data/complexes.go:220-226`), MUST still report "not connected"
under whatever replaces these checks — this requirement changes what a
*present* value is allowed to mean, not what an absent one means.

#### Scenario: A present-but-undecryptable credential is not reported as connected
- GIVEN a complex's `mp_access_token` column holds a non-null, non-empty
  value that fails to decrypt
- WHEN application code or a SQL query answers "is MercadoPago connected for
  this complex" for the purpose of gating a payment-affecting decision
- THEN presence of that non-null, non-empty value is not by itself treated
  as "connected and usable"

#### Scenario: A genuinely disconnected complex still reports not connected
- GIVEN a complex that has never connected MercadoPago, or was disconnected
  via `DisconnectMercadoPago`
- WHEN the same connectivity check runs
- THEN it reports "not connected", exactly as it does today

### Requirement: `data.CronBooking` cannot deliver ciphertext to MercadoPago as a bearer token

`data.CronBooking` (`internal/data/bookings.go:669-678`) is a second decode
surface carrying `MPAccessToken`/`MPRefreshToken`, populated by
`GetForReminder2hEnriched` (`:692`) and `GetExpiredPendingEnriched` (`:722`),
independent of `data.Complex` and its `json:"-"` tags. When a cron job uses a
`CronBooking`'s credential fields as a bearer token against MercadoPago (for
example `cmd/api/cron.go:171-174`, expiring a preference), the value used
MUST be the result of a successful, verified decryption — never the raw
stored ciphertext substituted in as if it were the plaintext token.

> Verified against source, not assumed: nothing currently marshals a
> `CronBooking` to JSON, returns it in a response, or passes it to the audit
> recorder — it is read only by the cron functions that consume it directly.
> The risk this requirement closes is latent, not a live leak: today's
> plaintext columns mean the raw value already happens to be the right
> bearer token, so nothing currently misuses it. It becomes live the moment
> the column holds ciphertext and this surface is not routed through the
> same accessor as `data.Complex`.

#### Scenario: A cron-sourced token is never raw ciphertext
- GIVEN a `CronBooking` whose `MPAccessToken` field was populated from an
  encrypted `mp_access_token` column value
- WHEN a cron job (e.g. `cronReleaseExpiredPayments`) uses that field as a
  bearer token in a MercadoPago request
- THEN the value used is either a successfully decrypted plaintext token, or
  the cron job has refused/skipped that booking per the applicable
  new-obligation or existing-obligation requirement above — never the raw
  ciphertext bytes

### Requirement: Both refresh-persist paths are equally loud on failure

MercadoPago rotates the refresh token on use, so a refresh that succeeds at
the provider but fails to persist locally strands the complex on a refresh
token that no longer works at MercadoPago's side. Both places that persist a
refreshed token MUST report a persist failure through the same class of
alert. Today `cmd/api/cron.go:345-351` sends both `slog.Error` and
`sentry.CaptureMessage`; `internal/bookings/public.go:631-633`'s inline
retry during a live checkout sends only `logger.Error`, so the same failure
there is silent until the next cron run, up to 12 hours later.

#### Scenario: The inline checkout retry alerts exactly as the cron does
- GIVEN the inline retry in `createMPPreferenceWithRetry` refreshes a
  complex's OAuth token after a 401 and then fails to persist the new
  refresh token
- WHEN that persist failure happens at `internal/bookings/public.go:631-633`
- THEN it produces a Sentry alert, matching what `cmd/api/cron.go:345-351`
  already produces for the equivalent persist failure — not only a log line

## Out of Scope

- **`UNIQUE` on `mp_user_id`.** Argued down in the proposal, not overlooked:
  one owner may legitimately hold several venues sharing one MercadoPago
  seller account (`GetComplexesByOwner`), and the real invariant ("unique
  within one owner") is an undecided product decision. A global constraint
  would refuse a legitimate case.
- **MercadoPago-side token revocation on `DisconnectMercadoPago`.** Feasibility
  against MercadoPago's OAuth API is unverified in this execution context
  (no network access); scoping a deliverable on an unverified endpoint is
  deferred as a follow-up.
- **Recording token expiry (`expires_in`, decoded and discarded at
  `internal/mp/mp.go:96`).** The 12h refresh cadence against a ~6-month token
  lifetime is a wide margin; the operationally relevant signal is whether the
  refresh cron ran at all, which `operational-observability` already covers.
- KMS integration, per-tenant keys, staged rollout, or dual-write windows —
  ruled out by the "nothing is deployed yet" context this change relies on.
- Per-tenant circuit-breaker isolation for an unreachable seller — belongs to
  `circuit-breaker-correctness`.
- The exact cipher, key-material/keyring shape, and credential accessor
  signature — design's to choose; this specification constrains only their
  observable behaviour.
