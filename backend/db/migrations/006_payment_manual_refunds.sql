-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- PAYMENT MANUAL REFUNDS
--
-- applyManualRefundRows (internal/payments/store/refunds.go) closes out a
-- partially refunded booking's cash/transfer rows once the owner confirms,
-- by hand, that they returned the money to the client. Until now it wrote
-- nothing of its own — only status='refunded' and refund_amount, the same
-- two columns a MercadoPago-driven automatic refund writes — so the only
-- other trace was payments.updated_at, which trigger_set_updated_at bumps on
-- ANY update to the row, including one that has nothing to do with a refund
-- (a status_detail change from a later webhook retry, say). The cashbox
-- could not tell a manual refund apart from an unrelated update, so
-- expected_cash never subtracted the cash an owner hands back by hand — a
-- documented gap in internal/cashbox/service.go.
--
-- This is the fifth migration on top of 001_init.sql written as a new
-- numbered file rather than folded into it directly (docs/adr/0003 records
-- why: Vibe now has a production deployment, so 001_init.sql is immutable
-- from here on). It follows 002_counter_payment_methods.sql and
-- 003_cashbox.sql; 004 and 005 (pos-catalog-stock, pos-sales) did not touch
-- payments.
--
-- Both columns are NULL for every existing row (no backfill: there is no way
-- to recover which past 'refunded' rows without a MercadoPago id were manual
-- versus automatic, or when a manual one actually happened) — cash-manual-
-- refunds' feature document decision: "rows refunded before 006 keep null
-- columns and are never counted".
ALTER TABLE payments
    ADD COLUMN manual_refund_amount INTEGER,
    ADD COLUMN manual_refunded_at TIMESTAMPTZ;

-- Both null (not a manual refund, or not refunded yet) or both set (the
-- owner confirmed handing the money back) — there is no half-written state,
-- the same shape cash_sessions_close_state_consistent (003_cashbox.sql)
-- gives the four close-state columns. amount > 0 because a zero-amount
-- manual refund would mean nothing was owed in the first place
-- (applyManualRefundRows already skips a row where owed <= 0); the upper
-- bound reuses payments_refund_within_amount_paid's own bound (amount +
-- service_fee) since a manual refund can never hand back more than the
-- payment ever collected.
--
-- Added NOT VALID: a plain ADD CONSTRAINT ... CHECK takes ACCESS EXCLUSIVE on
-- payments for the whole statement, including the full-table scan that
-- proves every existing row already satisfies it — on a table this hot
-- (every booking payment and refund writes it) that scan can hold the lock
-- long enough to back up the connection pool behind it. NOT VALID skips the
-- scan and takes ACCESS EXCLUSIVE only for the instant it takes to record
-- the constraint's existence.
--
-- The validating scan does NOT run here. Goose applies one migration file in
-- a single transaction, so a VALIDATE CONSTRAINT run in this same file would
-- still execute under the ACCESS EXCLUSIVE lock the ADD CONSTRAINT above
-- took — held until this transaction commits — which defeats NOT VALID
-- entirely. The scan runs in 007_validate_payment_manual_refunds.sql, its
-- own goose transaction, where it takes only SHARE UPDATE EXCLUSIVE and
-- does not block ordinary reads/writes. Until 007 runs, the constraint
-- exists but is not yet proven to hold for pre-existing rows; PostgreSQL
-- still enforces it against every new write in the meantime.
ALTER TABLE payments
    ADD CONSTRAINT payments_manual_refund_consistent CHECK (
        (manual_refund_amount IS NULL AND manual_refunded_at IS NULL)
        OR
        (manual_refund_amount IS NOT NULL AND manual_refunded_at IS NOT NULL
         AND manual_refund_amount > 0
         AND manual_refund_amount <= amount + service_fee)
    ) NOT VALID;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE payments
    DROP CONSTRAINT payments_manual_refund_consistent,
    DROP COLUMN manual_refund_amount,
    DROP COLUMN manual_refunded_at;
