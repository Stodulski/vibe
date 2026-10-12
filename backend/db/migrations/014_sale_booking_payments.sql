-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- SALE BOOKING PAYMENTS — the booking balances a counter checkout collected
-- alongside a sale's products (Caja > Vender, one checkout for both).
--
-- A booking balance is still a payments row, written by the same booking
-- payment logic the booking panel uses (internal/payments/store). This table
-- only links that row to the sale it was collected in, so the receipt can
-- name the booking and a void can refuse to undo money it cannot reverse.
-- The sale's own cash income carries the products only: the balance is
-- counted once, through its payments row, never again through the sale.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section, with the
-- same tenant_isolation + tenant_bypass pair (see 003_cashbox.sql). New tables
-- inherit vibe_app's grants from the ALTER DEFAULT PRIVILEGES that
-- 001_init.sql registered, so no GRANT statement is needed here.
CREATE TABLE sale_booking_payments (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id  UUID NOT NULL,
    sale_id     UUID NOT NULL,
    booking_id  UUID NOT NULL,
    -- One payments row belongs to at most one sale.
    payment_id  UUID NOT NULL,
    amount      INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sale_booking_payments_sale_fkey
        FOREIGN KEY (sale_id, complex_id) REFERENCES sales (id, complex_id),
    CONSTRAINT sale_booking_payments_booking_fkey
        FOREIGN KEY (booking_id, complex_id) REFERENCES bookings (id, complex_id),
    CONSTRAINT sale_booking_payments_payment_fkey
        FOREIGN KEY (payment_id) REFERENCES payments (id),
    CONSTRAINT sale_booking_payments_payment_key UNIQUE (payment_id),
    CONSTRAINT sale_booking_payments_amount_check CHECK (amount > 0 AND amount <= 2000000000)
);

CREATE INDEX idx_sale_booking_payments_sale ON sale_booking_payments (sale_id);

ALTER TABLE sale_booking_payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE sale_booking_payments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sale_booking_payments
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON sale_booking_payments
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE sale_booking_payments;
