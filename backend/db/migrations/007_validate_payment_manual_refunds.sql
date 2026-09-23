-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- VALIDATE payments_manual_refund_consistent
--
-- 006_payment_manual_refunds.sql adds payments_manual_refund_consistent as
-- NOT VALID, which is instant: it does not scan existing rows. The scan
-- happens here, in its own migration, because goose runs each file in one
-- transaction and the ACCESS EXCLUSIVE lock an ALTER TABLE ADD CONSTRAINT
-- takes on payments lasts until that transaction commits; validating in the
-- same file would scan the whole table under that lock and block every
-- booking payment, webhook and refund meanwhile. In a transaction of its own,
-- VALIDATE CONSTRAINT takes only SHARE UPDATE EXCLUSIVE: it blocks other DDL
-- on payments, not ordinary reads and writes.
ALTER TABLE payments
    VALIDATE CONSTRAINT payments_manual_refund_consistent;

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Nothing to undo: VALIDATE CONSTRAINT only tells PostgreSQL a constraint
-- that already exists (added NOT VALID by 006) is now known to hold for
-- every row. There is no meaningful "un-validate" — the constraint still
-- exists and is still enforced going forward regardless of this Down, the
-- same way 002_counter_payment_methods.sql's Down cannot undo an added enum
-- value. Rolling back 006 (which drops the constraint entirely) is the only
-- way to actually remove it.
SELECT 1;
