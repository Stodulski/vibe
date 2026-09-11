# Tasks: Refund Intent Durability and Collector Integrity

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~85 (slice 1) + ~140 (slice 2) + ~335 (slice 3) = ~560 total |
| 400-line budget risk | Medium — driven by slice 3 alone (~335 authored, migration + sqlc regen + two new test layers); slices 1-2 are Low individually |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (collector) → PR 2 (seller token) → PR 3 (marker + sweep) |
| Delivery strategy | auto-chain (per state.yaml) |
| Chain strategy | stacked-to-main — each slice reverts independently (rollback plan) and nothing is deployed, so sequential merge-to-main carries no live-traffic risk; the only ordering constraint (2 before 3) is exactly what sequential merging enforces |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

**The slice order is fixed and load-bearing, independent of the chain mechanics
above.** Slice 3's sweep calls `AutoRefundIfPaid`, which only refuses per-arm
seller-credential failures once slice 2 has landed; landing slice 3 first would
drive the pre-slice-2 platform-token fallback at 2-minute cron cadence instead of
once per cancellation. Do not reorder PR merges.

### Suggested Work Units

| Unit | Goal | PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Collector check moves ahead of every writing branch of `processApprovedPayment`, including the cancelled branch; recursion replaced by a direct call | PR 1 | `go test -race -v ./internal/payments/...` | N/A — pure unit coverage; no external MercadoPago call is exercised by this slice | pure code revert, no data touched |
| 2 | `getSellerToken` → `sellerCredential`; UNREADABLE/MISSING refuse before MercadoPago is called; UNAVAILABLE stays transient | PR 2, base=PR1 | `go test -race -v ./internal/payments/... ./internal/data/...` | N/A — provider is a stub in every existing test; no sandbox MercadoPago credential is available in this environment (state.yaml, unconfirmed premise) | pure code revert, reverses one archived requirement, no data touched |
| 3 | `refund_intent_at` column, cancel-write marker, `ClaimRefund`-clears-in-tx, reconciliation sweep, cron registration | PR 3, base=PR2 | `go test -race -v ./internal/bookings/... ./internal/data/... ./internal/payments/...` + `make test/integration` | `make e2e-db-up && make test/integration && make e2e-db-down` — the false-positive proof (Phase 17) and the concurrent-sweep proof (Phase 18) both need a real database | code revert + `make migrate-down` on 010; the dropped column is read by nothing else |

## Phase 0: Re-sync with concurrent edits
- [x] 0.1 Re-read `internal/payments/process.go`, `internal/payments/refund.go`,
      `internal/data/failed_refunds.go`, `internal/bookings/actions.go`,
      `internal/bookings/public.go`, `internal/bookings/cancel.go`,
      `internal/data/bookings.go`, `internal/data/refunds.go`,
      `internal/data/payments.go`, `internal/data/convert.go`,
      `internal/data/models.go`, `internal/payments/payments.go`,
      `cmd/api/cron.go`, `cmd/api/app.go`, `db/queries/bookings.sql` for drift
      since design.md was written. Confirm every file:line reference below still
      points at the code it names before editing anything. Confirm
      `db/migrations/` is still at `009_*` (010 is free).

# SLICE 1 — Collector Verification (PR 1, ~85 lines)

## Phase 1: Move the collector check ahead of every writing branch
- [x] 1.1 In `internal/payments/process.go`, hoist the `collectorMatchesComplex`
      call out of its current position after the amount check (`:92`) to sit
      immediately after the already-confirmed/completed/no_show skip (`:35-42`)
      and before the cancelled branch (`:46`). The skip itself (`:35-42`, no
      write, no money moved) needs no verification and stays first — do not hoist
      above it, that would add a `complexes` read to the hottest duplicate-webhook
      path for a branch that writes nothing.
- [x] 1.2 Add a comment at `:66` (the amount check,
      `actualCentavos := int(math.Round(...))`) explicitly naming
      `process.go:276-283` (`recordPaymentOwedARefund`'s deliberate use of
      `mpPayment.TransactionAmount`) as the reason this check does **not** move
      with the collector check. A deposit that changed since checkout would fail
      a recomputed comparison and refuse a legitimate refund. This guards against
      an implementer reading "verify before the branch writes" and moving both.
- [x] 1.3 Replace the recursive call at `:125`
      (`return h.processApprovedPayment(ctx, booking, mpPayment, mpPaymentID)`)
      with a direct `return h.refundCancelledBookingPayment(ctx, booking,
      mpPayment, mpPaymentID)` — exactly what the re-entry reaches once
      `booking.Status == "cancelled"` is re-checked at `:119`. This makes the
      collector check run once per webhook instead of twice, and removes a
      recursion from a money function along with the duplicate FRAUD-ALERT log
      line the second pass could produce.
