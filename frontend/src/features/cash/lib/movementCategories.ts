/**
 * A cash movement's category, one fixed set per `kind` (owner decision,
 * `odd/tasks/pos-cashbox.md`: "income other_income; expense supplies,
 * salaries, services, maintenance, cleaning, withdrawal, other_expense").
 * Mirrors the `CHECK` constraint on `cash_movements.category`
 * (`db/migrations/003_cashbox.sql`) and the `openapi.yaml` enum.
 */
export const INCOME_CATEGORIES = ['other_income'] as const;
export const EXPENSE_CATEGORIES = [
  'supplies',
  'salaries',
  'services',
  'maintenance',
  'cleaning',
  'withdrawal',
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
