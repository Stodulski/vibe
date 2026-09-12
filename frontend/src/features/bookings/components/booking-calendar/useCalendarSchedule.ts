import { getDayName, generateSlots } from './helpers';
import { todayInArgentina } from '../../lib/today';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

export function useCalendarSchedule({
  courts,
  date,
  schedules,
}: {
  courts: CourtWithPrices[];
  date: string;
  schedules: Schedule[];
}) {
  const activeCourts = courts.filter((c) => c.is_active);
  const isPast = date < todayInArgentina();

  const dayName = getDayName(date);
  const todaySchedule = schedules.find((s) => s.day === dayName);
  const isClosedToday = todaySchedule?.is_closed ?? false;

  // Staff can book any hour of the day from this dashboard regardless of the
  // complex's opening hours, so the free-slot controls always cover the
  // whole day (00:00-24:00) rather than the day's `Schedule` window — only a
  // day marked fully closed hides the grid at all (see `BookingCalendar`'s
  // `isClosedToday` branch).
  const slots = isClosedToday ? [] : generateSlots('00:00', '24:00');

  return {
    activeCourts,
    isPast,
    isClosedToday,
    slots,
  };
}
