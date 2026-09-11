import { useWatch } from 'react-hook-form';
import { scheduleField, type ScheduleFormControl } from './scheduleField';

/**
 * Derives the display state (closed/times/weekend/next-day) for a single
 * schedule row from its RHF field values. Shared by the desktop row and
 * mobile card (slice 10, max-lines decomposition).
 */
export function useScheduleRowState(index: number, control: ScheduleFormControl) {
  const isClosed = useWatch({ control, name: scheduleField(index, 'is_closed') });
  const openTime = useWatch({ control, name: scheduleField(index, 'open_time') });
  const closeTime = useWatch({ control, name: scheduleField(index, 'close_time') });
  const isWeekend = index >= 5;

  return { isClosed, openTime, closeTime, isWeekend };
}
