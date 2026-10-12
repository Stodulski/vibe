import type { CourtAvailability } from '../../api/types';

/**
 * Why the grid of the selected day has nothing to offer.
 *
 * - `filtered`: free slots exist for the day, but the sport or the duration the
 *   visitor chose hides them. The day is not full, so the hint says what to change.
 * - `full`: no court has a free slot for any sport at any duration. The day is
 *   fully booked.
 * - `checking`: the other durations are still loading, so the day cannot be
 *   called full yet.
 */
export type EmptyDay = 'filtered' | 'full' | 'checking';

export function hasFreeSlot(courts: readonly CourtAvailability[]): boolean {
  return courts.some((court) => court.slots.some((slot) => slot.available));
}

export interface EmptyDayInput {
  /** The grid, as the sport filter leaves it, has a free slot. */
  shownHasFreeSlot: boolean;
  /** The day, before the sport filter, has a free slot at the chosen duration. */
  unfilteredHasFreeSlot: boolean;
  /** Whether a duration the visitor did not choose has a free slot; `checking` while that is unknown. */
  otherDurations: boolean | 'checking';
}

/**
 * Decides an empty grid's state. Returns `null` when the grid has something to
 * show. A filter that hides free slots is never reported as a full day.
 */
export function classifyEmptyDay({
  shownHasFreeSlot,
  unfilteredHasFreeSlot,
  otherDurations,
}: EmptyDayInput): EmptyDay | null {
  if (shownHasFreeSlot) return null;
  if (unfilteredHasFreeSlot) return 'filtered';
  if (otherDurations === 'checking') return 'checking';
  return otherDurations ? 'filtered' : 'full';
}
