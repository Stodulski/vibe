import { useCallback } from 'react';
import type { AvailabilitySlot, CourtAvailability } from '@/shared/types/api.types';
import { computeTotalPrice, getEndTime } from './slotMath';
import type { SelectedSlot } from './types';

interface UseCourtSelectorStateParams {
  onSelect: (selection: SelectedSlot) => void;
}

/**
 * Turning a (court, slot) pair into the selection the confirm step expects.
 *
 * This used to also own arrow-key navigation across a court's slots, moving
 * focus by ±1 and ±3 — three being the number of columns, hardcoded, in a
 * grid that has been three, four, six or eight columns depending on the
 * viewport since it was written. Arrow keys therefore jumped to the wrong row
 * on every screen but the smallest. Tab order walks a grid of buttons
 * correctly without being told how wide it is, so the custom handler is gone
 * rather than fixed.
 */
export function useCourtSelectorState({ onSelect }: UseCourtSelectorStateParams) {
  /** Reports the selection and returns it, or null for a slot that is taken. */
  const handleSlotClick = useCallback(
    (court: CourtAvailability, slot: AvailabilitySlot): SelectedSlot | null => {
      if (!slot.available) return null;

      const selection: SelectedSlot = {
        courtId: court.court_id,
        courtName: court.court_name,
        sport: court.sport,
        courtType: court.court_type,
        courtDescription: court.description,
        slot,
        durationMinutes: slot.duration_minutes,
        totalPrice: computeTotalPrice(slot),
        endTime: getEndTime(slot),
      };
      onSelect(selection);
      return selection;
    },
    [onSelect],
  );

  return { handleSlotClick };
}
