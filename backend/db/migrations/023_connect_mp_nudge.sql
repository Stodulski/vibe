-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- CONNECT MERCADOPAGO NUDGE — a lifecycle email for complexes with no MercadoPago
-- ============================================================================
-- connect_mp joins the owner nudge kinds. Unlike 013 there is no launch baseline:
-- the owners who exist today and have not connected MercadoPago are exactly the
-- ones this email is for, so they are due on the first pass after deploy.

ALTER TABLE owner_nudges DROP CONSTRAINT owner_nudges_kind_known;
ALTER TABLE owner_nudges ADD CONSTRAINT owner_nudges_kind_known
    CHECK (kind IN ('verify_email', 'create_complex', 'add_courts', 'connect_mp', 'inactive'));

-- +goose Down

SET LOCAL lock_timeout = '3s';

DELETE FROM owner_nudges WHERE kind = 'connect_mp';
ALTER TABLE owner_nudges DROP CONSTRAINT owner_nudges_kind_known;
ALTER TABLE owner_nudges ADD CONSTRAINT owner_nudges_kind_known
    CHECK (kind IN ('verify_email', 'create_complex', 'add_courts', 'inactive'));
