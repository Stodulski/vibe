/**
 * Pesos <-> centavos conversion for the cashbox, plus the API's own caps for
 * its two kinds of money field (`backend/internal/openapi/openapi.yaml`,
 * `db/migrations/003_cashbox.sql`):
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
 */
export const MAX_SESSION_CASH_CENTAVOS = 99_999_999_999;
export const MAX_MOVEMENT_AMOUNT_CENTAVOS = 2_000_000_000;

export const MAX_SESSION_CASH_PESOS = MAX_SESSION_CASH_CENTAVOS / 100;
export const MAX_MOVEMENT_AMOUNT_PESOS = MAX_MOVEMENT_AMOUNT_CENTAVOS / 100;

/** Pesos (possibly with cents) to integer centavos — same rounding as `cleanBookingPayload`. */
export function pesosToCentavos(pesos: number): number {
  return Math.round(pesos * 100);
}

/** Centavos to pesos, for seeding a form field from a value the API returned. */
export function centavosToPesos(centavos: number): number {
  return centavos / 100;
}
