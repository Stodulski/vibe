import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { VENUE_TIME_ZONE } from './instants';

export function formatPrice(cents: number, currency = 'ARS'): string {
  // Whole amounts show no decimals ("$1.500"); an amount that carries
  // centavos shows exactly 2, never a lone trailing digit ("$1.500,50",
  // never "$1.500,5") — money-centavos change, same "hasta 2 decimales"
  // convention every money input/schema in the app now follows.
  const hasCentavos = cents % 100 !== 0;
  // Intl inserts a non-breaking space between the currency symbol and the
  // amount for es-AR (e.g. "$ 150") - tighten it to "$150".
  return new Intl.NumberFormat('es-AR', {
    style: 'currency',
    currency,
    minimumFractionDigits: hasCentavos ? 2 : 0,
    maximumFractionDigits: hasCentavos ? 2 : 0,
  })
    .format(cents / 100)
    .replace(/\s/, '');
}

const CALENDAR_DATE = /^(\d{4})-(\d{2})-(\d{2})(?:T00:00:00(?:\.0+)?Z)?$/;

/**
 * Parses a calendar day into a local `Date`.
 *
 * A booking's `date` is a calendar day, not an instant, and the API serialises
 * it as either `YYYY-MM-DD` or `YYYY-MM-DDT00:00:00Z`. `new Date()` reads both
 * as UTC midnight, which in Argentina (UTC-3) is 21:00 the day before — so
 * every booking used to render one day early. Calendar days are parsed as
 * local midnight instead; real timestamps (`created_at` and friends) still go
 * through `new Date()` untouched.
 *
 * Exported so any bespoke `format(...)` call has a safe way to get its `Date`.
 * The hand-rolled `new Date(date + 'T12:00:00')` some call sites used breaks
 * outright on the timestamp form — it yields `...T00:00:00ZT12:00:00`, an
 * Invalid Date that throws a RangeError out of date-fns.
 */
export function toDisplayDate(date: string | Date): Date {
  if (date instanceof Date) return date;
  const m = CALENDAR_DATE.exec(date);
  if (!m) return new Date(date);
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

export function formatDateFull(date: string | Date): string {
  return format(toDisplayDate(date), "EEEE d 'de' MMMM", { locale: es });
}

const DEADLINE_WEEKDAY = new Intl.DateTimeFormat('es-AR', {
  timeZone: 'America/Argentina/Buenos_Aires',
  weekday: 'long',
});
const DEADLINE_DAY_MONTH = new Intl.DateTimeFormat('es-AR', {
  timeZone: 'America/Argentina/Buenos_Aires',
  day: 'numeric',
  month: 'long',
});
const DEADLINE_TIME = new Intl.DateTimeFormat('es-AR', {
  timeZone: 'America/Argentina/Buenos_Aires',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
});

/**
 * Formats an RFC3339 instant as "sábado 6 de septiembre, 21:00", always
 * read in America/Argentina/Buenos_Aires regardless of the runtime's own
 * timezone. `formatDateFull` above reads a `Date` through the *system*
 * timezone via date-fns, which is right for a calendar day (there is no
 * instant to convert) but wrong for an actual instant like a refund
 * deadline the moment this runs anywhere other than Argentina (CI, most
 * hosting) — hence `Intl.DateTimeFormat` with an explicit `timeZone` here
 * instead of `date-fns`, which has no timezone support without the separate
 * `date-fns-tz` package.
 */
export function formatDeadline(instant: string): string {
  const date = new Date(instant);
  return `${DEADLINE_WEEKDAY.format(date)} ${DEADLINE_DAY_MONTH.format(date)}, ${DEADLINE_TIME.format(date)}`;
}

const VENUE_TIME = new Intl.DateTimeFormat('es-AR', {
  timeZone: VENUE_TIME_ZONE,
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
});
const VENUE_DAY = new Intl.DateTimeFormat('en-CA', {
  timeZone: VENUE_TIME_ZONE,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
});

/**
 * An RFC3339 instant as the clock at the venue reads it, `HH:MM`.
 *
 * This is what `formatTime` cannot do. `formatTime` is `time.slice(0, 5)` — it
 * takes the API's `HH:MM` strings apart with a knife, and handed an instant it
 * returns `"2026-"`. Both are strings and neither the compiler nor a test
 * fixture built from the same value would have said so, which is why this is a
 * separate function rather than a widened `formatTime`.
 *
 * The zone is explicit for the reason `formatDeadline` states next door: the
 * runtime's own timezone is CI's or the hosting region's, and the person
 * reading this is standing at the court.
 *
 * Returns `''` for anything unparseable, so a caller renders nothing rather
 * than "Invalid Date".
 */
export function formatInstantTime(instant: string): string {
  const date = new Date(instant);
  if (Number.isNaN(date.getTime())) return '';
  return VENUE_TIME.format(date);
}

/**
 * Whether a span's end falls on a later calendar day than its start, both read
 * at the venue.
 *
 * Compared as calendar days, not as elapsed hours: a booking is on the next day
 * because the date rolled. A span ending exactly at midnight counts, and is
 * meant to — "23:00 – 00:00" with nothing else is the ambiguity this exists to
 * remove.
 */
export function endsOnALaterDay(startsAt: string, endsAt: string): boolean {
  const start = new Date(startsAt);
  const end = new Date(endsAt);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return false;
  return VENUE_DAY.format(end) !== VENUE_DAY.format(start);
}

/**
 * A booking's hours as a person reads them: `21:00 – 22:30`, or
 * `23:00 – 01:00` for one ending the following day — no marker on the range
 * itself any more. There used to be one ("Día sig.") here; the owner had it
 * removed everywhere it rendered, visible text included. The fact it named
 * is not gone, only no longer printed by this function: a caller that needs
 * it for an accessible name reads `endsOnALaterDay` directly instead — see
 * `BookingBlock`'s `bookingAriaLabel`.
 *
 * Built from real instants, not clock readings, for a real bug: until the
 * server stopped sending `end_time`, `bookings.end_time` was produced by
 * modular arithmetic — a 23:00 booking of two hours read "23:00 – 01:00", an
 * end two hours before its own start, on the owner's calendar, in the booking
 * detail, in the cancel modal and on the public cancel page. Nothing anywhere
 * said which 01:00.
 *
 * `separator` defaults to an en dash with non-breaking spaces so the range
 * never wraps mid-way; the public pages pass their own joiner word.
 */
export function formatHourRange(startsAt: string, endsAt: string, separator = '\u00A0–\u00A0'): string {
  const start = formatInstantTime(startsAt);
  const end = formatInstantTime(endsAt);
  if (!start || !end) return start || end;

  return `${start}${separator}${end}`;
}
