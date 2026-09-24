import { describe, it, expect } from 'vitest';
import { categoriesFor, INCOME_CATEGORIES, EXPENSE_CATEGORIES, MOVEMENT_CATEGORIES } from './movementCategories';

describe('categoriesFor', () => {
  it('returns the seven income categories, other_income last', () => {
    expect(categoriesFor('income')).toEqual([
      'classes',
      'tournaments',
      'events',
      'memberships',
      'sponsorship',
      'cash_contribution',
      'other_income',
    ]);
  });

  it('returns the twelve expense categories, other_expense last', () => {
    expect(categoriesFor('expense')).toEqual([
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
    ]);
  });
});

describe('MOVEMENT_CATEGORIES', () => {
  it('is the union of income and expense categories, income first', () => {
    expect(MOVEMENT_CATEGORIES).toEqual([...INCOME_CATEGORIES, ...EXPENSE_CATEGORIES]);
    expect(MOVEMENT_CATEGORIES).toHaveLength(19);
  });
});