- [x] 1.4 Confirm `collectorMatchesComplex` itself (`process.go:402-441`) is
      untouched — only its call site moves.

## Phase 2: Collector-integrity tests
- [x] 2.1 [RED→GREEN] Unit test in `internal/payments`: a cancelled booking
      (`status='cancelled'`) plus an approved webhook payment whose
      `CollectorID` does not match the complex's `mp_user_id` — assert no
      `data.Payment` row is written, `ClaimRefund` is never called,
      `booking.PaymentStatus` never moves toward `refund_pending`, and
      `RefundPayment` is never called (spec `payment-collector-verification`
      scenario 1). **Mutation, run and recorded**: write this test against
      pre-1.1 code (collector check still at `:92`, below the cancelled-branch
      return at `:61`) — it fails, the fictitious payment row is written. Apply
      1.1-1.3, re-run — it passes. Record both runs in the PR description.
- [x] 2.2 [RED→GREEN] Unit test: the concurrent-cancellation re-entry path
      (booking cancelled between validation and confirmation, `:119-125`) calls
      `refundCancelledBookingPayment` exactly once, with the collector check
      evaluated exactly once — assert via a call-counting stub on
      `collectorMatchesComplex`'s dependency. **Mutation**: restore the
      recursive call from 1.3 — the check now runs twice; the test must fail.
- [x] 2.3 Regression tests (spec scenarios 2 and 3): a cancelled booking whose
      payment collector matches its complex still records and refunds exactly as
      today; the normal pending-booking branch confirms exactly as today,
      unaffected by the relocation.
- [x] 2.4 Regression test (spec requirement "amount-equality check is not
      extended"): a booking cancelled after checkout, whose `DepositAmount` (or
      complex pricing) changed since, still has its already-cancelled-branch
      payment recorded from `mpPayment.TransactionAmount` and is not refused for
      an amount mismatch — proves 1.2's comment was not silently contradicted by
      an accidental amount-check hoist.

## Phase 3: Slice 1 verification — PR 1 boundary
- [x] 3.1 `go build ./...`, `go vet ./...`, `golangci-lint run` clean on changed
      files.
- [x] 3.2 `go test -race -v -count=1 ./internal/payments/...` green; full
      `make test` green.
