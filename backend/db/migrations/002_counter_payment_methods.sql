-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- COUNTER PAYMENT METHODS
--
-- Counter (manual) payments used to accept only 'cash' and 'transfer'; a debit
-- card, a credit card and a QR/wallet payment taken in person got recorded as
-- one of those two, which made the monthly report by method wrong. This adds
-- the three missing values to payment_method (see 001_init.sql for the type).
-- 'mercadopago' is unaffected and stays reserved for the online checkout: it
-- carries mp_payment_id and drives an automatic MercadoPago API refund, which
-- a counter QR payment has no id for — see pos-cashbox's feature document
-- (owner decision, 2026-09-22) for the product rule this backs.
--
-- This is the first migration on top of 001_init.sql written as a new
-- numbered file rather than folded into it directly: docs/adr/0003 records why
-- (Vibe now has a production deployment, so 001_init.sql is immutable from
-- here on).
--
-- ADD VALUE inside a transaction is fine on PostgreSQL 12+ (the deployment is
-- on 18.6, see 001_init.sql's server capability guard) as long as the new
-- value is not read by the same transaction that adds it — which this
-- migration never does; it only adds the three values and stops.
ALTER TYPE payment_method ADD VALUE 'debit_card';
ALTER TYPE payment_method ADD VALUE 'credit_card';
ALTER TYPE payment_method ADD VALUE 'qr_wallet';

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- PostgreSQL has no ALTER TYPE ... DROP VALUE, and rebuilding payment_method
-- without these three values would fail the moment any payments row already
-- carries one of them — the general problem 001_init.sql's Down accepts by
-- dropping the whole schema instead, which is not an option here because this
-- migration is not the schema's first and running its Down must not take
-- every other object down with it. There is no honest partial rollback: a
-- statement that pretends to succeed while leaving the three values in place
-- would let goose report a downgrade that did not happen. Refusing loudly is
-- the truthful answer.
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION
        'payment_method cannot lose debit_card/credit_card/qr_wallet once added '
        '(PostgreSQL has no ALTER TYPE ... DROP VALUE); restore from a backup '
        'taken before this migration ran instead';
END;
$$;
-- +goose StatementEnd
