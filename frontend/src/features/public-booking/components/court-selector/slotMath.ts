import type { AvailabilitySlot } from '@/shared/types/api.types';

export type TimeGroup = 'morning' | 'afternoon' | 'evening';

/**
 * A slot's total price. The server already prices each slot for the
 * requested duration (via the availability endpoint's `duration` param), so
 * this is a direct passthrough — kept as a named function for symmetry with
 * `getEndTime` and so call sites don't reach into `AvailabilitySlot` directly.
 */
export function computeTotalPrice(slot: AvailabilitySlot): number {
  return slot.price;
}

/**
 * A slot's end time. Like `computeTotalPrice`, the server already computes
 * this for the requested duration.
 */
export function getEndTime(slot: AvailabilitySlot): string {
  return slot.end_time;
}

/**
 * Which part of the day an hour belongs to, from its window position rather
 * than its clock face.
 *
 * It took a `"HH:mm"` string and read the hour off it, which files the 00:30
 * slot of a club open until 02:00 under "morning" — the end of Thursday night
 * shown above Thursday morning. Window minutes do not wrap, so that slot is
 * minute 1470, hour 24, and lands in the evening it actually belongs to.
 */
export function getTimeGroup(startMin: number): TimeGroup {
  const hour = Math.floor(startMin / 60);
  if (hour < 12) return 'morning';
  if (hour < 18) return 'afternoon';
  return 'evening';
}

// `groupSlotsByTime` and its `GroupedSlot` lived here to bucket ONE court's
// slots into morning/afternoon/evening, which is what a court-major grid
// needed. Time-major buckets the hours themselves, across every court at
// once — see `groupTimeOptions` in `timeOptions.ts`.
