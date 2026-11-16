import { useMemo } from 'react';
import { useSchedules, useSelectedComplex } from '@/features/complex';
import { timeOptionsForOpeningHours } from '@/shared/lib/timeOptions';

/**
 * The hours the event form offers: those the selected complex is open, so an
 * event is never set for a time its courts are shut. `current` are the values
 * already chosen; they stay in the list even when outside it. Until the
 * schedule loads, the whole day is offered rather than an empty select.
 */
export function useEventTimeOptions(current: readonly string[]): string[] {
  const { complex } = useSelectedComplex();
  const { data: schedules } = useSchedules(complex?.id ?? '', complex?.slug);
  const key = current.join('|');
  return useMemo(
    () => timeOptionsForOpeningHours(schedules, current),
    // `current` is a fresh array every render; its content is what matters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [schedules, key],
  );
}
