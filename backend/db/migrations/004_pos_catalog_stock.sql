-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- POS CATALOG AND STOCK — products a complex sells at the counter, and the
-- append-only ledger that moves their stock.
--
-- Delivery 3 of pos-cashbox's feature document (owner decisions 2026-09-22,
-- Engram `vibe/product/pos-cashbox-mvp`). This is T4a: the catalog
-- (products), the stock ledger (stock_movements), restock and adjustment.
-- Selling a product (T4b's sales/sale_items, and the stock_movements rows of
-- kind 'sale'/'sale_void' they drive) is a later migration; the CHECK
-- constraints below already accept all four kinds so that migration needs no
-- schema change of its own, only new INSERTs.
--
-- 001_init.sql is immutable (docs/adr/0003, the 2026-09-22 entry): this is
-- the third migration on top of it, after 002_counter_payment_methods.sql
-- and 003_cashbox.sql.
--
-- products and stock_movements are tenant-scoped tables, so they get the same
-- row-level-security treatment as cash_sessions/cash_movements in
-- 003_cashbox.sql, which itself follows 001_init.sql's ACCESS section — see
-- that section's own comment for why. New tables inherit vibe_app's grants
-- from the ALTER DEFAULT PRIVILEGES 001_init.sql already registered; RLS is
-- not covered by a default and has to be turned on per table, below.

