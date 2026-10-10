import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { HTTPError, NetworkError, TimeoutError } from 'ky';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { ApiError, normalizeProblem, type Problem } from '@/shared/lib/ApiError';
import { toDisplayDate } from './format';

export { cn } from './cn';
export {
  formatPrice,
  toDisplayDate,
  formatDateFull,
  formatDeadline,
  formatInstantTime,
  endsOnALaterDay,
  formatHourRange,
} from './format';

const t = ES_AR;

export function formatDate(date: string | Date): string {
  return format(toDisplayDate(date), "dd 'de' MMMM", { locale: es });
}

export function formatDateShort(date: string | Date): string {
  return format(toDisplayDate(date), 'dd/MM/yyyy');
}

export function formatDateLong(date: string | Date): string {
  return format(toDisplayDate(date), "d 'de' MMMM 'de' yyyy", { locale: es });
}

/**
 * A date as "mar 2026" — the month abbreviated. Used where only the month a
 * record was created in matters (e.g. "Cliente desde" in a dense table row),
 * not the exact day.
 */
export function formatMonthYear(date: string | Date): string {
  return format(toDisplayDate(date), 'MMM yyyy', { locale: es });
}

export function formatTime(time: string): string {
  return time.slice(0, 5);
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
