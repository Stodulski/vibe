import { VENUE_TIME_ZONE } from '@/shared/lib/instants';

const SESSION_DATE_TIME = new Intl.DateTimeFormat('es-AR', {
  timeZone: VENUE_TIME_ZONE,
  day: '2-digit',
  month: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
});

/**
 * A session's `opened_at`/`closed_at` as "06/09 21:00", venue-local — compact
 * enough for a history row, where (unlike a movement inside one open shift)
 * the date itself can matter.
 */
export function formatSessionInstant(instant: string | null | undefined): string {
  if (!instant) return '';
  const date = new Date(instant);
  if (Number.isNaN(date.getTime())) return '';
  return SESSION_DATE_TIME.format(date);
}
