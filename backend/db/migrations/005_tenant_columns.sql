-- +goose Up

SET LOCAL lock_timeout = '3s';

-- Row-level security is FORCEd on every table touched below and applies to the
-- schema owner too, so the backfills further down would silently touch zero
-- rows without this line. See the header of 001_init.sql.
SET LOCAL app.bypass_tenant = 'on';

-- ==================== THE TENANT COLUMN ON THE FOUR CHILD TABLES ====================
--
-- Fifteen of the nineteen tables carry complex_id. Four do not: court_prices,
-- blocked_slots and slot_locks hang off a court, booking_link_tokens off a
-- booking, and each reaches its tenant through that parent. 001_init.sql calls
-- them the "two-hop tables" and decided against a denormalised copy, because
-- a stored derivable value needs a trigger and a composite foreign key to stay
-- honest, while an EXISTS subquery in the policy is a primary-key lookup the
-- planner runs as a semi-join.
--
-- That reasoning was about the POLICY, and it was right about the policy. What
-- it left is a different problem, and it is the one this migration is for: the
-- tenant of those four tables exists nowhere a query can name. Every statement
-- against them either joins to the parent or trusts the policy alone, so the
-- explicit half of the isolation — the WHERE clause a human can read, which is
-- what still holds when a policy is relaxed or a path takes the bypass it did
-- not need — is unwritable. On the other fifteen tables it is one predicate.
--
-- So the column is added, and the two things 001 named as its price are paid
-- in full rather than skipped:
--
--   * A COMPOSITE FOREIGN KEY, (court_id, complex_id) -> courts (id,
--     complex_id), which is what makes the copy impossible to contradict. A
--     plain FK on court_id alone would let a row name court A and tenant B; the
--     composite one is refused by the database. The UNIQUE (id, complex_id)
--     those references need is already on courts, clients and bookings — 001
--     put it there for bookings_court_in_same_complex, which is this same
--     technique applied one table up.
--
--   * A TRIGGER that fills the column from the parent, so no INSERT has to
--     supply it and none can supply it wrongly. It overwrites whatever the
--     caller passed rather than filling only NULLs: a derived column that can
--     be overridden is a derived column that will be, and the whole value of
--     this one is that it cannot disagree with the parent.
--
-- The ON DELETE behaviour is unchanged: the composite constraint carries the
-- same ON DELETE CASCADE the single-column one did, and the old constraint is
-- dropped so a deletion does not have to satisfy two.
--
-- No production data (pre-launch), so the backfill is one UPDATE per table and
-- the column goes straight to NOT NULL rather than through an expand-and-
-- contract dance.

-- +goose StatementBegin
-- Derives the tenant of a row that hangs off a court. The parent is read under
-- the same transaction as the write, and RLS on courts does not apply here:
-- the function is SECURITY DEFINER-free but runs inside the caller's session,
-- and every caller that may insert one of these rows can already see its own
-- court. A row whose court is invisible fails the composite FK instead, which
-- is the refusal we want.
CREATE OR REPLACE FUNCTION set_complex_id_from_court() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    SELECT c.complex_id INTO NEW.complex_id
    FROM courts c
    WHERE c.id = NEW.court_id;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
-- The same, for the one table that hangs off a booking.
CREATE OR REPLACE FUNCTION set_complex_id_from_booking() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    SELECT b.complex_id INTO NEW.complex_id
    FROM bookings b
    WHERE b.id = NEW.booking_id;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- -------------------- court_prices --------------------

ALTER TABLE court_prices ADD COLUMN complex_id UUID;

UPDATE court_prices cp
SET complex_id = c.complex_id
FROM courts c
WHERE c.id = cp.court_id;

ALTER TABLE court_prices ALTER COLUMN complex_id SET NOT NULL;

ALTER TABLE court_prices DROP CONSTRAINT court_prices_court_id_fkey;
ALTER TABLE court_prices ADD CONSTRAINT court_prices_court_id_fkey
    FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id) ON DELETE CASCADE;

CREATE INDEX idx_court_prices_complex ON court_prices (complex_id);

CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON court_prices
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_court();

-- -------------------- blocked_slots --------------------

ALTER TABLE blocked_slots ADD COLUMN complex_id UUID;

UPDATE blocked_slots bs
SET complex_id = c.complex_id
FROM courts c
WHERE c.id = bs.court_id;

ALTER TABLE blocked_slots ALTER COLUMN complex_id SET NOT NULL;

ALTER TABLE blocked_slots DROP CONSTRAINT blocked_slots_court_id_fkey;
ALTER TABLE blocked_slots ADD CONSTRAINT blocked_slots_court_id_fkey
    FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id) ON DELETE CASCADE;

CREATE INDEX idx_blocked_slots_complex ON blocked_slots (complex_id);

CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON blocked_slots
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_court();

-- -------------------- slot_locks --------------------

ALTER TABLE slot_locks ADD COLUMN complex_id UUID;

UPDATE slot_locks sl
SET complex_id = c.complex_id
FROM courts c
WHERE c.id = sl.court_id;

ALTER TABLE slot_locks ALTER COLUMN complex_id SET NOT NULL;

ALTER TABLE slot_locks DROP CONSTRAINT slot_locks_court_id_fkey;
ALTER TABLE slot_locks ADD CONSTRAINT slot_locks_court_id_fkey
    FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id) ON DELETE CASCADE;

CREATE INDEX idx_slot_locks_complex ON slot_locks (complex_id);

CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON slot_locks
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_court();

-- -------------------- booking_link_tokens --------------------

ALTER TABLE booking_link_tokens ADD COLUMN complex_id UUID;

