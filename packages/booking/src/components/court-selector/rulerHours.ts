import type { CourtAvailability } from '../../api/types';
import { buildTimeOptions, type TimeOption } from './timeOptions';

/** Argentina has no daylight saving time, so the venue's offset is fixed. */
const VENUE_UTC_OFFSET = '-03:00';

/** One hour on the time ruler: free, with the option the court question books from, or taken. */
export interface RulerHour {
  startTime: string;
  startMin: number;
  endTime: string;
  /** `null` when every court is taken at this hour. */
  free: TimeOption | null;
}

/** When a slot starts: the day's midnight in the venue's zone plus its minutes, which run past 1440 overnight. */
function startsAt(date: string, startMin: number): number {
  return new Date(`${date}T00:00:00${VENUE_UTC_OFFSET}`).getTime() + startMin * 60_000;
}

/**
 * Every hour of the day, free or taken, in window order. A taken hour that already
 * began is left out when the day is known: it is past, not busy. A free hour is the server's call.
 */
export function buildRulerHours(courts: CourtAvailability[], date?: string, now: Date = new Date()): RulerHour[] {
  const free = new Map(buildTimeOptions(courts).map((option) => [option.startTime, option]));
  const hours = new Map<string, RulerHour>();
  for (const court of courts) {
    for (const slot of court.slots) {
      if (!slot.available && date !== undefined && startsAt(date, slot.start_min) <= now.getTime()) continue;
      if (hours.has(slot.start_time)) continue;
      hours.set(slot.start_time, {
        startTime: slot.start_time,
        startMin: slot.start_min,
        endTime: slot.end_time,
        free: free.get(slot.start_time) ?? null,
      });
    }
  }
  return [...hours.values()].sort((a, b) => a.startMin - b.startMin);
}

/** The free hour nearest `index`, or `null` when the day has none. Ties go to the later hour. */
export function nearestFree(hours: readonly RulerHour[], index: number): number | null {
  for (let step = 0; step < hours.length; step++) {
    for (const candidate of [index + step, index - step]) {
      if (hours[candidate]?.free) return candidate;
    }
  }
  return null;
}
