import { describe, it, expect, afterEach } from 'vitest';
import { makeProduct } from '@/test/factories';
import { MAX_CART_LINES, MAX_LINE_QUANTITY } from './cart';
import { loadCart, saveCart, clearCart, reconcileCartAgainstProducts } from './cartStorage';

/**
 * Same stub as `safeStorage.test.ts`'s `stubThrowingStorage`: happy-dom's real
 * `sessionStorage` is Proxy-backed, so spying on `Storage.prototype` silently
 * fails to intercept calls — the global has to be replaced wholesale.
 */
function stubThrowingSessionStorage() {
  const original = window.sessionStorage;
  Object.defineProperty(window, 'sessionStorage', {
    configurable: true,
    value: {
      getItem: () => {
        throw new DOMException('SecurityError');
      },
      setItem: () => {
        throw new DOMException('QuotaExceededError');
      },
      removeItem: () => {
        throw new DOMException('SecurityError');
      },
      clear: () => {
        /* noop stub */
      },
      key: () => null,
      length: 0,
    },
  });
  return () => {
    Object.defineProperty(window, 'sessionStorage', { configurable: true, value: original });
  };
}

afterEach(() => {
  window.sessionStorage.clear();
});

describe('loadCart / saveCart / clearCart', () => {
  it('round-trips a saved cart', () => {
    const lines = [
      { productId: 'p1', quantity: 2 },
      { productId: 'p2', quantity: 1 },
    ];
    saveCart('c1', lines);
    expect(loadCart('c1')).toEqual(lines);
  });

  it('returns an empty cart when nothing was ever saved', () => {
    expect(loadCart('c1')).toEqual([]);
  });

  it('is scoped per complex', () => {
    saveCart('c1', [{ productId: 'p1', quantity: 1 }]);
    expect(loadCart('c2')).toEqual([]);
  });

  it('clears the stored cart, so a later load is empty', () => {
    saveCart('c1', [{ productId: 'p1', quantity: 1 }]);
    clearCart('c1');
    expect(loadCart('c1')).toEqual([]);
  });

  it('removes the key outright when saving an empty cart', () => {
    saveCart('c1', [{ productId: 'p1', quantity: 1 }]);
    saveCart('c1', []);
    expect(window.sessionStorage.getItem('vibe_pos_cart_c1')).toBeNull();
  });

  it('ignores malformed JSON left in storage', () => {
    window.sessionStorage.setItem('vibe_pos_cart_c1', '{not json');
    expect(loadCart('c1')).toEqual([]);
  });

  it('ignores a stored shape that no longer validates', () => {
    window.sessionStorage.setItem('vibe_pos_cart_c1', JSON.stringify([{ productId: 'p1', quantity: -1 }]));
    expect(loadCart('c1')).toEqual([]);
  });

  it('never throws when sessionStorage itself throws (private browsing, quota, disabled storage)', () => {
    const restore = stubThrowingSessionStorage();
    try {
      expect(() => {
        saveCart('c1', [{ productId: 'p1', quantity: 1 }]);
      }).not.toThrow();
      expect(loadCart('c1')).toEqual([]);
    } finally {
      // Restoring the real `sessionStorage` unconditionally — even if an
      // assertion above throws — matters because this stub replaces the
      // global itself (see the comment on `stubThrowingSessionStorage`): a
      // failed assertion used to leave every later test in this file running
      // against a storage that always throws.
      restore();
    }
  });
});

describe('loadCart — clamping a stored cart', () => {
  it('respects MAX_CART_LINES, MAX_LINE_QUANTITY, and unique product ids on a cart written by an older/corrupted build', () => {
    const overLimitLines = Array.from({ length: MAX_CART_LINES + 5 }, (_, i) => ({
      productId: `p${String(i)}`,
      quantity: 1,
    }));
    // A duplicate id (merged) and a quantity past the per-line cap (clamped).
    overLimitLines.push({ productId: 'p0', quantity: 3 });
    overLimitLines[1] = { productId: 'p1', quantity: MAX_LINE_QUANTITY + 10 };
    window.sessionStorage.setItem('vibe_pos_cart_c1', JSON.stringify(overLimitLines));

    const result = loadCart('c1');

    expect(result.length).toBeLessThanOrEqual(MAX_CART_LINES);
    expect(new Set(result.map((l) => l.productId)).size).toBe(result.length);
    for (const line of result) expect(line.quantity).toBeLessThanOrEqual(MAX_LINE_QUANTITY);
    expect(result.find((l) => l.productId === 'p0')?.quantity).toBe(4);
    expect(result.find((l) => l.productId === 'p1')?.quantity).toBe(MAX_LINE_QUANTITY);
  });
});

describe('reconcileCartAgainstProducts', () => {
  it('keeps every line whose product is in the active list', () => {
    const products = [makeProduct({ id: 'p1' }), makeProduct({ id: 'p2' })];
    const lines = [
      { productId: 'p1', quantity: 1 },
      { productId: 'p2', quantity: 2 },
    ];
    const result = reconcileCartAgainstProducts(lines, products);
    expect(result.lines).toEqual(lines);
    expect(result.droppedStaleLines).toBe(false);
  });

  it('drops a line for a product no longer in the active list and flags it', () => {
    const products = [makeProduct({ id: 'p1' })];
    const lines = [
      { productId: 'p1', quantity: 1 },
      { productId: 'deactivated', quantity: 5 },
    ];
    const result = reconcileCartAgainstProducts(lines, products);
    expect(result.lines).toEqual([{ productId: 'p1', quantity: 1 }]);
    expect(result.droppedStaleLines).toBe(true);
  });
});
