import { VENUE_TIME_ZONE } from './instants';

const DAY_TIME = new Intl.DateTimeFormat('es-AR', {
  timeZone: VENUE_TIME_ZONE,
  day: '2-digit',
  month: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
});

/**
 * An instant as "06/09 21:00", venue-local — compact enough for a history
 * row, where the date itself can matter.
 *
 * Moved here from `features/cash/lib/formatCashInstant.ts`'s
 * `formatSessionInstant` (pos-products-screen T5a): `features/products` had
 * duplicated this verbatim as `formatStockMovementInstant`, off the same
 * `VENUE_TIME_ZONE` constant, on the "features never import from one
 * another" reasoning — the repo's rule for that case is to move the shared
 * piece to `shared/` instead, same move as `shared/lib/paymentMethods.ts`.
 */
export function formatVenueDayTime(instant: string | null | undefined): string {
  if (!instant) return '';
  const date = new Date(instant);
  if (Number.isNaN(date.getTime())) return '';
  return DAY_TIME.format(date);
}
