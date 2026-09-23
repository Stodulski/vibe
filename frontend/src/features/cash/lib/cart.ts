import type { Product } from '@/shared/types/api.types';

/**
 * The "Vender" screen's cart — product id + quantity only, never a price
 * snapshot: the total shown is always derived from the currently loaded
 * product prices (`cartTotal` below), and the server computes the sale's real
 * total independently (`odd/tasks/pos-cashbox.md` T5b: "label it as the price
 * shown; the server total is authoritative after charging").
 */
export interface CartLine {
  productId: string;
  quantity: number;
}

/** Owner decision: 1..50 distinct lines, 1..1,000 units per line. */
export const MAX_CART_LINES = 50;
const MIN_LINE_QUANTITY = 1;
export const MAX_LINE_QUANTITY = 1000;

export interface AddToCartResult {
  lines: CartLine[];
  /** True when a brand-new (51st) line was refused — an existing line at the cap is silently clamped instead. */
  rejected: boolean;
}

/** Tapping a tile: increments an existing line, or adds a new one — refused past {@link MAX_CART_LINES} distinct lines. */
export function addToCart(lines: CartLine[], productId: string): AddToCartResult {
  const existing = lines.find((line) => line.productId === productId);
  if (existing) {
    if (existing.quantity >= MAX_LINE_QUANTITY) return { lines, rejected: false };
    const next = lines.map((line) => (line.productId === productId ? { ...line, quantity: line.quantity + 1 } : line));
    return { lines: next, rejected: false };
  }
  if (lines.length >= MAX_CART_LINES) return { lines, rejected: true };
  return { lines: [...lines, { productId, quantity: 1 }], rejected: false };
}

/** The cart's own "+" button — same clamp as {@link addToCart}, but never refused: the line already exists. */
export function incrementLine(lines: CartLine[], productId: string): CartLine[] {
  return lines.map((line) =>
    line.productId === productId ? { ...line, quantity: Math.min(MAX_LINE_QUANTITY, line.quantity + 1) } : line,
  );
}

/** The cart's own "-" button — decrementing a line at quantity 1 removes it. */
export function decrementLine(lines: CartLine[], productId: string): CartLine[] {
  const line = lines.find((l) => l.productId === productId);
  if (!line) return lines;
  if (line.quantity <= MIN_LINE_QUANTITY) return removeLine(lines, productId);
  return lines.map((l) => (l.productId === productId ? { ...l, quantity: l.quantity - 1 } : l));
}

/** A typed quantity (from a line's own numeric field), clamped to the valid range. */
export function setLineQuantity(lines: CartLine[], productId: string, quantity: number): CartLine[] {
  if (!Number.isFinite(quantity)) return lines;
  const clamped = Math.min(MAX_LINE_QUANTITY, Math.max(MIN_LINE_QUANTITY, Math.round(quantity)));
  return lines.map((line) => (line.productId === productId ? { ...line, quantity: clamped } : line));
}

export function removeLine(lines: CartLine[], productId: string): CartLine[] {
  return lines.filter((line) => line.productId !== productId);
}

/**
 * The sum of every line's current price × quantity, from `products` (the
 * live catalog) — not a price snapshot taken when the line was added, so a
 * price edited mid-sale is reflected immediately. A line whose product is no
 * longer in `products` (deactivated after being added) contributes nothing;
 * the UI is expected to flag that line separately rather than silently drop
 * it from the total.
 */
export function cartTotal(lines: CartLine[], products: Product[]): number {
  let total = 0;
  for (const line of lines) {
    const product = products.find((p) => p.id === line.productId);
    if (product) total += product.price * line.quantity;
  }
  return total;
}

/**
 * Non-blocking warning rule: a tracked product that would end at or below
 * zero after this line sells. `stock_on_hand <= 0` is already covered by this
 * same check — quantity is always >= 1, so an already-nonpositive stock makes
 * `stock_on_hand - quantity < 0` true on its own.
 */
export function lineHasStockWarning(line: CartLine, product: Product | undefined): boolean {
  if (!product?.tracks_stock) return false;
  return product.stock_on_hand - line.quantity < 0;
}