-- ==================== products ====================
--
-- A product is deactivated, never deleted, once it exists: stock_movements
-- rows reference it and (from T4b) sale_items snapshot its name and price at
-- sale time, so removing the row would either cascade away real history or
-- leave it dangling. active is the only "this product is gone" a client ever
-- sees.
--
-- uuidv7() rather than gen_random_uuid(): matches every other money-adjacent
-- table in this schema (bookings, payments, cash_sessions, cash_movements) —
-- see the note above the bookings table in 001_init.sql for why.
--
-- price is INTEGER, like payments.amount and cash_movements.amount: it is one
-- product's own unit price, never summed across rows the way
-- cash_sessions' BIGINT columns are (see that table's own comment in
-- 003_cashbox.sql for the distinction this schema draws between a per-entry
-- money column and a session-level aggregate). Request validation caps it
-- well under int32 range (internal/products/handlers.go's maxMoney), the
-- same bound cash_movements.amount already carries.
--
-- stock_on_hand is INTEGER, signed, and MAY be negative: selling at zero
-- stock is allowed in this MVP (owner decision, pos-cashbox's Why section),
-- and a product oversold before anyone notices should read as "-3, go
-- count it" rather than clamp to zero and hide the fact.
--
-- version follows the same optimistic-concurrency convention as
-- complexes/courts (001_init.sql's trigger_bump_version, reused here rather
-- than redefined): products are mutable through PATCH the same way courts are
-- through PUT, and a client that read a product before editing it should get
-- the same 409-on-stale-version protection.
CREATE TABLE products (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id          UUID NOT NULL,
    name                TEXT NOT NULL,
    -- Free text, owner-defined (e.g. "Bebidas") — there is no fixed catalog of
    -- categories the way cash_movements.category has one, because a product
    -- category is a merchandising label, not a system classification.
    category            TEXT,
    price               INTEGER NOT NULL,
    tracks_stock        BOOLEAN NOT NULL DEFAULT true,
    stock_on_hand       INTEGER NOT NULL DEFAULT 0,
    low_stock_threshold INTEGER,
    active              BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- The optimistic-concurrency counter; see trigger_bump_version in
    -- 001_init.sql.
    version             INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT products_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- Pointed at by stock_movements' composite FK below, the same shape as
    -- courts_id_complex_id_key / cash_sessions_id_complex_id_key.
    CONSTRAINT products_id_complex_id_key UNIQUE (id, complex_id),
    -- "Trimmed" is a schema invariant, not only a handler-side courtesy: the
    -- name = btrim(name) half stops a row inserted outside the handler (a
    -- seed, a data migration) from carrying whitespace the case-folded
    -- unique index below cannot see either — "Coca " and "Coca" would
    -- otherwise coexist as two active products with what reads as the same
    -- name.
    CONSTRAINT products_name_length CHECK (name = btrim(name) AND char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT products_category_length CHECK (category IS NULL OR char_length(category) <= 60),
    CONSTRAINT products_price_check CHECK (price >= 0),
    CONSTRAINT products_low_stock_threshold_check
        CHECK (low_stock_threshold IS NULL OR low_stock_threshold >= 0)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_version BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION trigger_bump_version();

CREATE INDEX idx_products_complex ON products (complex_id);

-- A product's name is unique per complex among ACTIVE products only — the
-- same "reuse a deleted name is ordinary" shape courts_active_name_unique
-- gives court names — but case-folded (lower(name)) rather than exact-match:
-- the feature document asks for it explicitly, unlike courts, because a
-- product name is typed at a counter under time pressure ("Coca" vs "coca")
-- and two catalog rows differing only by case would be a real merchandising
-- mistake, not a deliberate choice the way two courts named with different
-- case might be.
CREATE UNIQUE INDEX idx_products_active_name_unique ON products (complex_id, lower(name))
    WHERE active;

-- ==================== stock_movements ====================
--
-- Append-only ledger: there is no UPDATE or DELETE query for this table, the
-- same convention cash_movements and audit_log follow (by omission of a
-- write path — see 003_cashbox.sql's own comment on cash_movements). Every
-- change to stock_on_hand happens in the same transaction that inserts the
-- row here (internal/products/store.Store.Restock / .Adjust), under a lock on
-- the product row, so the two can never drift apart.
CREATE TABLE stock_movements (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id       UUID NOT NULL,
    product_id       UUID NOT NULL,
    kind             TEXT NOT NULL,
    quantity         INTEGER NOT NULL,
    -- Required (and constrained to this list) iff kind = 'adjustment' —
    -- stock_movements_reason_consistent below.
    reason           TEXT,
    note             TEXT,
    -- Required iff kind = 'restock' — stock_movements_cash_movement_consistent
    -- below. Composite FK so a restock's cash entry and its stock entry can
    -- never point at two different tenants' rows.
    cash_movement_id UUID,
    -- No FK yet: the sales table this will reference does not exist until
    -- T4b (pos-cashbox's next migration). A 'sale' or 'sale_void' row cannot
    -- be inserted through this codebase yet — the CHECK constraints below
    -- accept the kind, but internal/products has no write path for it — so
    -- there is nothing to enforce referential integrity against today.
    sale_id          UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by       UUID NOT NULL,
    CONSTRAINT stock_movements_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- CROSS-TENANT COMPOSITE KEY, same shape as
    -- cash_movements_session_in_same_complex: a movement naming one tenant's
    -- product and another tenant's complex_id would move stock across
    -- tenants.
    CONSTRAINT stock_movements_product_in_same_complex
        FOREIGN KEY (product_id, complex_id) REFERENCES products (id, complex_id),
    CONSTRAINT stock_movements_cash_movement_in_same_complex
        FOREIGN KEY (cash_movement_id, complex_id) REFERENCES cash_movements (id, complex_id),
    CONSTRAINT stock_movements_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT stock_movements_kind_check CHECK (kind IN ('sale', 'restock', 'adjustment', 'sale_void')),
    CONSTRAINT stock_movements_quantity_nonzero CHECK (quantity <> 0),
    -- Sign of quantity is fixed by kind: a sale always removes stock, a
    -- restock or a voided sale always adds it back, and an adjustment may go
    -- either way (breakage removes, a count correction may add or remove).
    CONSTRAINT stock_movements_quantity_sign_check CHECK (
        (kind IN ('restock', 'sale_void') AND quantity > 0)
        OR (kind = 'sale' AND quantity < 0)
        OR (kind = 'adjustment' AND quantity <> 0)
    ),
    CONSTRAINT stock_movements_reason_check CHECK (
        reason IS NULL OR reason IN ('breakage', 'expired', 'own_consumption', 'count_correction', 'other')
    ),
    CONSTRAINT stock_movements_reason_consistent CHECK (
        (kind = 'adjustment' AND reason IS NOT NULL) OR (kind <> 'adjustment' AND reason IS NULL)
    ),
    CONSTRAINT stock_movements_cash_movement_consistent CHECK (
        (kind = 'restock' AND cash_movement_id IS NOT NULL) OR (kind <> 'restock' AND cash_movement_id IS NULL)
    )
);

-- Newest first per product — the shape every other paginated list in this
-- schema pages on (created_at, id) DESC.
CREATE INDEX idx_stock_movements_product ON stock_movements (product_id, created_at DESC, id DESC);
CREATE INDEX idx_stock_movements_complex ON stock_movements (complex_id);

-- ==================== cash_movements: system categories ====================
--
-- A sale and a restock are cash_movements rows the SYSTEM writes, never a
-- category an owner can pick by hand — the same "mercadopago is not a
-- counter method" shape internal/paymentmethod already enforces for method.
-- Both constraints below are dropped and re-added rather than left alone:
-- cash_movements_category_check must accept the two new values, and
-- cash_movements_category_kind_consistent must know 'sale' is an income
-- category and 'restock' an expense one. The constraint bodies are rewritten
-- as explicit category lists per kind (rather than 003_cashbox.sql's
-- "anything that is not other_income" shape for expense) so that adding a
-- category here can never silently also legalize it for the other kind.
ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_check;
ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_kind_consistent;

ALTER TABLE cash_movements ADD CONSTRAINT cash_movements_category_check CHECK (
    category IN ('other_income', 'sale', 'supplies', 'salaries', 'services', 'maintenance',
                 'cleaning', 'withdrawal', 'other_expense', 'restock')
);

ALTER TABLE cash_movements ADD CONSTRAINT cash_movements_category_kind_consistent CHECK (
    voids_movement_id IS NOT NULL
    OR (kind = 'income' AND category IN ('other_income', 'sale'))
    OR (kind = 'expense' AND category IN ('supplies', 'salaries', 'services', 'maintenance',
                                           'cleaning', 'withdrawal', 'other_expense', 'restock'))
);

-- Manual movement creation (internal/cashbox's CreateMovement handler) keeps
-- rejecting 'sale' and 'restock': its IncomeCategories/ExpenseCategories
-- allowlists (internal/cashbox/service.go) are a fixed Go list, not driven by
-- this CHECK, and neither list was touched by this migration — see
-- internal/cashbox/handlers_test.go's
-- TestCreateMovementRejectsSystemCategories for the regression test that
-- pins it.

-- ==================== ROW-LEVEL SECURITY ====================
--
-- Same shape as every policy pair in 001_init.sql's ACCESS section and
-- 003_cashbox.sql. See that section's comment for the full reasoning.
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE products FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON products
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON products
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE stock_movements ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_movements FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON stock_movements
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON stock_movements
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_kind_consistent;
ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_check;
ALTER TABLE cash_movements ADD CONSTRAINT cash_movements_category_check CHECK (
    category IN ('other_income', 'supplies', 'salaries', 'services', 'maintenance',
                 'cleaning', 'withdrawal', 'other_expense')
);
ALTER TABLE cash_movements ADD CONSTRAINT cash_movements_category_kind_consistent CHECK (
    voids_movement_id IS NOT NULL
    OR (kind = 'income' AND category = 'other_income')
    OR (kind = 'expense' AND category <> 'other_income')
);

DROP TABLE stock_movements;
DROP TABLE products;
