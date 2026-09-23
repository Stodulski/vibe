import { describe, it, expect } from 'vitest';
import { productFormSchema, restockSchema, adjustSchema } from './products.schema';

describe('productFormSchema', () => {
  const base = { name: 'Agua', category: 'Bebidas', price: 1500, tracks_stock: true };

  it('accepts a valid product with no threshold', () => {
    expect(productFormSchema(false).safeParse(base).success).toBe(true);
  });

  it('rejects a name over 120 characters', () => {
    const result = productFormSchema(false).safeParse({ ...base, name: 'a'.repeat(121) });
    expect(result.success).toBe(false);
  });

  it('rejects a category over 60 characters', () => {
    const result = productFormSchema(false).safeParse({ ...base, category: 'a'.repeat(61) });
    expect(result.success).toBe(false);
  });

  it('accepts price 0 (a free product is allowed)', () => {
    expect(productFormSchema(false).safeParse({ ...base, price: 0 }).success).toBe(true);
  });

  it('rejects a non-integer price (whole pesos only)', () => {
    const result = productFormSchema(false).safeParse({ ...base, price: 1500.5 });
    expect(result.success).toBe(false);
  });

  it('rejects a negative price', () => {
    const result = productFormSchema(false).safeParse({ ...base, price: -1 });
    expect(result.success).toBe(false);
  });

  it('accepts an omitted threshold when the product never had one', () => {
    expect(productFormSchema(false).safeParse(base).success).toBe(true);
  });

  it('rejects an omitted threshold once the product already has one (locked)', () => {
    const result = productFormSchema(true).safeParse(base);
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(['low_stock_threshold']);
    }
  });

  it('accepts a replaced (not removed) threshold when locked', () => {
    expect(productFormSchema(true).safeParse({ ...base, low_stock_threshold: 5 }).success).toBe(true);
  });
});

describe('restockSchema', () => {
  const base = { quantity: 10, total_cost: 5000, method: 'cash' as const };

  it('accepts a valid restock', () => {
    expect(restockSchema.safeParse(base).success).toBe(true);
  });

  it('rejects a zero or negative quantity', () => {
    expect(restockSchema.safeParse({ ...base, quantity: 0 }).success).toBe(false);
  });

  it('rejects a quantity over 100000', () => {
    expect(restockSchema.safeParse({ ...base, quantity: 100_001 }).success).toBe(false);
  });

  it('rejects a total cost under 1 peso (a free restock is an adjustment)', () => {
    expect(restockSchema.safeParse({ ...base, total_cost: 0 }).success).toBe(false);
  });

  it('rejects a non-integer total cost', () => {
    expect(restockSchema.safeParse({ ...base, total_cost: 5000.5 }).success).toBe(false);
  });
});

describe('adjustSchema', () => {
  it('accepts a valid counted quantity and reason', () => {
    expect(adjustSchema.safeParse({ counted: 12, reason: 'count_correction' }).success).toBe(true);
  });

  it('accepts a counted quantity of 0 (the schema does not enforce non-zero difference — the dialog does)', () => {
    expect(adjustSchema.safeParse({ counted: 0, reason: 'breakage' }).success).toBe(true);
  });

  it('rejects a negative counted quantity', () => {
    expect(adjustSchema.safeParse({ counted: -1, reason: 'breakage' }).success).toBe(false);
  });

  it('rejects an unknown reason', () => {
    expect(adjustSchema.safeParse({ counted: 5, reason: 'lost' }).success).toBe(false);
  });
});
