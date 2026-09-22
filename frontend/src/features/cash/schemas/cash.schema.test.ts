import { describe, it, expect } from 'vitest';
import {
  openCashSessionSchema,
  closeCashSessionSchema,
  cashMovementSchema,
  voidCashMovementSchema,
} from './cash.schema';
import { MAX_SESSION_CASH_PESOS, MAX_MOVEMENT_AMOUNT_PESOS } from '../lib/money';

describe('openCashSessionSchema', () => {
  it('accepts an opening amount of exactly 0 (an empty drawer)', () => {
    expect(openCashSessionSchema.safeParse({ opening_cash: 0, note: '' }).success).toBe(true);
  });

  it('rejects a missing/blank amount (NaN from an empty number input)', () => {
    const result = openCashSessionSchema.safeParse({ opening_cash: NaN, note: '' });
    expect(result.success).toBe(false);
  });

  it('rejects a negative amount', () => {
    const result = openCashSessionSchema.safeParse({ opening_cash: -1, note: '' });
    expect(result.success).toBe(false);
  });

  it('rejects an amount over the session cash cap', () => {
    const result = openCashSessionSchema.safeParse({ opening_cash: MAX_SESSION_CASH_PESOS + 1, note: '' });
    expect(result.success).toBe(false);
  });

  it('accepts exactly the cap', () => {
    expect(openCashSessionSchema.safeParse({ opening_cash: MAX_SESSION_CASH_PESOS, note: '' }).success).toBe(true);
  });

  it('rejects a note over 500 chars', () => {
    const result = openCashSessionSchema.safeParse({ opening_cash: 0, note: 'a'.repeat(501) });
    expect(result.success).toBe(false);
  });

  it('rejects a decimal amount — whole pesos only', () => {
    const result = openCashSessionSchema.safeParse({ opening_cash: 1500.5, note: '' });
    expect(result.success).toBe(false);
  });
});

describe('closeCashSessionSchema', () => {
  it('accepts 0 counted cash', () => {
    expect(closeCashSessionSchema.safeParse({ counted_cash: 0, note: '' }).success).toBe(true);
  });

  it('rejects a missing counted amount', () => {
    expect(closeCashSessionSchema.safeParse({ counted_cash: NaN, note: '' }).success).toBe(false);
  });
});

describe('cashMovementSchema', () => {
  const base = {
    kind: 'expense' as const,
    category: 'supplies' as const,
    method: 'cash' as const,
    amount: 1000,
    note: '',
  };

  it('accepts a well-formed expense', () => {
    expect(cashMovementSchema.safeParse(base).success).toBe(true);
  });

  it('accepts a well-formed income', () => {
    expect(cashMovementSchema.safeParse({ ...base, kind: 'income', category: 'other_income' }).success).toBe(true);
  });

  it('rejects an income kind paired with an expense category', () => {
    const result = cashMovementSchema.safeParse({ ...base, kind: 'income', category: 'supplies' });
    expect(result.success).toBe(false);
  });

  it('rejects an expense kind paired with the income category', () => {
    const result = cashMovementSchema.safeParse({ ...base, kind: 'expense', category: 'other_income' });
    expect(result.success).toBe(false);
  });

  it('rejects an amount of 0 (must be positive)', () => {
    expect(cashMovementSchema.safeParse({ ...base, amount: 0 }).success).toBe(false);
  });

  it('rejects a missing amount', () => {
    expect(cashMovementSchema.safeParse({ ...base, amount: NaN }).success).toBe(false);
  });

  it('rejects an amount over the movement cap', () => {
    expect(cashMovementSchema.safeParse({ ...base, amount: MAX_MOVEMENT_AMOUNT_PESOS + 1 }).success).toBe(false);
  });

  it('rejects an unlisted payment method', () => {
    expect(cashMovementSchema.safeParse({ ...base, method: 'mercadopago' }).success).toBe(false);
  });

  it('rejects a decimal amount — whole pesos only', () => {
    expect(cashMovementSchema.safeParse({ ...base, amount: 1500.5 }).success).toBe(false);
  });
});

describe('voidCashMovementSchema', () => {
  it('accepts no note', () => {
    expect(voidCashMovementSchema.safeParse({}).success).toBe(true);
  });

  it('accepts a note', () => {
    expect(voidCashMovementSchema.safeParse({ note: 'error de tipeo' }).success).toBe(true);
  });
});
