/**
 * The venue's calendar day. Every complex is in Argentina and availability is in the
 * venue's local time, while the server (Vercel, UTC) and the browser each have their own
 * clock. "Today" is therefore read in the venue's zone, never from the process or the
 * browser, so the server render and the hydrating client agree.
 *
 * The public complex payload carries no timezone, so the venue zone is this constant.
 */
export const VENUE_TIME_ZONE = 'America/Argentina/Buenos_Aires';

/** The venue's calendar date as `yyyy-MM-dd`, for the instant `now`. Independent of the process zone. */
export function venueDateString(now: Date = new Date(), timeZone: string = VENUE_TIME_ZONE): string {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(now);
  const value = (type: Intl.DateTimeFormatPartTypes): string => parts.find((p) => p.type === type)?.value ?? '';
  return `${value('year')}-${value('month')}-${value('day')}`;
}

/**
 * The venue's today as a `Date` at local midnight, so `format`, `addDays` and `getDay`
 * read the venue's calendar date whatever the browser's zone is.
 */
export function todayInVenueTz(now: Date = new Date(), timeZone: string = VENUE_TIME_ZONE): Date {
  const [year = 1970, month = 1, day = 1] = venueDateString(now, timeZone).split('-').map(Number);
  return new Date(year, month - 1, day);
}
