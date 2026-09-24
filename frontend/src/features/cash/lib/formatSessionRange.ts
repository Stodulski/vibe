import { VENUE_TIME_ZONE } from '@/shared/lib/instants';

const DAY = new Intl.DateTimeFormat('es-AR', { timeZone: VENUE_TIME_ZONE, day: 'numeric', month: 'numeric' });
const TIME = new Intl.DateTimeFormat('es-AR', {
  timeZone: VENUE_TIME_ZONE,
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
});

function parse(instant: string | null | undefined): Date | null {
  if (!instant) return null;
  const date = new Date(instant);
  return Number.isNaN(date.getTime()) ? null : date;
}

/**
 * A cash session's span for a history row, venue-local and as short as the
 * dates allow: "20/9 · 09:00–22:00" when it opened and closed on the same
 * venue day, "20/9 09:00 – 21/9 02:00" when it ran past midnight. The
 * same-day form repeats no date, which is what keeps the row on one line at
 * 320 px.
 */
export function formatSessionRange(openedAt: string | null | undefined, closedAt: string | null | undefined): string {
  const opened = parse(openedAt);
  if (!opened) return '';
  const openDay = DAY.format(opened);
  const openTime = TIME.format(opened);
  const closed = parse(closedAt);
  if (!closed) return `${openDay} · ${openTime}`;
  const closeDay = DAY.format(closed);
  const closeTime = TIME.format(closed);
  return openDay === closeDay
    ? `${openDay} · ${openTime}–${closeTime}`
    : `${openDay} ${openTime} – ${closeDay} ${closeTime}`;
}
