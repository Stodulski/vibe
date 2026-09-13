import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const DAY_NAMES: Record<string, string> = {
  sunday: t.complex.days.sunday,
  monday: t.complex.days.monday,
  tuesday: t.complex.days.tuesday,
  wednesday: t.complex.days.wednesday,
  thursday: t.complex.days.thursday,
  friday: t.complex.days.friday,
  saturday: t.complex.days.saturday,
};

const WEEKDAYS = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'];

// `Date#getDay()` returns 0-6, always in-bounds for the 7-entry WEEKDAYS
// tuple; the undefined branch is a type-level-only safety net.
export function getDayName(date: Date): string {
  return WEEKDAYS[date.getDay()] ?? 'sunday';
}

export function getTodayName(): string {
  return getDayName(new Date());
}