UPDATE booking_link_tokens t
SET complex_id = b.complex_id
FROM bookings b
WHERE b.id = t.booking_id;

ALTER TABLE booking_link_tokens ALTER COLUMN complex_id SET NOT NULL;

ALTER TABLE booking_link_tokens DROP CONSTRAINT booking_link_tokens_booking_id_fkey;
ALTER TABLE booking_link_tokens ADD CONSTRAINT booking_link_tokens_booking_id_fkey
    FOREIGN KEY (booking_id, complex_id) REFERENCES bookings (id, complex_id) ON DELETE CASCADE;

CREATE INDEX idx_booking_link_tokens_complex ON booking_link_tokens (complex_id);

CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON booking_link_tokens
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_booking();

-- ==================== THE POLICIES FOLLOW THE COLUMN ====================
--
-- Same shape as the fifteen direct policies in 001_init.sql, which is the point:
-- the four exceptions stop being exceptions, and every tenant policy in the
-- schema now reads the same way. The predicate and its three subtleties —
-- missing_ok, nullif on the empty string, and NULL matching nothing so an
-- unscoped session fails closed — are written out at length in 001_init.sql and
-- are unchanged here.
--
-- The EXISTS subqueries these replace were correct. They were also the only
-- policies in the schema that had to be read twice to be believed, and they
-- cost a semi-join per row where the others cost a column comparison.
--
-- tenant_bypass is untouched on all four: it does not read the column.

DROP POLICY tenant_isolation ON court_prices;
CREATE POLICY tenant_isolation ON court_prices
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);

DROP POLICY tenant_isolation ON blocked_slots;
CREATE POLICY tenant_isolation ON blocked_slots
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);

DROP POLICY tenant_isolation ON slot_locks;
CREATE POLICY tenant_isolation ON slot_locks
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);

DROP POLICY tenant_isolation ON booking_link_tokens;
CREATE POLICY tenant_isolation ON booking_link_tokens
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);

-- +goose Down

SET LOCAL lock_timeout = '3s';
SET LOCAL app.bypass_tenant = 'on';

DROP POLICY tenant_isolation ON booking_link_tokens;
CREATE POLICY tenant_isolation ON booking_link_tokens
    USING (EXISTS (SELECT 1 FROM bookings b
                    WHERE b.id = booking_link_tokens.booking_id
                      AND b.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid))
    WITH CHECK (EXISTS (SELECT 1 FROM bookings b
                         WHERE b.id = booking_link_tokens.booking_id
                           AND b.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid));

DROP POLICY tenant_isolation ON slot_locks;
CREATE POLICY tenant_isolation ON slot_locks
    USING (EXISTS (SELECT 1 FROM courts c
                    WHERE c.id = slot_locks.court_id
                      AND c.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid))
    WITH CHECK (EXISTS (SELECT 1 FROM courts c
                         WHERE c.id = slot_locks.court_id
                           AND c.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid));

DROP POLICY tenant_isolation ON blocked_slots;
CREATE POLICY tenant_isolation ON blocked_slots
    USING (EXISTS (SELECT 1 FROM courts c
                    WHERE c.id = blocked_slots.court_id
                      AND c.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid))
    WITH CHECK (EXISTS (SELECT 1 FROM courts c
                         WHERE c.id = blocked_slots.court_id
                           AND c.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid));

DROP POLICY tenant_isolation ON court_prices;
CREATE POLICY tenant_isolation ON court_prices
    USING (EXISTS (SELECT 1 FROM courts c
                    WHERE c.id = court_prices.court_id
                      AND c.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid))
    WITH CHECK (EXISTS (SELECT 1 FROM courts c
                         WHERE c.id = court_prices.court_id
                           AND c.complex_id = nullif(current_setting('app.complex_id', true), '')::uuid));

DROP TRIGGER set_complex_id ON booking_link_tokens;
ALTER TABLE booking_link_tokens DROP CONSTRAINT booking_link_tokens_booking_id_fkey;
ALTER TABLE booking_link_tokens ADD CONSTRAINT booking_link_tokens_booking_id_fkey
    FOREIGN KEY (booking_id) REFERENCES bookings (id) ON DELETE CASCADE;
ALTER TABLE booking_link_tokens DROP COLUMN complex_id;

DROP TRIGGER set_complex_id ON slot_locks;
ALTER TABLE slot_locks DROP CONSTRAINT slot_locks_court_id_fkey;
ALTER TABLE slot_locks ADD CONSTRAINT slot_locks_court_id_fkey
    FOREIGN KEY (court_id) REFERENCES courts (id) ON DELETE CASCADE;
ALTER TABLE slot_locks DROP COLUMN complex_id;

DROP TRIGGER set_complex_id ON blocked_slots;
ALTER TABLE blocked_slots DROP CONSTRAINT blocked_slots_court_id_fkey;
ALTER TABLE blocked_slots ADD CONSTRAINT blocked_slots_court_id_fkey
    FOREIGN KEY (court_id) REFERENCES courts (id) ON DELETE CASCADE;
ALTER TABLE blocked_slots DROP COLUMN complex_id;

DROP TRIGGER set_complex_id ON court_prices;
ALTER TABLE court_prices DROP CONSTRAINT court_prices_court_id_fkey;
ALTER TABLE court_prices ADD CONSTRAINT court_prices_court_id_fkey
    FOREIGN KEY (court_id) REFERENCES courts (id) ON DELETE CASCADE;
ALTER TABLE court_prices DROP COLUMN complex_id;

DROP FUNCTION set_complex_id_from_booking();
DROP FUNCTION set_complex_id_from_court();
