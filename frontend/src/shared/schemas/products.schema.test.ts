import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeProduct, makeStockMovement } from '@/test/factories';
import {
  productSchema,
  stockMovementSchema,
  productsListResponseSchema,
  productEnvelopeSchema,
  productStockMovementsResponseSchema,
  productStockWriteResponseSchema,
} from './products.schema';

describe('productSchema', () => {
  it('validates a stock-tracked product with no threshold', () => {
    expect(productSchema.safeParse(makeProduct()).success).toBe(true);
  });

  it('validates a product with a low-stock threshold and category null', () => {
    const product = makeProduct({ category: null, low_stock_threshold: 5, low_stock: true });
    expect(productSchema.safeParse(product).success).toBe(true);
  });

  it('validates a product that does not track stock (no version)', () => {
    const { version: _version, ...rest } = makeProduct({ tracks_stock: false, stock_on_hand: 0 });
    expect(productSchema.safeParse(rest).success).toBe(true);
  });
});

describe('stockMovementSchema', () => {
  it('validates a restock movement', () => {
    expect(stockMovementSchema.safeParse(makeStockMovement()).success).toBe(true);
  });

  it('validates an adjustment movement with a reason', () => {
    const movement = makeStockMovement({ kind: 'adjustment', reason: 'breakage', quantity: -2, cash_movement_id: null });
    expect(stockMovementSchema.safeParse(movement).success).toBe(true);
  });

  it('validates a sale-void movement linked to a sale', () => {
    const movement = makeStockMovement({ kind: 'sale_void', sale_id: 's1', cash_movement_id: null, quantity: 3 });
    expect(stockMovementSchema.safeParse(movement).success).toBe(true);
  });

  it('rejects an unknown kind', () => {
    const movement = { ...makeStockMovement(), kind: 'unknown' };
    expect(stockMovementSchema.safeParse(movement).success).toBe(false);
  });
});

describe('response envelopes', () => {
  it('validates a products list response', () => {
    expect(productsListResponseSchema.safeParse({ products: [makeProduct()] }).success).toBe(true);
  });

  it('validates a single-product envelope', () => {
    expect(productEnvelopeSchema.safeParse({ product: makeProduct() }).success).toBe(true);
  });

  it('validates a paginated stock-movements response', () => {
    const body = {
      stock_movements: [makeStockMovement()],
      metadata: { has_more: false },
    };
    expect(productStockMovementsResponseSchema.safeParse(body).success).toBe(true);
  });

  it('validates a restock/adjustment write response', () => {
    const body = { product: makeProduct(), stock_movement: makeStockMovement() };
    expect(productStockWriteResponseSchema.safeParse(body).success).toBe(true);
  });
});
