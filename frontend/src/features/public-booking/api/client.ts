import ky, { isHTTPError, type KyInstance, type RetryOptions } from 'ky';
import { ApiError } from '@/shared/lib/ApiError';

/**
 * How long a public booking request gets before it is given up on. The same
 * ten seconds as `REQUEST_TIMEOUT_MS` in `shared/lib/ky.ts`, spelled out here
 * because importing that module would bring in the auth store and the session
 * refresh this client must never run.
 */
export const BOOKING_REQUEST_TIMEOUT_MS = 10_000;

/**
 * The retry policy of `shared/lib/ky.ts`, copied rather than imported for the
 * same reason as the timeout. Only idempotent methods are retried; a POST that
 * creates a booking survives a retry through its `Idempotency-Key` instead.
 */
const RETRY: RetryOptions = {
  limit: 2,
  methods: ['get', 'head', 'options', 'trace'],
  statusCodes: [408, 429, 500, 502, 503, 504],
};

/**
 * The HTTP client of the public booking flow.
 *
 * It sends no credentials, no CSRF header and no session refresh, and it never
 * redirects to `/login`: every endpoint it calls is public. Every failure leaves
 * it as an `ApiError`, so callers read `problem` and `requestId` the same way
 * they do everywhere else in the app.
 */
export function createBookingClient(apiBaseUrl: string): KyInstance {
  return ky.create({
    prefix: apiBaseUrl,
    timeout: BOOKING_REQUEST_TIMEOUT_MS,
    retry: RETRY,
    credentials: 'omit',
    hooks: {
      beforeError: [({ error }) => (isHTTPError(error) ? new ApiError(error) : error)],
    },
  });
}

/**
 * Forwards an optional `AbortSignal` only when there is one. Same helper as
 * `withSignal` in `shared/lib/ky.ts`; see the note there about
 * `exactOptionalPropertyTypes`.
 */
export function withSignal(signal?: AbortSignal): { signal: AbortSignal } | Record<string, never> {
  return signal ? { signal } : {};
}
