import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSellCart } from './useSellCart';
import { MAX_CART_LINES } from '../../lib/cart';
import { makeProduct } from '@/test/factories';

const products = Array.from({ length: MAX_CART_LINES + 1 }, (_, i) =>
  makeProduct({ id: `p${String(i)}`, name: `Producto ${String(i)}`, active: true }),
);

describe('useSellCart — cart limit notice', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('refuses the extra line and shows the notice even when the taps land in one batch', () => {
    const { result } = renderHook(() => useSellCart('complex-limit', products, true));

    // Every tap in one act() batch: each updater runs after an update is
    // already queued, which is when a flag read right after setState goes stale.
    act(() => {
      for (const product of products) result.current.add(product.id);
    });

    expect(result.current.lines).toHaveLength(MAX_CART_LINES);
    expect(result.current.cartLimitNotice).toBe(true);
  });

  it('clears the notice on the next accepted tap', () => {
    const { result } = renderHook(() => useSellCart('complex-limit', products, true));

    act(() => {
      for (const product of products) result.current.add(product.id);
    });
    act(() => {
      result.current.add('p0');
    });

    expect(result.current.cartLimitNotice).toBe(false);
    expect(result.current.lines.find((line) => line.productId === 'p0')?.quantity).toBe(2);
  });
});
