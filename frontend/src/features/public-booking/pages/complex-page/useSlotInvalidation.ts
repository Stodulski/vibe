import { useEffect } from 'react';
import { toast } from 'sonner';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { AvailabilityData, CourtAvailability } from '@/shared/types/api.types';
import type { SelectedSlot } from '@/features/public-booking';

const t = ES_AR;

/** Whether `selectedSlot`'s court and hour are still bookable in `availability`. */
export function isSlotStillAvailable(
  selectedSlot: SelectedSlot | null,
  availability: AvailabilityData | undefined,
): boolean {
  // Nothing selected, or nothing to check it against yet — there is nothing
  // to invalidate.
  if (!selectedSlot || !availability) return true;
  const court = availability.courts.find((c) => c.court_id === selectedSlot.courtId);
  const slot = court?.slots.find((s) => s.start_time === selectedSlot.slot.start_time);
  return !!slot?.available;
}

/**
 * Whether some court in `courts` still has an available slot at `startTime`
 * — the same rule `CourtSelector`'s own time options use to decide which
 * hours exist at all. Used to keep the open "which court?" question (held as
 * a start time, not an option object — see `CourtSelector`) from outliving
 * the hour it was about.
 */
export function isStartTimeStillOffered(startTime: string | null, courts: CourtAvailability[]): boolean {
  if (!startTime) return true;
  return courts.some((court) => court.slots.some((s) => s.start_time === startTime && s.available));
}

/**
 * The selected slot, or `null` once a fresh `availability` answer shows it is
 * no longer available — e.g. someone else booked it while the visitor was
 * filling in the form.
 *
 * "Still available" is derived on every render instead of copied into its
 * own piece of state, so there is nothing to keep in sync and no window where
 * the two can disagree. The one real side effect left — telling the visitor
 * why their selection disappeared — stays in an effect on purpose: a toast is
 * a call to an external system, not a `setState`, so it never trips
 * `react-hooks/set-state-in-effect` and needs no `queueMicrotask` workaround.
 */
export function useSlotInvalidation(
  selectedSlot: SelectedSlot | null,
  availability: AvailabilityData | undefined,
): SelectedSlot | null {
  const stillAvailable = isSlotStillAvailable(selectedSlot, availability);

  useEffect(() => {
    if (selectedSlot && !stillAvailable) {
      toast.error(t.publicBooking.slotNoLongerAvailable);
    }
  }, [selectedSlot, stillAvailable]);

  return stillAvailable ? selectedSlot : null;
}
