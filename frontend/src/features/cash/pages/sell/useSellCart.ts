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
interface CartState {
  lines: CartLine[];
  /** Whether the last tap was refused because the cart is at `MAX_CART_LINES`. */
  limitNotice: boolean;
}

export function useSellCart(complexId: string | null, products: Product[], productsReady: boolean) {
  // Lines and the limit notice share one state so a tap updates both in the
  // same functional updater: React may run an updater later than the call
  // (whenever an update is already queued, as in two quick taps), so a flag
  // written inside it and read right after the call would be stale.
  const [cart, setCart] = useState<CartState>({ lines: [], limitNotice: false });
  const lines = cart.lines;
  const setLines = (update: CartLine[] | ((prev: CartLine[]) => CartLine[])) => {
    setCart((prev) => ({ ...prev, lines: typeof update === 'function' ? update(prev.lines) : update }));
  };
  const [droppedStaleNotice, setDroppedStaleNotice] = useState(false);
  const [loadedForComplex, setLoadedForComplex] = useState<string | null>(null);

  // Always replaces `lines` for the new complex — even an empty stored cart —
  // so a line added for the previous complex never survives a switch: the
  // guard used to skip `setLines` entirely when `loadCart` came back empty,
  // which left whatever was in memory for the prior complex on screen.
  if (complexId && productsReady && loadedForComplex !== complexId) {
    setLoadedForComplex(complexId);
    const stored = loadCart(complexId);
    const { lines: kept, droppedStaleLines } = reconcileCartAgainstProducts(stored, products);
    setLines(kept);
    setDroppedStaleNotice(droppedStaleLines);
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
    cartLimitNotice: cart.limitNotice,
    add: (productId: string) => {
      // Functional updater: `addToCart` runs against the latest lines, not
      // this render's closed-over value — two taps before a re-render (a real
      // "two quick taps at the counter" case) must not both compute from the
      // same starting quantity and lose an increment.
      setCart((prev) => {
        const { lines: next, rejected } = addToCart(prev.lines, productId);
        return { lines: rejected ? prev.lines : next, limitNotice: rejected };
      });
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
    /**
     * Removes exactly what a completed charge sent — subtracting `charged`'s
     * quantities per product from the *current* `lines`, dropping any line
     * that reaches zero — instead of `clear()`'s wholesale wipe. `clear()`
     * would also erase a line added or bumped while the request was still in
     * flight (the charge button is disabled, but nothing stops a stepper tap
     * that lands before the response does), a real money loss at the
     * counter.
     */
    removeCharged: (charged: CartLine[]) => {
      setLines((prev) =>
        prev
          .map((line) => {
            const chargedLine = charged.find((c) => c.productId === line.productId);
            return chargedLine ? { ...line, quantity: line.quantity - chargedLine.quantity } : line;
          })
          .filter((line) => line.quantity > 0),
      );
    },
  };
}
