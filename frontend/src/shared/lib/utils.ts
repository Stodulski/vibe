import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';
import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { HTTPError, NetworkError, TimeoutError } from 'ky';
import { ES_AR } from '@/shared/i18n/es_AR';
import { VENUE_TIME_ZONE } from '@/shared/lib/instants';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { ApiError, normalizeProblem, type Problem } from '@/shared/lib/ApiError';

const t = ES_AR;

// tailwind-merge only recognizes conflicts within its built-in class groups,
// so it doesn't know these custom @theme font-size tokens (globals.css)
// belong to the same group as text-sm/text-lg/etc. Without this, passing
// e.g. text-nav as a className into a shadcn primitive that already ships
// its own text-sm never dedupes — both classes survive into the DOM, and
// which one wins becomes a coin flip decided by generated-CSS source order.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': ['text-micro', 'text-nav', 'text-brand'],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatPrice(cents: number, currency = 'ARS'): string {
  // Intl inserts a non-breaking space between the currency symbol and the
  // amount for es-AR (e.g. "$ 150") - tighten it to "$150".
  return new Intl.NumberFormat('es-AR', {
    style: 'currency',
    currency,
    minimumFractionDigits: 0,
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

export function formatDate(date: string | Date): string {
  return format(toDisplayDate(date), "dd 'de' MMMM", { locale: es });
}

export function formatDateShort(date: string | Date): string {
  return format(toDisplayDate(date), 'dd/MM/yyyy');
}

export function formatDateCompact(date: string | Date): string {
  return format(toDisplayDate(date), 'dd MMM yyyy', { locale: es });
}

export function formatDateLong(date: string | Date): string {
  return format(toDisplayDate(date), "d 'de' MMMM 'de' yyyy", { locale: es });
}

export function formatDateFull(date: string | Date): string {
  return format(toDisplayDate(date), "EEEE d 'de' MMMM", { locale: es });
}

export function formatTime(time: string): string {
  return time.slice(0, 5);
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
 * A booking's hours as a person reads them: `21:00 – 22:30`, and
 * `23:00 – 01:00 Día sig.` when the end is on the following day.
 *
 * The marker is the whole reason this function exists. Until the server
 * stopped sending `end_time` these ranges were built from two clock readings,
 * and `bookings.end_time` was produced by modular arithmetic — a 23:00 booking
 * of two hours read "23:00 – 01:00", an end two hours before its own start, on
 * the owner's calendar, in the booking detail, in the cancel modal and on the
 * public cancel page. Nothing anywhere said which 01:00.
 *
 * `separator` defaults to an en dash with non-breaking spaces so the range
 * never wraps mid-way; the public pages pass their own joiner word.
 */
export function formatHourRange(startsAt: string, endsAt: string, separator = '\u00A0–\u00A0'): string {
  const start = formatInstantTime(startsAt);
  const end = formatInstantTime(endsAt);
  if (!start || !end) return start || end;

  const range = `${start}${separator}${end}`;
  return endsOnALaterDay(startsAt, endsAt) ? `${range}\u00A0${t.complex.nextDay}` : range;
}

/**
 * Extract a user-facing error message from an API response body.
 *
 * Reads problem+json (`errors[]`, then `detail`, then `title`); falls back to
 * `fallback` for a body that isn't shaped like problem+json at all — a
 * non-JSON or empty response, the case a bare `HTTPError` reaches this from
 * (see `getHttpErrorMessage` below).
 */
export function getApiError(body: unknown, fallback: string): string {
  return problemMessage(normalizeProblem(body, 0), fallback);
}

/**
 * Extract a user-facing error message from a ky `HTTPError`.
 *
 * MUST read `error.data`, never `error.response.json()`: ky populates
 * `error.data` by consuming the response body internally BEFORE the error
 * is thrown, so by the time an `onError` handler runs, `error.response`'s
 * body stream is already read and `.json()`/`.text()` reject with
 * "body stream already read" — silently swallowing the real backend
 * message and always falling back to the generic i18n text.
 */
// Browsers reject a `fetch()` that never reached the server with a bare
// `TypeError` whose message is runtime-specific: "Failed to fetch" (Chromium),
// "NetworkError when attempting to fetch resource." (Firefox), "Load failed"
// (WebKit). Every other `TypeError` is a programming defect ("x is not a
// function", reading a property of `undefined`) and must keep the caller's
// fallback, so only these transport messages count as connectivity failures.
const FETCH_FAILURE_MESSAGES = ['Failed to fetch', 'NetworkError when attempting to fetch resource', 'Load failed'];

function isFetchFailure(error: unknown): error is TypeError {
  return error instanceof TypeError && FETCH_FAILURE_MESSAGES.some((m) => error.message.startsWith(m));
}

/**
 * The sentence a {@link Problem} should show.
 *
 * Field errors first and joined: a 422 that named three fields is more use
 * than the generic title above it. Then `detail` (specific to this
 * occurrence) over `title` (generic to the error type), per RFC 9457.
 */
function problemMessage(problem: Problem, fallback: string): string {
  if (problem.errors.length > 0) return problem.errors.map((e) => e.message).join('. ');
  return problem.detail ?? (problem.title || fallback);
}

export function getHttpErrorMessage(error: unknown, fallback: string): string {
  // The request succeeded but the body didn't match the schema — a
  // caller-specific `fallback` ("no pudimos cancelar la reserva") would
  // misattribute a shape mismatch to the action itself, so this always
  // answers with the generic "invalid response" copy instead, regardless of
  // what the caller passed.
  if (error instanceof ApiResponseError) return t.common.invalidResponse;
  // `ApiError` (every failure out of `src/shared/lib/ky.ts`) has already read
  // the problem+json body into a `Problem`, so its message is read straight
  // off that instead of re-parsing `error.data` through `getApiError` below.
  if (error instanceof ApiError) return problemMessage(error.problem, fallback);
  // A bare `HTTPError` still reaches here from the calls that bypass the
  // shared client (`auth/me`, `auth/refresh`) and from tests that build one.
  if (error instanceof HTTPError) return getApiError(error.data, fallback);

  // No response ever came back — a request that timed out (`TimeoutError`),
  // a dropped/refused connection (`NetworkError`, or the fetch-shaped
  // `TypeError` browsers throw for the same thing, see `isFetchFailure`), or
  // the browser already knowing it's offline. None of these are the caller's fault the way a 4xx/5xx is,
  // so they get the same "check your connection" copy regardless of which
  // caller-specific `fallback` was passed — the same reasoning as
  // `ApiResponseError` above.
  if (error instanceof TimeoutError || error instanceof NetworkError || isFetchFailure(error)) {
    return t.common.networkError;
  }
  // `typeof navigator.onLine === 'boolean'` (not just `typeof navigator !==
  // 'undefined'`): Node itself defines a global `navigator` without an
  // `onLine` property, which would otherwise read as `undefined` and make
  // every non-HTTP error look "offline".
  if (typeof navigator !== 'undefined' && typeof navigator.onLine === 'boolean' && !navigator.onLine) {
    return t.common.networkError;
  }

  return fallback;
}

/**
 * Safely read the HTTP status of a failed request.
 *
 * A mutation's onError can receive more than ky's `HTTPError` — a timeout
 * (`TimeoutError`) or a dropped connection (a plain `TypeError`) has no
 * `.response`, so code that reads `error.response.status` directly crashes
 * on those instead of showing the user an error toast.
 */
export function getHttpStatus(error: unknown): number | undefined {
  return error instanceof HTTPError ? error.response.status : undefined;
}
