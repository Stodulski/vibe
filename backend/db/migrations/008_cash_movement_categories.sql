-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- CASH MOVEMENT CATEGORIES — fixed income and expense categories
--
-- The owner could not categorize common manual cash movements beyond
-- 'other_income' (income) and the 7 expense buckets 003_cashbox.sql added.
-- This adds 6 fixed income categories and 5 fixed expense categories, shared
-- by every complex (owner decision 2026-09-24: a fixed set, not per-complex
-- custom categories — Engram `vibe/product/cash-movement-categories`).
--
-- 'sale' and 'restock' stay system-only (004_pos_catalog_stock.sql); this
-- migration only widens the categories a manual movement can carry.
--
-- category is a plain TEXT column with a CHECK, not a PostgreSQL enum (see
-- 001_init.sql: only user_role/booking_status/payment_status/payment_method/
-- court_type/sport_type/day_of_week are ENUM types) — so unlike
-- 002_counter_payment_methods.sql's ALTER TYPE ... ADD VALUE, this migration
-- follows 004_pos_catalog_stock.sql's own pattern: drop and re-add both
-- constraints with the widened explicit list. That also means, unlike 002,
-- this migration's Down can genuinely revert the constraint — there is no
-- ALTER TYPE ... DROP VALUE limitation here.
ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_check;
ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_kind_consistent;

ALTER TABLE cash_movements ADD CONSTRAINT cash_movements_category_check CHECK (
    category IN ('other_income', 'sale', 'classes', 'tournaments', 'events', 'memberships',
                 'sponsorship', 'cash_contribution', 'supplies', 'salaries', 'services',
                 'maintenance', 'cleaning', 'withdrawal', 'other_expense', 'restock',
                 'rent', 'taxes', 'professional_fees', 'marketing', 'bank_fees')
);

ALTER TABLE cash_movements ADD CONSTRAINT cash_movements_category_kind_consistent CHECK (
    voids_movement_id IS NOT NULL
    OR (kind = 'income' AND category IN ('other_income', 'sale', 'classes', 'tournaments',
                                          'events', 'memberships', 'sponsorship', 'cash_contribution'))
    OR (kind = 'expense' AND category IN ('supplies', 'salaries', 'services', 'maintenance',
                                           'cleaning', 'withdrawal', 'other_expense', 'restock',
                                           'rent', 'taxes', 'professional_fees', 'marketing', 'bank_fees'))
);

-- Manual movement creation (internal/cashbox's CreateMovement handler) keeps
-- rejecting 'sale' and 'restock': its IncomeCategories/ExpenseCategories
-- allowlists (internal/cashbox/service.go) are widened to add the 11 new
-- categories here, but not to the two system ones — unchanged from
-- 004_pos_catalog_stock.sql's own note.

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Unlike 002_counter_payment_methods.sql's Down (which must refuse: an ENUM
-- value cannot be dropped), this constraint can genuinely revert, the same
-- way 004_pos_catalog_stock.sql's own Down reverts 003_cashbox.sql's
-- constraint — but only while no row carries one of the 11 new categories.
-- A row that already does makes this ALTER TABLE fail with a CHECK
-- violation, which is the correct, truthful outcome: there is no silent way
-- to roll back a category enum that is already in use.
ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_kind_consistent;
ALTER TABLE cash_movements DROP CONSTRAINT cash_movements_category_check;

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
