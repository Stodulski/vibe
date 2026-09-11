import { timeToMinutes } from '@/shared/lib/time';
import { spanOnDay, type Span } from '@/shared/lib/instants';
import { courtObstacles, windowIsOccupied } from '@/shared/lib/courtOccupancy';
import { toDisplayDate } from '@/shared/lib/utils';
import type { DurationMinutes } from '@/shared/types/api.types';

export function getDayName(dateStr: string) {
  const d = toDisplayDate(dateStr);
  const days = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'] as const;
  return days[d.getDay()];
}

/**
 * The step between selectable start times. 30 rather than 60 because a
 * booking of 90 minutes ends on the half hour, and the next one has to be
 * able to start there — an hourly step makes those starts unreachable.
 */
export const SLOT_MINUTES = 30;

/** The start times the complex is open for, as `HH:MM`. */
export function generateSlots(openTime: string, closeTime: string): string[] {
  const slots: string[] = [];
  const startMin = timeToMinutes(openTime);
  const endMin = timeToMinutes(closeTime);
  for (let min = startMin; min + SLOT_MINUTES <= endMin; min += SLOT_MINUTES) {
    const h = Math.floor(min / 60);
    const m = min % 60;
    slots.push(`${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`);
  }
  return slots;
}

/** The lengths a booking may be sold in, shortest first. */
export const BOOKING_DURATIONS: DurationMinutes[] = [60, 90, 120];

/** The shortest of them, which is what a slot previews before a choice is made. */
export const SHORTEST_BOOKING_MINUTES = BOOKING_DURATIONS[0] ?? 60;

/**
 * The slots still ahead of `nowMin`, or all of them on a day that is not today.
 *
 * A slot whose half hour has already begun is gone: the grid does not offer to
 * book a court for time that has passed.
 */
export function upcomingSlots(slots: string[], nowMin: number | null): string[] {
  if (nowMin === null) return slots;
  return slots.filter((slot) => timeToMinutes(slot) + SLOT_MINUTES > nowMin);
}

/**
 * True when anything overlaps this slot on this court.
 *
 * A thin naming of the shared question in the grid's own vocabulary — a slot
 * here is always one `SLOT_MINUTES` cell. The question itself lives in
 * `@/shared/lib/courtOccupancy`, because the block-slot form asks it too.
 */
export function isSlotOccupied(obstacles: Span[], slotTime: string, date: string): boolean {
  return windowIsOccupied(obstacles, date, timeToMinutes(slotTime), SLOT_MINUTES);
}

export { courtObstacles };

export function fittingDurations(obstacles: Span[], slotTime: string, date: string): DurationMinutes[] {
  const startMin = timeToMinutes(slotTime);
  const midnight = spanOnDay(date, 0, 0);
  if (!midnight) return [];

  // Each obstacle's start, as minutes into the day being shown. An obstacle
  // that began yesterday lands negative and is filtered out below — it is not
  // the *next* thing, and if it were still running here the slot would not be
  // offered at all (isSlotOccupied refuses it first).
  const nextStarts = obstacles
    .map((o) => (o.start.getTime() - midnight.start.getTime()) / 60_000)
    .filter((min) => min > startMin);

  if (nextStarts.length === 0) return BOOKING_DURATIONS;

  const limit = Math.min(...nextStarts);
  return BOOKING_DURATIONS.filter((d) => startMin + d <= limit);
}
