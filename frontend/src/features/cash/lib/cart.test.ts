import { describe, it, expect } from 'vitest';
import { makeProduct } from '@/test/factories';
import {
  addToCart,
  incrementLine,
  decrementLine,
  removeLine,
  setLineQuantity,
  cartTotal,
  lineHasStockWarning,
  MAX_CART_LINES,
  MAX_LINE_QUANTITY,
} from './cart';

describe('addToCart', () => {
  it('adds a new line with quantity 1', () => {
    const { lines, rejected } = addToCart([], 'p1');
    expect(lines).toEqual([{ productId: 'p1', quantity: 1 }]);
    expect(rejected).toBe(false);
  });

  it('increments an existing line instead of adding a duplicate', () => {
    const { lines } = addToCart([{ productId: 'p1', quantity: 1 }], 'p1');
    expect(lines).toEqual([{ productId: 'p1', quantity: 2 }]);
  });

  it('refuses a 51st distinct line', () => {
    const full = Array.from({ length: MAX_CART_LINES }, (_, i) => ({ productId: `p${String(i)}`, quantity: 1 }));
    const { lines, rejected } = addToCart(full, 'new-product');
    expect(rejected).toBe(true);
    expect(lines).toBe(full);
    expect(lines).toHaveLength(MAX_CART_LINES);
  });

  it('clamps an existing line at the 1,000-unit cap instead of rejecting', () => {
    const { lines, rejected } = addToCart([{ productId: 'p1', quantity: MAX_LINE_QUANTITY }], 'p1');
    expect(rejected).toBe(false);
    expect(lines[0]?.quantity).toBe(MAX_LINE_QUANTITY);
  });
});

describe('incrementLine / decrementLine', () => {
  it('increments an existing line', () => {
    expect(incrementLine([{ productId: 'p1', quantity: 1 }], 'p1')).toEqual([{ productId: 'p1', quantity: 2 }]);
  });

  it('never increments past the 1,000-unit cap', () => {
    const lines = incrementLine([{ productId: 'p1', quantity: MAX_LINE_QUANTITY }], 'p1');
    expect(lines[0]?.quantity).toBe(MAX_LINE_QUANTITY);
  });

  it('decrements an existing line', () => {
    expect(decrementLine([{ productId: 'p1', quantity: 2 }], 'p1')).toEqual([{ productId: 'p1', quantity: 1 }]);
  });

  it('removes the line once it decrements past 1', () => {
    expect(decrementLine([{ productId: 'p1', quantity: 1 }], 'p1')).toEqual([]);
  });

  it('is a no-op for a product not in the cart', () => {
    expect(decrementLine([], 'p1')).toEqual([]);
  });
});

describe('removeLine', () => {
  it('drops the matching line and keeps the rest', () => {
    const lines = [
      { productId: 'p1', quantity: 1 },
      { productId: 'p2', quantity: 3 },
    ];
    expect(removeLine(lines, 'p1')).toEqual([{ productId: 'p2', quantity: 3 }]);
  });
});

describe('setLineQuantity', () => {
  it('sets a valid quantity', () => {
    expect(setLineQuantity([{ productId: 'p1', quantity: 1 }], 'p1', 5)).toEqual([{ productId: 'p1', quantity: 5 }]);
  });

  it('clamps below 1 up to 1', () => {
    expect(setLineQuantity([{ productId: 'p1', quantity: 5 }], 'p1', -3)).toEqual([{ productId: 'p1', quantity: 1 }]);
  });

  it('clamps above 1,000 down to 1,000', () => {
    expect(setLineQuantity([{ productId: 'p1', quantity: 5 }], 'p1', 5000)).toEqual([
      { productId: 'p1', quantity: MAX_LINE_QUANTITY },
    ]);
  });

  it('ignores a non-finite value (NaN from a cleared input)', () => {
    const lines = [{ productId: 'p1', quantity: 5 }];
    expect(setLineQuantity(lines, 'p1', Number.NaN)).toBe(lines);
  });

  it('rounds a fractional quantity', () => {
    expect(setLineQuantity([{ productId: 'p1', quantity: 1 }], 'p1', 2.7)).toEqual([{ productId: 'p1', quantity: 3 }]);
  });
});

describe('cartTotal', () => {
  it('sums quantity times the currently loaded price', () => {
    const products = [makeProduct({ id: 'p1', price: 100 }), makeProduct({ id: 'p2', price: 250 })];
    const lines = [
      { productId: 'p1', quantity: 2 },
      { productId: 'p2', quantity: 1 },
    ];
    expect(cartTotal(lines, products)).toBe(2 * 100 + 250);
  });

  it('excludes a line whose product is no longer in the loaded catalog', () => {
    const products = [makeProduct({ id: 'p1', price: 100 })];
    const lines = [
      { productId: 'p1', quantity: 1 },
      { productId: 'gone', quantity: 3 },
    ];
    expect(cartTotal(lines, products)).toBe(100);
  });
});

describe('lineHasStockWarning', () => {
  it('warns when a tracked product would go negative', () => {
    const product = makeProduct({ tracks_stock: true, stock_on_hand: 2 });
    expect(lineHasStockWarning({ productId: 'p1', quantity: 3 }, product)).toBe(true);
  });

  it('warns when a tracked product is already at or below zero', () => {
    const product = makeProduct({ tracks_stock: true, stock_on_hand: 0 });
    expect(lineHasStockWarning({ productId: 'p1', quantity: 1 }, product)).toBe(true);
  });

  it('does not warn when enough stock remains', () => {
    const product = makeProduct({ tracks_stock: true, stock_on_hand: 5 });
    expect(lineHasStockWarning({ productId: 'p1', quantity: 3 }, product)).toBe(false);
  });

  it('never warns for a product that does not track stock', () => {
    const product = makeProduct({ tracks_stock: false, stock_on_hand: -5 });
    expect(lineHasStockWarning({ productId: 'p1', quantity: 1 }, product)).toBe(false);
  });

  it('never warns when the product is unknown (undefined)', () => {
    expect(lineHasStockWarning({ productId: 'p1', quantity: 1 }, undefined)).toBe(false);
  });
});
