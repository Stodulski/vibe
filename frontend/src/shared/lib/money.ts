/**
 * Pesos <-> centavos conversion, shared by the cashbox and the product
 * catalog — every money-carrying API field is integer centavos, and every
 * form collects whole pesos.
 *
 * Moved here from `features/cash/lib/money.ts` (pos-products-screen T5a):
 * `features/products` had duplicated these two functions verbatim in its own
 * `lib/money.ts` on the "features never import from one another" reasoning —
 * the repo's rule for that case is to move the shared piece to `shared/`
 * instead, same move as `shared/lib/paymentMethods.ts`. Each feature's own
 * `lib/money.ts` keeps its feature-specific caps (session cash / movement
 * amount for cash; product price / restock cost / stock quantity for
 * products), which are not shared.
 */

/** Pesos (possibly with cents) to integer centavos — same rounding as `cleanBookingPayload`. */
export function pesosToCentavos(pesos: number): number {
  return Math.round(pesos * 100);
}

/** Centavos to pesos, for seeding a form field from a value the API returned. */
export function centavosToPesos(centavos: number): number {
  return centavos / 100;
}
