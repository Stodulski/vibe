/**
 * A cash movement's category, one fixed set per `kind` (owner decision,
 * `odd/tasks/cash-movement-categories.md`, 2026-09-24: a fixed set shared by
 * every complex). Mirrors the `CHECK` constraint on `cash_movements.category`
 * (`db/migrations/008_cash_movement_categories.sql`) and the `openapi.yaml`
 * enum. `other_income`/`other_expense` are listed last in both arrays so the
 * form's option order puts the catch-all category at the bottom.
 */
export const INCOME_CATEGORIES = [
  'classes',
  'tournaments',
  'events',
  'memberships',
  'sponsorship',
  'cash_contribution',
  'other_income',
] as const;
export const EXPENSE_CATEGORIES = [
  'supplies',
  'salaries',
  'services',
  'maintenance',
  'cleaning',
  'withdrawal',
  'rent',
  'taxes',
  'professional_fees',
  'marketing',
  'bank_fees',
  'other_expense',
] as const;

type IncomeCategory = (typeof INCOME_CATEGORIES)[number];
type ExpenseCategory = (typeof EXPENSE_CATEGORIES)[number];
export type MovementCategory = IncomeCategory | ExpenseCategory;

/** Every category a movement can carry, either kind — for parsing/lookups that don't branch on kind. */
export const MOVEMENT_CATEGORIES = [...INCOME_CATEGORIES, ...EXPENSE_CATEGORIES] as const;

export function categoriesFor(kind: 'income' | 'expense'): readonly MovementCategory[] {
  return kind === 'income' ? INCOME_CATEGORIES : EXPENSE_CATEGORIES;
}
