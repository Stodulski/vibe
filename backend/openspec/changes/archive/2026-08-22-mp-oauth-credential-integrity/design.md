# Design: MercadoPago Seller Credential Integrity

## Technical Approach

The raw columns stop being readable outside `internal/data`. `Complex.MPAccessToken`
/ `MPRefreshToken` and the same pair on `CronBooking` become **unexported**, populated
at the single decode points (`complexes.go:306` `complexFromDB`, `bookings.go:738`
`scanCronBookings`) and reachable only through accessors that return `(string, error)`.
The compiler then proves success criterion 3 rather than a reviewer asserting it: an
emptiness predicate on the stored field cannot be written outside the package.

Slice 1 does this while values are plaintext (open is identity) and relocates every
site. Slice 2 makes open a real decrypt. No store interface changes: the accessors
live on the value, so `ComplexMPManager` (`models.go:120-125`),
`complexes.Store:24-34` and `bookings.ComplexReader:45-49` keep their `string`
signatures — zero ISP churn.

## Architecture Decisions

### Decision: Keyring — ordered multi-key env var, kid carried per ciphertext

**Choice**: `-mp-credential-keys` flag, `MP_CREDENTIAL_KEYS` env override
(`main.go:206-209` / `:262-273` pattern). Value: `kid:base64key[,kid:base64key...]`.
**First entry writes; every entry opens.** Boot validates (≥1 entry, 32 raw bytes each,
unique kids) then `sentry.Flush` + `os.Exit(1)` on failure, exactly like the
`JWT_SECRET` check at `main.go:434-443`. Rotation: prepend the new key → deploy →
run `cmd/mpcredkey rekey` → drop the old key next deploy. No window, no migration.

**Envelope (byte for byte, ASCII, stored in the existing `TEXT` column):**

```
"v1" "." kid "." base64.RawURLEncoding( nonce || sealed )
  kid    = /^[A-Za-z0-9_-]{1,16}$/   (no ".", so SplitN(s,".",3) is exact)
  nonce  = 12 bytes, crypto/rand, fresh per Seal
  sealed = AES-256-GCM output = ciphertext(len == len(plaintext)) || tag(16)
  AAD    = "mpcred" 0x1F "v1" 0x1F complexID 0x1F column
  column = "mp_access_token" | "mp_refresh_token"
```

AAD binds each ciphertext to its row and its column, so copying a value between
complexes or between the two columns fails at open. `complexID` is available at every
seal and open site (`UpdateMPCredentials` takes it; `db.Complex.ID` and
`Booking.ComplexID` carry it). ~132 chars for a typical MP token.

**Rejected**: single static key with no kid — re-keying then needs a decrypt-all
maintenance step, free today and never again after first deploy, which is the whole
argument for paying now. Per-tenant keys / KMS — out of scope, no Railway KMS.

### Decision: Cipher — stdlib AES-256-GCM

**Choice**: `crypto/aes` + `crypto/cipher`. Zero new dependencies, AEAD, so tamper,
corruption and wrong-key all surface as an error at open instead of bytes that get
used — which is what makes the guard below answerable.

| Alternative | Rejected because |
|---|---|
| `x/crypto/chacha20poly1305` | Equivalent security; loses only on stdlib-preference and AES-NI on the amd64/arm64 target |
| AES-CBC + HMAC | Hand-rolled encrypt-then-MAC, more to get wrong, no gain |
| pgcrypto in SQL | Key travels in query text into the log and `pg_stat_statements` |
| Deterministic (AES-SIV) to keep the SQL predicate | Leaks which venues share a token, and does not save the predicate anyway |

Random 96-bit nonces are safe at this volume (a few writes per complex per day).
**A nil keyring errors on every Seal/Open** — it is never a plaintext passthrough,
because "silently not encrypted" is the failure this change exists to remove.

### Decision: Where the guard lands

Three outcomes, never a bare string: `ErrMPNotConnected` (nothing stored),
`ErrMPCredentialUnreadable` (stored, could not be opened), or a token.
`MPConnected()` reports `false` for both error cases — a venue whose credential
cannot be opened genuinely cannot take payments.

Split by consequence, per the proposal: **creating a new payment obligation refuses;
touching an existing one stays loud-but-continue.**

