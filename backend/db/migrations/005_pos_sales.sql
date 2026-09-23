-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- POS SALES — selling products from the catalog, with stock control.
--
-- Delivery 3 of pos-cashbox's feature document (owner decisions 2026-09-22,
-- Engram `vibe/product/pos-cashbox-mvp`). This is T4b, the last piece the
-- 004_pos_catalog_stock.sql header already named: "the CHECK constraints
-- below already accept all four kinds so that migration needs no schema
-- change of its own, only new INSERTs" — this file is that migration.
--
-- 001_init.sql is immutable (docs/adr/0003, the 2026-09-22 entry): this is
-- the fourth migration on top of it, after 002_counter_payment_methods.sql,
-- 003_cashbox.sql and 004_pos_catalog_stock.sql.
--
-- sales and sale_items are tenant-scoped tables, so they get the same
-- row-level-security treatment as every other table these three migrations
-- introduced — see 003_cashbox.sql's own comment for why.

-- ==================== sales ====================
--
-- One row per sale. Never deleted; the only mutation ever made to an
-- existing row is the one-time void (voided_at/voided_by/void_cash_movement_id,
-- written together exactly once by Store.Void). That is enforced twice, the
-- same belt-and-suspenders shape 003_cashbox.sql gives cash_sessions:
--
--   1. Store.Void loads the row under FOR UPDATE, checks voided_at IS NULL
--      in Go, and its own UPDATE still carries `WHERE voided_at IS NULL` as a
--      second, cheap guard against the same race that guard already closed.
--   2. sales_forbid_update_after_void (below), mirroring
--      cash_sessions_forbid_update_after_close: even a write that reaches
--      this table by some path other than Store.Void — a data repair, a
--      future bug — cannot silently re-void or edit an already-voided sale,
--      or edit an unvoided one into any shape other than "now voided".
--
-- A trigger was chosen over relying on the guarded UPDATE alone because it
-- is exactly as cheap as cash_sessions' own copy (one more BEFORE UPDATE
-- function, reused nowhere else, no extra table scan) and this table has the
-- identical "exactly one lifecycle mutation, ever" shape that pattern exists
-- for.
--
-- total is INTEGER, not BIGINT: it is one sale's own total, the same
-- per-entry bound cash_movements.amount already carries (and total becomes
-- exactly that column's value via cash_movement_id's linked row — see
-- sales_total_check below, which reuses the identical cap). It is not a
-- session-level aggregate the way cash_sessions' BIGINT columns are.
--
-- method excludes 'mercadopago' for the same reason cash_movements.method
-- does (sales_method_not_online, mirroring cash_movements_method_not_online):
-- a sale is always a counter transaction here, never the online checkout.
--
-- cash_movement_id is NOT NULL and UNIQUE: every sale writes exactly one
-- income movement (the "sale" category cash_movements row created by the
-- CREATE endpoint, in the same transaction as this row), and that movement
-- can never be shared by two sales.
CREATE TABLE sales (
    id                     UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id             UUID NOT NULL,
    session_id             UUID NOT NULL,
    method                 payment_method NOT NULL,
    total                  INTEGER NOT NULL,
    cash_movement_id       UUID NOT NULL UNIQUE,
    voided_at              TIMESTAMPTZ,
    voided_by              UUID,
    void_cash_movement_id  UUID UNIQUE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by             UUID NOT NULL,
    CONSTRAINT sales_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- CROSS-TENANT COMPOSITE KEYS, same shape as cash_movements_session_in_same_complex
    -- and stock_movements_product_in_same_complex: a sale naming one
    -- tenant's session or cash movement and another tenant's complex_id
    -- would land one tenant's sale in another tenant's shift or till.
    CONSTRAINT sales_session_in_same_complex
        FOREIGN KEY (session_id, complex_id) REFERENCES cash_sessions (id, complex_id),
    CONSTRAINT sales_cash_movement_in_same_complex
        FOREIGN KEY (cash_movement_id, complex_id) REFERENCES cash_movements (id, complex_id),
    CONSTRAINT sales_void_cash_movement_in_same_complex
        FOREIGN KEY (void_cash_movement_id, complex_id) REFERENCES cash_movements (id, complex_id),
    -- Pointed at by sale_items' and stock_movements' composite FKs below.
    CONSTRAINT sales_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT sales_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT sales_voided_by_fkey FOREIGN KEY (voided_by) REFERENCES users (id),
    CONSTRAINT sales_method_not_online CHECK (method <> 'mercadopago'),
    -- 2,000,000,000 is the same cap cash_movements.amount is validated
    -- against at the handler (internal/cashbox/handlers.go's
    -- maxMovementAmount) — a sale's total becomes exactly that column's
    -- value via the linked income movement, so the two must never disagree
    -- about what an INTEGER column here can hold.
    CONSTRAINT sales_total_check CHECK (total > 0 AND total <= 2000000000),
    -- All-or-nothing, the same shape cash_sessions_close_state_consistent
    -- gives that table's own close-state columns.
    CONSTRAINT sales_void_state_consistent CHECK (
        (voided_at IS NULL AND voided_by IS NULL AND void_cash_movement_id IS NULL)
        OR
        (voided_at IS NOT NULL AND voided_by IS NOT NULL AND void_cash_movement_id IS NOT NULL)
    )
);

