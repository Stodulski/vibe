/**
 * The API's own caps for the product catalog
 * (`backend/internal/openapi/openapi.yaml`'s `productsCreate`/
 * `productsUpdate`/`productsRestock`): a product's `price` and a restock's
 * `total_cost` are both `INTEGER` centavos, capped at 2,000,000,000 — same
 * cap as a cash movement's `amount` (`features/cash/lib/money.ts`), kept as
 * its own constant here since it is a distinct API field with its own limit,
 * even though the number happens to match.
 *
 * Enforced client-side too so the form reports the same "too large" error
 * the server would answer with a 422, instead of only finding out on submit.
 *
 * The pesos<->centavos conversion itself (`pesosToCentavos`/
 * `centavosToPesos`) moved to `shared/lib/money.ts` (pos-products-screen
 * T5a): it is not product-specific, and `features/cash` needs the same
 * conversion. These caps stay here — they are product-specific.
 */
export const MAX_PRODUCT_PRICE_CENTAVOS = 2_000_000_000;
export const MAX_PRODUCT_PRICE_PESOS = MAX_PRODUCT_PRICE_CENTAVOS / 100;

export const MAX_RESTOCK_COST_CENTAVOS = 2_000_000_000;
export const MAX_RESTOCK_COST_PESOS = MAX_RESTOCK_COST_CENTAVOS / 100;

/** `restock.quantity`'s own cap (1..100000) and the adjustment's signed range (-100000..100000). */
export const MAX_STOCK_QUANTITY = 100_000;
