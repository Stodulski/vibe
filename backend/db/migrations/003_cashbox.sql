-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- CASHBOX — cash shifts and cash movements
--
-- A complex's counter runs a shift ("cash session"): the owner opens it with
-- a starting float, records income and expenses against it over the day, and
-- closes it by counting the till and reconciling against what the system
-- expects. This is delivery 2 of pos-cashbox's feature document (owner
-- decisions 2026-09-22, Engram `vibe/product/pos-cashbox-mvp`).
--
-- 001_init.sql is immutable (docs/adr/0003, the 2026-09-22 entry): this is
-- the second migration on top of it, after 002_counter_payment_methods.sql.
--
-- cash_sessions and cash_movements are both tenant-scoped tables, so they
-- get the same row-level-security treatment as every table in 001_init.sql's
-- ACCESS section (tenant_isolation + tenant_bypass, one column comparison
-- each) — see that section's own comment for why. New tables created here
-- inherit vibe_app's grants from the ALTER DEFAULT PRIVILEGES 001_init.sql
-- already registered for both vibe_migrator and whoever ran it, so no
-- separate GRANT statement is needed for that half; RLS is not covered by a
-- default and has to be turned on per table, which is what this file does.

-- ==================== cash_sessions ====================
--
-- One row per shift. opening_cash is the float the owner counted into the
-- drawer at open; the four close-state columns (closed_at, closed_by,
-- counted_cash, expected_cash) are set together, exactly once, by Close, and
-- never touched again — cash_sessions_forbid_update_after_close (below)
-- enforces the "closed sessions are immutable and never reopen" rule at the
-- database, not only in the store.
--
-- difference is GENERATED rather than written by the application: it can
-- never disagree with the two columns it is computed from, and it reads NULL
-- automatically while the session is still open (either operand is NULL).
--
-- uuidv7() rather than gen_random_uuid(): see the note above the bookings
-- table in 001_init.sql for why the money tables use time-ordered ids.
-- opening_cash/counted_cash/expected_cash/difference are BIGINT, not INTEGER
-- like every other money column in this schema (payments.amount,
-- cash_movements.amount below): they are SESSION-level aggregates, not one
-- payment or one movement. expected_cash in particular is opening_cash plus
-- a SUM of this session's own cash movements plus a SUM of booking payments
-- collected in the session's window (Store.Close) — three Go-computed
-- accumulators added together — and an INTEGER column silently wraps once
-- their total exceeds 2,147,483,647 centavos (~21.4 million ARS): a busy
-- complex over a long open shift is not a hypothetical here, and a wrapped
-- expected_cash is worse than merely wrong, because cash_sessions_
-- forbid_update_after_close (below) makes a closed session's snapshot
-- permanent — there is no second write that could ever correct it. Movement
-- amount itself (cash_movements.amount, below) stays INTEGER on purpose: it
-- is one entry a person typed at the counter, the same bound as
-- payments.amount, and request validation caps it well under int32 range
-- (internal/cashbox/handlers.go) — this migration only widens the columns
-- that SUM many such entries together.
CREATE TABLE cash_sessions (
    id             UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id     UUID NOT NULL,
    opened_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    opened_by      UUID NOT NULL,
    opening_cash   BIGINT NOT NULL,
    closed_at      TIMESTAMPTZ,
    closed_by      UUID,
    counted_cash   BIGINT,
    expected_cash  BIGINT,
    difference     BIGINT GENERATED ALWAYS AS (counted_cash - expected_cash) STORED,
    -- Two columns, not one: the closer must never be able to erase what the
    -- opener recorded (owner correction, pos-cashbox T2 review). opening_note
    -- is written once, by Open; closing_note is written once, by Close;
    -- neither is ever merged with or overwritten by the other write.
    opening_note   TEXT,
    closing_note   TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT cash_sessions_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- Referenced by cash_movements_session_in_same_complex below, the same
    -- shape as bookings_id_complex_id_key / payments_booking_in_same_complex.
    CONSTRAINT cash_sessions_id_complex_id_key UNIQUE (id, complex_id),
    -- No ON DELETE clause: the same convention bookings_created_by_fkey uses.
    -- An owner's account is deleted only after the complex it owns, which
    -- CASCADEs cash_sessions away first, so this FK never actually blocks
    -- that delete — see 001_init.sql's note above bookings.created_by.
    CONSTRAINT cash_sessions_opened_by_fkey FOREIGN KEY (opened_by) REFERENCES users (id),
    CONSTRAINT cash_sessions_closed_by_fkey FOREIGN KEY (closed_by) REFERENCES users (id),
    CONSTRAINT cash_sessions_opening_cash_check CHECK (opening_cash >= 0),
    CONSTRAINT cash_sessions_counted_cash_check CHECK (counted_cash IS NULL OR counted_cash >= 0),
    -- The four close-state columns are all null (open) or all set (closed) —
    -- there is no partially-closed session.
    CONSTRAINT cash_sessions_close_state_consistent CHECK (
        (closed_at IS NULL AND closed_by IS NULL AND counted_cash IS NULL AND expected_cash IS NULL)
        OR
        (closed_at IS NOT NULL AND closed_by IS NOT NULL AND counted_cash IS NOT NULL AND expected_cash IS NOT NULL)
    )
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON cash_sessions
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- At most one open session per complex — the partial predicate is exactly
-- "this session is still open", so a second POST while one is already open
-- collides with this index (23505) rather than the application having to
-- take its word for it.
CREATE UNIQUE INDEX idx_cash_sessions_one_open ON cash_sessions (complex_id) WHERE closed_at IS NULL;

-- History, newest first — the shape every other paginated list in this
-- schema pages on (created_at/opened_at, id) DESC.
CREATE INDEX idx_cash_sessions_complex ON cash_sessions (complex_id, opened_at DESC, id DESC);

-- +goose StatementBegin
CREATE FUNCTION cash_sessions_forbid_update_after_close() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    IF OLD.closed_at IS NOT NULL THEN
        RAISE EXCEPTION
            'cash session %: a closed session is immutable and cannot be reopened or edited', OLD.id
            USING ERRCODE = '23514',
                  CONSTRAINT = 'cash_sessions_closed_is_immutable';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER cash_sessions_forbid_update_after_close
    BEFORE UPDATE ON cash_sessions
    FOR EACH ROW
    EXECUTE FUNCTION cash_sessions_forbid_update_after_close();

-- ==================== cash_movements ====================
--
-- Append-only: there is no UPDATE or DELETE query for this table anywhere in
-- the Go code (mirroring audit_log, which is append-only the same way —
-- by omission of a write path, not by a DB-level REVOKE or trigger; see
-- 001_init.sql's ACCESS section, whose blanket
-- "GRANT ... UPDATE, DELETE ON ALL TABLES" already covers every table
-- including audit_log, and which this migration does not narrow). A
-- correction is a new row: a void.
CREATE TABLE cash_movements (
    id                UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id        UUID NOT NULL,
    session_id        UUID NOT NULL,
    kind              TEXT NOT NULL,
    category          TEXT NOT NULL,
    -- The shared payment_method enum (001_init.sql, widened by
    -- 002_counter_payment_methods.sql), restricted to counter methods: a
    -- cash-drawer movement can never be the online checkout's own method,
    -- for the same reason isCounterPaymentMethod excludes it in Go
    -- (internal/paymentmethod) — 'mercadopago' carries mp_payment_id and its
    -- own automatic refund, neither of which a manual movement has.
    method            payment_method NOT NULL,
    amount            INTEGER NOT NULL,
    note              TEXT,
    -- The movement this one corrects, opposite kind, same amount/method/
    -- category — enforced below by cash_movements_check_void, not by a bare
    -- CHECK, because "same as the row voids_movement_id points at" needs to
    -- read that other row. UNIQUE is the other half of "an original can be
    -- voided once".
    voids_movement_id UUID UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by        UUID NOT NULL,
    CONSTRAINT cash_movements_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- CROSS-TENANT COMPOSITE KEY, same shape as payments_booking_in_same_complex:
    -- a movement naming one tenant's session and another tenant's complex_id
    -- would land one tenant's cash count in another tenant's till.
    CONSTRAINT cash_movements_session_in_same_complex
        FOREIGN KEY (session_id, complex_id) REFERENCES cash_sessions (id, complex_id),
    -- Referenced by cash_movements_void_in_same_complex below.
    CONSTRAINT cash_movements_id_complex_id_key UNIQUE (id, complex_id),
    -- The void and the row it voids must be the same tenant's — a
    -- self-referencing composite FK needs the UNIQUE above to reference.
    CONSTRAINT cash_movements_void_in_same_complex
        FOREIGN KEY (voids_movement_id, complex_id) REFERENCES cash_movements (id, complex_id),
    CONSTRAINT cash_movements_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT cash_movements_kind_check CHECK (kind IN ('income', 'expense')),
    CONSTRAINT cash_movements_category_check CHECK (
        category IN ('other_income', 'supplies', 'salaries', 'services', 'maintenance',
                      'cleaning', 'withdrawal', 'other_expense')
    ),
    -- Category must match kind — EXCEPT on a void row, which deliberately
    -- carries the ORIGINAL's category under the OPPOSITE kind (an expense's
    -- void is kind=income, category=<the expense category it corrects>), so
    -- the ordinary kind/category pairing does not apply to it. See the
    -- feature document's Decisions: "a void reuses the original's category".
    CONSTRAINT cash_movements_category_kind_consistent CHECK (
        voids_movement_id IS NOT NULL
        OR (kind = 'income' AND category = 'other_income')
        OR (kind = 'expense' AND category <> 'other_income')
    ),
    CONSTRAINT cash_movements_method_not_online CHECK (method <> 'mercadopago'),
    CONSTRAINT cash_movements_amount_check CHECK (amount > 0)
);

