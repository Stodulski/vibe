/**
 * The API's own caps for the cashbox's two kinds of money field
 * (`backend/internal/openapi/openapi.yaml`, `db/migrations/003_cashbox.sql`):
 *
 * - Session cash (`opening_cash`, `counted_cash`) is `BIGINT`, capped at
 *   99,999,999,999 centavos — the backend originally capped it at the
 *   `INTEGER` ceiling before the column was widened (pos-cashbox T2 review),
 *   which would have refused a completely ordinary Saturday till.
 * - A single movement's `amount` is `INTEGER`, capped at 2,000,000,000
 *   centavos.
 *
 * Enforced client-side too so the form reports the same "too large" error the
 * server would answer with a 422, instead of only finding out on submit.
 *
 * The pesos<->centavos conversion itself (`pesosToCentavos`/
 * `centavosToPesos`) moved to `shared/lib/money.ts` (pos-products-screen
 * T5a): it is not cash-specific, and `features/products` needs the same
 * conversion. These caps stay here — they are cash-specific.
 */
export const MAX_SESSION_CASH_CENTAVOS = 99_999_999_999;
export const MAX_MOVEMENT_AMOUNT_CENTAVOS = 2_000_000_000;

// `Math.floor`, not a plain `/ 100`: 99,999,999,999 centavos is not an even
// number of pesos (999,999,999.99), and the cashbox works in whole pesos
// only (see MoneyPesosField, `cash.schema.ts`'s `.int()` checks) — a plain
// division would make this cap itself fail its own "whole pesos" rule.
// Trimming 99 centavos off an arbitrary safety ceiling changes nothing in
// practice.
export const MAX_SESSION_CASH_PESOS = Math.floor(MAX_SESSION_CASH_CENTAVOS / 100);
export const MAX_MOVEMENT_AMOUNT_PESOS = MAX_MOVEMENT_AMOUNT_CENTAVOS / 100;
