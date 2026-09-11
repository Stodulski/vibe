import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const DAY_LABELS = [
  t.complex.days.monday,
  t.complex.days.tuesday,
  t.complex.days.wednesday,
  t.complex.days.thursday,
  t.complex.days.friday,
  t.complex.days.saturday,
  t.complex.days.sunday,
];

// Monday-first, matching DAY_LABELS above. Read from the shared day names
// rather than typed out again — the copy that lived here had lost the accents
// on Mié and Sáb, which is what happens to text nobody looks for.
export const DAY_LABELS_SHORT = [
  t.courts.daysShort.monday,
  t.courts.daysShort.tuesday,
  t.courts.daysShort.wednesday,
  t.courts.daysShort.thursday,
  t.courts.daysShort.friday,
  t.courts.daysShort.saturday,
  t.courts.daysShort.sunday,
];
export const DAY_LABELS_SINGLE = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];

/**
 * Every hour of the day, 00 to 23.
 *
 * It ran 07:00 to 22:00, which quietly decided that nothing happens outside
 * those sixteen hours. It does: the schedules tab lets a club close after
 * midnight, and this app has venues closing at 00:30 and 01:30. Their busiest
 * late slots were simply not on the chart, and neither was any early booking.
 *
 * The cost is eight more rows, paid for by shorter ones — see DesktopHeatmap.
 */
export const HOURS = Array.from({ length: 24 }, (_, i) => i);

export function heatKey(dayOfWeek: number, hour: number): string {
  return `${String(dayOfWeek)}-${String(hour)}`;
}

export function getHeatColor(percentage: number): string {
  if (percentage === 0) return 'rgba(255, 255, 255, 0.02)';
  if (percentage < 25) return 'rgba(20, 184, 166, 0.10)';
  if (percentage < 50) return 'rgba(20, 184, 166, 0.22)';
  if (percentage < 75) return 'rgba(20, 184, 166, 0.40)';
  return 'rgba(20, 184, 166, 0.62)';
}
