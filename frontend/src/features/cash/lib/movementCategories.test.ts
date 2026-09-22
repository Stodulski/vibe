import { describe, it, expect } from 'vitest';
import { categoriesFor, INCOME_CATEGORIES, EXPENSE_CATEGORIES, MOVEMENT_CATEGORIES } from './movementCategories';

describe('categoriesFor', () => {
  it('returns only other_income for income', () => {
    expect(categoriesFor('income')).toEqual(['other_income']);
  });

  it('returns the seven expense categories for expense', () => {
    expect(categoriesFor('expense')).toEqual([
      'supplies',
      'salaries',
      'services',
      'maintenance',
      'cleaning',
      'withdrawal',
      'other_expense',
    ]);
  });
});

describe('MOVEMENT_CATEGORIES', () => {
  it('is the union of income and expense categories, income first', () => {
    expect(MOVEMENT_CATEGORIES).toEqual([...INCOME_CATEGORIES, ...EXPENSE_CATEGORIES]);
    expect(MOVEMENT_CATEGORIES).toHaveLength(8);
  });
});
