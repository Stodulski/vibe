import { resolvePriceWindow } from './priceWindow';
import type { CourtPrice, CourtWithPrices, Schedule } from '@/shared/types/api.types';

/** A booking is priced in blocks this long, matching the server's walk. */
const BLOCK_MINUTES = 30;

/**
 * The hourly rate covering `min`, or `null` when no band does.
 *
 * `min` is minutes from the band's weekday's own midnight and may exceed 1440:
 * an hour sold out of Thursday's 08:00–01:30 window at 00:30 is Thursday minute
 * 1470, and Thursday's bands are laid out across the same span. Minutes rather
 * than a clock reading because a reading cannot say which day it belongs to —
 * "00:30" is the tail of one night and the small hours of the next, priced from
 * different cards.
 *
 * There is no fallback. It used to fall back to any band on that weekday, and
 * then to any band at all, so an hour with no rate quietly borrowed someone
 * else's and the booking failed on submit with an error the preview had already
 * contradicted.
 */
export function rateAt(prices: CourtPrice[], day: string, min: number): number | null {
  const band = prices.find((p) => p.day_type === day && min >= p.from_min && min < p.to_min);
  return band?.price ?? null;
}

/**
 * What a booking of `durationMinutes` from `startTime` costs, or `null` when
 * any part of it is unpriced.
 *
 * `court_prices.price` is an HOURLY rate. The server walks the booking in
 * 30-minute blocks and charges each at its band's rate, summing the rates and
 * halving once at the end rather than rounding each block — so this does the
 * same, block for block, and lands on the same number.
 *
 * The whole booking is priced against the window it was sold out of, not
 * against the calendar day. A Friday-night session running to 01:00 is Friday's,
 * all four blocks of it. See `resolvePriceWindow`, which mirrors the server's
 * own resolution.
 */
export function findTotalPrice(
  courts: CourtWithPrices[],
  courtId: string,
  date: string,
  startTime: string,
  durationMinutes: number,
  schedules: Schedule[],
): number | null {
  const court = courts.find((c) => c.id === courtId);
  if (!court || !date || !startTime) return null;

  const window = resolvePriceWindow(schedules, date, startTime);

  let hourlyRateSum = 0;
  for (let i = 0; i < durationMinutes / BLOCK_MINUTES; i++) {
    const rate = rateAt(court.prices, window.day, window.startMin + i * BLOCK_MINUTES);
    if (rate === null) return null;
    hourlyRateSum += rate;
  }
  return Math.round(hourlyRateSum / 2);
}
