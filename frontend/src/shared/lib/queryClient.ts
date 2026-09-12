import { QueryCache, QueryClient, MutationCache } from '@tanstack/react-query';
import * as Sentry from '@sentry/react';
import { HTTPError, NetworkError, TimeoutError } from 'ky';
import { ApiError } from '@/shared/lib/ApiError';

/**
 * Statuses a query/mutation fails with often enough in normal use that
 * reporting every one to Sentry would be noise, not signal: an anonymous
 * visit (401), a role boundary (403), a stale link (404), or a form the
 * backend rejected (422). Anything else — 5xx, or the request never getting
 * a response at all — is reported below.
 */
const EXPECTED_STATUSES = new Set([401, 403, 404, 422]);

/**
 * Shared `onError` for both caches (see `queryClient` below).
 *
 * Every failure that came through `src/shared/lib/ky.ts` is an `ApiError`,
 * which read `X-Request-ID` off the response once — so the tag that lets a
 * Sentry event be matched to the backend's own log line for the same request
 * is taken from there rather than re-read here. A bare `HTTPError` (the boot
 * calls that bypass the shared client) still reports, reading the header
 * directly.
 */
function reportQueryError(error: unknown): void {
  if (error instanceof HTTPError) {
    if (EXPECTED_STATUSES.has(error.response.status)) return;

    const requestId = error instanceof ApiError ? error.requestId : error.response.headers.get('X-Request-ID');
    Sentry.captureException(error, {
      tags: {
        status: error.response.status,
        pathname: new URL(error.request.url).pathname,
        ...(requestId ? { request_id: requestId } : {}),
      },
    });
    return;
  }

  // A timeout or a dropped connection never reached `HTTPError` above — no
  // status, no response — but it is exactly the kind of failure a person
  // never sees a toast explain, so it still needs reporting.
  if (error instanceof TimeoutError || error instanceof NetworkError) {
    Sentry.captureException(error);
  }
}

/**
 * Whether a failure is the caller's fault rather than a bad moment.
 *
 * A 401, 403, 404 or 422 answers the same way however many times it is asked:
 * the blind `retry: 1` this replaced spent a second round-trip — and a second
 * spinner — on every one of them before showing the person the error they
 * were always going to get. Only "not now" failures (5xx, a timeout, a dropped
 * connection) are worth asking again.
 */
function isClientError(error: unknown): boolean {
  return error instanceof HTTPError && error.response.status >= 400 && error.response.status < 500;
}

export const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: reportQueryError }),
  mutationCache: new MutationCache({ onError: reportQueryError }),
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,
      gcTime: 10 * 60 * 1000,
      retry: (failureCount, error) => failureCount < 1 && !isClientError(error),
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: 0,
    },
  },
});
