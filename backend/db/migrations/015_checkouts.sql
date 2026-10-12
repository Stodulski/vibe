-- +goose Up

SET LOCAL lock_timeout = '3s';
-- The backfill below reads and writes tenant-scoped rows as the schema owner,
-- which FORCE ROW LEVEL SECURITY would otherwise hide (see 003_cashbox.sql).
SET LOCAL app.bypass_tenant = 'on';

-- ============================================================================
-- CHECKOUTS — one row per counter checkout that collected booking balances.
--
-- A checkout that sold products has its sale (sale_id). A checkout that only
-- collected balances has no sale and no income: until now it was visible only
-- through its payments. This row is the sales list entry that stands for it,
-- with the checkout's id as the entry's id. sale_id is set when the checkout
-- also sold products, so each checkout names at most one sale and each sale at
-- most one checkout.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section, with the
-- same tenant_isolation + tenant_bypass pair as sale_booking_payments.
CREATE TABLE checkouts (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id  UUID NOT NULL,
    session_id  UUID NOT NULL,
    method      payment_method NOT NULL,
    sale_id     UUID,
    created_by  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT checkouts_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT checkouts_session_in_same_complex
        FOREIGN KEY (session_id, complex_id) REFERENCES cash_sessions (id, complex_id),
    CONSTRAINT checkouts_sale_in_same_complex
        FOREIGN KEY (sale_id, complex_id) REFERENCES sales (id, complex_id),
    CONSTRAINT checkouts_sale_id_key UNIQUE (sale_id),
    CONSTRAINT checkouts_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT checkouts_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT checkouts_method_not_online CHECK (method <> 'mercadopago')
);

CREATE INDEX idx_checkouts_complex ON checkouts (complex_id, created_at DESC, id DESC);
CREATE INDEX idx_checkouts_session ON checkouts (session_id, created_at DESC, id DESC);

ALTER TABLE checkouts ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkouts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON checkouts
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON checkouts
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- Checkouts that already collected balances get their row here. There are none
-- in production yet; this keeps the migration whole for any database that has
-- them. A sale's id doubles as its checkout's id, so the link below needs no
-- lookup.
INSERT INTO checkouts (id, complex_id, session_id, method, sale_id, created_by, created_at)
SELECT s.id, s.complex_id, s.session_id, s.method, s.id, s.created_by, s.created_at
FROM sales s
WHERE EXISTS (SELECT 1 FROM sale_booking_payments sbp WHERE sbp.sale_id = s.id);

-- Each balance now names its checkout. Its sale link moves to checkouts.sale_id,
-- the one place a checkout's sale is recorded, so the column here goes.
ALTER TABLE sale_booking_payments ADD COLUMN checkout_id UUID;
UPDATE sale_booking_payments SET checkout_id = sale_id;
ALTER TABLE sale_booking_payments
    ALTER COLUMN checkout_id SET NOT NULL,
    ADD CONSTRAINT sale_booking_payments_checkout_fkey
        FOREIGN KEY (checkout_id, complex_id) REFERENCES checkouts (id, complex_id);
CREATE INDEX idx_sale_booking_payments_checkout ON sale_booking_payments (checkout_id);
ALTER TABLE sale_booking_payments DROP COLUMN sale_id;

-- +goose Down

SET LOCAL lock_timeout = '3s';
SET LOCAL app.bypass_tenant = 'on';

-- Refuses while a balance-only checkout exists: the 014 shape cannot name one
-- (sale_id is NOT NULL there), so the rollback stops rather than dropping money.
ALTER TABLE sale_booking_payments ADD COLUMN sale_id UUID;
UPDATE sale_booking_payments sbp SET sale_id = ch.sale_id
FROM checkouts ch
WHERE ch.id = sbp.checkout_id;
ALTER TABLE sale_booking_payments
    ALTER COLUMN sale_id SET NOT NULL,
    ADD CONSTRAINT sale_booking_payments_sale_fkey
        FOREIGN KEY (sale_id, complex_id) REFERENCES sales (id, complex_id);
CREATE INDEX idx_sale_booking_payments_sale ON sale_booking_payments (sale_id);
DROP INDEX idx_sale_booking_payments_checkout;
ALTER TABLE sale_booking_payments DROP COLUMN checkout_id;

DROP TABLE checkouts;
