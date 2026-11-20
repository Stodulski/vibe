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

/** Minutes since the venue's midnight at the instant `now`. */
function venueMinutesOfDay(now: Date, timeZone: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone,
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(now);
  const value = (type: Intl.DateTimeFormatPartTypes): number => Number(parts.find((p) => p.type === type)?.value ?? 0);
  return value('hour') * 60 + value('minute');
}

/** `HH:MM` (or `HH:MM:SS`) as minutes since midnight. */
function clockMinutes(hhmm: string): number {
  const [hours = 0, minutes = 0] = hhmm.split(':').map(Number);
  return hours * 60 + minutes;
}

const WEEKDAYS = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'];

interface DaySchedule {
  day: string;
  open_time: string;
  close_time: string;
  is_closed: boolean;
}

/**
 * The previous grid day, while its window is still running past midnight: at
 * 00:10 a venue open 20:00-02:00 is still selling last night's 00:30 and 01:00,
 * which fall on today's calendar but belong to yesterday's grid (the API files
 * them there and accepts yesterday's date for exactly this). Null once that
 * window has closed, or when yesterday's window ends by midnight.
 *
 * A window spans midnight the way the server's grid reads it: a close at or
 * before the open runs into the next day, unless it closes exactly at 00:00.
 */
export function lateNightGridDay(
  now: Date,
  schedules: readonly DaySchedule[],
  timeZone: string = VENUE_TIME_ZONE,
): Date | null {
  const today = todayInVenueTz(now, timeZone);
  const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1);
  const schedule = schedules.find((s) => s.day === WEEKDAYS[yesterday.getDay()]);
  if (!schedule || schedule.is_closed) return null;
  const open = clockMinutes(schedule.open_time);
  const close = clockMinutes(schedule.close_time);
  if (close === 0 || close > open) return null;
  return venueMinutesOfDay(now, timeZone) < close ? yesterday : null;
}
