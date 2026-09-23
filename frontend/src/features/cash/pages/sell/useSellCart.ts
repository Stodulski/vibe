import { useEffect, useState } from 'react';
import { loadCart, saveCart, clearCart, reconcileCartAgainstProducts } from '../../lib/cartStorage';
import { addToCart, decrementLine, incrementLine, removeLine, setLineQuantity, type CartLine } from '../../lib/cart';
import type { Product } from '@/shared/types/api.types';

/**
 * The cart itself: lines, the tap/stepper/remove actions, and its
 * sessionStorage persistence — split out of `useSellPage` so neither function
 * grows past this repo's `max-lines-per-function` limit.
 *
 * The initial load-and-reconcile below runs during render (guarded by
 * `loadedForComplex`, the same "adjust state during render on a tracked
 * previous value" shape `VoidMovementDialog`'s `lastMovementId` and
 * `useCashPage`'s `lastSessionId` already use), not inside a `useEffect` —
 * `react-hooks/set-state-in-effect` flags a synchronous `setState` inside an
 * effect body, and this genuinely has no external system to subscribe to; it
 * only needs to run once real data (`productsReady`) is available.
 */
export function useSellCart(complexId: string | null, products: Product[], productsReady: boolean) {
  const [lines, setLines] = useState<CartLine[]>([]);
  const [droppedStaleNotice, setDroppedStaleNotice] = useState(false);
  const [cartLimitNotice, setCartLimitNotice] = useState(false);
  const [loadedForComplex, setLoadedForComplex] = useState<string | null>(null);

  if (complexId && productsReady && loadedForComplex !== complexId) {
    setLoadedForComplex(complexId);
    const stored = loadCart(complexId);
    if (stored.length > 0) {
      const { lines: kept, droppedStaleLines } = reconcileCartAgainstProducts(stored, products);
      setLines(kept);
      setDroppedStaleNotice(droppedStaleLines);
    }
  }

  // Persisting is a real effect: it synchronizes React's own state with an
  // external system (sessionStorage) any time `lines` changes.
  useEffect(() => {
    if (!complexId || loadedForComplex !== complexId) return;
    saveCart(complexId, lines);
  }, [complexId, loadedForComplex, lines]);

  return {
    lines,
    droppedStaleNotice,
    dismissDroppedStaleNotice: () => {
      setDroppedStaleNotice(false);
    },
    cartLimitNotice,
    add: (productId: string) => {
      const { lines: next, rejected } = addToCart(lines, productId);
      setCartLimitNotice(rejected);
      if (!rejected) setLines(next);
    },
    increment: (productId: string) => {
      setLines((prev) => incrementLine(prev, productId));
    },
    decrement: (productId: string) => {
      setLines((prev) => decrementLine(prev, productId));
    },
    setQuantity: (productId: string, quantity: number) => {
      setLines((prev) => setLineQuantity(prev, productId, quantity));
    },
    remove: (productId: string) => {
      setLines((prev) => removeLine(prev, productId));
    },
    clear: () => {
      setLines([]);
      if (complexId) clearCart(complexId);
    },
  };
}