- [x] 3.3 Confirm `process.go:402-441` (`collectorMatchesComplex`'s body) is
      byte-identical to before this slice — only its call site moved.

---

# SLICE 2 — Per-Arm Seller Token (PR 2, base=PR1, ~140 lines)

## Phase 4: `internal/data/failed_refunds.go` — extend the existing classifier
- [x] 4.1 Add one entry to `providerOutageMarkers` (`:75-85`): `"seller
      credential unavailable"`. No second classification function —
      `transientProviderFailure` (`:104-112`) itself is otherwise unchanged, and
      this string is matched only by the transient (`GetByID`-failed) arm's cause
      from Phase 5.

## Phase 5: `internal/payments/refund.go` — `sellerCredential` and refusal
- [x] 5.1 [RED] Write the three arm-distinguishing unit tests before touching
      `refund.go` (or in the same commit as 5.2-5.4, per this repo's TDD
      posture): UNREADABLE and MISSING never reach `RefundPayment` and alert on
      the first attempt with a credential-specific Sentry message; UNAVAILABLE is
      still classified by `transientProviderFailure` and does not spend the retry
      budget. All fail against current `getSellerToken`, which always returns
      `""` and never distinguishes them.
- [x] 5.2 [GREEN] Rename `getSellerToken` (`:441-461`) to
      `sellerCredential(ctx, complexID) (string, error)`. The `GetByID` failure
      (transient, UNAVAILABLE) returns the wrapped error unchanged. A non-nil
      `complex.SellerAccessToken()` error (`data.ErrMPNotConnected` — MISSING —
      or `data.ErrMPCredentialUnreadable` — UNREADABLE) returns that error
      unchanged. The provider is never called when `err != nil`; no `""` fallback
      remains inside `internal/payments`.
- [x] 5.3 [GREEN] Add `refuseForCredential(ctx context.Context, claim
      data.RefundClaim, err error) data.RefundOutcome`: `sentry.CaptureMessage`
      naming which arm occurred (UNAVAILABLE / UNREADABLE / MISSING) on this same
      attempt, then hand a cause string to `h.recordRefundFailure(ctx, claim,
      cause)` — the existing seam, no new classification rule. The UNAVAILABLE
      cause string must contain `"seller credential unavailable"` (matches 4.1);
      the other two must not.
- [x] 5.4 Update the three call sites to the two-line refusal pattern: `token,
      err := h.sellerCredential(ctx, booking.ComplexID); if err != nil { return
      h.refuseForCredential(ctx, *claim, err) }` — at `AutoRefundIfPaid`
      (`refund.go:273`), `refundCancelledBookingPayment` (`process.go:313`), and
      `RetryFailedRefunds` (`refund.go:505`, using its existing `claim` local and
      `fr.ComplexID`).
      Implemented as a shared `issueRefund(ctx, claim) *data.RefundOutcome`
      helper (get credential → refuse, or call provider → recordRefundFailure)
      used by all three call sites, rather than duplicating the two-line pattern
      three times — same observable behavior, required to keep
      `AutoRefundIfPaid` under golangci-lint's `funlen` budget. See "Deviations"
      in the apply report.
- [x] 5.5 Verification note for the PR description (no code change): confirm all
      three call sites already hold a committed claim/attempt row before this
      call — `ClaimRefund` (`AutoRefundIfPaid`) commits before the credential is
      asked for; `recordPaymentOwedARefund` + `ClaimRefund`
      (`refundCancelledBookingPayment`) commit before it there too;
      `RetryFailedRefunds`'s row already exists (fetched by `GetPendingDue`)
      before its own credential lookup. This is what makes every refusal queued
      rather than manual (spec scenario "a seller-token refusal always has a
      committed attempt row behind it"). Current source confirmed against
      function names rather than the design's line numbers, which had drifted
      by slice 1's edits (see "Line-number drift" below).

## Phase 6: Preference-expiry call sites stay unchanged
- [x] 6.1 Confirmed `internal/bookings/cancel.go:104`
      (`expireCheckoutPreference`) and `cmd/api/cron.go` (`cronReleaseExpiredPayments`,
      now at `:160`, not `:171` — drifted, re-verified against current source)
      are **not** touched by this slice — neither calls
      `sellerCredential`/`getSellerToken`; both call `SellerAccessToken()`
      directly on `*data.Complex`/`*data.CronBooking` and keep their existing
      loud-alert-and-continue trade, which the `seller-credential-integrity`
      delta explicitly leaves standing. `rg 'getSellerToken' --glob
      '!**/*_test.go'` returns nothing (fully renamed); `rg 'sellerCredential'`
      shows exactly the three call sites from 5.4 plus the shared `issueRefund`
      helper.

## Phase 7: Mutation-verified tests (design's testing table)
- [x] 7.1 [Mutation, run and recorded] UNREADABLE and MISSING: `RefundPayment`
      is never called, Sentry fires on attempt 1, the cause reaches
      `RecordRefundFailure`. Reverted `issueRefund` to ignore
      `sellerCredential`'s error and always call the provider (the pre-slice-2
      `""` fallback) — all three arm tests failed (provider called with `""`).
      Restored the fix — all three pass. See "Mutation evidence" in the apply
      report for both runs verbatim.
- [x] 7.2 [Mutation, run and recorded] UNAVAILABLE's cause satisfies
      `transientProviderFailure`; UNREADABLE's and MISSING's do not. Dropped the
      marker entry from 4.1, re-ran `TestTransientProviderFailure` — failed
      (UNAVAILABLE's cause no longer matched). Restored it — passes. See
      "Mutation evidence" in the apply report.
- [x] 7.3 Confirmed `mp.RefundPayment`'s own `""` fallback and the archived
      fallback-asymmetry guard test (`internal/mp/mp_test.go`, from
      `mp-oauth-credential-integrity`) remain green and unmodified —
      `internal/mp/mp.go` itself is untouched by this change (`git diff --stat`
      confirms); what changes is that `internal/payments` never passes `""` into
      it again.

## Phase 8: Slice 2 verification — PR 2 boundary
- [x] 8.1 `go build ./...`, `go vet ./...`, `golangci-lint run` — `0 issues.`
- [x] 8.2 `go test -race -v -count=1 ./internal/payments/... ./internal/data/...
      ./internal/mp/...` green; full `make test` green (all 29 packages `ok`).
- [x] 8.3 Confirmed every scenario in the `seller-credential-integrity` delta
      spec has a passing test: UNREADABLE
      (`TestAnUnreadableSellerTokenRefusesBeforeMercadoPagoIsCalled`), MISSING
      (`TestAMissingSellerTokenRefusesBeforeMercadoPagoIsCalled`), UNAVAILABLE
      (`TestAnUnfetchableComplexIsTreatedAsATransientOutage` +
      `TestTransientProviderFailure`), and the unchanged preference-expiry
      scenario (Phase 6, confirmed by inspection — no test needed since no code
      changed).

---

# SLICE 3 — Refund Intent Marker and Sweep (PR 3, base=PR2, ~335 lines)

Must land after slice 2 is merged, not before: the sweep drives
`AutoRefundIfPaid`, and only post-slice-2 does that refuse per-arm instead of
falling back to the platform token — landing this slice first would run the
pre-slice-2 fallback at 2-minute cron cadence instead of once per cancellation.

## Phase 9: `db/migrations/010_refund_intent_marker.sql`
- [x] 9.1 Write the migration: `ALTER TABLE bookings ADD COLUMN
      refund_intent_at TIMESTAMPTZ` (nullable, no backfill — nothing is
      deployed); `CREATE INDEX idx_bookings_refund_intent ON bookings
      (refund_intent_at) WHERE refund_intent_at IS NOT NULL` (every row is NULL
      except an unclaimed orphan, so the sweep must not scan the table);
      `ALTER TABLE bookings ADD CONSTRAINT
      bookings_refund_intent_only_when_cancelled CHECK (refund_intent_at IS NULL
      OR status = 'cancelled')`. Down: drop constraint, index, column, in that
      order.
- [x] 9.2 In the migration's own comment block (not only this task), record: the
      CHECK is confirmed unreachable by every path traced in design — migration
      006's `bookings_forbid_status_reversal` trigger only fires on
      `status`/`payment_status` changes (its `WHEN` clause,
      `006_tenant_and_money_invariants.sql:511-512`), so it cannot strand a
      marker on a resurrected row — but it is **flagged as the constraint to
      revisit** once real traffic exists, the same "floor, not proof" caveat
      migrations 006 and 009 carry for their own CHECKs.
- [x] 9.3 `make migrate-up` then `make migrate-down` against a local DB
      round-trips cleanly. Verified against the E2E database (`make
      e2e-db-up`, port 5433): up to version 10, down to 9, up to 10 again — all
      three clean.

## Phase 10: `db/queries/bookings.sql` — `UpdateBooking` carries the marker
- [x] 10.1 Add `refund_intent_at = $7` to `UpdateBooking`'s `SET` clause
      (`:38-47`) and `refund_intent_at` to its parameter list, after the existing
      `$1-$6` (`status`, `payment_status`, `notes`, `deposit_amount`, `id`,
      `version`). One added column on the existing optimistic-concurrency
      UPDATE — zero extra round trips, exactly as design requires.

## Phase 11: sqlc regeneration — its own commit
- [x] 11.1 From a clean tree, ran `make sqlc`. The generated diff under
      `internal/db/` (3 files, +58/-19) is isolated as its own commit-shaped
      step, not yet committed per the orchestrator's "DO NOT COMMIT" instruction
      — the diff is: `internal/db/models.go` (`Booking.RefundIntentAt`, plus
      new `JobLock` and `WebhookEvent` struct definitions), `internal/db/
      bookings.sql.go` (`refund_intent_at` threaded through every `Booking`
      query and `UpdateBookingParams`), `internal/db/payments.sql.go`
      (`UpdatePaymentParams.RefundAmount` `pgtype.Int4` → `int32`). Broader than
      the design's shorthand "WebhookEvent struct diff" — see Deviations below;
      the design's own reasoning (`internal/db` never regenerated since 006)
      predicts exactly this, migration 006's comment just named the smallest
      instance of it. No hand-edits to `internal/db/` anywhere in this change.
- [x] 11.2 Confirmed `db.Booking` and `db.UpdateBookingParams` now carry
      `RefundIntentAt pgtype.Timestamptz`. `go build ./...` did **not** fail at
      the four call sites Phase 12 targets — see Deviations below for why (Go
      keyed struct literals silently zero an omitted field rather than fail to
      compile) — it failed instead at six sites all caused by the collateral
      `RefundAmount` type change (`payments.go:138,247,287`, `refunds.go:
      127,216,230`), fixed as a mechanical, non-design-bearing correction
      (direct `int`/`int32` conversions replacing now-mismatched
      `int4ToPg`/`pgToInt` calls, same `//nolint:gosec` idiom already used for
      sibling currency fields) so the tree compiles for Phase 12's own build
      checkpoint. Verified the four `UpdateBookingParams` call sites Phase 12
      must edit are exactly and only `bookings.go:332`, `refunds.go:276`,
      `payments.go:199`, `payments.go:257` (`rg 'UpdateBookingParams'` outside
      `internal/db/`), since the compiler cannot be relied on to catch a missed
      one here.

## Phase 12: Carry the column through the data layer
- [x] 12.1 `internal/data/convert.go`: added `timePtrToPg(t *time.Time)
      pgtype.Timestamptz` / `pgToTimePtr(t pgtype.Timestamptz) *time.Time`,
      matching the existing `textToPg`/`pgToTextPtr` pointer-pair shape.
- [x] 12.2 `internal/data/bookings.go`: added `RefundIntentAt *time.Time
      \`json:"-"\`` to the `Booking` struct, after `Version`; `bookingFromDB`
      populates it via `pgToTimePtr(b.RefundIntentAt)`; `BookingModel.Update`
      passes `RefundIntentAt: timePtrToPg(b.RefundIntentAt)` into
      `UpdateBookingParams`.
- [x] 12.3 `internal/data/payments.go`: both `UpdateBooking` calls, inside
      `InsertAndConfirmBooking` and `ConfirmWebhookPayment`, pass
      `RefundIntentAt: timePtrToPg(b.RefundIntentAt)` — neither path sets
      the marker; both carry through whatever the in-memory `Booking` already
      holds (nil, for every booking reaching these two confirm paths).
- [x] 12.4 `internal/data/refunds.go` `cancelRefundedBooking`: passes
      `RefundIntentAt: locked.RefundIntentAt` unchanged — `locked` is read fresh
      under `FOR UPDATE`, so this writes back whatever `ClaimRefund` already
      cleared (12.5), never a stale caller-supplied value.
- [x] 12.5 `internal/data/refunds.go` `ClaimRefund`: added `clearRefundIntent`,
      a private helper sibling to `recordRefundAttempt`, that runs `UPDATE
      bookings SET refund_intent_at = NULL WHERE id = $1` via `tx.Exec` inside
      the claim's own transaction, called with `claim.BookingID` after the claim
      struct is built and before `tx.Commit`. This is the "clears it inside the
      claim transaction" design requires — it does **not** go through the new
      `ClearRefundIntent` store method (Phase 13), which would be a separate
      round trip outside this transaction and reopen the crash window the
      marker exists to close.

## Phase 13: `internal/data/refund_intents.go` — the sweep's store layer
- [x] 13.1 New file `internal/data/refund_intents.go` (precedent: `refunds.go`
      holds `PaymentModel`'s refund lifecycle apart from `payments.go`). Three
      hand-written statements on `BookingModel`, no sqlc:
      - `GetRefundIntentOrphans(ctx, olderThan time.Duration, limit int)
        ([]*Booking, error)` — `SELECT ... FROM bookings WHERE refund_intent_at
        IS NOT NULL AND refund_intent_at < NOW() - $1::interval ORDER BY
        refund_intent_at ASC LIMIT $2`. The predicate names `refund_intent_at`
        only — no `status`, `payment_status`, or join against `payments`.
      - `ClaimRefundIntent(ctx, id uuid.UUID, seen time.Time) error` — `UPDATE
        bookings SET refund_intent_at = NOW() WHERE id = $1 AND
        refund_intent_at = $2`; `RowsAffected() != 1` returns `ErrRecordNotFound`
        — `MarkProcessing`'s exact conditional-UPDATE idiom
        (`failed_refunds.go`'s `MarkProcessing`).
      - `ClearRefundIntent(ctx, id uuid.UUID) error` — `UPDATE bookings SET
        refund_intent_at = NULL WHERE id = $1`.
- [x] 13.2 `internal/data/models.go`: new `BookingRefundIntentManager` interface
      (the three methods above), composed into `BookingStore` alongside its
      existing five sub-interfaces. Not added to `BookingUpdater`, which stays
      its single `Update` method — ISP, matching this file's existing
      convention.

## Phase 14: Cancel handlers — `owesRefund`, the marker write, and the in-memory hazard
- [x] 14.1 `internal/bookings/actions.go` `Cancel`: before `h.store.Update`,
      compute `owesRefund := booking.PaymentStatus != "unpaid"` and, when true,
      set `booking.RefundIntentAt = &now` (`now := time.Now()` captured once).
      Rewrote the existing `if booking.PaymentStatus == "unpaid" { ... } else
      { ... }` block to branch on `!owesRefund` / `owesRefund` — the
      marker-setting condition and the refund-dispatch condition read one
      expression, never two.
- [x] 14.2 `internal/bookings/public.go` `PublicCancel`: computed `owesRefund :=
      booking.PaymentStatus != "unpaid" && withinRefundWindow` before
      `h.store.Update`, same marker-setting rule. Rewrote the `switch` so its
      `withinRefundWindow` case reads `owesRefund` and its `default`
      (`RefundNotEligible`) case is exactly `!owesRefund` in that context (the
      `unpaid` case already excludes the unpaid arm ahead of it) — an edit that
      marks a declined cancellation is, by construction, the same edit that
      refunds it (spec `refund-intent-durability`'s load-bearing property).
- [x] 14.3 **This is its own hazard, tasked explicitly.** `ClaimRefund` clears
      `refund_intent_at` in the database (Phase 12.5) but cannot reach the
      handler's in-memory `*Booking`. In both `actions.go` (right beside
      `booking.PaymentStatus = paymentStatusAfter(booking, outcome)`,
      immediately before the JSON response) and `public.go` (immediately before
      the JSON response block, where `paymentStatusAfter` is read inline), set
      `booking.RefundIntentAt = nil` once `outcome` is known — same place, same
      idiom, unconditional (the field was either never set, cleared by the
      claim, or cleared by Phase 15's `AutoRefundIfPaid` exits; nil is correct
      in every case).

## Phase 15: `AutoRefundIfPaid` — clear the marker on every exit that commits no claim
- [x] 15.1 In `internal/payments/refund.go` `AutoRefundIfPaid`, added a single
      `committed bool` + `defer`-guarded `h.refundIntents.ClearRefundIntent(...)`
      (on a context detached from the caller's own; logs, never fails the
      outcome) covering every return reached **without** a successfully
      committed `ClaimRefund`: `stop != nil`, `ErrAlreadyRefunded`,
      `ErrRefundInFlight`, and the `default:`/`owedManually(...)` branch — the
      defer fires only after `owedManually` has already alerted, matching
      design's ordering. `committed = true` is set immediately once
      `ClaimRefund` returns a nil error, so no clear runs after that point (the
      seller-credential refusal, the provider-rejection path, and the success
      path) — Phase 12.5 already cleared the marker inside that same committed
      transaction.
- [x] 15.2 Added `RefundIntentStore` (the same three methods as
      `BookingRefundIntentManager`) to `internal/payments/payments.go` as its own
      interface — **not** merged into the existing `BookingStore` interface
      there, which stays its current two methods (`GetByID`, `Update`). Added
      `refundIntents RefundIntentStore` to `Handler` and `RefundIntents
      RefundIntentStore` to `Dependencies`; wired in `NewHandler`.

## Phase 16: The sweep and its cron registration
- [x] 16.1 In `internal/payments/refund.go` (beside `RetryFailedRefunds`, the
      sibling queue job), declared `refundIntentGrace = 5 * time.Minute` (must
      outlast one honest in-request refund — the MercadoPago client's own
      timeout is 8s) and `refundIntentBatch = 12`, mirroring
      `failed_refunds.go`'s `staleRefundProcessing` / `refundSweepBatch`.
- [x] 16.2 Added `func (h *Handler) SweepOrphanedRefundIntents(ctx
      context.Context)`: fetches `h.refundIntents.GetRefundIntentOrphans(ctx,
      refundIntentGrace, refundIntentBatch)`; for each returned booking, calls
      `h.refundIntents.ClaimRefundIntent(ctx, b.ID, *b.RefundIntentAt)` — on
      `ErrRecordNotFound`, another instance took it, logs at Info and continues;
      on any other error, logs at Error and continues; otherwise calls
      `h.AutoRefundIfPaid(ctx, b)` and logs its `RefundOutcome` (alerting again
      at Error if it `NeedsAHuman()`). The sweep clears nothing itself —
      `AutoRefundIfPaid` (Phase 15) and `ClaimRefund` (Phase 12.5) own every
      clearing path.
- [x] 16.3 `cmd/api/cron.go`: registered `job("sweep_refund_intents",
      2*time.Minute, app.payments.SweepOrphanedRefundIntents)` in `cronJobs()`,
      alongside `retry_refunds` and `sweep_webhook_events` — same 2-minute
      cadence as those two.
- [x] 16.4 `cmd/api/app.go`: added `RefundIntents: d.models.Bookings` to the
      `payments.Dependencies{...}` literal — `d.models.Bookings` is typed
      `data.BookingStore`, which composes `BookingRefundIntentManager`
      (Phase 13.2) and therefore already structurally satisfies
      `payments.RefundIntentStore`; no new store instance is constructed.

## Phase 17: The false-positive proof — the single most important deliverable
A sweep that gets this wrong pays out money the venue is entitled to keep. Both
tests below are required, and each names the exact mutation that must break it.
- [x] 17.1 [Unit, mutation-verified] In `internal/bookings`
      (`cancel_test.go`): `TestAnOutOfWindowPublicCancelNeverSetsTheRefundIntentMarker`
      — a `PublicCancel` on a paid booking (`payment_status='deposit_paid'`),
      outside the complex's refund window — asserts the stub store's `Update`
      receives a booking with `RefundIntentAt == nil`. **False negative found
      and fixed before the mutation ran**: `stubStore.Update` recorded the
      caller's live `*data.Booking` pointer, and `PublicCancel` mutates that
      same pointer's `RefundIntentAt` to `nil` later in the same request
      (Phase 14.3's in-memory-hazard fix) — so the assertion would have read
      `nil` regardless of what was actually written at `Update`-call time,
      exactly the class of false negative flagged in the brief. Fixed by
      making `stubStore.Update` store a shallow copy at call time
      (`internal/bookings/stubs_test.go`); full `./internal/bookings/...`
      suite re-run green after the fix, unaffected. **Mutation, run and
      recorded**: forced `owesRefund := true` in `public.go` (unconditional) —
      re-ran, FAILED (`got 2026-08-23 01:29:01... `, a non-nil
      `RefundIntentAt`). Restored the guard — re-ran, PASSED.
- [x] 17.2 [Integration, mutation-verified, `//go:build integration`] New file
      `internal/bookings/refund_intent_integration_test.go`,
      `TestAnOutOfWindowPublicCancelIsNeverFoundByTheSweep`: cancels a paid
      booking out-of-window through the real `PublicCancel` handler (real
      `data.BookingModel`/`data.ComplexModel`, stubs for the rest) against the
      E2E database, confirms the row reads `status='cancelled'`,
      `payment_status='deposit_paid'`, no `failed_refunds` row, then runs
      `GetRefundIntentOrphans` against it: **zero rows** — PASSED. **Mutation,
      run and recorded**: added `OR (status='cancelled' AND payment_status IN
      ('deposit_paid','fully_paid'))` to the orphan query's `WHERE` clause —
      re-ran, FAILED (`a deliberately declined cancellation must never be
      found by the reconciliation sweep`), proving the test catches exactly
      the row-shape-inference bug the proposal identified as unsound. Removed
      the mutation — re-ran, PASSED.

## Phase 18: Remaining mutation-verified and unit tests
- [x] 18.1 [Unit, mutation-verified] `internal/bookings` (`cancel_test.go`):
      `TestStaffCancelAlwaysSetsTheRefundIntentMarkerWhenPaid` — staff `Cancel`
      out of any notional window: the marker **is** set (staff refunds ignore
      the window by product decision, `actions.go`'s existing comment on the
      asymmetry). **Mutation, run and recorded**: changed `owesRefund :=
      booking.PaymentStatus != "unpaid" && false` in `actions.go` (window-style
      gating) — re-ran, FAILED (`a paid staff cancellation must set the
      refund-intent marker...`). Restored — re-ran, PASSED. Also re-ran the
      full `./internal/bookings/...` suite green after restoring both 17.1's
      and 18.1's mutations.
- [x] 18.2 [Unit] New file `internal/payments/refund_intents_test.go`:
      `TestSweepNeverCallsMercadoPagoForACashBooking` — sweep processing a
      cash/no-MP-id booking is answered `RefundManual` via `refundable`'s
      existing no-MP-id branch, `RefundPayment` is never called, and the
      marker is cleared exactly once (`f.refundIntents.cleared` has length 1,
      via Phase 15's clearing on the `owedManually` exit — no separate
      sweep-side clear call exists to double it). Companion test
      `TestSweepLeavesAnAlreadyClaimedOrphanAlone` confirms a lost claim race
      (`ErrRecordNotFound`) never reaches `ClaimRefund` or clears a marker this
      instance does not own.
- [x] 18.3 [Unit] Reused the existing
      `TestConcurrentCancellationRunsTheCollectorCheckOnce`
      (`internal/payments/payments_test.go`, landed in slice 1): asserts
      `refundCancelledBookingPayment`'s collector check
      (`f.complexes.calls`) runs exactly once across the concurrent-cancellation
      re-entry — confirms slice 3 did not reintroduce the recursion, since it
      touched no code this test exercises.

## Phase 19: Remaining integration tests
- [x] 19.1 [Integration, `//go:build integration`, mutation-verified] Split
      across two purpose-built tests, since the payment-level `ClaimRefund` CAS
      already masks a broken booking-level CAS from any test that only counts
      `failed_refunds` rows:
      - `internal/data/refund_intents_integration_test.go`,
        `TestTwoConcurrentSweepersOnlyOneClaimsAnOrphan` — two real goroutines
        call `ClaimRefundIntent` directly on the same booking with the same
        `seen` value (mirroring `refunds_integration_test.go`'s
        `TestTwoConcurrentClaimsOnlyOneWins`); exactly one wins, one gets
        `ErrRecordNotFound`. **Mutation, run and recorded**: dropped `AND
        refund_intent_at = $2` from the `UPDATE` (kept only `WHERE id = $1`) —
        first attempt failed to even compile/execute cleanly (`mismatched param
        and argument count` from the now-unused `$2` bind), which the brief
        names explicitly as proving nothing; fixed by also removing the unused
        arg so the mutated query actually runs unconditionally — re-ran, FAILED
        (`2 won`, both claims succeeded). Restored — re-ran, PASSED.
      - `internal/payments/refund_intents_integration_test.go`,
        `TestTwoConcurrentSweepsClaimTheOrphanExactlyOnce` — the business-level
        companion: two real `SweepOrphanedRefundIntents` invocations race on
        the same orphan end to end; exactly one `failed_refunds` row exists
        afterward — proves what depends on the CAS the other test proves
        directly, matching proposal success criterion "proven with two
        concurrent sweepers." No separate mutation run for this one — same
        underlying CAS already mutation-verified above.
      - Also added `TestGetRefundIntentOrphansFindsAnAgedMarkerAndNothingElse`
        (an aged marker is found; a fresh one inside its grace period and an
        unmarked cancelled booking are not) — not separately tasked, added
        alongside 19.1's file as direct coverage of Phase 13.1's predicate
        against a real query planner.
- [x] 19.2 [Integration, `//go:build integration`]
      `internal/data/refund_intents_integration_test.go`:
      `TestClaimRefundLeavesTheRefundIntentMarkerCleared` — after a successful
      `ClaimRefund`, `refund_intent_at` reads NULL on the booking row (Phase
      12.5); `TestARefundIntentMarkerIsRefusedOnAConfirmedBooking` — attempting
      to set `refund_intent_at` on a `confirmed` row directly via SQL is
      refused by the `bookings_refund_intent_only_when_cancelled` CHECK
      (Phase 9.1).

## Phase 20: Slice 3 verification — PR 3 boundary
- [x] 20.1 `go build ./...`, `go vet ./...`, `go vet -tags integration ./...`
      clean. `golangci-lint run` required one fix: `AutoRefundIfPaid` grew past
      the `funlen` budget (70 lines) from Phase 15's clearing logic; extracted
      the deferred-clear closure into `clearRefundIntentUnlessClaimed` and
      collapsed two multi-line logger calls — `0 issues.` (see Deviations in
      the apply report). Also required adding
      `GetRefundIntentOrphans`/`ClaimRefundIntent`/`ClearRefundIntent` no-op
      methods to `cmd/api/mock_stores_test.go`'s `mockBookingStore`, which must
      satisfy the widened `data.BookingStore` — `go vet` caught this
      immediately (`missing method ClaimRefundIntent`).
- [x] 20.2 `go test -race -v -count=1 ./...` (unit suite) green — all 29
      tested packages `ok`.
- [x] 20.3 `make e2e-db-up && make test/integration && make test/security` —
      both green (exit 0 across all packages), including Phases 17.2, 19.1,
      and 19.2's new tests, individually re-verified `ok` for `internal/data`,
      `internal/payments`, `internal/bookings`.
- [x] 20.4 `rg 'refund_intent_at'` across `internal/db/` shows only
      sqlc-generated references from Phase 11 (`models.go`, `bookings.sql.go`)
      — no hand-edit crept into that directory (`git status --short
      internal/db/` shows exactly those files as modified, nothing else).

---

## Phase 21: Whole-change verification against `proposal.md`'s Success Criteria
- [x] 21.1 Confirmed standing: `TestAnUnreadableSellerTokenRefusesBeforeMercadoPagoIsCalled`
      / `TestAMissingSellerTokenRefusesBeforeMercadoPagoIsCalled`
      (`internal/payments/refund_test.go`, `outcomes_test.go`) — re-run green
      as part of Phase 20.2's full unit suite; unaffected by this slice's
      changes (Phase 15 touches only the marker-clearing exits, not
      `refuseForCredential`/`sellerCredential`).
- [x] 21.2 Confirmed standing: `TestAnUnfetchableComplexIsTreatedAsATransientOutage`
      (`internal/payments/outcomes_test.go`) — re-run green as part of Phase
      20.2. Unaffected by this slice for the same reason as 21.1.
- [x] 21.3 `TestTwoConcurrentSweepersOnlyOneClaimsAnOrphan` (Phase 19.1,
      `internal/data`) and its business-level companion
      `TestTwoConcurrentSweepsClaimTheOrphanExactlyOnce` (`internal/payments`)
      — both PASS, both re-run as part of Phase 20.3.
- [x] 21.4 `TestAnOutOfWindowPublicCancelIsNeverFoundByTheSweep` (Phase 17.2)
      and `TestSweepNeverCallsMercadoPagoForACashBooking` (Phase 18.2) — both
      PASS, both re-run as part of Phase 20.2/20.3.
- [x] 21.5 `TestApprovedPaymentForACancelledBookingWithAForeignCollectorWritesNothing`
      (Phase 2.1, `internal/payments/payments_test.go`) and its companion
      regression tests (Phase 2.3) — re-run green as part of Phase 20.2; slice
      3 touched no code on this path.
- [x] 21.6 `go build ./...`, `go vet ./...`, `go vet -tags integration ./...`,
      `golangci-lint run` (`0 issues.`), `go test -race -v -count=1 ./...`, and
      `make test/integration` + `make test/security` all passed against the
      current working tree, which carries all three slices together (nothing
      has been committed or split into separate PRs yet — see the apply
      report's "DO NOT COMMIT" instruction). This **is** the merged-tree
      re-run this task asks for; it will be worth repeating after the three
      PRs actually land as separate commits, per the task's own parenthetical.
