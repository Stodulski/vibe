import { useCalendarSchedule } from './useCalendarSchedule';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

interface UseBookingCalendarDataArgs {
  courts: CourtWithPrices[];
  date: string;
  schedules: Schedule[];
}

/**
 * All derived data for `BookingCalendar`. Every hook below (and in the sub-
 * hooks it composes) is called unconditionally (fixes a pre-existing
 * `react-hooks/rules-of-hooks` violation where these `useMemo` calls sat
 * after an early `return` in the component — a real crash risk if the same
 * mounted instance ever transitioned between a "closed today" and "open
 * today" schedule). The "closed" check itself moves to the caller via
 * `isClosedToday`.
 */
export function useBookingCalendarData({ courts, date, schedules }: UseBookingCalendarDataArgs) {
  const { activeCourts, isPast, isClosedToday, slots } = useCalendarSchedule({
    courts,
    date,
    schedules,
  });

  return {
    isClosedToday,
    isPast,
    activeCourts,
    slots,
  };
}
