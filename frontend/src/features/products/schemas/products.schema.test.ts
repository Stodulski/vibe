import { describe, it, expect } from 'vitest';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MAX_STOCK_QUANTITY } from '../lib/money';
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

  // money-centavos change: centavos are accepted everywhere now, up to 2
  // decimal digits.
  it('accepts a price with up to 2 decimal digits (centavos)', () => {
    expect(productFormSchema(false).safeParse({ ...base, price: 1500.5 }).success).toBe(true);
  });

  it('rejects a 3rd decimal digit', () => {
    const result = productFormSchema(false).safeParse({ ...base, price: 1500.555 });
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

  // A cleared `QuantityField` reports `NaN` (see `QuantityField`'s own
  // comment) — treated as "not set", the same as never having typed
  // anything, not as an invalid number.
  it('treats a NaN threshold (cleared input) as not set when unlocked', () => {
    const result = productFormSchema(false).safeParse({ ...base, low_stock_threshold: Number.NaN });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.low_stock_threshold).toBeUndefined();
    }
  });

  it('rejects a NaN threshold (cleared input) with the Spanish required-once-set error when locked', () => {
    const result = productFormSchema(true).safeParse({ ...base, low_stock_threshold: Number.NaN });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(['low_stock_threshold']);
      expect(result.error.issues[0]?.message).toBe(ES_AR.products.thresholdRequiredOnceSet);
    }
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

  // money-centavos change: centavos are accepted everywhere now, up to 2
  // decimal digits.
  it('accepts a total cost with up to 2 decimal digits (centavos)', () => {
    expect(restockSchema.safeParse({ ...base, total_cost: 5000.5 }).success).toBe(true);
  });

  it('rejects a 3rd decimal digit', () => {
    expect(restockSchema.safeParse({ ...base, total_cost: 5000.555 }).success).toBe(false);
  });
});

describe('adjustSchema', () => {
  it('accepts a valid counted quantity and reason', () => {
    expect(adjustSchema(0).safeParse({ counted: 12, reason: 'count_correction' }).success).toBe(true);
  });

  it('accepts a counted quantity of 0 (the schema does not enforce non-zero difference — the dialog does)', () => {
    expect(adjustSchema(0).safeParse({ counted: 0, reason: 'breakage' }).success).toBe(true);
  });

  it('rejects a negative counted quantity', () => {
    expect(adjustSchema(0).safeParse({ counted: -1, reason: 'breakage' }).success).toBe(false);
  });

  it('rejects an unknown reason', () => {
    expect(adjustSchema(0).safeParse({ counted: 5, reason: 'lost' }).success).toBe(false);
  });

  // The signed difference the API receives is `counted - currentStock`, not
  // `counted` alone — capped at the API's own ±100000 (`MAX_STOCK_QUANTITY`,
  // shared with `restockSchema`'s own quantity cap).
  it('accepts a counted quantity exactly at the +100000 difference boundary', () => {
    const currentStock = 0;
    const counted = currentStock + MAX_STOCK_QUANTITY;
    expect(adjustSchema(currentStock).safeParse({ counted, reason: 'count_correction' }).success).toBe(true);
  });

  it('rejects a counted quantity one over the +100000 difference boundary', () => {
    const currentStock = 0;
    const counted = currentStock + MAX_STOCK_QUANTITY + 1;
    const result = adjustSchema(currentStock).safeParse({ counted, reason: 'count_correction' });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(['counted']);
      expect(result.error.issues[0]?.message).toBe(ES_AR.validation.quantityTooLarge);
    }
  });

  it('accepts a counted quantity exactly at the -100000 difference boundary', () => {
    const currentStock = MAX_STOCK_QUANTITY;
    expect(adjustSchema(currentStock).safeParse({ counted: 0, reason: 'count_correction' }).success).toBe(true);
  });

  it('rejects a counted quantity one over the -100000 difference boundary', () => {
    const currentStock = MAX_STOCK_QUANTITY + 1;
    const result = adjustSchema(currentStock).safeParse({ counted: 0, reason: 'count_correction' });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(['counted']);
    }
  });
});
