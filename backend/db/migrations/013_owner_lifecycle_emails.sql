-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- OWNER LIFECYCLE EMAILS — the nudges that move an account from "signed up" to
-- "taking bookings", plus the suppression list and the one-click unsubscribe
-- they carry.
--
-- None of these tables is tenant data (no complex_id on the rows that identify
-- a person), so none carries row-level security. New tables inherit vibe_app's
-- grants from the ALTER DEFAULT PRIVILEGES that 001_init.sql registered.
--
-- users.last_active_at is written by the authenticated owner requests, throttled
-- to one write an hour. It is the only signal the "inactive" nudge reads.
ALTER TABLE users ADD COLUMN last_active_at TIMESTAMPTZ;

-- Addresses that asked not to receive lifecycle email. Keyed by the lowercased
-- address, never expires, and is written only by the unsubscribe endpoint.
-- Transactional mail (bookings, verification, password reset) does not consult it.
CREATE TABLE email_suppressions (
    email      TEXT PRIMARY KEY,
    reason     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_suppressions_email_normalized
        CHECK (email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254)
);

-- People who started the register form (or the Google sign-up) and left before
-- finishing. Written by the public capture endpoint, which still forwards to the
-- spreadsheet as well. Nothing here is a user until a users row with the same
-- address exists, and the daily cleanup removes rows older than 30 days.
CREATE TABLE registration_leads (
    email      TEXT PRIMARY KEY,
    first_name TEXT NOT NULL DEFAULT '',
    last_name  TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Set once, by the claim that enqueues the lead's nudge.
    nudged_at  TIMESTAMPTZ,
    CONSTRAINT registration_leads_email_normalized
        CHECK (email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254)
);
CREATE INDEX idx_registration_leads_created_at ON registration_leads (created_at);

-- One row per owner per kind: the record that a nudge was sent, and what makes
-- each nudge once-only. Claimed before the email is enqueued.
CREATE TABLE owner_nudges (
    user_id UUID NOT NULL,
    kind    TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, kind),
    CONSTRAINT owner_nudges_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT owner_nudges_kind_known
        CHECK (kind IN ('verify_email', 'create_complex', 'add_courts', 'inactive'))
);

-- Launch baseline: every account that exists when this runs is marked as already
-- nudged for every kind, so the first cron pass after deploy sends nothing to
-- people who were already here. Only accounts created afterwards are eligible.
INSERT INTO owner_nudges (user_id, kind, sent_at)
SELECT u.id, k.kind, NOW()
FROM users u
CROSS JOIN (VALUES ('verify_email'), ('create_complex'), ('add_courts'), ('inactive')) AS k(kind)
ON CONFLICT (user_id, kind) DO NOTHING;

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE owner_nudges;
DROP TABLE registration_leads;
DROP TABLE email_suppressions;
ALTER TABLE users DROP COLUMN last_active_at;
