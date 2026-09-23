import { useState } from 'react';

/**
 * `ProductDetailPage`'s four dialog open flags, reset whenever `productId`
 * changes.
 *
 * Navigating client-side from one `/cash/products/:productId` to another
 * keeps the same page component instance mounted (only the route param
 * changes), so a dialog left open for product A would otherwise reopen over
 * product B the instant its data arrives. Adjusted during render off a
 * tracked previous value — same pattern as `VoidMovementDialog`'s
 * `lastMovementId` — not a `useEffect`, which would setState after commit
 * and cause an extra render (T3/T5a review lesson: "dialog state must never
 * leak into the next entity").
 */
export function useProductDetailDialogs(productId: string | undefined) {
  const [editOpen, setEditOpen] = useState(false);
  const [restockOpen, setRestockOpen] = useState(false);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [toggleOpen, setToggleOpen] = useState(false);

  const [lastProductId, setLastProductId] = useState(productId);
  if (productId !== lastProductId) {
    setLastProductId(productId);
    setEditOpen(false);
    setRestockOpen(false);
    setAdjustOpen(false);
    setToggleOpen(false);
  }

  return {
    editOpen,
    setEditOpen,
    restockOpen,
    setRestockOpen,
    adjustOpen,
    setAdjustOpen,
    toggleOpen,
    setToggleOpen,
  };
}
