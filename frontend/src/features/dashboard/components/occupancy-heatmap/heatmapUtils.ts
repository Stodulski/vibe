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

/**
 * Reads a heat-ramp step from `globals.css` (odd/tasks/app-dark-contrast.md
 * T3) instead of a literal here, so the ramp has one definition. Step 0 ("sin
 * reservas") used to be 1.08:1 against the lowest active step — close enough
 * to read as the same cell — because both were a thin wash of nearly the same
 * apparent brightness. Step 0 is now a neutral white tint instead of the
 * faintest teal, and the active steps start much higher, so "no bookings" and
 * "some bookings" are never confusable again (see `scripts/contrast-report.mjs`).
 */
export function getHeatColor(percentage: number): string {
  if (typeof document === 'undefined') {
    if (percentage === 0) return 'rgba(255, 255, 255, 0.03)';
    if (percentage < 25) return 'rgba(20, 184, 166, 0.22)';
    if (percentage < 50) return 'rgba(20, 184, 166, 0.38)';
    if (percentage < 75) return 'rgba(20, 184, 166, 0.55)';
    return 'rgba(20, 184, 166, 0.75)';
  }
  const style = getComputedStyle(document.documentElement);
  const read = (name: string, fallback: string) => style.getPropertyValue(name).trim() || fallback;
  if (percentage === 0) return read('--color-chart-heat-0', 'rgba(255, 255, 255, 0.03)');
  if (percentage < 25) return read('--color-chart-heat-1', 'rgba(20, 184, 166, 0.22)');
  if (percentage < 50) return read('--color-chart-heat-2', 'rgba(20, 184, 166, 0.38)');
  if (percentage < 75) return read('--color-chart-heat-3', 'rgba(20, 184, 166, 0.55)');
  return read('--color-chart-heat-4', 'rgba(20, 184, 166, 0.75)');
}
