import { describe, it, expect } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useProductDetailDialogs } from './useProductDetailDialogs';

describe('useProductDetailDialogs', () => {
  it('closes every open dialog once productId changes', () => {
    const { result, rerender } = renderHook<ReturnType<typeof useProductDetailDialogs>, { productId?: string }>(
      ({ productId }) => useProductDetailDialogs(productId),
      { initialProps: { productId: 'p1' } },
    );

    act(() => {
      result.current.setEditOpen(true);
      result.current.setRestockOpen(true);
      result.current.setAdjustOpen(true);
      result.current.setToggleOpen(true);
    });
    rerender({ productId: 'p1' });

    expect(result.current.editOpen).toBe(true);
    expect(result.current.restockOpen).toBe(true);
    expect(result.current.adjustOpen).toBe(true);
    expect(result.current.toggleOpen).toBe(true);

    rerender({ productId: 'p2' });

    expect(result.current.editOpen).toBe(false);
    expect(result.current.restockOpen).toBe(false);
    expect(result.current.adjustOpen).toBe(false);
    expect(result.current.toggleOpen).toBe(false);
  });
});
