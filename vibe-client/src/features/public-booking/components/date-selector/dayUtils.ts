import type { Schedule } from '@/shared/types/api.types';

const DAY_TO_WEEKDAY: Record<string, number> = {
  sunday: 0,
  monday: 1,
  tuesday: 2,
  wednesday: 3,
  thursday: 4,
  friday: 5,
  saturday: 6,
};

export function isClosedDay(date: Date, schedules: Schedule[]): boolean {
  const weekday = date.getDay();
  const schedule = schedules.find((s) => DAY_TO_WEEKDAY[s.day] === weekday);
  return !schedule || schedule.is_closed;
}
