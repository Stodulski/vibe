import { describe, it, expect } from 'vitest';
import { pesosToCentavos } from '@/shared/lib/money';
import {
  MAX_PRODUCT_PRICE_PESOS,
  MAX_PRODUCT_PRICE_CENTAVOS,
  MAX_RESTOCK_COST_PESOS,
  MAX_RESTOCK_COST_CENTAVOS,
} from './money';

// `pesosToCentavos`/`centavosToPesos` themselves are tested in
// `shared/lib/money.test.ts` now that they live there (pos-products-screen
// T5a); this file keeps only the product-specific caps.
describe('products money caps', () => {
  it('matches the API product price cap (INTEGER column), a whole number of pesos', () => {
    expect(MAX_PRODUCT_PRICE_CENTAVOS).toBe(2_000_000_000);
    expect(Number.isInteger(MAX_PRODUCT_PRICE_PESOS)).toBe(true);
    expect(pesosToCentavos(MAX_PRODUCT_PRICE_PESOS)).toBe(MAX_PRODUCT_PRICE_CENTAVOS);
  });

  it('matches the API restock total_cost cap (the same cash-movement INTEGER cap)', () => {
    expect(MAX_RESTOCK_COST_CENTAVOS).toBe(2_000_000_000);
    expect(pesosToCentavos(MAX_RESTOCK_COST_PESOS)).toBe(MAX_RESTOCK_COST_CENTAVOS);
  });
});