CREATE INDEX idx_cash_movements_session ON cash_movements (session_id, created_at DESC, id DESC);
CREATE INDEX idx_cash_movements_complex ON cash_movements (complex_id);

-- +goose StatementBegin
CREATE FUNCTION cash_movements_check_void() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
DECLARE
    orig cash_movements%ROWTYPE;
BEGIN
    IF NEW.voids_movement_id IS NULL THEN
        RETURN NEW;
    END IF;

    -- Locked, not merely read: two concurrent voids of the same original
    -- would otherwise both pass this check before either commits, and the
    -- UNIQUE constraint on voids_movement_id would only catch it as a
    -- generic 23505 rather than this trigger's clearer message. FOR UPDATE
    -- on a row that already exists and is never updated (append-only) costs
    -- nothing beyond the lock itself.
    SELECT * INTO orig FROM cash_movements WHERE id = NEW.voids_movement_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'cash_movements: voids_movement_id % does not exist', NEW.voids_movement_id
            USING ERRCODE = '23503';
    END IF;

    IF orig.voids_movement_id IS NOT NULL THEN
        RAISE EXCEPTION 'cash_movements: cannot void movement % — it is itself a void', NEW.voids_movement_id
            USING ERRCODE = '23514',
                  CONSTRAINT = 'cash_movements_no_void_of_void';
    END IF;

    IF orig.kind = NEW.kind THEN
        RAISE EXCEPTION 'cash_movements: a void must carry the opposite kind of the original'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'cash_movements_void_opposite_kind';
    END IF;

    IF orig.amount <> NEW.amount OR orig.method <> NEW.method OR orig.category <> NEW.category THEN
        RAISE EXCEPTION 'cash_movements: a void must carry the same amount, method and category as the original'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'cash_movements_void_matches_original';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER cash_movements_check_void
    BEFORE INSERT ON cash_movements
    FOR EACH ROW
    EXECUTE FUNCTION cash_movements_check_void();

-- ==================== ROW-LEVEL SECURITY ====================
--
-- Same shape as every policy pair in 001_init.sql's ACCESS section: one
-- column comparison against app.complex_id, ORed with the bypass. See that
-- section's comment for the full reasoning (fail-closed on an unscoped
-- session, ENABLE+FORCE so the schema owner is bound too).
ALTER TABLE cash_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cash_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON cash_sessions
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON cash_sessions
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE cash_movements ENABLE ROW LEVEL SECURITY;
ALTER TABLE cash_movements FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON cash_movements
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON cash_movements
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE cash_movements;
DROP FUNCTION cash_movements_check_void();
DROP TABLE cash_sessions;
DROP FUNCTION cash_sessions_forbid_update_after_close();