| # | Site (today) | Lands as |
|---|---|---|
| 1 | `bookings/public.go:239` `MPAccessToken == nil \|\| *… == ""` | Same position, same 400. `tok, err := complex.SellerAccessToken()`; **refuses on both errors**, `ErrMPCredentialUnreadable` also `sentry.CaptureMessage`. This is the blocking constraint. |
| 2 | `bookings/public.go:311` `SellerAccessToken: *complex.MPAccessToken` | `SellerAccessToken: tok` from #1. Dereference gone. |
| 3 | `payments/refund.go:449` | `SellerAccessToken()`; keeps the `:429-440` trade — `slog.Error` + Sentry + `return ""`. Message distinguishes missing from unreadable. |
| 4 | `bookings/cancel.go:104` | Best-effort preference expiry; on error log and keep `sellerToken = ""` (platform token is safe for `UpdatePreferenceExpired`). |
| 5 | `complexes/handlers.go:433` `PaymentsEnabled` | `c.MPConnected()`. |
| 6 | `complexes/handlers.go:708` `connected` | `complex.MPConnected()`. |
| 7 | `cmd/api/cron.go:171-172` (`CronBooking`) | `b.SellerAccessToken()`; log + `""` on error, same contract as #4. Unexported field also closes the missing-`json:"-"` gap by construction. |
| 8 | `cmd/api/cron.go:328` + `:332` | `c.SellerRefreshToken()`; absent → silent skip, **unreadable → `failed++` + Sentry**. |
| 9 | `bookings/public.go:626,628` — **not in the proposal's eight** | `SellerRefreshToken()`. Sends an envelope to MP as a refresh token otherwise. Same function gets the Sentry line the proposal scoped for `:631-633`. |

**The SQL predicate (`data/complexes.go:241-242`).** `mp_refresh_token != ''` is
always true against an envelope, so it is deleted; only `IS NOT NULL` remains.
Presence is the one thing SQL can still answer; integrity moves to Go, where the
`cron.go:327` loop now filters on `SellerRefreshToken()` succeeding. That is strictly
better than the SQL filter: an unreadable row is now surfaced and alerted instead of
silently dropped from the result set. Migration 009 makes the deleted half provably
redundant with `CHECK (mp_access_token <> '')` and the same on `mp_refresh_token` /
`mp_user_id`, so "empty means disconnected" stops being representable.
`UpdateMPCredentials` also refuses to seal an empty input, so the CHECK is a floor,
not the guard.

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

So the hazard is "empty is ambiguous **and** the provider silently accepts it at exactly
one of the four." The mechanism belongs there and nowhere else. One of the four is the
defect, two are deliberate tolerance for *existing* obligations, and one has a caller
actively relying on the fallback.

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

**Choice**: at `:352` the fallback branch is **deleted**, not guarded —
`token := input.SellerAccessToken; if token == "" { return nil, ErrNoSellerToken }`.
`c.accessToken` stops being referenced in `CreatePreference` at all, so there is no
hazardous idiom left at that site for a fifth method to be copied from. The other
three keep the fallback and gain one line each naming the caller that relies on it, so
"platform token" is a documented deliberate state at three sites and an unrepresentable
one at the fourth.

**Alternative considered and deferred — and it is the better design.** Stop spelling
"platform" as `""`: an `mp.Caller` type with `AsSeller(token) (Caller, error)` and
`AsPlatform() Caller`, so the two meanings stop sharing a spelling and the *class*
becomes unrepresentable rather than this instance. **Cost, stated rather than
absorbed**: it changes two consumer-owned interfaces (`bookings.Checkout:75-77`,
`payments.PaymentProvider:97-98`), their stubs (`bookings/stubs_test.go:193,208`) and
mocks, and eight call sites — inside a change whose reviewable core is the guard move,
already split into two chained slices against an 800-line budget. **This widens the
change beyond what the proposal scoped** (which listed `internal/mp/mp.go` not at all),
so it is flagged as a follow-up rather than taken here. The deletion at `:352` is what
this change needs to be safe; the type is what the codebase needs to stay safe.

## Data Flow

