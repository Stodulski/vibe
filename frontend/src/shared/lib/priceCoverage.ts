import { timeToMinutes } from '@/shared/lib/time';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

const MINUTES_PER_DAY = 24 * 60;

type OpeningDay = Pick<Schedule, 'day' | 'open_time' | 'close_time' | 'is_closed'>;
type PricedCourt = Pick<CourtWithPrices, 'id' | 'name' | 'is_active' | 'prices'>;

/** A stretch of one day's opening hours no price band of one court covers. */
export interface UncoveredHours {
  courtId: string;
  courtName: string;
  day: Schedule['day'];
  /** "HH:MM"; `to` may read earlier than `from` when the gap runs past midnight. */
  from: string;
  to: string;
}

function minutesToTime(total: number): string {
  const wrapped = total % MINUTES_PER_DAY;
  return `${String(Math.floor(wrapped / 60)).padStart(2, '0')}:${String(wrapped % 60).padStart(2, '0')}`;
}

/** The day's window on the server's timeline: minutes from that day's midnight, past 1440 overnight. */
function openWindow(day: OpeningDay): { from: number; to: number } | null {
  if (day.is_closed || day.open_time === day.close_time) return null;
  const from = timeToMinutes(day.open_time);
  const to = timeToMinutes(day.close_time);
  return { from, to: to <= from ? to + MINUTES_PER_DAY : to };
}

/**
 * The opening hours each active court has no price for — hours the storefront
 * never offers, because the server sells only a minute some band covers
 * (`pricing.SlotPrice`: `from_min <= minute < to_min` on the window's own
 * day, minutes past 1440 for the hours after midnight). Changing the opening
 * hours never touches the bands, so hours added to a day open unpriced.
 *
 * Only days the court already has a price for are checked: a day with none is
 * the "this day has no rate" choice the price dialog offers, not a gap.
 */
export function uncoveredOpeningHours(courts: PricedCourt[], schedule: OpeningDay[]): UncoveredHours[] {
  const gaps: UncoveredHours[] = [];
  for (const court of courts) {
    if (!court.is_active) continue;
    for (const day of schedule) {
      const window = openWindow(day);
      const bands = court.prices.filter((p) => p.day_type === day.day).sort((a, b) => a.from_min - b.from_min);
      if (!window || bands.length === 0) continue;

      const push = (from: number, to: number) => {
        gaps.push({
          courtId: court.id,
          courtName: court.name,
          day: day.day,
          from: minutesToTime(from),
          to: minutesToTime(to),
        });
      };
      let cursor = window.from;
      for (const band of bands) {
        if (cursor >= window.to) break;
        if (band.from_min > cursor) push(cursor, Math.min(band.from_min, window.to));
        cursor = Math.max(cursor, band.to_min);
      }
      if (cursor < window.to) push(cursor, window.to);
    }
  }
  return gaps;
}