-- +goose StatementBegin
CREATE FUNCTION sales_forbid_update_after_void() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    IF OLD.voided_at IS NOT NULL THEN
        RAISE EXCEPTION
            'sale %: a voided sale is immutable and cannot be edited or voided again', OLD.id
            USING ERRCODE = '23514',
                  CONSTRAINT = 'sales_voided_is_immutable';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER sales_forbid_update_after_void
    BEFORE UPDATE ON sales
    FOR EACH ROW
    EXECUTE FUNCTION sales_forbid_update_after_void();

-- History, newest first per complex — the shape every other paginated list in
-- this schema pages on (created_at, id) DESC — plus the optional session_id
-- filter the list endpoint takes.
CREATE INDEX idx_sales_complex ON sales (complex_id, created_at DESC, id DESC);
CREATE INDEX idx_sales_session ON sales (session_id, created_at DESC, id DESC);

-- ==================== sale_items ====================
--
-- One row per product sold within a sale. Never updated or deleted: a sale's
-- line items are exactly as immutable as the sale itself, and there is no
-- write path for either after INSERT. product_name and unit_price are
-- SNAPSHOTS taken at sale time — a later rename or price change on the
-- product must never change what a past sale reads as having sold.
CREATE TABLE sale_items (
    id            UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id    UUID NOT NULL,
    sale_id       UUID NOT NULL,
    product_id    UUID NOT NULL,
    product_name  TEXT NOT NULL,
    unit_price    INTEGER NOT NULL,
    quantity      INTEGER NOT NULL,
    line_total    INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sale_items_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT sale_items_sale_in_same_complex
        FOREIGN KEY (sale_id, complex_id) REFERENCES sales (id, complex_id),
    CONSTRAINT sale_items_product_in_same_complex
        FOREIGN KEY (product_id, complex_id) REFERENCES products (id, complex_id),
    -- A product appears once per sale — the application already refuses a
    -- duplicate product id with 422 before this is ever reached, and this is
    -- the same defense-in-depth relationship idx_products_active_name_unique
    -- has with productstore.ErrDuplicateProductName.
    CONSTRAINT sale_items_sale_id_product_id_key UNIQUE (sale_id, product_id),
    CONSTRAINT sale_items_unit_price_check CHECK (unit_price >= 0),
    CONSTRAINT sale_items_quantity_check CHECK (quantity BETWEEN 1 AND 1000),
    CONSTRAINT sale_items_line_total_check CHECK (line_total = unit_price * quantity)
);

CREATE INDEX idx_sale_items_sale ON sale_items (sale_id);
CREATE INDEX idx_sale_items_complex ON sale_items (complex_id);

-- ==================== stock_movements: the sale link ====================
--
-- 004_pos_catalog_stock.sql created stock_movements.sale_id with no FK,
-- because sales did not exist yet — its own comment says so. sales now
-- exists, so the composite FK closes that gap: a stock movement naming one
-- tenant's sale and another tenant's complex_id would move stock across
-- tenants the same way stock_movements_product_in_same_complex already
-- guards against for product_id.
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_sale_in_same_complex
    FOREIGN KEY (sale_id, complex_id) REFERENCES sales (id, complex_id);

-- A 'sale' or 'sale_void' row always names the sale it belongs to; every
-- other kind never does — the same shape
-- stock_movements_cash_movement_consistent already gives cash_movement_id
-- for 'restock'.
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_sale_id_consistent CHECK (
    (kind IN ('sale', 'sale_void') AND sale_id IS NOT NULL)
    OR (kind NOT IN ('sale', 'sale_void') AND sale_id IS NULL)
);

-- ==================== ROW-LEVEL SECURITY ====================
--
-- Same shape as every policy pair in 003_cashbox.sql / 004_pos_catalog_stock.sql.
ALTER TABLE sales ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sales
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON sales
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE sale_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE sale_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sale_items
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON sale_items
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_sale_id_consistent;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_sale_in_same_complex;
DROP TABLE sale_items;
DROP TABLE sales;
DROP FUNCTION sales_forbid_update_after_void();
