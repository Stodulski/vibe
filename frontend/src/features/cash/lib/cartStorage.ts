import { z } from 'zod';
import { safeSessionStorage } from '@/shared/lib/safeStorage';
import { MAX_CART_LINES, MAX_LINE_QUANTITY } from './cart';
import type { CartLine } from './cart';
import { MAX_BALANCE_LINES, type BalanceLine } from './balanceCart';
import type { Product } from '@/shared/types/api.types';

const cartLineSchema = z.object({
  productId: z.string(),
  quantity: z.number().int().positive(),
});

const cartSchema = z.array(cartLineSchema);

/** Per-complex, not one global key — the same operator can sell for a different complex in another tab. */
function storageKey(complexId: string): string {
  return `vibe_pos_cart_${complexId}`;
}

/**
 * Enforces the same rules `addToCart`/`incrementLine` enforce on a live
 * cart — {@link MAX_CART_LINES} distinct lines, {@link MAX_LINE_QUANTITY} per
 * line, one line per product — against whatever a *stored* cart happens to
 * contain. The schema above only validates each line's own shape; it can't
 * catch a duplicate `productId` (merged here, quantity capped) or a cart that
 * grew past either cap while an older build wrote it.
 */
function clampStoredCart(lines: CartLine[]): CartLine[] {
  const merged = new Map<string, number>();
  for (const line of lines) {
    const quantity = Math.min(MAX_LINE_QUANTITY, (merged.get(line.productId) ?? 0) + line.quantity);
    merged.set(line.productId, quantity);
  }
  return [...merged.entries()].slice(0, MAX_CART_LINES).map(([productId, quantity]) => ({ productId, quantity }));
}

/**
 * Reads back a cart left behind by an accidental reload — `null`/malformed
 * storage, a private-browsing throw, or a shape this version no longer
 * recognizes all come back as an empty cart (`safeSessionStorage.getJSON`'s
 * own single empty case).
 */
export function loadCart(complexId: string): CartLine[] {
  const stored = safeSessionStorage.getJSON(storageKey(complexId), cartSchema) ?? [];
  return clampStoredCart(stored);
}

/** Persists the cart, or clears the key entirely once it's empty — an empty array is not worth a storage write to keep around. */
export function saveCart(complexId: string, lines: CartLine[]): void {
  if (lines.length === 0) {
    safeSessionStorage.remove(storageKey(complexId));
    return;
  }
  safeSessionStorage.set(storageKey(complexId), JSON.stringify(lines));
}

export function clearCart(complexId: string): void {
  safeSessionStorage.remove(storageKey(complexId));
}

export interface ReconciledCart {
  lines: CartLine[];
  /** True when at least one stored line was dropped — the caller shows a one-time notice. */
  droppedStaleLines: boolean;
}

/**
 * Drops any stored line whose product is no longer in the currently loaded
 * *active* catalog (deactivated, or deleted, while the cart sat in
 * storage) — `odd/tasks/pos-cashbox.md` T5b: "stale lines for products no
 * longer active are dropped on load with a notice." `products` must already
 * be the active-only list the sell screen queries with `active=true`.
 */
export function reconcileCartAgainstProducts(lines: CartLine[], products: Product[]): ReconciledCart {
  const activeIds = new Set(products.map((p) => p.id));
  const kept = lines.filter((line) => activeIds.has(line.productId));
  return { lines: kept, droppedStaleLines: kept.length < lines.length };
}

const balanceLineSchema = z.object({
  bookingId: z.string(),
  courtName: z.string(),
  startTime: z.string(),
  clientName: z.string(),
  amount: z.number().int().positive(),
});

const balanceCartSchema = z.array(balanceLineSchema);

/** Per-complex, the same as the product cart. A separate key, so a product cart stored by an earlier build reads back unchanged. */
function balanceStorageKey(complexId: string): string {
  return `vibe_pos_balances_${complexId}`;
}

/** One line per booking and at most the checkout's cap, whatever a stored cart holds. */
function clampStoredBalances(lines: BalanceLine[]): BalanceLine[] {
  const seen = new Set<string>();
  const unique = lines.filter((line) => {
    if (seen.has(line.bookingId)) return false;
    seen.add(line.bookingId);
    return true;
  });
  return unique.slice(0, MAX_BALANCE_LINES);
}

/** Reads back the booking balances left behind by a reload; anything malformed comes back as an empty cart. */
export function loadBalanceCart(complexId: string): BalanceLine[] {
  const stored = safeSessionStorage.getJSON(balanceStorageKey(complexId), balanceCartSchema) ?? [];
  return clampStoredBalances(stored);
}

export function saveBalanceCart(complexId: string, lines: BalanceLine[]): void {
  if (lines.length === 0) {
    safeSessionStorage.remove(balanceStorageKey(complexId));
    return;
  }
  safeSessionStorage.set(balanceStorageKey(complexId), JSON.stringify(lines));
}

export function clearBalanceCart(complexId: string): void {
  safeSessionStorage.remove(balanceStorageKey(complexId));
}