```
UpdateMPCredentials(complexID, access, refresh)
      │ refuse empty ─→ ErrMPCredentialEmpty
      └─ Keyring.Seal(AAD{complexID,column}) ──→ "v1.kid.<b64>" ──→ TEXT column

SELECT ─→ complexFromDB / scanCronBookings ─→ Keyring.Open ─→ Complex.mp{access,refresh,err}
                                                                   │
   SellerAccessToken() ──┬── (token, nil) ────────→ CreatePreferenceInput
                         ├── ErrMPNotConnected ────→ 400 refuse   (public.go:239)
                         └── ErrMPCredentialUnreadable ─→ 400 refuse + Sentry
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/crypto/envelope.go`, `keyring.go` | Create | AEAD seal/open, kid parsing, ordered keyring. MP-agnostic. |
| `internal/data/mpcred.go` | Create | `SellerAccessToken`, `SellerRefreshToken`, `MPConnected`, sentinels, test constructor |
| `internal/data/complexes.go` | Modify | `:36-37` unexport; `:209-217` seal + refuse empty; `:241-242` drop the `!= ''` half; `:327-329` open |
| `internal/data/bookings.go` | Modify | `:675-676` unexport; `:738-769` open |
| `internal/data/models.go` | Modify | `Config.Keys`; `ComplexModel`/`BookingModel` get the keyring in `NewModels` |
| `internal/bookings/public.go` | Modify | Sites 1, 2, 9 + Sentry on refresh-persist failure (`:631-633`) |
| `internal/bookings/cancel.go`, `internal/payments/refund.go`, `internal/complexes/handlers.go`, `cmd/api/cron.go` | Modify | Sites 3–8 |
| `internal/mp/mp.go` | Modify | `:352-355` **only** — delete the fallback, refuse empty `SellerAccessToken`. `:395`, `:423`, `:467` keep theirs unchanged (see the apply-time warning); `:423` gains a comment naming `webhook.go:334` as the caller that depends on it |
| `db/migrations/009_mp_credentials_nonempty.sql` | Create | NULL-out-then-CHECK, migration 006 SECTION 2 shape |
| `cmd/api/main.go` | Modify | `cfg.mp.credentialKeys`, flag→env, boot validation |
| `cmd/mpcredkey/main.go` | Create | `seal` (plaintext→v1, dev DBs) and `rekey` (kid→kid). Precedent: `cmd/benchmark` |
| 4 test files constructing `data.Complex{MPAccessToken:…}` | Modify | `complexes/handlers_test.go:446`, `bookings/handlers_test.go:235`, `payments/payments_test.go:288`, `cmd/api/cron_test.go:107` |

## Interfaces / Contracts

```go
// internal/data/mpcred.go
var ErrMPNotConnected, ErrMPCredentialUnreadable, ErrMPCredentialEmpty error

func (c *Complex) SellerAccessToken() (string, error)
func (c *Complex) SellerRefreshToken() (string, error)
func (c *Complex) MPConnected() bool           // false for BOTH error cases
func (b *CronBooking) SellerAccessToken() (string, error)

// internal/crypto
func ParseKeyring(spec string) (*Keyring, error) // "kid:b64[,kid:b64]" — first writes
func (k *Keyring) Seal(aad []byte, plaintext string) (string, error)
func (k *Keyring) Open(aad []byte, envelope string) (string, error) // nil *Keyring => error
```

## Testing Strategy

| Layer | What | How |
|---|---|---|
| Unit | Seal/open round-trip; wrong kid, wrong complexID in AAD, flipped tag byte, truncated envelope, plaintext input all return errors; nil keyring errors | table tests in `internal/crypto` |
| Unit | `CreatePreference` refuses `SellerAccessToken == ""` | `internal/mp` |
| Unit | **Guards the asymmetry**: `GetPayment(ctx, id, "")` still sends the platform token and succeeds; same for `UpdatePreferenceExpired` and `RefundPayment`. Fails the moment somebody "makes the four consistent" | `internal/mp`, httptest server asserting the `Authorization` header |
| Unit | Webhook confirmation still marks a booking paid with a stub whose `GetPayment` is called with `""` | `internal/payments` |
| Unit | **Mutation-verified**: an unreadable credential produces a 400 at `public.go:239` and no `CreatePreference` call. Delete the guard → test must fail | `internal/bookings`, existing stub checkout records inputs |
| Unit | Sites 4, 7 degrade to `""`; site 8 counts and alerts on unreadable | `bookings`, `cmd/api` |
| Integration | Raw `SELECT mp_access_token` returns `v1.…`, never an MP token; `GetWithMPConnected` returns rows the Go loop then rejects | `//go:build integration` |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file
classification, or process-integration boundary. `cmd/mpcredkey` is a separate
operator binary; `cmd/api`'s boot path gains config validation only, no new mode.

## Migration / Rollout

Slice 1 touches no data. Slice 2's migration 009 asserts
`SELECT count(*) FROM complexes WHERE mp_access_token = '' OR mp_refresh_token = ''
OR mp_user_id = ''` is 0, NULLs any stragglers, then adds the CHECKs — migration 006's
prove-then-constrain convention. No `sqlc` regeneration: every touched query is
hand-written, so the stray `WebhookEvent` diff is avoided. Nothing is deployed;
`cmd/mpcredkey seal` converts developer and E2E databases. Down: drop the CHECKs,
`cmd/mpcredkey` has no down mode because there is no production ciphertext.

## Open Questions

- [ ] Is the keyring required in `development`, or only `production`? Design assumes
      required unconditionally, matching `JWT_SECRET` at `main.go:434`.
- [ ] Should `MercadoPagoStatus` (site 6) tell the owner "connection broken" rather
      than "not connected"? Both lead to reconnect; an unreadable credential is an
      operator incident, so it alerts to Sentry and reports `false` here.
